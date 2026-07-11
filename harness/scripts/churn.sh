#!/bin/sh
# Miner + transaction generator for the regtest harness.
#
# Drives Bitcoin Core (which owns the wallets): mines blocks on an interval and
# sends a steady trickle of transactions between throwaway wallets. btcd syncs
# the resulting chain over P2P and is the node btcdwatch reads. Fully
# self-contained — ephemeral wallets, no host state, resilient to restarts.
#
# POSIX sh (busybox ash in the Bitcoin Core image); no bash-isms.
set -u

: "${RPC_USER:=regtest}"
: "${RPC_PASS:=regtest}"
: "${BLOCK_INTERVAL:=15}"
: "${TX_INTERVAL:=4}"
BITCOIND_HOST="${BITCOIND_HOST:-bitcoind}"
RPC_PORT="${CORE_RPC_PORT:-18443}"

MINER=miner
TXWALLETS="txgen1 txgen2"

bcli() {
    bitcoin-cli -regtest -rpcconnect="$BITCOIND_HOST" -rpcport="$RPC_PORT" \
        -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASS" "$@"
}
wcli() { _w="$1"; shift; bcli -rpcwallet="$_w" "$@"; }

# 0..65535 from the kernel CSPRNG ($RANDOM is not portable to ash).
rnd() { od -An -N2 -tu2 /dev/urandom | tr -d ' '; }
log() { echo "$(date '+%H:%M:%S') churn: $*"; }

until bcli getblockchaininfo >/dev/null 2>&1; do
    log "waiting for bitcoind RPC..."
    sleep 2
done

ensure_wallet() {
    bcli listwallets 2>/dev/null | grep -q "\"$1\"" && return 0
    bcli loadwallet "$1" 2>/dev/null && return 0
    bcli -named createwallet wallet_name="$1" load_on_startup=true >/dev/null 2>&1
}
for w in $MINER $TXWALLETS; do ensure_wallet "$w"; done

MINER_ADDR="$(wcli $MINER getnewaddress coinbase 2>/dev/null)"

# A fresh miner wallet has no spendable funds until its own coinbase matures
# (100 confirmations). Mine 101 up front so txgen can be funded immediately.
if [ "$(wcli $MINER getbalance 2>/dev/null)" = "0.00000000" ]; then
    log "bootstrapping: mining 101 blocks to mature coinbase"
    bcli generatetoaddress 101 "$MINER_ADDR" >/dev/null 2>&1
    log "bootstrap complete at height $(bcli getblockcount 2>/dev/null)"
fi

# One reused "pool" address per wallet → deep per-address history to browse.
pool_addr() {
    _a="$(wcli "$1" getaddressesbylabel pool 2>/dev/null \
        | grep -o '"bcrt1[^"]*"' | tr -d '"' | head -1)"
    [ -z "$_a" ] && _a="$(wcli "$1" getnewaddress pool 2>/dev/null)"
    echo "$_a"
}
fund_if_low() {
    _b="$(wcli "$1" getbalance 2>/dev/null || echo 0)"
    awk -v b="$_b" 'BEGIN{exit !(b < 1)}' || return 0
    _to="$(pool_addr "$1")"
    wcli $MINER sendtoaddress "$_to" 25 >/dev/null 2>&1 \
        && log "funded $1 with 25 BTC" \
        || log "WARN: funding $1 failed (coinbase still immature?)"
}
for w in $TXWALLETS; do pool_addr "$w" >/dev/null; fund_if_low "$w"; done

# Block production loop, in the background.
{
    while :; do
        if bcli generatetoaddress 1 "$MINER_ADDR" >/dev/null 2>&1; then
            log "mined block $(bcli getblockcount 2>/dev/null)"
        else
            log "WARN: mining failed, retrying"
        fi
        sleep "$BLOCK_INTERVAL"
    done
} &

# Transaction traffic loop. ~5% multi-output, ~2% fee-bumped (RBF); wallet txs
# signal BIP-125 by default so the explorer's RBF badge lights up.
amt() { awk -v r="$(rnd)" 'BEGIN{printf "%.5f", 0.001 + r/65535*0.05}'; }
pick() { set -- $TXWALLETS; _i=$(( $(rnd) % $# + 1 )); eval echo "\${$_i}"; }

log "sending ~1 tx every ${TX_INTERVAL}s across: $TXWALLETS"
while :; do
    SRC="$(pick)"; DST="$(pick)"
    fund_if_low "$SRC"
    TO="$(pool_addr "$DST")"
    ROLL=$(( $(rnd) % 100 ))
    if [ "$ROLL" -lt 5 ]; then
        A2="$(pool_addr "$(pick)")"
        wcli "$SRC" sendmany "" "{\"$TO\":$(amt),\"$A2\":$(amt)}" >/dev/null 2>&1 \
            && log "multi-out tx from $SRC" \
            || log "WARN: sendmany from $SRC failed"
    elif [ "$ROLL" -lt 7 ]; then
        TXID="$(wcli "$SRC" sendtoaddress "$TO" "$(amt)" 2>/dev/null)"
        if [ -n "$TXID" ] && wcli "$SRC" bumpfee "$TXID" >/dev/null 2>&1; then
            log "rbf tx from $SRC ($TXID bumped)"
        else
            log "tx from $SRC (rbf bump skipped)"
        fi
    else
        wcli "$SRC" sendtoaddress "$TO" "$(amt)" >/dev/null 2>&1 \
            && log "tx $SRC -> $DST" \
            || log "WARN: send from $SRC failed"
    fi
    sleep "$TX_INTERVAL"
done
