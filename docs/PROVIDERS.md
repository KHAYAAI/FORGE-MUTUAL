# Identity and KYC providers

Decided by the owner on 2026-11-03 (see `docs/DECISIONS.md`):

- **Sign-in: WorkOS (AuthKit).**
- **KYC and KYB: Didit.**
- Both in **new accounts the owner creates**, on a personal email address. Not A-CARD.

This page says where each stands, what only the owner can do, what Didit offers for South Africa, how the Didit adapter
works, and what must be checked first once the accounts exist. The Didit adapter was written from Didit's own
documentation, read through the Didit connector's documentation search. WorkOS facts marked *unverified* come from public
search summaries, because the WorkOS documentation host could not be reached from the build session.

## Where things stand

| Piece | State |
| --- | --- |
| WorkOS account for this project | Not created. |
| Didit organisation for this project | Not created. |
| Sign-in in both apps | Built and tested against the development identity provider (`tools/devidp`). Not run against WorkOS. |
| Claims the apps read | `forge_party` (customer) and `forge_roles` (staff). Whether WorkOS can issue them is *unverified* (see below). |
| Didit adapter in onboarding | **Built, never run against Didit.** It creates one hosted verification session per subject, records it, verifies Didit's webhooks and maps the three verdict statuses. Tested against a local fake of Didit's API and against recorded vectors (see "The Didit adapter"). The service itself defaults to Didit and will not start without Didit's settings; the local compose stack sets `KYC_PROVIDER=fake` unless told otherwise, because Didit cannot reach a laptop. |
| Didit webhook verification | **Built.** `X-Signature-V2`, as Didit's documentation describes it. Checked against Didit's own published Python reference on over 180 vectors. Not yet checked against a delivery Didit really sent. |
| Hosts blocked from the build session | `api.workos.com`, `workos.com`, `verification.didit.me`, `apx.didit.me`, `docs.didit.me`. |
| Connectors in the build session | A Didit and a WorkOS connector exist, both signed in to **A-CARD**. They must not be used to create or change anything for this project. The owner is adding separate connectors for the new accounts (next section). |

## What only the owner can do, in order

1. **Add the providers' own connectors, signed in to the new accounts.** Keep the A-CARD ones as they are and add two
   custom connectors beside them, named for this project (for example "Didit - Forge Mutual" and "WorkOS - Forge
   Mutual"). The server addresses: Didit `https://mcp.didit.me/mcp` (from Didit's documentation), WorkOS
   `https://mcp.workos.com/mcp` (*unverified*: from a search summary of WorkOS's documentation, so check it against
   workos.com/docs if the sign-in page does not come up). Sign out of A-CARD first, or use a private window, so the
   sign-in page does not reuse an A-CARD session. Sign up on that page with the personal email: that is also how each account gets created, and it
   accepts each provider's terms, which is the owner's act. Then tell the build session. It will check, with each
   connector's read-only identity call, that it is talking to the new account (not A-CARD) before it creates anything.
2. **Allow the provider hosts** in the cloud environment's network settings (the environment menu in the session title
   bar, then Edit, then Allowed domains, with *Allow package managers* left ticked). Add:
   `verification.didit.me`, `apx.didit.me`, `docs.didit.me`, `api.workos.com`, `workos.com`, and the project's AuthKit
   domain once WorkOS shows it. Steps: <https://code.claude.com/docs/en/cloud-environments#network-access>. The build
   session can also propose this itself, and the owner approves it.
3. **Hand over the secrets through the secure form or the environment's secrets setting, not in chat**: for Didit an API
   key and a webhook secret; for WorkOS the client ID, an API key, and the AuthKit issuer. They go in
   `deploy/compose/.env` for the local stack (never committed).
4. **Do the live steps only the owner can**: pass the Didit hosted check once with a real document, and sign in to
   WorkOS once.

Once the new accounts are connected, the build session creates the rest through the connectors: the Didit application,
the KYC and KYB workflows (the KYB workflow without Didit's own key-people check, because every director gets a KYC
session of their own), and the webhook destination; and the WorkOS AuthKit application, roles, redirect URIs and token
template.

## What Didit offers for South Africa

Read from Didit's own catalog through its connector on 2026-11-03. Prices are Didit's retail prices, not a quote.

- **Home Affairs ID lookup.** The applicant gives a 13-digit ID number (Luhn checked), first and last name, and a selfie.
  Didit checks the Department of Home Affairs and returns the full name, date of birth, gender, citizenship, the ID
  status (valid, deceased or blocked) and the registry portrait, which the selfie is matched against. About $2.68 per
  lookup. Consent basis shown as POPIA. Availability: available. Document OCR is about $0.15.
- **Business verification (KYB) from the company registry.** Company profile about $2.00, shareholders about $5.00,
  beneficial owners about $9.00, per company. Continuous registry monitoring is **not** offered for South Africa. The
  catalog marks South Africa `validated: false`; ask Didit what that means before relying on it.
