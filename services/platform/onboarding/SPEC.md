# SPEC: onboarding

Status: built and tested locally, including against a real Temporal dev server. The Didit adapter is not implemented (see section 6).
Human review required before any real customer (ARCHITECTURE.md section 0, rule 3).
Entity: PLAT. Sections referenced: ARCHITECTURE.md 11.2, 11.3, 15.1, P1-05.

## 1. Purpose

Takes a business application and, through a Temporal workflow, creates its parties, records consents, verifies
the company and each director, screens them, checks the sector, and opens the company's ZAR current account. A
business that passes every check is activated. One that does not is declined, and its parties are closed.

## 2. Interface

- ConnectRPC `forge.onboarding.v1.OnboardingService`: `StartBusinessOnboarding`, `GetOnboarding`. The caller
  allow-list applies.
- HTTP `POST /webhooks/kyc`: the provider's verdict, authenticated by `X-Forge-Signature` before it signals the
  workflow. A forged, altered or unsigned body returns 401 and never reaches the workflow.

## 3. The workflow (`internal/onboarding/workflow.go`)

Steps, in order, each an activity with bounded retries:

1. `prospects_created`: company and each director, as PROSPECT parties. Keys derive from the onboarding ID.
1b. `relationships_created`: each director is linked to the company as a director, and as a shareholder or UBO when
   the application gives them a share. The policy's person-to-entity rule (transfer code 63) relies on these links.
2. `consents_recorded`: company terms, KYB processing and director KYC consents, each with an evidence reference.
3. `verifications_started`: parties move to KYC_PENDING, and the provider starts each verification.
4. `kyc_approved`: waits for a signed verdict per party. Any decline, or 7 days without an answer, declines.
5. `screened`: sanctions and PEP check on the company and every director. A match declines. An unavailable or stale
   list declines too (fail closed).
6. `sector_eligible`: policy-service `sector` decision. Only listed sectors pass.
7. `accounts_created`: tier set, then the company's ZAR current account (ledger code 1001, debits must not exceed
   credits, owned by the company). This is the first money-side effect; nothing before it touches the ledger.
8. `activated`: every party moves to ACTIVE.

A decline before step 7 closes every party and creates nothing in the ledger. An error that exhausts its retries
marks the run FAILED and returns the error.

Each activity is idempotent: party, consent and account creation use keys derived from the onboarding ID, and
status changes read the current status first, so a retry is a no-op.

## 4. Identifiers and idempotency

- The onboarding ID is a version 5 UUID of the idempotency key, so the same key always names the same workflow.
- Starting with the same key and request returns the same onboarding. Starting with the same key and a different
  request returns `ALREADY_EXISTS`. The request hash is held in the workflow's status.
- The workflow ID is reused-rejected, so a finished onboarding cannot be started again under its key.

## 5. Tier

`AssessTier` returns `enhanced` when any director holds at least 5000 basis points (a controlling interest), and
`standard` otherwise. The 50% threshold is a placeholder for the risk owner to confirm. The limits for each tier are
in `policy/tiers.json`.

## 6. Open items and deviations

- **Temporal server.** Verified on a Temporal dev server (server 1.24.2, CLI v1.0.0) by `make test-temporal`
  (`TEMPORAL_HOST` required, behind the `temporaldev` build tag). The dev-server suite covers an approved run, the real
  KYC timer, duplicate and restarted workflow IDs, retry backoff for a transient ledger failure, a non-retryable failure
  that stops at once, the RPC idempotency and conflict paths, the status query, and signalled webhooks. Production
  must run against a real cluster with the same SDK; this has not been exercised with a multi-node server or
  Temporal Cloud (VERIFY-TEMPORAL).
- **Already-started errors must be requested.** The Temporal SDK swallows `WorkflowExecutionAlreadyStarted` by
  default and returns a handle to the existing run. The service sets `WorkflowExecutionErrorWhenAlreadyStarted`, so
  a changed request under the same key is refused. The dev-server test
  `TestDevSameKeyDifferentRequestIsRefusedOnTheRealServer` fails if the flag is removed (verified by mutation).
- **Didit is not implemented.** `kyc.Didit` refuses everything (`ErrNotConfigured`), so onboarding cannot approve
  anyone with the real provider. The session contract and webhook signature scheme must be confirmed against Didit's
  documentation before implementation (VERIFY-DIDIT).
- **Screening uses a fixture list.** The compliance engine (P1-06) replaces it. The list is mandatory: the service
  refuses to start without one.
- **Deposit insurance flag defaults to false.** `INSURED_DEPOSITS` must be set to `true` only when the sponsor confirms
  CODI coverage for business deposits (VERIFY CODI). Invariant 10 requires the flag to be true.
- **Accounts are created only for the company.** Personal founder accounts are a later step (ARCHITECTURE.md 6.1).
- **Fineract is not used.** ARCHITECTURE.md 11.3 step 7 creates a Fineract client; Fineract is not in this build.
- **Caller identity.** Calls to other services carry `X-Forge-Caller: svc-onboarding`. Production must use mTLS.
- **A UBO must hold a share.** `StartBusinessOnboarding` refuses a UBO with `ownership_bps` of zero. Party-service
  would refuse the relationship anyway; the early check gives the caller a clearer error.
- **`KYCTimeoutSeconds`** on the workflow input overrides the seven-day timer. Only tests set it.
- **Integration test.** `internal/integration` builds the real party, consent, policy and ledger binaries and runs the
  workflow against them, with an approved and a declined case. It needs `ONBOARDING_TEST_PG` (a Postgres base DSN).
