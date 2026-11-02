# Local stack

The stack runs the Postgres, Kafka and Temporal infrastructure and every Go service that the customer and back-office
journeys use, with each service's real code. It is for development and for the journey tests. It is not production:
the differences are listed below, and the production refusals in `pkg/serve` and `pkg/callerauth` enforce them.

## Start, check, stop

```bash
deploy/compose/up.sh            # generates .env (secrets, never committed), builds, starts, waits for every port
deploy/compose/up.sh oidc       # also starts the development identity provider (tools/devidp) on :3300
make down                       # stops everything and deletes the volumes (the data is gone)
```

`up.sh` probes each service's port and fails with the names of any that do not answer. Logs:
`docker compose -f deploy/compose/docker-compose.yml logs <service>`.

## Requirements

- Docker with host networking on Linux. Docker Desktop (macOS, Windows) does not provide host networking the same way:
  the stack is written for host networking because the bridge network is unavailable on the machines it was built on.
- About 6 GB of memory and 4 CPUs for the first build; later starts are seconds.
- A certificate authority for intercepted HTTPS, if your network has one: place it in `deploy/docker/ca/*.crt` (see
  `deploy/docker/ca/README.md`).

## Ports (loopback only)

| Port | What | Port | What |
| ---- | ---- | ---- | ---- |
| 5433 | Postgres 17 (5432 is often a local server) | 8096 | consolidated-view |
| 9192 | Kafka 3.9 (9092 is often a local broker) | 8097 | notifications |
| 7233 | Temporal (auto-setup 1.26.2) | 8099 | partner-bank-router |
| 8070 | ledger-gateway-bank (8080 is often Fineract) | 8100 | fx-service |
| 8081 | party-service | 8200 | payouts-service |
| 8082 | consent-service | 8093 | reconciliation |
| 8083 | onboarding | 8095 | treasury-risk |
| 8084 | policy-service | 8094 | audit-service |
| 8085 | payments-orchestrator | 8400 | compliance-engine (cases) |
| 8086 | rails-gateway | 8500 | backoffice-api (staff) |
| 8300 | workspace-api (customer) | 3300 | devidp (profile `oidc` only) |

Every service's address names its own port, so the services reach each other over loopback without a DNS name.

## Configuration

- `docker-compose.yml`: one service per block. Settings the code requires are set here, and nothing else.
- `config/callers/<service>.json`: which callers may call each service, and which methods. Dev-only identities:
  production uses the mutual-TLS identity of the calling service instead (`pkg/serve`).
- `config/kafka-topics.txt`: every topic the services use. The broker does not create topics on its own.
- `config/banklayer.yaml`: the bank layer, using the simulator (see below).
- `config/screening-list.txt`: a local fixture with one invented name. It is not a sanctions list.
- `initdb/01-databases.sql`: one database per service that keeps state. Each service migrates its own schema.

## What is real, and what is not

Real: Postgres, Kafka and Temporal; every service's own code; inter-service ConnectRPC calls with caller allow-lists; the
event bus and the transactional outbox; the Temporal workflows; the KYC review path; the compliance rules.

Not real, on purpose:

- **TigerBeetle.** The ledger runs `LEDGER_STORE=memory`. Balances are lost when it restarts, and nothing here proves
  TigerBeetle's behaviour. Production refuses the memory store (`RefuseInProduction`).
- **The bank.** `PAYMENTS_ALLOW_SIMULATION` and `RAILS_SIMULATION` accept the simulator, and the bank layer is the
  simulator (`config/banklayer.yaml`). There is no sponsor sandbox. Production refuses the simulator.
- **Identity.** The customer and staff apps sign in with the development sign-in (`WORKSPACE_AUTH=dev`,
  `BACKOFFICE_AUTH=dev`). The OIDC path is exercised by tests against `tools/devidp`.
- **Service identity.** Development uses the `X-Forge-Caller` header. Production uses mutual TLS, and refuses to start
  without certificates.
- **KYC.** The fake provider, with a local webhook secret. Didit is not configured in this build.
- **Policy.** The bundle loads unsigned (`POLICY_DEV_UNSIGNED`). Production loads only a signed bundle.
- **Core banking.** `core-bridge` is not started: it needs a live Apache Fineract.
- **Notifications.** Messages go to a sink file, not to a provider.
- **Screening.** A fixture list, refreshed by `up.sh` each start. The real list depends on the sanctions licence.

## Backups

`scripts/pg-backup.sh` and `scripts/pg-restore-check.sh` back up and verify a database on this stack. Use
`PG_TOOLS_IMAGE=postgres:17` when the local client is a different major version, which is the normal case on a
machine with an older `pg_dump`.

## Journeys against the running stack

```bash
deploy/compose/up.sh && make test-stack
```

`make test-stack` runs two suites against the running services, with nothing mocked:

- **Customer** (`apps/web/tests/stack`): a company is onboarded through the real onboarding workflow; the provider
  approves its subjects; the account it opens is visible to the company in the workspace, and not to the director.
- **Back office** (`apps/backoffice/tests/stack`): the provider's REVIEW verdict puts a review in the compliance
  queue; a compliance person decides it in the app; the onboarding workflow resumes and completes.

`deploy/compose/journeys/seed.cjs` does the seeding through the same APIs the apps use. It signs each provider webhook
with the stack's secret from `.env`, as the provider would. Each journey creates its own company with a new,
valid identity number, so the suites can run again without resetting the stack.

Two settings differ from production on purpose: the consolidated view reconciles every ten seconds
(`CONSOLIDATED_RECON_INTERVAL`, 15 minutes in production), so a new account appears in the workspace without a wait; and
the ledger is in memory, so a restart of the ledger removes the accounts the journeys created.
