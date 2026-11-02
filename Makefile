# Repository entry points. `make ci` is the arbiter (ARCHITECTURE.md section 0, rule 5).
SHELL := /bin/bash
GO ?= go
BUF ?= buf
COMPOSE ?= docker compose -f deploy/compose/docker-compose.yml
GATEWAY := services/bank/ledger-gateway-bank

# Suites that need a database or a broker. make ci refuses to pass unless every one is configured, because a skipped
# suite proves nothing. `make ci-db` creates the databases and sources the environment for you.
DB_TEST_VARS := LEDGER_TEST_DSN RECON_TEST_DSN PAYMENTS_TEST_PG INBOUND_TEST_DSN AUDIT_TEST_DSN TREASURY_TEST_DSN \
	COMPLIANCE_TEST_DSN POLICY_TEST_DSN PARTY_TEST_DSN CONSENT_TEST_DSN ONBOARDING_TEST_PG OUTBOX_TEST_DSN FX_TEST_DSN PAYOUTS_TEST_DSN \
	CONSOLIDATED_TEST_DSN CORE_BRIDGE_TEST_DSN PARTNER_ROUTER_TEST_DSN NOTIFICATIONS_TEST_DSN EVENTBUS_TEST_DSN KAFKA_TEST_BROKERS

.PHONY: ci ci-db require-test-dbs test-outbox test-fx test-payouts test-consolidated test-core-bridge test-partner-router test-notifications test-eventbus test-workspace test-pkg test-web test lint vet fmt-check test-isolation test-banklayer test-recon test-audit test-treasury test-compliance test-reg-reporting test-ledger test-ledger-pg test-ledger-tb test-policy policy-manifest test-platform test-bank test-temporal proto up down

ci: require-test-dbs lint test test-isolation

# Fails unless every database and broker variable is set. FORGE_SKIP_DB_TESTS=1 skips them knowingly, and says so.
require-test-dbs:
	@if [ "$(FORGE_SKIP_DB_TESTS)" = "1" ]; then \
	  echo "WARNING: FORGE_SKIP_DB_TESTS=1: database and Kafka suites are SKIPPED. This run proves less."; exit 0; fi; \
	missing=""; for v in $(DB_TEST_VARS); do [ -n "$${!v}" ] || missing="$$missing $$v"; done; \
	if [ -n "$$missing" ]; then \
	  echo "make ci needs these set so the database and Kafka suites run:$$missing"; \
	  echo "run 'make ci-db' (throwaway databases on the compose Postgres, broker from 'make up'), or set FORGE_SKIP_DB_TESTS=1 to skip knowingly"; \
	  exit 1; fi

# Creates throwaway databases on the Postgres at FORGE_PG_HOST_DSN (default: compose), then runs the full gate with them.
ci-db:
	scripts/test-databases.sh
	set -a; . ./.test-dbs.env; set +a; $(MAKE) ci

test: test-ledger test-policy test-platform test-bank test-banklayer test-recon test-audit test-treasury test-compliance test-reg-reporting test-outbox test-fx test-payouts test-consolidated test-core-bridge test-partner-router test-notifications test-eventbus test-workspace test-pkg test-web
	cd pkg/money && $(GO) test ./...
	cd tools/isolation && $(GO) test ./...
	cd pkg/callerauth && $(GO) test ./...

# FX service. The saga and service suites need FX_TEST_DSN and truncate the same tables, so packages run one at a time.
test-fx:
	cd services/bank/fx-service && $(GO) test -count=1 -p 1 ./...

# Payouts. The batch and service suites need PAYOUTS_TEST_DSN and truncate the same tables, so packages run one at a time.
test-payouts:
	cd services/bank/payouts-service && $(GO) test -count=1 -p 1 ./...

# Platform read model, bank-side services and the event bus. Each database-backed suite needs its own DSN from
# make ci-db. Packages run one at a time because they share a database; in parallel they truncate each other's rows.
test-consolidated:
	cd services/platform/consolidated-view && $(GO) test -count=1 -p 1 ./...

test-core-bridge:
	cd services/bank/core-bridge && $(GO) test -count=1 -p 1 ./...

test-partner-router:
	cd services/bank/partner-bank-router && $(GO) test -count=1 -p 1 ./...

test-notifications:
	cd services/platform/notifications && $(GO) test -count=1 -p 1 ./...

test-eventbus:
	cd pkg/eventbus && $(GO) test -count=1 -p 1 ./...

# The workspace BFF keeps no state, so its tests need no database.
test-workspace:
	cd services/platform/workspace-api && $(GO) test -count=1 ./...

test-pkg:
	cd pkg/events && $(GO) test -count=1 ./...
	cd pkg/systemaccounts && $(GO) test -count=1 ./...

# The customer web app: typecheck, production build, then the browser journeys. Needs Node 22 and Chromium. Set
# PLAYWRIGHT_BROWSERS_PATH, or CHROMIUM_PATH to a Chromium that is already installed. The suite never skips.
test-web:
	cd apps/web && npm ci && npm run typecheck && npm run build && npx playwright test

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

