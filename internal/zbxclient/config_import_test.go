package zbxclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"zx/internal/config"
)

func TestValidateLocalFormat(t *testing.T) {
	validYAML := []byte("zabbix_export:\n  version: '7.0'\n  templates:\n    - template: Test\n")
	if err := ValidateLocalFormat(validYAML, "yaml"); err != nil {
		t.Fatalf("expected valid YAML to pass, got: %v", err)
	}

	invalidYAML := []byte("zabbix_export:\n  version: '7.0'\n\ttemplates:\n - bad indentation\n")
	if err := ValidateLocalFormat(invalidYAML, "yaml"); err == nil {
		t.Fatalf("expected invalid YAML to fail, got nil")
	}

	validJSON := []byte(`{"zabbix_export": {"version": "7.0"}}`)
	if err := ValidateLocalFormat(validJSON, "json"); err != nil {
		t.Fatalf("expected valid JSON to pass, got: %v", err)
	}

	invalidJSON := []byte(`{"zabbix_export": invalid}`)
	if err := ValidateLocalFormat(invalidJSON, "json"); err == nil {
		t.Fatalf("expected invalid JSON to fail, got nil")
	}

	validXML := []byte(`<zabbix_export><version>7.0</version></zabbix_export>`)
	if err := ValidateLocalFormat(validXML, "xml"); err != nil {
		t.Fatalf("expected valid XML to pass, got: %v", err)
	}

	invalidXML := []byte(`<zabbix_export><unclosed>`)
	if err := ValidateLocalFormat(invalidXML, "xml"); err == nil {
		t.Fatalf("expected invalid XML to fail, got nil")
	}

	if err := ValidateLocalFormat(validYAML, "unknown"); err == nil {
		t.Fatalf("expected unsupported format to fail, got nil")
	}
}

func TestBuildImportRules(t *testing.T) {
	// Zabbix >= 6.2 (e.g. 7.4)
	rules74 := BuildImportRules(true, true, true, true)
	if _, ok := rules74["template_groups"]; !ok {
		t.Errorf("expected template_groups in >= 6.2 rules")
	}
	if _, ok := rules74["host_groups"]; !ok {
		t.Errorf("expected host_groups in >= 6.2 rules")
	}
	if _, ok := rules74["template_dashboards"]; !ok {
		t.Errorf("expected template_dashboards in >= 6.2 rules")
	}
	if _, ok := rules74["groups"]; ok {
		t.Errorf("did not expect groups in >= 6.2 rules")
	}
	itemsRule, ok := rules74["items"].(map[string]any)
	if !ok || itemsRule["deleteMissing"] != true {
		t.Errorf("expected items.deleteMissing to be true")
	}

	// Zabbix < 6.2 (e.g. 5.2)
	rules52 := BuildImportRules(false, true, true, false)
	if _, ok := rules52["groups"]; !ok {
		t.Errorf("expected groups in < 6.2 rules")
	}
	if _, ok := rules52["template_groups"]; ok {
		t.Errorf("did not expect template_groups in < 6.2 rules")
	}
	if _, ok := rules52["host_groups"]; ok {
		t.Errorf("did not expect host_groups in < 6.2 rules")
	}
	items52, ok := rules52["items"].(map[string]any)
	if !ok || items52["deleteMissing"] != false {
		t.Errorf("expected items.deleteMissing to be false")
	}
}

func TestImportConfiguration_RPC_Version74(t *testing.T) {
	var capturedMethod string
	var capturedParams map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			ID     int             `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedMethod = req.Method

		if req.Method == "apiinfo.version" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"result":  "7.4.0",
				"id":      req.ID,
			})
			return
		}
		if req.Method == "configuration.import" {
			_ = json.Unmarshal(req.Params, &capturedParams)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"result":  true,
				"id":      req.ID,
			})
			return
		}
		http.Error(w, "unknown method", http.StatusBadRequest)
	}))
	defer srv.Close()

	client := NewClient(&config.Profile{URL: srv.URL, Token: "test-token"}, 5*time.Second)

	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "template.yaml")
	yamlContent := "zabbix_export:\n  version: '7.4'\n"
	if err := os.WriteFile(validFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := client.ImportConfiguration(context.Background(), ImportConfigParams{
		FilePath:       validFile,
		UpdateExisting: true,
		CreateMissing:  true,
		DeleteMissing:  false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success true")
	}
	if capturedMethod != "configuration.import" {
		t.Errorf("expected last method configuration.import, got %s", capturedMethod)
	}
	if capturedParams["format"] != "yaml" {
		t.Errorf("expected format yaml, got %v", capturedParams["format"])
	}
	if capturedParams["source"] != yamlContent {
		t.Errorf("expected source to match yamlContent")
	}
	rules, ok := capturedParams["rules"].(map[string]any)
	if !ok || rules["template_groups"] == nil {
		t.Errorf("expected rules to contain template_groups for 7.4")
	}
}

func TestImportConfiguration_InvalidYAML_NoHTTPCall(t *testing.T) {
	httpCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalled = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(&config.Profile{URL: srv.URL, Token: "test-token"}, 5*time.Second)

	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.yaml")
	badContent := "zabbix_export:\n  version: '7.4'\n\ttemplates:\n - bad\n"
	if err := os.WriteFile(badFile, []byte(badContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := client.ImportConfiguration(context.Background(), ImportConfigParams{
		FilePath: badFile,
	})
	if err == nil {
		t.Fatalf("expected error on bad YAML, got nil")
	}
	if httpCalled {
		t.Fatalf("expected no HTTP call on invalid YAML syntax")
	}
}
