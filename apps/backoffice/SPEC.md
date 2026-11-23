# backoffice (apps/backoffice)

Entity: staff tooling. Architecture: ARCHITECTURE.md section 6.6 (back office: KYC review, cases, payment repair, reconciliation
dashboard, maker-checker). Next.js (App Router), TypeScript, Tailwind. The API types are generated from
`services/shared/backoffice-api/openapi/backoffice.yaml`.

Status: built, with four screens. The journeys pass against the production build with the API mocked in the browser. Not yet
run against the real backoffice-api, and development sign-in is the only sign-in (section 6). KYC review is not built (section 6).

## 1. Screens

| Route | Who | What |
|---|---|---|
| `/sign-in` | anyone | Development sign-in: a name and at least one role. |
| `/cases` | any role | Compliance cases, oldest SLA first, with overdue flagged. Closed cases on request. |
| `/cases/[id]` | any role to read; compliance to close | A case's rule, evidence and status. A compliance person closes it with a note of at least 20 characters. |
| `/payments/lookup` | any role | Finds a payment by its ID. |
| `/payments/[id]` | any role to read; operations to decide | A payment's state. Approve or refuse one awaiting approval; record a repair when the rail's outcome is unknown. |
| `/recon` | any role to read; operations to investigate | The payments gate with its reasons, and the breaks. An investigation is recorded against a break. |

## 2. Rules the UI shows, and the server enforces

- **The UI hides an action the role does not allow, and says why.** The server checks the role again on every call, so hiding
  is never the only protection. A test checks that a compliance person sees no decision buttons, and an operations person sees
  no close form.
- **The person who opened a case is not offered the close button.** They are told another compliance person must close it. The
  case store refuses it too.
- **An investigation does not clear a break.** The page says so, and the break stays open until a later matching run clears it.
- **A repair records what the rail confirms.** The two outcomes are stated as facts the rail gave, not a guess.
- **A refusal from the server is shown as the server gave it.** For example, "the person who opened a case cannot close it".

## 3. How identity reaches the API

The browser never holds the identity headers. The staff session is an httpOnly, SameSite=strict cookie set by `/api/session`,
holding a name and roles. The route handler `/api/v1/[...path]` reads it and forwards `X-Forge-Staff` and `X-Forge-Roles` to
backoffice-api. Only `content-type` is copied from the page, so a page cannot set the identity headers.

When backoffice-api cannot be reached, the proxy answers 503 with a message the page shows, rather than failing silently.

## 4. Tests

`make test-backoffice-web` runs the typecheck, a production build, and 16 Playwright journeys (Chromium). The API is mocked in
the browser, except one journey that runs with no mock: the proxy points at a closed port, so the unreachable path is tested for
real. The suite never skips.

## 5. Dependencies

The same pinned versions as `apps/web` (see that SPEC, section 5).

## 6. Not yet done

- **No KYC review screen.** The architecture puts KYC review in the back office. The onboarding service has no review API for
  it to call (a gap in the service, not a choice).
- **No production sign-in.** `/sign-in` is for development, and is visible in production builds. It must be removed or gated
  when a real identity provider is chosen, and roles must come from it (VERIFY).
- **Not tested against a running backoffice-api.** The journeys mock it.
- **No case list is populated in practice yet.** The monitoring rules have no event feed, so no cases are opened (see
  backoffice-api SPEC, section 4).
- **Payment lookup is by ID only.** There is no list of payments awaiting approval or repair, because the payments API has no
  list. A staff member needs the ID.
- **No security headers (CSP), no accessibility audit, no not-found page.**
