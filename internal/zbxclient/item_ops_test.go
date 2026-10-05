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

func TestParseItemType(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"zabbix_active", 7, false},
		{"active", 7, false},
		{"ZABBIX_ACTIVE", 7, false},
		{"7", 7, false},
		{"zabbix_agent", 0, false},
		{"agent", 0, false},
		{"0", 0, false},
		{"trapper", 2, false},
		{"zabbix_trapper", 2, false},
		{"2", 2, false},
		{"simple", 3, false},
		{"simple_check", 3, false},
		{"3", 3, false},
		{"calculated", 15, false},
		{"15", 15, false},
		{"invalid", 0, true},
		{"-1", 0, true},
		{"99", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseItemType(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseItemType(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseItemType(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseValueType(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"numeric_unsigned", 3, false},
		{"unsigned", 3, false},
		{"uint", 3, false},
		{"3", 3, false},
		{"numeric_float", 0, false},
		{"float", 0, false},
		{"0", 0, false},
		{"char", 1, false},
		{"character", 1, false},
		{"1", 1, false},
		{"log", 2, false},
		{"2", 2, false},
		{"text", 4, false},
		{"TEXT", 4, false},
		{"4", 4, false},
		{"invalid", 0, true},
		{"-1", 0, true},
		{"5", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseValueType(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseValueType(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseValueType(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestCreateItem(t *testing.T) {
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
			"result": map[string]any{
				"itemids": []string{"50001"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(&config.Profile{URL: ts.URL, Token: "test-token"}, 5*time.Second)

	params := CreateItemParams{
		HostID:    "10001",
		Name:      "Convert Process Max Runtime",
		Key:       "convert.process.max_runtime",
		Type:      7,
		ValueType: 3,
		Delay:     "1m",
		Units:     "s",
		Status:    0,
	}

	id, err := client.CreateItem(context.Background(), params)
	if err != nil {
		t.Fatalf("CreateItem failed: %v", err)
	}
	if id != "50001" {
		t.Errorf("expected item ID 50001, got %s", id)
	}

	if capturedMethod != "item.create" {
		t.Errorf("expected method item.create, got %s", capturedMethod)
	}
	if capturedParams["hostid"] != "10001" {
		t.Errorf("expected hostid 10001, got %v", capturedParams["hostid"])
	}
	if capturedParams["name"] != "Convert Process Max Runtime" {
		t.Errorf("expected name 'Convert Process Max Runtime', got %v", capturedParams["name"])
	}
	if capturedParams["key_"] != "convert.process.max_runtime" {
		t.Errorf("expected key_ 'convert.process.max_runtime', got %v", capturedParams["key_"])
	}
	if capturedParams["type"] != float64(7) {
		t.Errorf("expected type 7, got %v", capturedParams["type"])
	}
	if capturedParams["value_type"] != float64(3) {
		t.Errorf("expected value_type 3, got %v", capturedParams["value_type"])
	}
	if capturedParams["delay"] != "1m" {
		t.Errorf("expected delay '1m', got %v", capturedParams["delay"])
	}
	if capturedParams["units"] != "s" {
		t.Errorf("expected units 's', got %v", capturedParams["units"])
	}
	if capturedParams["status"] != float64(0) {
		t.Errorf("expected status 0, got %v", capturedParams["status"])
	}
}

func TestUpdateItem(t *testing.T) {
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
			"result": map[string]any{
				"itemids": []string{"50001"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(&config.Profile{URL: ts.URL, Token: "test-token"}, 5*time.Second)

	updates := map[string]any{
		"delay":  "30s",
		"status": 0,
	}

	err := client.UpdateItem(context.Background(), "50001", updates)
	if err != nil {
		t.Fatalf("UpdateItem failed: %v", err)
	}

	if capturedMethod != "item.update" {
		t.Errorf("expected method item.update, got %s", capturedMethod)
	}
	if capturedParams["itemid"] != "50001" {
		t.Errorf("expected itemid 50001, got %v", capturedParams["itemid"])
	}
	if capturedParams["delay"] != "30s" {
		t.Errorf("expected delay '30s', got %v", capturedParams["delay"])
	}
	if capturedParams["status"] != float64(0) {
		t.Errorf("expected status 0, got %v", capturedParams["status"])
	}
}
