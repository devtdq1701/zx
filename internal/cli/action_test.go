package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCLIActionList(t *testing.T) {
	mockActions := []map[string]any{
		{
			"actionid":    "10",
			"name":        "Telegram Notification",
			"eventsource": "0",
			"status":      "0",
			"esc_period":  "1h",
		},
		{
			"actionid":    "20",
			"name":        "Email Admin",
			"eventsource": "0",
			"status":      "1",
			"esc_period":  "30m",
		},
	}

	h, cleanup := setupMockClient(t, map[string]any{
		"action.get": mockActions,
	})
	defer cleanup()

	// 1. Table format (default)
	out, _, err := runCLI(t, "action", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "ACTIONID") || !strings.Contains(out, "Telegram Notification") {
		t.Errorf("expected table output with headers and action name, got: %s", out)
	}
	if !strings.Contains(out, "active") || !strings.Contains(out, "disabled") {
		t.Errorf("expected friendly status labels active/disabled, got: %s", out)
	}

	// 2. JSON format
	outJSON, _, err := runCLI(t, "action", "list", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error with --format json: %v", err)
	}
	var res []map[string]any
	if err := json.Unmarshal([]byte(outJSON), &res); err != nil {
		t.Fatalf("invalid json output: %v, raw: %s", err, outJSON)
	}
	if len(res) != 2 || res[0]["actionid"] != "10" {
		t.Errorf("unexpected json result: %v", res)
	}

	if n := h.count("action.get"); n != 2 {
		t.Errorf("expected 2 action.get calls, got %d", n)
	}
}

func TestCLIActionGet(t *testing.T) {
	mockAction := []map[string]any{
		{
			"actionid":    "15",
			"name":        "Get Action Test",
			"eventsource": "0",
			"status":      "0",
			"esc_period":  "1h",
			"filter": map[string]any{
				"evaltype":   0,
				"conditions": []any{},
			},
		},
	}

	h, cleanup := setupMockClient(t, map[string]any{
		"action.get": mockAction,
	})
	defer cleanup()

	out, _, err := runCLI(t, "action", "get", "15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("expected JSON output for action get: %v, got: %s", err, out)
	}
	if parsed["actionid"] != "15" || parsed["name"] != "Get Action Test" {
		t.Errorf("unexpected action get output: %v", parsed)
	}
	if n := h.count("action.get"); n != 1 {
		t.Errorf("expected 1 action.get call, got %d", n)
	}
}

func TestCLIActionCreateValidation(t *testing.T) {
	_, cleanup := setupMockClient(t, map[string]any{})
	defer cleanup()

	// Missing required flags
	_, _, err := runCLI(t, "action", "create", "--name", "Test")
	if err == nil {
		t.Fatalf("expected error when --subject and --message are missing")
	}
}

func TestCLIActionCreateDryRunAndYes(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"action.create": map[string]any{
			"actionids": []string{"99"},
		},
	})
	defer cleanup()

	// 1. Dry-run without --yes
	out, _, err := runCLI(t, "action", "create",
		"--name", "DryRunAction",
		"--subject", "Subj",
		"--message", "Msg",
	)
	if err != nil {
		t.Fatalf("unexpected error on dry run: %v", err)
	}
	if !strings.Contains(out, "[DRY-RUN]") || !strings.Contains(out, "action.create") {
		t.Errorf("expected [DRY-RUN] preview, got: %s", out)
	}
	if n := h.count("action.create"); n != 0 {
		t.Fatalf("dry-run should not call action.create, got %d calls", n)
	}

	// 2. Execution with --yes
	outYes, _, err := runCLI(t, "action", "create",
		"--name", "DryRunAction",
		"--subject", "Subj",
		"--message", "Msg",
		"--yes",
	)
	if err != nil {
		t.Fatalf("unexpected error with --yes: %v", err)
	}
	if strings.Contains(outYes, "[DRY-RUN]") {
		t.Errorf("did not expect [DRY-RUN] with --yes")
	}
	if !strings.Contains(outYes, "Created action") {
		t.Errorf("expected success message, got: %s", outYes)
	}
	if n := h.count("action.create"); n != 1 {
		t.Fatalf("expected 1 action.create call, got %d", n)
	}
}

func TestCLIActionUpdate(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"action.update": map[string]any{
			"actionids": []string{"15"},
		},
	})
	defer cleanup()

	// 1. Dry run
	out, _, err := runCLI(t, "action", "update", "15", "--status", "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "[DRY-RUN]") || !strings.Contains(out, "action.update") {
		t.Errorf("expected [DRY-RUN] preview, got: %s", out)
	}
	if n := h.count("action.update"); n != 0 {
		t.Fatalf("dry run sent action.update")
	}

	// 2. With --yes
	outYes, _, err := runCLI(t, "action", "update", "15", "--status", "active", "--yes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(outYes, "Updated action 15") {
		t.Errorf("expected update confirmation, got: %s", outYes)
	}
	if n := h.count("action.update"); n != 1 {
		t.Fatalf("expected 1 action.update call, got %d", n)
	}
}

func TestCLIActionDelete(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"action.delete": map[string]any{
			"actionids": []string{"15"},
		},
	})
	defer cleanup()

	// 1. Dry run
	out, _, err := runCLI(t, "action", "delete", "15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "[DRY-RUN]") || !strings.Contains(out, "action.delete") {
		t.Errorf("expected [DRY-RUN] preview, got: %s", out)
	}
	if n := h.count("action.delete"); n != 0 {
		t.Fatalf("dry run sent action.delete")
	}

	// 2. With --yes
	outYes, _, err := runCLI(t, "action", "delete", "15", "--yes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(outYes, "Deleted action 15") {
		t.Errorf("expected delete confirmation, got: %s", outYes)
	}
	if n := h.count("action.delete"); n != 1 {
		t.Fatalf("expected 1 action.delete call, got %d", n)
	}
}
