#!/usr/bin/env bash
# Exercises deploy/btcd-sync-watchdog.sh against a stub btcctl. No node needed.
# Run: deploy/tests/sync-watchdog-test.sh   (also part of `make test`)
set -u

here="$(cd "$(dirname "$0")" && pwd)"
wd="$here/../btcd-sync-watchdog.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# --- stub btcctl: answers from files the test writes ------------------------
stub="$tmp/btcctl"
cat >"$stub" <<'STUB'
#!/usr/bin/env bash
dir="$STUB_DIR"
case "$1 ${2:-}" in
    "getblockcount ")   [ -f "$dir/fail" ] && { echo "connection refused" >&2; exit 1; }
                        cat "$dir/height" ;;
    "getpeerinfo ")     cat "$dir/peers" ;;
    "node disconnect")  echo "$3" >>"$dir/disconnects" ;;
    *) echo "stub: unexpected: $*" >&2; exit 1 ;;
esac
STUB
chmod +x "$stub"

peers() {  # peers <syncnode-of-outbound-peer>
    cat >"$STUB_DIR/peers" <<JSON
[
  {
    "id": 1,
    "addr": "10.0.0.1:8333",
    "inbound": true,
    "startingheight": 100,
    "syncnode": false
  },
  {
    "id": 2,
    "addr": "10.0.0.2:8333",
    "inbound": false,
    "startingheight": 100,
    "syncnode": $1
  },
  {
    "id": 3,
    "addr": "10.0.0.3:8333",
    "inbound": false,
    "startingheight": 100,
    "syncnode": false
  }
]
JSON
}

export STUB_DIR="$tmp/stub" BTCCTL="$stub" STATE_DIR="$tmp/state"
mkdir -p "$STUB_DIR"
log="$STATE_DIR/interventions.log"
T0=1699999200   # 2023-11-14T22:00:00Z
tick() {  # tick <minutes since T0> [--dry-run]  → stdout in $out, status in $rc
    local m=$1; shift
    NOW=$(( T0 + m * 60 )) "$wd" "$@" >"$tmp/out" 2>&1; rc=$?; out="$(cat "$tmp/out")"
}
fails=0
check() {  # check <desc> <condition...>
    local d=$1; shift
    if "$@"; then echo "ok   $d"; else echo "FAIL $d"; echo "     out: $out"; fails=$(( fails + 1 )); fi
}
disconnects() { [ -f "$STUB_DIR/disconnects" ] && wc -l <"$STUB_DIR/disconnects" | tr -d ' ' || echo 0; }
interventions() { [ -f "$log" ] && grep -c ' intervention ' "$log" || echo 0; }

# 1. moving chain: silent
echo 100 >"$STUB_DIR/height"; peers false
tick 0;  check "first tick is silent" [ -z "$out" ]
echo 101 >"$STUB_DIR/height"
tick 10; check "progress is silent" [ -z "$out" ]
check "no log file while healthy" [ ! -f "$log" ]

# 2. slow block under the threshold: silent
tick 50; check "59 min stuck: silent" [ -z "$out" ]
check "no disconnect under threshold" [ "$(disconnects)" = 0 ]

# 3. stall: 60 min stuck, no sync peer → disconnect first OUTBOUND peer
tick 70; check "stall triggers an intervention" [ "$(disconnects)" = 1 ]
check "skips the inbound peer" [ "$(cat "$STUB_DIR/disconnects")" = "10.0.0.2:8333" ]
check "log line is pending" grep -q '^2023-11-14T23:10:00Z intervention height=101 stuck=60m syncnode=no peer=10.0.0.2:8333 outcome=pending$' "$log"
check "tick output mirrors the log line" grep -q 'intervention height=101' <<<"$out"

# 4. next tick, height moved → recovered, clock reset
echo 102 >"$STUB_DIR/height"
tick 80; check "outcome patched to recovered" grep -q 'peer=10.0.0.2:8333 outcome=recovered$' "$log"
check "outcome reported on stdout" grep -q 'recovered (height 101 -> 102)' <<<"$out"
tick 130; check "reset: 50 min after progress is silent" [ "$(disconnects)" = 1 ]

# 5. no effect + rate limit + give-up
tick 140; check "second stall intervenes at 60m" [ "$(disconnects)" = 2 ]
tick 150; check "no-effect recorded when height stays" [ "$(grep -c 'outcome=no-effect' "$log")" = 1 ]
check "rate limit: no second disconnect 10 min later" [ "$(disconnects)" = 2 ]
tick 170; check "second intervention after 30 min" [ "$(disconnects)" = 3 ]
tick 200; check "third intervention after 30 more" [ "$(disconnects)" = 4 ]
tick 230; check "fourth is refused (give up)" [ "$(disconnects)" = 4 ]
check "give-up message printed once" grep -q '^gave up: 3 interventions' <<<"$out"
tick 240; check "after giving up, ticks are silent" [ -z "$out" ]
check "interventions counted via grep" [ "$(interventions)" = 4 ]

# 6. progress after give-up resets everything
echo 103 >"$STUB_DIR/height"
tick 250; check "progress after give-up is quiet" [ -z "$out" ]
tick 310; check "fresh stall intervenes again" [ "$(disconnects)" = 5 ]

# 7. sync peer present: hands off
echo 104 >"$STUB_DIR/height"; peers true
tick 320; tick 390
check "sync peer present: no disconnect" [ "$(disconnects)" = 5 ]
check "sync peer present: explains itself" grep -q 'a sync peer exists' <<<"$out"

# 8. dry run: logs but never disconnects
echo 105 >"$STUB_DIR/height"; peers false
tick 400; tick 470 --dry-run
check "dry-run: no disconnect" [ "$(disconnects)" = 5 ]
check "dry-run: logged as dry-run" grep -q ' dry-run height=105 .* outcome=pending$' "$log"
check "dry-run lines are not counted" [ "$(interventions)" = 5 ]

# 9. btcctl failure: exit 1, state intact
touch "$STUB_DIR/fail"
tick 480; check "btcctl failure exits 1" [ "$rc" = 1 ]
check "btcctl failure is reported" grep -q 'getblockcount failed' <<<"$out"
rm "$STUB_DIR/fail"
tick 490; check "state survived the failure (dry-run outcome settled)" grep -q ' dry-run height=105 .* outcome=no-effect$' "$log"

echo
if [ "$fails" -eq 0 ]; then echo "sync-watchdog: all checks passed"; else echo "sync-watchdog: $fails check(s) FAILED"; exit 1; fi
