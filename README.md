# FORGE-MUTUAL

FORGE Mutual Bank: the bank platform behind Forge Technologies. Source of truth for scope and design:
[`docs/CONTEXT.md`](docs/CONTEXT.md) and [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md). Open questions and
items that need a human decision are in [`docs/QUESTIONS.md`](docs/QUESTIONS.md).

> **Status: concept, pre-licence.** Forge is unlicensed and has no customers. Nothing here moves real money.
> Every external integration is either a simulator or unverified (see *Blocked* below).

## Built and tested

Each service is its own Go module. `make ci-db` creates throwaway databases on the compose Postgres 17, then runs the
full gate: lint, every module's tests against real Postgres and a real Kafka broker, and the isolation check. Every
database-backed suite runs; the only opt-in suite is the live Fineract test, which needs `FINERACT_TEST_URL`.

- **Ledger gateway** (`services/bank/ledger-gateway-bank`): the only writer to the bank ledger. Linked transfer chains,
  pending, post and void, idempotency, transactional outbox, and `ListAccounts` over the account directory. Passes one
  shared behaviour suite in memory and against Postgres. The TigerBeetle adapter is compiled and unit-tested, but
  **the cluster has never run** (see *Blocked*).
- **Policy service** (`services/shared/policy-service`, `policy/`): embedded OPA v1.13.0, Ed25519-signed bundles, every
  decision audited to the outbox before it is returned. The break rule is strict: while any reconciliation break is
  open, every payment is held (`docs/DECISIONS.md`).
- **Party and consent services** (`services/platform/party-service`, `consent-service`): national IDs encrypted
  (AES-256-GCM) with a keyed index, founder and company separation enforced, consent checked live.
- **Onboarding** (`services/platform/onboarding`): Temporal workflow `OnboardBusiness`, tested on a real Temporal dev
  server and run on the local stack. A provider's REVIEW verdict parks the workflow until a compliance person decides
  it; the decision is recorded once and the workflow resumes. KYC uses a **fake provider**; the Didit adapter is a stub
  (see *Blocked*).
- **Rails gateway** (`services/bank/rails-gateway`) and **payments orchestrator** (`services/bank/payments-orchestrator`):
  domestic payment saga with reserve, initiate, settle or release, and repair. Settlement posts the hold, then records
  the sponsor's debit as a clearing leg into the pool. Each rail status is recorded when it is seen. Inbound credits
  are booked once, to a customer or to suspense. Rails are simulated.
- **Reconciliation** (`services/bank/reconciliation`): five checks per asset. A moving ledger makes a run inconclusive.
  The in-flight terms come from payments: inbound credits the bank has reported but the books do not yet hold, and
  outbound payments the bank has settled but the books have not yet cleared. The gate fails closed and is tested
  against Postgres.
- **Audit service** (`services/shared/audit-service`): append-only hash-chained log. Triggers refuse update, delete and
  truncate. Head anchors detect a full rewrite that reseals every hash. `forge-audit verify` checks the chain.
- **Treasury risk** (`services/bank/treasury-risk`): concentration, net outflow, liquidity and suspense alerts. Each
  alert is raised once and cleared once, with one outbox event per transition. Thresholds are placeholders.
- **Outbox and Kafka** (`pkg/outbox`): the relay publishes to a real broker. Tests show that events arrive in order, and
  that during a broker outage every event stays unpublished for the next pass.
- **BankLayer** (`pkg/banklayer`): the interface, a sponsor simulator, an HTTP client, a contract suite that the
  simulator and the client both pass, config-driven bootstrap that refuses own-licence, and a CI check that only
  `pkg/banklayer` may read the licence route.
- **Core bridge** (`services/bank/core-bridge`): mirrors committed ledger transfers into Apache Fineract. Before a retry
  it looks up the reference, so a lost response cannot post the same journal twice. Reconciles 14 general-ledger
  accounts (seven codes in ZAR and USD) against the ledger, and refuses to run while a posting is pending. The suite
  runs against a stateful Fineract fake. The posting path was also checked once against a live Fineract.
- **Partner-bank router** (`services/bank/partner-bank-router`): routes USD operations by partner health and cost. A
  payout is pinned to the partner it was first sent to, so a retry never reaches a second partner. Partners are
  simulated.
