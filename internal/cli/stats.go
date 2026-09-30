package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"zx/internal/render"
	"zx/internal/timeparse"
	"zx/internal/zbxclient"
)

var (
	statsInputFile     string
	statsDays          int
	statsBusinessHours bool
	statsPeak          bool
	statsConcurrency   int
	statsHours         string
	statsWeekdaysOnly  bool
	statsHostgroup     string
	statsRange         string
	statsFrom          string
	statsTo            string
)

func statsJSON(stats []zbxclient.HostStats, peak *zbxclient.ClusterPeak, start, end timeparse.Point,
	loc *time.Location, f zbxclient.TrendFilter, usePeak bool) map[string]any {
	hosts := make([]map[string]any, 0, len(stats))
	for _, s := range stats {
		hosts = append(hosts, map[string]any{
			"hostid": s.HostID, "host": s.HostName, "ip": s.IP, "cpu_cores": s.CPUCores,
			"ram_total_gb": s.RAMTotalGB, "cpu_avg": s.CPUAvg, "cpu_max": s.CPUMax,
			"ram_avg": s.RAMAvg, "ram_max": s.RAMMax, "load_avg": s.LoadAvg, "load_max": s.LoadMax,
			"error": s.Error,
		})
	}
	var cp any
	if peak != nil {
		cp = map[string]any{"time": peak.Time.In(loc).Format(time.RFC3339), "cpu_avg": peak.CPUAvg,
			"ram_avg": peak.RAMAvg, "load_avg": peak.LoadAvg}
	}
	return map[string]any{
		"window":       map[string]any{"from": start.Zabbix, "to": end.Zabbix, "since": start.Unix, "until": end.Unix, "timezone": loc.String()},
		"filters":      map[string]any{"hour_ranges": f.HourRanges, "weekdays_only": f.WeekdaysOnly, "peak": usePeak},
		"hosts":        hosts,
		"cluster_peak": cp,
	}
}

var showHostStatsCmd = &cobra.Command{
	Use:   "show_host_stats [TARGET]",
	Short: "Show CPU, RAM, and Load statistics for hosts over time",
	Long:  "Query trend metrics across multiple hosts concurrently, filter by business hours, and compute peak statistics and cluster peak.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, _, err := GetActiveClient()
		if err != nil {
			return err
		}

		var targets []string
		if len(args) > 0 && args[0] != "" {
			parts := strings.Split(args[0], ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					targets = append(targets, p)
				}
			}
		}

		if statsInputFile != "" {
			f, err := os.Open(statsInputFile)
			if err != nil {
				return fmt.Errorf("reading input file %s: %w", statsInputFile, err)
			}
			defer f.Close()

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" && !strings.HasPrefix(line, "#") {
					targets = append(targets, line)
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("scanning input file: %w", err)
			}
		}

		var hostgroups []string
		for _, g := range strings.Split(statsHostgroup, ",") {
			if g = strings.TrimSpace(g); g != "" {
				hostgroups = append(hostgroups, g)
			}
		}

		if len(targets) == 0 && len(hostgroups) == 0 {
			return fmt.Errorf("no targets specified; provide a target, -i/--input-file or --hostgroup")
		}

		concurrency := statsConcurrency
		if concurrency <= 0 && appConfig != nil {
			concurrency = appConfig.Defaults.Concurrency
		}
		if concurrency <= 0 {
			concurrency = 10
		}

		loc, err := Location()
		if err != nil {
			return err
		}
		filter, err := zbxclient.NewTrendFilter(statsBusinessHours, statsHours, statsWeekdaysOnly, loc)
		if err != nil {
			return err
		}
		start, end, err := timeparse.ParseRange(statsRange, statsFrom, statsTo, statsDays, time.Now(), loc)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "Calculating resource statistics for %d targets (%s..%s, business_hours=%v, peak=%v)...\n",
			len(targets), start.Zabbix, end.Zabbix, statsBusinessHours, statsPeak)

		stats, clusterPeak, err := client.GetHostStatsSummary(cmd.Context(), zbxclient.StatsQuery{
			Targets: targets, Hostgroups: hostgroups, TimeFrom: start.Unix, TimeTill: end.Unix,
			Filter: filter, Peak: statsPeak, Concurrency: concurrency,
		})
		if err != nil {
			return fmt.Errorf("calculating stats: %w", err)
		}

		if OutputFormat() == "json" {
			return writeJSON(cmd.OutOrStdout(), statsJSON(stats, clusterPeak, start, end, loc, filter, statsPeak))
		}

		title := fmt.Sprintf("BÁO CÁO TẢI MÁY CHỦ (%s..%s", start.Zabbix, end.Zabbix)
		if statsBusinessHours {
			title += " | Giờ hành chính: 08:00 - 12:00, 13:00 - 17:00 T2-T6"
		}
		if statsHours != "" {
			title += fmt.Sprintf(" | Giờ: %s", statsHours)
		}
		if statsWeekdaysOnly {
			title += " | T2-T6"
		}
		if statsHostgroup != "" {
			title += fmt.Sprintf(" | Nhóm: %s", statsHostgroup)
		}
		if statsPeak {
			title += " | Đỉnh tức thời (Peak Samples)"
		} else {
			title += " | Đỉnh trung bình giờ"
		}
		title += ")"

		render.RenderStatsTable(cmd.OutOrStdout(), stats, clusterPeak, title)
		return nil
	},
}

func init() {
	showHostStatsCmd.Flags().StringVarP(&statsInputFile, "input-file", "i", "", "path to file containing list of IPs or hostnames")
	showHostStatsCmd.Flags().IntVarP(&statsDays, "days", "d", 30, "number of days to aggregate metrics")
	showHostStatsCmd.Flags().BoolVar(&statsBusinessHours, "business-hours", false, "filter Monday-Friday 08:00-12:00, 13:00-17:00")
	showHostStatsCmd.Flags().BoolVar(&statsPeak, "peak", false, "use 1-sample raw instant peak instead of peak hourly average")
	showHostStatsCmd.Flags().IntVar(&statsConcurrency, "concurrency", 0, "max concurrent requests (default 10)")

	showHostStatsCmd.Flags().StringVar(&statsHours, "hours", "", "hour ranges 'START-END[,START-END]' e.g. '8-12,13-17'")
	showHostStatsCmd.Flags().BoolVar(&statsWeekdaysOnly, "weekdays-only", false, "only Monday-Friday samples")
	showHostStatsCmd.Flags().StringVar(&statsHostgroup, "hostgroup", "", "filter hosts by hostgroup name or ID (comma-separated)")
	showHostStatsCmd.Flags().StringVarP(&statsRange, "time-range", "r", "", "time range e.g. 'now-7d..now'")
	showHostStatsCmd.Flags().StringVarP(&statsFrom, "from", "f", "", "start time")
	showHostStatsCmd.Flags().StringVarP(&statsTo, "to", "t", "", "end time")

	rootCmd.AddCommand(showHostStatsCmd)
}
