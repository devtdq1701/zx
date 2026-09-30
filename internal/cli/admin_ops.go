package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"zx/internal/zbxclient"
)

func runMutation(cmd *cobra.Command, method string, params any, yes bool, successMsg string) error {
	client, _, _, err := GetActiveClient()
	if err != nil {
		return err
	}
	if !yes {
		b, err := json.MarshalIndent(params, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[DRY-RUN] The following JSON-RPC call would be made:\nMethod: %s\nParams: %s\nTo execute this action, rerun with --yes\n", method, string(b))
		return nil
	}
	var res json.RawMessage
	if err := client.Call(cmd.Context(), method, params, &res); err != nil {
		return fmt.Errorf("%s failed: %w", method, err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), successMsg)
	return nil
}

func resolveSingleHostID(cmd *cobra.Command, target string) (string, error) {
	client, _, _, err := GetActiveClient()
	if err != nil {
		return "", err
	}
	h, err := client.ResolveExactHost(cmd.Context(), target)
	return h.HostID, err
}

func resolveSingleGroupID(cmd *cobra.Command, nameOrID string) (string, error) {
	client, _, _, err := GetActiveClient()
	if err != nil {
		return "", err
	}
	g, err := client.ResolveExactGroup(cmd.Context(), nameOrID)
	return g.GroupID, err
}

func resolveSingleTemplateID(cmd *cobra.Command, nameOrID string) (string, error) {
	client, _, _, err := GetActiveClient()
	if err != nil {
		return "", err
	}
	tp, err := client.ResolveExactTemplate(cmd.Context(), nameOrID)
	return tp.TemplateID, err
}

// 1. create_maintenance_definition
var (
	maintDesc      string
	maintHosts     string
	maintGroups    string
	maintPeriod    string
	maintDataColl  string
	maintYes       bool
	createMaintCmd = &cobra.Command{
		Use:   "create_maintenance_definition [NAME]",
		Short: "Create a one-time maintenance definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := oneOf("--data-collection", maintDataColl, "with_data", "no_data"); err != nil {
				return err
			}
			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}
			dur, perr := time.ParseDuration(maintPeriod)
			if perr != nil {
				// Fallback: check if integer seconds
				sec, sErr := strconv.Atoi(maintPeriod)
				if sErr != nil {
					return fmt.Errorf("invalid period %q (use e.g. 1h, 30m): %w", maintPeriod, perr)
				}
				dur = time.Duration(sec) * time.Second
			}
			if dur <= 0 {
				return fmt.Errorf("--period must be positive, got %q", maintPeriod)
			}
			now := time.Now().Unix()
			activeTill := now + int64(dur.Seconds())

			var hostIDs []string
			if maintHosts != "" {
				for _, h := range splitCSV(maintHosts) {
					hid, err := resolveSingleHostID(cmd, h)
					if err != nil {
						return err
					}
					hostIDs = append(hostIDs, hid)
				}
			}

			var groupIDs []string
			if maintGroups != "" {
				for _, g := range splitCSV(maintGroups) {
					gid, err := resolveSingleGroupID(cmd, g)
					if err != nil {
						return err
					}
					groupIDs = append(groupIDs, gid)
				}
			}

			if len(hostIDs) == 0 && len(groupIDs) == 0 {
				return fmt.Errorf("specify at least one --host or --hostgroup")
			}
			maintType := 0
			if maintDataColl == "no_data" {
				maintType = 1
			}

			params := map[string]any{
				"name":             name,
				"active_since":     now,
				"active_till":      activeTill,
				"description":      maintDesc,
				"maintenance_type": maintType,
				"timeperiods": []map[string]any{{
					"timeperiod_type": 0,
					"start_date":      now,
					"period":          int(dur.Seconds()),
				}},
			}
			// Zabbix 6.0 replaced hostids/groupids with hosts/groups objects.
			v60, err := client.APIAtLeast(cmd.Context(), 6, 0)
			if err != nil {
				return err
			}
			if v60 {
				if len(hostIDs) > 0 {
					params["hosts"] = idObjects("hostid", hostIDs)
				}
				if len(groupIDs) > 0 {
					params["groups"] = idObjects("groupid", groupIDs)
				}
			} else {
				params["hostids"] = append([]string{}, hostIDs...)
				params["groupids"] = append([]string{}, groupIDs...)
			}

			return runMutation(cmd, "maintenance.create", params, maintYes, fmt.Sprintf("Created maintenance definition %q.", name))
		},
	}
)

