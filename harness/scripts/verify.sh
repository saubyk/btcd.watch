#!/usr/bin/env bash
# Live end-to-end check of btcdwatch against the Docker regtest harness.
# Assumes: `make regtest-up` is running AND btcdwatchd is serving on :8480
# (see harness/README.md for the env exports). Read-only; safe to re-run.
set -u

API="${BTCDWATCH_URL:-http://127.0.0.1:8480}"
COMPOSE="docker compose -f harness/docker-compose.yml"
CTL="btcctl --configfile=/dev/null --regtest --rpcuser=regtest --rpcpass=regtest --rpccert=/data/rpc.cert --rpcserver=127.0.0.1:18334"

jq_get() { python3 -c "import sys,json;d=json.load(sys.stdin);print($1)" 2>/dev/null; }
pass() { printf '  \033[32m✓\033[0m %s\n' "$1"; }
fail() { printf '  \033[31m✗\033[0m %s\n' "$1"; }

echo "== 1. health =="
H=$(curl -s "$API/api/healthz")
echo "$H" | grep -q '"ok"\|true\|healthy' && pass "healthz: $H" || fail "healthz: $H"

echo "== 2. stats (height / mempool / queue / price) =="
S=$(curl -s "$API/api/stats")
echo "  height:       $(echo "$S" | jq_get 'd.get("blockHeight")')"
echo "  mempool txs:  $(echo "$S" | jq_get 'd.get("mempool",{}).get("txCount")')"
echo "  queue bands:  $(echo "$S" | jq_get 'len(d.get("queue",{}).get("bands",[]))')"
echo "  price:        $(echo "$S" | jq_get 'd.get("price")')"

echo "== 3. pending tx (from btcd mempool) =="
TXID=$($COMPOSE exec -T btcd $CTL getrawmempool 2>/dev/null | grep -o '[0-9a-f]\{64\}' | head -1)
if [ -n "$TXID" ]; then
  echo "  txid: $TXID"
  T=$(curl -s "$API/api/search?q=$TXID")
  echo "  kind:    $(echo "$T" | jq_get 'd.get("kind")')"
  echo "  status:  $(echo "$T" | jq_get '(d.get("tx") or {}).get("status")')"
  echo "  rbf:     $(echo "$T" | jq_get '(d.get("tx") or {}).get("rbf")')"
  echo "  feeRate: $(echo "$T" | jq_get '(d.get("tx") or {}).get("feeRateSatPerVb")')"
else
  echo "  (no pending tx this instant — re-run in a few seconds)"
fi

echo "== 4. block (tip) =="
HGT=$(echo "$S" | jq_get 'd.get("blockHeight")')
B=$(curl -s "$API/api/search?q=$HGT")
echo "  height:  $(echo "$B" | jq_get '(d.get("block") or {}).get("height")')"
echo "  txCount: $(echo "$B" | jq_get '(d.get("block") or {}).get("txCount")')"

echo "== 5. address (a txgen pool address) =="
ADDR=$($COMPOSE exec -T bitcoind bitcoin-cli -regtest -rpcuser=regtest -rpcpassword=regtest -rpcwallet=txgen1 getaddressesbylabel pool 2>/dev/null | grep -o 'bcrt1[a-z0-9]*' | head -1)
if [ -n "$ADDR" ]; then
  echo "  addr: $ADDR"
  A=$(curl -s "$API/api/search?q=$ADDR")
  echo "  kind:     $(echo "$A" | jq_get 'd.get("kind")')"
  echo "  balance:  $(echo "$A" | jq_get '(d.get("address") or {}).get("balanceSats")')"
  echo "  activity: $(echo "$A" | jq_get 'len((d.get("address") or {}).get("activity",[]))') rows"
else
  echo "  (no pool address yet — churn may still be warming up)"
fi
echo "== done =="
