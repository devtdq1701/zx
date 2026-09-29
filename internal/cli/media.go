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
	Short: "Show configured Zabbix alert media types and user channels",
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
			desc := strings.TrimSpace(m.Description)
			if idx := strings.Index(desc, "\n"); idx != -1 {
				desc = desc[:idx]
			}
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", m.MediaTypeID, m.Name, statusStr, desc)
		}
		_ = tw.Flush()
		return nil
	},
}

var mediaUserCmd = &cobra.Command{
	Use:   "user [USERNAME_OR_ID]",
	Short: "Show configured notification media for a specific user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, _, err := GetActiveClient()
		if err != nil {
			return err
		}

		username := args[0]
		medias, err := client.GetUserMedia(cmd.Context(), username)
		if err != nil {
			return err
		}

		if len(medias) == 0 {
			fmt.Printf("No notification media channels configured for user '%s'.\n", username)
			return nil
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintf(tw, "MEDIA ID\tTYPE NAME\tSEND TO / RECIPIENT\tSTATUS\tPERIOD\n")
		fmt.Fprintln(tw, strings.Repeat("-", 80))

		for _, m := range medias {
			statusStr := "Enabled"
			if m.Active == "1" {
				statusStr = "Disabled"
			}
			name := m.MediaName
			if name == "" {
				name = "Type #" + m.MediaTypeID
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", m.MediaID, name, m.SendTo, statusStr, m.Period)
		}
		_ = tw.Flush()
		return nil
	},
}

func init() {
	mediaCmd.AddCommand(mediaUserCmd)
	rootCmd.AddCommand(mediaCmd)
}
