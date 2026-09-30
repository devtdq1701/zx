#!/usr/bin/env python3
"""Compare zabbix-cli and zx export_window JSON for the same arguments."""
import json
import sys

TOL = 1e-6


def load(path):
    with open(path, encoding="utf-8") as fh:
        return json.load(fh)


def main(py_path, zx_path):
    py, zx = load(py_path), load(zx_path)
    problems = []
    for key in ("schema", "version", "status", "role"):
        if py.get(key) != zx.get(key):
            problems.append(f"{key}: py={py.get(key)!r} zx={zx.get(key)!r}")
    for key in ("since", "until", "timezone", "inclusive"):
        if py["window"].get(key) != zx["window"].get(key):
            problems.append(f"window.{key}: py={py['window'].get(key)!r} zx={zx['window'].get(key)!r}")
    py_hosts = {h.get("hostid") or h["host"]: h for h in py["hosts"]}
    zx_hosts = {h.get("hostid") or h["host"]: h for h in zx["hosts"]}
    if set(py_hosts) != set(zx_hosts):
        problems.append(f"hosts: py={sorted(py_hosts)} zx={sorted(zx_hosts)}")
    notes = []
    for hid in sorted(set(py_hosts) & set(zx_hosts)):
        for metric in ("cpu", "ram", "load", "io"):
            a, b = py_hosts[hid]["metrics"].get(metric, {}), zx_hosts[hid]["metrics"].get(metric, {})
            ka = (a.get("mapping") or {}).get("key")
            kb = (b.get("mapping") or {}).get("key")
            if ka != kb:
                notes.append(f"{hid}.{metric}: item differs py={ka} zx={kb}")
                continue
            if a.get("state") != b.get("state") or a.get("sample_count") != b.get("sample_count"):
                problems.append(f"{hid}.{metric}: state/samples py={a.get('state')}/{a.get('sample_count')} zx={b.get('state')}/{b.get('sample_count')}")
                continue
            for stat in ("min", "avg", "max", "peak"):
                va, vb = a.get(stat), b.get(stat)
                if (va is None) != (vb is None) or (va is not None and abs(va - vb) > TOL):
                    problems.append(f"{hid}.{metric}.{stat}: py={va} zx={vb}")
    for n in notes:
        print("NOTE", n)
    for p in problems:
        print("DIFF", p)
    print("PARITY", "FAIL" if problems else "PASS")
    return 1 if problems else 0


if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: parity_export_window.py <py.json> <zx.json>", file=sys.stderr)
        sys.exit(2)
    sys.exit(main(sys.argv[1], sys.argv[2]))
