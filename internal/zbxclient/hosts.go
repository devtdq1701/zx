package zbxclient

import (
	"context"
	"fmt"
	"net"
	"strings"
)

type DetailedHost struct {
	HostID     string   `json:"hostid"`
	Host       string   `json:"host"`
	Name       string   `json:"name"`
	Status     string   `json:"status"` // "0" = Monitored, "1" = Unmonitored
	IPs        []string `json:"ips"`
	Hostgroups []string `json:"hostgroups"`
}

func hostGroupsParam(version string) (string, string) {
	var major, minor int
	if _, err := fmt.Sscanf(version, "%d.%d", &major, &minor); err != nil {
		return "selectHostGroups", "hostgroups"
	}
	if major > 6 || (major == 6 && minor >= 2) {
		return "selectHostGroups", "hostgroups"
	}
	return "selectGroups", "groups"
}

func (c *Client) GetDetailedHosts(ctx context.Context, target string) ([]DetailedHost, error) {
	var hostIDs []string

	target = strings.TrimSpace(target)
	if target != "" && net.ParseIP(target) != nil {
		var ifaces []HostInterfaceRecord
		err := c.Call(ctx, "hostinterface.get", map[string]any{
			"filter": map[string]string{"ip": target},
			"output": []string{"hostid", "ip"},
		}, &ifaces)
		if err == nil {
			for _, iface := range ifaces {
				hostIDs = append(hostIDs, iface.HostID)
			}
		}
	}

	v, _ := c.APIVersion(ctx)
	param, field := hostGroupsParam(v)

	params := map[string]any{
		"output":           []string{"hostid", "host", "name", "status"},
		param:              []string{"name"},
		"selectInterfaces": []string{"ip", "main"},
	}

	if len(hostIDs) > 0 {
		params["hostids"] = hostIDs
	} else if target != "" {
		params["search"] = map[string]string{
			"host": target,
			"name": target,
		}
		params["searchWildcardsEnabled"] = true
		params["searchByAny"] = true
	}

	type rawHost struct {
		HostID string `json:"hostid"`
		Host   string `json:"host"`
		Name   string `json:"name"`
		Status string `json:"status"`
		Groups []struct {
			Name string `json:"name"`
		} `json:"groups"`
		HostGroups []struct {
			Name string `json:"name"`
		} `json:"hostgroups"`
		Interfaces []struct {
			IP string `json:"ip"`
		} `json:"interfaces"`
	}

	var raw []rawHost
	if err := c.Call(ctx, "host.get", params, &raw); err != nil {
		return nil, fmt.Errorf("fetching hosts: %w", err)
	}

	res := []DetailedHost{}
	for _, r := range raw {
		ips := []string{}
		for _, iface := range r.Interfaces {
			if iface.IP != "" {
				ips = append(ips, iface.IP)
			}
		}
		groups := []string{}
		src := r.Groups
		if field == "hostgroups" && len(r.HostGroups) > 0 {
			src = r.HostGroups
		} else if len(src) == 0 {
			src = r.HostGroups
		}
		for _, g := range src {
			if g.Name != "" {
				groups = append(groups, g.Name)
			}
		}

		res = append(res, DetailedHost{
			HostID:     r.HostID,
			Host:       r.Host,
			Name:       r.Name,
			Status:     r.Status,
			IPs:        ips,
			Hostgroups: groups,
		})
	}

	return res, nil
}
