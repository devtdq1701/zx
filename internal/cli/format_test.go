package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatFlagValidation(t *testing.T) {
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--config", "/nonexistent-zx-test.yaml", "--format", "xml", "show_hosts", "x"})
	err := root.Execute()
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
