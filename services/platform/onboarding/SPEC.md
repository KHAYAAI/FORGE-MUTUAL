# SPEC: onboarding

Status: built and tested locally, and run end to end on the local stack for a personal application through the web
app (`apps/web/tests/stack/apply.spec.ts`). Built and tested locally, including against a real Temporal dev server. The Didit adapter is built from Didit's documentation
and tested without an account; it has never run against Didit (see section 6).
Human review required before any real customer (ARCHITECTURE.md section 0, rule 3).
Entity: PLAT. Sections referenced: ARCHITECTURE.md 11.2, 11.3, 15.1, P1-05.

## 1. Purpose

Takes an application, for a business or for a person, and runs it through a Temporal workflow. For a business it creates its parties, records consents, verifies
the company and each director, screens them, checks the sector, and opens the company's ZAR current account. A
business that passes every check is activated. One that does not is declined, and its parties are closed. For a person
(`OnboardPerson`, section 3b) it creates the person, records their consents, verifies and screens them, and opens a
personal ZAR current account.

## 2. Interface

- ConnectRPC `forge.onboarding.v1.OnboardingService`: `StartBusinessOnboarding`, `StartPersonOnboarding`,
  `FindApplication`, `GetOnboarding`, `ListKYCReviews`, `DecideKYCReview`. A start request may carry
  `applicant_subject`, the sign-in subject of the person applying; `FindApplication(applicant_subject, kind)` returns
  that applicant's onboarding of that kind (`person` or `business`), or `NOT_FOUND`. Only workspace-api, for the web
  app's front door, and the stack's journeys may call these two. The caller allow-list applies. `GetOnboarding` returns, besides the steps, a `verification_links`
  entry for each subject whose verification has been started: the subject, `person` or `business`, the provider's
  session ID and the hosted link the person completes.
- HTTP `POST /webhooks/kyc`: the provider's verdict. The provider authenticates it before it signals the workflow: the
  fake provider by `X-Forge-Signature`, Didit by `X-Signature-V2` with `X-Timestamp` (section 6). A forged, altered, stale
  or unsigned body returns 401 and never reaches the workflow. Other replies: 200 for a delivery that is not a verdict, or
  for an onboarding that has already ended; 202 for a verdict, signalled to the workflow; 400 for an authenticated
  delivery that is refused (a wrong environment, or no onboarding named); 503 when Temporal is unreachable, so that a
  provider which retries will try again. The body is limited to 1 MiB and the handler to 4 seconds. The sender is told
  only the status; the log says why a delivery was refused (a missing header, a clock more than five minutes out, a
  secret that does not match, a wrong environment) and which verdict was delivered, and never holds a secret, a
  signature or anything from the body.

## 3. The workflow (`internal/onboarding/workflow.go`)

Steps, in order, each an activity with bounded retries:

1. `prospects_created`: company and each director, as PROSPECT parties. Keys derive from the onboarding ID.
1b. `relationships_created`: each director is linked to the company as a director, and as a shareholder or UBO when
   the application gives them a share. The policy's person-to-entity rule (transfer code 63) relies on these links.
2. `consents_recorded`: company terms, KYB processing and director KYC consents, each with an evidence reference.
3. `verifications_started`: parties move to KYC_PENDING, and the provider starts each verification. The provider's
   session (ID and hosted link) is recorded in `kyc_session`, one row per subject per onboarding. A retried activity
   finds the stored session and does not start a second one, which with a live provider would be a second charge. A
   provider refusal that a retry cannot fix (a 4xx other than 429) is a non-retryable `ProviderRejected` error, and the
   onboarding fails; anything else is retried.
4. `kyc_approved`: waits for a signed verdict per party. Any decline, or 7 days without an answer, declines. A
   verdict of REVIEW means the provider cannot decide: a review is opened for a person (section 3a), and the wait
   continues under the same seven days.
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

## 3b. A person (`internal/onboarding/person.go`)

`OnboardPerson` reuses the business steps for one subject: `prospects_created` (the person), `consents_recorded`
(account terms and KYC processing), `verifications_started`, `kyc_approved` (the same verdict wait and review path),
`screened`, `accounts_created` (a `personal-zar-current` account owned by the person, tier `standard`) and `activated`.
There is no sector check. `StartPersonOnboarding` refuses an identity number that is not 13 digits with a valid check
digit.

**One application per applicant.** When a start carries `applicant_subject`, the idempotency key is
`applicant/<kind>/<subject>`, so the workflow ID follows from the applicant: a second start returns the first
application, and `FindApplication` finds it without a lookup table. `GetOnboarding`'s status carries `kind`.

## 3a. KYC review

