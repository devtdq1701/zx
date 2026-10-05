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

func TestTestMediaType(t *testing.T) {
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
				"result": true,
				"error":  "",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(&config.Profile{URL: ts.URL, Token: "test-token"}, 5*time.Second)

	params := MediaTypeTestParams{
		MediaTypeID: "104",
		SendTo:      "-5032963651",
		Subject:     "Test Alert",
		Message:     "Ping from zx",
	}

	res, err := client.TestMediaType(context.Background(), params)
	if err != nil {
		t.Fatalf("TestMediaType failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected res.Success == true, got false")
	}
	if res.Error != "" {
		t.Errorf("expected empty res.Error, got %q", res.Error)
	}

	if capturedMethod != "mediatype.test" {
		t.Errorf("expected method mediatype.test, got %s", capturedMethod)
	}
	if capturedParams["mediatypeid"] != "104" {
		t.Errorf("expected mediatypeid 104, got %v", capturedParams["mediatypeid"])
	}
	if capturedParams["sendto"] != "-5032963651" {
		t.Errorf("expected sendto -5032963651, got %v", capturedParams["sendto"])
	}
	if capturedParams["subject"] != "Test Alert" {
		t.Errorf("expected subject 'Test Alert', got %v", capturedParams["subject"])
	}
	if capturedParams["message"] != "Ping from zx" {
		t.Errorf("expected message 'Ping from zx', got %v", capturedParams["message"])
	}
}

func TestExecuteItemNow(t *testing.T) {
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
				"taskids": []string{"1"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(&config.Profile{URL: ts.URL, Token: "test-token"}, 5*time.Second)

	err := client.ExecuteItemNow(context.Background(), "12345")
	if err != nil {
		t.Fatalf("ExecuteItemNow failed: %v", err)
	}

	if capturedMethod != "task.create" {
		t.Errorf("expected method task.create, got %s", capturedMethod)
	}
	if capturedParams["type"] != float64(6) {
		t.Errorf("expected type 6, got %v", capturedParams["type"])
	}
	reqObj, ok := capturedParams["request"].(map[string]any)
	if !ok {
		t.Fatalf("expected request object in params, got %v", capturedParams["request"])
	}
	if reqObj["itemid"] != "12345" {
		t.Errorf("expected itemid 12345, got %v", reqObj["itemid"])
	}
}
