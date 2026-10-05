package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"zx/internal/zbxclient"
)

var (
	itemCreateHost      string
	itemCreateName      string
	itemCreateKey       string
	itemCreateType      string
	itemCreateValueType string
	itemCreateDelay     string
	itemCreateUnits     string
	itemCreateStatus    string
	itemCreateYes       bool

	itemUpdateDelay  string
	itemUpdateStatus string
	itemUpdateName   string
	itemUpdateUnits  string
	itemUpdateYes    bool

	itemCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new item on a host",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(itemCreateHost) == "" {
				return fmt.Errorf("--host is required")
			}
			if strings.TrimSpace(itemCreateName) == "" {
				return fmt.Errorf("--name is required")
			}
			if strings.TrimSpace(itemCreateKey) == "" {
				return fmt.Errorf("--key is required")
			}

			if err := oneOf("--status", itemCreateStatus, "active", "disabled"); err != nil {
				return err
			}
			statusVal := 0
			if itemCreateStatus == "disabled" {
				statusVal = 1
			}

			typeVal, err := zbxclient.ParseItemType(itemCreateType)
			if err != nil {
				return err
			}

			valueTypeVal, err := zbxclient.ParseValueType(itemCreateValueType)
			if err != nil {
				return err
			}

			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			host, err := client.ResolveExactHost(ctx, itemCreateHost)
			if err != nil {
				return err
			}

			params := zbxclient.CreateItemParams{
				HostID:    host.HostID,
				Name:      itemCreateName,
				Key:       itemCreateKey,
				Type:      typeVal,
				ValueType: valueTypeVal,
				Delay:     itemCreateDelay,
				Units:     itemCreateUnits,
				Status:    statusVal,
			}

			payload, err := zbxclient.BuildCreateItemPayload(params)
			if err != nil {
				return err
			}

			if !itemCreateYes {
				return runMutation(cmd, "item.create", payload, false, "")
			}

			id, err := client.CreateItem(ctx, params)
			if err != nil {
				return fmt.Errorf("item.create: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created item %s (%s).\n", id, params.Name)
			return nil
		},
	}

	itemUpdateCmd = &cobra.Command{
		Use:   "update [ITEM_ID]",
		Short: "Update an existing item by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := strings.TrimSpace(args[0])
			if itemID == "" {
				return fmt.Errorf("item ID is required")
			}

			updates := make(map[string]any)

			if cmd.Flags().Changed("delay") {
				updates["delay"] = itemUpdateDelay
			}
			if cmd.Flags().Changed("status") {
				if err := oneOf("--status", itemUpdateStatus, "active", "disabled"); err != nil {
					return err
				}
				statusVal := 0
				if itemUpdateStatus == "disabled" {
					statusVal = 1
				}
				updates["status"] = statusVal
			}
			if cmd.Flags().Changed("name") {
				if strings.TrimSpace(itemUpdateName) == "" {
					return fmt.Errorf("--name cannot be empty")
				}
				updates["name"] = itemUpdateName
			}
			if cmd.Flags().Changed("units") {
				updates["units"] = itemUpdateUnits
			}

			if len(updates) == 0 {
				return fmt.Errorf("no attributes specified to update (use --delay, --status, --name, --units)")
			}

			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			payload := map[string]any{
				"itemid": itemID,
			}
			for k, v := range updates {
				payload[k] = v
			}

			if !itemUpdateYes {
				return runMutation(cmd, "item.update", payload, false, "")
			}

			if err := client.UpdateItem(ctx, itemID, updates); err != nil {
				return fmt.Errorf("item.update: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated item %s.\n", itemID)
			return nil
		},
	}
)

func init() {
	itemCreateCmd.Flags().StringVar(&itemCreateHost, "host", "", "target host name, visible name, or IP (required)")
	itemCreateCmd.Flags().StringVar(&itemCreateName, "name", "", "item name (required)")
	itemCreateCmd.Flags().StringVar(&itemCreateKey, "key", "", "item key (required)")
	itemCreateCmd.Flags().StringVar(&itemCreateType, "type", "zabbix_active", "item type (zabbix_active, zabbix_agent, trapper, simple, calculated)")
	itemCreateCmd.Flags().StringVar(&itemCreateValueType, "value-type", "unsigned", "value type (numeric_unsigned, unsigned, numeric_float, float, char, log, text)")
	itemCreateCmd.Flags().StringVar(&itemCreateDelay, "delay", "1m", "update interval (e.g. 1m, 30s)")
	itemCreateCmd.Flags().StringVar(&itemCreateUnits, "units", "", "units of measurement")
	itemCreateCmd.Flags().StringVar(&itemCreateStatus, "status", "active", "initial status: active or disabled (default active)")
	itemCreateCmd.Flags().BoolVar(&itemCreateYes, "yes", false, "confirm execution (bypasses dry-run)")
	_ = itemCreateCmd.MarkFlagRequired("host")
	_ = itemCreateCmd.MarkFlagRequired("name")
	_ = itemCreateCmd.MarkFlagRequired("key")

	itemUpdateCmd.Flags().StringVar(&itemUpdateDelay, "delay", "", "update interval (e.g. 1m, 30s)")
	itemUpdateCmd.Flags().StringVar(&itemUpdateStatus, "status", "", "item status: active or disabled")
	itemUpdateCmd.Flags().StringVar(&itemUpdateName, "name", "", "new name for the item")
	itemUpdateCmd.Flags().StringVar(&itemUpdateUnits, "units", "", "units of measurement")
	itemUpdateCmd.Flags().BoolVar(&itemUpdateYes, "yes", false, "confirm execution (bypasses dry-run)")

	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "item" {
			cmd.AddCommand(itemCreateCmd)
			cmd.AddCommand(itemUpdateCmd)
			break
		}
	}
}
