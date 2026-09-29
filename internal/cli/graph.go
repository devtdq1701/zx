package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"zx/internal/zbxclient"
)

var (
	graphInputFile string
	graphMetric    string
	graphRange     string
	graphFrom      string
	graphTo        string
	graphDays      int
	graphOutput    string
	graphWidth     int
	graphHeight    int
)

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

		if len(targets) == 0 {
			return fmt.Errorf("no targets specified; provide a target argument or use -i/--input-file")
		}

		// Resolve time range
		fromTime := graphFrom
		toTime := graphTo
		if graphRange != "" {
			parts := strings.Split(graphRange, "..")
			if len(parts) == 2 {
				fromTime = strings.TrimSpace(parts[0])
				toTime = strings.TrimSpace(parts[1])
			}
		}
		if fromTime == "" {
			fromTime = fmt.Sprintf("now-%dd", graphDays)
		}
		if toTime == "" {
			toTime = "now"
		}

		hosts, _, err := client.ResolveHosts(cmd.Context(), targets)
		if err != nil {
			return fmt.Errorf("resolving hosts: %w", err)
		}
		if len(hosts) == 0 {
			return fmt.Errorf("no hosts found matching targets")
		}

		var itemIDs []string
		for _, h := range hosts {
			var items []zbxclient.ItemRecord
			err := client.Call(cmd.Context(), "item.get", map[string]any{
				"hostids": h.HostID,
				"output":  []string{"itemid", "name", "key_"},
			}, &items)
			if err != nil {
				continue
			}

			metricLower := strings.ToLower(graphMetric)
			for _, it := range items {
				k := it.Key
				nameLower := strings.ToLower(it.Name)
				matched := false

				switch metricLower {
				case "ram", "memory":
					if k == "vm.memory.utilization" || k == "vm.memory.util" || k == "vm.memory.size[pused]" || strings.Contains(nameLower, "memory utilization") {
						matched = true
					}
				case "cpu":
					if k == "system.cpu.util" || strings.Contains(nameLower, "cpu utilization") {
						matched = true
					}
				case "load":
					if k == "system.cpu.load[all,avg15]" || (strings.Contains(k, "system.cpu.load") && strings.Contains(k, "15")) {
						matched = true
					}
				}

				if matched {
					itemIDs = append(itemIDs, it.ItemID)
					break
				}
			}
		}

		if len(itemIDs) == 0 {
			return fmt.Errorf("no matching metric items found for metric '%s'", graphMetric)
		}

		fmt.Printf("Exporting %s graph for %d hosts (%d items) from '%s' to '%s' -> %s...\n",
			graphMetric, len(hosts), len(itemIDs), fromTime, toTime, graphOutput)

		err = client.DownloadCombinedGraph(
			cmd.Context(),
			itemIDs,
			fromTime,
			toTime,
			graphWidth,
			graphHeight,
			graphOutput,
		)
		if err != nil {
			return fmt.Errorf("export graph failed: %w", err)
		}

		fmt.Printf("Graph successfully saved: %s (%dx%d px)\n", graphOutput, graphWidth, graphHeight)
		return nil
	},
}

func init() {
	exportGraphCmd.Flags().StringVarP(&graphInputFile, "input-file", "i", "", "path to file containing list of IPs or hostnames")
	exportGraphCmd.Flags().StringVarP(&graphMetric, "metric", "m", "ram", "metric type: ram, cpu, load")
	exportGraphCmd.Flags().StringVarP(&graphRange, "time-range", "r", "", "time range (e.g. 'now-7d..now')")
	exportGraphCmd.Flags().StringVarP(&graphFrom, "from", "f", "", "start time (e.g. 'now-30d')")
	exportGraphCmd.Flags().StringVarP(&graphTo, "to", "t", "now", "end time")
	exportGraphCmd.Flags().IntVarP(&graphDays, "days", "d", 30, "days back from now (if from/range not specified)")
	exportGraphCmd.Flags().StringVarP(&graphOutput, "output", "o", "graph.png", "output PNG file path")
	exportGraphCmd.Flags().IntVar(&graphWidth, "width", 2050, "chart image width in pixels")
	exportGraphCmd.Flags().IntVar(&graphHeight, "height", 368, "chart image height in pixels")

	rootCmd.AddCommand(exportGraphCmd)
}
