package cli

import (
	"strings"
	"testing"
)

func TestHTTPTestList(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"httptest.get": []any{
			map[string]any{
				"httptestid": "101",
				"name":       "Check Login Page",
				"delay":      "1m",
				"status":     "0",
			},
		},
	})
	defer cleanup()

	out, _, err := runCLI(t, "httptest", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "101") || !strings.Contains(out, "Check Login Page") {
		t.Fatalf("expected httptest list in table output, got: %s", out)
	}
	if n := h.count("httptest.get"); n != 1 {
		t.Fatalf("expected exactly 1 httptest.get call, got %d", n)
	}
}

func TestHTTPTestCreateDryRun(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"host.get": []any{
			map[string]any{"hostid": "10001", "host": "web01"},
		},
	})
	defer cleanup()

	out, _, err := runCLI(t, "httptest", "create", "--host", "web01", "--name", "Check Web", "--url", "http://web01:3080/login.jsp")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[DRY-RUN]") || !strings.Contains(out, "httptest.create") {
		t.Fatalf("expected dry-run preview, got: %s", out)
	}
	if n := h.count("httptest.create"); n != 0 {
		t.Fatalf("dry-run must not send httptest.create, got %d calls", n)
	}
}

func TestHTTPTestCreateYes(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"host.get": []any{
			map[string]any{"hostid": "10001", "host": "web01"},
		},
		"httptest.create": map[string]any{"httptestids": []string{"101"}},
	})
	defer cleanup()

	out, _, err := runCLI(t, "httptest", "create", "--host", "web01", "--name", "Check Web", "--url", "http://web01:3080/login.jsp", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Created web scenario 'Check Web'") {
		t.Fatalf("expected success message, got: %s", out)
	}
	if n := h.count("httptest.create"); n != 1 {
		t.Fatalf("want 1 httptest.create call, got %d", n)
	}
}