// 2. remove_maintenance_definition
var (
	removeMaintYes bool
	removeMaintCmd = &cobra.Command{
		Use:   "remove_maintenance_definition [NAME_OR_ID]",
		Short: "Remove a maintenance definition by name or ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			maintID := target
			if _, err := strconv.Atoi(target); err != nil {
				client, _, _, err := GetActiveClient()
				if err != nil {
					return err
				}
				var maints []struct {
					MaintenanceID string `json:"maintenanceid"`
					Name          string `json:"name"`
				}
				err = client.Call(cmd.Context(), "maintenance.get", map[string]any{
					"filter": map[string]string{"name": target},
					"output": []string{"maintenanceid", "name"},
				}, &maints)
				if err != nil {
					return err
				}
				if len(maints) == 0 {
					return fmt.Errorf("maintenance definition not found: %s", target)
				}
				maintID = maints[0].MaintenanceID
			}

			return runMutation(cmd, "maintenance.delete", []string{maintID}, removeMaintYes, fmt.Sprintf("Removed maintenance definition %s.", target))
		},
	}
)

// 3. acknowledge_event
var (
	ackMsg   string
	ackClose bool
	ackYes   bool
	ackCmd   = &cobra.Command{
		Use:   "acknowledge_event [EVENT_IDS]",
		Short: "Acknowledge event(s) by ID (comma-separated)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			eids := splitCSV(args[0])
			if err := validateIDs("event", eids); err != nil {
				return err
			}
			if strings.TrimSpace(ackMsg) == "" {
				return fmt.Errorf("acknowledge message must not be empty (-m)")
			}
			action := 2 | 4 // ack + message
			if ackClose {
				action |= 1 // close
			}
			params := map[string]any{
				"eventids": eids,
				"action":   action,
				"message":  ackMsg,
			}
			msg := fmt.Sprintf("Acknowledged %d event(s).", len(eids))
			if ackClose {
				msg = fmt.Sprintf("Acknowledged and closed %d event(s).", len(eids))
			}
			return runMutation(cmd, "event.acknowledge", params, ackYes, msg)
		},
	}
)

// 4. acknowledge_trigger_last_event
var (
	ackTrigMsg   string
	ackTrigClose bool
	ackTrigYes   bool
	ackTrigCmd   = &cobra.Command{
		Use:   "acknowledge_trigger_last_event [TRIGGER_IDS]",
		Short: "Acknowledge the latest active event for the given trigger(s)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trigIDs := splitCSV(args[0])
			if err := validateIDs("trigger", trigIDs); err != nil {
				return err
			}
			if strings.TrimSpace(ackTrigMsg) == "" {
				return fmt.Errorf("acknowledge message must not be empty (-m)")
			}
			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}
			var problems []struct {
				EventID  string `json:"eventid"`
				ObjectID string `json:"objectid"`
			}
			err = client.Call(cmd.Context(), "problem.get", map[string]any{
				"objectids": trigIDs,
				"output":    []string{"eventid", "objectid"},
				"sortfield": "eventid",
				"sortorder": "DESC",
			}, &problems)
			if err != nil {
				return err
			}
			if len(problems) == 0 {
				return fmt.Errorf("no active problem found for trigger(s): %s", args[0])
			}
			seen := make(map[string]bool)
			var eventIDs []string
			for _, p := range problems {
				if !seen[p.ObjectID] {
					seen[p.ObjectID] = true
					eventIDs = append(eventIDs, p.EventID)
				}
			}
			action := 2 | 4
			if ackTrigClose {
				action |= 1
			}
			params := map[string]any{
				"eventids": eventIDs,
				"action":   action,
				"message":  ackTrigMsg,
			}
			return runMutation(cmd, "event.acknowledge", params, ackTrigYes, fmt.Sprintf("Acknowledged %d event(s) for trigger(s).", len(eventIDs)))
		},
	}
)

