#!/usr/bin/env bash
# Runs btcdwatchd from source against the local btc-regtest-env harness.
set -euo pipefail

source "$(dirname "$0")/env-map.sh"

cd "$ROOT"
exec go run ./cmd/btcdwatchd "$@"
