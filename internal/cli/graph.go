package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"zx/internal/render"
	"zx/internal/timeparse"
	"zx/internal/zbxclient"
)

var (
	graphInputFile     string
	graphMetric        string
	graphRange         string
	graphFrom          string
	graphTo            string
	graphDays          int
	graphOutput        string
	graphWidth         int
	graphHeight        int
	graphBusinessHours bool
	graphHours         string
	graphWeekdaysOnly  bool
	graphPeak          bool
	graphHostgroup     string
)

func graphOutputPath(base, metric string, multi bool) string {
	if !multi {
		return base
	}
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext) + "_" + metric + ext
}

func summarize(samples []zbxclient.TrendSample, peak bool) (min, avg, max float64, n int) {
	for i, s := range samples {
		top := s.ValueAvg
		if peak {
			top = s.ValueMax
		}
		if i == 0 || s.ValueMin < min {
			min = s.ValueMin
		}
		if i == 0 || top > max {
			max = top
		}
		avg += s.ValueAvg
	}
	if n = len(samples); n > 0 {
		avg /= float64(n)
	}
	return min, avg, max, n
}

var exportGraphCmd = &cobra.Command{
	Use:   "export_graph [TARGET]",
	Short: "Export combined multi-host PNG graph from Zabbix web frontend",
	Long:  "Authenticate web session, sync timeline, and download high-resolution PNG chart for multiple hosts directly from chart.php.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, _, err := GetActiveClient()
		if err != nil {
			return err
		}

		var targets []string
		if len(args) > 0 && args[0] != "" {
			for _, p := range strings.Split(args[0], ",") {
				if t := strings.TrimSpace(p); t != "" {
					targets = append(targets, t)
				}
			}
		}

		if graphInputFile != "" {
			f, err := os.Open(graphInputFile)
			if err != nil {
				return fmt.Errorf("reading input file %s: %w", graphInputFile, err)
			}
			defer f.Close()

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				if t := strings.TrimSpace(scanner.Text()); t != "" && !strings.HasPrefix(t, "#") {
					targets = append(targets, t)
				}
			}
		}

		if len(targets) == 0 && strings.TrimSpace(graphHostgroup) == "" {
			return fmt.Errorf("no targets specified; provide a target argument, -i/--input-file, or --hostgroup")
		}

		metrics, err := zbxclient.ParseMetricList(graphMetric)
		if err != nil {
			return err
		}
		loc, err := Location()
		if err != nil {
			return err
		}
		filter, err := zbxclient.NewTrendFilter(graphBusinessHours, graphHours, graphWeekdaysOnly, loc)
		if err != nil {
			return err
		}
		start, end, err := timeparse.ParseRange(graphRange, graphFrom, graphTo, graphDays, time.Now(), loc)
		if err != nil {
			return err
		}

		hosts, ipMap, err := client.ResolveHostsFiltered(cmd.Context(), targets, splitCSV(graphHostgroup))
		if err != nil {
			return fmt.Errorf("resolving hosts: %w", err)
		}
		if len(hosts) == 0 {
			return fmt.Errorf("no hosts found matching targets")
		}

		itemsByHost := map[string][]zbxclient.ItemRecord{}
		for _, h := range hosts {
			var items []zbxclient.ItemRecord
			if err := client.Call(cmd.Context(), "item.get", map[string]any{
				"hostids": h.HostID, "output": []string{"itemid", "name", "key_"},
			}, &items); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: item.get failed for %s: %v\n", h.Name, err)
				continue
			}
			itemsByHost[h.HostID] = items
		}

		multi := len(metrics) > 1
		var graphsJSON []map[string]any
		for _, m := range metrics {
			var itemIDs []string
			var rows []render.MetricSummaryRow
			for _, h := range hosts {
				it, ok := m.FindItem(itemsByHost[h.HostID])
				if !ok {
					continue
				}
				itemIDs = append(itemIDs, it.ItemID)
				row := render.MetricSummaryRow{Host: h.Name, IP: ipMap[h.HostID]}
				trends, err := client.FetchTrends(cmd.Context(), it.ItemID, start.Unix, end.Unix)
				if err != nil {
					row.Error = err.Error()
				} else {
					row.Min, row.Avg, row.Max, row.Samples = summarize(zbxclient.FilterTrendSamples(trends, filter), graphPeak)
				}
				rows = append(rows, row)
			}
			if len(itemIDs) == 0 {
				if !multi {
					return fmt.Errorf("no matching metric items found for metric '%s'", m.Name)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: no items for metric '%s', skipped\n", m.Name)
				continue
			}
			out := graphOutputPath(graphOutput, m.Name, multi)
			fmt.Fprintf(cmd.ErrOrStderr(), "Exporting %s graph for %d hosts (%d items) from '%s' to '%s' -> %s...\n",
				m.Name, len(hosts), len(itemIDs), start.Zabbix, end.Zabbix, out)
			if err := client.DownloadCombinedGraph(cmd.Context(), itemIDs, start.Zabbix, end.Zabbix, graphWidth, graphHeight, out); err != nil {
				return fmt.Errorf("export %s graph failed: %w", m.Name, err)
			}
			if OutputFormat() == "json" {
				graphsJSON = append(graphsJSON, map[string]any{"metric": m.Name, "output": out, "items": itemIDs, "summary": rows})
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Graph successfully saved: %s (%dx%d px)\n", out, graphWidth, graphHeight)
			render.RenderMetricSummary(cmd.OutOrStdout(),
				fmt.Sprintf("THỐNG KÊ %s (%s → %s)", m.Label, start.Zabbix, end.Zabbix), rows)
		}
		if OutputFormat() == "json" {
			return writeJSON(cmd.OutOrStdout(), map[string]any{"graphs": graphsJSON})
		}
		return nil
	},
}

