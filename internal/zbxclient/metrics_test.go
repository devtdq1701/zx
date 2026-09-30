package zbxclient

import "testing"

func TestMetricRegistry(t *testing.T) {
	items := []ItemRecord{
		{ItemID: "1", Key: "system.cpu.load[all,avg1]"},
		{ItemID: "2", Key: "system.cpu.load[all,avg15]"},
		{ItemID: "3", Key: "system.cpu.util"},
		{ItemID: "4", Key: "system.cpu.util[,iowait]"},
		{ItemID: "5", Key: "system.swap.size[,pfree]"},
		{ItemID: "6", Key: "vm.memory.utilization"},
		{ItemID: "7", Key: "vfs.dev.read[sda]"},
		{ItemID: "8", Key: "vfs.dev.util[sda]"},
	}
	want := map[string]string{"cpu": "3", "ram": "6", "load": "2", "free_swap": "5", "iowait": "4", "io": "7"}
	for name, id := range want {
		m, ok := LookupMetric(name)
		if !ok {
			t.Fatalf("metric %s not registered", name)
		}
		it, found := m.FindItem(items)
		if !found || it.ItemID != id {
			t.Errorf("%s: got %+v found=%v, want itemid %s", name, it, found, id)
		}
	}
	nameOnly := []ItemRecord{{ItemID: "9", Key: "custom.cpu", Name: "CPU utilization"}}
	if it, ok := mustMetric(t, "cpu").FindItem(nameOnly); !ok || it.ItemID != "9" {
		t.Errorf("cpu name fallback failed: %+v", it)
	}
}

func mustMetric(t *testing.T, name string) MetricSpec {
	t.Helper()
	m, ok := LookupMetric(name)
	if !ok {
		t.Fatalf("missing metric %s", name)
	}
	return m
}

func TestParseMetricList(t *testing.T) {
	got, err := ParseMetricList("all")
	if err != nil || len(got) != 5 || got[3].Name != "free_swap" || got[4].Name != "iowait" {
		t.Fatalf("all: %+v %v", got, err)
	}
	got, err = ParseMetricList("CPU, swap,cpu-iowait,cpu")
	if err != nil || len(got) != 3 || got[0].Name != "cpu" || got[1].Name != "free_swap" || got[2].Name != "iowait" {
		t.Fatalf("aliases/dedupe: %+v %v", got, err)
	}
	if _, err := ParseMetricList("disk"); err == nil {
		t.Fatal("expected error for unknown metric")
	}
	if _, err := ParseMetricList(" , "); err == nil {
		t.Fatal("expected error for empty list")
	}
}
