# SPEC: payments-orchestrator

Status: built and tested locally: unit tests, the workflow in Temporal's test environment, an end-to-end test against
the real party, policy, ledger and rails binaries, and the Temporal dev server. Human review required before any real
customer (ARCHITECTURE.md section 0, rule 3). Entity: BANK. Sections referenced: ARCHITECTURE.md 9.1, 9.2, 13, 16.

## 1. Purpose

Moves money out of a customer's account by domestic EFT (`DomesticPayment`). It checks the beneficiary, the party,
policy and the day's velocity, places a hold in the ledger, sends the instruction to the rail, waits for the outcome,
then settles the hold or releases it. Each payment is one Temporal workflow.

## 2. Interface

ConnectRPC `forge.payments.v1.PaymentsService`: `CreatePayment`, `GetPayment`, `ApprovePayment`, `ResolveRepair`.
The caller allow-list applies to each RPC.

## 3. The workflow (`internal/payments/workflow.go`)

1. `party_loaded`: the paying party must be ACTIVE, otherwise DECLINED `party_not_active`.
2. `screened`: the beneficiary name is checked against the screening list. A match is DECLINED. An unavailable list is
   DECLINED (fail closed).
3. `policy_allowed` (asset): `assets` and `entity` policy. ZAR for the BANK entity, and a same-entity transfer under
   code 11.
4. `policy_allowed` (limits): `limits` policy with the party's tier, the day's velocity (held and settled today) and
   the reconciliation gate. A refusal is DECLINED.
5. Step-up: when policy asks for it the payment is AWAITING_APPROVAL. A checker's approval is needed. The initiator's
   own approval is ignored. Refusal or the approval timeout CANCELs the payment before any hold.
6. `reserved`: the hold is placed: a pending ledger transfer from the customer's account to RAIL_SETTLEMENT_EFT,
   code 11. Insufficient funds is DECLINED.
7. `rail_initiated`: the instruction is sent. A rejection on initiation releases the hold. If the call fails, the
   outcome is unknown, so the payment goes to REPAIR_REQUIRED.
8. `rail_final`: the rail is polled every `PollSeconds` until it reaches SETTLED, REJECTED or RETURNED. After
   `MaxPolls` polls with no outcome the payment goes to REPAIR_REQUIRED.
9. `settled` posts the hold, or `released` voids it. REJECTED and RETURNED release.

## 4. Safety rules

- A failure before the hold is placed declines the payment. Nothing has moved.
- Once a hold exists, no automatic step releases it unless the rail reports REJECTED or RETURNED. An unknown outcome,
  or a failed step after the hold, goes to REPAIR_REQUIRED and waits for an operator (`ResolveRepair`).
- A payment the rail has settled is never released. If posting the hold fails, the payment goes to repair.
- Each step is idempotent: the ledger keys are `<payment>:reserve`, `:settle` and `:release`, and the rail key is the
  payment ID. A retry returns the same answer.
- The payment row and its outbox event are written in one transaction (`payment` table, `pkg/outbox`).

## 5. Velocity and limits

The day's velocity is the sum of the party's payments in the asset that are RESERVED, SETTLED or REPAIR_REQUIRED
since 00:00 UTC. Policy's `limits` package compares it with the tier's daily limit.

## 6. Open items and deviations

- **Hold timeout equals the ledger maximum (7 days).** The ledger releases a hold at its timeout. If the rail has not
  reported an outcome by then, the release happens without a human decision. The repair path covers the common
  cases; the window is set by the rail's outcome time, so a slow rail needs an alert before the timeout. VERIFY-SPONSOR.
- **Reconciliation gate.** Payments reads `ReconService.GetGate` from the reconciliation service at `RECON_URL`. The
  binary refuses to start without it. An unreadable gate is an error, so the limits activity retries and no money
  moves. The gate is open only when every monitored asset has a fresh, conclusive run in which every check matched.
- **The account-to-party binding is not checked here.** The ledger does not expose an account's owner, so a caller
  could name another party's account. A directory lookup by owner is needed before production.
- **Only the DomesticPayment workflow exists.** InboundCredit (P1-07) is not built.
- **Compliance is the screening list only.** The compliance engine (P1-06) and transaction monitoring are not built.
- **Simulation.** `PAYMENTS_ALLOW_SIMULATION` lets callers pick a rail scenario. It is off unless set.

## 7. Tests

- `internal/payments`: thirteen workflow cases with fake activities, including a mutation check that settling a
  rejected payment fails the suite.
- `internal/service`: validation, idempotency, conflicts, simulation gating, the maker-checker rule, repair gating,
  and the proto mapping of every workflow state.
- `internal/integration` (needs `PAYMENTS_TEST_PG`): five scenarios against the real party, policy, ledger and rails
  binaries. The money is checked after each: settled moves it, rejected returns it, insufficient funds moves nothing,
  a blocked beneficiary places no hold, and a large payment waits for a checker. A mutation check that settles
  rejected payments fails the rejection scenario.
- `internal/service` dev-server tests (`temporaldev` tag, `TEMPORAL_HOST`): idempotent creation and conflicts on a
  real server, and repair resolved through the service's signal.

## Inbound credits (internal/inbound)

Credits the bank reports are recorded, then booked once each.

- **Matching.** A bank reference is matched to a customer account registered with `RegisterReference`. A reference
  is bound to one account for good: rebinding is refused, because a moved reference would send later money to the
  wrong customer. Anything unmatched, or matched to an account in another asset, goes to SUSPENSE (code 2005) and
  waits for an operator.
- **Posting.** Debit the pool (sponsor pool 3003 for ZAR, the partner mirror 5001 for USD), credit the customer or
  SUSPENSE, transfer code 10 (DEPOSIT_IN). The idempotency key is `inbound:<asset>:<bank movement>`, so a retry after a
  crash returns the original posting and never books twice.
- **In flight.** A credit is PENDING from the moment it is recorded until it is posted. `InFlight` totals PENDING
  credits. Reconciliation should read this as its inbound in-flight term; that wiring is not yet built, so the
  reconciliation service still assumes zero (see its SPEC).
- **Not yet wired.** Customer accounts do not register their bank references yet. Until onboarding does, every
  inbound credit goes to suspense. The binary runs this loop only when `INBOUND_ENABLED=1`.

## Sponsor clearing and the in-flight endpoint

- **Clearing leg.** When the rail reports SETTLED, Settle posts the hold (customer to the settlement account 3001), and
  then records the sponsor's debit: RAIL_SETTLE (code 31) moves the amount from the settlement account to the sponsor
  pool (3003). Without this leg the pool keeps the money and reconciliation breaks after every settled payment. The
  integration test checks that the settlement account nets to zero and the pool holds the amount. A mutation without
  the leg fails that test.
- **Status is recorded when it is seen.** `RailStatus` writes the rail status on every poll, not only at the end. That
  is what makes "rail says settled, payment not complete" visible to reconciliation during the wait.
- **`InFlightService.GetInFlight(asset)`** returns two totals. Inbound: pending inbound credits. Outbound: payments whose
  rail status is SETTLED and whose state is not COMPLETED, including payments in REPAIR_REQUIRED, because the money has
  left the bank and the books still show it. Callers are allow-listed like every other method.
- The sponsor pool account is created by `pkg/systemaccounts` under the fixed key `system:pool:ZAR:v1`. Inbound credits
  use the same key, so both see the same account.
