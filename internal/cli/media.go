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

var (
	tgParseMode string
	tgDryRun    bool
)

var createTelegramCmd = &cobra.Command{
	Use:     "create_telegram_mediatype [NAME] [TOKEN]",
	Aliases: []string{"create-telegram"},
	Short:   "Create a new Telegram webhook media type",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, _, err := GetActiveClient()
		if err != nil {
			return err
		}

		res, err := client.CreateTelegramMediaType(cmd.Context(), args[0], args[1], tgParseMode, tgDryRun)
		if err != nil {
			return err
		}

		if tgDryRun {
			fmt.Println("! [DRY-RUN] Would create Telegram media type:")
		} else {
			fmt.Println("Successfully created Telegram media type:")
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "ID\tNAME\tTYPE\tSTATUS\tDETAILS")
		fmt.Fprintln(tw, strings.Repeat("-", 80))
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", res.MediaTypeID, res.Name, res.Type, res.Status, res.Description)
		_ = tw.Flush()
		return nil
	},
}

var (
	userMediaMediaType string
	userMediaSendTo    string
	userMediaPeriod    string
	userMediaSeverity  int
	userMediaEnabled   bool
	userMediaDryRun    bool
)

var addUserMediaCmd = &cobra.Command{
	Use:     "add_user_media [USERNAME_OR_ID]",
	Aliases: []string{"add-user"},
	Short:   "Add a new notification media record to a user while preserving existing media",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if userMediaMediaType == "" || userMediaSendTo == "" {
			return fmt.Errorf("--mediatype and --sendto are required")
		}

		client, _, _, err := GetActiveClient()
		if err != nil {
			return err
		}

		medias, err := client.AddUserMedia(cmd.Context(), args[0], userMediaMediaType, userMediaSendTo, userMediaPeriod, userMediaSeverity, userMediaEnabled, userMediaDryRun)
		if err != nil {
			return err
		}

		if userMediaDryRun {
			fmt.Println("! [DRY-RUN] Would add user media:")
		} else {
			fmt.Println("Successfully updated user media channels:")
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintf(tw, "MEDIA ID\tTYPE NAME\tSEND TO / RECIPIENT\tSTATUS\tPERIOD\n")
		fmt.Fprintln(tw, strings.Repeat("-", 80))

		for _, m := range medias {
			statusStr := "Enabled"
			if m.Active == "1" {
				statusStr = "Disabled"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", m.MediaID, m.MediaName, m.SendTo, statusStr, m.Period)
		}
		_ = tw.Flush()
		return nil
	},
}

func init() {
	createTelegramCmd.Flags().StringVar(&tgParseMode, "parse-mode", "", "Telegram parse mode")
	createTelegramCmd.Flags().BoolVar(&tgDryRun, "dryrun", false, "Preview without writing")

	addUserMediaCmd.Flags().StringVar(&userMediaMediaType, "mediatype", "", "Media type name or ID")
	addUserMediaCmd.Flags().StringVar(&userMediaSendTo, "sendto", "", "Recipient / chat ID")
	addUserMediaCmd.Flags().StringVar(&userMediaPeriod, "period", "1-7,00:00-24:00", "Zabbix active period")
	addUserMediaCmd.Flags().IntVar(&userMediaSeverity, "severity", 63, "Severity bitmask (default 63 = all)")
	addUserMediaCmd.Flags().BoolVar(&userMediaEnabled, "enabled", true, "Enable or disable media")
	addUserMediaCmd.Flags().BoolVar(&userMediaDryRun, "dryrun", false, "Preview without writing")

	mediaCmd.AddCommand(mediaUserCmd)
	mediaCmd.AddCommand(createTelegramCmd)
	mediaCmd.AddCommand(addUserMediaCmd)

	rootCmd.AddCommand(mediaCmd)
	rootCmd.AddCommand(createTelegramCmd)
	rootCmd.AddCommand(addUserMediaCmd)
}
