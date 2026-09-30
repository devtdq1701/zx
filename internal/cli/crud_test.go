package cli

import (
	"strings"
	"testing"
)

func TestCRUDDeleteDefaultsToDryRun(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{})
	defer cleanup()
	out, _, err := runCLI(t, "hostgroup", "delete", "42")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[DRY-RUN]") || !strings.Contains(out, "hostgroup.delete") {
		t.Fatalf("expected dry-run preview, got %q", out)
	}
	if n := h.count("hostgroup.delete"); n != 0 {
		t.Fatalf("dry-run sent %d hostgroup.delete", n)
	}
}

func TestCRUDDeleteYesSendsOnce(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{"usermacro.delete": map[string]any{"hostmacroids": []string{"7"}}})
	defer cleanup()
	if _, _, err := runCLI(t, "macro", "delete", "7", "--yes"); err != nil {
		t.Fatal(err)
	}
	if n := h.count("usermacro.delete"); n != 1 {
		t.Fatalf("want exactly one usermacro.delete, got %d", n)
	}
}

func TestCRUDDeleteRejectsNonNumericID(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{})
	defer cleanup()
	if _, _, err := runCLI(t, "user", "delete", "Admin", "--yes"); err == nil || !strings.Contains(err.Error(), "invalid user ID") {
		t.Fatalf("got %v", err)
	}
	if n := h.count("user.delete"); n != 0 {
		t.Fatalf("user.delete sent %d times", n)
	}
}

func TestCRUDProblemHasNoDelete(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Name() != "problem" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Name() == "delete" {
				t.Fatal("problem must not have a delete subcommand (no problem.delete API)")
			}
		}
		return
	}
	t.Fatal("problem command not registered")
}
