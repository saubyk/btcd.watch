#!/usr/bin/env bash
# Runs the built single binary (make build) against the local
# btc-regtest-env harness.
set -euo pipefail

source "$(dirname "$0")/env-map.sh"

if [[ ! -x "$ROOT/bin/btcdwatchd" ]]; then
    echo "error: bin/btcdwatchd not found — run 'make build' first" >&2
    exit 1
fi

cd "$ROOT"
exec ./bin/btcdwatchd "$@"
