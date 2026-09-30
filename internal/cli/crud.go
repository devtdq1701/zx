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

var entityIDKeyMap = map[string]string{
	"user":          "userid",
	"usergroup":     "usrgrpid",
	"template":      "templateid",
	"templategroup": "groupid",
	"hostgroup":     "groupid",
	"item":          "itemid",
	"proxy":         "proxyid",
	"maintenance":   "maintenanceid",
	"problem":       "eventid",
	"macro":         "macroid",
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
				idKey := entityIDKeyMap[ent]
				if idKey == "" {
					idKey = ent + "id"
				}

				fmt.Fprintf(tw, "ID\tNAME / IDENTIFIER\tDETAILS\n")
				fmt.Fprintln(tw, strings.Repeat("-", 70))

				for _, r := range raw {
					idVal, _ := r[idKey]
					if idVal == nil || idVal == "" {
						for _, k := range []string{"globalmacroid", "hostmacroid", "macroid", "groupid", "usrgrpid", "eventid"} {
							if v, found := r[k]; found && v != nil && v != "" {
								idVal = v
								break
							}
						}
					}
					nameVal, ok := r["name"]
					if !ok || nameVal == "" {
						for _, k := range []string{"username", "alias", "macro", "host", "description"} {
							if v, found := r[k]; found && v != "" {
								nameVal = v
								break
							}
						}
					}

					delete(r, idKey)
					delete(r, "name")
					delete(r, "username")
					delete(r, "alias")
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

				idKey := entityIDKeyMap[ent] + "s"
				if idKey == "s" {
					idKey = ent + "ids"
				}

				method := ent + ".get"
				if ent == "macro" {
					method = "usermacro.get"
				}

				var raw []map[string]any
				params := map[string]any{
					"output": "extend",
					idKey:    []string{args[0]},
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
		var delYes bool
		delShort := fmt.Sprintf("Delete a %s by ID (dry-run unless --yes)", ent)
		if ent == "macro" {
			delShort = "Delete a host macro by hostmacroid via usermacro.delete (dry-run unless --yes)"
		}
		delCmd := &cobra.Command{
			Use:   "delete [ID]",
			Short: delShort,
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := validateIDs(ent, args); err != nil {
					return err
				}
				method := ent + ".delete"
				if ent == "macro" {
					method = "usermacro.delete"
				}
				return runMutation(cmd, method, []string{args[0]}, delYes, fmt.Sprintf("Deleted %s '%s'.", ent, args[0]))
			},
		}
		delCmd.Flags().BoolVar(&delYes, "yes", false, "Confirm execution (bypasses dry-run)")

		entityCmd.AddCommand(listCmd)
		entityCmd.AddCommand(getCmd)
		// Zabbix has no problem.delete API.
		if ent != "problem" {
			entityCmd.AddCommand(delCmd)
		}
		rootCmd.AddCommand(entityCmd)
	}
}

func init() {
	registerCRUDRoutes()
}
