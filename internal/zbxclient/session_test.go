package zbxclient

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "zx-session-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	profName := "test_prof"
	testToken := "abcdef0123456789abcdef0123456789"

	// 1. Initial load should be empty
	if tok := LoadSessionToken(profName); tok != "" {
		t.Fatalf("expected empty token, got %s", tok)
	}

	// 2. Save token
	if err := SaveSessionToken(profName, testToken); err != nil {
		t.Fatalf("SaveSessionToken failed: %v", err)
	}

	// 3. Verify file permissions 0600
	sessionPath := filepath.Join(tempDir, ".config", "zx", "sessions", profName+".json")
	fi, err := os.Stat(sessionPath)
	if err != nil {
		t.Fatalf("stat session file failed: %v", err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Errorf("expected 0600 permissions, got %o", fi.Mode().Perm())
	}

	// 4. Load token back
	if tok := LoadSessionToken(profName); tok != testToken {
		t.Fatalf("expected %s, got %s", testToken, tok)
	}

	// 5. Clear token
	if err := ClearSessionToken(profName); err != nil {
		t.Fatalf("ClearSessionToken failed: %v", err)
	}
	if tok := LoadSessionToken(profName); tok != "" {
		t.Fatalf("expected empty token after clear, got %s", tok)
	}
}
