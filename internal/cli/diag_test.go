package cli

import (
	"strings"
	"testing"
)

func TestCLIMediaTypeTestDryRunAndYes(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"mediatype.test": map[string]any{
			"result": true,
			"error":  "",
		},
	})
	defer cleanup()

	// 1. Dry run without --yes using mediatype alias
	out, _, err := runCLI(t, "mediatype", "test",
		"--id", "104",
		"--sendto", "-5032963651",
		"--subject", "Test",
		"--message", "Ping",
	)
	if err != nil {
		t.Fatalf("unexpected error on dry run: %v", err)
	}
	if !strings.Contains(out, "[DRY-RUN]") || !strings.Contains(out, "mediatype.test") {
		t.Errorf("expected [DRY-RUN] preview with mediatype.test, got: %s", out)
	}
	if !strings.Contains(out, `"mediatypeid": "104"`) {
		t.Errorf("expected mediatypeid 104 in payload, got: %s", out)
	}
	if !strings.Contains(out, `"-5032963651"`) {
		t.Errorf("expected sendto in payload, got: %s", out)
	}
	if n := h.count("mediatype.test"); n != 0 {
		t.Fatalf("dry-run should not call mediatype.test, got %d calls", n)
	}

	// 2. Execution with --yes
	outYes, _, err := runCLI(t, "mediatype", "test",
		"--id", "104",
		"--sendto", "-5032963651",
		"--subject", "Test",
		"--message", "Ping",
		"--yes",
	)
	if err != nil {
		t.Fatalf("unexpected error with --yes: %v", err)
	}
	if strings.Contains(outYes, "[DRY-RUN]") {
		t.Errorf("did not expect [DRY-RUN] with --yes")
	}
	if !strings.Contains(outYes, "-5032963651") {
		t.Errorf("expected confirmation output, got: %s", outYes)
	}
	if n := h.count("mediatype.test"); n != 1 {
		t.Errorf("expected 1 call to mediatype.test, got %d", n)
	}

	// 3. Execution with media test alias
	_, _, err = runCLI(t, "media", "test",
		"--id", "104",
		"--sendto", "-5032963651",
		"--subject", "Test",
		"--message", "Ping",
		"--yes",
	)
	if err != nil {
		t.Fatalf("unexpected error on media test alias: %v", err)
	}
	if n := h.count("mediatype.test"); n != 2 {
		t.Errorf("expected 2 calls to mediatype.test, got %d", n)
	}
}

func TestCLIItemExecuteNowDryRunAndYes(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"task.create": map[string]any{
			"taskids": []string{"1"},
		},
	})
	defer cleanup()

	// 1. Dry run without --yes
	out, _, err := runCLI(t, "item", "execute_now",
		"--itemid", "12345",
	)
	if err != nil {
		t.Fatalf("unexpected error on dry run: %v", err)
	}
	if !strings.Contains(out, "[DRY-RUN]") || !strings.Contains(out, "task.create") {
		t.Errorf("expected [DRY-RUN] preview with task.create, got: %s", out)
	}
	if !strings.Contains(out, `"itemid": "12345"`) {
		t.Errorf("expected itemid 12345 in payload, got: %s", out)
	}
	if !strings.Contains(out, `"type": 6`) {
		t.Errorf("expected type 6 in payload, got: %s", out)
	}
	if n := h.count("task.create"); n != 0 {
		t.Fatalf("dry-run should not call task.create, got %d calls", n)
	}

	// 2. Execution with --yes
	outYes, _, err := runCLI(t, "item", "execute_now",
		"--itemid", "12345",
		"--yes",
	)
	if err != nil {
		t.Fatalf("unexpected error with --yes: %v", err)
	}
	if strings.Contains(outYes, "[DRY-RUN]") {
		t.Errorf("did not expect [DRY-RUN] with --yes")
	}
	if !strings.Contains(outYes, "12345") {
		t.Errorf("expected item id in output, got: %s", outYes)
	}
	if n := h.count("task.create"); n != 1 {
		t.Errorf("expected 1 call to task.create, got %d", n)
	}

	// 3. Execution with execute-now alias and positional argument
	_, _, err = runCLI(t, "item", "execute-now",
		"12345",
		"--yes",
	)
	if err != nil {
		t.Fatalf("unexpected error with execute-now alias: %v", err)
	}
	if n := h.count("task.create"); n != 2 {
		t.Errorf("expected 2 calls to task.create, got %d", n)
	}
}
