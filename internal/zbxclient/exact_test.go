package zbxclient

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"zx/internal/config"
)

func exactClient(t *testing.T, handler func(method string, params json.RawMessage) string) *Client {
	t.Helper()
	ts := newRPCServerFunc(t, handler)
	t.Cleanup(ts.Close)
	return NewClient(&config.Profile{URL: ts.URL, Token: "t"}, 5*time.Second)
}

func TestResolveExactHostRejectsPattern(t *testing.T) {
	// A server that ignores the filter and returns partial matches.
	c := exactClient(t, func(m string, p json.RawMessage) string {
		if m == "host.get" {
			return `[{"hostid":"111","host":"eofhni1","name":"eofhni1"},{"hostid":"222","host":"eofhni10","name":"eofhni10"}]`
		}
		return `[]`
	})
	if _, err := c.ResolveExactHost(context.Background(), "eofhni"); err == nil || !strings.Contains(err.Error(), "host not found: eofhni") {
		t.Fatalf("pattern must not resolve, got %v", err)
	}
	h, err := c.ResolveExactHost(context.Background(), "eofhni1")
	if err != nil || h.HostID != "111" {
		t.Fatalf("exact name must resolve to 111, got %+v %v", h, err)
	}
}

func TestResolveExactHostFallsBackToVisibleName(t *testing.T) {
	c := exactClient(t, func(m string, p json.RawMessage) string {
		if m != "host.get" {
			return `[]`
		}
		if strings.Contains(string(p), `"name":"App 01"`) {
			return `[{"hostid":"7","host":"app01","name":"App 01"}]`
		}
		return `[]`
	})
	h, err := c.ResolveExactHost(context.Background(), "App 01")
	if err != nil || h.HostID != "7" {
		t.Fatalf("got %+v %v", h, err)
	}
}

func TestResolveExactHostAmbiguousIP(t *testing.T) {
	c := exactClient(t, func(m string, p json.RawMessage) string {
		switch m {
		case "hostinterface.get":
			return `[{"hostid":"1","ip":"10.0.0.9"},{"hostid":"2","ip":"10.0.0.9"}]`
		case "host.get":
			return `[{"hostid":"1","host":"a","name":"a"},{"hostid":"2","host":"b","name":"b"}]`
		}
		return `[]`
	})
	_, err := c.ResolveExactHost(context.Background(), "10.0.0.9")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous error, got %v", err)
	}
}

func TestResolveExactGroupVerifiesNumericID(t *testing.T) {
	c := exactClient(t, func(m string, p json.RawMessage) string { return `[]` })
	if _, err := c.ResolveExactGroup(context.Background(), "999"); err == nil || !strings.Contains(err.Error(), "hostgroup not found: 999") {
		t.Fatalf("unknown numeric group must fail, got %v", err)
	}
}

func TestResolveExactTemplateByName(t *testing.T) {
	c := exactClient(t, func(m string, p json.RawMessage) string {
		if m == "template.get" && strings.Contains(string(p), `"host":"Template OS Linux"`) {
			return `[{"templateid":"10001","host":"Template OS Linux","name":"Template OS Linux"}]`
		}
		return `[]`
	})
	tp, err := c.ResolveExactTemplate(context.Background(), "Template OS Linux")
	if err != nil || tp.TemplateID != "10001" {
		t.Fatalf("got %+v %v", tp, err)
	}
}

func TestResolveExactGroupNumericName(t *testing.T) {
	cases := []struct {
		name, byID, byName, want string
	}{
		{"ambiguous", `[{"groupid":"2024","name":"Linux"}]`, `[{"groupid":"55","name":"2024"}]`, ""},
		{"only id", `[{"groupid":"2024","name":"Linux"}]`, `[]`, "2024"},
		{"only name", `[]`, `[{"groupid":"55","name":"2024"}]`, "55"},
	}
	for _, tc := range cases {
		c := exactClient(t, func(m string, p json.RawMessage) string {
			if m != "hostgroup.get" {
				return `[]`
			}
			if strings.Contains(string(p), `"groupids"`) {
				return tc.byID
			}
			return tc.byName
		})
		g, err := c.ResolveExactGroup(context.Background(), "2024")
		if tc.want == "" {
			if err == nil || !strings.Contains(err.Error(), "ambiguous") {
				t.Fatalf("%s: want ambiguous, got %+v %v", tc.name, g, err)
			}
			continue
		}
		if err != nil || g.GroupID != tc.want {
			t.Fatalf("%s: got %+v %v", tc.name, g, err)
		}
	}
}

