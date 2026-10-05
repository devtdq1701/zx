package zbxclient

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type ImportConfigParams struct {
	FilePath       string
	Source         string
	Format         string
	UpdateExisting bool
	CreateMissing  bool
	DeleteMissing  bool
}

type ImportResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func DetectFormat(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	default:
		return ""
	}
}

func ValidateLocalFormat(data []byte, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "yaml", "yml":
		var y any
		if err := yaml.Unmarshal(data, &y); err != nil {
			return fmt.Errorf("invalid YAML syntax: %w", err)
		}
	case "json":
		var j any
		if err := json.Unmarshal(data, &j); err != nil {
			return fmt.Errorf("invalid JSON syntax: %w", err)
		}
	case "xml":
		var x any
		if err := xml.Unmarshal(data, &x); err != nil {
			return fmt.Errorf("invalid XML syntax: %w", err)
		}
	default:
		return fmt.Errorf("unsupported format %q (expected yaml, json, or xml)", format)
	}
	return nil
}

func BuildImportRules(atLeast62 bool, updateExisting, createMissing, deleteMissing bool) map[string]any {
	rules := map[string]any{
		"templates": map[string]any{
			"createMissing":  createMissing,
			"updateExisting": updateExisting,
		},
		"items": map[string]any{
			"createMissing":  createMissing,
			"updateExisting": updateExisting,
			"deleteMissing":  deleteMissing,
		},
		"triggers": map[string]any{
			"createMissing":  createMissing,
			"updateExisting": updateExisting,
			"deleteMissing":  deleteMissing,
		},
		"discoveryRules": map[string]any{
			"createMissing":  createMissing,
			"updateExisting": updateExisting,
			"deleteMissing":  deleteMissing,
		},
		"httptests": map[string]any{
			"createMissing":  createMissing,
			"updateExisting": updateExisting,
			"deleteMissing":  deleteMissing,
		},
		"valueMaps": map[string]any{
			"createMissing":  createMissing,
			"updateExisting": updateExisting,
		},
	}

	if atLeast62 {
		rules["template_groups"] = map[string]any{
			"createMissing":  createMissing,
			"updateExisting": updateExisting,
		}
		rules["host_groups"] = map[string]any{
			"createMissing":  createMissing,
			"updateExisting": updateExisting,
		}
		rules["template_dashboards"] = map[string]any{
			"createMissing":  createMissing,
			"updateExisting": updateExisting,
		}
	} else {
		rules["groups"] = map[string]any{
			"createMissing":  createMissing,
			"updateExisting": updateExisting,
		}
	}

	return rules
}

func (c *Client) BuildImportPayload(ctx context.Context, params ImportConfigParams) (map[string]any, error) {
	var data []byte
	if params.Source != "" {
		data = []byte(params.Source)
	} else if params.FilePath != "" {
		var err error
		data, err = os.ReadFile(params.FilePath)
		if err != nil {
			return nil, fmt.Errorf("reading configuration file: %w", err)
		}
	} else {
		return nil, errors.New("no source or file path provided")
	}

	format := strings.ToLower(strings.TrimSpace(params.Format))
	if format == "" && params.FilePath != "" {
		format = DetectFormat(params.FilePath)
	}
	if format == "" {
		format = "yaml"
	}

	if err := ValidateLocalFormat(data, format); err != nil {
		return nil, err
	}

	atLeast62, err := c.APIAtLeast(ctx, 6, 2)
	if err != nil {
		return nil, err
	}

	rules := BuildImportRules(atLeast62, params.UpdateExisting, params.CreateMissing, params.DeleteMissing)

	return map[string]any{
		"format": format,
		"source": string(data),
		"rules":  rules,
	}, nil
}

func (c *Client) ImportConfiguration(ctx context.Context, params ImportConfigParams) (*ImportResult, error) {
	payload, err := c.BuildImportPayload(ctx, params)
	if err != nil {
		return nil, err
	}

	var res any
	if err := c.Call(ctx, "configuration.import", payload, &res); err != nil {
		return nil, fmt.Errorf("configuration.import: %w", err)
	}

	return &ImportResult{Success: true}, nil
}
