package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	apiYes    bool
	apiDryRun bool

	apiCmd = &cobra.Command{
		Use:   "api <method> [params]",
		Short: "Call arbitrary Zabbix JSON-RPC API method directly",
		Long: `Call arbitrary Zabbix JSON-RPC API method using active profile credentials.
Params can be passed as JSON string argument or via stdin.
Read methods (*.get, apiinfo.*) execute immediately.
Mutation methods default to dry-run unless --yes is specified.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.TrimSpace(args[0])
			if method == "" {
				return fmt.Errorf("method is required")
			}

			var rawInput []byte
			if len(args) > 1 {
				if args[1] == "-" {
					var err error
					rawInput, err = io.ReadAll(cmd.InOrStdin())
					if err != nil {
						return fmt.Errorf("reading stdin: %w", err)
					}
				} else {
					rawInput = []byte(args[1])
				}
			} else {
				// Check if stdin has data piped in (terminal check)
				stat, _ := os.Stdin.Stat()
				if (stat.Mode() & os.ModeCharDevice) == 0 {
					var err error
					rawInput, err = io.ReadAll(cmd.InOrStdin())
					if err != nil {
						return fmt.Errorf("reading stdin: %w", err)
					}
				}
			}

			var params any
			rawStr := strings.TrimSpace(string(rawInput))
			if rawStr != "" {
				if err := json.Unmarshal([]byte(rawStr), &params); err != nil {
					return fmt.Errorf("invalid JSON params: %w", err)
				}
			} else {
				params = map[string]any{}
			}

			isRead := strings.HasSuffix(method, ".get") || strings.HasPrefix(method, "apiinfo.")
			if (!isRead && !apiYes) || apiDryRun {
				b, err := json.MarshalIndent(params, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY-RUN] The following JSON-RPC call would be made:\nMethod: %s\nParams: %s\nTo execute this action, rerun with --yes\n", method, string(b))
				return nil
			}

			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}

			var result json.RawMessage
			if err := client.Call(cmd.Context(), method, params, &result); err != nil {
				return fmt.Errorf("%s failed: %w", method, err)
			}

			var pretty bytes.Buffer
			if err := json.Indent(&pretty, result, "", "  "); err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), string(result))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), pretty.String())
			}

			return nil
		},
	}
)

func init() {
	apiCmd.Flags().BoolVar(&apiYes, "yes", false, "confirm execution for mutation methods")
	apiCmd.Flags().BoolVar(&apiDryRun, "dry-run", false, "preview request payload without executing")
	rootCmd.AddCommand(apiCmd)
}
