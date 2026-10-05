package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"zx/internal/zbxclient"
)

var (
	triggerListHost     string
	triggerListPriority string

	triggerCreateHost        string
	triggerCreateDescription string
	triggerCreateExpression  string
	triggerCreatePriority    string
	triggerCreateDependsOn   string
	triggerCreateStatus      string
	triggerCreateYes         bool

	triggerDeleteYes bool

	triggerCmd = &cobra.Command{
		Use:   "trigger",
		Short: "Manage Zabbix triggers and dependencies",
		Long:  "List, create, and delete Zabbix triggers with support for priority mapping and dependency linking.",
	}

	triggerListCmd = &cobra.Command{
		Use:   "list",
		Short: "List triggers for a host",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(triggerListHost) == "" {
				return fmt.Errorf("--host is required")
			}

			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			host, err := client.ResolveExactHost(ctx, triggerListHost)
			if err != nil {
				return err
			}

			filter := zbxclient.TriggerFilter{
				HostID: host.HostID,
			}

			if strings.TrimSpace(triggerListPriority) != "" {
				prio, err := zbxclient.ParsePriority(triggerListPriority)
				if err != nil {
					return err
				}
				filter.Priority = &prio
			}

			triggers, err := client.GetTriggers(ctx, filter)
			if err != nil {
				return err
			}

			if OutputFormat() == "json" {
				return writeJSON(cmd.OutOrStdout(), triggers)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
			fmt.Fprintln(tw, "TRIGGERID\tSEVERITY\tSTATUS\tDESCRIPTION\tDEPENDS ON")
			fmt.Fprintln(tw, strings.Repeat("-", 80))
			for _, tr := range triggers {
				sev := zbxclient.FormatPriority(tr.Priority)
				statusStr := "active"
				if tr.Status == "1" {
					statusStr = "disabled"
				}

				var deps []string
				for _, d := range tr.Dependencies {
					deps = append(deps, d.TriggerID)
				}
				depStr := "-"
				if len(deps) > 0 {
					depStr = strings.Join(deps, ",")
				}

				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", tr.TriggerID, sev, statusStr, tr.Description, depStr)
			}
			return tw.Flush()
		},
	}

	triggerCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new trigger on a host",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(triggerCreateHost) == "" {
				return fmt.Errorf("--host is required")
			}
			if strings.TrimSpace(triggerCreateDescription) == "" {
				return fmt.Errorf("--description is required")
			}
			if strings.TrimSpace(triggerCreateExpression) == "" {
				return fmt.Errorf("--expression is required")
			}

			if err := oneOf("--status", triggerCreateStatus, "active", "disabled"); err != nil {
				return err
			}
			statusVal := 0
			if triggerCreateStatus == "disabled" {
				statusVal = 1
			}

			prioVal, err := zbxclient.ParsePriority(triggerCreatePriority)
			if err != nil {
				return err
			}

			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			host, err := client.ResolveExactHost(ctx, triggerCreateHost)
			if err != nil {
				return err
			}

			expr := triggerCreateExpression
			if host.Host != "" {
				expr = strings.ReplaceAll(expr, "{HOST.HOST}", host.Host)
			}

			params := zbxclient.CreateTriggerParams{
				Description: triggerCreateDescription,
				Expression:  expr,
				Priority:    prioVal,
				Status:      statusVal,
				DependsOn:   strings.TrimSpace(triggerCreateDependsOn),
			}

			payload, err := zbxclient.BuildCreateTriggerPayload(params)
			if err != nil {
				return err
			}

			if !triggerCreateYes {
				return runMutation(cmd, "trigger.create", payload, false, "")
			}

			id, err := client.CreateTrigger(ctx, params)
			if err != nil {
				return fmt.Errorf("trigger.create: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created trigger %s (%s).\n", id, params.Description)
			return nil
		},
	}

	triggerDeleteCmd = &cobra.Command{
		Use:   "delete [TRIGGER_ID]",
		Short: "Delete a trigger by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			triggerID := args[0]
			return runMutation(cmd, "trigger.delete", []string{triggerID}, triggerDeleteYes, fmt.Sprintf("Deleted trigger %s.", triggerID))
		},
	}
)

func init() {
	// list flags
	triggerListCmd.Flags().StringVar(&triggerListHost, "host", "", "host name, visible name, or IP (required)")
	triggerListCmd.Flags().StringVar(&triggerListPriority, "priority", "", "filter by severity (0..5 or name: disaster, high, average, warning, info, not_classified)")
	_ = triggerListCmd.MarkFlagRequired("host")

	// create flags
	triggerCreateCmd.Flags().StringVar(&triggerCreateHost, "host", "", "target host name, visible name, or IP (required)")
	triggerCreateCmd.Flags().StringVar(&triggerCreateDescription, "description", "", "trigger description/name (required)")
	triggerCreateCmd.Flags().StringVar(&triggerCreateExpression, "expression", "", "trigger expression (required)")
	triggerCreateCmd.Flags().StringVar(&triggerCreatePriority, "priority", "average", "severity priority (0..5 or disaster, high, average, warning, info, not_classified)")
	triggerCreateCmd.Flags().StringVar(&triggerCreateDependsOn, "depends-on", "", "trigger ID that this trigger depends on")
	triggerCreateCmd.Flags().StringVar(&triggerCreateStatus, "status", "active", "initial status: active or disabled (default active)")
	triggerCreateCmd.Flags().BoolVar(&triggerCreateYes, "yes", false, "confirm execution (bypasses dry-run)")
	_ = triggerCreateCmd.MarkFlagRequired("host")
	_ = triggerCreateCmd.MarkFlagRequired("description")
	_ = triggerCreateCmd.MarkFlagRequired("expression")

	// delete flags
	triggerDeleteCmd.Flags().BoolVar(&triggerDeleteYes, "yes", false, "confirm execution (bypasses dry-run)")

	triggerCmd.AddCommand(triggerListCmd)
	triggerCmd.AddCommand(triggerCreateCmd)
	triggerCmd.AddCommand(triggerDeleteCmd)
}
