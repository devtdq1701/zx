package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestAdminOpsDryRun(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version": "7.0.0",
	})
	defer cleanup()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"acknowledge_event", "1001,1002", "-m", "testing dry-run"})

	err := rootCmd.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[DRY-RUN]") {
		t.Errorf("expected [DRY-RUN] in output, got: %s", out)
	}
	if !strings.Contains(out, "event.acknowledge") {
		t.Errorf("expected event.acknowledge method in output, got: %s", out)
	}

	// Verify no event.acknowledge was called on server
	for _, call := range h.calls {
		if call == "event.acknowledge" {
			t.Errorf("event.acknowledge should NOT be called in dry-run mode")
		}
	}
}

func TestAdminOpsAcknowledgeEvent(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version":   "7.0.0",
		"event.acknowledge": map[string]any{"eventids": []string{"1001"}},
	})
	defer cleanup()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"acknowledge_event", "1001", "-m", "fix verified", "--close", "--yes"})

	err := rootCmd.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for i, c := range h.calls {
		if c == "event.acknowledge" {
			found = true
			req := h.requests[i]
			if action, ok := req["action"].(float64); !ok || int(action) != 7 { // 2+4+1 = 7
				t.Errorf("expected action=7, got %v", req["action"])
			}
			if msg, ok := req["message"].(string); !ok || msg != "fix verified" {
				t.Errorf("expected message 'fix verified', got %v", req["message"])
			}
		}
	}
	if !found {
		t.Fatalf("event.acknowledge was not called")
	}
}

func TestAdminOpsMaintenance(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version":    "7.0.0",
		"hostgroup.get":      []map[string]any{{"groupid": "5", "name": "G1"}},
		"maintenance.create": map[string]any{"maintenanceids": []string{"1"}},
		"maintenance.delete": []string{"1"},
	})
	defer cleanup()

	// Test create
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"create_maintenance_definition", "Weekly-Patch", "--hostgroup", "G1", "--period", "2h", "--yes"})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("create maint error: %v", err)
	}

	createFound := false
	for i, c := range h.calls {
		if c == "maintenance.create" {
			createFound = true
			req := h.requests[i]
			if name, _ := req["name"].(string); name != "Weekly-Patch" {
				t.Errorf("expected name Weekly-Patch, got %v", name)
			}
		}
	}
	if !createFound {
		t.Fatalf("maintenance.create was not called")
	}

	// Test remove
	buf.Reset()
	rootCmd.SetArgs([]string{"remove_maintenance_definition", "1", "--yes"})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("remove maint error: %v", err)
	}
	delFound := false
	for _, c := range h.calls {
		if c == "maintenance.delete" {
			delFound = true
		}
	}
	if !delFound {
		t.Fatalf("maintenance.delete was not called")
	}
}

func TestAdminOpsHostAndGroup(t *testing.T) {
	_, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version":      "7.0.0",
		"host.get":             []map[string]any{{"hostid": "100", "host": "srv1"}},
		"hostgroup.get":        []map[string]any{{"groupid": "2", "name": "Linux servers"}},
		"host.update":          map[string]any{"hostids": []string{"100"}},
		"hostgroup.massadd":    map[string]any{"groupids": []string{"2"}},
		"hostgroup.massremove": map[string]any{"groupids": []string{"2"}},
	})
	defer cleanup()

	// monitor_host
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"monitor_host", "srv1", "--status", "unmonitored", "--yes"})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("monitor_host error: %v", err)
	}

	// add_host_to_hostgroup
	buf.Reset()
	rootCmd.SetArgs([]string{"add_host_to_hostgroup", "srv1", "2", "--yes"})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("add_host_to_hostgroup error: %v", err)
	}

	// remove_host_from_hostgroup
	buf.Reset()
	rootCmd.SetArgs([]string{"remove_host_from_hostgroup", "srv1", "2", "--yes"})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("remove_host_from_hostgroup error: %v", err)
	}
}

