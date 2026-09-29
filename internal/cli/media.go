package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var mediaCmd = &cobra.Command{
	Use:   "media",
	Short: "Show configured Zabbix alert media types",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, _, err := GetActiveClient()
		if err != nil {
			return err
		}

		types, err := client.GetMediaTypes(cmd.Context())
		if err != nil {
			return err
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "ID\tNAME\tSTATUS\tDESCRIPTION")
		fmt.Fprintln(tw, strings.Repeat("-", 80))

		for _, m := range types {
			statusStr := "Enabled"
			if m.Status == "1" {
				statusStr = "Disabled"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", m.MediaTypeID, m.Name, statusStr, m.Description)
		}
		_ = tw.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mediaCmd)
}
