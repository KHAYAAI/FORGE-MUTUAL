# treasury-risk (services/bank/treasury-risk)

Entity: BANK. Architecture: ARCHITECTURE.md 6.2 and 13. v1 scope: "concentration and outflow alerts (tech deposits
are volatile)".

## What it watches

Every interval, for each monitored asset (ZAR on ledger 710, USD on ledger 840), it reads the ledger and computes:

| Alert | Holds when | Severity |
|---|---|---|
| `CONCENTRATION` | The largest customer account holds at least `TREASURY_CONCENTRATION_BPS` of deposits | WARNING |
| `NET_OUTFLOW` | Deposits fell by at least `TREASURY_OUTFLOW_BPS` against the snapshot about 24 hours earlier | WARNING |
| `LIQUIDITY` | Deposits exceed the bank-side pool balance (3003 or 5001). The books do not reconcile with the bank | CRITICAL |
| `SUSPENSE_AGE` | Suspense (2005) holds more than `TREASURY_SUSPENSE_MINOR` | WARNING |

The limit is inclusive. A bound is only ever compared in integer basis points, and the arithmetic cannot overflow.

## Alerts and events

An alert is **raised** once and **resolved** once. While it holds, its detail is updated quietly. Each transition
writes one `forge.bank.risk.alert.v1` outbox event (`RiskAlertRaised`) in the same transaction as the state change.
A threshold that stays crossed does not flood consumers.

## Limits, stated

- **Thresholds are placeholders** (VERIFY). The defaults are 25 percent concentration, 20 percent outflow in a day,
  and R10,000 of suspense. Treasury and counsel must set them. They are configuration, not code.
- **Outflow is a net measure.** It compares deposits between two snapshots. It cannot separate payments out from
  fees or reversals. Gross outflow needs the payment events, which are not consumed yet.
- **No relay runs in this binary.** Events are written to the outbox and stay there until a relay publishes them to
  the broker. The broker has never run (QUESTIONS.md). The binary logs this at start.
- **Read-only against the ledger.** It needs the ledger to allow `svc-treasury-risk` to call `ListAccounts` and
  `GetBalances`. That allow-list entry is a deployment step, not something this service can do.

## Configuration

| Variable | Meaning |
|---|---|
| `DATABASE_URL` | Postgres (required) |
| `LEDGER_URL` | Ledger gateway (required) |
| `TREASURY_INTERVAL` | Time between passes (default 15m) |
| `TREASURY_ENTITY` | Ledger entity (default BANK) |
| `TREASURY_CONCENTRATION_BPS`, `TREASURY_OUTFLOW_BPS`, `TREASURY_SUSPENSE_MINOR` | Thresholds (placeholders) |
| `TREASURY_ADDR` | Health listener (default `:8095`) |

## Tests

`make test-treasury`. The store tests need `TREASURY_TEST_DSN` and skip without it.
