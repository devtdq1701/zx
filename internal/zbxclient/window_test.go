package zbxclient

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"zx/internal/config"
)

func TestExportWindowContract(t *testing.T) {
	since := time.Date(2026, 9, 24, 3, 0, 0, 0, time.UTC).Unix()
	until := since + 1800
	ts := newRPCServerFunc(t, func(method string, params json.RawMessage) string {
		switch method {
		case "hostinterface.get":
			return `[{"hostid":"10","ip":"10.0.0.1"}]`
		case "host.get":
			if strings.Contains(string(params), "missing-host") {
				return `[]`
			}
			return `[{"hostid":"10","host":"h1","name":"H1 visible"}]`
		case "item.get":
			return `[{"itemid":"1","key_":"system.cpu.util"},{"itemid":"2","key_":"vm.memory.utilization"},{"itemid":"3","key_":"system.cpu.load[all,avg15]"}]`
		case "trend.get":
			return `[{"clock":"` + itoa(since) + `","value_min":"1","value_avg":"2","value_max":"3"},
			         {"clock":"` + itoa(until) + `","value_min":"9","value_avg":"9","value_max":"9"}]`
		default:
			return `[]`
		}
	})
	defer ts.Close()
	c := NewClient(&config.Profile{URL: ts.URL, Token: "t"}, 5*time.Second)
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	f, _ := NewTrendFilter(false, "", false, loc)
	doc := c.ExportWindow(context.Background(), WindowRequest{
		From: "2026-09-24T03:00:00Z", To: "2026-09-24T03:30:00Z", Since: since, Until: until,
		Timezone: "Asia/Ho_Chi_Minh", Role: "app", Targets: []string{"10.0.0.1", "missing-host"},
		Filter: f, AdapterVersion: "test",
	})
	raw, _ := json.Marshal(doc)
	var got map[string]any
	_ = json.Unmarshal(raw, &got)

	if got["schema"] != "zabbix-cli.export_window" || got["version"].(float64) != 1 || got["status"] != "PARTIAL" {
		t.Fatalf("header: %s", raw)
	}
	w := got["window"].(map[string]any)
	if w["since"].(float64) != float64(since) || w["until"].(float64) != float64(until) || w["inclusive"] != "[since,until)" || w["timezone"] != "Asia/Ho_Chi_Minh" {
		t.Fatalf("window: %v", w)
	}
	hosts := got["hosts"].([]any)
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	h := hosts[0].(map[string]any)
	if h["host"] != "H1 visible" || h["hostid"] != "10" || h["ip"] != "10.0.0.1" || h["state"] != "PARTIAL" {
		t.Fatalf("host: %v", h)
	}
	m := h["metrics"].(map[string]any)
	cpu := m["cpu"].(map[string]any)
	// The sample at clock==until must be excluded (half-open window).
	if cpu["state"] != "OK" || cpu["sample_count"].(float64) != 1 || cpu["min"].(float64) != 1 || cpu["avg"].(float64) != 2 || cpu["max"].(float64) != 3 || cpu["peak"].(float64) != 2 {
		t.Fatalf("cpu: %v", cpu)
	}
	io := m["io"].(map[string]any)
	if io["state"] != "UNPROVEN" || io["mapping"] != nil || io["min"] != nil || io["missing_interval"] != true {
		t.Fatalf("io: %v", io)
	}
	errs := got["errors"].([]any)
	if len(errs) != 1 {
		t.Fatalf("errors: %v", errs)
	}
	e0 := errs[0].(map[string]any)
	if e0["host"] != "missing-host" || e0["error"] != "host not found" || e0["state"] != "UNPROVEN" {
		t.Fatalf("error 0: %v", e0)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
