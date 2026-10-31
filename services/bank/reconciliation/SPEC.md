# reconciliation (services/bank/reconciliation)

Entity: BANK. Architecture: ARCHITECTURE.md section 13. Invariant 9: a break alarms and blocks outbound payments
until it is resolved.

## What it does

Every run reconciles one monitored asset (ZAR against the sponsor, USD against the partner bank) and commits one
run record. A run:

1. Reads the ledger: every account of the asset's ledger, with posted debits and credits (`ListAccounts`, then
   `GetBalances`). It takes this snapshot twice, around the bank read, and compares digests.
2. Reads today's statement from the bank that holds the asset, through `BankLayer` (`StatementSource` for ZAR,
   `ForeignCurrency` for USD). The bank route is never read here.
3. If the two ledger snapshots differ, the books moved during the read and the run is **inconclusive**. It records
   that and changes no break. Three attempts, then inconclusive.
4. Otherwise it evaluates five checks (`internal/recon/checks.go`) and commits the run and its breaks in one
   transaction.

| Check | Holds when |
|---|---|
| `statement_arithmetic` | Closing equals opening plus inbound minus outbound, from the statement's own movements |
| `ledger_double_entry` | Debits minus credits over every account of the ledger is zero (invariant 1) |
| `unclassified_balance` | Every non-zero account carries a code that Appendix B classifies for the rail |
| `sponsor_equation` / `partner_equation` | Statement closing equals liabilities plus in-flight inbound minus in-flight outbound |
| `sponsor_pool_mirror` / `partner_pool_mirror` | The pool mirror (3003 for ZAR, 5001 for USD) equals statement closing adjusted for in-flight |

Liability codes for ZAR: 1001, 2001, 2002, 2005, 3001. For USD: 1001, 2001, 2002, 2005. Pool codes: 3003 (ZAR),
5001 (USD). A new account code that holds monitored money must be classified here, or it breaks
`unclassified_balance`. That is deliberate.

## The gate

`GetGate` is open only when, for every monitored asset, the latest conclusive run is no older than `RECON_MAX_AGE`,
every check in it matched, and no break is open. Anything else closes it: no run yet, a stale run, an inconclusive
latest run, a break, or an error reading the store. The gate fails closed, and payments treat an unreadable gate as
an error rather than "no break".

## Breaks

- A mismatch opens one break per (asset, check). A repeat mismatch updates that break; it does not add another.
- A later conclusive run that matches **clears** the break. The row stays in history.
- `InvestigateBreak` records a human's findings and an actor. It never changes the gate. A difference cannot be
  accepted or waved through: only a matching run clears it.

## Known limits (not hidden)

- **In-flight terms are zero.** No feed of unbooked inbound credits or sponsor debits is connected yet, so the
  equation and mirror checks assume none exist. Each of those checks says so in its detail. A payment that the
  bank settles before the ledger clears it will break a run, and the gate will stay closed until the ledger
  catches up. That is intended fail-closed behaviour, and it is covered by a test.
- **Reads are live, not as-of.** The ledger has no historical balances, so the run compares today's statement with
  today's books, and relies on the digest check to make the comparison valid.
- **Fineract GL is not checked.** The third leg of the three-way check needs core-bridge, which is not built.
- **Partner statements are a single USD partner.** The routing across partners is not built.

## Interfaces

ConnectRPC `forge.recon.v1.ReconService` (`proto/forge/recon/v1/recon.proto`):

| RPC | Caller | Effect |
|---|---|---|
| `GetGate` | `svc-payments` | Read the gate |
| `ListBreaks` | `svc-backoffice` | Read breaks |
| `InvestigateBreak` | `svc-backoffice` | Record findings; the actor is the caller identity |
| `RunRecon` | `svc-backoffice` | Run one asset now |

Callers are allow-listed in the file named by `RECON_CALLERS_FILE`. A call with no identity is `Unauthenticated`.

## Configuration

| Variable | Meaning |
|---|---|
| `DATABASE_URL` | Postgres for runs and breaks (required) |
| `LEDGER_URL` | Ledger gateway, for `svc-reconciliation` (required) |
| `RECON_CALLERS_FILE` | JSON allow-list (required) |
| `BANK_CONFIG` | Path to `config/banklayer.yaml` (default `config/banklayer.yaml`) |
| `RECON_INTERVAL` | Time between runs (default 15m) |
| `RECON_MAX_AGE` | How old a conclusive run may be before the gate closes (default 30m). Must exceed `RECON_INTERVAL` |
| `RECON_ADDR` | Listen address (default `:8093`) |

The bank's own variables (for example `SPONSOR_BASE_URL`) are named in `config/banklayer.yaml`.

## Tests

- `make test-recon`: arithmetic, the ledger reader (paging, digest, missing balances), the store, the runner (a payment
  cycle through the simulator, and an unstable read), and the service's caller allow-list.
- The store, runner and service tests need `RECON_TEST_DSN`, a throwaway database. Without it they skip, and
  `make test-recon` reports them as skipped.
