package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"zx/internal/zbxclient"
)

var (
	actionListStatus      string
	actionListEventSource int

	actionCreateName        string
	actionCreateSubject     string
	actionCreateMessage     string
	actionCreateHostGroup   string
	actionCreateHostGroupID string
	actionCreateMediaType   string
	actionCreateMediaTypeID string
	actionCreateUser        string
	actionCreateUserID      string
	actionCreateUserGroup   string
	actionCreateUserGroupID string
	actionCreateEscPeriod   string
	actionCreateStatus      string
	actionCreateYes         bool

	actionUpdateStatus  string
	actionUpdateSubject string
	actionUpdateMessage string
	actionUpdateYes     bool

	actionDeleteYes bool

	actionCmd = &cobra.Command{
		Use:   "action",
		Short: "Manage Zabbix notification actions",
		Long:  "List, inspect, create, update, and delete Zabbix notification actions.",
	}

	actionListCmd = &cobra.Command{
		Use:   "list",
		Short: "List trigger actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}

			filter := zbxclient.ActionFilter{
				EventSource: &actionListEventSource,
			}
			if cmd.Flags().Changed("status") {
				if err := oneOf("--status", actionListStatus, "active", "disabled"); err != nil {
					return err
				}
				s := 1
				if actionListStatus == "active" {
					s = 0
				}
				filter.Status = &s
			}

			actions, err := client.GetActions(cmd.Context(), filter)
			if err != nil {
				return err
			}

			if OutputFormat() == "json" {
				return writeJSON(cmd.OutOrStdout(), actions)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
			fmt.Fprintln(tw, "ACTIONID\tNAME\tSTATUS\tESC PERIOD")
			fmt.Fprintln(tw, strings.Repeat("-", 60))
			for _, a := range actions {
				statusStr := a.Status
				switch a.Status {
				case "0":
					statusStr = "active"
				case "1":
					statusStr = "disabled"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", a.ActionID, a.Name, statusStr, a.EscPeriod)
			}
			return tw.Flush()
		},
	}

	actionGetCmd = &cobra.Command{
		Use:   "get [ACTION_ID]",
		Short: "Get detailed action configuration in JSON format",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}

			action, err := client.GetAction(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			b, err := json.MarshalIndent(action, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}

	actionCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new trigger notification action",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(actionCreateName) == "" {
				return fmt.Errorf("--name is required")
			}
			if strings.TrimSpace(actionCreateSubject) == "" {
				return fmt.Errorf("--subject is required")
			}
			if strings.TrimSpace(actionCreateMessage) == "" {
				return fmt.Errorf("--message is required")
			}

			if err := oneOf("--status", actionCreateStatus, "active", "disabled"); err != nil {
				return err
			}
			statusVal := 1
			if actionCreateStatus == "active" {
				statusVal = 0
			}

			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			hostGroupID := actionCreateHostGroupID
			if hostGroupID == "" && actionCreateHostGroup != "" {
				g, err := client.ResolveExactGroup(ctx, actionCreateHostGroup)
				if err != nil {
					return err
				}
				hostGroupID = g.GroupID
			}

			mediaTypeID := actionCreateMediaTypeID
			if mediaTypeID == "" && actionCreateMediaType != "" {
				mt, err := client.ResolveExactMediaType(ctx, actionCreateMediaType)
				if err != nil {
					return err
				}
				mediaTypeID = mt.MediaTypeID
			}

			userID := actionCreateUserID
			if userID == "" && actionCreateUser != "" {
				u, err := client.ResolveExactUser(ctx, actionCreateUser)
				if err != nil {
					return err
				}
				userID = u.UserID
			}

			userGroupID := actionCreateUserGroupID
			if userGroupID == "" && actionCreateUserGroup != "" {
				ug, err := client.ResolveExactUserGroup(ctx, actionCreateUserGroup)
				if err != nil {
					return err
				}
				userGroupID = ug.UserGroupID
			}

			params := zbxclient.CreateActionParams{
				Name:        actionCreateName,
				EventSource: 0,
				Status:      statusVal,
				EscPeriod:   actionCreateEscPeriod,
				HostGroupID: hostGroupID,
				MediaTypeID: mediaTypeID,
				UserID:      userID,
				UserGroupID: userGroupID,
				Subject:     actionCreateSubject,
				Message:     actionCreateMessage,
			}

			payload, err := zbxclient.BuildCreateActionPayload(params)
			if err != nil {
				return err
			}

			if !actionCreateYes {
				return runMutation(cmd, "action.create", payload, false, "")
			}

			id, err := client.CreateAction(ctx, params)
			if err != nil {
				return fmt.Errorf("action.create: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created action %s (%s).\n", id, params.Name)
			return nil
		},
	}

	actionUpdateCmd = &cobra.Command{
		Use:   "update [ACTION_ID]",
		Short: "Update an existing action (status, subject, message)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			actionID := args[0]
			payload := map[string]any{
				"actionid": actionID,
			}

			hasChanges := false
			if cmd.Flags().Changed("status") {
				if err := oneOf("--status", actionUpdateStatus, "active", "disabled"); err != nil {
					return err
				}
				statusVal := 1
				if actionUpdateStatus == "active" {
					statusVal = 0
				}
				payload["status"] = statusVal
				hasChanges = true
			}

			if cmd.Flags().Changed("subject") || cmd.Flags().Changed("message") {
				op := map[string]any{
					"operationtype": 0,
					"esc_period":    "0",
					"esc_step_from": 1,
					"esc_step_to":   1,
					"opmessage": map[string]any{
						"default_msg": 0,
					},
				}
				opMsg := op["opmessage"].(map[string]any)
				if actionUpdateSubject != "" {
					opMsg["subject"] = actionUpdateSubject
				}
				if actionUpdateMessage != "" {
					opMsg["message"] = actionUpdateMessage
				}
				payload["operations"] = []map[string]any{op}
				hasChanges = true
			}

			if !hasChanges {
				return fmt.Errorf("no update flags provided (use --status, --subject, or --message)")
			}

			return runMutation(cmd, "action.update", payload, actionUpdateYes, fmt.Sprintf("Updated action %s.", actionID))
		},
	}

	actionDeleteCmd = &cobra.Command{
		Use:   "delete [ACTION_ID]",
		Short: "Delete an action by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			actionID := args[0]
			return runMutation(cmd, "action.delete", []string{actionID}, actionDeleteYes, fmt.Sprintf("Deleted action %s.", actionID))
		},
	}
)

