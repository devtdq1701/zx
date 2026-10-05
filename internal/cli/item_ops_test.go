package cli

import (
	"strings"
	"testing"
)

func TestCLIItemCreateDryRunAndYes(t *testing.T) {
	mockHosts := []map[string]any{
		{"hostid": "10050", "host": "hni-srv01", "name": "HNI Server 01"},
	}
	h, cleanup := setupMockClient(t, map[string]any{
		"host.get": mockHosts,
		"item.create": map[string]any{
			"itemids": []string{"50001"},
		},
	})
	defer cleanup()

	// 1. Dry run without --yes
	out, _, err := runCLI(t, "item", "create",
		"--host", "hni-srv01",
		"--name", "Convert Process Max Runtime",
		"--key", "convert.process.max_runtime",
		"--type", "zabbix_active",
		"--value-type", "unsigned",
		"--delay", "1m",
		"--units", "s",
	)
	if err != nil {
		t.Fatalf("unexpected error on dry run: %v", err)
	}
	if !strings.Contains(out, "[DRY-RUN]") || !strings.Contains(out, "item.create") {
		t.Errorf("expected [DRY-RUN] preview, got: %s", out)
	}
	if !strings.Contains(out, `"hostid": "10050"`) {
		t.Errorf("expected hostid 10050 in payload, got: %s", out)
	}
	if !strings.Contains(out, `"key_": "convert.process.max_runtime"`) {
		t.Errorf("expected key_ in payload, got: %s", out)
	}
	if !strings.Contains(out, `"type": 7`) {
		t.Errorf("expected type 7 in payload, got: %s", out)
	}
	if !strings.Contains(out, `"value_type": 3`) {
		t.Errorf("expected value_type 3 in payload, got: %s", out)
	}
	if n := h.count("item.create"); n != 0 {
		t.Fatalf("dry-run should not call item.create, got %d calls", n)
	}

	// 2. Execution with --yes
	outYes, _, err := runCLI(t, "item", "create",
		"--host", "hni-srv01",
		"--name", "Convert Process Max Runtime",
		"--key", "convert.process.max_runtime",
		"--type", "zabbix_active",
		"--value-type", "unsigned",
		"--delay", "1m",
		"--units", "s",
		"--yes",
	)
	if err != nil {
		t.Fatalf("unexpected error with --yes: %v", err)
	}
	if strings.Contains(outYes, "[DRY-RUN]") {
		t.Errorf("did not expect [DRY-RUN] with --yes")
	}
	if !strings.Contains(outYes, "Created item 50001 (Convert Process Max Runtime)") {
		t.Errorf("expected success message with ID 50001, got: %s", outYes)
	}
	if n := h.count("item.create"); n != 1 {
		t.Fatalf("expected 1 item.create call, got %d", n)
	}
}

func TestCLIItemUpdateDryRunAndYes(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"item.update": map[string]any{
			"itemids": []string{"50001"},
		},
	})
	defer cleanup()

	// 1. Dry run without --yes
	out, _, err := runCLI(t, "item", "update", "50001",
		"--delay", "30s",
		"--status", "active",
	)
	if err != nil {
		t.Fatalf("unexpected error on dry run: %v", err)
	}
	if !strings.Contains(out, "[DRY-RUN]") || !strings.Contains(out, "item.update") {
		t.Errorf("expected [DRY-RUN] preview, got: %s", out)
	}
	if !strings.Contains(out, `"itemid": "50001"`) {
		t.Errorf("expected itemid 50001 in payload, got: %s", out)
	}
	if !strings.Contains(out, `"delay": "30s"`) {
		t.Errorf("expected delay 30s in payload, got: %s", out)
	}
	if !strings.Contains(out, `"status": 0`) {
		t.Errorf("expected status 0 in payload, got: %s", out)
	}
	if n := h.count("item.update"); n != 0 {
		t.Fatalf("dry-run should not call item.update, got %d calls", n)
	}

	// 2. Execution with --yes
	outYes, _, err := runCLI(t, "item", "update", "50001",
		"--delay", "30s",
		"--status", "active",
		"--yes",
	)
	if err != nil {
		t.Fatalf("unexpected error with --yes: %v", err)
	}
	if strings.Contains(outYes, "[DRY-RUN]") {
		t.Errorf("did not expect [DRY-RUN] with --yes")
	}
	if !strings.Contains(outYes, "Updated item 50001") {
		t.Errorf("expected success message with ID 50001, got: %s", outYes)
	}
	if n := h.count("item.update"); n != 1 {
		t.Fatalf("expected 1 item.update call, got %d", n)
	}
}

func TestCLIItemUpdateValidation(t *testing.T) {
	_, cleanup := setupMockClient(t, map[string]any{})
	defer cleanup()

	// Missing ID
	_, _, err := runCLI(t, "item", "update")
	if err == nil {
		t.Errorf("expected error when item ID is missing")
	}

	// Missing update flags
	_, _, err = runCLI(t, "item", "update", "50001")
	if err == nil {
		t.Errorf("expected error when no update flags specified")
	}
}
