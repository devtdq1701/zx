package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var crudEntities = []string{
	"user",
	"usergroup",
	"template",
	"templategroup",
	"hostgroup",
	"item",
	"proxy",
	"maintenance",
	"problem",
	"macro",
}

func registerCRUDRoutes() {
	for _, entity := range crudEntities {
		ent := entity
		entityCmd := &cobra.Command{
			Use:   ent,
			Short: fmt.Sprintf("Manage Zabbix %s objects", ent),
		}

		// List sub-command
		listCmd := &cobra.Command{
			Use:   "list",
			Short: fmt.Sprintf("List %s objects", ent),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, _, _, err := GetActiveClient()
				if err != nil {
					return err
				}

				method := ent + ".get"
				if ent == "macro" {
					method = "usermacro.get"
				}

				var raw []map[string]any
				params := map[string]any{
					"output": "extend",
					"limit":  50,
				}

				if err := client.Call(cmd.Context(), method, params, &raw); err != nil {
					return fmt.Errorf("%s.get failed: %w", ent, err)
				}

				if len(raw) == 0 {
					fmt.Printf("No %s records found.\n", ent)
					return nil
				}

				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
				// Pick primary key and name/description
				idKey := ent + "id"
				if ent == "macro" {
					idKey = "macroid"
				}

				fmt.Fprintf(tw, "ID\tNAME / IDENTIFIER\tDETAILS\n")
				fmt.Fprintln(tw, strings.Repeat("-", 70))

				for _, r := range raw {
					idVal, _ := r[idKey]
					nameVal, ok := r["name"]
					if !ok {
						nameVal, ok = r["macro"]
						if !ok {
							nameVal, _ = r["host"]
						}
					}

					// Summary of other fields
					delete(r, idKey)
					delete(r, "name")
					delete(r, "macro")
					delete(r, "host")

					brief, _ := json.Marshal(r)
					briefStr := string(brief)
					if len(briefStr) > 60 {
						briefStr = briefStr[:57] + "..."
					}

					fmt.Fprintf(tw, "%v\t%v\t%s\n", idVal, nameVal, briefStr)
				}
				_ = tw.Flush()
				return nil
			},
		}

		// Get sub-command
		getCmd := &cobra.Command{
			Use:   "get [ID]",
			Short: fmt.Sprintf("Get detailed JSON for a specific %s", ent),
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, _, _, err := GetActiveClient()
				if err != nil {
					return err
				}

				idKey := ent + "ids"
				if ent == "macro" {
					idKey = "macroids"
				}

				method := ent + ".get"
				if ent == "macro" {
					method = "usermacro.get"
				}

				var raw []map[string]any
				params := map[string]any{
					"output": "extend",
					idKey:   []string{args[0]},
				}

				if err := client.Call(cmd.Context(), method, params, &raw); err != nil {
					return fmt.Errorf("%s.get failed: %w", ent, err)
				}

				if len(raw) == 0 {
					return fmt.Errorf("%s with id '%s' not found", ent, args[0])
				}

				pretty, _ := json.MarshalIndent(raw[0], "", "  ")
				fmt.Println(string(pretty))
				return nil
			},
		}

		// Delete sub-command
		delCmd := &cobra.Command{
			Use:   "delete [ID]",
			Short: fmt.Sprintf("Delete a %s by ID", ent),
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, _, _, err := GetActiveClient()
				if err != nil {
					return err
				}

				method := ent + ".delete"
				if ent == "macro" {
					method = "usermacro.delete"
				}

				var result any
				if err := client.Call(cmd.Context(), method, []string{args[0]}, &result); err != nil {
					return fmt.Errorf("%s.delete failed: %w", ent, err)
				}

				fmt.Printf("Successfully deleted %s '%s'.\n", ent, args[0])
				return nil
			},
		}

		entityCmd.AddCommand(listCmd)
		entityCmd.AddCommand(getCmd)
		entityCmd.AddCommand(delCmd)
		rootCmd.AddCommand(entityCmd)
	}
}

func init() {
	registerCRUDRoutes()
}