func TestAdminOpsTemplate(t *testing.T) {
	_, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version": "7.0.0",
		"host.get":        []map[string]any{{"hostid": "100", "host": "srv1"}},
		"template.get":    []map[string]any{{"templateid": "50", "host": "Linux by Zabbix agent"}},
		"host.massadd":    map[string]any{"hostids": []string{"100"}},
		"host.massremove": map[string]any{"hostids": []string{"100"}},
	})
	defer cleanup()

	// link_template_to_host
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"link_template_to_host", "srv1", "50", "--yes"})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("link_template error: %v", err)
	}

	// unlink_template_from_host
	buf.Reset()
	rootCmd.SetArgs([]string{"unlink_template_from_host", "srv1", "50", "--clear", "--yes"})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unlink_template error: %v", err)
	}
}

func TestAdminOpsMacro(t *testing.T) {
	_, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version":        "7.0.0",
		"host.get":               []map[string]any{{"hostid": "100", "host": "srv1"}},
		"usermacro.get":          []any{},
		"usermacro.create":       map[string]any{"hostmacroids": []string{"1"}},
		"usermacro.createglobal": map[string]any{"globalmacroids": []string{"1"}},
	})
	defer cleanup()

	// define_host_macro
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"define_host_macro", "srv1", "{$PORT}", "8080", "--yes"})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("define_host_macro error: %v", err)
	}

	// define_global_macro
	buf.Reset()
	rootCmd.SetArgs([]string{"define_global_macro", "{$ENV}", "PROD", "--yes"})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("define_global_macro error: %v", err)
	}
}

func TestAdminOpsReadOnly(t *testing.T) {
	_, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version": "7.0.0",
		"hostgroup.get":   []map[string]any{{"groupid": "5", "name": "G1"}},
		"trigger.get": []map[string]any{{
			"triggerid": "99", "description": "CPU high", "priority": "4", "lastchange": "1727712000",
			"hosts": []map[string]any{{"host": "srv1"}},
		}},
		"host.get": []map[string]any{{"hostid": "100", "host": "srv1"}},
		"item.get": []map[string]any{
			{"itemid": "1", "name": "CPU load", "key_": "system.cpu.load", "lastvalue": "1.5", "lastclock": "1727712000", "units": ""},
		},
		"configuration.export": `{"zabbix_export":{"version":"7.0"}}`,
	})
	defer cleanup()

	// show_alarms table
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"show_alarms"})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("show_alarms error: %v", err)
	}
	if !strings.Contains(buf.String(), "CPU high") {
		t.Errorf("expected CPU high in show_alarms output, got: %s", buf.String())
	}

	// show_last_values
	buf.Reset()
	rootCmd.SetArgs([]string{"show_last_values", "srv1", "load"})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("show_last_values error: %v", err)
	}
	if !strings.Contains(buf.String(), "system.cpu.load") {
		t.Errorf("expected system.cpu.load in show_last_values output, got: %s", buf.String())
	}

	// export_configuration
	buf.Reset()
	rootCmd.SetArgs([]string{"export_configuration", "--hostgroups", "G1", "--format", "json"})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("export_configuration error: %v", err)
	}
	if !strings.Contains(buf.String(), "zabbix_export") {
		t.Errorf("expected zabbix_export in export_configuration output, got: %s", buf.String())
	}
}

func TestMutationRefusesAmbiguousOrPatternHost(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version": "7.4.0",
		"host.get": []map[string]any{
			{"hostid": "111", "host": "eofhni1", "name": "eofhni1"},
			{"hostid": "222", "host": "eofhni10", "name": "eofhni10"},
		},
	})
	defer cleanup()
	_, _, err := runCLI(t, "monitor_host", "eofhni", "--status", "unmonitored", "--yes")
	if err == nil || !strings.Contains(err.Error(), "host not found: eofhni") {
		t.Fatalf("expected host not found, got %v", err)
	}
	if h.count("host.update") != 0 {
		t.Fatal("host.update must not be sent for an unresolved host")
	}
}

func TestMonitorHostRejectsUnknownStatus(t *testing.T) {
	h, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version": "7.4.0",
		"host.get":        []map[string]any{{"hostid": "111", "host": "eofhni1", "name": "eofhni1"}},
	})
	defer cleanup()
	_, _, err := runCLI(t, "monitor_host", "eofhni1", "--status", "unmonitor", "--yes")
	if err == nil || !strings.Contains(err.Error(), "invalid --status 'unmonitor'") {
		t.Fatalf("got %v", err)
	}
	if h.count("host.update") != 0 {
		t.Fatal("no mutation on invalid status")
	}
}

