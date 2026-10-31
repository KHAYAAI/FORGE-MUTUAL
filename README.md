# FORGE-MUTUAL

FORGE Mutual Bank: the bank platform behind Forge Technologies. Source of truth for scope and design:
[`docs/CONTEXT.md`](docs/CONTEXT.md) and [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md). Open questions and
items that need a human decision are in [`docs/QUESTIONS.md`](docs/QUESTIONS.md).

> **Status: concept, pre-licence.** Forge is unlicensed and has no customers. Nothing here moves real money.
> Every external integration is either a simulator or unverified (see *Blocked* below).

## Built and tested

Each service is its own Go module. `make ci-db` creates throwaway databases on the compose Postgres 17, then runs the
full gate: lint, every module's tests against real Postgres and a real Kafka broker, and the isolation check. It passes
with 49 packages and no skipped tests.

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
  server. KYC uses a **fake provider**; the Didit adapter is a stub (see *Blocked*).
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

## Partly built

- **Compliance engine** (`services/shared/compliance-engine`): a sanctions loader that checks integrity and freshness
  and fails closed, four monitoring rules, and a case store with maker-checker and SLA timers. These are building
  blocks, not a running service. There is no event feed for the rules, and no API. The list source is a recommendation
  (`docs/SANCTIONS_SOURCES.md`), not a decision.
- **Reg-reporting** (`services/bank/reg-reporting`): immutable evidence packs with stable digests and maker-checker.
  goAML submission is **deliberately refused** until its schema is verified (VERIFY-GOAML). No report builders yet.

## Not started (v1 scope)

Nine services in [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) section 6 are not started:

- **core-bridge**: projects ledger events into Fineract and runs nightly GL reconciliation. Waits on Fineract's deployment.
- **partner-bank-router**: routes foreign-currency operations to partner banks. The USD partner is simulated.
- **fx-service** (FX): quotes, disclosed spreads, quote lock, FX booking legs.
- **payouts-service** (payouts): bulk payouts as child workflows.
- **consolidated-view**: read models for balances and statements.
- **notifications**: email, SMS and push.
- **workspace-api** (workspace): backend-for-frontend for the customer workspace.
- **web**: customer workspace front end.
- **backoffice**: operations front end (KYC review, cases, payment repair, reconciliation dashboard, maker-checker).

Also not yet built: moving the payments orchestrator's rails from the rails-gateway simulator onto `BankLayer`, the customer
USD account flow, and the Didit adapter beyond its stub.

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
  tests. Not verified: a multi-broker cluster, TLS and SASL, topic creation policy in production, and the Kafka wiring of
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
make up               # local stack: Postgres 17, Kafka 3.9.0, and the others in deploy/compose
make ci-db            # the full gate, with throwaway databases on the compose Postgres and the broker from make up
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
