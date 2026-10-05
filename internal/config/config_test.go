package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndSaveConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := &Config{
		ActiveProfile: "central",
		Defaults: DefaultsConfig{
			TimeoutSeconds: 30,
			Concurrency:    10,
			BusinessHours:  "08:00-12:00,13:00-17:00",
		},
		Profiles: map[string]Profile{
			"central": {
				URL:       "http://zabbix-central.example.com/zabbix",
				Token:     "test_token_123",
				VerifySSL: true,
			},
			"staging": {
				URL:       "http://192.0.2.1/zabbix",
				User:      "admin",
				Password:  "secret_pass",
				VerifySSL: false,
			},
		},
	}

	if err := cfg.SaveTo(configPath); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected file mode 0600, got %o", perm)
	}

	loaded, err := LoadConfigFrom(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFrom failed: %v", err)
	}

	if loaded.ActiveProfile != "central" {
		t.Errorf("expected active profile central, got %s", loaded.ActiveProfile)
	}

	p, name, err := loaded.GetActiveProfile()
	if err != nil {
		t.Fatalf("GetActiveProfile failed: %v", err)
	}
	if name != "central" || p.URL != "http://zabbix-central.example.com/zabbix" {
		t.Errorf("unexpected profile data: %s, %+v", name, p)
	}
}

func TestMaskedPasswordAndToken(t *testing.T) {
	p1 := Profile{User: "admin", Password: "super_secret_password"}
	if p1.MaskedPassword() != "***" {
		t.Errorf("expected ***, got %s", p1.MaskedPassword())
	}

	p2 := Profile{Token: "0123456789abcdef"}
	if p2.MaskedToken() != "01234567***" {
		t.Errorf("expected masked token starting with prefix, got %s", p2.MaskedToken())
	}
}
