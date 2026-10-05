package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"zx/internal/timeparse"
)

var (
	historyLimit int
	historyType  string
	historySince string
	historyUntil string

	historyCmd = &cobra.Command{
		Use:   "history",
		Short: "Manage and query Zabbix history data points",
	}

	historyGetCmd = &cobra.Command{
		Use:   "get [ITEM_ID]",
		Short: "Get raw history data points or logs for an item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := strings.TrimSpace(args[0])
			if _, err := strconv.ParseUint(itemID, 10, 64); err != nil {
				return fmt.Errorf("invalid item ID '%s'", itemID)
			}

			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}

			valType := -1
			if strings.TrimSpace(historyType) != "" {
				switch strings.ToLower(historyType) {
				case "0", "float", "numeric_float":
					valType = 0
				case "1", "str", "string", "char":
					valType = 1
				case "2", "log":
					valType = 2
				case "3", "uint", "numeric_unsigned", "unsigned":
					valType = 3
				case "4", "text":
					valType = 4
				default:
					return fmt.Errorf("invalid --type '%s'; expected float, uint, str, log, or text", historyType)
				}
			} else {
				// Auto-detect item value_type
				var items []map[string]any
				err := client.Call(cmd.Context(), "item.get", map[string]any{
					"itemids": []string{itemID},
					"output":  []string{"itemid", "value_type"},
				}, &items)
				if err == nil && len(items) > 0 {
					if vtStr, ok := items[0]["value_type"].(string); ok {
						if vt, convErr := strconv.Atoi(vtStr); convErr == nil {
							valType = vt
						}
					}
				}
				if valType < 0 {
					valType = 3 // default to unsigned
				}
			}

			loc, locErr := time.LoadLocation(timezoneFlag)
			if locErr != nil {
				loc = time.Local
			}
			now := time.Now()

			params := map[string]any{
				"itemids":   []string{itemID},
				"history":   valType,
				"output":    "extend",
				"sortfield": "clock",
				"sortorder": "DESC",
				"limit":     historyLimit,
			}

			if historySince != "" {
				pt, err := timeparse.Parse(historySince, false, now, loc)
				if err != nil {
					return fmt.Errorf("invalid --since: %w", err)
				}
				params["time_from"] = pt.Unix
			}
			if historyUntil != "" {
				pt, err := timeparse.Parse(historyUntil, true, now, loc)
				if err != nil {
					return fmt.Errorf("invalid --until: %w", err)
				}
				params["time_till"] = pt.Unix
			}

			var records []map[string]any
			if err := client.Call(cmd.Context(), "history.get", params, &records); err != nil {
				return fmt.Errorf("history.get failed: %w", err)
			}

			if formatFlag == "json" {
				b, err := json.MarshalIndent(records, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			if len(records) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No history records found for item %s.\n", itemID)
				return nil
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
			if valType == 2 {
				fmt.Fprintln(tw, "TIMESTAMP\tSEVERITY\tLOG MESSAGE")
				fmt.Fprintln(tw, strings.Repeat("-", 70))
				for _, r := range records {
					clkStr, _ := r["clock"].(string)
					clk, _ := strconv.ParseInt(clkStr, 10, 64)
					tm := time.Unix(clk, 0).In(loc).Format("2006-01-02 15:04:05")
					sev, _ := r["severity"]
					val, _ := r["value"].(string)
					fmt.Fprintf(tw, "%s\t%v\t%s\n", tm, sev, val)
				}
			} else {
				fmt.Fprintln(tw, "TIMESTAMP\tVALUE")
				fmt.Fprintln(tw, strings.Repeat("-", 50))
				for _, r := range records {
					clkStr, _ := r["clock"].(string)
					clk, _ := strconv.ParseInt(clkStr, 10, 64)
					tm := time.Unix(clk, 0).In(loc).Format("2006-01-02 15:04:05")
					val, _ := r["value"]
					fmt.Fprintf(tw, "%s\t%v\n", tm, val)
				}
			}
			_ = tw.Flush()
			return nil
		},
	}
)

func init() {
	historyGetCmd.Flags().IntVar(&historyLimit, "limit", 50, "maximum records to return")
	historyGetCmd.Flags().StringVarP(&historyType, "type", "t", "", "value type: float, uint, str, log, text (auto-detected if omitted)")
	historyGetCmd.Flags().StringVar(&historySince, "since", "", "start time expression (e.g. 1h, now-1h, '2026-10-05 08:00:00')")
	historyGetCmd.Flags().StringVar(&historyUntil, "until", "", "end time expression (e.g. now, '2026-10-05 11:00:00')")

	historyCmd.AddCommand(historyGetCmd)
	rootCmd.AddCommand(historyCmd)
}
