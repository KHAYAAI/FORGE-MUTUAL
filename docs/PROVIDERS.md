# Identity and KYC providers

Decided by the owner on 2026-11-03 (see `docs/DECISIONS.md`):

- **Sign-in: WorkOS (AuthKit).**
- **KYC and KYB: Didit.**
- Both in **new accounts the owner creates**, on a personal email address. Not A-CARD.

This page says where each stands, what only the owner can do, what Didit offers for South Africa, and what must be
checked first once the accounts exist. Facts marked *unverified* come from public search summaries of the providers'
documentation, because the documentation hosts could not be reached from the build session.

## Where things stand

| Piece | State |
| --- | --- |
| WorkOS account for this project | Not created. |
| Didit organisation for this project | Not created. |
| Sign-in in both apps | Built and tested against the development identity provider (`tools/devidp`). Not run against WorkOS. |
| Claims the apps read | `forge_party` (customer) and `forge_roles` (staff). Whether WorkOS can issue them is *unverified* (see below). |
| Didit adapter in onboarding | A stub. It refuses every verification. The stack uses the fake provider. |
| Didit webhook verification | Not written. The public sources disagree on the header and the signing string (see below). |
| Hosts blocked from the build session | `api.workos.com`, `workos.com`, `verification.didit.me`, `apx.didit.me`, `docs.didit.me`. |

The Didit and WorkOS connectors in the build session are signed in to **A-CARD**. They must not be used to create or
change anything for this project.

## What only the owner can do, in order

1. **Allow the provider hosts** in the cloud environment's network settings (the environment menu in the session title
   bar, then Edit, then Allowed domains, with *Allow package managers* left ticked). Add:
   `verification.didit.me`, `apx.didit.me`, `docs.didit.me`, `api.workos.com`, `workos.com`, and the project's AuthKit
   domain once WorkOS shows it. Steps: <https://code.claude.com/docs/en/cloud-environments#network-access>.
2. **Create the Didit account** with the personal email. Either sign up in the Didit business console, or let the build
   session call Didit's programmatic registration (*unverified*: `POST /auth/v2/programmatic/register/` on
   `apx.didit.me`, then `.../verify-email/` with the code emailed to the owner). The code goes only to the owner, who
   passes it on. Signing up accepts Didit's terms, which is the owner's act.
3. **Create the WorkOS account** with the same email, and a project for this system with a staging environment.
4. **Make each account reachable by the build session without using A-CARD.** Either invite the session's integration
   login as an admin of the new Didit organisation and the new WorkOS team, or reconnect the connectors to the new
   account. Check that the new organisation and project appear in the connector's own listing before anything is created.
5. **Hand over the secrets through the environment's secrets setting, not in chat**: for Didit an API key, a webhook
   secret, and the identifiers of one KYC workflow and one KYB workflow; for WorkOS the client ID, an API key, and the
   AuthKit issuer. They go in `deploy/compose/.env` for the local stack (never committed).

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

## Open questions for compliance and legal

This repository does not answer these.

- Does this combination meet the bank's customer-identification duties under the FIC Act and its risk management and
  compliance programme? Is a Home Affairs lookup required for every customer, or only for some tiers?
- POPIA: the lawful basis and consent for processing biometrics; whether personal information leaves South Africa and
  under what safeguards; the operator agreement with Didit; retention against FIC Act record-keeping.
- Which lists does Didit's AML screening use, compared with the FIC targeted financial sanctions list? It covers
  onboarding screening. Screening beneficiaries on payments is a separate, still undecided matter (`docs/SANCTIONS_SOURCES.md`).
- What happens when Didit is down. Onboarding must fail closed: no verdict, no approval.

## First checks once access exists

Do these in order, and stop at the first surprise.

1. **Webhook signature.** Send a test webhook from the Didit console and read what arrives. Didit's documentation (as
   summarised) lists `X-Signature-V2` (recommended), `X-Signature-Simple` and `X-Signature`; Didit blog posts describe
   `X-Didit-Signature` with `sha256=` over a timestamp, a dot, and the raw body. One of these is out of date. Write the
   verifier only against what arrives, and reject stale timestamps.
2. **Sandbox sessions.** Create sessions with the sandbox scenarios (approve, a decline, a review) and check each lands
   as `APPROVED`, `DECLINED` or `REVIEW` in onboarding, with the review reaching the compliance queue.
3. **One live session** by the owner, with a real document, to see the real decision payload and the real cost.
4. **WorkOS sign-in on staging.** Sign in to each app, and read a real access token: its issuer, audience, signing keys,
   and which claims it carries. WorkOS tokens carry `sub`, `org_id`, `role` and `permissions` by default (*unverified*);
   custom claims come from a JWT template. Whether a template can emit an organisation's external ID is *unverified*.
   If it cannot, the apps map `org_id` to a party themselves.

## Mapping design (a proposal, unverified)

- **Customer.** A WorkOS organisation is a business. Its external ID is the company's party ID, and a JWT template adds
  `forge_party`. Directors are members of the organisation.
- **Staff.** A WorkOS organisation for staff, with roles `compliance` and `operations`, and a template that adds
  `forge_roles`. The back office already ignores roles it does not know.
- **Didit.** One session per subject, with the onboarding ID and the subject's party ID in `vendor_data`. Didit's
  `Approved`, `Declined` and `In Review` become `APPROVED`, `DECLINED` and `REVIEW`. Names *unverified*.

## What is still missing around the providers

Choosing and connecting the providers does not build the customer's way in. As of 2026-11-03 the code has none of:

- a customer-facing application flow (no public onboarding endpoint and no apply page);
- the `OnboardPerson` workflow for individuals (only `OnboardBusiness` exists);
- the Fineract client and product creation that the design lists as an onboarding step;
- a way to give the approved director a login (a WorkOS organisation and invitation created at activation).
