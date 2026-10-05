package zbxclient

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// TriggerDependency represents a dependency reference in a trigger.
type TriggerDependency struct {
	TriggerID   string `json:"triggerid"`
	Description string `json:"description,omitempty"`
}

// TriggerRecord represents a Zabbix trigger object.
type TriggerRecord struct {
	TriggerID    string              `json:"triggerid"`
	Description  string              `json:"description"`
	Expression   string              `json:"expression,omitempty"`
	Priority     string              `json:"priority"`
	Status       string              `json:"status"` // "0" = active/enabled, "1" = disabled
	Value        string              `json:"value,omitempty"`
	LastChange   string              `json:"lastchange,omitempty"`
	Dependencies []TriggerDependency `json:"dependencies,omitempty"`
}

// TriggerFilter specifies filters for querying triggers.
type TriggerFilter struct {
	HostID   string
	Priority *int
	Status   *int
}

// CreateTriggerParams contains inputs required to create a trigger.
type CreateTriggerParams struct {
	Description string
	Expression  string
	Priority    int    // 0..5
	Status      int    // 0 = active/enabled, 1 = disabled
	DependsOn   string // optional trigger ID
}

// ParsePriority parses a severity name or integer string into a 0-5 integer.
func ParsePriority(s string) (int, error) {
	norm := strings.ToLower(strings.TrimSpace(s))
	switch norm {
	case "0", "not_classified", "not classified", "none":
		return 0, nil
	case "1", "information", "info":
		return 1, nil
	case "2", "warning", "warn":
		return 2, nil
	case "3", "average", "avg":
		return 3, nil
	case "4", "high":
		return 4, nil
	case "5", "disaster", "crit", "critical":
		return 5, nil
	}
	return 0, fmt.Errorf("invalid priority %q (expected: not_classified(0), info(1), warning(2), average(3), high(4), disaster(5))", s)
}

// FormatPriority returns a friendly severity name from an int or numeric string.
func FormatPriority(p any) string {
	var s string
	switch v := p.(type) {
	case int:
		s = strconv.Itoa(v)
	case string:
		s = strings.TrimSpace(v)
	default:
		s = fmt.Sprint(v)
	}
	switch s {
	case "0":
		return "Not classified"
	case "1":
		return "Information"
	case "2":
		return "Warning"
	case "3":
		return "Average"
	case "4":
		return "High"
	case "5":
		return "Disaster"
	default:
		return s
	}
}

// BuildCreateTriggerPayload prepares the JSON-RPC params map for trigger.create.
func BuildCreateTriggerPayload(p CreateTriggerParams) (map[string]any, error) {
	if strings.TrimSpace(p.Description) == "" {
		return nil, errors.New("trigger description is required")
	}
	if strings.TrimSpace(p.Expression) == "" {
		return nil, errors.New("trigger expression is required")
	}
	if p.Priority < 0 || p.Priority > 5 {
		return nil, fmt.Errorf("invalid priority %d (must be 0..5)", p.Priority)
	}

	payload := map[string]any{
		"description": p.Description,
		"expression":  p.Expression,
		"priority":    p.Priority,
		"status":      p.Status,
	}

	if dep := strings.TrimSpace(p.DependsOn); dep != "" {
		payload["dependencies"] = []map[string]string{
			{"triggerid": dep},
		}
	}

	return payload, nil
}

// GetTriggers retrieves triggers matching the specified filter.
func (c *Client) GetTriggers(ctx context.Context, filter TriggerFilter) ([]TriggerRecord, error) {
	params := map[string]any{
		"output":             []string{"triggerid", "description", "expression", "priority", "status", "value", "lastchange"},
		"selectDependencies": "extend",
		"expandExpression":   true,
		"sortfield":          "priority",
		"sortorder":          "DESC",
	}

	if filter.HostID != "" {
		params["hostids"] = []string{filter.HostID}
	}

	filterObj := map[string]any{}
	if filter.Priority != nil {
		filterObj["priority"] = *filter.Priority
	}
	if filter.Status != nil {
		filterObj["status"] = *filter.Status
	}
	if len(filterObj) > 0 {
		params["filter"] = filterObj
	}

	var triggers []TriggerRecord
	if err := c.Call(ctx, "trigger.get", params, &triggers); err != nil {
		return nil, fmt.Errorf("trigger.get: %w", err)
	}
	return triggers, nil
}

// GetTrigger fetches a single trigger by ID.
func (c *Client) GetTrigger(ctx context.Context, triggerID string) (*TriggerRecord, error) {
	params := map[string]any{
		"triggerids":         []string{triggerID},
		"output":             []string{"triggerid", "description", "expression", "priority", "status", "value", "lastchange"},
		"selectDependencies": "extend",
		"expandExpression":   true,
	}

	var triggers []TriggerRecord
	if err := c.Call(ctx, "trigger.get", params, &triggers); err != nil {
		return nil, fmt.Errorf("trigger.get: %w", err)
	}
	if len(triggers) == 0 {
		return nil, fmt.Errorf("trigger not found: %s", triggerID)
	}
	return &triggers[0], nil
}

// CreateTrigger creates a new trigger and returns the created trigger ID.
func (c *Client) CreateTrigger(ctx context.Context, p CreateTriggerParams) (string, error) {
	payload, err := BuildCreateTriggerPayload(p)
	if err != nil {
		return "", err
	}

	var res struct {
		TriggerIDs []string `json:"triggerids"`
	}
	if err := c.Call(ctx, "trigger.create", payload, &res); err != nil {
		return "", fmt.Errorf("trigger.create: %w", err)
	}
	if len(res.TriggerIDs) == 0 {
		return "", errors.New("trigger.create returned no triggerids")
	}
	return res.TriggerIDs[0], nil
}

// DeleteTrigger deletes triggers by their IDs.
func (c *Client) DeleteTrigger(ctx context.Context, triggerIDs []string) error {
	var res struct {
		TriggerIDs []string `json:"triggerids"`
	}
	if err := c.Call(ctx, "trigger.delete", triggerIDs, &res); err != nil {
		return fmt.Errorf("trigger.delete: %w", err)
	}
	return nil
}
