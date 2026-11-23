# backoffice-api (services/shared/backoffice-api)

Entity: PLAT (staff tooling; it moves no money itself). Architecture: ARCHITECTURE.md section 6.6 (back office), section 15
(compliance cases). The contract is `openapi/backoffice.yaml`. The back-office web app generates its types from it.

Status: built and tested with fake downstream clients. Not yet run against the real services end to end. There is no
production staff authenticator (section 3), so the service does not start in production.

## 1. What it does

A view and a set of actions for staff. It keeps no state:

| Route | Role | Forwards to |
|---|---|---|
| `GET /v1/cases` | any | compliance-engine `ListCases` |
| `GET /v1/cases/{case}` | any | compliance-engine `GetCase` |
| `POST /v1/cases/{case}/close` | compliance | compliance-engine `CloseCase` |
| `GET /v1/payments/{payment}` | any | payments-orchestrator `GetPayment` |
| `POST /v1/payments/{payment}/approval` | operations | payments-orchestrator `ApprovePayment` |
| `POST /v1/payments/{payment}/repair` | operations | payments-orchestrator `ResolveRepair` |
| `GET /v1/recon/gate` | any | reconciliation `GetGate` |
| `GET /v1/recon/breaks` | any | reconciliation `ListBreaks` |
| `POST /v1/recon/breaks/{break}/investigate` | operations | reconciliation `InvestigateBreak` |

## 2. Rules

- **A role is checked before any downstream call.** A person without the role gets 403, and the service is not asked.
- **The acting person comes from the session.** A body that names an actor is refused with 400 (strict decoding). The name
  sent downstream is the signed-in staff member's.
- **Closing a case is a checker's act.** The compliance-engine store refuses a close by the person who opened the case, and
  refuses a note under 20 characters. The back office does not duplicate those rules; it passes the refusal on.
- **Payment approval and repair are recorded by the signed-in person.** Whether the approver may be the initiator is decided
  by payments-orchestrator, not here (VERIFY that it refuses a self-approval).
- **Investigating a break does not clear it.** Only a later conclusive run that matches can clear a break. The response shows
  the investigation next to the break, which stays open.
- **Overdue is computed when a case is read**, against its SLA. A case is never marked overdue in storage.

## 3. Authentication

Only `dev` is built. It reads `X-Forge-Staff` (the name) and `X-Forge-Roles` (a comma-separated list of `compliance` and
`operations`). An unknown role is refused, not ignored, so a typo cannot quietly withhold or grant access. A person with no
valid role has no session. Dev headers are refused when `FORGE_ENV=production`, and the service does not start without a
configured mode.

**Not built:** a production staff authenticator. Choosing it, and how roles are granted, is a decision for the owner (VERIFY).

## 4. Compliance cases API (compliance-engine)

This service's cases API is part of compliance-engine, not this module. It is `forge.cases.v1.CasesService`, served by
`services/shared/compliance-engine/cmd/compliance-engine`: `ListCases`, `GetCase`, `CloseCase`. It is tested against
Postgres in `make test-compliance`.

**Gap:** the monitoring rules have no event feed, so nothing calls the store that opens cases. Until a feed exists, the cases
list is empty in practice. The API is correct; it has nothing to show yet.

## 5. Configuration

| Variable | Required | Default | Meaning |
|---|---|---|---|
| `BACKOFFICE_AUTH` | yes | | `dev` (refused in production) |
| `FORGE_ENV` | no | `development` | `production` refuses dev auth |
| `CASES_URL` | yes | | compliance-engine |
| `PAYMENTS_URL` | yes | | payments-orchestrator |
| `RECON_URL` | yes | | reconciliation |
| `BACKOFFICE_ADDR` | no | `:8500` | Listen address; `/healthz` is open |

Each downstream must list `svc-backoffice-api` in its caller allowlist. The header is `X-Forge-Caller`. Payments must allow
`ApprovePayment` and `ResolveRepair` for it, and the cases service `ListCases`, `GetCase` and `CloseCase`.

## 6. Tests

`make test-backoffice` covers the routes and roles, with fake downstream clients: no session refused before any call; an
operations person refused a case close; a compliance person refused a payment decision; the closer taken from the session
and a body with an actor refused; the approver and operator forwarded; a downstream state refusal kept as a conflict; the
case list defaulting to open and bounding its limit; the gate's reasons passed through; breaks defaulting to open. A
mutation check (removing the compliance gate on close) makes the close test fail.

## 7. Not yet done

- **No KYC review.** The architecture puts KYC review in the back office. There is no onboarding review API to call, so the
  screen is not built (a gap in the service it depends on, not a choice).
- **No production authenticator** (section 3).
- **No event feed for the monitoring rules** (section 4), so cases are not opened yet.
- **Not run against the real services end to end.**
