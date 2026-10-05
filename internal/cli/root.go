package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"zx/internal/config"
	"zx/internal/zbxclient"
)

var (
	cfgFile      string
	profileFlag  string
	debugFlag    bool
	appConfig    *config.Config
	activeClient *zbxclient.Client
	activeProf   *config.Profile
	activeName   string

	// testProfile, when set by tests, replaces config loading so tests never
	// read the user's real ~/.config/zx or ~/.config/zabbix-cli files.
	testProfile *config.Profile

	rootCmd = &cobra.Command{
		Use:   "zx",
		Short: "zx - Fast Golang Zabbix CLI & REPL client",
		Long:  "Fast, standalone Zabbix terminal client with multi-cluster profiles, concurrent host metrics, and PNG graph exports.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(); err != nil {
				return err
			}
			// Flags and args are valid by now (cobra validates them before
			// this hook), so later errors are runtime errors: print the error
			// without the full usage text. executeLine resets this per line.
			cmd.Root().SilenceUsage = true
			// Re-resolve the client on every execution so a REPL line always
			// talks to the profile selected for that line.
			activeClient, activeProf, activeName = nil, nil, ""
			if testProfile != nil {
				activeProf, activeName = testProfile, "test"
				activeClient = zbxclient.NewClient(testProfile, 5*time.Second)
				return nil
			}
			var err error
			if cfgFile != "" {
				appConfig, err = config.LoadConfigFrom(cfgFile)
			} else {
				appConfig, err = config.LoadConfig()
			}
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			targetProfile := appConfig.ActiveProfile
			if profileFlag != "" {
				targetProfile = profileFlag
			}
			if targetProfile == "" {
				return nil
			}
			prof, err := appConfig.GetProfile(targetProfile)
			if err != nil {
				if profileFlag != "" {
					return fmt.Errorf("profile '%s': %w", profileFlag, err)
				}
				return nil
			}
			activeProf = prof
			activeName = targetProfile
			timeout := time.Duration(appConfig.Defaults.TimeoutSeconds) * time.Second
			if timeout <= 0 {
				timeout = 30 * time.Second
			}
			activeClient = zbxclient.NewClient(prof, timeout)
			return nil
		},
	}
)

func RootCmd() *cobra.Command {
	return rootCmd
}

func Execute() error {
	return rootCmd.Execute()
}

func ExecuteContext(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

func GetActiveClient() (*zbxclient.Client, *config.Profile, string, error) {
	if activeClient == nil {
		return nil, nil, "", fmt.Errorf("no active Zabbix profile configured or selected")
	}
	return activeClient, activeProf, activeName, nil
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path (default ~/.config/zx/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&profileFlag, "profile", "p", "", "Zabbix connection profile name")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable debug logging")
	rootCmd.PersistentFlags().StringVar(&formatFlag, "format", "table", "output format: table or json")
	rootCmd.PersistentFlags().StringVar(&timezoneFlag, "timezone", "Asia/Ho_Chi_Minh", "timezone for hour filters and absolute times")

	rootCmd.AddCommand(preflightCmd)
	rootCmd.AddCommand(importConfigCmd)
	rootCmd.AddCommand(actionCmd)
}
