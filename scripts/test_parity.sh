#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"
PY_CLI="/home/quangtd/Workspace/VNPT/EOF/zabbix-cli/.venv/bin/zabbix-cli"
# Always test a fresh build of the working tree, never bin/zx (user WIP).
# Every temp file lives under BUILD_DIR so the EXIT trap cleans all of it.
BUILD_DIR=$(mktemp -d)
trap 'rm -rf "$BUILD_DIR"' EXIT
if [[ -n "${ZX_CLI:-}" && "$(realpath -m "$ZX_CLI")" == "$(realpath -m "$REPO_DIR/bin/zx")" ]]; then
    echo "ZX_CLI must not point at $REPO_DIR/bin/zx (user WIP); unset it to build a fresh binary" >&2
    exit 1
fi
if [[ -z "${ZX_CLI:-}" ]]; then
    ZX_CLI="$BUILD_DIR/zx"
    (cd "$REPO_DIR" && CGO_ENABLED=0 go build \
        -ldflags="-s -w -X zx/internal/cli.Version=parity-$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
        -o "$ZX_CLI" ./cmd/zx)
fi
PROFILE="${1:-${PROFILE:-}}"
TARGETS="${2:-${TARGETS:-}}"
if [[ -z "$PROFILE" || -z "$TARGETS" ]]; then
    echo "Usage: $0 <profile> <targets>" >&2
    echo "Example: $0 production host1,host2" >&2
    exit 1
fi

echo "=== ZABBIX-CLI PARITY TEST (Python vs Golang zx) ==="

echo "[1/4] Preflight contract on profile '$PROFILE'..."
for cli in "$PY_CLI" "$ZX_CLI"; do
    out=$("$cli" --profile "$PROFILE" preflight 2>&1) || { echo "preflight FAILED ($cli): $out"; exit 1; }
    [[ "$out" == PASS* ]] || { echo "preflight output must start with PASS ($cli): $out"; exit 1; }
    echo "$out"
done

# show_host_stats is NOT compared: Python and zx resolve targets and define
# the cluster peak differently, so the two tables are printed for a human.
echo "[2/4] show_host_stats side by side (informational, not compared)..."
echo "--- Python zabbix-cli output ---"
$PY_CLI --profile "$PROFILE" show_host_stats "$TARGETS" -d 7 --business-hours

echo "--- Golang zx output ---"
$ZX_CLI --profile "$PROFILE" show_host_stats "$TARGETS" -d 7 --business-hours

echo "[3/4] Testing export_graph PNG output..."
OUT_PNG="$BUILD_DIR/zx_parity_test.png"
$ZX_CLI --profile "$PROFILE" export_graph "$TARGETS" -m ram -d 7 -o "$OUT_PNG"
if [[ ! -s "$OUT_PNG" ]]; then
    echo "PNG validation FAILED: $OUT_PNG was not written"
    exit 1
fi
SIZE=$(stat -c%s "$OUT_PNG")
HEADER=$(head -c 4 "$OUT_PNG")
if [[ "$HEADER" != $'\x89PNG' ]]; then
    echo "PNG validation FAILED: invalid magic header"
    exit 1
fi
echo "PNG validation PASSED ($SIZE bytes, header=\x89PNG)"

echo "[4/4] Comparing export_window JSON..."
WORK="$BUILD_DIR/window"
mkdir -p "$WORK"
for t in $(tr ',' ' ' <<<"$TARGETS"); do
  h=$($ZX_CLI --profile "$PROFILE" --format json show_hosts "$t" 2>/dev/null | python3 -c 'import sys, json; data=json.load(sys.stdin); print(data[0]["host"] if data else sys.argv[1])' "$t" 2>/dev/null || echo "$t")
  echo "$h" >> "$WORK/hosts.txt"
done
FROM="${WINDOW_FROM:-$(date -u -d '2 days ago 03:00' +%Y-%m-%dT%H:%M:%SZ)}"
TO="${WINDOW_TO:-$(date -u -d '2 days ago 05:00' +%Y-%m-%dT%H:%M:%SZ)}"
$PY_CLI --profile "$PROFILE" --format json export_window --from "$FROM" --to "$TO" \
  --timezone Asia/Ho_Chi_Minh --role app --input-file "$WORK/hosts.txt" > "$WORK/py.json"
$ZX_CLI --profile "$PROFILE" --format json export_window --from "$FROM" --to "$TO" \
  --timezone Asia/Ho_Chi_Minh --role app --input-file "$WORK/hosts.txt" > "$WORK/zx.json"
python3 "$SCRIPT_DIR/parity_export_window.py" "$WORK/py.json" "$WORK/zx.json"

echo "=== PARITY VERIFIED: preflight contract, export_graph PNG, export_window JSON (show_host_stats printed only) ==="
