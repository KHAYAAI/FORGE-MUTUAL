# FORGE-MUTUAL

FORGE Mutual Bank: the bank platform behind Forge Technologies. Source of truth for scope and design:
[`docs/CONTEXT.md`](docs/CONTEXT.md) and [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md). Open questions and
items that need a human decision are in [`docs/QUESTIONS.md`](docs/QUESTIONS.md).

> **Status: concept, pre-licence.** Forge is unlicensed and has no customers. Nothing here moves real money.
> Every external integration is either a simulator or unverified (see *Blocked* below).

## Built and tested

Each service is its own Go module; `make ci` runs lint, every module's tests, and the isolation check.

- **Ledger gateway** (`services/bank/ledger-gateway-bank`): the only writer to the bank ledger. Linked transfer
  chains, pending, post and void, idempotency, transactional outbox. In-memory and Postgres stores pass one shared
  behaviour suite. TigerBeetle adapter compiled and unit-tested; **not yet run against a live cluster**.
- **Policy service** (`services/shared/policy-service`, `policy/`): embedded OPA v1.13.0, Ed25519-signed bundles,
  every decision audited to the outbox before it is returned.
- **Party and consent services** (`services/platform/party-service`, `consent-service`): national IDs encrypted
  (AES-256-GCM) with a keyed index, founder and company separation enforced, consent checked live.
- **Onboarding** (`services/platform/onboarding`): Temporal workflow `OnboardBusiness`, tested on a real Temporal
  dev server. KYC uses a **fake provider**; the Didit adapter is a stub.
- **Rails gateway** (`services/bank/rails-gateway`) and **payments orchestrator** (`services/bank/payments-orchestrator`):
  domestic payment saga with reserve, initiate, settle or release, and repair. Rails are simulated; the real
  scenarios are gated behind an explicit switch.
- **Reconciliation** (`services/bank/reconciliation`): sponsor and partner reconciliation with five checks, a
  stability guard that makes a moving ledger inconclusive, and a fail-closed gate that payments now read instead of
  a static flag. Runs and breaks are committed in one transaction. Tested against Postgres; the bank side uses the
  simulator.
- **Ledger listing**: `ListAccounts` pages the account directory for reconciliation.
- **BankLayer** (`pkg/banklayer`, P0-03B): the interface, the sponsor simulator, an HTTP client for the sponsor
  boundary, a contract suite that the simulator and the HTTP client both pass, config-driven bootstrap that refuses
  own-licence and refuses the simulator in production, a `forge.banklayer` Rego package, and a CI check that only
  `pkg/banklayer` may read the licence route. `make test-banklayer`.

## Not started (v1 scope)

Thirteen services in [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) section 6 are not started, including:

- **core-bridge**: projects ledger events into Fineract; nightly GL reconciliation (the Fineract leg of reconciliation waits on it)
- **treasury-risk**: concentration and outflow alerts
- **reg-reporting** (reporting): goAML reports and evidence packs
- **compliance-engine** (compliance): sanctions, monitoring rules, alerts and cases
- **audit-service** (audit): append-only, hash-chained audit log
- **fx-service** (FX): quotes, spreads, quote lock, FX booking legs
- **payouts-service** (payouts): bulk payouts as child workflows
- **partner-bank-router**: USD account through one partner bank
- **workspace-api** (workspace): backend-for-frontend for the customer workspace
- **notifications**: email, SMS and push
- **consolidated-view**: read models for balances and statements
- **web**: customer workspace front end
- **backoffice**: operations front end (KYC review, cases, payment repair, reconciliation dashboard, maker-checker)

Also not yet built: inbound credit matching (suspense account), cross-border payments, sanctions list loading, and
moving the payments orchestrator's rails from the rails-gateway simulator onto `BankLayer`.

## Blocked

- **Didit (KYC/KYB)**: the connected Didit organisation and its application mode (sandbox or live) are unconfirmed.
  No verification sessions have been created, because live sessions bill.
- **TigerBeetle cluster**: the server image could not be pulled from its blob host, so `make test-ledger-tb` has
  not run.
- **Kafka**: the publisher compiles; no broker has run it.
- **Sponsor and partner-bank APIs**: unknown and unverified (VERIFY-SPONSOR, VERIFY-PARTNER). The simulators define
  Forge's own wire format.
- **goAML XML**: the official schema is unverified (VERIFY-GOAML). Submission is refused until it is confirmed.
- **ADR-0020** (legal form of the bank layer) is open, so the own-licence route is not built.

## Running it

```sh
make ci               # lint, all module tests, isolation check
make test-banklayer   # BankLayer contract suite, bootstrap, simulator, route check
make test-policy      # policy service and Rego tests
make test-platform    # party, consent and onboarding
make test-bank        # rails gateway and payments orchestrator
make up               # local stack (Postgres, TigerBeetle, Kafka, Valkey, Keycloak, Temporal, OpenBao, OTel)
```

Database and cluster tests need their environment variables; each `Makefile` target names the variable it needs.
The simulator (`tools/sim/sponsor`) refuses to start unless `SPONSOR_SIMULATION=1` is set.

## Conventions

- Money is integer minor units in 64-bit integers. ZAR and USD use scale 2.
- Only `pkg/banklayer` may read the licence route; behaviour that depends on it is data for OPA.
- Money-path changes need two human reviews (`docs/ARCHITECTURE.md` section 23.4).
- Anything marked VERIFY in the docs is unconfirmed and must be checked by a human before it is relied on.
