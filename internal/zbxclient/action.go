package zbxclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ActionRecord represents a Zabbix action.
type ActionRecord struct {
	ActionID           string          `json:"actionid"`
	Name               string          `json:"name"`
	EventSource        string          `json:"eventsource"`
	Status             string          `json:"status"` // "0" = active, "1" = disabled
	EscPeriod          string          `json:"esc_period"`
	Filter             json.RawMessage `json:"filter,omitempty"`
	Operations         json.RawMessage `json:"operations,omitempty"`
	RecoveryOperations json.RawMessage `json:"recovery_operations,omitempty"`
	UpdateOperations   json.RawMessage `json:"update_operations,omitempty"`
}

// ActionFilter controls query filters for action.get.
type ActionFilter struct {
	ActionIDs   []string `json:"actionids,omitempty"`
	EventSource *int     `json:"eventsource,omitempty"`
	Status      *int     `json:"status,omitempty"`
	Name        string   `json:"name,omitempty"`
}

// CreateActionParams specifies parameters to build and create an action.
type CreateActionParams struct {
	Name        string
	EventSource int    // default 0 (triggers)
	Status      int    // 0 = active, 1 = disabled (default 1)
	EscPeriod   string // default "1h"
	HostGroupID string
	MediaTypeID string
	UserID      string
	UserGroupID string
	Subject     string
	Message     string
}

// BuildCreateActionPayload builds the JSON-RPC params map for action.create.
func BuildCreateActionPayload(p CreateActionParams) (map[string]any, error) {
	if strings.TrimSpace(p.Name) == "" {
		return nil, errors.New("action name is required")
	}
	if strings.TrimSpace(p.Subject) == "" {
		return nil, errors.New("action subject is required")
	}
	if strings.TrimSpace(p.Message) == "" {
		return nil, errors.New("action message is required")
	}

	escPeriod := p.EscPeriod
	if escPeriod == "" {
		escPeriod = "1h"
	}

	filter := map[string]any{
		"evaltype":   0,
		"conditions": []map[string]any{},
	}
	if p.HostGroupID != "" {
		filter["conditions"] = []map[string]any{
			{
				"conditiontype": 0, // CONDITION_TYPE_HOST_GROUP
				"operator":      0, // CONDITION_OPERATOR_EQUAL
				"value":         p.HostGroupID,
			},
		}
	}

	opMsg := map[string]any{
		"default_msg": 0,
		"subject":     p.Subject,
		"message":     p.Message,
	}
	if p.MediaTypeID != "" {
		opMsg["mediatypeid"] = p.MediaTypeID
	}

	op := map[string]any{
		"operationtype": 0, // OPERATION_TYPE_MESSAGE
		"esc_period":    "0",
		"esc_step_from": 1,
		"esc_step_to":   1,
		"opmessage":     opMsg,
	}
	if p.UserID != "" {
		op["opmessage_usr"] = []map[string]string{{"userid": p.UserID}}
	}
	if p.UserGroupID != "" {
		op["opmessage_grp"] = []map[string]string{{"usrgrpid": p.UserGroupID}}
	}

	payload := map[string]any{
		"name":        p.Name,
		"eventsource": p.EventSource,
		"status":      p.Status,
		"esc_period":  escPeriod,
		"filter":      filter,
		"operations":  []map[string]any{op},
	}

	return payload, nil
}

// GetActions retrieves actions matching the given filter.
func (c *Client) GetActions(ctx context.Context, filter ActionFilter) ([]ActionRecord, error) {
	params := map[string]any{
		"output": []string{"actionid", "name", "eventsource", "status", "esc_period"},
	}

	f := map[string]any{}
	if filter.EventSource != nil {
		f["eventsource"] = *filter.EventSource
	}
	if filter.Status != nil {
		f["status"] = *filter.Status
	}
	if filter.Name != "" {
		f["name"] = filter.Name
	}
	if len(f) > 0 {
		params["filter"] = f
	}
	if len(filter.ActionIDs) > 0 {
		params["actionids"] = filter.ActionIDs
	}

	var actions []ActionRecord
	if err := c.Call(ctx, "action.get", params, &actions); err != nil {
		return nil, fmt.Errorf("action.get: %w", err)
	}
	return actions, nil
}

// GetAction retrieves full details for a single action by ID.
func (c *Client) GetAction(ctx context.Context, actionID string) (*ActionRecord, error) {
	params := map[string]any{
		"actionids":                []string{actionID},
		"output":                   "extend",
		"selectFilter":             "extend",
		"selectOperations":         "extend",
		"selectRecoveryOperations": "extend",
		"selectUpdateOperations":   "extend",
	}

	var actions []ActionRecord
	if err := c.Call(ctx, "action.get", params, &actions); err != nil {
		return nil, fmt.Errorf("action.get: %w", err)
	}
	if len(actions) == 0 {
		return nil, fmt.Errorf("action not found: %s", actionID)
	}
	return &actions[0], nil
}

// CreateAction creates a new action on the Zabbix server.
func (c *Client) CreateAction(ctx context.Context, p CreateActionParams) (string, error) {
	payload, err := BuildCreateActionPayload(p)
	if err != nil {
		return "", err
	}

	var res struct {
		ActionIDs []string `json:"actionids"`
	}
	if err := c.Call(ctx, "action.create", payload, &res); err != nil {
		return "", fmt.Errorf("action.create: %w", err)
	}
	if len(res.ActionIDs) == 0 {
		return "", errors.New("action.create returned no actionids")
	}
	return res.ActionIDs[0], nil
}

// UpdateAction updates an existing action by ID.
func (c *Client) UpdateAction(ctx context.Context, actionID string, updates map[string]any) error {
	payload := make(map[string]any, len(updates)+1)
	for k, v := range updates {
		payload[k] = v
	}
	payload["actionid"] = actionID

	_, hasSubj := payload["subject"]
	_, hasMsg := payload["message"]
	if hasSubj || hasMsg {
		if _, hasOps := payload["operations"]; !hasOps {
			op := map[string]any{
				"operationtype": 0,
				"esc_period":    "0",
				"esc_step_from": 1,
				"esc_step_to":   1,
				"opmessage": map[string]any{
					"default_msg": 0,
				},
			}
			opMsg := op["opmessage"].(map[string]any)
			if s, ok := payload["subject"].(string); ok {
				opMsg["subject"] = s
			}
			if m, ok := payload["message"].(string); ok {
				opMsg["message"] = m
			}
			payload["operations"] = []map[string]any{op}
		}
		delete(payload, "subject")
		delete(payload, "message")
	}

	var res struct {
		ActionIDs []string `json:"actionids"`
	}
	if err := c.Call(ctx, "action.update", payload, &res); err != nil {
		return fmt.Errorf("action.update: %w", err)
	}
	return nil
}

// DeleteAction removes actions by IDs.
func (c *Client) DeleteAction(ctx context.Context, actionIDs []string) error {
	var res struct {
		ActionIDs []string `json:"actionids"`
	}
	if err := c.Call(ctx, "action.delete", actionIDs, &res); err != nil {
		return fmt.Errorf("action.delete: %w", err)
	}
	return nil
}
