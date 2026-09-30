package render

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"zx/internal/zbxclient"
)

func RenderStatsTable(w io.Writer, stats []zbxclient.HostStats, peak *zbxclient.ClusterPeak, title string, loc *time.Location) {
	if w == nil {
		w = os.Stdout
	}
	if loc == nil {
		loc = time.Local
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
		has := func(metric string) bool {
			for _, m := range s.Missing {
				if m == metric {
					return true
				}
			}
			return false
		}
		pct := func(v float64, metric string) string {
			if has(metric) {
				return "-"
			}
			return fmt.Sprintf("%.2f%%", v)
		}
		num := func(v float64, metric string) string {
			if has(metric) {
				return "-"
			}
			return fmt.Sprintf("%.2f", v)
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%.1f\t%s\t%s\t%s\t%s\t%s\t%s\n",
			s.IP, s.HostName, s.CPUCores, s.RAMTotalGB,
			pct(s.CPUAvg, "cpu"), pct(s.CPUMax, "cpu"),
			pct(s.RAMAvg, "ram"), pct(s.RAMMax, "ram"),
			num(s.LoadAvg, "load"), num(s.LoadMax, "load"),
		)
	}

	if peak != nil {
		fmt.Fprintln(tw, strings.Repeat("-", 100))
		fmt.Fprintf(tw, "CLUSTER PEAK\t%s\t-\t-\t%.2f%%\t-\t%.2f%%\t-\t%.2f\t-\n",
			peak.Time.In(loc).Format("2006-01-02 15:04"),
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

type MetricSummaryRow struct {
	Host    string  `json:"host"`
	IP      string  `json:"ip"`
	Min     float64 `json:"min"`
	Avg     float64 `json:"avg"`
	Max     float64 `json:"max"`
	Samples int     `json:"samples"`
	Error   string  `json:"error,omitempty"`
}

func RenderMetricSummary(w io.Writer, title string, rows []MetricSummaryRow) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, "")
	if title != "" {
		fmt.Fprintf(w, "=== %s ===\n\n", title)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "IP\tHOST\tMIN\tAVG\tMAX\tSAMPLES")
	fmt.Fprintln(tw, strings.Repeat("-", 70))
	var failedCount int
	for _, r := range rows {
		if r.Error != "" {
			failedCount++
			fmt.Fprintf(tw, "%s\t%s\tERROR: %s\t\t\t\n", r.IP, r.Host, r.Error)
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\t%.2f\t%.2f\t%.2f\t%d\n", r.IP, r.Host, r.Min, r.Avg, r.Max, r.Samples)
	}
	_ = tw.Flush()
	if failedCount > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: %d host(s) failed\n", failedCount)
	}
	fmt.Fprintln(w, "")
}
