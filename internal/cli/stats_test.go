package cli

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"zx/internal/timeparse"
	"zx/internal/zbxclient"
)

func TestStatsJSONMissingMetricsAreNull(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	f, _ := zbxclient.NewTrendFilter(false, "", false, loc)
	stats := []zbxclient.HostStats{{HostID: "1", HostName: "h1", CPUAvg: 5, CPUMax: 9, Missing: []string{"ram", "load"}}}
	var buf bytes.Buffer
	if err := writeJSON(&buf, statsJSON(stats, nil, timeparse.Point{}, timeparse.Point{}, loc, f, false)); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Hosts       []map[string]any `json:"hosts"`
		ClusterPeak any              `json:"cluster_peak"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	h := got.Hosts[0]
	for _, k := range []string{"ram_avg", "ram_max", "load_avg", "load_max"} {
		if v, ok := h[k]; !ok || v != nil {
			t.Fatalf("%s must be null for a missing metric, got %v (present=%v)", k, v, ok)
		}
	}
	if h["cpu_avg"] != float64(5) || h["cpu_max"] != float64(9) {
		t.Fatalf("present metric changed: %v", h)
	}
	if got.ClusterPeak != nil {
		t.Fatalf("nil peak must be null, got %v", got.ClusterPeak)
	}
}
