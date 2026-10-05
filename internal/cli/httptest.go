package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	httpTestHost        string
	httpTestName        string
	httpTestURL         string
	httpTestStatusCodes string
	httpTestDelay       string
	httpTestYes         bool

	httpTestCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a web scenario (HTTP test) on a host",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(httpTestHost) == "" {
				return fmt.Errorf("--host is required")
			}
			if strings.TrimSpace(httpTestName) == "" {
				return fmt.Errorf("--name is required")
			}
			if strings.TrimSpace(httpTestURL) == "" {
				return fmt.Errorf("--url is required")
			}

			hostID, err := resolveSingleHostID(cmd, httpTestHost)
			if err != nil {
				return err
			}

			statusCodes := httpTestStatusCodes
			if statusCodes == "" {
				statusCodes = "200"
			}
			delay := httpTestDelay
			if delay == "" {
				delay = "1m"
			}

			payload := map[string]any{
				"name":   httpTestName,
				"hostid": hostID,
				"delay":  delay,
				"steps": []map[string]any{
					{
						"name":         httpTestName,
						"url":          httpTestURL,
						"status_codes": statusCodes,
						"no":           1,
					},
				},
			}

			return runMutation(cmd, "httptest.create", payload, httpTestYes, fmt.Sprintf("Created web scenario '%s' on host '%s'.", httpTestName, httpTestHost))
		},
	}
)

func init() {
	httpTestCreateCmd.Flags().StringVar(&httpTestHost, "host", "", "target host name, visible name, or IP (required)")
	httpTestCreateCmd.Flags().StringVar(&httpTestName, "name", "", "web scenario name (required)")
	httpTestCreateCmd.Flags().StringVar(&httpTestURL, "url", "", "URL to test (required)")
	httpTestCreateCmd.Flags().StringVar(&httpTestStatusCodes, "status-codes", "200", "expected HTTP status codes")
	httpTestCreateCmd.Flags().StringVar(&httpTestDelay, "delay", "1m", "execution frequency")
	httpTestCreateCmd.Flags().BoolVar(&httpTestYes, "yes", false, "confirm execution (bypasses dry-run)")

	_ = httpTestCreateCmd.MarkFlagRequired("host")
	_ = httpTestCreateCmd.MarkFlagRequired("name")
	_ = httpTestCreateCmd.MarkFlagRequired("url")

	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "httptest" {
			cmd.AddCommand(httpTestCreateCmd)
			break
		}
	}
}
