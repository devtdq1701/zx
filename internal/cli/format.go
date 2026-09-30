package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Version is set at build time via -ldflags.
var Version = "dev"

var (
	formatFlag   string
	timezoneFlag string
)

// OutputFormat returns "table" or "json".
func OutputFormat() string { return formatFlag }

// Location returns the configured --timezone location.
func Location() (*time.Location, error) {
	loc, err := time.LoadLocation(timezoneFlag)
	if err != nil {
		return nil, fmt.Errorf("unknown timezone '%s'", timezoneFlag)
	}
	return loc, nil
}

func validateFormat() error {
	if formatFlag != "table" && formatFlag != "json" {
		return fmt.Errorf("invalid --format '%s'; expected table or json", formatFlag)
	}
	return nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
