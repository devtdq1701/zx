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

func TestClientJSONRPC(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test_bearer_token" {
			t.Errorf("missing or invalid Authorization header: %s", r.Header.Get("Authorization"))
		}

		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.Method != "apiinfo.version" {
			t.Errorf("unexpected method: %s", req.Method)
		}

		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`"7.0.0"`),
			ID:      req.ID,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	prof := &config.Profile{
		URL:       ts.URL,
		Token:     "test_bearer_token",
		VerifySSL: false,
	}

	client := NewClient(prof, 5*time.Second)

	var version string
	err := client.Call(context.Background(), "apiinfo.version", map[string]any{}, &version)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	if version != "7.0.0" {
		t.Errorf("expected version 7.0.0, got %s", version)
	}
}

func TestClientPreflight(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPreconditionFailed) // 412 is normal for Zabbix web root probe
	}))
	defer ts.Close()

	prof := &config.Profile{
		URL:       ts.URL,
		VerifySSL: false,
	}

	client := NewClient(prof, 2*time.Second)
	status, duration, err := client.Preflight(context.Background())
	if err != nil {
		t.Fatalf("Preflight failed: %v", err)
	}

	if status != http.StatusPreconditionFailed {
		t.Errorf("expected status 412, got %d", status)
	}
	if duration <= 0 {
		t.Errorf("expected positive duration, got %v", duration)
	}
}

func TestNormalizeRPCURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"http://192.0.2.10", "http://192.0.2.10/api_jsonrpc.php"},
		{"http://192.0.2.10/", "http://192.0.2.10/api_jsonrpc.php"},
		{"https://198.51.100.20/gsm.php", "https://198.51.100.20/api_jsonrpc.php"},
		{"https://198.51.100.20/index.php", "https://198.51.100.20/api_jsonrpc.php"},
		{"http://example.com/zabbix/index.php", "http://example.com/zabbix/api_jsonrpc.php"},
		{"http://example.com/zabbix", "http://example.com/zabbix/api_jsonrpc.php"},
		{"http://example.com/zabbix/api_jsonrpc.php", "http://example.com/zabbix/api_jsonrpc.php"},
	}

	for _, tt := range tests {
		got := normalizeRPCURL(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeRPCURL(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
