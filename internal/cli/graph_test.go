package cli

import (
	"testing"

	"zx/internal/zbxclient"
)

func TestGraphOutputPath(t *testing.T) {
	if got := graphOutputPath("out/qni.png", "cpu", false); got != "out/qni.png" {
		t.Errorf("single: %s", got)
	}
	if got := graphOutputPath("out/qni.png", "free_swap", true); got != "out/qni_free_swap.png" {
		t.Errorf("multi: %s", got)
	}
	if got := graphOutputPath("graph", "ram", true); got != "graph_ram" {
		t.Errorf("no ext: %s", got)
	}
}

func TestSummarize(t *testing.T) {
	s := []zbxclient.TrendSample{
		{Clock: 1, ValueMin: 1, ValueAvg: 10, ValueMax: 50},
		{Clock: 2, ValueMin: 5, ValueAvg: 20, ValueMax: 30},
	}
	min, avg, max, n := summarize(s, false)
	if min != 1 || avg != 15 || max != 20 || n != 2 {
		t.Errorf("avg-peak: %v %v %v %d", min, avg, max, n)
	}
	_, _, max, _ = summarize(s, true)
	if max != 50 {
		t.Errorf("raw peak: %v", max)
	}
	if _, _, _, n = summarize(nil, false); n != 0 {
		t.Error("empty")
	}
}
