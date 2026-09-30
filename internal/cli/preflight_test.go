package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"zx/internal/config"
)

func withStatusServer(t *testing.T, code int) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
	}))
	t.Cleanup(ts.Close)
	prev := testProfile
	testProfile = &config.Profile{URL: ts.URL}
	t.Cleanup(func() { testProfile = prev })
}

func TestPreflightPassLine(t *testing.T) {
	for _, code := range []int{200, 412} {
		withStatusServer(t, code)
		out, errOut, err := runCLI(t, "preflight")
		if err != nil {
			t.Fatalf("%d: %v", code, err)
		}
		re := regexp.MustCompile(`^PASS endpoint=http://\S+/api_jsonrpc\.php http=\d{3} \(expected\) latency_ms=\d+\n$`)
		if !re.MatchString(out) {
			t.Fatalf("%d: stdout %q does not match skill contract", code, out)
		}
		if errOut != "" {
			t.Fatalf("%d: stderr must be empty, got %q", code, errOut)
		}
	}
}

func TestPreflightFailLine(t *testing.T) {
	withStatusServer(t, 500)
	out, errOut, err := runCLI(t, "preflight")
	if err == nil {
		t.Fatal("HTTP 500 must fail")
	}
	if out != "" {
		t.Fatalf("stdout must be empty on FAIL, got %q", out)
	}
	if !strings.HasPrefix(errOut, "FAIL endpoint=") || !strings.Contains(errOut, "http=500 (unexpected)") {
		t.Fatalf("stderr %q", errOut)
	}
	if strings.Count(errOut, "\n") != 1 {
		t.Fatalf("stderr must be exactly the FAIL line, got %q", errOut)
	}
}

func TestPreflightOtherErrorsStillPrinted(t *testing.T) {
	withStatusServer(t, 500)
	// A FAIL must not silence errors of later commands in the same process.
	if _, _, err := runCLI(t, "preflight"); err == nil {
		t.Fatal("HTTP 500 must fail")
	}
	prev := testProfile
	testProfile = nil
	t.Cleanup(func() { testProfile = prev })
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfg, []byte("profiles: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--config", cfg, "preflight"},
		{"--config", cfg, "--profile", "zz_nosuch", "preflight"},
	} {
		out, errOut, err := runCLI(t, args...)
		if err == nil || out != "" || strings.TrimSpace(errOut) == "" {
			t.Fatalf("%v: want error on stderr, got out=%q err=%q %v", args, out, errOut, err)
		}
	}
}
