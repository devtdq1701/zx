package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var showHostsCmd = &cobra.Command{
	Use:   "show_hosts [TARGET]",
	Short: "Show details for hosts by IP or hostname",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, _, err := GetActiveClient()
		if err != nil {
			return err
		}

		target := ""
		if len(args) > 0 {
			target = args[0]
		}

		hosts, err := client.GetDetailedHosts(cmd.Context(), target)
		if err != nil {
			return err
		}

		if OutputFormat() == "json" {
			return writeJSON(cmd.OutOrStdout(), hosts)
		}

		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "HOSTID\tIP\tNAME\tSTATUS\tHOSTGROUPS")
		fmt.Fprintln(tw, strings.Repeat("-", 90))

		for _, h := range hosts {
			statusStr := "Monitored"
			if h.Status == "1" {
				statusStr = "Unmonitored"
			}
			ips := strings.Join(h.IPs, ", ")
			groups := strings.Join(h.Hostgroups, ", ")
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", h.HostID, ips, h.Name, statusStr, groups)
		}
		_ = tw.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(showHostsCmd)
}
