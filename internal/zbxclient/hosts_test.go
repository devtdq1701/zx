package zbxclient

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"zx/internal/config"
)

func TestHostGroupsParam(t *testing.T) {
	cases := map[string][2]string{
		"7.4.14":  {"selectHostGroups", "hostgroups"},
		"6.2.0":   {"selectHostGroups", "hostgroups"},
		"6.0.30":  {"selectGroups", "groups"},
		"5.2.2":   {"selectGroups", "groups"},
		"garbage": {"selectHostGroups", "hostgroups"},
	}
	for v, want := range cases {
		p, f := hostGroupsParam(v)
		if p != want[0] || f != want[1] {
			t.Errorf("%s -> %s,%s want %v", v, p, f, want)
		}
	}
}

func TestDetailedHosts(t *testing.T) {
	ts := newRPCServer(t, map[string]string{
		"apiinfo.version": `"7.4.14"`,
		"host.get":        `[{"hostid":"1","name":"h","status":"0","hostgroups":[{"name":"HNI"}],"interfaces":[{"ip":"10.0.0.1"}]}]`,
	})
	defer ts.Close()
	c := NewClient(&config.Profile{URL: ts.URL, Token: "t"}, 5*time.Second)
	hosts, err := c.GetDetailedHosts(context.Background(), "h")
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || len(hosts[0].Hostgroups) != 1 || hosts[0].Hostgroups[0] != "HNI" {
		t.Fatalf("unexpected hosts: %+v", hosts)
	}
}

func TestGetDetailedHostsJSONArraysNotNull(t *testing.T) {
	ts := newRPCServer(t, map[string]string{
		"apiinfo.version": `"7.4.0"`,
		"host.get":        `[{"hostid":"1","host":"h","name":"h","status":"0"}]`,
	})
	defer ts.Close()
	c := NewClient(&config.Profile{URL: ts.URL, Token: "t"}, 5*time.Second)
	hosts, err := c.GetDetailedHosts(context.Background(), "h")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(hosts)
	if strings.Contains(string(b), "null") {
		t.Fatalf("arrays must be [] not null: %s", b)
	}
	ts2 := newRPCServer(t, map[string]string{"apiinfo.version": `"7.4.0"`, "host.get": `[]`})
	defer ts2.Close()
	c2 := NewClient(&config.Profile{URL: ts2.URL, Token: "t"}, 5*time.Second)
	none, _ := c2.GetDetailedHosts(context.Background(), "zz")
	if b, _ := json.Marshal(none); string(b) != "[]" {
		t.Fatalf("empty result must marshal to [], got %s", b)
	}
}
