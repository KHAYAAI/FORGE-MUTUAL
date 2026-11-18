# fx-service (services/bank/fx-service)

Entity: BANK. v1: a customer converts ZAR to USD or USD to ZAR between two of their own accounts, at a quoted rate. The pair is
ZAR/USD only. Events are in ARCHITECTURE.md Appendix C; ledger transfer codes are in Appendix B.

## Quoting

- The rate is ZAR per USD, scaled by 10^8 and parsed from a decimal string, never a float. It comes from
  `FX_RATE_ZAR_PER_USD` with `FX_RATE_AS_OF`. **This is a static rate for development only.** A market-rate provider is not
  chosen (VERIFY). A rate older than `FX_RATE_MAX_AGE` (default one hour) is refused, so a quote is never built on a stale rate.
- The spread is `FX_SPREAD_BPS` (default 50) and the customer pays it, in the sell asset.
  - Buying USD: the customer's rate is mid x (1 + spread), rounded **up**, so each USD costs more ZAR.
  - Selling USD: the customer's rate is mid x (1 - spread), rounded **down**, so each USD returns less ZAR.
  - Rounding always favours the bank. Round trips never gain the customer money (a test checks this).
- A quote is valid for 60 seconds (`QuoteValidity`) and can be booked once. Booking takes the quote row with `FOR UPDATE`, so
  a second booking of the same quote is refused.

## Booking

- Both accounts must belong to the party. Ownership is read from consolidated-view (`GetBalances`). A foreign account is
  `PERMISSION_DENIED`, and it is checked before the quote is locked.
- The deal is recorded before any ledger write. A crash after that point leaves a deal that the resume loop finishes. The
  caller is told the state the deal reached.

## The saga

The sell asset and the buy asset sit on different ledgers, and no single transfer spans them. So the saga holds both legs,
posts the buy, then posts the sell. Every step has a fixed idempotency key (`<deal>:<leg>:<step>`), so a replay is a no-op.

| State | Meaning |
|---|---|
| `BOOKED` | Recorded. No ledger write yet. |
| `SELL_PENDING` | The sell leg is held. |
| `BUY_PENDING` | The buy leg is held as well. |
| `BUY_POSTED` | The buy moved the customer's money. The sell leg is still held. |
| `COMPLETED` | Both legs posted. `forge.bank.fx.booked.v1` is written to the outbox in the same transaction. |
| `VOIDED` | Failed before anything posted. The holds were released. |
| `REVERSED` | The sell was refused after the buy posted. The buy was reversed. |

- The sell hold moves the customer's sell asset to the FX pool (code 20, FX_LEG) and the spread to the fee account (code 21,
  FX_SPREAD). The buy hold moves the pool's buy asset to the customer.
- A **definitive** refusal (the ledger says no) voids the holds before anything posts, or reverses the buy (code 90, REVERSAL)
  after it. An **uncertain** step (a timeout or a lost response) leaves the deal non-final. Run again continues from the same
  place with the same keys.
- Pool (2003) and fee (2002) accounts per asset are ensured through `systemaccounts` on every run.

## One worker per deal

`Saga.Run` takes a Postgres advisory lock on the deal for the whole run. A booking and the resume loop, or two replicas, cannot
drive the same deal at once. The second worker gets `ErrBusy` and leaves the deal alone. `Book` then reports the state the deal
has reached, which is the state the first worker is moving through. A test holds the lock from a second connection and checks
that the run is refused and the deal does not move.

## Resume

Every `FX_RESUME_INTERVAL` (default 30s) the resume loop lists up to 100 non-final deals that have not been updated for one
interval, and runs each one. A deal that is busy is skipped until its owner releases it.

## Configuration

| Variable | Required | Default | Meaning |
|---|---|---|---|
| `DATABASE_URL` | yes | | Postgres for deals, quotes and the outbox |
| `LEDGER_URL` | yes | | Ledger gateway (ConnectRPC) |
| `CONSOLIDATED_URL` | yes | | consolidated-view, for ownership |
| `FX_CALLERS_FILE` | yes | | Allowlist of callers and methods (`callerauth`) |
| `FX_RATE_ZAR_PER_USD` | yes | | Decimal rate, e.g. `18.25` (static, development only) |
| `FX_RATE_AS_OF` | yes | | RFC 3339 time the rate was observed |
| `FX_RATE_MAX_AGE` | no | `1h` | Refuse a rate older than this |
| `FX_SPREAD_BPS` | no | `50` | Spread in basis points, paid by the customer |
| `FX_RESUME_INTERVAL` | no | `30s` | How often the resume loop runs |
| `FX_ADDR` | no | `:8100` | Listen address; `/healthz` and `/readyz` are also served |

## Tests

`make test-fx` runs the pricing and rate suites without a database. The saga and service suites need `FX_TEST_DSN`, a
throwaway database. They skip without it, and `make ci` refuses to pass without it.

## Not yet done

- The rate is static. A market-rate provider, and its licence, are not chosen (VERIFY).
- Only the ZAR/USD pair is priced.
- The booked event is not yet tested end to end through Kafka.
