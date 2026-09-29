package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"zx/internal/render"
)

var (
	statsInputFile     string
	statsDays          int
	statsBusinessHours bool
	statsPeak          bool
	statsConcurrency   int
)

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

		if len(targets) == 0 {
			return fmt.Errorf("no targets specified; provide a target argument or use -i/--input-file")
		}

		concurrency := statsConcurrency
		if concurrency <= 0 && appConfig != nil {
			concurrency = appConfig.Defaults.Concurrency
		}
		if concurrency <= 0 {
			concurrency = 10
		}

		fmt.Printf("Calculating resource statistics for %d targets (%d days, business_hours=%v, peak=%v)...\n",
			len(targets), statsDays, statsBusinessHours, statsPeak)

		stats, clusterPeak, err := client.GetHostStatsSummary(
			cmd.Context(),
			targets,
			statsDays,
			statsBusinessHours,
			statsPeak,
			concurrency,
		)
		if err != nil {
			return fmt.Errorf("calculating stats: %w", err)
		}

		title := fmt.Sprintf("BÁO CÁO TẢI MÁY CHỦ (%d ngày", statsDays)
		if statsBusinessHours {
			title += " | Giờ hành chính: 08:00 - 12:00, 13:00 - 17:00 T2-T6"
		}
		if statsPeak {
			title += " | Đỉnh tức thời (Peak Samples)"
		} else {
			title += " | Đỉnh trung bình giờ"
		}
		title += ")"

		render.RenderStatsTable(os.Stdout, stats, clusterPeak, title)
		return nil
	},
}

func init() {
	showHostStatsCmd.Flags().StringVarP(&statsInputFile, "input-file", "i", "", "path to file containing list of IPs or hostnames")
	showHostStatsCmd.Flags().IntVarP(&statsDays, "days", "d", 30, "number of days to aggregate metrics")
	showHostStatsCmd.Flags().BoolVar(&statsBusinessHours, "business-hours", false, "filter Monday-Friday 08:00-12:00, 13:00-17:00")
	showHostStatsCmd.Flags().BoolVar(&statsPeak, "peak", false, "use 1-sample raw instant peak instead of peak hourly average")
	showHostStatsCmd.Flags().IntVar(&statsConcurrency, "concurrency", 0, "max concurrent requests (default 10)")

	rootCmd.AddCommand(showHostStatsCmd)
}
