# Regtest harness

A self-contained regtest network for running btcdwatch end-to-end on your own
machine — no external node, no personal wallet, nothing but Docker required.

```
 bitcoind ──p2p──▶ btcd ──RPC──▶ btcdwatchd (on the host)
 mines + txgen     node under test
 (Bitcoin Core)    (txindex + addrindex, built from source)
```

Bitcoin Core owns the wallets and produces the traffic (blocks on an interval
plus a trickle of transactions, ~5% multi-output and ~2% RBF). btcd peers with
Core over P2P and is the node btcdwatch actually reads. Everything runs in
disposable containers; `make regtest-down` removes all of it.

## Requirements

- Docker with Compose v2 (`docker compose`).
- Nothing else — btcd is built from pinned source; Bitcoin Core is a pinned image.

## Usage

From the repository root:

```sh
make regtest-up      # build + start btcd, bitcoind, and the churn generator
make regtest-logs    # follow the miner/txgen output
make regtest-down    # stop everything and delete all state
```

First start takes a minute (it builds btcd and pulls Bitcoin Core, then mines
101 blocks to mature the miner's coinbase). After that you'll see blocks and
transactions streaming in `make regtest-logs`.

Once btcdwatch is running against it (below), `bash harness/scripts/verify.sh`
is a read-only smoke test that hits the API and prints the height, mempool,
a live pending tx (with its RBF flag + fee rate), the tip block, and a txgen
address's balance and activity.

## Point btcdwatch at it

btcd's RPC is published on `127.0.0.1:18334` and its self-signed cert is written
to `harness/.data/btcd/rpc.cert`. Run the explorer on the host against it:

```sh
export BTCDWATCH_NETWORK=regtest
export BTCDWATCH_RPC_HOST=127.0.0.1:18334
export BTCDWATCH_RPC_USER=regtest
export BTCDWATCH_RPC_PASS=regtest
export BTCDWATCH_RPC_CERT="$PWD/harness/.data/btcd/rpc.cert"

go run ./cmd/btcdwatchd            # API on :8480
# in another shell, for the SPA with hot reload:
cd web && npm run dev              # :5174, /api proxied
```

Open <http://localhost:5174> and walk the end-to-end recipe in the top-level
README (paste a txid from the churn logs, press **Watch**, etc.).

## Configuration

Settings live in [`.env`](.env): pinned image/version tags, the (public,
regtest-only) RPC credentials, and the block/transaction cadence
(`BLOCK_INTERVAL`, `TX_INTERVAL`). The credentials are safe to commit **only**
because this network is isolated and localhost-only — never reuse them.

## Notes

- The RPC credentials and `--connect=bitcoind` wiring assume the isolated
  Docker network; do not expose these containers publicly.
- No official Docker image exists for either btcd or Bitcoin Core. btcd is
  therefore built from source (`Dockerfile.btcd`, pinned to the `BTCD_VERSION`
  the app targets) and Bitcoin Core is a pinned community image — after the
  first pull, pin it by `@sha256` digest in `.env` for full reproducibility.
