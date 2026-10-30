# Repository entry points. `make ci` is the arbiter (ARCHITECTURE.md section 0, rule 5).
GO ?= go
BUF ?= buf
COMPOSE ?= docker compose -f deploy/compose/docker-compose.yml
GATEWAY := services/bank/ledger-gateway-bank

.PHONY: ci test lint vet fmt-check test-isolation test-ledger test-ledger-pg test-ledger-tb test-policy policy-manifest test-platform test-temporal proto up down

ci: lint test test-isolation

test: test-ledger test-policy test-platform
	cd pkg/money && $(GO) test ./...
	cd tools/isolation && $(GO) test ./...
	cd pkg/callerauth && $(GO) test ./...

# Ledger gateway. Postgres tests run only when LEDGER_TEST_DSN is set (see test-ledger-pg).
test-ledger:
	cd $(GATEWAY) && $(GO) test -count=1 ./...

# Runs the shared behaviour suite and the restart test against Postgres.
# Tables are truncated; use a throwaway database.
test-ledger-pg:
	@test -n "$$LEDGER_TEST_DSN" || (echo "set LEDGER_TEST_DSN to a throwaway database"; exit 1)
	cd $(GATEWAY) && $(GO) test -count=1 -v ./internal/postgres/

# Policy service: Rego tests, bundle verification and decision tests. Decision tests need POLICY_TEST_DSN.
test-policy:
	cd services/shared/policy-service && $(GO) test -count=1 ./...

# Signs policy/ into manifest.json and manifest.sig. Needs POLICY_SIGNING_KEY from the secret store and a
# version string. Never commit the key. Commit the regenerated manifest only with the rule changes it covers.
policy-manifest:
	@test -n "$$POLICY_VERSION" || (echo "set POLICY_VERSION, for example 2026.10.29-1"; exit 1)
	cd services/shared/policy-service && $(GO) run ./cmd/policy-sign -dir ../../../policy -version "$$POLICY_VERSION"

# Party, consent and onboarding services. Database tests need PARTY_TEST_DSN, CONSENT_TEST_DSN, and
# ONBOARDING_TEST_PG (a base DSN; the onboarding integration test creates and drops its own databases).
test-platform:
	cd services/platform/party-service && $(GO) test -count=1 ./...
	cd services/platform/consent-service && $(GO) test -count=1 ./...
	cd services/platform/onboarding && $(GO) test -count=1 ./...

# Onboarding workflow and API against a real Temporal server. Start one first, for example:
#   temporal server start-dev --headless --port 7233
# then: TEMPORAL_HOST=127.0.0.1:7233 make test-temporal
test-temporal:
	@test -n "$$TEMPORAL_HOST" || (echo "set TEMPORAL_HOST, for example 127.0.0.1:7233"; exit 1)
	cd services/platform/onboarding && $(GO) test -tags temporaldev -count=1 -v -timeout 600s ./internal/onboarding/ ./internal/service/

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
	cd pkg/outbox && $(GO) vet ./...
	cd pkg/callerauth && $(GO) vet ./...
	cd services/shared/policy-service && $(GO) vet ./...
	cd services/platform/party-service && $(GO) vet ./...
	cd services/platform/consent-service && $(GO) vet ./...
	cd services/platform/onboarding && $(GO) vet ./... && $(GO) vet -tags temporaldev ./internal/...
	cd pkg/uuid && $(GO) vet ./...

test-isolation:
	cd tools/isolation && $(GO) run . ../..

# Regenerates Go code from proto/. Requires buf and the protoc-gen-go and protoc-gen-connect-go plugins on PATH.
proto:
	cd proto && $(BUF) lint && $(BUF) generate

up:
	$(COMPOSE) up -d --wait

down:
	$(COMPOSE) down -v
