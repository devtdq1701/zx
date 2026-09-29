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
	IPs        []string `json:"-"`
	Hostgroups []string `json:"-"`
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

	params := map[string]any{
		"output":         []string{"hostid", "host", "name", "status"},
		"selectGroups":   []string{"name"},
		"selectInterfaces": []string{"ip", "main"},
	}

	if len(hostIDs) > 0 {
		params["hostids"] = hostIDs
	} else if target != "" {
		if strings.Contains(target, "*") {
			params["search"] = map[string]string{"host": strings.ReplaceAll(target, "*", "")}
			params["searchWildcardsEnabled"] = true
		} else {
			params["search"] = map[string]string{"name": target}
		}
	}

	type rawHost struct {
		HostID     string `json:"hostid"`
		Host       string `json:"host"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Groups     []struct{ Name string `json:"name"` } `json:"groups"`
		Interfaces []struct{ IP string `json:"ip"` } `json:"interfaces"`
	}

	var raw []rawHost
	if err := c.Call(ctx, "host.get", params, &raw); err != nil {
		return nil, fmt.Errorf("fetching hosts: %w", err)
	}

	var res []DetailedHost
	for _, r := range raw {
		var ips []string
		for _, iface := range r.Interfaces {
			if iface.IP != "" {
				ips = append(ips, iface.IP)
			}
		}
		var groups []string
		for _, g := range r.Groups {
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