// 5. show_alarms (Read-only) — same query as zabbix-cli show_alarms.
var (
	alarmHostgroup   string
	alarmPriority    int
	alarmDescription string
	alarmIncludeAck  bool
	showAlarmsCmd    = &cobra.Command{
		Use:   "show_alarms [TARGET]",
		Short: "Show triggers currently in PROBLEM state (like zabbix-cli show_alarms)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if alarmPriority < -1 || alarmPriority > 5 {
				return fmt.Errorf("invalid --priority %d; expected 0..5", alarmPriority)
			}
			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}
			loc, err := Location()
			if err != nil {
				return err
			}
			filter := map[string]any{"value": 1}
			if alarmPriority >= 0 {
				filter["priority"] = alarmPriority
			}
			params := map[string]any{
				"output":            []string{"triggerid", "description", "priority", "lastchange"},
				"selectHosts":       []string{"host"},
				"filter":            filter,
				"skipDependent":     true,
				"monitored":         true,
				"active":            true,
				"expandDescription": true,
				"sortfield":         "lastchange",
				"sortorder":         "DESC",
			}
			if !alarmIncludeAck {
				params["withLastEventUnacknowledged"] = true
			}
			if alarmDescription != "" {
				params["search"] = map[string]string{"description": alarmDescription}
			}
			if len(args) > 0 && args[0] != "" {
				hid, err := resolveSingleHostID(cmd, args[0])
				if err != nil {
					return err
				}
				params["hostids"] = []string{hid}
			}
			if alarmHostgroup != "" {
				var gids []string
				for _, g := range splitCSV(alarmHostgroup) {
					gid, err := resolveSingleGroupID(cmd, g)
					if err != nil {
						return err
					}
					gids = append(gids, gid)
				}
				params["groupids"] = gids
			}

			type alarmRecord struct {
				TriggerID   string `json:"triggerid"`
				Description string `json:"description"`
				Priority    string `json:"priority"`
				LastChange  string `json:"lastchange"`
				Hosts       []struct {
					Host string `json:"host"`
				} `json:"hosts"`
			}
			alarms := []alarmRecord{}
			if err := client.Call(cmd.Context(), "trigger.get", params, &alarms); err != nil {
				return fmt.Errorf("trigger.get: %w", err)
			}
			if alarms == nil {
				alarms = []alarmRecord{}
			}
			if OutputFormat() == "json" {
				return writeJSON(cmd.OutOrStdout(), alarms)
			}

			sevNames := map[string]string{
				"0": "Not classified", "1": "Information", "2": "Warning",
				"3": "Average", "4": "High", "5": "Disaster",
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
			fmt.Fprintln(tw, "TRIGGERID\tHOST\tSEVERITY\tLAST CHANGE\tDESCRIPTION")
			fmt.Fprintln(tw, strings.Repeat("-", 80))
			for _, a := range alarms {
				var hosts []string
				for _, h := range a.Hosts {
					hosts = append(hosts, h.Host)
				}
				sev := sevNames[a.Priority]
				if sev == "" {
					sev = a.Priority
				}
				sec, _ := strconv.ParseInt(a.LastChange, 10, 64)
				when := time.Unix(sec, 0).In(loc).Format("2006-01-02 15:04:05")
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", a.TriggerID, strings.Join(hosts, ","), sev, when, a.Description)
			}
			return tw.Flush()
		},
	}
)

