package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

var profileImportLegacyCmd = &cobra.Command{
	Use:   "import-legacy",
	Short: "Import profiles from legacy ~/.config/zabbix-cli/profiles.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		legacyPath := filepath.Join(home, ".config", "zabbix-cli", "profiles.json")
		ldata, err := os.ReadFile(legacyPath)
		if err != nil {
			return fmt.Errorf("reading legacy profiles: %w", err)
		}

		type legacyStore struct {
			Active   string `json:"active"`
			Profiles map[string]struct {
				URL       string `json:"url"`
				User      string `json:"user"`
				Username  string `json:"username"`
				Password  string `json:"password"`
				Token     string `json:"token"`
				VerifySSL *bool  `json:"verify_ssl"`
			} `json:"profiles"`
		}
		var leg legacyStore
		if err := json.Unmarshal(ldata, &leg); err != nil {
			return fmt.Errorf("parsing legacy profiles: %w", err)
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			cfg = config.DefaultConfig()
		}

		if leg.Active != "" {
			cfg.ActiveProfile = leg.Active
		}
		count := 0
		for name, lp := range leg.Profiles {
			verify := true
			if lp.VerifySSL != nil {
				verify = *lp.VerifySSL
			}
			u := lp.User
			if u == "" {
				u = lp.Username
			}
			cfg.Profiles[name] = config.Profile{
				URL:       lp.URL,
				User:      u,
				Password:  lp.Password,
				Token:     lp.Token,
				VerifySSL: verify,
			}
			count++
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("Successfully imported %d profiles from %s\n", count, legacyPath)
		return nil
	},
}

var profileCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Print the current active profile/context name",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		if cfg.ActiveProfile == "" {
			fmt.Println("No active profile set")
			return nil
		}
		fmt.Println(cfg.ActiveProfile)
		return nil
	},
}

var contextCmd = &cobra.Command{
	Use:     "context",
	Aliases: []string{"ctx"},
	Short:   "Manage connection contexts (alias for profile)",
}

var contextUseCmd = &cobra.Command{
	Use:   "use [NAME]",
	Short: "Switch active context",
	Args:  cobra.ExactArgs(1),
	RunE:  profileSwitchCmd.RunE,
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
	profileCmd.AddCommand(profileImportLegacyCmd)
	profileCmd.AddCommand(profileCurrentCmd)

	contextCmd.AddCommand(profileListCmd)
	contextCmd.AddCommand(contextUseCmd)
	contextCmd.AddCommand(profileCurrentCmd)

	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(contextCmd)
}
