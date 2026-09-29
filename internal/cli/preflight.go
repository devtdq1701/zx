package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Check connection and reachability to active Zabbix endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, prof, name, err := GetActiveClient()
		if err != nil {
			return err
		}

		fmt.Printf("Preflight probe for profile '%s' (%s)...\n", name, prof.URL)
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()

		status, duration, err := client.Preflight(ctx)
		if err != nil {
			return fmt.Errorf("preflight failed: %w", err)
		}

		ms := duration.Milliseconds()
		fmt.Printf("HTTP Status: %d | Latency: %d ms | Target reachable: OK\n", status, ms)
		return nil
	},
}
