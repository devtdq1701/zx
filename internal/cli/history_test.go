package cli

import (
	"strings"
	"testing"
)

func TestHistoryGetAutoDetectType(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"item.get": []any{
			map[string]any{"itemid": "1001", "value_type": "0"}, // float
		},
		"history.get": []any{
			map[string]any{"clock": "1728123456", "value": "45.67"},
		},
	})
	defer cleanup()

	out, _, err := runCLI(t, "history", "get", "1001", "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "45.67") {
		t.Fatalf("expected value 45.67 in output, got: %s", out)
	}
	if n := h.count("item.get"); n != 1 {
		t.Fatalf("expected 1 item.get call, got %d", n)
	}
	if n := h.count("history.get"); n != 1 {
		t.Fatalf("expected 1 history.get call, got %d", n)
	}
}

func TestHistoryGetWithExplicitType(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"history.get": []any{
			map[string]any{"clock": "1728123456", "severity": "1", "value": "Error connecting to DB"},
		},
	})
	defer cleanup()

	out, _, err := runCLI(t, "history", "get", "2002", "--type", "log")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Error connecting to DB") {
		t.Fatalf("expected log message in output, got: %s", out)
	}
	// item.get should NOT be called when explicit --type is given
	if n := h.count("item.get"); n != 0 {
		t.Fatalf("expected 0 item.get call, got %d", n)
	}
	if n := h.count("history.get"); n != 1 {
		t.Fatalf("expected 1 history.get call, got %d", n)
	}
}

func TestHistoryGetJSONFormat(t *testing.T) {
	_, cleanup := setupMockClient(t, map[string]any{
		"history.get": []any{
			map[string]any{"clock": "1728123456", "value": "123"},
		},
	})
	defer cleanup()

	out, _, err := runCLI(t, "history", "get", "3003", "--type", "uint", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"value": "123"`) {
		t.Fatalf("expected json output, got: %s", out)
	}
}
