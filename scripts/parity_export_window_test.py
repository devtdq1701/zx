#!/usr/bin/env python3
import copy
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from parity_export_window import compare  # noqa: E402


def doc(load_key="system.cpu.load[all,avg1]", cpu_key="system.cpu.util"):
    metric = lambda key: {"state": "OK", "sample_count": 1, "min": 1.0, "avg": 2.0, "max": 3.0,
                          "peak": 2.0, "mapping": {"itemid": "1", "key": key}}
    return {
        "schema": "zabbix-cli.export_window", "version": 1, "status": "OK", "role": "app",
        "window": {"since": 1, "until": 2, "timezone": "Asia/Ho_Chi_Minh", "inclusive": "[since,until)"},
        "filters": {"business_hours": False},
        "errors": [],
        "hosts": [{"hostid": "10", "host": "h1", "state": "OK",
                   "metrics": {"cpu": metric(cpu_key), "ram": metric("vm.memory.util"),
                               "load": metric(load_key), "io": metric("vfs.dev.util[sda]")}}],
    }


class CompareTest(unittest.TestCase):
    def test_identical_passes(self):
        problems, _ = compare(doc(), doc())
        self.assertEqual(problems, [])

    def test_allowed_load_avg15_is_note(self):
        problems, notes = compare(doc(), doc(load_key="system.cpu.load[all,avg15]"))
        self.assertEqual(problems, [])
        self.assertEqual(len(notes), 1)

    def test_other_key_difference_fails(self):
        problems, _ = compare(doc(), doc(cpu_key="system.cpu.util[,idle]"))
        self.assertTrue(any("cpu" in p for p in problems))

    def test_both_empty_fails(self):
        a, b = doc(), doc()
        a["hosts"], b["hosts"] = [], []
        problems, _ = compare(a, b)
        self.assertTrue(any("not exercised" in p for p in problems))

    def test_errors_and_state_compared(self):
        a, b = doc(), copy.deepcopy(doc())
        b["errors"] = [{"host": "x", "error": "host not found", "state": "UNPROVEN"}]
        b["hosts"][0]["state"] = "PARTIAL"
        problems, _ = compare(a, b)
        self.assertTrue(any("errors" in p for p in problems))
        self.assertTrue(any("state" in p for p in problems))

    def test_filters_compared(self):
        b = doc()
        b["filters"] = {"business_hours": True}
        problems, _ = compare(doc(), b)
        self.assertTrue(any("filters" in p for p in problems))

    def test_no_ok_metric_fails(self):
        a, b = doc(), doc()
        for d in (a, b):
            for m in d["hosts"][0]["metrics"].values():
                m.update(state="NO_SAMPLES", sample_count=0, min=None, avg=None, max=None, peak=None)
        problems, _ = compare(a, b)
        self.assertTrue(any("not exercised" in p for p in problems))

    def test_allowed_load_key_still_compares_state_and_samples(self):
        b = doc(load_key="system.cpu.load[all,avg15]")
        b["hosts"][0]["metrics"]["load"].update(state="NO_SAMPLES", sample_count=0)
        problems, notes = compare(doc(), b)
        self.assertEqual(len(notes), 1)
        self.assertTrue(any("load" in p and "state/samples" in p for p in problems))


if __name__ == "__main__":
    unittest.main()
