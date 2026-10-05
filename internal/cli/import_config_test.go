package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportConfigDryRun(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version": "7.4.0",
	})
	defer cleanup()

	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "template.yaml")
	content := "zabbix_export:\n  version: '7.4'\n"
	if err := os.WriteFile(yamlFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"import_configuration", "--file", yamlFile})

	err := rootCmd.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[DRY-RUN]") {
		t.Errorf("expected [DRY-RUN] in output, got: %s", out)
	}
	if !strings.Contains(out, "configuration.import") {
		t.Errorf("expected configuration.import in output, got: %s", out)
	}
	if !strings.Contains(out, "template_groups") {
		t.Errorf("expected template_groups in dry-run rules, got: %s", out)
	}

	// Verify configuration.import was NOT called on server
	if n := h.count("configuration.import"); n != 0 {
		t.Errorf("configuration.import should not be called in dry-run, got %d calls", n)
	}
}

func TestImportConfigWithYes(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version":      "7.4.0",
		"configuration.import": true,
	})
	defer cleanup()

	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "template.yaml")
	content := "zabbix_export:\n  version: '7.4'\n"
	if err := os.WriteFile(yamlFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"import_configuration", "--file", yamlFile, "--yes"})

	err := rootCmd.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "[DRY-RUN]") {
		t.Errorf("did not expect [DRY-RUN] with --yes")
	}
	if !strings.Contains(out, "successfully") {
		t.Errorf("expected success message, got: %s", out)
	}

	if n := h.count("configuration.import"); n != 1 {
		t.Fatalf("expected 1 configuration.import call, got %d", n)
	}

	params := h.last("configuration.import")
	if params["format"] != "yaml" {
		t.Errorf("expected format yaml, got %v", params["format"])
	}
	if params["source"] != content {
		t.Errorf("expected source to match file content")
	}
}

func TestImportConfigInvalidYAML(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version": "7.4.0",
	})
	defer cleanup()

	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.yaml")
	badContent := "zabbix_export:\n  version: '7.4'\n\ttemplates:\n - bad\n"
	if err := os.WriteFile(badFile, []byte(badContent), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"import_configuration", "--file", badFile})

	err := rootCmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatalf("expected error on invalid YAML, got nil")
	}

	// Ensure no API calls were made to Zabbix
	if len(h.calls) != 0 {
		t.Errorf("expected 0 API calls on syntax error, got: %v", h.calls)
	}
}

func TestImportConfigWithFlags(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version":      "7.4.0",
		"configuration.import": true,
	})
	defer cleanup()

	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "template.json")
	content := `{"zabbix_export": {"version": "7.4"}}`
	if err := os.WriteFile(jsonFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"import_configuration", "-f", jsonFile, "--format", "json", "--delete-missing", "--yes"})

	err := rootCmd.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := h.last("configuration.import")
	if params["format"] != "json" {
		t.Errorf("expected format json, got %v", params["format"])
	}
	rules, ok := params["rules"].(map[string]any)
	if !ok {
		t.Fatalf("rules not a map: %v", params["rules"])
	}
	items, ok := rules["items"].(map[string]any)
	if !ok || items["deleteMissing"] != true {
		t.Errorf("expected items.deleteMissing to be true, got %v", items["deleteMissing"])
	}
}
