package zbxclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// CreateItemParams defines parameters for creating a new item.
type CreateItemParams struct {
	HostID    string
	Name      string
	Key       string
	Type      int
	ValueType int
	Delay     string
	Units     string
	Status    int
}

// ParseItemType parses an item type name or integer string into Zabbix item type ID.
func ParseItemType(s string) (int, error) {
	norm := strings.ToLower(strings.TrimSpace(s))
	switch norm {
	case "0", "zabbix_agent", "agent":
		return 0, nil
	case "2", "trapper", "zabbix_trapper":
		return 2, nil
	case "3", "simple", "simple_check":
		return 3, nil
	case "7", "zabbix_active", "active":
		return 7, nil
	case "15", "calculated":
		return 15, nil
	default:
		return 0, fmt.Errorf("invalid item type %q (supported: zabbix_active(7), zabbix_agent(0), trapper(2), simple(3), calculated(15))", s)
	}
}

// ParseValueType parses a value type name or integer string into Zabbix value type ID.
func ParseValueType(s string) (int, error) {
	norm := strings.ToLower(strings.TrimSpace(s))
	switch norm {
	case "0", "numeric_float", "float":
		return 0, nil
	case "1", "char", "character":
		return 1, nil
	case "2", "log":
		return 2, nil
	case "3", "numeric_unsigned", "unsigned", "uint":
		return 3, nil
	case "4", "text":
		return 4, nil
	default:
		return 0, fmt.Errorf("invalid value type %q (supported: numeric_unsigned(3), numeric_float(0), char(1), log(2), text(4))", s)
	}
}

// BuildCreateItemPayload builds the JSON-RPC params map for item.create.
func BuildCreateItemPayload(p CreateItemParams) (map[string]any, error) {
	if strings.TrimSpace(p.HostID) == "" {
		return nil, errors.New("hostid is required")
	}
	if strings.TrimSpace(p.Name) == "" {
		return nil, errors.New("item name is required")
	}
	if strings.TrimSpace(p.Key) == "" {
		return nil, errors.New("item key is required")
	}

	payload := map[string]any{
		"hostid":     p.HostID,
		"name":       p.Name,
		"key_":       p.Key,
		"type":       p.Type,
		"value_type": p.ValueType,
		"delay":      p.Delay,
		"status":     p.Status,
	}

	if p.Units != "" {
		payload["units"] = p.Units
	}

	return payload, nil
}

// CreateItem creates a new item in Zabbix and returns its created item ID.
func (c *Client) CreateItem(ctx context.Context, p CreateItemParams) (string, error) {
	payload, err := BuildCreateItemPayload(p)
	if err != nil {
		return "", err
	}

	var res struct {
		ItemIDs []string `json:"itemids"`
	}
	if err := c.Call(ctx, "item.create", payload, &res); err != nil {
		return "", fmt.Errorf("item.create: %w", err)
	}
	if len(res.ItemIDs) == 0 {
		return "", errors.New("item.create returned no itemids")
	}
	return res.ItemIDs[0], nil
}

// UpdateItem updates an existing item by ID.
func (c *Client) UpdateItem(ctx context.Context, itemID string, updates map[string]any) error {
	if strings.TrimSpace(itemID) == "" {
		return errors.New("itemid is required")
	}
	if len(updates) == 0 {
		return errors.New("no updates provided")
	}

	payload := make(map[string]any, len(updates)+1)
	for k, v := range updates {
		payload[k] = v
	}
	payload["itemid"] = itemID
	if k, ok := payload["key"]; ok {
		payload["key_"] = k
		delete(payload, "key")
	}

	var res struct {
		ItemIDs []string `json:"itemids"`
	}
	if err := c.Call(ctx, "item.update", payload, &res); err != nil {
		return fmt.Errorf("item.update: %w", err)
	}
	return nil
}