// 6. show_last_values (Read-only)
var showLastValuesCmd = &cobra.Command{
	Use:   "show_last_values [TARGET] [ITEM_PATTERN]",
	Short: "Show latest collected values for items on a host",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, _, err := GetActiveClient()
		if err != nil {
			return err
		}
		target := args[0]
		pattern := ""
		if len(args) > 1 {
			pattern = strings.ToLower(args[1])
		}

		hid, err := resolveSingleHostID(cmd, target)
		if err != nil {
			return err
		}

		type ItemVal struct {
			ItemID    string `json:"itemid"`
			Name      string `json:"name"`
			Key       string `json:"key_"`
			LastValue string `json:"lastvalue"`
			LastClock string `json:"lastclock"`
			Units     string `json:"units"`
		}
		var items []ItemVal
		err = client.Call(cmd.Context(), "item.get", map[string]any{
			"hostids":   []string{hid},
			"output":    []string{"itemid", "name", "key_", "lastvalue", "lastclock", "units"},
			"monitored": true,
		}, &items)
		if err != nil {
			return err
		}

		filtered := []ItemVal{}
		for _, it := range items {
			if pattern == "" || strings.Contains(strings.ToLower(it.Name), pattern) || strings.Contains(strings.ToLower(it.Key), pattern) {
				filtered = append(filtered, it)
			}
		}

		if OutputFormat() == "json" {
			return writeJSON(cmd.OutOrStdout(), filtered)
		}

		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "ITEMID\tKEY\tLAST VALUE\tLAST CLOCK")
		fmt.Fprintln(tw, strings.Repeat("-", 80))
		loc, _ := Location()
		if loc == nil {
			loc = time.Local
		}
		for _, it := range filtered {
			cSec, _ := strconv.ParseInt(it.LastClock, 10, 64)
			cTime := "-"
			if cSec > 0 {
				cTime = time.Unix(cSec, 0).In(loc).Format("2006-01-02 15:04:05")
			}
			val := it.LastValue
			if it.Units != "" && val != "" {
				val += " " + it.Units
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", it.ItemID, it.Key, val, cTime)
		}
		_ = tw.Flush()
		return nil
	},
}

// 7. monitor_host
var (
	monitorStatus  string
	monitorYes     bool
	monitorHostCmd = &cobra.Command{
		Use:   "monitor_host [TARGET]",
		Short: "Enable or disable monitoring for a host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := oneOf("--status", monitorStatus, "monitored", "unmonitored"); err != nil {
				return err
			}
			hid, err := resolveSingleHostID(cmd, args[0])
			if err != nil {
				return err
			}
			statusVal := 0
			if monitorStatus == "unmonitored" {
				statusVal = 1
			}
			params := map[string]any{
				"hostid": hid,
				"status": statusVal,
			}
			statusName := "monitored"
			if statusVal == 1 {
				statusName = "unmonitored"
			}
			return runMutation(cmd, "host.update", params, monitorYes, fmt.Sprintf("Updated host %s monitoring status to %s.", args[0], statusName))
		},
	}
)

// 8. add_host_to_hostgroup
var (
	addHostGroupYes   bool
	addHostToGroupCmd = &cobra.Command{
		Use:   "add_host_to_hostgroup [TARGET] [HOSTGROUP]",
		Short: "Add a host to a hostgroup",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			hid, err := resolveSingleHostID(cmd, args[0])
			if err != nil {
				return err
			}
			gid, err := resolveSingleGroupID(cmd, args[1])
			if err != nil {
				return err
			}
			params := map[string]any{
				"groups": []map[string]string{{"groupid": gid}},
				"hosts":  []map[string]string{{"hostid": hid}},
			}
			return runMutation(cmd, "hostgroup.massadd", params, addHostGroupYes, fmt.Sprintf("Added host %s to hostgroup %s.", args[0], args[1]))
		},
	}
)

