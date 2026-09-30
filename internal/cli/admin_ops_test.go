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
		"maintenance.create": map[string]any{"maintenanceids": []string{"1"}},
		"maintenance.delete": []string{"1"},
	})
	defer cleanup()

	// Test create
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"create_maintenance_definition", "Weekly-Patch", "--period", "2h", "--yes"})
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
		"problem.get": []map[string]any{
			{"eventid": "99", "name": "CPU high", "severity": "4", "clock": "1727712000", "acknowledged": "0"},
		},
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
	rootCmd.SetArgs([]string{"export_configuration", "--format", "json"})
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("export_configuration error: %v", err)
	}
	if !strings.Contains(buf.String(), "zabbix_export") {
		t.Errorf("expected zabbix_export in export_configuration output, got: %s", buf.String())
	}
}