func init() {
	exportGraphCmd.Flags().StringVarP(&graphInputFile, "input-file", "i", "", "path to file containing list of IPs or hostnames")
	exportGraphCmd.Flags().StringVarP(&graphMetric, "metric", "m", "ram", "ram, cpu, load, free_swap, iowait, or all (comma-separated allowed)")
	exportGraphCmd.Flags().StringVarP(&graphRange, "time-range", "r", "", "time range e.g. 'now-7d..now'")
	exportGraphCmd.Flags().StringVarP(&graphFrom, "from", "f", "", "start time (e.g. 'now-7d', '2026-03-01 00:00')")
	exportGraphCmd.Flags().StringVarP(&graphTo, "to", "t", "", "end time (e.g. 'now', '2026-03-08 00:00')")
	exportGraphCmd.Flags().IntVarP(&graphDays, "days", "d", 7, "number of days back if from/to/range are not set")
	exportGraphCmd.Flags().StringVarP(&graphOutput, "output", "o", "zabbix_graph.png", "output PNG file path")
	exportGraphCmd.Flags().IntVarP(&graphWidth, "width", "W", 2050, "image width in pixels")
	exportGraphCmd.Flags().IntVarP(&graphHeight, "height", "H", 368, "image height in pixels")

	exportGraphCmd.Flags().BoolVar(&graphBusinessHours, "business-hours", false, "filter Monday-Friday 08:00-12:00, 13:00-17:00 for summary table")
	exportGraphCmd.Flags().StringVar(&graphHours, "hours", "", "hour ranges for summary table e.g. '8-12,13-17' (only affects summary table, not graph)")
	exportGraphCmd.Flags().BoolVar(&graphWeekdaysOnly, "weekdays-only", false, "only Monday-Friday samples for summary table")
	exportGraphCmd.Flags().BoolVar(&graphPeak, "peak", false, "use 1-sample raw instant peak instead of peak hourly average")
	exportGraphCmd.Flags().StringVar(&graphHostgroup, "hostgroup", "", "filter hosts by hostgroup name or ID (comma-separated)")

	rootCmd.AddCommand(exportGraphCmd)
}