// 9. remove_host_from_hostgroup
var (
	remHostGroupYes     bool
	remHostFromGroupCmd = &cobra.Command{
		Use:   "remove_host_from_hostgroup [TARGET] [HOSTGROUP]",
		Short: "Remove a host from a hostgroup",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			hid, err := resolveSingleHostID(cmd, args[0])
			if err != nil {
				return err
			}
			gid, err := resolveSingleGroupID(cmd, args[1])
			if err != nil {
				return err
			}
			params := map[string]any{
				"groupids": []string{gid},
				"hostids":  []string{hid},
			}
			return runMutation(cmd, "hostgroup.massremove", params, remHostGroupYes, fmt.Sprintf("Removed host %s from hostgroup %s.", args[0], args[1]))
		},
	}
)

// 10. link_template_to_host
var (
	linkTmplYes     bool
	linkTemplateCmd = &cobra.Command{
		Use:   "link_template_to_host [TARGET] [TEMPLATE]",
		Short: "Link a template to a host",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			hid, err := resolveSingleHostID(cmd, args[0])
			if err != nil {
				return err
			}
			tid, err := resolveSingleTemplateID(cmd, args[1])
			if err != nil {
				return err
			}
			params := map[string]any{
				"hosts":     []map[string]string{{"hostid": hid}},
				"templates": []map[string]string{{"templateid": tid}},
			}
			return runMutation(cmd, "host.massadd", params, linkTmplYes, fmt.Sprintf("Linked template %s to host %s.", args[1], args[0]))
		},
	}
)

// 11. unlink_template_from_host
var (
	unlinkClear       bool
	unlinkYes         bool
	unlinkTemplateCmd = &cobra.Command{
		Use:   "unlink_template_from_host [TARGET] [TEMPLATE]",
		Short: "Unlink a template from a host (optional --clear)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			hid, err := resolveSingleHostID(cmd, args[0])
			if err != nil {
				return err
			}
			tid, err := resolveSingleTemplateID(cmd, args[1])
			if err != nil {
				return err
			}
			params := map[string]any{
				"hostids": []string{hid},
			}
			if unlinkClear {
				params["templateids_clear"] = []string{tid}
			} else {
				params["templateids"] = []string{tid}
			}
			return runMutation(cmd, "host.massremove", params, unlinkYes, fmt.Sprintf("Unlinked template %s from host %s.", args[1], args[0]))
		},
	}
)

// 12. update_user_media
var (
	updMediaPeriod     string
	updMediaSeverity   int
	updMediaActive     bool
	updMediaYes        bool
	updateUserMediaCmd = &cobra.Command{
		Use:   "update_user_media [USERNAME] [MEDIATYPE] [SENDTO]",
		Short: "Update user notification media configuration",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if updMediaSeverity < 0 || updMediaSeverity > 63 {
				return fmt.Errorf("invalid --severity %d; expected a bitmask 0..63", updMediaSeverity)
			}
			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			u, err := client.ResolveExactUser(ctx, args[0])
			if err != nil {
				return err
			}
			mt, err := client.ResolveExactMediaType(ctx, args[1])
			if err != nil {
				return err
			}
			raw, err := client.GetUserMediasRaw(ctx, u.UserID)
			if err != nil {
				return err
			}
			medias := zbxclient.EditableMedias(raw)
			active := "0"
			if !updMediaActive {
				active = "1"
			}
			entry := map[string]any{
				"mediatypeid": mt.MediaTypeID,
				"sendto":      zbxclient.SendToValue(mt, args[2]),
				"active":      active,
				"severity":    updMediaSeverity,
				"period":      updMediaPeriod,
			}
			var idx []int
			for i, m := range medias {
				if fmt.Sprint(m["mediatypeid"]) == mt.MediaTypeID {
					idx = append(idx, i)
				}
			}
			switch len(idx) {
			case 0:
				medias = append(medias, entry)
			case 1:
				medias[idx[0]] = entry
			default:
				return fmt.Errorf("user %s has %d media of type %s; refusing to guess which one to update", u.Login, len(idx), mt.Name)
			}
			params := map[string]any{"userid": u.UserID, "medias": medias}
			return runMutation(cmd, "user.update", params, updMediaYes, fmt.Sprintf("Updated %s media for user %s.", mt.Name, u.Login))
		},
	}
)