func maintenanceMock(version string) map[string]any {
	return map[string]any{
		"apiinfo.version":    version,
		"hostgroup.get":      []map[string]any{{"groupid": "5", "name": "G1"}},
		"maintenance.create": map[string]any{"maintenanceids": []string{"1"}},
	}
}

func TestMaintenanceUsesObjectsOn74(t *testing.T) {
	h, cleanup := setupMockClient(t, maintenanceMock("7.4.14"))
	defer cleanup()
	if _, _, err := runCLI(t, "create_maintenance_definition", "m1", "--hostgroup", "G1", "--period", "1h", "--yes"); err != nil {
		t.Fatal(err)
	}
	p := h.last("maintenance.create")
	if _, bad := p["groupids"]; bad {
		t.Fatalf("7.4 must not receive groupids: %v", p)
	}
	if _, bad := p["hostids"]; bad {
		t.Fatalf("7.4 must not receive hostids: %v", p)
	}
	groups, _ := p["groups"].([]any)
	if len(groups) != 1 || groups[0].(map[string]any)["groupid"] != "5" {
		t.Fatalf("groups=%v", p["groups"])
	}
	if _, has := p["hosts"]; has {
		t.Fatalf("no hosts key when no --host given: %v", p)
	}
}

func TestMaintenanceUsesIDArraysOn52(t *testing.T) {
	h, cleanup := setupMockClient(t, maintenanceMock("5.2.2"))
	defer cleanup()
	if _, _, err := runCLI(t, "create_maintenance_definition", "m1", "--hostgroup", "G1", "--yes"); err != nil {
		t.Fatal(err)
	}
	p := h.last("maintenance.create")
	hostids, ok := p["hostids"].([]any)
	if !ok || len(hostids) != 0 {
		t.Fatalf("5.2 hostids must be [] not null: %#v", p["hostids"])
	}
	if g, _ := p["groupids"].([]any); len(g) != 1 || g[0] != "5" {
		t.Fatalf("groupids=%v", p["groupids"])
	}
}

func TestMaintenanceRequiresTarget(t *testing.T) {
	_, cleanup := setupMockClient(t, maintenanceMock("7.4.14"))
	defer cleanup()
	_, _, err := runCLI(t, "create_maintenance_definition", "m1")
	if err == nil || !strings.Contains(err.Error(), "--host or --hostgroup") {
		t.Fatalf("got %v", err)
	}
	_, _, err = runCLI(t, "create_maintenance_definition", "m1", "--hostgroup", "G1", "--data-collection", "nodata")
	if err == nil || !strings.Contains(err.Error(), "invalid --data-collection") {
		t.Fatalf("got %v", err)
	}
}

func TestExportConfigurationGroupKeyByVersion(t *testing.T) {
	for _, tc := range []struct{ version, key string }{{"7.4.14", "host_groups"}, {"5.2.2", "groups"}} {
		h, cleanup := setupMockClient(t, map[string]any{
			"apiinfo.version":      tc.version,
			"hostgroup.get":        []map[string]any{{"groupid": "5", "name": "G1"}},
			"configuration.export": "{}",
		})
		if _, _, err := runCLI(t, "export_configuration", "--hostgroups", "G1", "--format", "xml"); err != nil {
			cleanup()
			t.Fatalf("%s: %v", tc.version, err)
		}
		opts, _ := h.last("configuration.export")["options"].(map[string]any)
		if _, ok := opts[tc.key]; !ok {
			t.Errorf("%s: expected options.%s, got %v", tc.version, tc.key, opts)
		}
		cleanup()
	}
}

func TestExportConfigurationValidation(t *testing.T) {
	_, cleanup := setupMockClient(t, map[string]any{"apiinfo.version": "7.4.14"})
	defer cleanup()
	if _, _, err := runCLI(t, "export_configuration", "--hostgroups", "G1", "--format", "csv"); err == nil || !strings.Contains(err.Error(), "invalid --format 'csv'") {
		t.Fatalf("got %v", err)
	}
	if _, _, err := runCLI(t, "export_configuration"); err == nil || !strings.Contains(err.Error(), "at least one of") {
		t.Fatalf("got %v", err)
	}
}

