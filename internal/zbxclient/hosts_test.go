package zbxclient

import (
	"context"
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
