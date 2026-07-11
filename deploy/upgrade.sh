#!/usr/bin/env bash
# Tag-based production upgrade for btcdwatch.com.
#
#   ./deploy/upgrade.sh v1.2.0        # deploy a tag
#   ./deploy/upgrade.sh --rollback    # restore the previous binary
#
# Run as the btcdwatch user from the repo checkout. Deploys are tags-only
# so "what is running" is always answerable; master is never deployed.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
API=http://127.0.0.1:8480

healthcheck() {
    for _ in $(seq 1 15); do
        sleep 1
        if curl -sf "$API/api/healthz" >/dev/null; then
            echo "healthz OK: $(curl -s "$API/api/healthz")"
            return 0
        fi
    done
    return 1
}

if [[ "${1:-}" == "--rollback" ]]; then
    [[ -x bin/btcdwatchd.prev ]] || { echo "no previous binary to roll back to" >&2; exit 1; }
    cp bin/btcdwatchd.prev bin/btcdwatchd
    sudo systemctl restart btcdwatchd
    healthcheck || { echo "ROLLBACK UNHEALTHY — investigate immediately" >&2; exit 1; }
    echo "rolled back."
    exit 0
fi

TAG="${1:?usage: upgrade.sh <tag> | --rollback}"

# Refuse to build from a dirty tree — the deployed binary must be exactly
# the tag.
if [[ -n "$(git status --porcelain)" ]]; then
    echo "working tree not clean — refusing to deploy" >&2
    exit 1
fi

git fetch --tags origin
git rev-parse -q --verify "refs/tags/$TAG" >/dev/null \
    || { echo "unknown tag: $TAG" >&2; exit 1; }

echo "== deploying $TAG (currently: $(git describe --tags --always)) =="
git checkout -q "$TAG"

# Keep the last-known-good binary for instant rollback.
[[ -x bin/btcdwatchd ]] && cp bin/btcdwatchd bin/btcdwatchd.prev

make build

sudo systemctl restart btcdwatchd
if healthcheck; then
    echo "== $TAG deployed =="
else
    echo "UNHEALTHY after restart — rolling back" >&2
    exec "$0" --rollback
fi