// 13. define_host_macro
var (
	hostMacroDesc      string
	hostMacroType      int
	hostMacroYes       bool
	defineHostMacroCmd = &cobra.Command{
		Use:   "define_host_macro [TARGET] [MACRO] [VALUE]",
		Short: "Define or update a host macro ({$MACRO})",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateMacro(args[1]); err != nil {
				return err
			}
			if err := validateMacroType(hostMacroType); err != nil {
				return err
			}
			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}
			hid, err := resolveSingleHostID(cmd, args[0])
			if err != nil {
				return err
			}
			macro := args[1]
			value := args[2]

			var existing []struct {
				HostMacroID string `json:"hostmacroid"`
				Macro       string `json:"macro"`
			}
			err = client.Call(cmd.Context(), "usermacro.get", map[string]any{
				"hostids": []string{hid},
				"filter":  map[string]string{"macro": macro},
				"output":  []string{"hostmacroid", "macro"},
			}, &existing)
			if err != nil {
				return err
			}

			if len(existing) > 0 {
				params := map[string]any{
					"hostmacroid": existing[0].HostMacroID,
					"value":       value,
					"description": hostMacroDesc,
					"type":        hostMacroType,
				}
				return runMutation(cmd, "usermacro.update", params, hostMacroYes, fmt.Sprintf("Updated macro %s on host %s.", macro, args[0]))
			}

			params := map[string]any{
				"hostid":      hid,
				"macro":       macro,
				"value":       value,
				"description": hostMacroDesc,
				"type":        hostMacroType,
			}
			return runMutation(cmd, "usermacro.create", params, hostMacroYes, fmt.Sprintf("Created macro %s on host %s.", macro, args[0]))
		},
	}
)

// 14. define_global_macro
var (
	globalMacroDesc      string
	globalMacroType      int
	globalMacroYes       bool
	defineGlobalMacroCmd = &cobra.Command{
		Use:   "define_global_macro [MACRO] [VALUE]",
		Short: "Define or update a global macro ({$MACRO})",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateMacro(args[0]); err != nil {
				return err
			}
			if err := validateMacroType(globalMacroType); err != nil {
				return err
			}
			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}
			macro := args[0]
			value := args[1]

			var existing []struct {
				GlobalMacroID string `json:"globalmacroid"`
				Macro         string `json:"macro"`
			}
			err = client.Call(cmd.Context(), "usermacro.get", map[string]any{
				"globalmacro": true,
				"filter":      map[string]string{"macro": macro},
				"output":      []string{"globalmacroid", "macro"},
			}, &existing)
			if err != nil {
				return err
			}

			if len(existing) > 0 {
				params := map[string]any{
					"globalmacroid": existing[0].GlobalMacroID,
					"value":         value,
					"description":   globalMacroDesc,
					"type":          globalMacroType,
				}
				return runMutation(cmd, "usermacro.updateglobal", params, globalMacroYes, fmt.Sprintf("Updated global macro %s.", macro))
			}

			params := map[string]any{
				"macro":       macro,
				"value":       value,
				"description": globalMacroDesc,
				"type":        globalMacroType,
			}
			return runMutation(cmd, "usermacro.createglobal", params, globalMacroYes, fmt.Sprintf("Created global macro %s.", macro))
		},
	}
)

