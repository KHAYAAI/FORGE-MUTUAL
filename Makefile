# Repository entry points. `make ci` is the arbiter (ARCHITECTURE.md section 0, rule 5).
GO ?= go
BUF ?= buf
COMPOSE ?= docker compose -f deploy/compose/docker-compose.yml
GATEWAY := services/bank/ledger-gateway-bank

.PHONY: ci test lint vet fmt-check test-isolation test-ledger test-ledger-pg test-ledger-tb proto up down

ci: lint test test-isolation

test: test-ledger
	cd pkg/money && $(GO) test ./...
	cd tools/isolation && $(GO) test ./...

# Ledger gateway. Postgres tests run only when LEDGER_TEST_DSN is set (see test-ledger-pg).
test-ledger:
	cd $(GATEWAY) && $(GO) test -count=1 ./...

# Runs the shared behaviour suite and the restart test against Postgres.
# Tables are truncated; use a throwaway database.
test-ledger-pg:
	@test -n "$$LEDGER_TEST_DSN" || (echo "set LEDGER_TEST_DSN to a throwaway database"; exit 1)
	cd $(GATEWAY) && $(GO) test -count=1 -v ./internal/postgres/

# Runs the shared behaviour suite against a live TigerBeetle cluster.
test-ledger-tb:
	@test -n "$$TB_ADDRESSES" -a -n "$$TB_CLUSTER_ID" || (echo "set TB_ADDRESSES and TB_CLUSTER_ID"; exit 1)
	cd $(GATEWAY) && $(GO) test -count=1 -tags tbintegration -v ./internal/ledger/

lint: fmt-check vet

fmt-check:
	@out=$$(gofmt -l pkg tools services); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	cd pkg/money && $(GO) vet ./...
	cd tools/isolation && $(GO) vet ./...
	cd $(GATEWAY) && $(GO) vet ./... && $(GO) vet -tags tbintegration ./internal/ledger/

test-isolation:
	cd tools/isolation && $(GO) run . ../..

# Regenerates Go code from proto/. Requires buf and the protoc-gen-go and protoc-gen-connect-go plugins on PATH.
proto:
	cd proto && $(BUF) lint && $(BUF) generate

up:
	$(COMPOSE) up -d --wait

down:
	$(COMPOSE) down -v
