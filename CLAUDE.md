# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

btcd.watch is a beginner-friendly Bitcoin transaction/address/block explorer served
by a single Go binary that reads a local **btcd** node over websocket RPC and embeds
a React SPA. `docs/ARCHITECTURE.md` is the authoritative design document and is kept
updated as part of each change.

## Commands

```sh
make build          # npm ci + vite build → embed web/dist → bin/btcdwatchd
make test           # go test ./... -race  AND  tsc -b + vitest in web/  AND  deploy/tests/*.sh
make fmt            # gofmt -w ./cmd ./internal
make regtest-up     # Docker btcd + bitcoind miner/txgen (harness/README.md)
make regtest-down   # tear down and delete all harness state
make regtest-logs   # follow miner/txgen output
```

Single tests:

```sh
go test ./internal/explorer -run TestOnBlockDoesNotBlockBehindIntervalMeasurement -race
cd web && npx vitest run src/lib/format.test.ts
```

Go tests use a mock `node.Backend` with btcjson fixtures — no node required. Always
run Go tests with `-race`; several regression tests exist specifically to catch
notification-goroutine deadlocks and only fail under the race detector's scheduling.

## Dev loop

Two processes against a node. With the Docker harness:

```sh
make regtest-up
export BTCDWATCH_NETWORK=regtest BTCDWATCH_RPC_HOST=127.0.0.1:18334 \
       BTCDWATCH_RPC_USER=regtest BTCDWATCH_RPC_PASS=regtest \
       BTCDWATCH_RPC_CERT="$PWD/harness/.data/btcd/rpc.cert"
go run ./cmd/btcdwatchd          # API on :8480
cd web && npm run dev            # SPA on :5174, proxies /api (incl. ws) to :8480
```

`make dev` / `make run` (`scripts/env-map.sh`) instead inject credentials from the
maintainer's external `btc-regtest-env` harness — not usable without that checkout.

## Architecture

**Request path.** `cmd/btcdwatchd/main.go` wires everything: config → `chain.ParamsForNetwork`
→ `node.New` (rpcclient, websocket mode) → `explorer.NewService` → `api.NewHub` →
`api.New` (routing + hardening middleware) → `http.Server`. Notification handlers
(`OnConnect` / `OnBlock` / `OnTxAccepted`) are registered *before* `backend.Start`, so the
consumers exist when the first notification lands.

**The `node.Backend` seam.** `internal/explorer` never imports `rpcclient` — only the
narrow `node.Backend` interface (`internal/node/client.go`). This is what makes every
derivation unit-testable against `internal/explorer/mock_test.go`. Preserve it.

**The live cache (`internal/explorer/live.go`).** Stats, fees, and the mempool update are
never computed on the request path. `liveSnapshot()` returns the last cached values
immediately and kicks a background recompute when one is due (5s), with a 2-minute
stuck-refresh watchdog; failed feeds keep their previous value. `RunLiveRefresh` (a 5s
ticker started in main) keeps it warm with zero viewers. Reason: btcd stalls the RPCs
these depend on for *minutes* while flushing its UTXO cache, and the dashboard must keep
answering. The same shape applies to `SyncStatus` — a background single-flight probe,
never a blocking call.

**WebSocket hub (`internal/api/hub.go`).** One event-loop goroutine owns all shared state
(client registry, per-client watched-txid sets, buffered send channels) — no mutexes on
the hot path. A client whose send buffer is full is dropped, so one stuck reader can never
stall the hub. `ws.go` holds the per-connection read/write pumps.

**Sync gating.** btcd exposes no `initialblockdownload` flag, so "syncing" is inferred from
tip age vs wall clock (>4h), gated off on regtest/simnet (`tipAgeGated`). While syncing,
lookups return `503 node_syncing`, `/api/healthz` reports `status:"syncing"` at HTTP 200,
and the UI hides search/stats/mempool/fees.

