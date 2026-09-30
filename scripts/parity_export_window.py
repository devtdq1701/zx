#!/usr/bin/env python3
"""Compare zabbix-cli and zx export_window JSON for the same arguments."""
import json
import sys

TOL = 1e-6
# Intentional zx difference: load prefers the 15-minute average.
ALLOWED_KEY_DIFFS = {
    ("load", "system.cpu.load[all,avg1]", "system.cpu.load[all,avg15]"),
    ("load", "system.cpu.load[percpu,avg1]", "system.cpu.load[percpu,avg15]"),
}


def load(path):
    with open(path, encoding="utf-8") as fh:
        return json.load(fh)


def _errors(d):
    return sorted((e.get("host"), e.get("error"), e.get("state")) for e in d.get("errors") or [])


def compare(py, zx):
    problems, notes = [], []
    for key in ("schema", "version", "status", "role"):
        if py.get(key) != zx.get(key):
            problems.append(f"{key}: py={py.get(key)!r} zx={zx.get(key)!r}")
    for key in ("since", "until", "timezone", "inclusive"):
        if py["window"].get(key) != zx["window"].get(key):
            problems.append(f"window.{key}: py={py['window'].get(key)!r} zx={zx['window'].get(key)!r}")
    if py.get("filters") != zx.get("filters"):
        problems.append(f"filters: py={py.get('filters')!r} zx={zx.get('filters')!r}")
    if _errors(py) != _errors(zx):
        problems.append(f"errors: py={_errors(py)} zx={_errors(zx)}")
    py_hosts = {h.get("hostid") or h["host"]: h for h in py["hosts"]}
    zx_hosts = {h.get("hostid") or h["host"]: h for h in zx["hosts"]}
    if not py_hosts and not zx_hosts:
        problems.append("hosts: empty on both sides; parity not exercised")
    if set(py_hosts) != set(zx_hosts):
        problems.append(f"hosts: py={sorted(py_hosts)} zx={sorted(zx_hosts)}")
    for hid in sorted(set(py_hosts) & set(zx_hosts)):
        if py_hosts[hid].get("state") != zx_hosts[hid].get("state"):
            problems.append(f"{hid}.state: py={py_hosts[hid].get('state')} zx={zx_hosts[hid].get('state')}")
        for metric in ("cpu", "ram", "load", "io"):
            a, b = py_hosts[hid]["metrics"].get(metric, {}), zx_hosts[hid]["metrics"].get(metric, {})
            ma, mb = a.get("mapping") or {}, b.get("mapping") or {}
            if ma.get("key") != mb.get("key"):
                if (metric, ma.get("key"), mb.get("key")) in ALLOWED_KEY_DIFFS:
                    notes.append(f"{hid}.{metric}: intentional item difference py={ma.get('key')} zx={mb.get('key')}")
                else:
                    problems.append(f"{hid}.{metric}: item key py={ma.get('key')} zx={mb.get('key')}")
                continue
            if ma.get("itemid") != mb.get("itemid"):
                problems.append(f"{hid}.{metric}: itemid py={ma.get('itemid')} zx={mb.get('itemid')}")
            if a.get("state") != b.get("state") or a.get("sample_count") != b.get("sample_count"):
                problems.append(f"{hid}.{metric}: state/samples py={a.get('state')}/{a.get('sample_count')} zx={b.get('state')}/{b.get('sample_count')}")
                continue
            for stat in ("min", "avg", "max", "peak"):
                va, vb = a.get(stat), b.get(stat)
                if (va is None) != (vb is None) or (va is not None and abs(va - vb) > TOL):
                    problems.append(f"{hid}.{metric}.{stat}: py={va} zx={vb}")
    return problems, notes


def main(py_path, zx_path):
    problems, notes = compare(load(py_path), load(zx_path))
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
