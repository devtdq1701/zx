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

	rootCmd = &cobra.Command{
		Use:   "zx",
		Short: "zx - Fast Golang Zabbix CLI & REPL client",
		Long:  "Fast, standalone Zabbix terminal client with multi-cluster profiles, concurrent host metrics, and PNG graph exports.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(); err != nil {
				return err
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

			if targetProfile != "" && activeClient == nil {
				prof, err := appConfig.GetProfile(targetProfile)
				if err == nil {
					activeProf = prof
					activeName = targetProfile
					timeout := time.Duration(appConfig.Defaults.TimeoutSeconds) * time.Second
					if timeout <= 0 {
						timeout = 30 * time.Second
					}
					activeClient = zbxclient.NewClient(prof, timeout)
				}
			}
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
}
