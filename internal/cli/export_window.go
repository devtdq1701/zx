package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"zx/internal/zbxclient"
)

var (
	windowFrom     string
	windowTo       string
	windowRole     string
	windowInput    string
	windowHours    string
	windowBusiness bool
	windowWeekdays bool
	windowPeak     bool
)

func parseWindowTime(s string, loc *time.Location, now time.Time) (int64, error) {
	text := strings.TrimSpace(s)
	if text == "" || strings.ContainsAny(s, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7f") {
		return 0, fmt.Errorf("time contains control characters or is empty")
	}
	if text == "now" {
		return now.Unix(), nil
	}
	if t, err := time.Parse(time.RFC3339, text); err == nil {
		return t.Unix(), nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, text, loc); err == nil {
			return t.Unix(), nil
		}
	}
	return 0, fmt.Errorf("invalid time '%s'", s)
}

func validateWindowArgs(role string, since, until int64) error {
	if role != "app" && role != "db" {
		return fmt.Errorf("role must be app or db")
	}
	if since >= until {
		return fmt.Errorf("from must be earlier than to")
	}
	return nil
}

func readTargets(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("input file '%s' does not exist", path)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out, sc.Err()
}

var exportWindowCmd = &cobra.Command{
	Use:          "export_window",
	Short:        "Export exact-range host metrics as a machine-readable JSON document",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		loc, err := Location()
		if err != nil {
			return err
		}
		now := time.Now()
		since, err := parseWindowTime(windowFrom, loc, now)
		if err != nil {
			return err
		}
		until, err := parseWindowTime(windowTo, loc, now)
		if err != nil {
			return err
		}
		if err := validateWindowArgs(windowRole, since, until); err != nil {
			return err
		}
		filter, err := zbxclient.NewTrendFilter(windowBusiness, windowHours, windowWeekdays, loc)
		if err != nil {
			return err
		}
		targets, err := readTargets(windowInput)
		if err != nil {
			return err
		}
		client, _, _, err := GetActiveClient()
		if err != nil {
			return err
		}
		doc := client.ExportWindow(cmd.Context(), zbxclient.WindowRequest{
			From: windowFrom, To: windowTo, Since: since, Until: until, Timezone: loc.String(),
			Role: windowRole, Targets: targets, Filter: filter, BusinessHours: windowBusiness,
			Hours: windowHours, Peak: windowPeak, AdapterVersion: Version,
		})
		return writeJSON(cmd.OutOrStdout(), doc)
	},
}

func init() {
	f := exportWindowCmd.Flags()
	f.StringVar(&windowFrom, "from", "", "window start (RFC3339, or naive local time in --timezone)")
	f.StringVar(&windowTo, "to", "", "window end (exclusive)")
	f.StringVar(&windowRole, "role", "", "target role: app or db")
	f.StringVarP(&windowInput, "input-file", "i", "", "file with one Zabbix host/IP per line")
	f.BoolVar(&windowBusiness, "business-hours", false, "Monday-Friday 08:00-12:00, 13:00-17:00")
	f.StringVar(&windowHours, "hours", "", "hour ranges e.g. '8-12,13-17'")
	f.BoolVar(&windowWeekdays, "weekdays-only", false, "only Monday-Friday samples")
	f.BoolVar(&windowPeak, "peak", false, "peak = max(value_max) instead of max(value_avg)")
	for _, name := range []string{"from", "to", "role", "input-file"} {
		_ = exportWindowCmd.MarkFlagRequired(name)
	}
	rootCmd.AddCommand(exportWindowCmd)
}
