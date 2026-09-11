#!/usr/bin/env bash
# btcd sync-peer stall watchdog — one tick; run it every 10 minutes from
# deploy/btcd-sync-watchdog.timer. See deploy/README.md §7 and issue #37.
#
# btcd 0.26 can wedge after losing its sync peer near the tip: it stays
# connected to every peer, logs nothing, ignores their new-block announcements,
# and only recovers when a *new* outbound peer connects and advertises a higher
# height (btcsuite/btcd#2606). Disconnecting any outbound peer makes btcd open
# a replacement connection, which is the whole fix. This script does that
# automatically instead of a human doing it hours later.
#
# Trigger — both must hold:
#   1. getblockcount has not changed for STALL_MIN minutes.
#   2. getpeerinfo shows no peer with "syncnode": true.
# Peer heights are deliberately not consulted: they freeze during the stall.
#
# Rails: at most one intervention per INTERVAL_MIN minutes; after
# MAX_INTERVENTIONS with no progress it stops and leaves it to the healthz
# alert (#34). It never restarts btcd.
#
# Log: every intervention is one line in $STATE_DIR/interventions.log and on
# stdout (the journal, under the unit's SyslogIdentifier). The next tick
# patches the line's outcome: "recovered" if the height moved, else
# "no-effect". Quiet ticks print nothing. Count interventions with
#   grep -c ' intervention ' interventions.log
#
# Env: BTCCTL (default "btcctl"; may carry flags, e.g. "btcctl -C /etc/btcd/btcctl.conf")
#      STATE_DIR (default /var/lib/btcd-sync-watchdog)
#      STALL_MIN (60)  INTERVAL_MIN (30)  MAX_INTERVENTIONS (3)
#      NOW (epoch seconds; tests only)
# Flags: --dry-run   detect and log, but never disconnect. Dry-run ticks share
#                    the state file, so they use the same rate-limit and
#                    per-stall slots a live run would; a stall rehearsed in
#                    dry-run counts as handled until the height moves.
set -u

dry=0
case "${1:-}" in
    --dry-run) dry=1 ;;
    "") ;;
    *) echo "usage: $0 [--dry-run]" >&2; exit 2 ;;
esac

BTCCTL="${BTCCTL:-btcctl}"
STATE_DIR="${STATE_DIR:-/var/lib/btcd-sync-watchdog}"
STALL_MIN="${STALL_MIN:-60}"
INTERVAL_MIN="${INTERVAL_MIN:-30}"
MAX_INTERVENTIONS="${MAX_INTERVENTIONS:-3}"
now="${NOW:-$(date +%s)}"

state="$STATE_DIR/state"
log="$STATE_DIR/interventions.log"

# shellcheck disable=SC2086  # BTCCTL is intentionally word-split to allow flags
bc() { $BTCCTL "$@"; }

stamp() { date -u -d "@$1" +%FT%TZ 2>/dev/null || date -u -r "$1" +%FT%TZ; }

# state: height=<last seen> since=<epoch first seen at that height>
#        last=<epoch of last intervention> count=<interventions this stall>
#        pending=<stamp of intervention awaiting an outcome>
s_height="" s_since="" s_last=0 s_count=0 s_pending=""
if [ -f "$state" ]; then
    while IFS='=' read -r k v; do
        case "$k" in
            height)  s_height=$v ;;
            since)   s_since=$v ;;
            last)    s_last=$v ;;
            count)   s_count=$v ;;
            pending) s_pending=$v ;;
        esac
    done <"$state"
fi

save() {
    printf 'height=%s\nsince=%s\nlast=%s\ncount=%s\npending=%s\n' \
        "$s_height" "$s_since" "$s_last" "$s_count" "$s_pending" >"$state.tmp" \
        && mv "$state.tmp" "$state"
}

mkdir -p "$STATE_DIR" || { echo "cannot create $STATE_DIR" >&2; exit 1; }

height="$(bc getblockcount 2>&1)"
if ! [[ "$height" =~ ^[0-9]+$ ]]; then
    echo "getblockcount failed: $height" >&2
    exit 1
fi

# Settle the outcome of the previous intervention, if any.
if [ -n "$s_pending" ]; then
    if [ "$height" != "$s_height" ]; then outcome=recovered; else outcome=no-effect; fi
    sed "s/^\(${s_pending} .*\) outcome=pending$/\1 outcome=${outcome}/" "$log" >"$log.tmp" \
        && mv "$log.tmp" "$log"
    echo "intervention at $s_pending: $outcome (height $s_height -> $height)"
    s_pending=""
fi

# Progress resets the stall clock.
if [ "$height" != "$s_height" ]; then
    s_height=$height; s_since=$now; s_count=0
    save
    exit 0
fi

stuck=$(( now - s_since ))
if [ "$stuck" -lt $(( STALL_MIN * 60 )) ]; then
    save
    exit 0
fi
stuck_min=$(( stuck / 60 ))

peers="$(bc getpeerinfo 2>&1)" || { echo "getpeerinfo failed: $peers" >&2; save; exit 1; }

if grep -q '"syncnode": true' <<<"$peers"; then
    # btcd's own 3-minute stall handler owns this case; not the bug we fix.
    echo "height $height unchanged for ${stuck_min}m but a sync peer exists; leaving it to btcd"
    save
    exit 0
fi

if [ "$s_count" -ge "$MAX_INTERVENTIONS" ]; then
    if [ "$s_count" -eq "$MAX_INTERVENTIONS" ]; then
        echo "gave up: $s_count interventions without progress at height $height; leaving it to the healthz alert"
        s_count=$(( s_count + 1 ))   # say it once, then stay quiet
        save
    fi
    exit 0
fi

if [ $(( now - s_last )) -lt $(( INTERVAL_MIN * 60 )) ]; then
    save
    exit 0
fi

# First outbound peer. btcd only replaces outbound connections, and the
# replacement's version handshake is what re-arms sync-peer election.
# btcctl prints one field per line with "addr" before "inbound".
addr="$(awk -F'"' '/"addr":/ {a=$4} /"inbound": false/ {print a; exit}' <<<"$peers")"
if [ -z "$addr" ]; then
    echo "height $height unchanged for ${stuck_min}m, no sync peer, and no outbound peer to disconnect" >&2
    save
    exit 1
fi

kind=intervention
if [ "$dry" -eq 1 ]; then
    kind='dry-run'
else
    out="$(bc node disconnect "$addr" 2>&1)" || {
        echo "node disconnect $addr failed: $out" >&2
        save
        exit 1
    }
fi

when="$(stamp "$now")"
line="$when $kind height=$height stuck=${stuck_min}m syncnode=no peer=$addr outcome=pending"
echo "$line" >>"$log"
echo "$line"

s_last=$now; s_count=$(( s_count + 1 )); s_pending=$when
save
