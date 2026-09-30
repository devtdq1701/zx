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
