package zbxclient

import (
	"context"
	"encoding/json"
	"testing"
)

func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		v            string
		major, minor int
		want         bool
	}{
		{"5.2.2", 5, 4, false},
		{"5.4.0", 5, 4, true},
		{"6.0.0", 6, 0, true},
		{"6.10.1", 6, 2, true},
		{"7.4.14", 6, 2, true},
		{"", 5, 4, false},
		{"garbage", 6, 0, false},
	}
	for _, c := range cases {
		if got := VersionAtLeast(c.v, c.major, c.minor); got != c.want {
			t.Errorf("VersionAtLeast(%q,%d,%d)=%v want %v", c.v, c.major, c.minor, got, c.want)
		}
	}
}

func TestAPIAtLeastUnknownVersionErrors(t *testing.T) {
	for _, raw := range []string{`""`, `"garbage"`} {
		c := exactClient(t, func(m string, p json.RawMessage) string {
			if m == "apiinfo.version" {
				return raw
			}
			return `[]`
		})
		if _, err := c.APIAtLeast(context.Background(), 6, 0); err == nil {
			t.Fatalf("version %s must be an error, not a guess", raw)
		}
	}
	c := exactClient(t, func(m string, p json.RawMessage) string { return `"7.4.14"` })
	if ok, err := c.APIAtLeast(context.Background(), 6, 0); err != nil || !ok {
		t.Fatalf("7.4.14 >= 6.0: got %v %v", ok, err)
	}
}
