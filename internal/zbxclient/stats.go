package zbxclient

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type TrendSample struct {
	Clock    int64   `json:"clock,string"`
	ValueAvg float64 `json:"value_avg,string"`
	ValueMax float64 `json:"value_max,string"`
}

type HostStats struct {
	HostID     string
	HostName   string
	IP         string
	CPUCores   int
	CPUAvg     float64
	CPUMax     float64
	RAMTotalGB float64
	RAMAvg     float64
	RAMMax     float64
	LoadAvg    float64
	LoadMax    float64
}

type ClusterPeak struct {
	Time    time.Time
	CPUAvg  float64
	RAMAvg  float64
	LoadAvg float64
}

func FilterTrendSamples(samples []TrendSample, businessHours bool) []TrendSample {
	if !businessHours {
		return samples
	}

	var res []TrendSample
	for _, s := range samples {
		t := time.Unix(s.Clock, 0)
		wd := t.Weekday()
		// Monday = 1, Friday = 5
		if wd < time.Monday || wd > time.Friday {
			continue
		}
		h := t.Hour()
		if (h >= 8 && h < 12) || (h >= 13 && h < 17) {
			res = append(res, s)
		}
	}
	return res
}

func CalculateTrendMetrics(samples []TrendSample, peak bool) (float64, float64) {
	if len(samples) == 0 {
		return 0, 0
	}

	var sumAvg float64
	var maxVal float64
	for i, s := range samples {
		sumAvg += s.ValueAvg
		val := s.ValueAvg
		if peak {
			val = s.ValueMax
		}
		if i == 0 || val > maxVal {
			maxVal = val
		}
	}
	avgVal := sumAvg / float64(len(samples))
	return avgVal, maxVal
}

type ItemRecord struct {
	ItemID    string `json:"itemid"`
	Name      string `json:"name"`
	Key       string `json:"key_"`
	LastValue string `json:"lastvalue"`
}

type HostRecord struct {
	HostID string `json:"hostid"`
	Host   string `json:"host"`
	Name   string `json:"name"`
}

type HostInterfaceRecord struct {
	HostID string `json:"hostid"`
	IP     string `json:"ip"`
}

func (c *Client) ResolveHosts(ctx context.Context, targets []string) ([]HostRecord, map[string]string, error) {
	hostMap := make(map[string]HostRecord)
	ipMap := make(map[string]string)

	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}

		// Check if target is an IP
		if net.ParseIP(target) != nil {
			var ifaces []HostInterfaceRecord
			err := c.Call(ctx, "hostinterface.get", map[string]any{
				"filter": map[string]string{"ip": target},
				"output": []string{"hostid", "ip"},
			}, &ifaces)
			if err == nil && len(ifaces) > 0 {
				var hostIDs []string
				for _, iface := range ifaces {
					hostIDs = append(hostIDs, iface.HostID)
					ipMap[iface.HostID] = iface.IP
				}
				var hosts []HostRecord
				err = c.Call(ctx, "host.get", map[string]any{
					"hostids": hostIDs,
					"output":  []string{"hostid", "host", "name"},
				}, &hosts)
				if err == nil {
					for _, h := range hosts {
						hostMap[h.HostID] = h
					}
				}
				continue
			}
		}

		// Otherwise resolve by host name / wildcard search
		searchParam := map[string]string{"name": target}
		if strings.Contains(target, "*") {
			searchParam = map[string]string{"host": strings.ReplaceAll(target, "*", "")}
		}
		var hosts []HostRecord
		err := c.Call(ctx, "host.get", map[string]any{
			"search":                searchParam,
			"searchWildcardsEnabled": true,
			"output":                []string{"hostid", "host", "name"},
		}, &hosts)
		if err == nil {
			for _, h := range hosts {
				hostMap[h.HostID] = h
			}
		}
	}

	var res []HostRecord
	for _, h := range hostMap {
		res = append(res, h)
	}

	// For any host without IP in ipMap, fetch its interfaces
	var missingIPHostIDs []string
	for _, h := range res {
		if _, ok := ipMap[h.HostID]; !ok {
			missingIPHostIDs = append(missingIPHostIDs, h.HostID)
		}
	}
	if len(missingIPHostIDs) > 0 {
		var ifaces []HostInterfaceRecord
		_ = c.Call(ctx, "hostinterface.get", map[string]any{
			"hostids": missingIPHostIDs,
			"output":  []string{"hostid", "ip"},
		}, &ifaces)
		for _, iface := range ifaces {
			if _, exists := ipMap[iface.HostID]; !exists {
				ipMap[iface.HostID] = iface.IP
			}
		}
	}

	return res, ipMap, nil
}

