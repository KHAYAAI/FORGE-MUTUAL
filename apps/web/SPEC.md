# web (apps/web)

Entity: the customer workspace. Architecture: ARCHITECTURE.md section 6.6 (Apps) and P1-10 ("every balance shows entity and
insurance status"). Next.js (App Router), TypeScript, Tailwind. The API types are generated from
`services/platform/workspace-api/openapi/workspace.yaml`.

Status: built. The journeys pass against the production build with the workspace API mocked in the browser. Not yet tested
against a running workspace-api, and development sign-in is the only sign-in (see section 6).

## 1. Screens

| Route | What the customer does |
|---|---|
| `/sign-in` | Development sign-in: a party ID and a name. Sets the session cookie. |
| `/accounts` | Sees every balance, with the holding entity and whether it is insured. |
| `/accounts/[id]` | Reads an account's activity, a page at a time. |
| `/payments/new` | Sends a domestic ZAR payment from a ZAR account. |
| `/payments/[id]` | Follows a payment's state. |
| `/fx` | Gets a quote, reads the disclosure, accepts it, and books. |
| `/payouts/new` | Creates a bulk payout batch as the maker. |
| `/payouts/[id]` | Sees a batch. Approves it as the checker, when it is not the maker's own. |

## 2. Invariants the UI enforces

- **Every balance names its holding entity and its insurance status.** The accounts page shows both on each balance. A
  test checks this.
- **Money is integer minor units, formatted with BigInt.** A float never touches an amount. A test checks a balance with
  an odd number of cents.
- **A quote's disclosure is shown before it can be booked.** Booking needs the checkbox, and the disclosure text is the
  API's own.
- **An expired quote cannot be booked.** The page shows that the quote expired and offers no booking. The server also
  refuses an expired quote.
- **One idempotency key per form.** A payment, a payout batch, and a double click all send the same key, so the money
  moves once. The key is per page load.
- **Maker and checker.** The approve button sits on a batch that is waiting for approval. When the API refuses the checker
  because they are the maker, the page shows the API's message.
- **Amounts are checked before anything is sent.** A zero, a negative, more than two decimals, or text is refused without a
  request.

## 3. How identity reaches the API

The browser never holds the identity headers. The session is an httpOnly, SameSite=strict cookie set by `/api/session`. The
route handler `/api/v1/[...path]` reads it, and forwards the request to workspace-api with `X-Forge-Party` and
`X-Forge-Actor`. Only `content-type` and `idempotency-key` are copied from the page, so a page cannot set the identity
headers. The `secure` flag is set in production.

## 4. Tests

`make test-web` runs the typecheck, a production build, and the Playwright suite (14 journeys, Chromium). The API is mocked at
the browser (`page.route`), so the journeys need no backend. They run the real sign-in page and the real `/api/session` route,
which sets the session cookie, and the real pages. They do not run the `/api/v1` proxy, because the mock intercepts those
calls first (see section 6). The suite never skips: a missing browser fails it.

The Playwright version is pinned to the one that matches the pre-installed Chromium in this environment. `CHROMIUM_PATH` may
point at another Chromium.

## 5. Dependencies

Pinned exactly, not with a range. Versions were chosen to be at least two weeks old at the time of writing:
`next` 16.4.1, `react` and `react-dom` 19.3.1, `openapi-fetch` 0.17.0, `typescript` 5.9.3, `tailwindcss` and
`@tailwindcss/postcss` 4.3.3, `openapi-typescript` 7.13.0, `@playwright/test` 1.63.0, `@types/react` 19.2.14,
`@types/react-dom` 19.2.3, `@types/node` 22.19.1, `postcss` 8.5.6.

## 6. Not yet done

- **No production sign-in.** `/sign-in` is for development. It is visible in production builds too, and must be removed or
  gated when a real identity provider is chosen (VERIFY).
- **Not tested against a running workspace-api.** The journeys mock it. An end-to-end run needs the real services.
- **The proxy's header handling is not tested with a live upstream.** It is covered by review. A test needs a fake
  upstream server.
- **No batch list.** workspace-api has no route that lists a party's batches, so the page can open a batch only by its ID.
  The customer cannot find a batch they did not just create.
- **The payment and payout forms do not reset after success.** Sending another needs a reload, which gives a new key.
- **No accessibility audit, no security headers (CSP), no not-found page.**
- **Amounts shown in both currencies only in the quote.** Balances show one currency each; a combined view is not built.
