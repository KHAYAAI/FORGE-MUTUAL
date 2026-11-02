# workspace-api (services/platform/workspace-api)

Entity: PLAT. Architecture: ARCHITECTURE.md section 6 (`workspace-api`: backend-for-frontend, REST with OpenAPI, Postgres
session-free, reads views). The contract is `openapi/workspace.yaml`. The web app generates its types from it.

Status: built and tested with fake downstream clients. Not yet run against the real services end to end. There is no
production authenticator (see section 3), so the service does not start in production.

## 1. What it does

It is a view for the signed-in party. It keeps no state and moves no money. Each route forwards to the service that owns
the data:

| Route | Forwards to |
|---|---|
| `GET /v1/accounts` | consolidated-view `GetBalances` |
| `GET /v1/accounts/{account}/activity` | consolidated-view `GetActivity` |
| `POST /v1/payments` | payments-orchestrator `CreatePayment` (after an ownership check) |
| `GET /v1/payments/{payment}` | payments-orchestrator `GetPayment` |
| `POST /v1/fx/quotes`, `POST /v1/fx/deals`, `GET /v1/fx/deals/{deal}` | fx-service `Quote`, `Book`, `GetDeal` |
| `POST /v1/payout-batches`, `GET .../{batch}`, `POST .../{batch}/approve` | payouts-service `CreateBatch`, `GetBatch`, `ApproveBatch` |

## 2. Rules

- **The party comes from the session, never from the request.** A request body is decoded strictly. A `party_id` in a body
  is refused with 400 and never used.
- **Idempotency is scoped to the party.** The client's `Idempotency-Key` is sent downstream as `<party>:<key>`, so two
  customers using the same key cannot collide. Payments and payouts require the header.
- **The actor comes from the session.** It is the maker on batch creation and the checker on approval. The approve route
  takes no body, so a client cannot name another approver.
- **Ownership is checked before a payment is sent.** The source account must belong to the session's party and hold the
  requested asset. Downstream services check again where they can.
- **Errors keep their meaning.** Connect codes map to HTTP: invalid_argument 400, permission_denied 403, not_found 404,
  already_exists and failed_precondition 409, unavailable 503. A downstream message is passed through, so each service
  must keep its messages free of internal detail.
- **Money is a string of integer minor units**, as in the downstream services.

## 3. Authentication

Only one authenticator is built: `dev`, which trusts the `X-Forge-Party` and `X-Forge-Actor` headers. It refuses to run
when `FORGE_ENV=production`. `WORKSPACE_AUTH` is required; an unknown or empty mode stops the service at start.

**Not built:** a production session authenticator. Until one exists, the service cannot run in production. Choosing and
building it is a decision for the owner (VERIFY).

## 4. Configuration

| Variable | Required | Default | Meaning |
|---|---|---|---|
| `WORKSPACE_AUTH` | yes | | `dev` (refused in production) |
| `FORGE_ENV` | no | `development` | `production` refuses dev auth |
| `CONSOLIDATED_URL` | yes | | consolidated-view |
| `PAYMENTS_URL` | yes | | payments-orchestrator |
| `FX_URL` | yes | | fx-service |
| `PAYOUTS_URL` | yes | | payouts-service |
| `WORKSPACE_ADDR` | no | `:8300` | Listen address; `/healthz` is open |

Each downstream service must list `svc-workspace` in its caller allowlist. The header is `X-Forge-Caller`.

## 5. Tests

`make test-workspace`. The tests use fake downstream clients, so each one checks exactly what was forwarded. The tests
cover: no session refused; accounts read for the session's party only; body party refused; idempotency required; payment
from another party's account refused before payments is asked; wrong asset refused; session party, actor and scoped key
forwarded; downstream codes kept; activity bounds; maker and checker taken from the session; FX book uses the session party;
strict body decoding; dev auth refused in production.

## 6. Not yet done

- No production authenticator (section 3).
- No CORS policy. The web app is intended to reach the API through its own origin (a rewrite), so none is needed yet.
- No rate limiting, and no request logging or tracing beyond the downstream services.
- The routes are not yet exercised end to end against the real services.
- Downstream allowlists for `svc-workspace` are configuration, not yet written into the services' deployment files.