func TestResolveExactTemplateNumericName(t *testing.T) {
	cases := []struct {
		name, byID, byName, want string
	}{
		{"ambiguous", `[{"templateid":"10050","host":"Linux","name":"Linux"}]`, `[{"templateid":"77","host":"10050","name":"10050"}]`, ""},
		{"only id", `[{"templateid":"10050","host":"Linux","name":"Linux"}]`, `[]`, "10050"},
		{"only name", `[]`, `[{"templateid":"77","host":"10050","name":"10050"}]`, "77"},
	}
	for _, tc := range cases {
		c := exactClient(t, func(m string, p json.RawMessage) string {
			if m != "template.get" {
				return `[]`
			}
			if strings.Contains(string(p), `"templateids"`) {
				return tc.byID
			}
			return tc.byName
		})
		tp, err := c.ResolveExactTemplate(context.Background(), "10050")
		if tc.want == "" {
			if err == nil || !strings.Contains(err.Error(), "ambiguous") {
				t.Fatalf("%s: want ambiguous, got %+v %v", tc.name, tp, err)
			}
			continue
		}
		if err != nil || tp.TemplateID != tc.want {
			t.Fatalf("%s: got %+v %v", tc.name, tp, err)
		}
	}
}

func TestResolveExactHostTechnicalVisibleCollision(t *testing.T) {
	c := exactClient(t, func(m string, p json.RawMessage) string {
		if m != "host.get" {
			return `[]`
		}
		if strings.Contains(string(p), `"host":"web1"`) {
			return `[{"hostid":"1","host":"web1","name":"Web One"}]`
		}
		return `[{"hostid":"2","host":"web-b","name":"web1"}]`
	})
	if _, err := c.ResolveExactHost(context.Background(), "web1"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("technical vs visible name collision must be ambiguous, got %v", err)
	}
}

func TestResolveExactHostSameHostBothNames(t *testing.T) {
	c := exactClient(t, func(m string, p json.RawMessage) string {
		if m == "host.get" {
			return `[{"hostid":"1","host":"web1","name":"web1"}]`
		}
		return `[]`
	})
	h, err := c.ResolveExactHost(context.Background(), "web1")
	if err != nil || h.HostID != "1" {
		t.Fatalf("one host matching both fields must resolve, got %+v %v", h, err)
	}
}

func TestResolveExactMaintenance(t *testing.T) {
	cases := []struct {
		name, target, byID, byName, want, errPart string
	}{
		{"by name", "patch", `[]`, `[{"maintenanceid":"5","name":"patch"}]`, "5", ""},
		{"filter ignored", "patch", `[]`, `[{"maintenanceid":"5","name":"patch-old"},{"maintenanceid":"6","name":"patch"}]`, "6", ""},
		{"none", "zz", `[]`, `[{"maintenanceid":"5","name":"other"}]`, "", "not found"},
		{"duplicate names", "patch", `[]`, `[{"maintenanceid":"5","name":"patch"},{"maintenanceid":"6","name":"patch"}]`, "", "ambiguous"},
		{"only id", "31", `[{"maintenanceid":"31","name":"nightly"}]`, `[]`, "31", ""},
		{"id vs name", "31", `[{"maintenanceid":"31","name":"nightly"}]`, `[{"maintenanceid":"8","name":"31"}]`, "", "ambiguous"},
	}
	for _, tc := range cases {
		c := exactClient(t, func(m string, p json.RawMessage) string {
			if m != "maintenance.get" {
				return `[]`
			}
			if strings.Contains(string(p), `"maintenanceids"`) {
				return tc.byID
			}
			return tc.byName
		})
		got, err := c.ResolveExactMaintenance(context.Background(), tc.target)
		if tc.errPart != "" {
			if err == nil || !strings.Contains(err.Error(), tc.errPart) {
				t.Fatalf("%s: want %q error, got %+v %v", tc.name, tc.errPart, got, err)
			}
			continue
		}
		if err != nil || got.MaintenanceID != tc.want {
			t.Fatalf("%s: got %+v %v", tc.name, got, err)
		}
	}
}
