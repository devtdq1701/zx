package zbxclient

import (
	"context"
	"testing"
	"time"

	"zx/internal/config"
)

func TestFilterTrendSamples(t *testing.T) {
	// 2026-09-21 was Monday
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")

	// Sample 1: Monday 09:30 (Business hours: inside 08:00-12:00) -> KEEP
	t1 := time.Date(2026, 9, 21, 9, 30, 0, 0, loc).Unix()

	// Sample 2: Monday 12:30 (Lunch break: 12:00-13:00) -> EXCLUDE
	t2 := time.Date(2026, 9, 21, 12, 30, 0, 0, loc).Unix()

	// Sample 3: Monday 15:00 (Business hours: inside 13:00-17:00) -> KEEP
	t3 := time.Date(2026, 9, 21, 15, 0, 0, 0, loc).Unix()

	// Sample 4: Monday 20:00 (Night time) -> EXCLUDE
	t4 := time.Date(2026, 9, 21, 20, 0, 0, 0, loc).Unix()

	// Sample 5: Sunday 10:00 (Weekend) -> EXCLUDE
	t5 := time.Date(2026, 9, 20, 10, 0, 0, 0, loc).Unix()

	samples := []TrendSample{
		{Clock: t1, ValueAvg: 50.0, ValueMax: 70.0},
		{Clock: t2, ValueAvg: 10.0, ValueMax: 15.0},
		{Clock: t3, ValueAvg: 60.0, ValueMax: 80.0},
		{Clock: t4, ValueAvg: 20.0, ValueMax: 30.0},
		{Clock: t5, ValueAvg: 40.0, ValueMax: 50.0},
	}

	f, _ := NewTrendFilter(true, "", false, loc)
	filtered := FilterTrendSamples(samples, f)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(filtered))
	}
	if filtered[0].Clock != t1 || filtered[1].Clock != t3 {
		t.Errorf("unexpected filtered clocks: %v, %v", filtered[0].Clock, filtered[1].Clock)
	}

	// Test calculate metrics
	// Default (peak=false): max is the peak of hourly average (value_avg)
	avg, max := CalculateTrendMetrics(filtered, false)
	if avg != 55.0 { // (50 + 60) / 2
		t.Errorf("expected avg 55.0, got %f", avg)
	}
	if max != 60.0 { // max of 50.0 and 60.0
		t.Errorf("expected max hourly avg 60.0, got %f", max)
	}

	// Instant peak (peak=true): max is the highest value_max
	_, instantMax := CalculateTrendMetrics(filtered, true)
	if instantMax != 80.0 { // max of 70.0 and 80.0
		t.Errorf("expected instant max 80.0, got %f", instantMax)
	}
}

func TestHostStatsSurfacesItemError(t *testing.T) {
	ts := newRPCServer(t, map[string]string{
		"hostinterface.get": `[{"hostid":"10","ip":"10.0.0.1"}]`,
		"host.get":          `[{"hostid":"10","host":"h1","name":"h1"}]`,
		"item.get":          `ERROR`,
	})
	defer ts.Close()
	c := NewClient(&config.Profile{URL: ts.URL, Token: "t"}, 5*time.Second)
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	f, _ := NewTrendFilter(false, "", false, loc)
	stats, _, err := c.GetHostStatsSummary(context.Background(), StatsQuery{
		Targets: []string{"10.0.0.1"}, TimeFrom: 0, TimeTill: 3600, Filter: f, Concurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 || stats[0].Error == "" {
		t.Fatalf("expected per-host error, got %+v", stats)
	}
}