func TestMacroAndAckValidation(t *testing.T) {
	_, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version": "7.4.0",
		"host.get":        []map[string]any{{"hostid": "111", "host": "eofhni1", "name": "eofhni1"}},
	})
	defer cleanup()
	if _, _, err := runCLI(t, "define_global_macro", "BAD", "v"); err == nil || !strings.Contains(err.Error(), "invalid macro") {
		t.Fatalf("got %v", err)
	}
	if _, _, err := runCLI(t, "define_host_macro", "eofhni1", "{$OK}", "v", "--type", "7"); err == nil || !strings.Contains(err.Error(), "invalid --type") {
		t.Fatalf("got %v", err)
	}
	if _, _, err := runCLI(t, "acknowledge_event", "12a"); err == nil || !strings.Contains(err.Error(), "invalid event ID") {
		t.Fatalf("got %v", err)
	}
	if _, _, err := runCLI(t, "acknowledge_event", "12", "-m", " "); err == nil || !strings.Contains(err.Error(), "message") {
		t.Fatalf("got %v", err)
	}
}

func alarmsMock() map[string]any {
	return map[string]any{
		"apiinfo.version": "7.4.0",
		"host.get":        []map[string]any{{"hostid": "111", "host": "eofhni1", "name": "eofhni1"}},
		"trigger.get": []map[string]any{{
			"triggerid": "900", "description": "CPU high", "priority": "4", "lastchange": "1790218800",
			"hosts": []map[string]any{{"host": "eofhni1"}},
		}},
	}
}

func TestShowAlarmsUsesTriggerGetLikePython(t *testing.T) {
	h, cleanup := setupMockClient(t, alarmsMock())
	defer cleanup()
	out, _, err := runCLI(t, "show_alarms", "eofhni1")
	if err != nil {
		t.Fatal(err)
	}
	if h.count("problem.get") != 0 {
		t.Fatal("show_alarms must not use problem.get")
	}
	p := h.last("trigger.get")
	if f, _ := p["filter"].(map[string]any); f["value"] != float64(1) {
		t.Fatalf("filter.value must be 1, got %v", p["filter"])
	}
	if p["withLastEventUnacknowledged"] != true {
		t.Fatalf("default must be unacknowledged only: %v", p)
	}
	if _, bad := p["recent"]; bad {
		t.Fatal("recent must not be sent")
	}
	for _, want := range []string{"TRIGGERID", "HOST", "eofhni1", "High", "CPU high"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestShowAlarmsAckIncludesAcknowledged(t *testing.T) {
	h, cleanup := setupMockClient(t, alarmsMock())
	defer cleanup()
	if _, _, err := runCLI(t, "show_alarms", "--ack", "--priority", "4", "--description", "CPU"); err != nil {
		t.Fatal(err)
	}
	p := h.last("trigger.get")
	if _, has := p["withLastEventUnacknowledged"]; has {
		t.Fatalf("--ack must drop the unack filter: %v", p)
	}
	if f, _ := p["filter"].(map[string]any); f["priority"] != float64(4) {
		t.Fatalf("priority filter: %v", p["filter"])
	}
	if s, _ := p["search"].(map[string]any); s["description"] != "CPU" {
		t.Fatalf("description search: %v", p["search"])
	}
}

func TestShowAlarmsJSONEmptyIsArray(t *testing.T) {
	_, cleanup := setupMockClient(t, map[string]any{"apiinfo.version": "7.4.0"})
	defer cleanup()
	out, _, err := runCLI(t, "--format", "json", "show_alarms")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty result must be [], got %q", out)
	}
}

func TestShowLastValuesJSONEmptyIsArray(t *testing.T) {
	_, cleanup := setupMockClient(t, map[string]any{
		"apiinfo.version": "7.4.0",
		"host.get":        []map[string]any{{"hostid": "111", "host": "eofhni1", "name": "eofhni1"}},
	})
	defer cleanup()
	out, _, err := runCLI(t, "--format", "json", "show_last_values", "eofhni1", "nomatch")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("got %q", out)
	}
}