- **Consolidated view** (`services/platform/consolidated-view`): balances and activity per party, projected idempotently
  from transfers and reconciled to the ledger. Drift is counted, not hidden. A party sees only its own accounts.
- **Notifications** (`services/platform/notifications`): email and SMS from platform events. Contacts are sealed with
  AES-256-GCM. Failed sends are retried, then marked failed. Only a development file sink exists; no provider is chosen
  (VERIFY).
- **FX service** (`services/bank/fx-service`): quotes with a disclosed spread, rounding against the customer, a 60-second
  single-use quote lock, and a saga across two ledgers with holds, reversal and a resume loop. One worker runs a deal at
  a time, enforced by an advisory lock. The rate is static configuration (VERIFY).
- **Payouts** (`services/bank/payouts-service`): bulk ZAR payouts. A checker other than the maker approves the batch. Each
  item is sent to payments under a key derived from the batch and the item, so a retry or a resume cannot pay it twice. A
  refused item is final; an unreachable one is marked uncertain and sent again on resume. Tested against Postgres and in
  Temporal's test environment. **Not yet run against a Temporal server** (see its SPEC).
- **Workspace API** (`services/platform/workspace-api`): the customer REST surface, with the OpenAPI contract in
  `openapi/workspace.yaml`. The party comes from the session and request bodies are strict. Only development sign-in
  exists. Its downstream calls are tested with fakes, and it runs against the real services on the local stack.
- **Back office** (`services/shared/backoffice-api`, `apps/backoffice`): staff tools for compliance cases (close as a second
  person), payment approval and repair, and the reconciliation gate and breaks. Each action checks the role before any
  downstream call. The cases service is part of compliance-engine and is tested against Postgres. Development sign-in only.
  The app has a KYC review screen, which decides real reviews on the local stack. Sixteen Playwright journeys pass
  against the production build, and two more run against the local stack with nothing mocked.
- **Customer web app** (`apps/web`): accounts with entity and insurance on every balance, activity, payments, FX with a
  disclosure that must be accepted, and bulk payouts with maker-checker. Fourteen Playwright journeys pass against the
  production build, with the API mocked in the browser. One more runs against the local stack, where an account opened
  by a real onboarding is visible to its owner. Development sign-in only. See its SPEC.

## Partly built

- **Compliance engine** (`services/shared/compliance-engine`): a sanctions loader that checks integrity and freshness
  and fails closed, monitoring rules over the event bus that open cases, and a case store with maker-checker and SLA
  timers, served to the back office. Cases open from settled payments and customer credits on the local stack. The
  rules' thresholds are placeholders (`docs/THRESHOLDS.md`). The list source is a recommendation
  (`docs/SANCTIONS_SOURCES.md`), not a decision; the local stack uses a one-name fixture.
- **Reg-reporting** (`services/bank/reg-reporting`): immutable evidence packs with stable digests and maker-checker.
  goAML submission is **deliberately refused** until its schema is verified (VERIFY-GOAML). No report builders yet.

## Not started (v1 scope)

Every v1 service in [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) section 6 now exists, and the back office has its
KYC review screen and live cases. Not yet built: moving the payments orchestrator's rails from the rails-gateway
simulator onto `BankLayer`, the customer USD account flow, the Didit adapter beyond its stub, and the core-bridge on the
local stack (it needs a live Apache Fineract there).

## Production configuration

Done and tested: TLS and mutual TLS between services (`pkg/serve`: production refuses to start without certificates,
and the caller is the service named in its certificate, not a header); request IDs, timeouts and security headers on
every service and both web apps (CSP keeps `'unsafe-inline'` for scripts, a known gap until there is a nonce); a
production refusal of every development switch (memory ledger, simulators, unsigned policy, fake KYC); OpenTelemetry
spans, with OTLP export when `OTEL_EXPORTER_OTLP_ENDPOINT` is set; a backup and restore check that proves a dump
restores (`scripts/pg-backup.sh`, `scripts/pg-restore-check.sh`); and an axe accessibility scan of every page in both
apps, failing on serious and critical violations.

Not done: the OTLP export has not been run against a collector; the local stack runs in development mode, so mutual TLS
is covered by `pkg/serve`'s tests, not by the stack; backups are not scheduled and there is no point-in-time recovery;
secrets are environment variables, with no secret store integrated.

## Blocked

