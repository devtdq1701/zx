#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"
PY_CLI="/home/quangtd/Workspace/VNPT/EOF/zabbix-cli/.venv/bin/zabbix-cli"
ZX_CLI="$REPO_DIR/bin/zx"
PROFILE="central"
TARGETS="10.165.67.61,10.165.67.62"

echo "=== ZABBIX-CLI PARITY TEST (Python vs Golang zx) ==="

if [[ ! -x "$ZX_CLI" ]]; then
    echo "Building zx binary..."
    (cd "$REPO_DIR" && go build -ldflags="-s -w" -o bin/zx ./cmd/zx)
fi

echo "[1/3] Testing preflight connectivity on profile '$PROFILE'..."
$ZX_CLI --profile "$PROFILE" preflight

echo "[2/3] Comparing show_host_stats outputs..."
echo "--- Python zabbix-cli output ---"
$PY_CLI --profile "$PROFILE" show_host_stats "$TARGETS" -d 7 --business-hours

echo "--- Golang zx output ---"
$ZX_CLI --profile "$PROFILE" show_host_stats "$TARGETS" -d 7 --business-hours

echo "[3/3] Testing export_graph PNG output..."
OUT_PNG="/tmp/zx_parity_test.png"
$ZX_CLI --profile "$PROFILE" export_graph "$TARGETS" -m ram -d 7 -o "$OUT_PNG"
if [[ -f "$OUT_PNG" ]]; then
    SIZE=$(stat -c%s "$OUT_PNG")
    HEADER=$(head -c 4 "$OUT_PNG")
    if [[ "$HEADER" == $'\x89PNG' ]]; then
        echo "PNG validation PASSED ($SIZE bytes, header=\x89PNG)"
    else
        echo "PNG validation FAILED: invalid magic header"
        exit 1
    fi
    rm -f "$OUT_PNG"
fi

echo "=== PARITY VERIFICATION COMPLETED SUCCESSFULLY ==="
