package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// expectedPreflightStatus matches zabbix-cli: the JSON-RPC endpoint answers
// an unauthenticated GET with 200 or 412.
var expectedPreflightStatus = map[int]bool{200: true, 412: true}

var errPreflightFailed = errors.New("preflight failed")

var preflightCmd = &cobra.Command{
	Use:          "preflight",
	Short:        "Check connection and reachability to active Zabbix endpoint",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, _, err := GetActiveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()

		status, duration, err := client.Preflight(ctx)
		ms := duration.Milliseconds()
		endpoint := client.RPCURL()
		// Output format is a contract with tomcat-diagnostics (PASS prefix on stdout).
		if err == nil && expectedPreflightStatus[status] {
			fmt.Fprintf(cmd.OutOrStdout(), "PASS endpoint=%s http=%d (expected) latency_ms=%d\n", endpoint, status, ms)
			return nil
		}
		detail := fmt.Sprintf("http=%d (unexpected)", status)
		if err != nil {
			detail = "error=" + preflightErrorName(err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "FAIL endpoint=%s %s latency_ms=%d\n", endpoint, detail, ms)
		// The FAIL line is the whole stderr contract. Silence only this
		// error: a static SilenceErrors would also hide config/profile errors.
		cmd.Root().SilenceErrors = true
		return errPreflightFailed
	},
}

func preflightErrorName(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "Timeout"
	}
	return "ConnectError"
}
