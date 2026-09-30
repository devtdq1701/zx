package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteLineResetsYesBetweenCommands(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version": "7.4.0",
		"host.get":        []map[string]any{{"hostid": "111", "host": "eofhni1", "name": "eofhni1"}},
		"host.update":     map[string]any{"hostids": []string{"111"}},
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	defer rootCmd.SetOut(nil)
	if err := executeLine(context.Background(), []string{"monitor_host", "eofhni1", "--status", "unmonitored", "--yes"}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	// Same process, next REPL line, no --yes: must be a dry-run.
	if err := executeLine(context.Background(), []string{"monitor_host", "eofhni1", "--status", "unmonitored"}); err != nil {
		t.Fatal(err)
	}
	if n := h.count("host.update"); n != 1 {
		t.Fatalf("host.update sent %d times; the second command must not inherit --yes", n)
	}
	if !strings.Contains(out.String(), "[DRY-RUN]") {
		t.Fatalf("expected dry-run output, got %q", out.String())
	}
}

func writeTwoProfileConfig(t *testing.T, urlA, urlB string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	body := "active_profile: a\nprofiles:\n  a:\n    url: " + urlA + "\n    token: ta\n  b:\n    url: " + urlB + "\n    token: tb\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProfileFlagIsHonouredOnEveryExecution(t *testing.T) {
	ha := &rpcHandler{resps: map[string]any{"apiinfo.version": "7.4.0"}}
	hb := &rpcHandler{resps: map[string]any{"apiinfo.version": "7.4.0"}}
	tsA, tsB := httptestServer(t, ha), httptestServer(t, hb)
	cfg := writeTwoProfileConfig(t, tsA.URL, tsB.URL)
	resetFlags(rootCmd)
	defer resetFlags(rootCmd)

	if _, _, err := runCLI(t, "--config", cfg, "--profile", "a", "--format", "json", "show_hosts", "x"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runCLI(t, "--config", cfg, "--profile", "b", "--format", "json", "show_hosts", "x"); err != nil {
		t.Fatal(err)
	}
	if hb.count("host.get") == 0 {
		t.Fatalf("second command with --profile b never reached profile b (calls a=%v b=%v)", ha.calls, hb.calls)
	}
}

func TestUnknownExplicitProfileFails(t *testing.T) {
	cfg := writeTwoProfileConfig(t, "http://127.0.0.1:1", "http://127.0.0.1:1")
	_, _, err := runCLI(t, "--config", cfg, "--profile", "nope", "show_hosts", "x")
	if err == nil || !strings.Contains(err.Error(), "profile 'nope'") {
		t.Fatalf("expected unknown profile error, got %v", err)
	}
}

func TestFormatFlagDoesNotLeak(t *testing.T) {
	_, _, _ = runCLI(t, "--config", "/nonexistent-zx-test.yaml", "--format", "xml", "show_hosts", "x")
	_, cleanup := setupMockClient(t, map[string]any{"apiinfo.version": "7.4.0"})
	defer cleanup()
	if _, _, err := runCLI(t, "--format", "json", "show_hosts", "x"); err != nil {
		t.Fatalf("format leaked from previous run: %v", err)
	}
	if _, _, err := runCLI(t, "show_hosts", "x"); err != nil && strings.Contains(err.Error(), "invalid --format") {
		t.Fatalf("format leaked from previous run: %v", err)
	}
}
