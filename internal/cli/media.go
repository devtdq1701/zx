package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"zx/internal/zbxclient"
)

var mediaCmd = &cobra.Command{
	Use:     "media",
	Aliases: []string{"mediatype"},
	Short:   "Show configured Zabbix alert media types and user channels",
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
	tgYes       bool
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

		// --dryrun is kept only so old scripts can never turn into writes.
		dryRun := tgDryRun || !tgYes
		res, err := client.CreateTelegramMediaType(cmd.Context(), args[0], args[1], tgParseMode, dryRun)
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		if dryRun {
			fmt.Fprintln(out, "! [DRY-RUN] Would create Telegram media type (rerun with --yes to apply):")
		} else {
			fmt.Fprintln(out, "Successfully created Telegram media type:")
		}
		tw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
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
	userMediaYes       bool
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

		dryRun := userMediaDryRun || !userMediaYes
		medias, err := client.AddUserMedia(cmd.Context(), args[0], userMediaMediaType, userMediaSendTo, userMediaPeriod, userMediaSeverity, userMediaEnabled, dryRun)
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		if dryRun {
			fmt.Fprintln(out, "! [DRY-RUN] Would add user media (rerun with --yes to apply):")
		} else {
			fmt.Fprintln(out, "Successfully updated user media channels:")
		}
		tw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
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

var (
	mediaTestID      string
	mediaTestSendTo  string
	mediaTestSubject string
	mediaTestMessage string
	mediaTestYes     bool
)

var mediaTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Send a test message using a media type",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(mediaTestID) == "" {
			return fmt.Errorf("--id is required")
		}
		if strings.TrimSpace(mediaTestSendTo) == "" {
			return fmt.Errorf("--sendto is required")
		}
		if strings.TrimSpace(mediaTestSubject) == "" {
			return fmt.Errorf("--subject is required")
		}
		if strings.TrimSpace(mediaTestMessage) == "" {
			return fmt.Errorf("--message is required")
		}

		payload := zbxclient.MediaTypeTestParams{
			MediaTypeID: mediaTestID,
			SendTo:      mediaTestSendTo,
			Subject:     mediaTestSubject,
			Message:     mediaTestMessage,
		}

		if !mediaTestYes {
			return runMutation(cmd, "mediatype.test", payload, false, "")
		}

		client, _, _, err := GetActiveClient()
		if err != nil {
			return err
		}

		res, err := client.TestMediaType(cmd.Context(), payload)
		if err != nil {
			return fmt.Errorf("mediatype.test: %w", err)
		}
		if !res.Success {
			return fmt.Errorf("mediatype test failed: %s", res.Error)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully sent test message to %s.\n", mediaTestSendTo)
		return nil
	},
}

func init() {
	mediaTestCmd.Flags().StringVar(&mediaTestID, "id", "", "Media type ID (required)")
	mediaTestCmd.Flags().StringVar(&mediaTestSendTo, "sendto", "", "Recipient address or chat ID (required)")
	mediaTestCmd.Flags().StringVar(&mediaTestSubject, "subject", "", "Message subject (required)")
	mediaTestCmd.Flags().StringVar(&mediaTestMessage, "message", "", "Message body (required)")
	mediaTestCmd.Flags().BoolVar(&mediaTestYes, "yes", false, "Confirm execution (bypasses dry-run)")
	_ = mediaTestCmd.MarkFlagRequired("id")
	_ = mediaTestCmd.MarkFlagRequired("sendto")
	_ = mediaTestCmd.MarkFlagRequired("subject")
	_ = mediaTestCmd.MarkFlagRequired("message")

	createTelegramCmd.Flags().StringVar(&tgParseMode, "parse-mode", "", "Telegram parse mode")
	createTelegramCmd.Flags().BoolVar(&tgYes, "yes", false, "Confirm execution (bypasses dry-run)")
	createTelegramCmd.Flags().BoolVar(&tgDryRun, "dryrun", false, "Preview without writing")
	_ = createTelegramCmd.Flags().MarkDeprecated("dryrun", "preview is now the default; use --yes to apply")

	addUserMediaCmd.Flags().StringVar(&userMediaMediaType, "mediatype", "", "Media type name or ID")
	addUserMediaCmd.Flags().StringVar(&userMediaSendTo, "sendto", "", "Recipient / chat ID")
	addUserMediaCmd.Flags().StringVar(&userMediaPeriod, "period", "1-7,00:00-24:00", "Zabbix active period")
	addUserMediaCmd.Flags().IntVar(&userMediaSeverity, "severity", 63, "Severity bitmask (default 63 = all)")
	addUserMediaCmd.Flags().BoolVar(&userMediaEnabled, "enabled", true, "Enable or disable media")
	addUserMediaCmd.Flags().BoolVar(&userMediaYes, "yes", false, "Confirm execution (bypasses dry-run)")
	addUserMediaCmd.Flags().BoolVar(&userMediaDryRun, "dryrun", false, "Preview without writing")
	_ = addUserMediaCmd.Flags().MarkDeprecated("dryrun", "preview is now the default; use --yes to apply")

	mediaCmd.AddCommand(mediaTestCmd)
	mediaCmd.AddCommand(mediaUserCmd)
	mediaCmd.AddCommand(createTelegramCmd)
	mediaCmd.AddCommand(addUserMediaCmd)

	rootCmd.AddCommand(mediaCmd)
	rootCmd.AddCommand(createTelegramCmd)
	rootCmd.AddCommand(addUserMediaCmd)
}
