package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type DefaultsConfig struct {
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	Concurrency    int    `yaml:"concurrency"`
	BusinessHours  string `yaml:"business_hours"`
}

type Config struct {
	ActiveProfile string             `yaml:"active_profile"`
	Defaults      DefaultsConfig     `yaml:"defaults"`
	Profiles      map[string]Profile `yaml:"profiles"`
	filePath      string             `yaml:"-"`
}

func DefaultConfig() *Config {
	return &Config{
		ActiveProfile: "central",
		Defaults: DefaultsConfig{
			TimeoutSeconds: 30,
			Concurrency:    10,
			BusinessHours:  "08:00-12:00,13:00-17:00",
		},
		Profiles: make(map[string]Profile),
	}
}

func GetDefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "zx", "config.yaml"), nil
}

func LoadConfig() (*Config, error) {
	path, err := GetDefaultConfigPath()
	if err != nil {
		return nil, err
	}
	return LoadConfigFrom(path)
}

func LoadConfigFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			cfg.filePath = path
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	cfg.filePath = path
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}
	return &cfg, nil
}

func (c *Config) Save() error {
	if c.filePath == "" {
		p, err := GetDefaultConfigPath()
		if err != nil {
			return err
		}
		c.filePath = p
	}
	return c.SaveTo(c.filePath)
}

func (c *Config) SaveTo(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating dir %s: %w", dir, err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Chmod 0600 enforced
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}

func (c *Config) GetActiveProfile() (*Profile, string, error) {
	if c.ActiveProfile == "" {
		return nil, "", fmt.Errorf("no active profile set")
	}
	p, ok := c.Profiles[c.ActiveProfile]
	if !ok {
		return nil, "", fmt.Errorf("profile '%s' not found", c.ActiveProfile)
	}
	return &p, c.ActiveProfile, nil
}

func (c *Config) GetProfile(name string) (*Profile, error) {
	p, ok := c.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}
	return &p, nil
}
