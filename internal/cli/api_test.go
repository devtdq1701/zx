package cli

import (
	"strings"
	"testing"
)

func TestAPIInfoVersionDirect(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version": "7.4.0",
	})
	defer cleanup()

	out, _, err := runCLI(t, "api", "apiinfo.version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"7.4.0"`) {
		t.Fatalf("expected version 7.4.0 in output, got: %s", out)
	}
	if n := h.count("apiinfo.version"); n != 1 {
		t.Fatalf("expected 1 apiinfo.version call, got %d", n)
	}
}

func TestAPIReadMethodWithParams(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"history.get": []any{
			map[string]any{"clock": "12345", "value": "10"},
		},
	})
	defer cleanup()

	out, _, err := runCLI(t, "api", "history.get", `{"itemids":["101"]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"value": "10"`) {
		t.Fatalf("expected value 10 in output, got: %s", out)
	}
	if n := h.count("history.get"); n != 1 {
		t.Fatalf("expected 1 history.get call, got %d", n)
	}
}

func TestAPIMutationDefaultsToDryRun(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{})
	defer cleanup()

	out, _, err := runCLI(t, "api", "host.delete", `["10001"]`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[DRY-RUN]") || !strings.Contains(out, "host.delete") {
		t.Fatalf("expected dry-run preview, got: %s", out)
	}
	if n := h.count("host.delete"); n != 0 {
		t.Fatalf("dry-run must not send host.delete, got %d calls", n)
	}
}

func TestAPIMutationYesExecutes(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"host.delete": map[string]any{"hostids": []string{"10001"}},
	})
	defer cleanup()

	out, _, err := runCLI(t, "api", "host.delete", `["10001"]`, "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"hostids"`) {
		t.Fatalf("expected response with hostids, got: %s", out)
	}
	if n := h.count("host.delete"); n != 1 {
		t.Fatalf("want 1 host.delete call, got %d", n)
	}
}

func TestAPIInvalidJSONParams(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{})
	defer cleanup()

	_, _, err := runCLI(t, "api", "history.get", `{"invalid json`)
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("expected invalid JSON error, got: %v", err)
	}
	if n := h.count("history.get"); n != 0 {
		t.Fatalf("must not call API when JSON invalid, got %d calls", n)
	}
}
