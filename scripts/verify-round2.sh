#!/usr/bin/env bash
# One-shot live verification of the round-2 design changes against the
# regtest harness. Assumes the dev server (scripts/dev.sh) or the built
# binary is serving on :8480. Optional arg: a pending txid from txgen.
set -u

API="${BTCDWATCH_URL:-http://127.0.0.1:8480}"
PENDING_TXID="${1:-}"
pass=0 fail=0

check() { # check <exit-code> <label> <detail>
  if [ "$1" = 0 ]; then pass=$((pass + 1)); echo "  ✓ $2  $3"
  else fail=$((fail + 1)); echo "  ✗ $2  $3"; fi
}

get() { # get <url> <dot.path>  → prints value or nothing
  curl -sf "$1" | python3 -c "
import sys, json, functools
try:
    d = json.load(sys.stdin)
    v = functools.reduce(
        lambda o, k: o[int(k)] if isinstance(o, list) else o[k],
        '$2'.split('.'), d)
    print(json.dumps(v) if isinstance(v, (dict, list, bool)) else v)
except Exception:
    pass" 2>/dev/null
}

echo "— stats.queue (mempool bands) —"
txcount=$(get "$API/api/stats" queue.txCount)
cutoff=$(get "$API/api/stats" queue.cutoffFraction)
rate=$(get "$API/api/stats" queue.nextBlockRate)
nbands=$(get "$API/api/stats" queue.bands | python3 -c 'import sys,json;print(len(json.load(sys.stdin)))' 2>/dev/null)
[ "$nbands" = 5 ]; check $? "queue in /api/stats" "bands=$nbands txCount=$txcount cutoff=$cutoff nextBlockRate=$rate"

echo "— /api/examples removed —"
code=$(curl -s -o /dev/null -w '%{http_code}' "$API/api/examples")
[ "$code" != 200 ]; check $? "examples endpoint gone" "HTTP $code"

echo "— block endpoint + search routing —"
tip=$(get "$API/api/stats" blockHeight)
h=$((tip - 1))
bhash=$(get "$API/api/block/$h" hash)
[ -n "$bhash" ]; check $? "block by height" "h=$h txs=$(get "$API/api/block/$h" txCount) avgFee=$(get "$API/api/block/$h" avgFeeSatPerVb) next=$(get "$API/api/block/$h" nextHeight)"

[ "$(get "$API/api/block/$bhash" height)" = "$h" ]; check $? "block by hash" "→ height $h"
[ "$(get "$API/api/search?q=$h" kind)" = "block" ]; check $? "search: digits → block" ""
[ "$(get "$API/api/search?q=$bhash" kind)" = "block" ]; check $? "search: 64-hex block hash → block" ""
[ "$(get "$API/api/block/$h" txs.0.isCoinbase)" = "true" ]; check $? "first row is coinbase" ""
[ "$(get "$API/api/search?q=99999999" kind)" = "notfound" ]; check $? "search: absurd height → notfound" ""

txid=$(get "$API/api/block/$h" txs.0.txid)
[ "$(get "$API/api/search?q=$txid" kind)" = "tx" ]; check $? "search: txid still → tx" ""

echo "— pending queue position —"
if [ -n "$PENDING_TXID" ]; then
  frac=$(get "$API/api/tx/$PENDING_TXID" pending.queueVbytesFraction)
  [ -n "$frac" ]; check $? "pending.queueVbytesFraction" "= $frac"
else
  echo "  (skipped — rerun with a pending txid from txgen as \$1)"
fi

echo
echo "RESULT: $pass passed, $fail failed"
echo
echo "Manual UI spot-checks at $API (or :5174 via Vite):"
echo "  1. Landing: no 'Try an example' chips; 'The line right now' card sits between hero and stats."
echo "  2. Header: 'Fees: N sat/vB' pill on every view; click → slide-over; Esc/backdrop/✕ close it."
echo "  3. Pending tx: 'Your place in line' bar with cutoff + amber 'you are here' pills; moves on new blocks while watching."
echo "  4. Confirmed tx: 'In block' tile is an orange link → Block view."
echo "  5. Block view: prev/next pills, coinbase row badged 'miner reward', '…and N more — show more' paginates, rows open the tx."