- **Didit (KYC/KYB)**: the decision is a new organisation, not A-CARD. That organisation does not exist yet, and no tool
  in this environment can create one. The owner must create it and grant access. No verification session has been created,
  and none is approved.
- **TigerBeetle cluster: has never run.** Tried and failed, all from this environment: the `ghcr.io` image's blob host is
  forbidden; `quay.io` and Docker Hub have no published image; GitHub release downloads return 403; the Zig download host
  is blocked, so the server cannot be built; the Go module proxy has the module path but no tagged versions. Without the
  server, `make test-ledger-tb` cannot run. Obtain the server binary by another route, or run the test on a host that
  can reach the release artefacts.
- **Kafka: runs in the dev stack, but only single-node.** `apache/kafka:3.9.0` in KRaft mode is verified by the outbox
  tests and by the local stack (on port 9192, so it does not clash with a broker on 9092). Not verified: a multi-broker cluster, TLS and SASL, topic creation policy in production, and the Kafka wiring of
  the other services' relays.
- **Sponsor, partner-bank and FIC list formats**: unverified (VERIFY-SPONSOR, VERIFY-PARTNER, VERIFY-FIC). The simulators
  define Forge's own wire formats. The government and UN sites were not reachable from the build environment, so the
  sanctions facts in `docs/SANCTIONS_SOURCES.md` are secondary and need checking.
- **goAML XML**: the official schema is unverified (VERIFY-GOAML). Submission is refused until it is confirmed.
- **WORM storage** for audit anchors and evidence packs (VERIFY-WORM). Local files are used for development only.
- **Threshold owners**: every placeholder needs a named owner (`docs/THRESHOLDS.md`). None is named yet.
- **Sanctions list source and licence**: a recommendation exists; the decision and the licence do not.
- **ADR-0020** (legal form of the bank layer) is open, so the own-licence route is not built.
- **Reconciliation's window**: between the bank debiting a payment and payments seeing it, the run breaks and the gate
  closes until a later clean run. That is fail-closed by design. It is documented in the reconciliation spec.

## Running it

```sh
make up               # the local stack: builds every service and starts it (deploy/compose/README.md)
make test-stack       # the customer and back-office journeys against the running stack, with nothing mocked
make ci-db            # the full gate, with throwaway databases (defaults: Postgres on 5432, broker on 9092; for the
                      # stack's own, set FORGE_PG_HOST_DSN=postgres://postgres:forge-local-only@127.0.0.1:5433 and
                      # FORGE_KAFKA_BROKERS=127.0.0.1:9192)
make ci               # the same gate, if you have set every database and broker variable yourself
make test-outbox      # the outbox against a real broker
make test-recon       # reconciliation (serial: packages share a database)
make test-banklayer   # BankLayer contract suite, bootstrap, simulator, route check
make test-audit       # audit log, anchors and service
make test-treasury    # treasury alerts
make test-compliance  # sanctions, monitoring and cases
make test-policy      # policy service and Rego tests
make test-platform    # party, consent and onboarding
make test-bank        # rails gateway and payments orchestrator
```

**`make ci` refuses to pass unless the database and broker suites are configured.** Set them with `make ci-db`, or set
every variable in `DB_TEST_VARS` (see the Makefile). The only opt-out is `FORGE_SKIP_DB_TESTS=1`, which prints a warning
and proves less. Each suite still reads its own variable, and a test that cannot reach its database fails rather than
skips.

The sponsor simulator (`tools/sim/sponsor`) refuses to start unless `SPONSOR_SIMULATION=1` is set.

## Decisions and open items

See [`docs/DECISIONS.md`](docs/DECISIONS.md) for every owner decision, what it changed, and what is still open. The
threshold register is [`docs/THRESHOLDS.md`](docs/THRESHOLDS.md). The sanctions recommendation is
[`docs/SANCTIONS_SOURCES.md`](docs/SANCTIONS_SOURCES.md).

## Conventions

- Money is integer minor units in 64-bit integers. ZAR and USD use scale 2. Ratios are basis points.
- Only `pkg/banklayer` may read the licence route. Behaviour that depends on it is data for OPA.
- Every money-moving command carries an idempotency key. Replays return the original result.
- Money-path changes need two human reviews (`docs/ARCHITECTURE.md` section 23.4).
- Anything marked VERIFY in the docs is unconfirmed and must be checked by a human before it is relied on.