func init() {
	// list flags
	actionListCmd.Flags().StringVar(&actionListStatus, "status", "", "filter by status: active or disabled")
	actionListCmd.Flags().IntVar(&actionListEventSource, "eventsource", 0, "filter by event source (0 for triggers)")

	// create flags
	actionCreateCmd.Flags().StringVar(&actionCreateName, "name", "", "action name (required)")
	actionCreateCmd.Flags().StringVar(&actionCreateSubject, "subject", "", "notification message subject (required)")
	actionCreateCmd.Flags().StringVar(&actionCreateMessage, "message", "", "notification message body (required)")
	actionCreateCmd.Flags().StringVar(&actionCreateHostGroup, "hostgroup", "", "filter by hostgroup name")
	actionCreateCmd.Flags().StringVar(&actionCreateHostGroupID, "hostgroup-id", "", "filter by hostgroup ID")
	actionCreateCmd.Flags().StringVar(&actionCreateMediaType, "mediatype", "", "send via media type name (e.g. Telegram)")
	actionCreateCmd.Flags().StringVar(&actionCreateMediaTypeID, "mediatype-id", "", "send via media type ID")
	actionCreateCmd.Flags().StringVar(&actionCreateUser, "user", "", "send to user username")
	actionCreateCmd.Flags().StringVar(&actionCreateUserID, "user-id", "", "send to user ID")
	actionCreateCmd.Flags().StringVar(&actionCreateUserGroup, "usergroup", "", "send to user group name")
	actionCreateCmd.Flags().StringVar(&actionCreateUserGroupID, "usergroup-id", "", "send to user group ID")
	actionCreateCmd.Flags().StringVar(&actionCreateEscPeriod, "esc-period", "1h", "escalation period (default 1h)")
	actionCreateCmd.Flags().StringVar(&actionCreateStatus, "status", "disabled", "initial status: active or disabled (default disabled)")
	actionCreateCmd.Flags().BoolVar(&actionCreateYes, "yes", false, "confirm execution (bypasses dry-run)")
	_ = actionCreateCmd.MarkFlagRequired("name")
	_ = actionCreateCmd.MarkFlagRequired("subject")
	_ = actionCreateCmd.MarkFlagRequired("message")

	// update flags
	actionUpdateCmd.Flags().StringVar(&actionUpdateStatus, "status", "", "update status: active or disabled")
	actionUpdateCmd.Flags().StringVar(&actionUpdateSubject, "subject", "", "update message subject")
	actionUpdateCmd.Flags().StringVar(&actionUpdateMessage, "message", "", "update message body")
	actionUpdateCmd.Flags().BoolVar(&actionUpdateYes, "yes", false, "confirm execution (bypasses dry-run)")

	// delete flags
	actionDeleteCmd.Flags().BoolVar(&actionDeleteYes, "yes", false, "confirm execution (bypasses dry-run)")

	actionCmd.AddCommand(actionListCmd)
	actionCmd.AddCommand(actionGetCmd)
	actionCmd.AddCommand(actionCreateCmd)
	actionCmd.AddCommand(actionUpdateCmd)
	actionCmd.AddCommand(actionDeleteCmd)
}
