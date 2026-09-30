package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"zx/internal/zbxclient"
)

func TestRenderStatsTablePeakUsesLocation(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	peak := &zbxclient.ClusterPeak{Time: time.Unix(1790218800, 0).UTC()} // 2026-09-24 03:00Z = 10:00 +07
	var b bytes.Buffer
	RenderStatsTable(&b, nil, peak, "", loc)
	if !strings.Contains(b.String(), "2026-09-24 10:00") {
		t.Fatalf("peak must be rendered in --timezone:\n%s", b.String())
	}
}

func TestRenderStatsTableMissingShowsDash(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	var b bytes.Buffer
	RenderStatsTable(&b, []zbxclient.HostStats{{HostName: "h1", IP: "10.0.0.1", CPUAvg: 5, CPUMax: 12, Missing: []string{"ram", "load"}}}, nil, "", loc)
	line := ""
	for _, l := range strings.Split(b.String(), "\n") {
		if strings.Contains(l, "h1") {
			line = l
		}
	}
	if strings.Contains(line, "0.00%") || strings.Count(line, " - ") < 2 {
		t.Fatalf("missing metrics must render as '-', got %q", line)
	}
	if !strings.Contains(line, "5.00%") {
		t.Fatalf("present metric lost: %q", line)
	}
}
