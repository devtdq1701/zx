package zbxclient

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
)

func isNumericID(s string) bool {
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}

// exactlyOne returns the only element of items, or an error naming the
// candidates, so write commands never act on an arbitrary first match.
func exactlyOne[T any](kind, target string, items []T, label func(T) string) (T, error) {
	var zero T
	switch len(items) {
	case 0:
		return zero, fmt.Errorf("%s not found: %s", kind, target)
	case 1:
		return items[0], nil
	}
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, label(it))
	}
	sort.Strings(names)
	return zero, fmt.Errorf("%s '%s' is ambiguous (%d matches: %s); use the exact name or ID",
		kind, target, len(items), strings.Join(names, ", "))
}

// ResolveExactHost resolves target by exact IP, technical name or visible
// name. Results are re-checked locally because a server that ignores an
// unknown filter returns every host.
func (c *Client) ResolveExactHost(ctx context.Context, target string) (HostRecord, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return HostRecord{}, fmt.Errorf("empty host target")
	}
	output := []string{"hostid", "host", "name"}
	var hosts []HostRecord
	if net.ParseIP(target) != nil {
		var ifaces []HostInterfaceRecord
		if err := c.Call(ctx, "hostinterface.get", map[string]any{
			"filter": map[string]string{"ip": target},
			"output": []string{"hostid", "ip"},
		}, &ifaces); err != nil {
			return HostRecord{}, fmt.Errorf("hostinterface.get: %w", err)
		}
		seen := map[string]bool{}
		var ids []string
		for _, i := range ifaces {
			if i.IP == target && !seen[i.HostID] {
				seen[i.HostID] = true
				ids = append(ids, i.HostID)
			}
		}
		if len(ids) > 0 {
			if err := c.Call(ctx, "host.get", map[string]any{"hostids": ids, "output": output}, &hosts); err != nil {
				return HostRecord{}, fmt.Errorf("host.get: %w", err)
			}
		}
	} else {
		for _, field := range []string{"host", "name"} {
			var found []HostRecord
			if err := c.Call(ctx, "host.get", map[string]any{
				"filter": map[string]string{field: target},
				"output": output,
			}, &found); err != nil {
				return HostRecord{}, fmt.Errorf("host.get: %w", err)
			}
			for _, h := range found {
				if (field == "host" && h.Host == target) || (field == "name" && h.Name == target) {
					hosts = append(hosts, h)
				}
			}
			if len(hosts) > 0 {
				break
			}
		}
	}
	return exactlyOne("host", target, hosts, func(h HostRecord) string { return h.Host + " (" + h.HostID + ")" })
}

type GroupRecord struct {
	GroupID string `json:"groupid"`
	Name    string `json:"name"`
}

// ResolveExactGroup resolves a host group by exact name or existing ID.
func (c *Client) ResolveExactGroup(ctx context.Context, nameOrID string) (GroupRecord, error) {
	nameOrID = strings.TrimSpace(nameOrID)
	params := map[string]any{"output": []string{"groupid", "name"}}
	if isNumericID(nameOrID) {
		params["groupids"] = []string{nameOrID}
	} else {
		params["filter"] = map[string]string{"name": nameOrID}
	}
	var found []GroupRecord
	if err := c.Call(ctx, "hostgroup.get", params, &found); err != nil {
		return GroupRecord{}, fmt.Errorf("hostgroup.get: %w", err)
	}
	var exact []GroupRecord
	for _, g := range found {
		if g.GroupID == nameOrID || g.Name == nameOrID {
			exact = append(exact, g)
		}
	}
	return exactlyOne("hostgroup", nameOrID, exact, func(g GroupRecord) string { return g.Name + " (" + g.GroupID + ")" })
}

type TemplateRecord struct {
	TemplateID string `json:"templateid"`
	Host       string `json:"host"`
	Name       string `json:"name"`
}

// ResolveExactTemplate resolves a template by existing ID, technical name
// or visible name.
func (c *Client) ResolveExactTemplate(ctx context.Context, nameOrID string) (TemplateRecord, error) {
	nameOrID = strings.TrimSpace(nameOrID)
	output := []string{"templateid", "host", "name"}
	var exact []TemplateRecord
	if isNumericID(nameOrID) {
		var found []TemplateRecord
		if err := c.Call(ctx, "template.get", map[string]any{"templateids": []string{nameOrID}, "output": output}, &found); err != nil {
			return TemplateRecord{}, fmt.Errorf("template.get: %w", err)
		}
		for _, tp := range found {
			if tp.TemplateID == nameOrID {
				exact = append(exact, tp)
			}
		}
	} else {
		for _, field := range []string{"host", "name"} {
			var found []TemplateRecord
			if err := c.Call(ctx, "template.get", map[string]any{
				"filter": map[string]string{field: nameOrID},
				"output": output,
			}, &found); err != nil {
				return TemplateRecord{}, fmt.Errorf("template.get: %w", err)
			}
			for _, tp := range found {
				if (field == "host" && tp.Host == nameOrID) || (field == "name" && tp.Name == nameOrID) {
					exact = append(exact, tp)
				}
			}
			if len(exact) > 0 {
				break
			}
		}
	}
	return exactlyOne("template", nameOrID, exact, func(tp TemplateRecord) string { return tp.Host + " (" + tp.TemplateID + ")" })
}