A REVIEW verdict opens one review per subject (a repeat verdict returns the same review). `ListKYCReviews` lists them,
oldest first. `DecideKYCReview` takes APPROVED or DECLINED, the reviewer's name, and a note of 20 to 1000 characters
saying what was checked and why. The back office supplies the reviewer from the signed-in person, never from the request.

A decision is a compliance record, so it is **recorded first**, and the workflow is told second:

- Only one decision is ever recorded. A second reviewer is refused. The same decision again (a retry whose response was
  lost) succeeds and changes nothing.
- A recorded decision is marked delivered only once the workflow has been told. If Temporal is unreachable, the decision
  stays on the record, undelivered, and the reviewer is told it will be retried (`UNAVAILABLE`). A retry of the same
  decision, or the redelivery loop (every 15 seconds), finishes the delivery.
- The workflow ignores a decision for a subject it is not waiting on, so telling it twice is harmless.
- If the onboarding has already ended (declined, timed out, completed) there is nothing to resume. The decision stays on
  the record, is marked settled so it is not retried, and the reviewer is told (`FAILED_PRECONDITION`).
- An APPROVED decision lets the subject count as verified. A DECLINED decision declines the whole application.

Verified on the local stack by stopping Temporal between the decision and its delivery: the decision was recorded and the
reviewer told, and once Temporal was back the onboarding completed on its own. A table made before delivery was tracked
is upgraded in place; its decided rows are told to the workflow once more, which is harmless.

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
- **Didit is built but has never run against Didit (VERIFY-DIDIT).** `internal/kyc/didit.go` creates a session per
  subject (`POST /v3/session/`, `vendor_data` the party ID, `metadata` the onboarding ID, party ID and kind), and
  verifies webhooks by `X-Signature-V2`: the hex HMAC-SHA256 of the body's canonical JSON, with `X-Timestamp` within five
  minutes. `Approved`, `Declined` and `In Review` become APPROVED, DECLINED and REVIEW, exactly and case-sensitively;
  every other status is acknowledged and ignored. A delivery from a different environment than `DIDIT_ENVIRONMENT`, or
  one that does not name an onboarding and a subject, is refused. The canonical JSON is checked against Didit's own
  Python reference on over 180 vectors (`internal/kyc/testdata`); the rest is checked against a local fake of the API.
  Settings: `KYC_PROVIDER` (`didit`, the default, or `fake`), `DIDIT_API_KEY`, `DIDIT_KYC_WORKFLOW_ID`,
  `DIDIT_KYB_WORKFLOW_ID`, `DIDIT_WEBHOOK_SECRET`, `DIDIT_WEBHOOK_SECRET_PREVIOUS` (while rotating), `DIDIT_ENVIRONMENT`
  (`sandbox` or `live`), `DIDIT_BASE_URL`, `DIDIT_CALLBACK_URL`. Production (`FORGE_ENV=production`) refuses
  `KYC_PROVIDER=fake` and `DIDIT_ENVIRONMENT=sandbox`. `cmd/didit-verify` runs a delivery Didit recorded through the
  verifier, which is the first check against a real account. Details, and what is not yet verified, are in
  `docs/PROVIDERS.md`.
- **A verification link reaches the applicant through the web app's status page** (`apps/web/app/apply/status`), for
  the person who applied. For a business, the applicant passes each director their own link; no notification sends it.
- **An abandoned or expired Didit session is not a verdict.** The onboarding waits out its seven days and declines. Whether
  to decline earlier or start a new session is a product decision.
- **Screening uses a fixture list.** The compliance engine (P1-06) replaces it. The list is mandatory: the service
  refuses to start without one.
- **Deposit insurance flag defaults to false.** `INSURED_DEPOSITS` must be set to `true` only when the sponsor confirms
  CODI coverage for business deposits (VERIFY CODI). Invariant 10 requires the flag to be true.
- **Personal accounts** are opened by `OnboardPerson`. A director of a business onboarding does not get one, and gets
  no sign-in of their own: only the applicant can see the business account.
- **Fineract is not used.** ARCHITECTURE.md 11.3 step 7 creates a Fineract client; Fineract is not in this build.
- **Caller identity.** In development, calls to other services carry `X-Forge-Caller: svc-onboarding`. In production the
  service presents its own certificate and the callee reads the service name from it (`pkg/serve`); the header is
  ignored there. This is covered by unit tests, not yet by a running production-mode stack.
- **A UBO must hold a share.** `StartBusinessOnboarding` refuses a UBO with `ownership_bps` of zero. Party-service
  would refuse the relationship anyway; the early check gives the caller a clearer error.
- **`KYCTimeoutSeconds`** on the workflow input overrides the seven-day timer. Only tests set it.
- **Integration test.** `internal/integration` builds the real party, consent, policy and ledger binaries and runs the
  workflow against them, with an approved and a declined case. It needs `ONBOARDING_TEST_PG` (a Postgres base DSN).
