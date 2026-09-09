#!/usr/bin/env bash
# Probe /api/healthz and decide whether the service is healthy.
#
#   deploy/healthz-check.sh [URL]      (default: https://btcd.watch/api/healthz)
#
# Exit 0 when the endpoint answers HTTP 200 with status "ok"; exit 1 for
# anything else — connection failure, timeout, 503 degraded, or a 200 with
# status "syncing" (which healthz deliberately reports as HTTP 200 so a
# naive uptime monitor won't page over IBD; on mainnet a synced tip is never
# four hours old, so "syncing" after launch means the node has stalled).
# Prints a one-line reason either way so callers can quote it in an alert.
set -u

url="${1:-https://btcd.watch/api/healthz}"
tmp="$(mktemp)"
errs="$(mktemp)"
trap 'rm -f "$tmp" "$errs"' EXIT

# Retry transient failures so a single dropped connection doesn't alert.
code="$(curl -sS -o "$tmp" -w '%{http_code}' \
    --max-time 20 --retry 2 --retry-delay 10 --retry-all-errors \
    "$url" 2>"$errs")" || code="000"
body="$(tr -d '\n' <"$tmp" | head -c 500)"
[ -s "$tmp" ] || body="$(tail -n 1 "$errs")"
status="$(printf '%s' "$body" | sed -n 's/.*"status" *: *"\([a-z_]*\)".*/\1/p')"

if [ "$code" = "200" ] && [ "$status" = "ok" ]; then
    echo "healthy: $body"
    exit 0
fi

case "$code" in
    000) echo "unhealthy: no response from $url (${body:-timeout})" ;;
    *)   echo "unhealthy: HTTP $code status=${status:-?} body=$body" ;;
esac
exit 1
