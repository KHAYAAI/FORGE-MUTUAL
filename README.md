# FORGE-MUTUAL

FORGE Mutual Bank: the bank platform behind Forge Technologies. Source of truth for scope and design:
[`docs/CONTEXT.md`](docs/CONTEXT.md) and [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md). Open questions and
items that need a human decision are in [`docs/QUESTIONS.md`](docs/QUESTIONS.md).

> **Status: concept, pre-licence.** Forge is unlicensed and has no customers. Nothing here moves real money.
> Every external integration is either a simulator or unverified (see *Blocked* below).

## Built and tested

Each service is its own Go module. `make ci` runs lint, every module's tests and the isolation check.

- **Ledger gateway** (`services/bank/ledger-gateway-bank`): the only writer to the bank ledger. Linked transfer
  chains, pending, post and void, idempotency, transactional outbox, and `ListAccounts` over the account directory.
  Passes one shared behaviour suite in memory and against Postgres. The TigerBeetle adapter is compiled and
  unit-tested but **not yet run against a live cluster**.
- **Policy service** (`services/shared/policy-service`, `policy/`): embedded OPA v1.13.0, Ed25519-signed bundles,
  every decision audited to the outbox before it is returned. The `forge.banklayer` rules gate own-licence features.
- **Party and consent services** (`services/platform/party-service`, `consent-service`): national IDs encrypted
  (AES-256-GCM) with a keyed index, founder and company separation enforced, consent checked live.
- **Onboarding** (`services/platform/onboarding`): Temporal workflow `OnboardBusiness`, tested on a real Temporal
  dev server. KYC uses a **fake provider**; the Didit adapter is a stub.
- **Rails gateway** (`services/bank/rails-gateway`) and **payments orchestrator** (`services/bank/payments-orchestrator`):
  domestic payment saga with reserve, initiate, settle or release, and repair. Rails are simulated. Payments read the
  reconciliation gate and refuse to start without it. Inbound credits are booked once, to a customer or to suspense.
- **Reconciliation** (`services/bank/reconciliation`): five checks per asset (statement arithmetic, ledger double
  entry, unclassified balances, the sponsor or partner equation, and the pool mirror). A moving ledger makes a run
  inconclusive. The gate fails closed and is tested against Postgres.
- **Audit service** (`services/shared/audit-service`): append-only hash-chained log. Triggers refuse update, delete and
  truncate. Head anchors detect a full rewrite that reseals every hash. `forge-audit verify` checks the chain.
- **Treasury risk** (`services/bank/treasury-risk`): concentration, net outflow, liquidity and suspense alerts. Each
  alert is raised once and cleared once, with one outbox event per transition. Thresholds are placeholders.
- **BankLayer** (`pkg/banklayer`): the interface, a sponsor simulator, an HTTP client, a contract suite that the
  simulator and the client both pass, config-driven bootstrap that refuses own-licence, and a CI check that only
  `pkg/banklayer` may read the licence route.

## Partly built

- **Compliance engine** (`services/shared/compliance-engine`): a sanctions loader that checks integrity and freshness
  and fails closed, four monitoring rules, and a case store with maker-checker and SLA timers. These are building
  blocks, not a running service. There is no event feed for the rules, and no API.
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

Also not yet built: moving the payments orchestrator's rails from the rails-gateway simulator onto `BankLayer`, and the
customer USD account flow. Reconciliation's in-flight terms are zero until the inbound feed is read by it.

## Blocked

- **Didit (KYC/KYB)**: the connected Didit organisation and its application mode (sandbox or live) are unconfirmed.
  No verification sessions have been created, because live sessions bill.
- **TigerBeetle cluster**: the server image could not be pulled from its blob host, so `make test-ledger-tb` has not run.
- **Kafka**: the publisher compiles; no broker has run it. Events (treasury alerts, ledger and policy events) wait in
  the outbox until a relay publishes them.
- **Sponsor, partner-bank and FIC list formats**: unverified (VERIFY-SPONSOR, VERIFY-PARTNER, VERIFY-FIC). The
  simulators define Forge's own wire formats.
- **goAML XML**: the official schema is unverified (VERIFY-GOAML). Submission is refused until it is confirmed.
- **WORM storage** for audit anchors and evidence packs (VERIFY-WORM). Local files are used for development only.
- **ADR-0020** (legal form of the bank layer) is open, so the own-licence route is not built.

## Running it

```sh
make ci               # lint, all module tests, isolation check
make test-banklayer   # BankLayer contract suite, bootstrap, simulator, route check
make test-recon       # reconciliation (serial: packages share a database)
make test-audit       # audit log, anchors and service
make test-treasury    # treasury alerts
make test-compliance  # sanctions, monitoring and cases
make test-policy      # policy service and Rego tests
make test-platform    # party, consent and onboarding
make test-bank        # rails gateway and payments orchestrator
make up               # local stack (Postgres, TigerBeetle, Kafka, Valkey, Keycloak, Temporal, OpenBao, OTel)
```

**Database-backed tests skip when their variable is unset**, so a green `make ci` without them proves less. Set these
to throwaway databases to run every suite: `LEDGER_TEST_DSN`, `RECON_TEST_DSN`, `PAYMENTS_TEST_PG`,
`INBOUND_TEST_DSN`, `AUDIT_TEST_DSN`, `TREASURY_TEST_DSN`, `COMPLIANCE_TEST_DSN`. Each Makefile target names what it
needs. The sponsor simulator (`tools/sim/sponsor`) refuses to start unless `SPONSOR_SIMULATION=1` is set.

## Conventions

- Money is integer minor units in 64-bit integers. ZAR and USD use scale 2. Ratios are basis points.
- Only `pkg/banklayer` may read the licence route. Behaviour that depends on it is data for OPA.
- Every money-moving command carries an idempotency key. Replays return the original result.
- Money-path changes need two human reviews (`docs/ARCHITECTURE.md` section 23.4).
- Anything marked VERIFY in the docs is unconfirmed and must be checked by a human before it is relied on.
