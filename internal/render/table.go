package render

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"zx/internal/zbxclient"
)

func RenderStatsTable(w io.Writer, stats []zbxclient.HostStats, peak *zbxclient.ClusterPeak, title string) {
	if w == nil {
		w = os.Stdout
	}

	fmt.Fprintln(w, "")
	if title != "" {
		fmt.Fprintf(w, "=== %s ===\n\n", title)
	}

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	header := "IP\tHOST\tCORES\tRAM (GB)\tCPU AVG\tCPU MAX\tRAM AVG\tRAM MAX\tLOAD AVG\tLOAD MAX"
	fmt.Fprintln(tw, header)
	fmt.Fprintln(tw, strings.Repeat("-", 100))

	var failedCount int
	for _, s := range stats {
		if s.Error != "" {
			failedCount++
			fmt.Fprintf(tw, "%s\t%s\tERROR: %s\t\t\t\t\t\t\t\n", s.IP, s.HostName, s.Error)
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%.1f\t%.2f%%\t%.2f%%\t%.2f%%\t%.2f%%\t%.2f\t%.2f\n",
			s.IP,
			s.HostName,
			s.CPUCores,
			s.RAMTotalGB,
			s.CPUAvg,
			s.CPUMax,
			s.RAMAvg,
			s.RAMMax,
			s.LoadAvg,
			s.LoadMax,
		)
	}

	if peak != nil {
		fmt.Fprintln(tw, strings.Repeat("-", 100))
		fmt.Fprintf(tw, "CLUSTER PEAK\t%s\t-\t-\t%.2f%%\t-\t%.2f%%\t-\t%.2f\t-\n",
			peak.Time.Format("2006-01-02 15:04"),
			peak.CPUAvg,
			peak.RAMAvg,
			peak.LoadAvg,
		)
	}

	_ = tw.Flush()
	if failedCount > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: %d host(s) failed; numbers for them are not shown\n", failedCount)
	}
	fmt.Fprintln(w, "")
}
