# FORGE-MUTUAL

FORGE Mutual Bank: the bank platform behind Forge Technologies. Source of truth for scope and design:
[`docs/CONTEXT.md`](docs/CONTEXT.md) and [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md). Open questions and
items that need a human decision are in [`docs/QUESTIONS.md`](docs/QUESTIONS.md).

> **Status: concept, pre-licence.** Forge is unlicensed and has no customers. Nothing here moves real money.
> Every external integration is either a simulator or unverified (see *Blocked* below).

## Launch readiness

**Not ready to take real customers.** The software is built, tested and runs end to end on a local stack, including
the customer front door: a person or a company signs in, applies, verifies and gets an account. What stands between
this and a launch is mostly not code: a banking licence or sponsor bank, accounts with live providers, owner decisions,
production infrastructure and an independent security review. Assessed 2026-11-23.

**Scorecard.** Regulatory register (`docs/regulatory/register.csv`, 114 requirements): 100 Partial, 10 Open, 4 waiting
on an external party, none fully met. The 10 open rows all wait on a person, capital or a hosting choice. Governance pack (`docs/governance/`): 55 draft policies, none approved;
37 seats, 1 named (founder Tebogo Mvelase), 36 vacant. Thresholds: 31 placeholders, none with a named owner. Legal form: mutual bank planned, bank kept open (`docs/governance/LEGAL_FORMS.md`); both built into the code. Licence: none.
Production infrastructure: none. Software gate (`make ci-db`): passing.

### Hosting

Fly.io for the synthetic-data preview, AWS (Cape Town proposed) for production: [`docs/HOSTING.md`](docs/HOSTING.md).
The AWS landing zone is written as Terraform and checked on paper, never applied: [`deploy/aws`](deploy/aws/README.md).

### The model

How the bank is shaped by Erebor (technology) and Investec (service), and where it differs: [`docs/BANK_MODEL.md`](docs/BANK_MODEL.md).


### Readiness by stage

