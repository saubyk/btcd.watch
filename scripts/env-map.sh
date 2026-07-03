# Maps btc-regtest-env credentials into BTCDWATCH_* variables. Sourced by
# dev.sh (go run) and run.sh (built binary) — nothing secret touches the
# repo.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_SH="${BTC_REGTEST_ENV_SH:-$ROOT/../btc-regtest-env/scripts/env.sh}"

if [[ ! -f "$ENV_SH" ]]; then
    echo "error: regtest env file not found: $ENV_SH" >&2
    echo "set BTC_REGTEST_ENV_SH to your btc-regtest-env/scripts/env.sh" >&2
    exit 1
fi

# shellcheck source=/dev/null
source "$ENV_SH"

export BTCDWATCH_NETWORK=regtest
export BTCDWATCH_RPC_HOST="127.0.0.1:${BTCD_RPC_PORT}"
export BTCDWATCH_RPC_USER="$BTCD_RPC_USER"
export BTCDWATCH_RPC_PASS="$BTCD_RPC_PASS"
export BTCDWATCH_RPC_CERT="$BTCD_RPC_CERT"
