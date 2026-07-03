.PHONY: build web-build go-build test test-go test-web dev run fmt clean

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
