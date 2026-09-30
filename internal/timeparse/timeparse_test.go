package timeparse

import (
	"testing"
	"time"
)

func mustLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestParse(t *testing.T) {
	loc := mustLoc(t)
	now := time.Date(2026, 9, 30, 16, 0, 0, 0, loc)
	cases := []struct {
		in     string
		isEnd  bool
		unix   int64
		zabbix string
	}{
		{"now", false, now.Unix(), "now"},
		{"now-1h", false, now.Add(-time.Hour).Unix(), "now-1h"},
		{"now-2H", false, now.Add(-2 * time.Hour).Unix(), "now-2h"},
		{"now-7d", false, now.Add(-7 * 24 * time.Hour).Unix(), "now-7d"},
		{"now-30m", false, now.Add(-30 * time.Minute).Unix(), "now-30m"},
		{"now-1M", false, now.Add(-30 * 24 * time.Hour).Unix(), "now-1M"},
		{"yesterday 17:00", false, time.Date(2026, 9, 29, 17, 0, 0, 0, loc).Unix(), "2026-09-29 17:00:00"},
		{"Yesterday", false, time.Date(2026, 9, 29, 0, 0, 0, 0, loc).Unix(), "2026-09-29 00:00:00"},
		{"yesterday", true, time.Date(2026, 9, 29, 23, 59, 59, 0, loc).Unix(), "2026-09-29 23:59:59"},
		{"2026-09-24 10:00", false, time.Date(2026, 9, 24, 10, 0, 0, 0, loc).Unix(), "2026-09-24 10:00:00"},
		{"24/09/2026 10:30:15", false, time.Date(2026, 9, 24, 10, 30, 15, 0, loc).Unix(), "2026-09-24 10:30:15"},
		{"2026-09-24", true, time.Date(2026, 9, 24, 23, 59, 59, 0, loc).Unix(), "2026-09-24 23:59:59"},
		{"2026-09-24T03:00:00Z", false, time.Date(2026, 9, 24, 10, 0, 0, 0, loc).Unix(), "2026-09-24 10:00:00"},
	}
	for _, c := range cases {
		got, err := Parse(c.in, c.isEnd, now, loc)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.in, err)
		}
		if got.Unix != c.unix || got.Zabbix != c.zabbix {
			t.Errorf("Parse(%q)=%+v, want unix=%d zabbix=%q", c.in, got, c.unix, c.zabbix)
		}
	}
	for _, bad := range []string{"", "tomorrow", "now-", "2026-13-01", "yesterday 25:00"} {
		if _, err := Parse(bad, false, now, loc); err == nil {
			t.Errorf("Parse(%q) expected error", bad)
		}
	}
}

func TestParseRange(t *testing.T) {
	loc := mustLoc(t)
	now := time.Date(2026, 9, 30, 16, 0, 0, 0, loc)
	s, e, err := ParseRange("yesterday 17:00..yesterday 18:00", "", "", 30, now, loc)
	if err != nil || s.Zabbix != "2026-09-29 17:00:00" || e.Zabbix != "2026-09-29 18:00:00" {
		t.Fatalf("range: %+v %+v %v", s, e, err)
	}
	s, e, err = ParseRange("", "", "", 7, now, loc)
	if err != nil || s.Zabbix != "now-7d" || e.Zabbix != "now" {
		t.Fatalf("default: %+v %+v %v", s, e, err)
	}
	s, _, err = ParseRange("now-1h", "", "", 7, now, loc)
	if err != nil || s.Zabbix != "now-1h" {
		t.Fatalf("single: %+v %v", s, err)
	}
	if _, _, err = ParseRange("now..now-1h", "", "", 7, now, loc); err == nil {
		t.Fatal("expected start>=end error")
	}
}
