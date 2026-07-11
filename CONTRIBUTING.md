# Contributing to btcd.watch

Thanks for your interest in improving btcd.watch! This project is a
beginner-friendly Bitcoin explorer backed by your own [btcd](https://github.com/btcsuite/btcd)
node. Contributions of all sizes are welcome — bug fixes, features, docs, and
tests.

By contributing, you agree that your contributions are licensed under the
project's [MIT License](LICENSE).

## Getting started

Requirements:

- **btcd 0.26+** with `txindex=1` and `addrindex=1`, websocket RPC enabled.
- **Go 1.25+** and **Node 22+**.

The app needs a btcd node to talk to. The simplest path is the bundled Docker
harness ([`harness/README.md`](harness/README.md)), which brings up btcd plus a
miner and transaction generator so real pending/confirmed data exists from the
first request. Any btcd node also works via `config.yaml` (copy
`config.example.yaml`) or `BTCDWATCH_*` environment variables.

Run the harness, then the two dev processes with hot reload:

```sh
make regtest-up                        # btcd + miner/txgen in Docker
go run ./cmd/btcdwatchd                # Go API on :8480 — see harness/README.md
                                       #   for the BTCDWATCH_* env it needs
cd web && npm install && npm run dev   # SPA on :5174, /api proxied
```

Then open <http://localhost:5174>. (`./scripts/dev.sh` / `make dev` is a
shortcut that injects credentials from a local harness, if you run your own.)

## Common commands

All driven from the `Makefile`:

| Command | Purpose |
|---------|---------|
| `make build` | Build the frontend and the single `bin/btcdwatchd` binary (embeds the SPA) |
| `make test` | Run all tests: `go test ./... -race`, `tsc -b`, and `vitest` |
| `make fmt` | Format Go sources with `gofmt` |
| `make dev` | Start the Go API against the regtest harness |
| `make run` | Run the built binary (requires `make build` first) |

Please run `make test` and `make fmt` before opening a pull request.

## Project layout & design rules

Read [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) first — it is the
authoritative technical design. A few conventions worth calling out:

- **`internal/explorer` never imports `rpcclient` directly** — it talks to
  the node only through the `node.Backend` interface. This keeps data
  derivation unit-testable with a mock backend (see `internal/explorer/mock_test.go`).
- **No hardcoded network constants.** Everything network-dependent (address
  HRP, halving interval, etc.) flows from `internal/chain` so the app works
  across mainnet/testnet/regtest/signet/simnet.
- **No secrets in the repo.** Credentials come from config/env only; never
  commit `config.yaml` (it's gitignored).

## Making a change

1. Fork the repo and create a topic branch off `master`.
2. Make your change with tests where it makes sense — the derivation logic in
   `internal/explorer` and `internal/chain` is covered by table-driven unit
   tests against the mock backend; please extend them for new behavior.
3. Run `make test` and `make fmt`.
4. Open a pull request against `master` describing **what** changed and
   **why**. Link any related issue.

## Reporting bugs & requesting features

Open a GitHub issue. For bugs, include the network you're on (regtest,
mainnet, …), your btcd version, and steps to reproduce. For UI issues, a
screenshot helps.

## Code of conduct

Be respectful and constructive. We want this to be a welcoming project for
newcomers to Bitcoin and to open source alike.
