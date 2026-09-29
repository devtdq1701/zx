package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"zx/internal/config"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage Zabbix connection profiles",
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "ACTIVE\tNAME\tURL\tAUTH")
		fmt.Fprintln(tw, strings.Repeat("-", 60))

		for name, p := range cfg.Profiles {
			activeMark := " "
			if name == cfg.ActiveProfile {
				activeMark = "*"
			}
			authType := "None"
			if p.Token != "" {
				authType = "Token: " + p.MaskedToken()
			} else if p.User != "" {
				authType = "User: " + p.User
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", activeMark, name, p.URL, authType)
		}
		_ = tw.Flush()
		return nil
	},
}

var profileSwitchCmd = &cobra.Command{
	Use:   "switch [NAME]",
	Short: "Switch active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		if _, ok := cfg.Profiles[name]; !ok {
			return fmt.Errorf("profile '%s' not found", name)
		}

		cfg.ActiveProfile = name
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Printf("Switched active profile to '%s'.\n", name)
		return nil
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show [NAME]",
	Short: "Show details of a profile with masked secrets",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		targetName := cfg.ActiveProfile
		if len(args) > 0 {
			targetName = args[0]
		}

		p, err := cfg.GetProfile(targetName)
		if err != nil {
			return err
		}

		fmt.Printf("Profile: %s\n", targetName)
		fmt.Printf("  URL:        %s\n", p.URL)
		fmt.Printf("  Verify SSL: %v\n", p.VerifySSL)
		if p.Token != "" {
			fmt.Printf("  Token:      %s\n", p.MaskedToken())
		}
		if p.User != "" {
			fmt.Printf("  User:       %s\n", p.User)
			fmt.Printf("  Password:   %s\n", p.MaskedPassword())
		}
		return nil
	},
}

var (
	newProfURL       string
	newProfToken     string
	newProfUser      string
	newProfPassword  string
	newProfVerifySSL bool
)

var profileAddCmd = &cobra.Command{
	Use:   "add [NAME]",
	Short: "Add or update a connection profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if newProfURL == "" {
			return fmt.Errorf("--url is required")
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		cfg.Profiles[name] = config.Profile{
			URL:       newProfURL,
			Token:     newProfToken,
			User:      newProfUser,
			Password:  newProfPassword,
			VerifySSL: newProfVerifySSL,
		}

		if cfg.ActiveProfile == "" {
			cfg.ActiveProfile = name
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving profile: %w", err)
		}

		fmt.Printf("Profile '%s' saved successfully.\n", name)
		return nil
	},
}

func init() {
	profileAddCmd.Flags().StringVar(&newProfURL, "url", "", "Zabbix web/API base URL")
	profileAddCmd.Flags().StringVar(&newProfToken, "token", "", "API token (Zabbix 6.4/7.0)")
	profileAddCmd.Flags().StringVar(&newProfUser, "user", "", "Zabbix username")
	profileAddCmd.Flags().StringVar(&newProfPassword, "password", "", "Zabbix password")
	profileAddCmd.Flags().BoolVar(&newProfVerifySSL, "verify-ssl", true, "verify SSL certificate")

	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileSwitchCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileAddCmd)

	rootCmd.AddCommand(profileCmd)
}