- Workflows can combine document OCR, liveness, face match, AML screening and IP analysis for a person, and registry,
  documents, key people and AML for a business.
- At these prices a business with one director costs roughly $12 to check at the beneficial-owner tier, plus liveness and
  AML for the director. That is indicative only.

## The Didit adapter

`services/platform/onboarding/internal/kyc/didit.go`. It is the service's default provider (`KYC_PROVIDER=didit`; the
other value is `fake`, for development). The service refuses to start without every setting it needs, and a production
deployment (`FORGE_ENV=production`) refuses both `KYC_PROVIDER=fake` and `DIDIT_ENVIRONMENT=sandbox`.

| Setting | Meaning |
| --- | --- |
| `DIDIT_API_KEY` | The application's API key, sent as `x-api-key`. |
| `DIDIT_KYC_WORKFLOW_ID`, `DIDIT_KYB_WORKFLOW_ID` | The workflow UUIDs for a person and for a business. |
| `DIDIT_WEBHOOK_SECRET` | The webhook destination's shared secret (`secret_shared_key`), at least 16 characters. |
| `DIDIT_WEBHOOK_SECRET_PREVIOUS` | Optional. Also accepted, so the secret can be rotated without dropping deliveries. |
| `DIDIT_ENVIRONMENT` | `sandbox` or `live`. A delivery from the other environment is refused. A sandbox application mocks its providers and can approve anyone, so a live deployment must never act on its verdicts. |
| `DIDIT_BASE_URL` | Default `https://verification.didit.me`. Must be https (plain http only on the loopback interface, for tests). |
| `DIDIT_CALLBACK_URL` | Optional, https. Where Didit sends the person when they finish. |

**Starting a verification.** `POST /v3/session/` with the workflow, the subject's party ID as `vendor_data`, and the
onboarding ID, the party ID and the kind (`person` or `business`) as `metadata`. No name or ID number is sent: Didit
learns those from the person. The reply's session ID and hosted link are written to the `kyc_session` table, one row per
subject per onboarding, so a retried activity returns the stored session and does not start (and in live mode, pay for) a
second one. `GetOnboarding` returns each link as `verification_links`, for whatever hands the person their link. Nothing
sends it yet (see "What is still missing").

**Failures.** A transport error, a 429 or a 5xx is retried by Temporal. Any other 4xx is a request Didit will not accept,
so it is not retried: the onboarding fails with `PROVIDER_REJECTED` rather than looping. A reply with no session ID, or no
https link, is an error. The API key is sent only over https, a redirect is never followed (Go would forward the key to
it), and no error carries the key or any secret.

**Receiving a verdict.** `POST /webhooks/kyc`, one URL for both people and businesses.

1. Authenticate before reading anything: `X-Signature-V2` must be the hex HMAC-SHA256, under the destination's secret, of
   the body's canonical JSON (keys sorted, no spaces, Unicode kept, whole-valued floats written as integers), and
   `X-Timestamp` must be within five minutes of the clock. Every configured secret is tried, so the time taken does not
   say which matched. Anything that fails returns 401 and says no more.
2. Only `webhook_type: status.updated` can carry a verdict, and only from the configured environment.
3. `Approved` becomes APPROVED, `Declined` becomes DECLINED and `In Review` becomes REVIEW. The strings are exact and
   case-sensitive. Every other status (`Not Started`, `In Progress`, `Awaiting User`, `Resubmitted`, `Expired`,
   `Abandoned`, `Kyc Expired`) is acknowledged with 200 and ignored.
4. The delivery must name an onboarding (in `metadata`) and a subject (`vendor_data`), and if the metadata also names the
   subject, the two must agree. Otherwise it is refused with 400, which Didit does not retry.

Replies: 401 not authenticated; 200 not a verdict, or the onboarding has already ended; 202 a verdict, signalled to the
workflow; 400 authenticated but refused; 503 Temporal is unreachable, which Didit retries.

