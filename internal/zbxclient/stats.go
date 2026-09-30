package zbxclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type TrendSample struct {
	Clock    int64   `json:"clock"`
	ValueMin float64 `json:"value_min"`
	ValueAvg float64 `json:"value_avg"`
	ValueMax float64 `json:"value_max"`
}

func (s *TrendSample) UnmarshalJSON(data []byte) error {
	var raw struct {
		Clock    any `json:"clock"`
		ValueMin any `json:"value_min"`
		ValueAvg any `json:"value_avg"`
		ValueMax any `json:"value_max"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	parseFloat := func(v any) float64 {
		switch val := v.(type) {
		case float64:
			return val
		case string:
			f, _ := strconv.ParseFloat(val, 64)
			return f
		default:
			return 0
		}
	}

	parseInt64 := func(v any) int64 {
		switch val := v.(type) {
		case float64:
			return int64(val)
		case string:
			n, _ := strconv.ParseInt(val, 10, 64)
			return n
		default:
			return 0
		}
	}

	s.Clock = parseInt64(raw.Clock)
	s.ValueMin = parseFloat(raw.ValueMin)
	s.ValueAvg = parseFloat(raw.ValueAvg)
	s.ValueMax = parseFloat(raw.ValueMax)
	return nil
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
	Error      string
}

type ClusterPeak struct {
	Time    time.Time
	CPUAvg  float64
	RAMAvg  float64
	LoadAvg float64
}

// StatsQuery describes one show_host_stats / export_graph summary request.
type StatsQuery struct {
	Targets     []string
	Hostgroups  []string
	TimeFrom    int64
	TimeTill    int64
	Filter      TrendFilter
	Peak        bool
	Concurrency int
}

// FetchTrends returns hourly trend samples for one item in [from, till].
func (c *Client) FetchTrends(ctx context.Context, itemID string, from, till int64) ([]TrendSample, error) {
	var trends []TrendSample
	err := c.Call(ctx, "trend.get", map[string]any{
		"itemids":   []string{itemID},
		"time_from": from,
		"time_till": till,
		"output":    []string{"clock", "value_min", "value_avg", "value_max"},
	}, &trends)
	return trends, err
}

func FilterTrendSamples(samples []TrendSample, f TrendFilter) []TrendSample {
	var res []TrendSample
	for _, s := range samples {
		if f.Keep(s.Clock) {
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
		searchParam := map[string]string{
			"host": target,
			"name": target,
		}
		var hosts []HostRecord
		err := c.Call(ctx, "host.get", map[string]any{
			"search":                 searchParam,
			"searchWildcardsEnabled": true,
			"searchByAny":            true,
			"output":                 []string{"hostid", "host", "name"},
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
	q StatsQuery,
) ([]HostStats, *ClusterPeak, error) {
	hosts, ipMap, err := c.ResolveHosts(ctx, q.Targets)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving hosts: %w", err)
	}
	if len(hosts) == 0 {
		return nil, nil, fmt.Errorf("no hosts found matching targets")
	}

	concurrency := q.Concurrency
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

			hName := host.Name
			if hName == "" {
				hName = host.Host
			}

			// Fetch items
			var items []ItemRecord
			err := c.Call(gctx, "item.get", map[string]any{
				"hostids": host.HostID,
				"output":  []string{"itemid", "name", "key_", "lastvalue"},
			}, &items)
			if err != nil {
				mu.Lock()
				results[idx] = HostStats{
					HostID:   host.HostID,
					HostName: hName,
					IP:       ipMap[host.HostID],
					Error:    "item.get: " + err.Error(),
				}
				mu.Unlock()
				return nil
			}

			var cpuNumID, cpuNumVal string
			for _, it := range items {
				if strings.Contains(it.Key, "system.cpu.num") {
					cpuNumID = it.ItemID
					cpuNumVal = it.LastValue
					break
				}
			}

			var memTotID, memTotVal string
			for _, it := range items {
				if strings.Contains(it.Key, "vm.memory.size[total]") {
					memTotID = it.ItemID
					memTotVal = it.LastValue
					break
				}
			}

			itemID := func(metric string) string {
				m, _ := LookupMetric(metric)
				if it, ok := m.FindItem(items); ok {
					return it.ItemID
				}
				return ""
			}
			cpuUtilID, memUtilID, loadID := itemID("cpu"), itemID("ram"), itemID("load")

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

			var fetchErrs []string
			fetchMetric := func(itemID string) (float64, float64, []TrendSample) {
				if itemID == "" {
					return 0, 0, nil
				}
				trends, err := c.FetchTrends(gctx, itemID, q.TimeFrom, q.TimeTill)
				if err != nil {
					fetchErrs = append(fetchErrs, "trend.get "+itemID+": "+err.Error())
					return 0, 0, nil
				}
				filtered := FilterTrendSamples(trends, q.Filter)
				avg, max := CalculateTrendMetrics(filtered, q.Peak)
				return avg, max, filtered
			}

			cpuAvg, cpuMax, cpuTrends := fetchMetric(cpuUtilID)
			ramAvg, ramMax, ramTrends := fetchMetric(memUtilID)
			loadAvg, loadMax, loadTrends := fetchMetric(loadID)

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
				Error:      strings.Join(fetchErrs, "; "),
			}

			mu.Lock()
			results[idx] = stat

			// Aggregate for cluster peak calculation
			if stat.Error == "" {
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
