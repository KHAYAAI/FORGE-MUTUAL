# partner-bank-router (services/bank/partner-bank-router)

Entity: BANK. Architecture: ARCHITECTURE.md 12.3. v1: a USD account through one partner bank. The router is built for more
than one partner, because partner banks can be lost (the Mercury lesson in the architecture).

## How a partner is chosen

- Candidates are the partners the bank layer offers for the asset. Costs come from `PARTNER_COSTS_BPS`.
- Health is read from the last 50 outcomes of each partner. Three consecutive failures make a partner **down**. A success
  rate under 50% is down; under 90% is degraded. With fewer than five outcomes a partner is healthy: there is no evidence
  against it yet.
- A down partner is never chosen. A healthy partner beats a cheaper degraded one. Among equals the cheapest wins, with a
  stable tie-break.
- With no partner for the asset, the operation is refused (`FAILED_PRECONDITION`). It is never sent to another asset's
  partner.

## Payments stay with their partner (the rule that matters)

A payout is pinned to the partner it is first sent to, **before** the request leaves. A retry or a status check goes to the
pinned partner, even when that partner is down. The reason: a timeout does not tell us whether the partner paid. If the
router sent the retry to another partner, the same money could be paid twice.

A test plants the case: the partner takes the payment and the response is lost. The router keeps the pin, the status check
is answered by that partner, and the other partner never sees the payment.

## What counts as a failure

A refusal of the request itself (`ErrInvalid`, `ErrConflict`, `ErrNotFound`) is not a partner failure: the partner is working
and the request was wrong. Anything else (unreachable, timeout, server error) counts against the partner.

## Interfaces

ConnectRPC `forge.partner.v1.PartnerRouterService`: `OpenAccount`, `Payout`, `PayoutStatus`, `GetHealth`. Callers are
allow-listed through `PARTNER_CALLERS_FILE`.

## Limits, stated

- **One partner is configured in v1.** The failover logic is tested with two simulated partners. Real second-partner
  onboarding is a commercial decision (VERIFY-PARTNER).
- **Health comes from traffic, not from probes.** A partner that is down and receives no traffic is not marked healthy
  again until traffic returns; a probe loop is not built.
- **Thresholds are placeholders** (VERIFY): three consecutive failures, 50% and 90%, and the five-outcome minimum.
- **Customer account references** are returned to the caller, which must register them with the inbound-credit store. The
  registration call is not yet wired, so an inbound credit to a new USD account goes to suspense until it is.
- **Exchange control** for South African residents holding foreign-currency accounts applies (VERIFY with counsel).

## Configuration

| Variable | Meaning |
|---|---|
| `DATABASE_URL` | Postgres for outcomes and pins (required) |
| `BANK_CONFIG` | Path to `config/banklayer.yaml` (default) |
| `PARTNER_COSTS_BPS` | Cost per partner, `name=bps,name=bps` (default 0 for each) |
| `PARTNER_CALLERS_FILE` | JSON allow-list (required) |
| `PARTNER_ROUTER_ADDR` | Listen address (default `:8099`) |

## Tests

`make test-partner-router`. Database tests need `PARTNER_ROUTER_TEST_DSN`.