// 15. export_configuration (Read-only)
var (
	expConfigFormat string
	expConfigHosts  string
	expConfigTmpls  string
	expConfigGroups string
	exportConfigCmd = &cobra.Command{
		Use:   "export_configuration",
		Short: "Export Zabbix configuration in XML, JSON, or YAML format",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := oneOf("--format", expConfigFormat, "json", "xml", "yaml"); err != nil {
				return err
			}
			client, _, _, err := GetActiveClient()
			if err != nil {
				return err
			}
			options := make(map[string]any)
			if expConfigHosts != "" {
				var hids []string
				for _, h := range splitCSV(expConfigHosts) {
					hid, err := resolveSingleHostID(cmd, h)
					if err != nil {
						return err
					}
					hids = append(hids, hid)
				}
				options["hosts"] = hids
			}
			if expConfigTmpls != "" {
				var tids []string
				for _, t := range splitCSV(expConfigTmpls) {
					tid, err := resolveSingleTemplateID(cmd, t)
					if err != nil {
						return err
					}
					tids = append(tids, tid)
				}
				options["templates"] = tids
			}
			if expConfigGroups != "" {
				var gids []string
				for _, g := range splitCSV(expConfigGroups) {
					gid, err := resolveSingleGroupID(cmd, g)
					if err != nil {
						return err
					}
					gids = append(gids, gid)
				}
				// Zabbix 6.2 split host groups from template groups.
				v62, err := client.APIAtLeast(cmd.Context(), 6, 2)
				if err != nil {
					return err
				}
				key := "groups"
				if v62 {
					key = "host_groups"
				}
				options[key] = gids
			}

			if len(options) == 0 {
				return fmt.Errorf("specify at least one of --hosts, --templates, --hostgroups")
			}

			params := map[string]any{
				"format":  expConfigFormat,
				"options": options,
			}

			var exported string
			if err := client.Call(cmd.Context(), "configuration.export", params, &exported); err != nil {
				return fmt.Errorf("configuration.export failed: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), exported)
			return nil
		},
	}
)

func init() {
	// Flags for 1
	createMaintCmd.Flags().StringVar(&maintDesc, "description", "", "Maintenance description")
	createMaintCmd.Flags().StringVar(&maintHosts, "host", "", "Comma-separated host(s)")
	createMaintCmd.Flags().StringVar(&maintGroups, "hostgroup", "", "Comma-separated hostgroup(s)")
	createMaintCmd.Flags().StringVar(&maintPeriod, "period", "1h", "Duration period (e.g. 1h, 30m, 2h45m)")
	createMaintCmd.Flags().StringVar(&maintDataColl, "data-collection", "with_data", "Data collection: with_data or no_data")
	createMaintCmd.Flags().BoolVar(&maintYes, "yes", false, "Confirm execution (bypasses dry-run)")

	// Flags for 2
	removeMaintCmd.Flags().BoolVar(&removeMaintYes, "yes", false, "Confirm execution (bypasses dry-run)")

	// Flags for 3
	ackCmd.Flags().StringVarP(&ackMsg, "message", "m", "[zx] Acknowledged via CLI", "Acknowledgment message")
	ackCmd.Flags().BoolVar(&ackClose, "close", false, "Close problem event")
	ackCmd.Flags().BoolVar(&ackYes, "yes", false, "Confirm execution (bypasses dry-run)")

	// Flags for 4
	ackTrigCmd.Flags().StringVarP(&ackTrigMsg, "message", "m", "[zx] Acknowledged via CLI", "Acknowledgment message")
	ackTrigCmd.Flags().BoolVar(&ackTrigClose, "close", false, "Close problem event")
	ackTrigCmd.Flags().BoolVar(&ackTrigYes, "yes", false, "Confirm execution (bypasses dry-run)")

	// Flags for 5
	showAlarmsCmd.Flags().StringVar(&alarmHostgroup, "hostgroup", "", "Comma-separated hostgroup(s) to filter by")
	showAlarmsCmd.Flags().IntVar(&alarmPriority, "priority", -1, "Only this priority (0=Not classified..5=Disaster)")
	showAlarmsCmd.Flags().StringVar(&alarmDescription, "description", "", "Only triggers whose description contains this text")
	showAlarmsCmd.Flags().BoolVar(&alarmIncludeAck, "ack", false, "Include alarms whose last event is acknowledged (zabbix-cli --ack)")

	// Flags for 7
	monitorHostCmd.Flags().StringVar(&monitorStatus, "status", "monitored", "Monitoring status: monitored or unmonitored")
	monitorHostCmd.Flags().BoolVar(&monitorYes, "yes", false, "Confirm execution (bypasses dry-run)")

	// Flags for 8
	addHostToGroupCmd.Flags().BoolVar(&addHostGroupYes, "yes", false, "Confirm execution (bypasses dry-run)")

	// Flags for 9
	remHostFromGroupCmd.Flags().BoolVar(&remHostGroupYes, "yes", false, "Confirm execution (bypasses dry-run)")

	// Flags for 10
	linkTemplateCmd.Flags().BoolVar(&linkTmplYes, "yes", false, "Confirm execution (bypasses dry-run)")

	// Flags for 11
	unlinkTemplateCmd.Flags().BoolVar(&unlinkClear, "clear", false, "Clear all items and triggers from host")
	unlinkTemplateCmd.Flags().BoolVar(&unlinkYes, "yes", false, "Confirm execution (bypasses dry-run)")

	// Flags for 12
	updateUserMediaCmd.Flags().StringVar(&updMediaPeriod, "period", "1-7,00:00-24:00", "Active time period")
	updateUserMediaCmd.Flags().IntVar(&updMediaSeverity, "severity", 63, "Active severity bitmask")
	updateUserMediaCmd.Flags().BoolVar(&updMediaActive, "active", true, "Enable or disable media")
	updateUserMediaCmd.Flags().BoolVar(&updMediaYes, "yes", false, "Confirm execution (bypasses dry-run)")

	// Flags for 13
	defineHostMacroCmd.Flags().StringVar(&hostMacroDesc, "description", "", "Macro description")
	defineHostMacroCmd.Flags().IntVar(&hostMacroType, "type", 0, "Macro type (0=Text, 1=Secret, 2=Vault)")
	defineHostMacroCmd.Flags().BoolVar(&hostMacroYes, "yes", false, "Confirm execution (bypasses dry-run)")

	// Flags for 14
	defineGlobalMacroCmd.Flags().StringVar(&globalMacroDesc, "description", "", "Macro description")
	defineGlobalMacroCmd.Flags().IntVar(&globalMacroType, "type", 0, "Macro type (0=Text, 1=Secret, 2=Vault)")
	defineGlobalMacroCmd.Flags().BoolVar(&globalMacroYes, "yes", false, "Confirm execution (bypasses dry-run)")

	// Flags for 15
	exportConfigCmd.Flags().StringVar(&expConfigFormat, "format", "json", "Output format: json, xml, or yaml")
	exportConfigCmd.Flags().StringVar(&expConfigHosts, "hosts", "", "Comma-separated host names")
	exportConfigCmd.Flags().StringVar(&expConfigTmpls, "templates", "", "Comma-separated template names")
	exportConfigCmd.Flags().StringVar(&expConfigGroups, "hostgroups", "", "Comma-separated hostgroup names")

	// Register all 15 commands into rootCmd
	rootCmd.AddCommand(createMaintCmd)
	rootCmd.AddCommand(removeMaintCmd)
	rootCmd.AddCommand(ackCmd)
	rootCmd.AddCommand(ackTrigCmd)
	rootCmd.AddCommand(showAlarmsCmd)
	rootCmd.AddCommand(showLastValuesCmd)
	rootCmd.AddCommand(monitorHostCmd)
	rootCmd.AddCommand(addHostToGroupCmd)
	rootCmd.AddCommand(remHostFromGroupCmd)
	rootCmd.AddCommand(linkTemplateCmd)
	rootCmd.AddCommand(unlinkTemplateCmd)
	rootCmd.AddCommand(updateUserMediaCmd)
	rootCmd.AddCommand(defineHostMacroCmd)
	rootCmd.AddCommand(defineGlobalMacroCmd)
	rootCmd.AddCommand(exportConfigCmd)
}
