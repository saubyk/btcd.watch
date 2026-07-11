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
- A btcd node to talk to. The bundled Docker harness gives you one with live
  regtest traffic and zero setup (`make regtest-up`; see
  [harness/README.md](harness/README.md)); any btcd also works via config/env.

## Quickstart (development)

Bring up a regtest node with live traffic, then run the two app processes with
hot reload:

```sh
make regtest-up                         # btcd + miner/txgen in Docker
go run ./cmd/btcdwatchd                 # Go API on :8480 — see harness/README.md
                                        #   for the BTCDWATCH_* env it needs
cd web && npm install && npm run dev    # SPA on :5174, /api proxied
```

Open <http://localhost:5174>. (`./scripts/dev.sh` is a shortcut that injects RPC
credentials from a local harness, if you run your own instead of the Docker one.)

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
| `server.rate_limit_per_min` | `BTCDWATCH_RATE_LIMIT_PER_MIN` | `0` (off) |
| `server.rate_limit_burst` | `BTCDWATCH_RATE_LIMIT_BURST` | `0` (= per-min budget) |
| `server.trusted_proxy_header` | `BTCDWATCH_TRUSTED_PROXY_HEADER` | — (socket address) |
| `server.max_ws_clients` | `BTCDWATCH_MAX_WS_CLIENTS` | `0` (unlimited) |
| `node.network` | `BTCDWATCH_NETWORK` | `regtest` |
| `node.rpc_host` | `BTCDWATCH_RPC_HOST` | `127.0.0.1:18334` |
| `node.rpc_user` / `node.rpc_pass` | `BTCDWATCH_RPC_USER` / `_PASS` | — (required) |
| `node.rpc_cert` | `BTCDWATCH_RPC_CERT` | — |
| `node.rpc_notls` | `BTCDWATCH_RPC_NOTLS` | `false` (set for btcd `notls=1`) |
| `price.source` (`coingecko`\|`static`) | `BTCDWATCH_PRICE_SOURCE` | `coingecko` |
| `price.static_usd` | `BTCDWATCH_PRICE_STATIC_USD` | `98000` |
| `price.refresh_seconds` | `BTCDWATCH_PRICE_REFRESH_SECONDS` | `60` |
| `fees.floor_slow` / `_standard` / `_urgent` | `BTCDWATCH_FEES_FLOOR_*` | `1` / `2` / `5` |
| `address.max_scan_txs` | `BTCDWATCH_ADDRESS_MAX_SCAN_TXS` | `2000` |
| `address.max_concurrent_scans` | `BTCDWATCH_ADDRESS_MAX_CONCURRENT_SCANS` | `0` (unlimited) |

The hardening knobs (rate limit, proxy header, WS cap, scan cap) default to
off for localhost use; **set them all before exposing the server publicly**
— `config.example.yaml` carries recommended starting values.

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
producing traffic. The bundled Docker harness provides exactly that — see
[harness/README.md](harness/README.md).

1. `make regtest-up` starts btcd plus a miner/txgen; point `btcdwatchd` at it
   (env in [harness/README.md](harness/README.md)) and run the SPA with `npm run
   dev` (or the single binary). `bash harness/scripts/verify.sh` is a quick
   read-only check that data is flowing.
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
