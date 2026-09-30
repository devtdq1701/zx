package cli

import (
	"testing"
	"time"
)

func TestParseWindowTime(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	want := time.Date(2026, 9, 24, 3, 0, 0, 0, time.UTC).Unix()
	for _, in := range []string{"2026-09-24T03:00:00Z", "2026-09-24T10:00:00+07:00", "2026-09-24 10:00:00", "2026-09-24 10:00"} {
		got, err := parseWindowTime(in, loc, time.Now())
		if err != nil || got != want {
			t.Errorf("%q -> %d %v, want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "yesterday", "2026-09-24\x00"} {
		if _, err := parseWindowTime(bad, loc, time.Now()); err == nil {
			t.Errorf("%q expected error", bad)
		}
	}
}

func TestExportWindowRejectsBadRole(t *testing.T) {
	err := validateWindowArgs("web", 10, 20)
	if err == nil {
		t.Fatal("expected role error")
	}
	if validateWindowArgs("db", 20, 20) == nil {
		t.Fatal("expected from<to error")
	}
	if validateWindowArgs("app", 10, 20) != nil {
		t.Fatal("valid args rejected")
	}
}
