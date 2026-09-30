package zbxclient

import (
	"fmt"
	"strings"
)

type itemMatcher func(it ItemRecord) bool

// MetricSpec maps a user-facing metric name to Zabbix items.
type MetricSpec struct {
	Name     string
	Label    string
	matchers []itemMatcher
}

// FindItem returns the first item matched by the highest-priority matcher.
func (m MetricSpec) FindItem(items []ItemRecord) (ItemRecord, bool) {
	for _, match := range m.matchers {
		for _, it := range items {
			if match(it) {
				return it, true
			}
		}
	}
	return ItemRecord{}, false
}

func keyIs(keys ...string) itemMatcher {
	return func(it ItemRecord) bool {
		for _, k := range keys {
			if it.Key == k {
				return true
			}
		}
		return false
	}
}

func keyOrParam(keys ...string) itemMatcher {
	return func(it ItemRecord) bool {
		for _, k := range keys {
			if it.Key == k || strings.HasPrefix(it.Key, k+"[") {
				return true
			}
		}
		return false
	}
}

func keyContainsAll(parts ...string) itemMatcher {
	return func(it ItemRecord) bool {
		for _, p := range parts {
			if !strings.Contains(it.Key, p) {
				return false
			}
		}
		return true
	}
}

func nameContains(sub string) itemMatcher {
	return func(it ItemRecord) bool { return strings.Contains(strings.ToLower(it.Name), sub) }
}

var metricSpecs = []MetricSpec{
	{"cpu", "CPU", []itemMatcher{keyIs("system.cpu.util"), nameContains("cpu utilization")}},
	{"ram", "RAM", []itemMatcher{keyIs("vm.memory.utilization", "vm.memory.util", "vm.memory.size[pused]"), nameContains("memory utilization")}},
	{"load", "LOAD", []itemMatcher{keyIs("system.cpu.load[all,avg15]", "system.cpu.load[percpu,avg15]"), keyContainsAll("system.cpu.load", "15")}},
	{"free_swap", "FREE SWAP", []itemMatcher{keyIs("system.swap.size[,pfree]"), nameContains("free swap space in %")}},
	{"iowait", "CPU IOWAIT", []itemMatcher{keyIs("system.cpu.util[,iowait]"), nameContains("iowait")}},
	{"io", "IO", []itemMatcher{keyOrParam("vfs.dev.util", "vfs.dev.read", "vfs.dev.write")}},
}

var metricAliases = map[string]string{
	"memory": "ram", "free-swap": "free_swap", "swap": "free_swap", "swap_free": "free_swap",
	"cpu_iowait": "iowait", "cpu-iowait": "iowait",
}

// GraphMetrics is what "all" expands to for graphs and stats.
var GraphMetrics = []string{"cpu", "ram", "load", "free_swap", "iowait"}

// WindowMetrics is the fixed export_window metric set (skill contract).
var WindowMetrics = []string{"cpu", "ram", "load", "io"}

// LookupMetric resolves a metric name or alias (case-insensitive).
func LookupMetric(name string) (MetricSpec, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	if a, ok := metricAliases[n]; ok {
		n = a
	}
	for _, m := range metricSpecs {
		if m.Name == n {
			return m, true
		}
	}
	return MetricSpec{}, false
}

// ParseMetricList parses "all" or a comma-separated metric list.
func ParseMetricList(s string) ([]MetricSpec, error) {
	names := strings.Split(s, ",")
	if strings.EqualFold(strings.TrimSpace(s), "all") {
		names = GraphMetrics
	}
	var out []MetricSpec
	seen := map[string]bool{}
	for _, raw := range names {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		m, ok := LookupMetric(raw)
		if !ok {
			return nil, fmt.Errorf("invalid metric '%s'; expected ram, cpu, load, free_swap, iowait, or all", strings.TrimSpace(raw))
		}
		if !seen[m.Name] {
			seen[m.Name] = true
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no metric specified")
	}
	return out, nil
}
