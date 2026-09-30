package zbxclient

import (
	"testing"
	"time"
)

func TestTrendFilterTimezone(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	f, err := NewTrendFilter(true, "", false, loc)
	if err != nil {
		t.Fatal(err)
	}
	// 2026-09-21 is a Monday. 02:30 UTC == 09:30 +07 -> keep, regardless of time.Local.
	if !f.Keep(time.Date(2026, 9, 21, 2, 30, 0, 0, time.UTC).Unix()) {
		t.Error("09:30 +07 Monday should be kept")
	}
	// 05:30 UTC == 12:30 +07 lunch -> drop
	if f.Keep(time.Date(2026, 9, 21, 5, 30, 0, 0, time.UTC).Unix()) {
		t.Error("12:30 +07 should be dropped")
	}
	// Sunday 10:00 +07 -> drop
	if f.Keep(time.Date(2026, 9, 20, 10, 0, 0, 0, loc).Unix()) {
		t.Error("Sunday should be dropped")
	}
}

func TestNewTrendFilterHours(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	f, err := NewTrendFilter(false, "20-24", false, loc)
	if err != nil {
		t.Fatal(err)
	}
	sun := time.Date(2026, 9, 20, 21, 0, 0, 0, loc).Unix()
	if !f.Keep(sun) {
		t.Error("--hours without --weekdays-only must keep Sunday 21:00")
	}
	f, _ = NewTrendFilter(false, "20-24", true, loc)
	if f.Keep(sun) {
		t.Error("--weekdays-only must drop Sunday")
	}
	none, _ := NewTrendFilter(false, "", false, loc)
	if !none.Keep(sun) {
		t.Error("empty filter keeps everything")
	}
	for _, bad := range []string{"8", "a-b", "12-8", "0-25", ","} {
		if _, err := ParseHourRanges(bad); err == nil {
			t.Errorf("ParseHourRanges(%q) expected error", bad)
		}
	}
	r, err := ParseHourRanges("8-12, 13-17")
	if err != nil || len(r) != 2 || r[1] != (HourRange{13, 17}) {
		t.Fatalf("ranges: %+v %v", r, err)
	}
}