The sender is told no more than that, but the service's log says why. A refused delivery logs `kyc webhook refused` with
the status and a reason: `X-Signature-V2 or X-Timestamp is missing`, `X-Timestamp is 3600s old`, `X-Timestamp is 3600s
ahead of this service's clock` (the clock here runs slow), `the body is not a JSON document`, `X-Signature-V2 does not
match the body under any configured secret` (a wrong, rotated or other destination's secret), or why an authentic
delivery was refused (for example a sandbox delivery at a live service). A verdict logs `kyc verdict delivered` with the
onboarding, subject, verdict and session. The log never holds a secret, a signature or anything from the body, and a
status that is not a verdict is not logged. These lines are what to read when the first live delivery does not arrive.

Two properties of Didit's scheme to know. First, the signature covers the body, not `X-Timestamp`, so a captured delivery
could be replayed with a fresh header. A replay changes nothing: the workflow takes a verdict once, ignores a REVIEW for a
subject it is not waiting on, and the service is reached over TLS. Second, Didit signs every attempt, retries included,
with a fresh timestamp, so the five-minute window never rejects a genuine retry.

**How it was checked without an account.** `internal/kyc/testdata/gen_didit_vectors.py` is Didit's published reference
implementation, copied from its documentation. It generates over 180 vectors (nested objects, Unicode, control
characters, large and tiny numbers, whole floats such as `1.0` and `1e3`, escaping) with their signatures, and the Go
tests must reproduce every one. Breaking the Go canonicalisation in several ways (sort order, escaping, integers, floats,
`-0`) makes the tests fail, so they would notice drift. Against a local fake of Didit's API the tests cover session
creation and its failures, secret handling, the environment check, forged, altered, stale and unsigned deliveries, and
every status. `cmd/didit-verify` (below) is the live counterpart.

**What is not checked until Didit is reached:** that the real create-session reply has the shape the adapter reads
(`session_id` and `url`), that the real webhook is signed exactly as documented, that a `metadata` object is echoed in
webhooks, and how a KYB workflow behaves for South Africa.

**Didit cannot reach the build session**, so no live webhook can arrive there. The check that does not need a public
endpoint is below.

## First checks once access exists

Do these in order, and stop at the first surprise.

1. **Webhook signature, against what Didit really sent.** Didit records every delivery it makes, with the exact headers
   and body, even when the address could not be reached, and returns it from `GET /v3/session/{sessionId}/webhooks/` and
   `GET /v3/session/{sessionId}/webhook/{webhook_uuid}` (the connector tools `didit_session_webhooks`). Create a sandbox
   session, finish it with `didit_session_simulate`, fetch the recorded delivery, and run it through the verifier:

   ```
   cd services/platform/onboarding
   DIDIT_WEBHOOK_SECRET=<the destination secret> DIDIT_ENVIRONMENT=sandbox \
     go run ./cmd/didit-verify -file delivery.json
   ```

   Exit 0 means the signature verifies and the delivery is a verdict; 3 means it verifies and is not a verdict; 1 means
   it does not, and says which part. The command sets the clock to the delivery's own timestamp, so an old delivery is
   not rejected as stale, and it never prints the secret. If it fails, fix the canonical form in `kyc/didit_canon.go`
   before anything else. Didit's documentation lists `X-Signature-V2` (recommended), `X-Signature-Simple` and
   `X-Signature`; the adapter trusts only V2 and refuses a delivery that has none.
2. **Sandbox sessions through the service.** With the stack on `KYC_PROVIDER=didit` and `DIDIT_ENVIRONMENT=sandbox`, start
   an onboarding, open the returned link, and check that each outcome (approve, decline, review) lands as `APPROVED`,
   `DECLINED` or `REVIEW` in onboarding, with the review reaching the compliance queue. This needs the webhook destination
   to be reachable from the internet, so it waits for a staging deployment or a tunnel the owner provides.
3. **One live session** by the owner, with a real document, to see the real decision payload and the real cost.
4. **WorkOS sign-in on staging.** Sign in to each app, and read a real access token: its issuer, audience, signing keys,
   and which claims it carries. WorkOS tokens carry `sub`, `org_id`, `role` and `permissions` by default (*unverified*);
   custom claims come from a JWT template. Whether a template can emit an organisation's external ID is *unverified*.
   If it cannot, the apps map `org_id` to a party themselves.

## Mapping design (a proposal, unverified for WorkOS)

- **Customer.** A WorkOS organisation is a business. Its external ID is the company's party ID, and a JWT template adds
  `forge_party`. Directors are members of the organisation.
- **Staff.** A WorkOS organisation for staff, with roles `compliance` and `operations`, and a template that adds
  `forge_roles`. The back office already ignores roles it does not know.
- **Didit.** One session per subject, with the onboarding ID and the subject's party ID in `vendor_data` and `metadata`.
  Built as described above.

## Open product questions this raises

- **A person who walks away.** `Expired` and `Abandoned` are not verdicts, so the onboarding waits out its seven days and
  then declines. Whether to decline earlier, or to start a fresh session, is a product decision.
- **Didit's console and Forge's review.** If a Didit reviewer approves a session that Forge has put in its own review
  queue, the onboarding moves on and the Forge review stays open until someone closes it. Decide whether Didit's console
  is allowed to decide at all, or whether `In Review` must always be decided in the Forge back office.

## What is still missing around the providers

Choosing and connecting the providers does not build the customer's way in. As of 2026-11-13 the code has none of:

- a customer-facing application flow (no public onboarding endpoint and no apply page), and so nothing that hands the
  person their verification link;
- the `OnboardPerson` workflow for individuals (only `OnboardBusiness` exists);
- the Fineract client and product creation that the design lists as an onboarding step;
- a way to give the approved director a login (a WorkOS organisation and invitation created at activation).
