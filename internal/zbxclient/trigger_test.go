package zbxclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zx/internal/config"
)

func TestParsePriority(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"disaster", 5, false},
		{"DISASTER", 5, false},
		{"high", 4, false},
		{"High", 4, false},
		{"average", 3, false},
		{"avg", 3, false},
		{"warning", 2, false},
		{"warn", 2, false},
		{"info", 1, false},
		{"information", 1, false},
		{"not_classified", 0, false},
		{"not classified", 0, false},
		{"none", 0, false},
		{"0", 0, false},
		{"1", 1, false},
		{"2", 2, false},
		{"3", 3, false},
		{"4", 4, false},
		{"5", 5, false},
		{"6", 0, true},
		{"-1", 0, true},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		got, err := ParsePriority(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParsePriority(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParsePriority(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestFormatPriority(t *testing.T) {
	tests := []struct {
		input any
		want  string
	}{
		{0, "Not classified"},
		{"0", "Not classified"},
		{1, "Information"},
		{"1", "Information"},
		{2, "Warning"},
		{"2", "Warning"},
		{3, "Average"},
		{"3", "Average"},
		{4, "High"},
		{"4", "High"},
		{5, "Disaster"},
		{"5", "Disaster"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		got := FormatPriority(tt.input)
		if got != tt.want {
			t.Errorf("FormatPriority(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGetTriggers(t *testing.T) {
	var capturedMethod string
	var capturedParams map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
					"triggerid":   "1001",
					"description": "High CPU utilization",
					"expression":  "min(/host/system.cpu.util,5m)>=85",
					"priority":    "4",
					"status":      "0",
					"value":       "0",
					"lastchange":  "1700000000",
					"dependencies": []map[string]any{
						{"triggerid": "1002", "description": "Critical CPU"},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(&config.Profile{URL: ts.URL, Token: "test-token"}, 5*time.Second)

	prio := 4
	triggers, err := client.GetTriggers(context.Background(), TriggerFilter{
		HostID:   "10500",
		Priority: &prio,
	})
	if err != nil {
		t.Fatalf("GetTriggers failed: %v", err)
	}

	if capturedMethod != "trigger.get" {
		t.Errorf("expected trigger.get, got %s", capturedMethod)
	}

	hostIDs, ok := capturedParams["hostids"].([]any)
	if !ok || len(hostIDs) != 1 || hostIDs[0] != "10500" {
		t.Errorf("expected hostids [10500], got %v", capturedParams["hostids"])
	}

	filter, ok := capturedParams["filter"].(map[string]any)
	if !ok || filter["priority"] != float64(4) {
		t.Errorf("expected filter.priority = 4, got %v", capturedParams["filter"])
	}

	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	tr := triggers[0]
	if tr.TriggerID != "1001" || tr.Priority != "4" || len(tr.Dependencies) != 1 {
		t.Errorf("unexpected trigger record: %+v", tr)
	}
	if tr.Dependencies[0].TriggerID != "1002" {
		t.Errorf("unexpected dependency: %+v", tr.Dependencies[0])
	}
}

func TestGetTrigger(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": []map[string]any{
				{
					"triggerid":   "2001",
					"description": "Disk space low",
					"priority":    "3",
					"status":      "0",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(&config.Profile{URL: ts.URL, Token: "test-token"}, 5*time.Second)
	tr, err := client.GetTrigger(context.Background(), "2001")
	if err != nil {
		t.Fatalf("GetTrigger failed: %v", err)
	}
	if tr.TriggerID != "2001" || tr.Description != "Disk space low" {
		t.Errorf("unexpected trigger: %+v", tr)
	}
}

func TestCreateTriggerWithDependencies(t *testing.T) {
	var capturedMethod string
	var capturedParams map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				"triggerids": []string{"3001"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(&config.Profile{URL: ts.URL, Token: "test-token"}, 5*time.Second)

	params := CreateTriggerParams{
		Description: "CPU Utilization HIGH (>= 85%) on host1",
		Expression:  "min(/host1/system.cpu.util,5m)>=85",
		Priority:    4,
		Status:      0,
		DependsOn:   "3000",
	}

	id, err := client.CreateTrigger(context.Background(), params)
	if err != nil {
		t.Fatalf("CreateTrigger failed: %v", err)
	}
	if id != "3001" {
		t.Errorf("expected trigger ID 3001, got %s", id)
	}

	if capturedMethod != "trigger.create" {
		t.Errorf("expected method trigger.create, got %s", capturedMethod)
	}
	if capturedParams["description"] != params.Description {
		t.Errorf("expected description %q, got %v", params.Description, capturedParams["description"])
	}
	if capturedParams["expression"] != params.Expression {
		t.Errorf("expected expression %q, got %v", params.Expression, capturedParams["expression"])
	}
	if capturedParams["priority"] != float64(4) {
		t.Errorf("expected priority 4, got %v", capturedParams["priority"])
	}
	deps, ok := capturedParams["dependencies"].([]any)
	if !ok || len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %v", capturedParams["dependencies"])
	}
	depMap := deps[0].(map[string]any)
	if depMap["triggerid"] != "3000" {
		t.Errorf("expected dependency triggerid 3000, got %v", depMap["triggerid"])
	}
}

func TestDeleteTrigger(t *testing.T) {
	var capturedMethod string
	var capturedParams []any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				"triggerids": []string{"3001"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(&config.Profile{URL: ts.URL, Token: "test-token"}, 5*time.Second)
	err := client.DeleteTrigger(context.Background(), []string{"3001"})
	if err != nil {
		t.Fatalf("DeleteTrigger failed: %v", err)
	}
	if capturedMethod != "trigger.delete" {
		t.Errorf("expected trigger.delete, got %s", capturedMethod)
	}
	if len(capturedParams) != 1 || capturedParams[0] != "3001" {
		t.Errorf("expected params [3001], got %v", capturedParams)
	}
}
