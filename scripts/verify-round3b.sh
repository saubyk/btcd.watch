#!/usr/bin/env bash
# Live verification of round 3 PR B (script-type classification) against
# the regtest harness. Optional args: <txid> <address> to inspect specific
# ones; otherwise picks from a recent block / its outputs.
set -u

API="${BTCDWATCH_URL:-http://127.0.0.1:8480}"
pass=0 fail=0

check() { # check <exit-code> <label> <detail>
  if [ "$1" = 0 ]; then pass=$((pass + 1)); echo "  ✓ $2  $3"
  else fail=$((fail + 1)); echo "  ✗ $2  $3"; fi
}

get() { # get <url> <dot.path>
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

tip=$(get "$API/api/stats" blockHeight)
h=$((tip - 1))

echo "— transaction type + RBF —"
txid="${1:-$(get "$API/api/block/$h" txs.1.txid)}"
[ -n "$txid" ] || txid=$(get "$API/api/block/$h" txs.0.txid)
tcode=$(get "$API/api/tx/$txid" type.code)
tin=$(get "$API/api/tx/$txid" type.in)
tout=$(get "$API/api/tx/$txid" type.out)
rbf=$(get "$API/api/tx/$txid" rbf)
[ -n "$tcode" ]; check $? "tx type present" "code=$tcode in=$tin out=$tout rbf=$rbf"

cb=$(get "$API/api/block/$h" txs.0.txid)
cbin=$(get "$API/api/tx/$cb" type.in)
cbcode=$(get "$API/api/tx/$cb" type.code)
cbrbf=$(get "$API/api/tx/$cb" rbf)
[ "$cbin" = '""' ] || [ -z "$cbin" ]; check $? "coinbase: in empty, out-only code" "code=$cbcode rbf=$cbrbf"
[ "$cbrbf" = "false" ]; check $? "coinbase never signals RBF" ""

echo "— address type —"
addr="${2:-}"
if [ -z "$addr" ]; then
  # Take a to-address from the sampled tx.
  addr=$(get "$API/api/tx/$txid" to.0 | tr -d '"')
fi
atype=$(get "$API/api/search?q=$addr" address.type)
[ -n "$atype" ] && [ "$atype" != '""' ]; check $? "address type classified" "addr=${addr:0:16}… type=$atype"

echo
echo "RESULT: $pass passed, $fail failed"
echo
echo "Manual UI spot-checks at $API:"
echo "  1. Address view: two chips next to '◆ WALLET ADDRESS' (mono code + friendly name) and a"
echo "     💡 explainer box between the address and the balance."
echo "  2. Confirmed tx: chip row under from→to ('P2WPKH' + 'Native SegWit transaction');"
echo "     Detailed tab has a 'Type' row above 'Fee paid' like: Native SegWit (P2WPKH → P2WPKH)."
echo "  3. Pending tx: same chips plus the amber 'RBF — fee can be bumped' chip (hover for the"
echo "     tooltip) — only when the tx signals BIP-125 (txgen may or may not; check a few)."
echo "  4. Coinbase tx (top row of a block view): type chip shows the output type; no RBF chip."