# Rails gateway and payments orchestrator. The payments integration test needs PAYMENTS_TEST_PG (a Postgres base DSN).
test-bank:
	cd services/bank/rails-gateway && $(GO) test -count=1 ./...
	cd services/bank/payments-orchestrator && $(GO) test -count=1 ./...

# Onboarding and payments workflows and APIs against a real Temporal server. Start one first, for example:
#   temporal server start-dev --headless --port 7233
# then: TEMPORAL_HOST=127.0.0.1:7233 make test-temporal
test-temporal:
	@test -n "$$TEMPORAL_HOST" || (echo "set TEMPORAL_HOST, for example 127.0.0.1:7233"; exit 1)
	cd services/platform/onboarding && $(GO) test -tags temporaldev -count=1 -v -timeout 600s ./internal/onboarding/ ./internal/service/
	cd services/bank/payments-orchestrator && $(GO) test -tags temporaldev -count=1 -v -timeout 600s ./internal/service/

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
	cd services/bank/rails-gateway && $(GO) vet ./...
	cd services/bank/payments-orchestrator && $(GO) vet ./... && $(GO) vet -tags temporaldev ./internal/...
	cd pkg/screening && $(GO) vet ./...
	cd pkg/banklayer && $(GO) vet ./... && cd bootstrap && $(GO) vet ./...
	cd tools/sim/sponsor && $(GO) vet ./...
	cd tools/banklayercheck && $(GO) vet ./...
	cd services/bank/reconciliation && $(GO) vet ./...
	cd services/shared/audit-service && $(GO) vet ./...
	cd services/bank/treasury-risk && $(GO) vet ./...
	cd services/shared/compliance-engine && $(GO) vet ./...
	cd services/bank/reg-reporting && $(GO) vet ./...
	cd pkg/uuid && $(GO) vet ./...
	cd pkg/eventbus && $(GO) vet ./...
	cd pkg/events && $(GO) vet ./...
	cd pkg/systemaccounts && $(GO) vet ./...
	cd services/bank/fx-service && $(GO) vet ./...
	cd services/bank/payouts-service && $(GO) vet ./...
	cd services/bank/core-bridge && $(GO) vet ./...
	cd services/bank/partner-bank-router && $(GO) vet ./...
	cd services/platform/consolidated-view && $(GO) vet ./...
	cd services/platform/notifications && $(GO) vet ./...
	cd services/platform/workspace-api && $(GO) vet ./...

test-isolation:
	cd tools/isolation && $(GO) run . ../..

# Reconciliation (ARCHITECTURE.md 13). The store, runner and service tests need RECON_TEST_DSN (a throwaway database).
# Packages run one at a time because they share it; in parallel they truncate each other's rows.
test-recon:
	cd services/bank/reconciliation && $(GO) test -count=1 -p 1 ./...

# Audit log (ARCHITECTURE.md 15.3, invariant 14). Database tests need AUDIT_TEST_DSN (a throwaway database). Packages
# run one at a time because they share it.
test-audit:
	cd services/shared/audit-service && $(GO) test -count=1 -p 1 ./...

# Treasury risk (ARCHITECTURE.md 6.2, 13). Database tests need TREASURY_TEST_DSN.
test-treasury:
	cd services/bank/treasury-risk && $(GO) test -count=1 -p 1 ./...

# Compliance engine (ARCHITECTURE.md 15.1). Case tests need COMPLIANCE_TEST_DSN.
test-compliance:
	cd services/shared/compliance-engine && $(GO) test -count=1 -p 1 ./...

# Transactional outbox against a real broker (ARCHITECTURE.md 18.2). Needs OUTBOX_TEST_DSN and KAFKA_TEST_BROKERS.
test-outbox:
	cd pkg/outbox && $(GO) test -count=1 -p 1 ./...

# Regulatory reporting (ARCHITECTURE.md 15.2): evidence packs and the goAML refusal. No database.
test-reg-reporting:
	cd services/bank/reg-reporting && $(GO) test -count=1 ./...

# BankLayer (ARCHITECTURE.md 5.2): the contract suite on the simulator and on the HTTP client, the config bootstrap,
# the simulator, and the route check. Only pkg/banklayer may read the licence route.
test-banklayer:
	cd pkg/banklayer && $(GO) test -count=1 ./...
	cd pkg/banklayer/bootstrap && $(GO) test -count=1 ./...
	cd tools/sim/sponsor && $(GO) test -count=1 ./...
	cd tools/banklayercheck && $(GO) test -count=1 ./... && $(GO) run . ../..

# Regenerates Go code from proto/. Requires buf and the protoc-gen-go and protoc-gen-connect-go plugins on PATH.
proto:
	cd proto && $(BUF) lint && $(BUF) generate

up:
	$(COMPOSE) up -d --wait

down:
	$(COMPOSE) down -v
