package zbxclient

import (
	"context"
	"fmt"
	"regexp"
)

var controlChars = regexp.MustCompile(`[\x00-\x1f\x7f]`)

// WindowRequest is one validated export_window invocation.
type WindowRequest struct {
	From, To       string
	Since, Until   int64
	Timezone       string
	Role           string
	Targets        []string
	Filter         TrendFilter
	BusinessHours  bool
	Hours          string
	Peak           bool
	AdapterVersion string
}

type windowMapping struct {
	ItemID string `json:"itemid"`
	Key    string `json:"key"`
}

type windowMetric struct {
	Mapping         *windowMapping `json:"mapping"`
	State           string         `json:"state"`
	SampleCount     int            `json:"sample_count"`
	Min             *float64       `json:"min"`
	Avg             *float64       `json:"avg"`
	Max             *float64       `json:"max"`
	Peak            *float64       `json:"peak"`
	MissingInterval bool           `json:"missing_interval"`
	Error           string         `json:"error,omitempty"`
}

type windowHost struct {
	Host    string                  `json:"host"`
	HostID  string                  `json:"hostid,omitempty"`
	IP      string                  `json:"ip"`
	Metrics map[string]windowMetric `json:"metrics"`
	State   string                  `json:"state"`
	Errors  []map[string]string     `json:"errors,omitempty"`
}

// WindowDocument is the zabbix-cli.export_window v1 payload.
type WindowDocument struct {
	Schema     string              `json:"schema"`
	Version    int                 `json:"version"`
	Status     string              `json:"status"`
	Role       string              `json:"role"`
	Window     map[string]any      `json:"window"`
	Filters    map[string]any      `json:"filters"`
	Source     map[string]string   `json:"source"`
	Provenance map[string]any      `json:"provenance"`
	Hosts      []windowHost        `json:"hosts"`
	Errors     []map[string]string `json:"errors"`
}

func windowStats(samples []TrendSample, peak bool) windowMetric {
	if len(samples) == 0 {
		return windowMetric{State: "UNPROVEN", MissingInterval: true}
	}
	min, max, maxAvg, sum := samples[0].ValueMin, samples[0].ValueMax, samples[0].ValueAvg, 0.0
	for _, s := range samples {
		if s.ValueMin < min {
			min = s.ValueMin
		}
		if s.ValueMax > max {
			max = s.ValueMax
		}
		if s.ValueAvg > maxAvg {
			maxAvg = s.ValueAvg
		}
		sum += s.ValueAvg
	}
	avg := sum / float64(len(samples))
	pk := maxAvg
	if peak {
		pk = max
	}
	return windowMetric{State: "OK", SampleCount: len(samples), Min: &min, Avg: &avg, Max: &max, Peak: &pk}
}

// ExportWindow builds the document; it never fails as a whole, it records
// per-host and per-metric failures in the payload instead.
func (c *Client) ExportWindow(ctx context.Context, r WindowRequest) WindowDocument {
	doc := WindowDocument{
		Schema: "zabbix-cli.export_window", Version: 1, Role: r.Role,
		Window: map[string]any{"from": r.From, "to": r.To, "since": r.Since, "until": r.Until,
			"timezone": r.Timezone, "inclusive": "[since,until)"},
		Source: map[string]string{"type": "zabbix_api"},
		Provenance: map[string]any{"source": "zabbix_api", "adapter_version": r.AdapterVersion, "command": "export_window",
			"argv": []string{"--from", r.From, "--to", r.To, "--timezone", r.Timezone, "--role", r.Role, "--input-file", "<redacted>"}},
		Hosts: []windowHost{}, Errors: []map[string]string{},
	}
	var hours, ranges any
	if r.Hours != "" {
		hours = r.Hours
	}
	if len(r.Filter.HourRanges) > 0 {
		ranges = r.Filter.HourRanges
	}
	doc.Filters = map[string]any{"business_hours": r.BusinessHours, "hours": hours, "hour_ranges": ranges,
		"weekdays_only": r.Filter.WeekdaysOnly, "peak": r.Peak}

	overall := "OK"
	for _, target := range r.Targets {
		if controlChars.MatchString(target) {
			doc.Errors = append(doc.Errors, map[string]string{"host": "<invalid>", "error": "host contains control characters", "state": "ERROR"})
			continue
		}
		hosts, ipMap, err := c.resolveTargets(ctx, []string{target})
		if err != nil {
			doc.Errors = append(doc.Errors, map[string]string{"host": target, "error": fmt.Sprintf("%T", err), "state": "ERROR"})
			continue
		}
		if len(hosts) == 0 {
			doc.Errors = append(doc.Errors, map[string]string{"host": target, "error": "host not found", "state": "UNPROVEN"})
			continue
		}
		for _, h := range hosts {
			name := h.Name
			if name == "" {
				name = h.Host
			}
			if name == "" {
				name = h.HostID
			}
			var items []ItemRecord
			if err := c.Call(ctx, "item.get", map[string]any{"hostids": h.HostID, "output": []string{"itemid", "name", "key_"}}, &items); err != nil {
				doc.Hosts = append(doc.Hosts, windowHost{Host: name, Metrics: map[string]windowMetric{}, State: "ERROR",
					Errors: []map[string]string{{"error": err.Error()}}})
				overall = "PARTIAL"
				continue
			}
			wh := windowHost{Host: name, HostID: h.HostID, IP: ipMap[h.HostID], Metrics: map[string]windowMetric{}, State: "OK"}
			for _, metricName := range WindowMetrics {
				spec, _ := LookupMetric(metricName)
				it, ok := spec.FindItem(items)
				if !ok {
					wh.Metrics[metricName] = windowMetric{State: "UNPROVEN", MissingInterval: true}
					wh.State, overall = "PARTIAL", "PARTIAL"
					continue
				}
				mapping := &windowMapping{ItemID: it.ItemID, Key: it.Key}
				trends, err := c.FetchTrends(ctx, it.ItemID, r.Since, r.Until-1)
				if err != nil {
					wh.Metrics[metricName] = windowMetric{Mapping: mapping, State: "ERROR", Error: err.Error()}
					wh.State, overall = "PARTIAL", "PARTIAL"
					continue
				}
				var kept []TrendSample
				for _, s := range trends {
					if s.Clock >= r.Since && s.Clock < r.Until && r.Filter.Keep(s.Clock) {
						kept = append(kept, s)
					}
				}
				m := windowStats(kept, r.Peak)
				m.Mapping = mapping
				wh.Metrics[metricName] = m
				if m.State != "OK" {
					wh.State, overall = "PARTIAL", "PARTIAL"
				}
			}
			doc.Hosts = append(doc.Hosts, wh)
		}
	}
	doc.Status = overall
	if len(doc.Hosts) == 0 {
		doc.Status = "UNPROVEN"
	}
	return doc
}
