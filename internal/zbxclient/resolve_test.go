package zbxclient

import (
	"context"
	"testing"
	"time"

	"zx/internal/config"
)

func TestResolveHostsByHostgroupOnly(t *testing.T) {
	ts := newRPCServer(t, map[string]string{
		"hostgroup.get":     `[{"groupid":"7","name":"HNI App"}]`,
		"host.get":          `[{"hostid":"10","host":"a","name":"a"},{"hostid":"11","host":"b","name":"b"}]`,
		"hostinterface.get": `[{"hostid":"10","ip":"10.0.0.1"},{"hostid":"11","ip":"10.0.0.2"}]`,
	})
	defer ts.Close()
	c := NewClient(&config.Profile{URL: ts.URL, Token: "t"}, 5*time.Second)
	hosts, ips, err := c.ResolveHostsFiltered(context.Background(), nil, []string{"HNI App"})
	if err != nil || len(hosts) != 2 || ips["11"] != "10.0.0.2" {
		t.Fatalf("got %+v %+v %v", hosts, ips, err)
	}
}

func TestResolveHostsUnknownHostgroup(t *testing.T) {
	ts := newRPCServer(t, map[string]string{"hostgroup.get": `[]`})
	defer ts.Close()
	c := NewClient(&config.Profile{URL: ts.URL, Token: "t"}, 5*time.Second)
	if _, _, err := c.ResolveHostsFiltered(context.Background(), nil, []string{"nope"}); err == nil {
		t.Fatal("expected hostgroup not found error")
	}
}
