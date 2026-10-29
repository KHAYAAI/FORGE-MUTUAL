# Repository entry points. `make ci` is the arbiter (ARCHITECTURE.md section 0, rule 5).
GO ?= go
COMPOSE ?= docker compose -f deploy/compose/docker-compose.yml

.PHONY: ci test lint vet fmt-check test-isolation up down

ci: lint test test-isolation

test:
	cd pkg/money && $(GO) test ./...
	cd tools/isolation && $(GO) test ./...

lint: fmt-check vet

fmt-check:
	@out=$$(gofmt -l pkg tools services); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	cd pkg/money && $(GO) vet ./...
	cd tools/isolation && $(GO) vet ./...

test-isolation:
	cd tools/isolation && $(GO) run . ../..

up:
	$(COMPOSE) up -d --wait

down:
	$(COMPOSE) down -v
