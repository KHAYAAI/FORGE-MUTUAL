# Local stack

The stack runs the Postgres, Kafka, Temporal and TigerBeetle infrastructure and every Go service that the customer and
back-office journeys use, with each service's real code. It is for development and for the journey tests. It is not production:
the differences are listed below, and the production refusals in `pkg/serve` and `pkg/callerauth` enforce them.

## Start, check, stop

```bash
deploy/compose/up.sh            # generates .env (secrets, never committed), builds, starts, waits for every port
deploy/compose/up.sh oidc       # also starts the development identity provider (tools/devidp) on :3300
deploy/compose/up.sh mtls       # every service speaks TLS and requires a client certificate (see "Mutual TLS")
make down                       # stops everything and deletes the volumes (the data is gone)
```

`up.sh` probes each service's port and fails with the names of any that do not answer. Logs:
`docker compose -f deploy/compose/docker-compose.yml logs <service>`.

## Requirements

- Docker with host networking on Linux. Docker Desktop (macOS, Windows) does not provide host networking the same way:
  the stack is written for host networking because the bridge network is unavailable on the machines it was built on.
- About 8 GB of memory and 4 CPUs for the first build; later starts are seconds. TigerBeetle takes about 1.7 GB of that
  and a 1.1 GB data file.
- The TigerBeetle image, `ghcr.io/tigerbeetle/tigerbeetle:0.16.60` (`TB_IMAGE` overrides it). Its layers come from
  `pkg-containers.githubusercontent.com`, which a restricted network must allow. Where it cannot be pulled, put
  `LEDGER_STORE=memory` in `.env`: the ledger then keeps balances in memory and loses them when it restarts.
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
| 3034 | TigerBeetle 0.16.60 (3033 is the test replica) | | |

Every service's address names its own port, so the services reach each other over loopback without a DNS name.

## Configuration

- `docker-compose.yml`: one service per block. Settings the code requires are set here, and nothing else.
- `config/callers/<service>.json`: which callers may call each service, and which methods. Each list is least
  privilege: a caller is granted a method only if its code calls it, and `make ci` fails otherwise
  (`tools/isolation/callers.go`). An empty file denies everyone. Without mutual TLS the caller names itself in a
  header, which is development only; with it, the caller is the service its certificate names.
- `config/kafka-topics.txt`: every topic the services use. The broker does not create topics on its own.
- `config/banklayer.yaml`: the bank layer, using the simulator (see below).
- `config/screening-list.txt`: a local fixture with one invented name. It is not a sanctions list.
- `initdb/01-databases.sql`: one database per service that keeps state. Each service migrates its own schema.

## Mutual TLS

```bash
deploy/compose/up.sh mtls && make test-stack-mtls
```

`up.sh mtls` runs `pki.sh`, which makes a private certificate authority and one certificate per service in `.pki/`
(never committed), and starts every service with `TLS_CERT_FILE`, `TLS_KEY_FILE` and `TLS_CA_FILE`. From then on:

- Every service-to-service call is TLS, and both sides present a certificate. A caller with no certificate, or one
  from another authority, gets no answer at all.
- The caller is the service its certificate names (`spiffe://forge/svc/<name>`). The development caller header is
  ignored.
- The two APIs for people serve only their own web app's server: `WORKSPACE_CLIENTS=svc-web` and
  `BACKOFFICE_CLIENTS=svc-backoffice-web` (plus the journey tests here). Production refuses to start without these.
- Each web app's server presents its own certificate to its API (`apps/*/lib/upstream.ts`).
- A certificate that is renewed on disk is picked up within 15 seconds, without a restart. Running `pki.sh` again
  while the stack is up renews every certificate, which is how that was checked.

`check-mtls.sh` states each of these as a claim and tests it against the running stack. `make test-stack-mtls` runs it
and then the journeys.

What this mode does not make real: the certificates are valid for a year and their keys are world-readable files, for
a stack on one machine. Production certificates come from the platform's PKI and are short-lived. Health checks are
served on the same listener, so they need a certificate too: a separate health listener for an orchestrator's probes
is not built. Postgres, Kafka, Temporal and TigerBeetle are still reached without TLS here.

## What is real, and what is not

Real: Postgres, Kafka, Temporal and TigerBeetle; every service's own code; inter-service ConnectRPC calls with caller
allow-lists; the event bus and the transactional outbox; the Temporal workflows; the KYC review path; the compliance
rules. Balances live in TigerBeetle: a transfer made on the stack was still there after the ledger service was
restarted, and after TigerBeetle itself was (checked 2026-11-16).

Not real, on purpose:

- **TigerBeetle is one replica in development mode.** `--development` relaxes the direct-I/O requirement and shrinks the
  caches. One replica has no fault tolerance. Production is six replicas on dedicated hosts, without that flag
  (`services/bank/ledger-gateway-bank/SPEC.md` section 12). The ledger's Postgres database and TigerBeetle's data file
  belong together: `make down` removes both, and removing only one leaves a directory that names accounts the ledger
  does not hold. Two containers run with `seccomp=unconfined`, TigerBeetle and the ledger gateway, because both use
  io_uring, which Docker's default profile blocks; without it the gateway does not report an error, it aborts.
- **The bank.** `PAYMENTS_ALLOW_SIMULATION` and `RAILS_SIMULATION` accept the simulator, and the bank layer is the
  simulator (`config/banklayer.yaml`). There is no sponsor sandbox. Production refuses the simulator.
- **Identity.** The customer and staff apps sign in with the development sign-in (`WORKSPACE_AUTH=dev`,
  `BACKOFFICE_AUTH=dev`). The OIDC path is exercised by tests against `tools/devidp`.
- **Service identity.** By default the stack uses the `X-Forge-Caller` header. `up.sh mtls` turns on mutual TLS, as
  production requires (see "Mutual TLS"). Production refuses to start without certificates.
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

One setting differs from production on purpose: the consolidated view reconciles every ten seconds
(`CONSOLIDATED_RECON_INTERVAL`, 15 minutes in production), so a new account appears in the workspace without a wait.