| Stage | Where we are | What is missing |
|---|---|---|
| **Conversations** (sponsor banks, a regulatory lawyer, the Prudential Authority's pre-application meeting, investors, first hires) | **Ready to start.** A working platform, a governance pack (structure, 37 seats, 55 draft policies), a 114-line regulatory register, a bank-model note, a legal-forms comparison (mutual planned, bank open), a sponsor pitch, a lawyer brief and a risk appetite worksheet. | The founder's own fit-and-proper file; real financial projections (the pro formas use illustrative figures); a named lawyer and a sponsor shortlist; the owner's answers to the risk appetite worksheet. |
| **Licence application** (Prudential Authority registration as a mutual bank or a bank) | **Not ready.** Policies and structure are drafted, but nothing is approved. For the mutual route the official forms are now in hand (27 DI forms, from the PA): DI 100, DI 200, DI 110, DI 300 (liquidity ladder), DI 310 (liquid assets), DI 400 (capital), DI 410 (interest-rate risk), DI 500 (credit risk), DI 505 and DI 510 (large exposures), DI 600 (currency risk), DI 700 (investment limits) and DI 930 (interest rates) are built on the forms' own line numbers, the DI 099 control sheet is built, 9 forms are nil returns and 3 do not apply; 1 is still to build (the DI 900 statistical breakdown) (`docs/regulatory/returns/catalogue.csv`). The line transcription still needs a second person's check. | Capital (R10m mutual, R250m bank); at least 7 founding members for a mutual; a board with independent directors; fit-and-proper filings for every director and executive; approved policies; a business plan with real projections; the lawyer's advice on the legal form. Typically 18 to 24 months or more from a complete application (general knowledge; confirm with the lawyer). |
| **Live launch** (real customers, through a sponsor bank first) | **Not ready.** The software passes its gate on a local stack, with every external party simulated. | A signed sponsor bank; the MLRO, risk and compliance heads; approved policies; production hosting and a Prudential Authority notice 20 working days ahead; a penetration test; live WorkOS and Didit; a licensed sanctions list; lawyer-reviewed terms and privacy notice; FIC registration; a pilot. Roughly 6 to 12 months if the first steps start now (general knowledge). |

### Licence application pack

[`docs/licence-application/PACK.md`](docs/licence-application/PACK.md) is generated (`make licence-pack`) from the regulatory register: 18 sections, 59
requirements, each with its owner, evidence and gap. It is an index of what the application needs, not the application.
Two working drafts sit beside it, both for the lawyer and the founder to correct and neither approved: a
[draft constitution](docs/licence-application/CONSTITUTION_DRAFT.md) (20 decisions listed; the Act's own provisions still to be added) and a
[draft business plan](docs/licence-application/BUSINESS_PLAN_DRAFT.md) (projections, capital plan, board and auditor are gaps).

### Governance and people

The board, committees, executive chart, three lines of defence and every seat the bank needs are drawn in
[`docs/governance/STRUCTURE.md`](docs/governance/STRUCTURE.md). The roster of 37 seats with what each needs is
[`docs/governance/ROLES.md`](docs/governance/ROLES.md); everything still needed (people, decisions, money,
registrations, providers, documents) is the checklist in [`docs/governance/README.md`](docs/governance/README.md).

### What is left before real customers

Ticked items are done. Each open item says who must act: **Owner** (a decision or an account only a person can make),
**External** (a third party) or **Engineering** (work in this repository).

**1. Licence and regulators: the hard gate**

- [ ] **Owner/External.** A licence or a sponsor bank. No customer money may be held until one of the two is in place.
  ADR-0020 (legal form of the bank layer) is open.
- [ ] **External.** Sponsor bank sandbox and wire formats (VERIFY-SPONSOR), partner and FIC formats (VERIFY-PARTNER,
  VERIFY-FIC), and the goAML schema (VERIFY-GOAML; submission is refused until it is confirmed).
- [ ] **Owner/External.** Regulatory review of the policy, screening and reporting rules.
- [ ] **Owner.** Name an owner for each of the 31 placeholder thresholds (`docs/THRESHOLDS.md`), and decide ADR-0012
  (sector appetite).
- [x] Daily re-screening of every customer: a match opens a critical case and freezes the customer.
- [x] A per-person annual foreign allowance on FX (threshold 28), monthly credit statements and default notices
  (threshold 27), and a data-classification register checked by the gate.
- [ ] **Owner.** Choose and license the sanctions list source (`docs/SANCTIONS_SOURCES.md`). Screening runs on a
  one-name fixture today.

**2. Identity: sign-in and KYC/KYB**

- [x] Sign-in through OIDC, built and tested against a local identity provider.
- [x] The Didit adapter, built from Didit's documentation; its webhook signature matches Didit's reference code.
- [x] Both MCP servers are in `.mcp.json` (`workos-forge-mutual`, `didit`).
- [ ] **Owner.** Sign in to the WorkOS and Didit MCP servers with the new Forge account (not A-CARD): run `/mcp` in
  Claude Code. The Didit connection on the cloud sessions reaches only A-CARD and must not be used.
- [ ] **Engineering, once connected.** Configure WorkOS AuthKit (redirect addresses, roles, a `forge_party` claim for
  approved applicants). Build the Didit KYC and KYB workflows in its sandbox, register the webhook, and run an
  onboarding end to end against Didit with simulated outcomes. Then repeat in Didit's live application.

**3. Customer front door**

- [x] Public application endpoints (`POST /v1/applications/person|business`, `GET /v1/applications/{kind}`), for
  signed-in applicants only, rate-limited, with no decline reason shown (no tipping-off).
- [x] The `OnboardPerson` workflow: personal account, KYC, screening, account opening.
- [x] Apply, status and welcome pages, restyled; Playwright and accessibility tests.
- [x] A personal application journey on the running stack (`make test-stack`): sign in, apply, verify, approved,
  account opened in the ledger and shown to its owner; and a decline, shown without a reason. The one stand-in is the
  identity provider, whose signed verdict the test delivers until Didit is connected. It found two bugs, both fixed:
  every personal application stopped at verification, and the stand-in provider gave no verification link.
- [x] The onboarding SPEC, the workspace-api SPEC and its OpenAPI describe the application routes and `OnboardPerson`.
- [x] The signed-in customer pages share the front door's style.
- [ ] **Engineering.** Give an approved director a login, and create the Fineract client the design lists.
- [ ] **Owner.** Terms and conditions, privacy notice and fee schedule, reviewed by a lawyer, linked from the forms.
- [ ] **Owner.** Decide how soon an account reaches its owner. Today reconciliation names the owner every 15 minutes
  by default.

**3a. Products beyond v1: built, and off in production until approved**

Each is a working service on the local stack, tested against Postgres, and refused in production by `pkg/licence`
until `FORGE_APPROVAL_<PRODUCT>` names the decision that approved it (`docs/DECISIONS.md`). Their rates, fees, limits
and affordability rule are placeholders 17 to 21 in `docs/THRESHOLDS.md`.

- [x] **Currencies:** USD, EUR and GBP accounts at the partner bank, one per currency, with the reference payers
  quote; FX between the rand and each; payments received from abroad booked and recorded for SARB cross-border
  reporting with the reason the customer gives (`services/bank/payments-orchestrator`, `onboarding` OpenCurrencyAccount).
- [x] **Savings and notice deposits:** daily interest, paid monthly; 32- and 90-day notice (`services/bank/deposits-service`).
- [x] **Credit:** overdrafts, working-capital loans and revenue-based financing, decided by a credit officer in the back
  office (`services/bank/lending-service`).
- [x] **Card payments:** payment links and point-of-sale requests, next-day settlement, refunds; card details never
  reach Forge (`services/bank/acquiring-service`).
- [ ] **Owner/External.** For currencies: an authorised-dealer arrangement and the SARB reporting format and channel
  (VERIFY-FINSURV); nothing is submitted yet.
- [ ] **Owner/External.** For savings: treasury and regulatory sign-off of the products and rates.
- [ ] **Owner/External.** For credit: a credit policy and committee, NCA registration, a credit bureau, capital.
  Built, against simulators and with draft wording: a credit bureau check (consent, fail closed, an adverse record needs a written
  reason), the NCA pre-agreement statement (generated, acknowledged by hash), and write-offs (two people, after a default notice, 90 days
  past due). Not built: a real bureau, collections beyond retrying, recoveries after a write-off.
- [x] **Card issuing:** virtual debit cards with real-time approval against the ledger (`services/bank/card-service`),
  tested against a processor simulator; wired to the customer app (`/cards`) but not switched on (its navigation link appears only while it is on).
- [x] **Third-party offers** (`services/platform/partner-offers`): introduces investment and crypto-asset products that a licensed
  provider supplies. **Every product is provided, licensed and monitored by a third party, never by Forge**, and the code states
  it in every offer. Wired to the customer app (`/invest`, with a consent step) and to a compliance screen in the back office (`/partners`), but **switched off**; no provider chosen. The navigation links appear only while the product is on. Built behind that switch: a signed, consent-checked sender of the customer's name to the provider, an automatic licence re-check against a regulator register extract (suspends on mismatch, never trusts a stale file), and a referral-fee ledger entry booked only against a bank statement line, within the cap the customer was told, by two people. The real regulator register source and a real provider are not yet available.
- [ ] **Owner/External.** For card issuing: a programme manager or sponsor, a processor, and the regulator's view.
- [ ] **Owner/External.** For card payments: an acquiring bank (VERIFY-ACQUIRER) and its adapter; chargebacks;
  reconciliation of acquirer payouts.

**4. Production infrastructure: none exists yet**

- [x] Kubernetes manifests for the 23 services (`deploy/k8s`): mutual TLS certificates, caller lists, network policies,
  hardened pods, health probes. Generated and checked on paper; **never applied to a cluster**.
- [ ] **Owner/Engineering.** A cloud account and region (ADR-0002), and the data layer and secret store the manifests
  expect (`deploy/k8s/README.md` lists 9 things still to decide). Nothing provisions production yet.
- [ ] **Engineering.** A production TigerBeetle cluster (six replicas on dedicated hosts), measured, with an upgrade
  rehearsal.
- [ ] **Engineering.** Production Temporal, Kafka (TLS and SASL, more than one broker) and Postgres (managed, with TLS).
- [ ] **Engineering.** A secret store delivering the `*_FILE` secrets; a certificate authority issuing the service
  certificates.
- [ ] **Engineering.** Off-site, immutable storage for backups, the WAL archive, audit anchors and evidence packs
  (VERIFY-WORM).
- [ ] **Engineering.** A tracing backend, metrics, dashboards, alerts and an on-call rota.
- [x] A health listener that needs no client certificate (`HEALTH_ADDR`, `/livez`, `/readyz`), tested in `pkg/serve`.

**5. Assurance before go-live**

- [ ] **External.** An independent security review and penetration test.
- [x] Returns: balance sheet and income statement for both forms (BA 100/120, DI 100/200) from the ledger, with
  auditor sign-off; pro formas from projections (`make proforma`). Other forms not built.
- [x] Outsourcing notice dates, provider contract clauses, a business impact analysis and a security findings
  register, each checked by the gate.
- [ ] **Engineering.** A load test through the whole payment path, on production hardware.
- [ ] **Owner/Engineering.** An incident and recovery drill on production: a restore, the loss of a site.
- [ ] **Owner.** A pilot with a small number of friendly customers before opening applications to the public.

### Next steps, in order

1. **Owner:** sign in to the WorkOS and Didit MCP servers with the Forge account.
2. **Engineering:** configure WorkOS and Didit, and run the first live sandbox onboarding.
3. **Owner:** name the threshold owners (a mock roster with hiring advice is in
   [`docs/THRESHOLD_OWNERS.md`](docs/THRESHOLD_OWNERS.md)), choose the sanctions source, and start the licence or sponsor conversation.
4. **Engineering:** production infrastructure as code, then the production clusters, measured.
5. **External:** sponsor sandbox, security review, then a pilot.

**Verified**

- The full gate (`make ci-db`) passes on the head of the branch, last run 2026-11-23: lint, 83 test packages against real Postgres 17, Kafka 3.9 and TigerBeetle 0.16.60, the
  entity-isolation check and the caller-list check.
- 477 Go test functions in 37 modules. Playwright: 70 tests across the two apps, including the accessibility scans
  and the security-header checks.
- The local stack starts from nothing in one command (`make up`). `make test-stack` passes on a fresh stack: a customer
  onboarded through the real workflow sees the account it opened, and a compliance person decides a real review and
  the onboarding completes.
- The Didit adapter's webhook signature reproduces Didit's own published reference code on over 180 vectors, and the
  onboarding suites pass against a real Temporal dev server (`make test-temporal`, which the gate does not run), including
  a whole onboarding completed by Didit-signed deliveries. It has **not** run against Didit itself.
- **The ledger runs on TigerBeetle 0.16.60**, and the gate tests against it on every run. The in-memory model that every
  other suite relies on is held to the real ledger: the same random batches go to both, and every outcome, balance and
  stored transfer must match (48,000 batches on four seeds, after the test found and fixed four differences). A
  three-replica cluster was taken apart while in use: it survived losing its primary, used a restarted replica to
  survive losing another, accepted nothing without a quorum, and applied a transfer retried across the outage exactly
  once. On the local stack a balance survived a restart of the ledger service and of TigerBeetle. All of this is a
  development-mode cluster on one machine (`services/bank/ledger-gateway-bank/SPEC.md` section 12).
- **Mutual TLS runs end to end on the stack** (`deploy/compose/up.sh mtls`, `make test-stack-mtls`). Every service
  speaks TLS and requires a client certificate, and the journeys pass through it, browser to ledger. Checked against the
  running services: a caller with no certificate, or one from another authority, gets no answer; the caller is the
  service its certificate names, and a header that says otherwise is ignored; each of the two APIs for people serves
  only its own web app's server. Every certificate was renewed on disk while the stack ran, and every service presented
  its new one within seconds, with no restart.
- **Caller lists are least privilege, and stay that way.** Each service may call only the methods its code calls: 83
  unneeded grants were removed (reconciliation could create transfers and approve payments; every reader of the ledger
  could write to it). `make ci` fails on a grant the caller's code does not use (`tools/isolation`).
- Production refusals, request IDs and security headers are unit-tested (`pkg/serve`, `pkg/callerauth`).
- **No shipped code calls a known vulnerability** (`make vulncheck`, part of the gate): every Go module is scanned
  against the Go vulnerability database and both apps against npm's. Getting there meant moving from Go 1.24.7 (out of
  support, 41 reachable standard-library vulnerabilities) to Go 1.27.4, and upgrading pgx, klauspost/compress,
  golang.org/x/net, OpenTelemetry and gRPC.
- **Secrets come from files, not the environment.** Every secret a service reads (database URLs, encryption keys,
  webhook secrets, API keys) may be given as `NAME_FILE`, a mounted file, which is how Kubernetes secrets, Vault Agent
  and cloud secret drivers deliver them. Production refuses to start with any of them in the plain environment. On the
  stack, the generated keys are files, and `docker inspect` shows only their paths.
- **Traces reach a collector and follow a request across services.** On the stack all 18 services export spans to
  an OpenTelemetry collector. The trace context crosses every service call and goes into Temporal workflows and out
  through their activities, so one onboarding is one trace through onboarding, party, consent, policy and the
  ledger. Not done: a tracing backend to read them in, and sampling settings for production volume.
- **Point-in-time recovery works.** The stack's Postgres archives every WAL segment (at least once a minute), and a
  backup service takes a verified base backup on a schedule and keeps 14 days. `scripts/pitr-check.sh` writes rows,
  picks a moment, then adds a row and drops a table, and restores a fresh copy to that moment: the rows from before
  are there, the later row is not, and the dropped table is back. Passed twice on the running stack.
- **Kafka survives losing a broker, and refuses writes it cannot make durable.** Against three brokers (three
  replicas, two in sync required) the outbox relay lost a broker mid-run with no event lost or reordered; with two of
  three down it refused to publish and left the events in the outbox; on recovery it delivered them. That test found
  the relay hung for ever, inside its database transaction, when the brokers had no quorum: the Kafka client retried a
  record with no deadline, and its own timeout did not stop it once the brokers had lost their controller majority.
  Both publishers now return at the caller's deadline whatever the client does, and the relay retries on its next pass.
  Passed four runs in a row (`make test-kafka-cluster`).
- **Only the page's own scripts run.** Both apps send a Content-Security-Policy with a fresh nonce on every page, and
  every script Next.js renders carries it; `'unsafe-inline'` is gone from `script-src`. A script tag injected into a
  page's HTML does not run (tested in both apps; those tests fail under the old policy). Styles keep
  `'unsafe-inline'`, since React sets style attributes; an injected style cannot run code.
- **Under load, and through crashes, no money is made, lost or moved twice** (`make load-ledger`). 32 clients sent
  about 400 transfers a second for a minute to the ledger on TigerBeetle, with refusals, retries of the same request,
  the ledger service killed and then TigerBeetle killed. Afterwards every account held exactly what the accepted
  transfers say, no customer was negative, and the balances summed to zero; every retry got the first answer back.
  p50 about 60 ms, p99 under 200 ms on one shared machine, with development-mode TigerBeetle.
- Both web apps are on Next.js 16.4.6. 16.4.1 was in the range of a published advisory (GHSA-mfr8-8qww-v76h, moderate;
  the apps do not use the affected feature). `npm audit --omit=dev` reports no known vulnerability in either app.
- A KYC decision is recorded before the onboarding is told, and is redelivered until it is. Shown on the stack: with
  Temporal stopped, a decision was recorded and the reviewer was told it would be retried; once Temporal was back, the
  onboarding completed within 20 seconds with no further action.

**Blocked by people** (`docs/DECISIONS.md`, `docs/QUESTIONS.md`, `docs/THRESHOLDS.md`)

- Threshold owners: the 31 placeholder thresholds each need a named owner who confirms them. None is named.
- Sanctions list source and licence: undecided. Screening runs on a one-name fixture.
- Identity provider: decided on 2026-11-03 (WorkOS AuthKit), but the account does not exist and nothing has run against
  it. The apps sign in with development sign-in only.
- Didit for KYC and KYB: decided on 2026-11-03, in a new account the owner creates. The organisation does not exist. The
  adapter is built from Didit's documentation and tested without an account (its webhook signature is checked against
  Didit's own reference code), but it has never run against Didit, so the local stack uses the fake provider. The
  owner's steps are in `docs/PROVIDERS.md`.
- ADR-0020 (legal form of the bank layer) and ADR-0012 (sector appetite): open human decisions.
- goAML schema (VERIFY-GOAML): submission is refused until it is confirmed.
- WORM storage for audit anchors and evidence packs (VERIFY-WORM): local files only.
- Regulatory review of the policy and reporting rules: not started, and not in this repository.

**Blocked by external systems**

- TigerBeetle runs, in development mode only (since 2026-11-16; see *Verified*). Not done: a production cluster (six
  replicas on dedicated hosts, without `--development`), its throughput, an upgrade rehearsal, and the repair rate of a
  restarted replica on that hardware.
- Sponsor bank: no sandbox, and the wire format is unverified (VERIFY-SPONSOR). Partner and FIC formats too
  (VERIFY-PARTNER, VERIFY-FIC).
- Kafka: verified as a three-broker cluster in tests, but the stack runs one broker, and TLS and SASL to the brokers
  are not set up.
- Temporal runs as a single development server. No production cluster is set up.
- core-bridge's posting path was checked once against a live Fineract. It is not on the stack.

**Engineering gaps**: see *What is left before real customers* above.

## Built and tested

Each service is its own Go module. `make ci-db` creates throwaway databases on the compose Postgres 17, then runs the
full gate: lint, every module's tests against real Postgres and a real Kafka broker, and the isolation check. Every
database-backed suite runs; the only opt-in suite is the live Fineract test, which needs `FINERACT_TEST_URL`.

- **Ledger gateway** (`services/bank/ledger-gateway-bank`): the only writer to the bank ledger. Linked transfer chains,
  pending, post and void, idempotency, transactional outbox, and `ListAccounts` over the account directory. Passes one
  shared behaviour suite in memory, against Postgres and against TigerBeetle. A request the ledger refused stays
  refused if it is retried after its answer was lost (`PREVIOUSLY_REFUSED`), and a ledger call gives up at a deadline
  instead of waiting for a cluster that has no quorum.
- **Policy service** (`services/shared/policy-service`, `policy/`): embedded OPA v1.13.0, Ed25519-signed bundles, every
  decision audited to the outbox before it is returned. The break rule is strict: while any reconciliation break is
  open, every payment is held (`docs/DECISIONS.md`).
- **Party and consent services** (`services/platform/party-service`, `consent-service`): national IDs encrypted
  (AES-256-GCM) with a keyed index, founder and company separation enforced, consent checked live.
- **Onboarding** (`services/platform/onboarding`): Temporal workflow `OnboardBusiness`, tested on a real Temporal dev
  server and run on the local stack. A provider's REVIEW verdict parks the workflow until a compliance person decides
  it; the decision is recorded once, then delivered to the workflow until it has been told. On the local stack KYC uses a **fake provider**. The Didit
  adapter is built, with a recorded session per subject and signed-webhook verification, and tested without an account; it has never run
  against Didit (see *Blocked*).
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
  The app has a KYC review screen, which decides real reviews on the local stack. Twenty-two Playwright journeys pass
  against the production build, including the accessibility and security-header checks. One more runs against the
  local stack with nothing mocked.
- **Customer web app** (`apps/web`): accounts with entity and insurance on every balance, activity, payments, FX with a
  disclosure that must be accepted, and bulk payouts with maker-checker. Twenty-two Playwright journeys pass against the
  production build, with the API mocked in the browser. Two run against the local stack, where an account opened
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
KYC review screen and live cases. The customer application flow and the `OnboardPerson`
workflow were built on 2026-11-17. Not yet built: moving the payments orchestrator's rails from the rails-gateway simulator onto
`BankLayer`, and the core-bridge on the local stack (it needs a live Apache Fineract there).

## Production configuration

Done and tested: TLS and mutual TLS between services, on the running stack as well as in unit tests (`pkg/serve`:
production refuses to start without certificates, the caller is the service named in its certificate and not a header,
a renewed certificate is loaded without a restart, and the two APIs for people serve only the web apps named in
`WORKSPACE_CLIENTS` and `BACKOFFICE_CLIENTS`, which production requires); each web app's server presents its own
certificate to its API and refuses a plain-HTTP API address in production; request IDs, timeouts and security headers on
every service and both web apps (the Content-Security-Policy runs only scripts carrying a per-request nonce); a
production refusal of every development switch (memory ledger, simulators, unsigned policy, fake KYC); OpenTelemetry
spans, with OTLP export when `OTEL_EXPORTER_OTLP_ENDPOINT` is set; a backup and restore check that proves a dump
restores (`scripts/pg-backup.sh`, `scripts/pg-restore-check.sh`); and an axe accessibility scan of every page in both
apps, failing on serious and critical violations.

Not done: under mutual TLS a health check needs a certificate,
because there is no separate health listener; no secret store is wired to deliver the secret files. Backups are
scheduled and point-in-time recovery is proven on the stack (see *Verified*).

## Blocked

- **Didit (KYC/KYB)**: the decision is a new organisation, not A-CARD. That organisation does not exist yet, and no tool
  in this environment can create one. The owner must create it (by adding Didit's own connector and signing up on its
  page with the personal email) and grant access. The adapter is built, but no verification session has been created, no
  delivery from Didit has been checked, and none is approved.
- **TigerBeetle: unblocked on 2026-11-16**, by allowing one host (`pkg-containers.githubusercontent.com`, which serves
  the image's layers) in the build environment. What is still blocked is a production cluster: hardware, sites and an
  operator. Nothing in this repository provisions one.
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
make ci-db            # the full gate, with throwaway databases and a throwaway TigerBeetle replica in Docker
                      # (defaults: Postgres on 5432, broker on 9092; for the stack's own, set
                      # FORGE_PG_HOST_DSN=postgres://postgres:forge-local-only@127.0.0.1:5433 and
                      # FORGE_KAFKA_BROKERS=127.0.0.1:9192)
make test-ledger-tb-cluster  # three TigerBeetle replicas, taken apart while in use (not part of the gate)
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