func (c *Client) GetHostStatsSummary(
	ctx context.Context,
	targets []string,
	days int,
	businessHours bool,
	peak bool,
	concurrency int,
) ([]HostStats, *ClusterPeak, error) {
	hosts, ipMap, err := c.ResolveHosts(ctx, targets)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving hosts: %w", err)
	}
	if len(hosts) == 0 {
		return nil, nil, fmt.Errorf("no hosts found matching targets")
	}

	now := time.Now().Unix()
	timeFrom := now - int64(days*86400)

	if concurrency <= 0 {
		concurrency = 10
	}

	var mu sync.Mutex
	results := make([]HostStats, len(hosts))

	// Hourly aggregation map for cluster peak: hourTimestamp -> struct
	type hourlyClusterMetric struct {
		cpuSum   float64
		cpuCount int
		ramSum   float64
		ramCount int
		loadSum  float64
	}
	clusterHours := make(map[int64]*hourlyClusterMetric)

	g, gctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, concurrency)

	for i, h := range hosts {
		idx := i
		host := h
		g.Go(func() error {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-gctx.Done():
				return gctx.Err()
			}

			// Fetch items
			var items []ItemRecord
			err := c.Call(gctx, "item.get", map[string]any{
				"hostids": host.HostID,
				"output":  []string{"itemid", "name", "key_", "lastvalue"},
			}, &items)
			if err != nil {
				return nil
			}

			var cpuNumID, memTotID, cpuUtilID, memUtilID, loadID string
			var cpuNumVal, memTotVal string

			for _, it := range items {
				k := it.Key
				nameLower := strings.ToLower(it.Name)

				if strings.Contains(k, "system.cpu.num") {
					cpuNumID = it.ItemID
					cpuNumVal = it.LastValue
				} else if strings.Contains(k, "vm.memory.size[total]") {
					memTotID = it.ItemID
					memTotVal = it.LastValue
				} else if k == "system.cpu.util" || (cpuUtilID == "" && strings.Contains(nameLower, "cpu utilization")) {
					cpuUtilID = it.ItemID
				} else if k == "vm.memory.utilization" || k == "vm.memory.util" || k == "vm.memory.size[pused]" || (memUtilID == "" && strings.Contains(nameLower, "memory utilization")) {
					memUtilID = it.ItemID
				} else if k == "system.cpu.load[all,avg15]" || (loadID == "" && strings.Contains(k, "system.cpu.load") && strings.Contains(k, "15")) {
					loadID = it.ItemID
				}
			}

			cores, _ := strconv.Atoi(cpuNumVal)
			if cores <= 0 && cpuNumID != "" {
				// Fallback: try history
				type HistRecord struct {
					Value string `json:"value"`
				}
				var hist []HistRecord
				_ = c.Call(gctx, "history.get", map[string]any{
					"itemids":   []string{cpuNumID},
					"history":   3,
					"limit":     1,
					"sortfield": "clock",
					"sortorder": "DESC",
				}, &hist)
				if len(hist) > 0 {
					cores, _ = strconv.Atoi(hist[0].Value)
				}
			}

			memBytes, _ := strconv.ParseFloat(memTotVal, 64)
			ramGB := memBytes / (1024 * 1024 * 1024)
			if ramGB <= 0 && memTotID != "" {
				type HistRecord struct {
					Value string `json:"value"`
				}
				var hist []HistRecord
				_ = c.Call(gctx, "history.get", map[string]any{
					"itemids":   []string{memTotID},
					"history":   3,
					"limit":     1,
					"sortfield": "clock",
					"sortorder": "DESC",
				}, &hist)
				if len(hist) > 0 {
					val, _ := strconv.ParseFloat(hist[0].Value, 64)
					ramGB = val / (1024 * 1024 * 1024)
				}
			}

			// Helper to fetch and filter trends
			fetchMetric := func(itemID string) (float64, float64, []TrendSample) {
				if itemID == "" {
					return 0, 0, nil
				}
				var trends []TrendSample
				_ = c.Call(gctx, "trend.get", map[string]any{
					"itemids":   []string{itemID},
					"time_from": timeFrom,
					"time_till": now,
					"output":    []string{"clock", "value_avg", "value_max"},
				}, &trends)
				filtered := FilterTrendSamples(trends, businessHours)
				avg, max := CalculateTrendMetrics(filtered, peak)
				return avg, max, filtered
			}

			cpuAvg, cpuMax, cpuTrends := fetchMetric(cpuUtilID)
			ramAvg, ramMax, ramTrends := fetchMetric(memUtilID)
			loadAvg, loadMax, loadTrends := fetchMetric(loadID)

			hName := host.Name
			if hName == "" {
				hName = host.Host
			}

			stat := HostStats{
				HostID:     host.HostID,
				HostName:   hName,
				IP:         ipMap[host.HostID],
				CPUCores:   cores,
				CPUAvg:     cpuAvg,
				CPUMax:     cpuMax,
				RAMTotalGB: ramGB,
				RAMAvg:     ramAvg,
				RAMMax:     ramMax,
				LoadAvg:    loadAvg,
				LoadMax:    loadMax,
			}

			mu.Lock()
			results[idx] = stat

			// Aggregate for cluster peak calculation
			for _, s := range cpuTrends {
				hour := (s.Clock / 3600) * 3600
				rec, ok := clusterHours[hour]
				if !ok {
					rec = &hourlyClusterMetric{}
					clusterHours[hour] = rec
				}
				rec.cpuSum += s.ValueAvg
				rec.cpuCount++
			}
			for _, s := range ramTrends {
				hour := (s.Clock / 3600) * 3600
				rec, ok := clusterHours[hour]
				if !ok {
					rec = &hourlyClusterMetric{}
					clusterHours[hour] = rec
				}
				rec.ramSum += s.ValueAvg
				rec.ramCount++
			}
			for _, s := range loadTrends {
				hour := (s.Clock / 3600) * 3600
				rec, ok := clusterHours[hour]
				if !ok {
					rec = &hourlyClusterMetric{}
					clusterHours[hour] = rec
				}
				rec.loadSum += s.ValueAvg
			}
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	// Calculate Cluster Peak (hour with highest combined CPU load)
	var peakHour int64
	var peakCPU, peakRAM, peakLoad float64
	for hour, rec := range clusterHours {
		var avgCPU, avgRAM float64
		if rec.cpuCount > 0 {
			avgCPU = rec.cpuSum / float64(rec.cpuCount)
		}
		if rec.ramCount > 0 {
			avgRAM = rec.ramSum / float64(rec.ramCount)
		}
		if avgCPU > peakCPU || peakHour == 0 {
			peakHour = hour
			peakCPU = avgCPU
			peakRAM = avgRAM
			peakLoad = rec.loadSum
		}
	}

	var clusterPeak *ClusterPeak
	if peakHour > 0 {
		clusterPeak = &ClusterPeak{
			Time:    time.Unix(peakHour, 0),
			CPUAvg:  peakCPU,
			RAMAvg:  peakRAM,
			LoadAvg: peakLoad,
		}
	}

	return results, clusterPeak, nil
}
