# btcdwatch.com

A beginner-friendly Bitcoin transaction & address explorer that answers one
question well: **"is my Bitcoin confirmed?"** — powered entirely by your own
[btcd](https://github.com/btcsuite/btcd) node. *Don't trust. Verify.*

Paste a transaction ID, address, or block height/hash and get a
plain-English answer: pending (with your place in the mempool queue and a
live **Watch** mode that flips to "Confirmed 🎉" the moment a block lands),
confirmed (with a 6-segment safety meter and script-type chips — plus an
RBF badge while replaceable), an address summary with balance, activity,
and a plain-English address-type explainer, or a block with its
transaction list. The landing page
doubles as a network dashboard: a live mempool "queue" that grows and
shrinks with real traffic (with a per-transaction "just joined the line"
feed and a flash when a block is mined), block height, halving countdown,
and BTC price — with a fee ticker in the header that opens the fee helper
from any view.

- **Architecture**: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)

## Requirements

- **btcd 0.26+** with `txindex=1` and `addrindex=1`, websocket RPC enabled.
- Go 1.25+, Node 22+ (for building the frontend).
- Development assumes the `btc-regtest-env` harness as a sibling directory
  (its `scripts/env.sh` provides RPC credentials); any btcd works via
  config/env.

## Quickstart (development)

Two processes with hot reload:

```sh
./scripts/dev.sh            # Go API on :8480 (creds from btc-regtest-env)
cd web && npm install && npm run dev   # SPA on :5174, /api proxied
```

Open <http://localhost:5174>.

## Production build (single binary)

```sh
make build      # npm build → embeds web/dist → bin/btcdwatchd
./scripts/run.sh              # regtest harness credentials
# or, standalone:
./bin/btcdwatchd --config config.yaml
```

The binary serves the SPA and the API from one port with an SPA fallback,
so shared links like `http://host:8480/?q=<txid>` work on cold load.
`--static-dir <path>` overrides the embedded frontend.

## Configuration

YAML file (`--config`, or `config.yaml` in the working directory) with
`BTCDWATCH_*` environment overrides — env wins. Copy
[`config.example.yaml`](config.example.yaml) to start. **Never commit a
config containing credentials** (`config.yaml` is gitignored).

| Key | Env | Default |
| --- | --- | --- |
| `server.listen` | `BTCDWATCH_LISTEN` | `127.0.0.1:8480` |
| `node.network` | `BTCDWATCH_NETWORK` | `regtest` |
| `node.rpc_host` | `BTCDWATCH_RPC_HOST` | `127.0.0.1:18334` |
| `node.rpc_user` / `node.rpc_pass` | `BTCDWATCH_RPC_USER` / `_PASS` | — (required) |
| `node.rpc_cert` | `BTCDWATCH_RPC_CERT` | — |
| `price.source` (`coingecko`\|`static`) | `BTCDWATCH_PRICE_SOURCE` | `coingecko` |
| `price.static_usd` | `BTCDWATCH_PRICE_STATIC_USD` | `98000` |
| `price.refresh_seconds` | `BTCDWATCH_PRICE_REFRESH_SECONDS` | `60` |
| `fees.floor_slow` / `_standard` / `_urgent` | `BTCDWATCH_FEES_FLOOR_*` | `1` / `2` / `5` |
| `address.max_scan_txs` | `BTCDWATCH_ADDRESS_MAX_SCAN_TXS` | `2000` |

## API

All endpoints under `/api`; amounts are satoshis. See
[ARCHITECTURE.md §4–5](docs/ARCHITECTURE.md) for full schemas.

| Endpoint | Purpose |
| --- | --- |
| `GET /api/search?q=` | Classify + resolve a block height/hash, txid, or address |
| `GET /api/tx/{txid}` | Transaction detail (fee, from/to, queue position) |
| `GET /api/address/{addr}?offset&limit` | Balance, totals, paginated activity |
| `GET /api/block/{heightOrHash}?offset&limit` | Block stats + paginated tx list |
| `GET /api/fees` | Three fee tiers from mempool percentiles |
| `GET /api/stats` | Height, mempool + queue bands, ETAs, halving, price |
| `GET /api/ws` | WebSocket: live stats, mempool queue + arrivals, block flashes, watched-tx pushes |
| `GET /api/healthz` | Node connectivity |

## Testing

```sh
make test    # go test ./... plus tsc + vitest in web/
```

Go tests run against a mocked node backend with btcjson fixtures — no
running node needed. The WebSocket hub suite runs under the race detector
in CI-style usage: `go test ./... -race`.

### End-to-end recipe (regtest)

This exercises the live flows against a regtest node that is actively
producing traffic. It needs an **external** harness — the `btc-regtest-env`
used in development, or any equivalent that mines blocks and generates
transactions on a loop. Those harness scripts are not part of this
repository.

1. Start your regtest node with a miner and a transaction generator running,
   then launch the app with `./scripts/dev.sh` + Vite (or the single binary).
2. Take a fresh txid from the generated traffic → the pending view shows your
   place in the mempool queue and the estimated wait.
3. Press **🔔 Watch this transaction** → the panel shows it is live-connected;
   on the next mined block the view flips to **Confirmed** with the 🎉
   banner — no refresh.
4. Search an address that appears in the traffic → balance, totals, and
   activity grow as the generator keeps running.
5. Search a block height (or click **In block** on a confirmed tx) → the
   block view lists its transactions, coinbase first with a `miner reward`
   badge; prev/next buttons walk the chain.

## Layout

```
cmd/btcdwatchd/    server entry point
internal/config    YAML + env configuration
internal/chain     network params, query classification, halving math
internal/node      websocket rpcclient behind the mockable Backend seam
internal/explorer  fee/from-to/amount derivation, mempool snapshot + queue,
                   fees/stats/block/address aggregation
internal/price     BTC/USD quote (CoinGecko + static fallback)
internal/api       REST handlers, WebSocket hub, embedded-SPA serving
web/               Vite + React + TypeScript SPA (embedded via web/embed.go)
```

## Contributing

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) for setup,
commands, and conventions.

## License

Released under the [MIT License](LICENSE).
