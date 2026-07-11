.PHONY: build web-build go-build test test-go test-web dev run fmt clean \
	regtest-up regtest-down regtest-logs

build: web-build go-build

web-build:
	cd web && npm install --no-audit --no-fund && npm run build
	@touch web/dist/.keep # vite empties dist; restore the committed embed placeholder

go-build:
	go build -o bin/btcdwatchd ./cmd/btcdwatchd

test: test-go test-web

test-go:
	go test ./... -race

test-web:
	cd web && npx tsc -b && npx vitest run

dev:
	./scripts/dev.sh

run:
	./scripts/run.sh

fmt:
	gofmt -w ./cmd ./internal

clean:
	rm -rf bin web/dist/assets web/dist/index.html

# --- Local regtest harness (Docker) — see harness/README.md ---------------
regtest-up:
	cd harness && docker compose up -d --build
	@echo "btcd RPC → 127.0.0.1:18334 · cert → harness/.data/btcd/rpc.cert"
	@echo "next: point btcdwatchd at it (see harness/README.md), then 'make regtest-logs'"

regtest-down:
	cd harness && docker compose down -v

regtest-logs:
	cd harness && docker compose logs -f