**Degraded, never dead.** If btcd is down at startup or drops, the server does not exit:
REST returns `503 node_unavailable`, healthz reports the state, WS clients keep their
subscriptions, and rpcclient reconnects. Notification registrations do **not** survive a
reconnect — they are re-issued from the connect handler.

**Serving.** `web/embed.go` embeds `web/dist` via `//go:embed all:dist`; `api/static.go`
serves it with SPA fallback so `/?q=<txid>` deep links work on cold load. Unknown `/api/*`
paths return JSON 404 rather than the SPA. `--static-dir` overrides the embedded build.

**Frontend.** No router: `web/src/state.ts` is a `useReducer` state machine (`view` +
payload), `useSearch` classifies via `/api/search` and dispatches `search-result`, and
`?q=` is maintained with `history.replaceState`. `web/src/appConfig.ts` holds build-time
design switches (motion level, live-mempool toggle, poll intervals). Styling is plain CSS
with `bp-`-prefixed classes and tokens in `web/src/styles/`; there is no CSS framework.

## Invariants

- **Never hold a mutex across a node RPC.** This caused four production deadlocks. `OnBlock`
  and `OnTxAccepted` run on the rpcclient *notification* goroutine, which also drains the
  websocket; if that goroutine blocks on a lock held by an in-flight RPC, the RPC response
  is never read and the entire node connection freezes (site alive, all data frozen, no
  reconnect). Read-copy-unlock, do the RPC, then re-lock to store. See the comments at
  `internal/explorer/tx.go:486` and the regression tests in `stats_test.go`.
  Diagnosis for any future "connected but frozen": SIGQUIT goroutine dump first.
- **No hardcoded network constants.** Address HRP, halving interval, block target, subsidy —
  all flow from `internal/chain` so mainnet/testnet3/signet/regtest/simnet all work.
- **Amounts are satoshis as JSON `int64`**, never float BTC. Fiat is computed server-side.
- **Hardening knobs default to off** (rate limit, trusted proxy header, WS cap, address-scan
  semaphore) so localhost users are unaffected; `config.example.yaml` carries the values to
  use when exposed publicly. `trusted_proxy_header` parses the first XFF entry — only safe
  behind a replacing proxy such as Cloudflare.
- **No secrets in the repo.** Credentials come from `config.yaml` (gitignored) or `BTCDWATCH_*`
  env, which wins over the file.

## btcd 0.26 imports

btcd 0.26 split into nested v2 modules; most training-data examples are wrong. Use
`chaincfg/v2`, `chainhash/v2`, `btcutil/v2`, `wire/v2`, `txscript/v2`, and the **new**
`address/v2` package for `DecodeAddress` / the `Address` interface. `rpcclient` and
`btcjson` stay in the root `github.com/btcsuite/btcd` module. `btcjson.ScriptPubKeyResult`
carries both `Addresses []string` (deprecated) and `Address string` — handle both.
`docs/ARCHITECTURE.md` §9 lists the RPCs btcd lacks vs Bitcoin Core and the workarounds.

## Gotchas

- `vite build` empties `web/dist`, deleting the committed `dist/.keep` that keeps the Go
  embed valid. `make build` restores it — but check `git status` before committing, since
  `git add -A` will happily stage its deletion.
- The dev server is pinned to **:5174 with `strictPort`**. Do not move it to 5173.
- Use `npm ci`, not `npm install`, in build paths: an install that rewrote
  `package-lock.json` used to break `deploy/upgrade.sh`'s clean-tree check.

## Workflow

Feature branch off `master` → PR → the maintainer merges (no CI/CD). Run `make test` and
`make fmt` before opening one, and update `docs/ARCHITECTURE.md` when behavior changes.
Production deploys are manual and tag-based: tag a release, then run
`deploy/upgrade.sh <tag>` on the VPS (builds from the tag, keeps the previous binary,
gates on healthz, auto-rolls-back). See `deploy/README.md`.
