package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatFlagValidation(t *testing.T) {
	_, _, err := runCLI(t, "--config", "/nonexistent-zx-test.yaml", "--format", "xml", "show_hosts", "x")
	if err == nil || !strings.Contains(err.Error(), "invalid --format") {
		t.Fatalf("expected invalid --format error, got %v", err)
	}
}

func TestWriteJSONSingleLine(t *testing.T) {
	var b bytes.Buffer
	if err := writeJSON(&b, map[string]any{"a": "<b>"}); err != nil {
		t.Fatal(err)
	}
	if b.String() != "{\"a\":\"<b>\"}\n" {
		t.Fatalf("got %q", b.String())
	}
}
