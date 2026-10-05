package zbxclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"zx/internal/config"
)

func newTestClientWithHandler(t *testing.T, handler http.HandlerFunc) (*Client, func()) {
	t.Helper()
	ts := httptest.NewServer(handler)
	prof := &config.Profile{URL: ts.URL, Token: "test-token"}
	client := NewClient(prof, 5*time.Second)
	return client, ts.Close
}

func TestGetActions(t *testing.T) {
	var capturedMethod string
	var capturedParams map[string]any

	client, cleanup := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
			ID     int            `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode error: %v", err)
			return
		}
		capturedMethod = req.Method
		capturedParams = req.Params

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": []map[string]any{
				{
					"actionid":    "101",
					"name":        "Notify Telegram on Problem",
					"eventsource": "0",
					"status":      "0",
					"esc_period":  "1h",
				},
				{
					"actionid":    "102",
					"name":        "Disabled Action",
					"eventsource": "0",
					"status":      "1",
					"esc_period":  "30m",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	ctx := context.Background()
	es := 0
	actions, err := client.GetActions(ctx, ActionFilter{
		EventSource: &es,
	})
	if err != nil {
		t.Fatalf("GetActions failed: %v", err)
	}

	if capturedMethod != "action.get" {
		t.Errorf("expected method action.get, got %s", capturedMethod)
	}

	filter, ok := capturedParams["filter"].(map[string]any)
	if !ok {
		t.Fatalf("expected filter map, got: %v", capturedParams["filter"])
	}
	if esVal, ok := filter["eventsource"].(float64); !ok || int(esVal) != 0 {
		t.Errorf("expected filter.eventsource = 0, got %v", filter["eventsource"])
	}

	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	if actions[0].ActionID != "101" || actions[0].Name != "Notify Telegram on Problem" {
		t.Errorf("unexpected action[0]: %+v", actions[0])
	}
	if actions[0].Status != "0" {
		t.Errorf("expected status '0', got %s", actions[0].Status)
	}
}

func TestGetAction(t *testing.T) {
	var capturedMethod string
	var capturedParams map[string]any

	client, cleanup := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
			ID     int            `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedMethod = req.Method
		capturedParams = req.Params

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": []map[string]any{
				{
					"actionid":    "101",
					"name":        "Notify Telegram",
					"eventsource": "0",
					"status":      "0",
					"esc_period":  "1h",
					"filter": map[string]any{
						"evaltype": 0,
						"conditions": []map[string]any{
							{"conditiontype": 0, "operator": 0, "value": "15"},
						},
					},
					"operations": []map[string]any{
						{
							"operationtype": 0,
							"opmessage": map[string]any{
								"subject": "Problem: {EVENT.NAME}",
								"message": "Details: {EVENT.OPDATA}",
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	ctx := context.Background()
	action, err := client.GetAction(ctx, "101")
	if err != nil {
		t.Fatalf("GetAction failed: %v", err)
	}

	if capturedMethod != "action.get" {
		t.Errorf("expected method action.get, got %s", capturedMethod)
	}
	ids, ok := capturedParams["actionids"].([]any)
	if !ok || len(ids) != 1 || ids[0] != "101" {
		t.Errorf("expected actionids ['101'], got %v", capturedParams["actionids"])
	}
	if action.ActionID != "101" || action.Name != "Notify Telegram" {
		t.Errorf("unexpected action: %+v", action)
	}
}

func TestCreateAction(t *testing.T) {
	var capturedMethod string
	var capturedParams map[string]any

	client, cleanup := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
			ID     int            `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedMethod = req.Method
		capturedParams = req.Params

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"actionids": []string{"201"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	ctx := context.Background()
	params := CreateActionParams{
		Name:        "Telegram Alert for Web Servers",
		EventSource: 0,
		Status:      1,
		EscPeriod:   "1h",
		HostGroupID: "2",
		MediaTypeID: "5",
		UserID:      "1",
		UserGroupID: "7",
		Subject:     "Problem: {EVENT.NAME}",
		Message:     "Host: {HOST.NAME}\nSeverity: {EVENT.SEVERITY}",
	}

	actionID, err := client.CreateAction(ctx, params)
	if err != nil {
		t.Fatalf("CreateAction failed: %v", err)
	}

	if actionID != "201" {
		t.Errorf("expected actionID '201', got %s", actionID)
	}
	if capturedMethod != "action.create" {
		t.Errorf("expected method action.create, got %s", capturedMethod)
	}

	// Verify params structure
	if capturedParams["name"] != "Telegram Alert for Web Servers" {
		t.Errorf("expected name 'Telegram Alert for Web Servers', got %v", capturedParams["name"])
	}
	if status, ok := capturedParams["status"].(float64); !ok || int(status) != 1 {
		t.Errorf("expected status 1, got %v", capturedParams["status"])
	}

	// Filter conditions
	filter, ok := capturedParams["filter"].(map[string]any)
	if !ok {
		t.Fatalf("expected filter map, got %v", capturedParams["filter"])
	}
	conditions, ok := filter["conditions"].([]any)
	if !ok || len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %v", filter["conditions"])
	}
	cond0 := conditions[0].(map[string]any)
	if cond0["value"] != "2" {
		t.Errorf("expected condition value '2', got %v", cond0["value"])
	}

	// Operations
	ops, ok := capturedParams["operations"].([]any)
	if !ok || len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %v", capturedParams["operations"])
	}
	op0 := ops[0].(map[string]any)
	opmsg, ok := op0["opmessage"].(map[string]any)
	if !ok {
		t.Fatalf("expected opmessage, got %v", op0["opmessage"])
	}
	if opmsg["subject"] != "Problem: {EVENT.NAME}" {
		t.Errorf("expected subject, got %v", opmsg["subject"])
	}
	if opmsg["message"] != "Host: {HOST.NAME}\nSeverity: {EVENT.SEVERITY}" {
		t.Errorf("expected message, got %v", opmsg["message"])
	}
	if opmsg["mediatypeid"] != "5" {
		t.Errorf("expected mediatypeid '5', got %v", opmsg["mediatypeid"])
	}

	opUsers, ok := op0["opmessage_usr"].([]any)
	if !ok || len(opUsers) != 1 {
		t.Errorf("expected 1 user in opmessage_usr, got %v", op0["opmessage_usr"])
	}
	opGroups, ok := op0["opmessage_grp"].([]any)
	if !ok || len(opGroups) != 1 {
		t.Errorf("expected 1 group in opmessage_grp, got %v", op0["opmessage_grp"])
	}
}

func TestUpdateAction(t *testing.T) {
	var capturedMethod string
	var capturedParams map[string]any

	client, cleanup := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
			ID     int            `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedMethod = req.Method
		capturedParams = req.Params

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"actionids": []string{"101"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	ctx := context.Background()

	// Update status
	err := client.UpdateAction(ctx, "101", map[string]any{
		"status": 0,
	})
	if err != nil {
		t.Fatalf("UpdateAction failed: %v", err)
	}
	if capturedMethod != "action.update" {
		t.Errorf("expected method action.update, got %s", capturedMethod)
	}
	if capturedParams["actionid"] != "101" {
		t.Errorf("expected actionid '101', got %v", capturedParams["actionid"])
	}
	if status, ok := capturedParams["status"].(float64); !ok || int(status) != 0 {
		t.Errorf("expected status 0, got %v", capturedParams["status"])
	}

	// Update subject / message
	err = client.UpdateAction(ctx, "101", map[string]any{
		"subject": "New Subject",
		"message": "New Message",
	})
	if err != nil {
		t.Fatalf("UpdateAction subject/message failed: %v", err)
	}
	ops, ok := capturedParams["operations"].([]any)
	if !ok || len(ops) != 1 {
		t.Fatalf("expected operations with 1 op, got %v", capturedParams["operations"])
	}
	op0 := ops[0].(map[string]any)
	opmsg := op0["opmessage"].(map[string]any)
	if opmsg["subject"] != "New Subject" || opmsg["message"] != "New Message" {
		t.Errorf("unexpected opmessage: %v", opmsg)
	}
	if _, exists := capturedParams["subject"]; exists {
		t.Errorf("expected subject removed from top level payload")
	}
}

func TestDeleteAction(t *testing.T) {
	var capturedMethod string
	var capturedParams []any

	client, cleanup := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
			ID     int    `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedMethod = req.Method
		capturedParams = req.Params

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"actionids": []string{"101", "102"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	ctx := context.Background()
	err := client.DeleteAction(ctx, []string{"101", "102"})
	if err != nil {
		t.Fatalf("DeleteAction failed: %v", err)
	}

	if capturedMethod != "action.delete" {
		t.Errorf("expected action.delete, got %s", capturedMethod)
	}
	expectedParams := []any{"101", "102"}
	if !reflect.DeepEqual(capturedParams, expectedParams) {
		t.Errorf("expected %v, got %v", expectedParams, capturedParams)
	}
}
