# consolidated-view (services/platform/consolidated-view)

Entity: PLAT (the read model is shared by the customer workspace and back office). Architecture: ARCHITECTURE.md 6.1, 7.9.
Invariant 10: every balance shown to a customer names its holding entity and its deposit-insurance status.

## What it does

- **Projection.** Consumes `forge.bank.ledger.transfer.committed.v1` through `pkg/eventbus`. Each committed transfer adds to
  the debit and credit account balances and writes one activity row per account. A pending transfer is a hold and is not
  posted, so it changes nothing. Each transfer counts once per account, even if it arrives twice.
- **Reconciliation.** Every `CONSOLIDATED_RECON_INTERVAL` (default 15 minutes) it lists every account in the entity and
  reads the ledger's posted balances. The ledger is authoritative: each balance is set to the ledger's figure. A difference
  is counted as drift. The owner, product and insured status of every account are taken from the ledger's directory.
  A run that cannot read the ledger in full records nothing.
- **Ownership.** An account is visible only to its owner, as named by the last reconciliation. An account no reconciliation
  has claimed is invisible to every party (fail closed). Reading another party's account returns the same answer as a
  missing account.

## Interfaces

ConnectRPC `forge.consolidated.v1.ConsolidatedViewService`:

| RPC | Effect |
|---|---|
| `GetBalances(party_id)` | The party's accounts: available, holding entity, insured, product, as-of and source |
| `GetActivity(party_id, account_id, after_seq, limit)` | Movements on one owned account, paged |
| `GetReconStatus()` | When the last reconciliation ran, and how many accounts drifted |

Callers are allow-listed through `CONSOLIDATED_CALLERS_FILE`. The workspace API is the only caller that should read
balances on a customer's behalf, and it passes the party it has authenticated.

## Known limits (stated, not hidden)

- **Pending holds are not reflected until the next reconciliation.** The projection shows posted balances. A customer
  with a payment on hold sees the balance before the hold. Projecting hold events is the next step; until then, the
  `as_of` and `source` fields show how fresh a figure is.
- **Newly opened accounts are invisible until reconciled.** The owner is set by reconciliation, so an account can be
  invisible for up to one interval after it is opened.
- **Drift is counted and corrected, not alerted.** The count is recorded; an alert to the treasury and operations
  feeds is not yet wired.

## Configuration

| Variable | Meaning |
|---|---|
| `DATABASE_URL` | Postgres for the read model and the processed-event table (required) |
| `KAFKA_BROKERS` | Broker list (required) |
| `LEDGER_URL` | Ledger gateway, for reconciliation as `svc-consolidated-view` (required) |
| `CONSOLIDATED_CALLERS_FILE` | JSON allow-list (required) |
| `CONSOLIDATED_RECON_INTERVAL` | Time between reconciliations (default 15m) |
| `CONSOLIDATED_ENTITY` | Ledger entity (default BANK) |
| `CONSOLIDATED_ADDR` | Listen address (default `:8096`) |

The ledger must allow `svc-consolidated-view` to call `ListAccounts` and `GetBalances`.

## Tests

`make test-consolidated`. Store and reconciler tests need `CONSOLIDATED_TEST_DSN`. The projection test also needs
`KAFKA_TEST_BROKERS` and runs the path from a ledger event through the broker into the read model.
