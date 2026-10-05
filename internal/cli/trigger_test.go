package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCLITriggerList(t *testing.T) {
	mockHosts := []map[string]any{
		{"hostid": "10050", "host": "hni-srv01", "name": "HNI Server 01"},
	}
	mockTriggers := []map[string]any{
		{
			"triggerid":   "5001",
			"description": "High CPU utilization on {HOST.NAME}",
			"priority":    "4",
			"status":      "0",
			"dependencies": []map[string]any{
				{"triggerid": "5000", "description": "Critical CPU"},
			},
		},
		{
			"triggerid":   "5002",
			"description": "Low disk space",
			"priority":    "2",
			"status":      "1",
			"dependencies": []any{},
		},
	}

	h, cleanup := setupMockClient(t, map[string]any{
		"host.get":    mockHosts,
		"trigger.get": mockTriggers,
	})
	defer cleanup()

	// 1. Table format
	out, _, err := runCLI(t, "trigger", "list", "--host", "hni-srv01")
	if err != nil {
		t.Fatalf("unexpected error on trigger list: %v", err)
	}
	if !strings.Contains(out, "TRIGGERID") || !strings.Contains(out, "High CPU utilization") {
		t.Errorf("expected table output with headers and trigger description, got: %s", out)
	}
	if !strings.Contains(out, "High") || !strings.Contains(out, "Warning") {
		t.Errorf("expected friendly severity labels High and Warning, got: %s", out)
	}
	if !strings.Contains(out, "active") || !strings.Contains(out, "disabled") {
		t.Errorf("expected status active/disabled, got: %s", out)
	}
	if !strings.Contains(out, "5000") {
		t.Errorf("expected dependency trigger ID 5000 in output, got: %s", out)
	}

	// 2. JSON format
	outJSON, _, err := runCLI(t, "trigger", "list", "--host", "hni-srv01", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error on trigger list --format json: %v", err)
	}
	var res []map[string]any
	if err := json.Unmarshal([]byte(outJSON), &res); err != nil {
		t.Fatalf("invalid json output: %v, raw: %s", err, outJSON)
	}
	if len(res) != 2 || res[0]["triggerid"] != "5001" {
		t.Errorf("unexpected json result: %v", res)
	}

	if n := h.count("trigger.get"); n != 2 {
		t.Errorf("expected 2 trigger.get calls, got %d", n)
	}
}

func TestCLITriggerCreateDryRunAndYes(t *testing.T) {
	mockHosts := []map[string]any{
		{"hostid": "10050", "host": "hni-srv01", "name": "HNI Server 01"},
	}
	h, cleanup := setupMockClient(t, map[string]any{
		"host.get": mockHosts,
		"trigger.create": map[string]any{
			"triggerids": []string{"6001"},
		},
	})
	defer cleanup()

	// 1. Dry run without --yes
	out, _, err := runCLI(t, "trigger", "create",
		"--host", "hni-srv01",
		"--description", "HNIPC OS: CPU Utilization HIGH (>= 85%) on {HOST.NAME}",
		"--expression", "min(/{HOST.HOST}/system.cpu.util,5m)>=85",
		"--priority", "high",
		"--depends-on", "6000",
	)
	if err != nil {
		t.Fatalf("unexpected error on dry run: %v", err)
	}
	if !strings.Contains(out, "[DRY-RUN]") || !strings.Contains(out, "trigger.create") {
		t.Errorf("expected [DRY-RUN] preview, got: %s", out)
	}
	if !strings.Contains(out, "min(/hni-srv01/system.cpu.util,5m)") {
		t.Errorf("expected {HOST.HOST} replaced by host name, got: %s", out)
	}
	if n := h.count("trigger.create"); n != 0 {
		t.Fatalf("dry-run should not call trigger.create, got %d calls", n)
	}

	// 2. Execution with --yes
	outYes, _, err := runCLI(t, "trigger", "create",
		"--host", "hni-srv01",
		"--description", "HNIPC OS: CPU Utilization HIGH (>= 85%) on {HOST.NAME}",
		"--expression", "min(/{HOST.HOST}/system.cpu.util,5m)>=85",
		"--priority", "high",
		"--depends-on", "6000",
		"--yes",
	)
	if err != nil {
		t.Fatalf("unexpected error with --yes: %v", err)
	}
	if strings.Contains(outYes, "[DRY-RUN]") {
		t.Errorf("did not expect [DRY-RUN] with --yes")
	}
	if !strings.Contains(outYes, "Created trigger 6001") {
		t.Errorf("expected success message with ID 6001, got: %s", outYes)
	}
	if n := h.count("trigger.create"); n != 1 {
		t.Fatalf("expected 1 trigger.create call, got %d", n)
	}
}

func TestCLITriggerDelete(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"trigger.delete": map[string]any{
			"triggerids": []string{"7001"},
		},
	})
	defer cleanup()

	// 1. Dry run
	out, _, err := runCLI(t, "trigger", "delete", "7001")
	if err != nil {
		t.Fatalf("unexpected error on delete dry-run: %v", err)
	}
	if !strings.Contains(out, "[DRY-RUN]") || !strings.Contains(out, "trigger.delete") {
		t.Errorf("expected [DRY-RUN] preview, got: %s", out)
	}
	if n := h.count("trigger.delete"); n != 0 {
		t.Fatalf("dry run should not call trigger.delete")
	}

	// 2. Execution with --yes
	outYes, _, err := runCLI(t, "trigger", "delete", "7001", "--yes")
	if err != nil {
		t.Fatalf("unexpected error on delete with --yes: %v", err)
	}
	if !strings.Contains(outYes, "Deleted trigger 7001") {
		t.Errorf("expected success message, got: %s", outYes)
	}
	if n := h.count("trigger.delete"); n != 1 {
		t.Fatalf("expected 1 trigger.delete call, got %d", n)
	}
}
