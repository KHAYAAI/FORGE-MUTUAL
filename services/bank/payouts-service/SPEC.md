# payouts-service (services/bank/payouts-service)

Entity: BANK. Architecture: ARCHITECTURE.md 9.1 (`BulkPayout`), 6.2, and P2-03 (bulk payouts, partial failure, idempotent
batches). v1: domestic ZAR payouts from one customer account, in bulk, sent through payments.

Status: built and tested against PostgreSQL and in Temporal's in-process test environment. **Not yet run against a Temporal
server.** `go.temporal.io` is blocked by this environment's network policy, so the Temporal client and worker in `main` have
not been exercised against a real server (see *Not yet done*).

## 1. Interface

ConnectRPC `forge.payouts.v1.PayoutsService`: `CreateBatch`, `GetBatch`, `ApproveBatch`, `ResumeBatch`. Callers are
allowlisted by `PAYOUTS_CALLERS_FILE` (`callerauth`). Payments must allowlist `svc-payouts` for `CreatePayment` and
`GetPayment`.

## 2. Batch lifecycle

```
DRAFT --approve (checker)--> RUNNING, run 1
RUNNING --run ends, every item settled--> COMPLETED
RUNNING --run ends, some item open or failed--> COMPLETED_WITH_EXCEPTIONS
COMPLETED_WITH_EXCEPTIONS --resume--> RUNNING, run n+1
RUNNING --resume--> the current run is started again (no-op while it is still going)
```

## 3. Maker and checker

The creator is the maker. An approval by the maker is refused (`PERMISSION_DENIED`). The comparison ignores case and
surrounding spaces. The check runs in the same transaction that moves the batch to `RUNNING`, under a row lock, so an
approval and a resume cannot interleave. Approving a batch that is already running or finished changes nothing.

## 4. Money safety

- **One key per item.** Each item is sent under `payout:<batch>:<item>`. Payments derives the payment from that key, so an
  item always maps to one payment. A retry or a resume is a replay, never a second payment.
- **A refusal is final.** Payments refusing the request (`INVALID_ARGUMENT`, `ALREADY_EXISTS`, `FAILED_PRECONDITION`,
  `PERMISSION_DENIED`, `NOT_FOUND`) marks the item `FAILED`. It is never sent again.
- **An unknown outcome is retried, then parked.** Any other error (unreachable, timeout, server error) is retried by
  Temporal, five attempts with backoff. If it is still unknown, the item becomes `UNCERTAIN`. A resume sends it again under
  the same key.
- **A sent item is never sent again.** It is followed through `GetPayment`. `COMPLETED` settles it. `DECLINED`, `FAILED`
  and `CANCELLED` fail it. Any other state (running, reserved, awaiting approval, repair required) leaves it `SENT`.
- **Final states stay final.** Store updates move only items that are `PENDING`, `SENT` or `UNCERTAIN`. A settled or
  failed item cannot move back (a test checks this).
- **One item cannot fail the batch.** Each item is a child workflow that always returns a result, and the batch's state is
  read from the store at the end of the run.

## 5. Concurrency and polling

Items run in groups of `PAYOUTS_CONCURRENCY` (default 10). A group must finish before the next one starts. An item polls
up to `PAYOUTS_MAX_POLLS` times (default 120) at `PAYOUTS_POLL_INTERVAL` (default 30s), so about an hour. A payment still in
progress after that stays `SENT`, and a resume follows it again.

Limitation: because a group waits for its slowest item, one slow payment holds up its group. A sliding window is the fix; it
is not built yet.

## 6. Validation and ownership

- The source account must belong to the party (consolidated-view `GetBalances`) and hold ZAR. This is checked at creation.
- Asset must be ZAR. Payments accepts ZAR only in this version.
- Limits are placeholders (`docs/THRESHOLDS.md`, rows 14 to 16): up to `PAYOUTS_MAX_ITEMS` items (default 500), each up to
  `PAYOUTS_MAX_ITEM_MINOR` minor units (default 100,000,000, that is R1,000,000).
- Item key: 1 to 64 letters, digits, dots, dashes or underscores, unique in the batch.
- Account number: 4 to 34 letters or digits. Bank code: 1 to 11 letters or digits. Both are placeholders until the rail's
  format is confirmed (VERIFY).
- Beneficiary name and reference are required (up to 140 characters each).

## 7. Workflow ids

- A run is `bulk-payout:<batch>:<run>`. Starting a run that is already started is not an error.
- An item is `payout-item:<batch>:<run>:<item>`. The run number keeps a later run from colliding with an earlier one.
- Workflow inputs and results carry item keys and states only. Beneficiary details never enter the Temporal history.

## 8. Configuration

| Variable | Required | Default | Meaning |
|---|---|---|---|
| `DATABASE_URL` | yes | | Postgres for batches and items |
| `PAYMENTS_URL` | yes | | payments-orchestrator (ConnectRPC) |
| `CONSOLIDATED_URL` | yes | | consolidated-view, for ownership |
| `PAYOUTS_CALLERS_FILE` | yes | | Allowlist of callers and methods |
| `TEMPORAL_HOST` | no | `localhost:7233` | Temporal frontend |
| `TEMPORAL_NAMESPACE` | no | `default` | Temporal namespace |
| `TEMPORAL_TASK_QUEUE` | no | `payouts` | Task queue for the workflows and activities |
| `PAYOUTS_CONCURRENCY` | no | `10` | Items in flight at once per batch |
| `PAYOUTS_POLL_INTERVAL` | no | `30s` | Wait between payment polls |
| `PAYOUTS_MAX_POLLS` | no | `120` | Polls before an item stays sent |
| `PAYOUTS_MAX_ITEMS` | no | `500` | Items per batch (placeholder) |
| `PAYOUTS_MAX_ITEM_MINOR` | no | `100000000` | Per-item limit in minor units (placeholder) |
| `PAYOUTS_ADDR` | no | `:8200` | Listen address; `/healthz` and `/readyz` are also served |

## 9. Tests

`make test-payouts` runs every suite. The activity and service suites need `PAYOUTS_TEST_DSN`, a throwaway database, and
skip without it. `make ci` refuses to pass without it. The workflow suite runs in Temporal's in-process test environment, with
the activities mocked. It needs no server.

## 10. Not yet done

- **Not run against a Temporal server.** `go.temporal.io` is blocked by this environment's network policy. The client,
  worker and the `StartRun` path have only been compiled, not exercised.
- **Beneficiary account numbers are stored in plaintext** in the payouts store. Payments' treatment should be aligned, and
  sealing added if payments seals them (VERIFY).
- **No batch events.** The service publishes no lifecycle events, so notifications for payouts are not wired.
- **Approver identity is the caller's `actor` field.** Authenticating the actor is the job of the caller (workspace-api and
  backoffice), which are not built yet.
- **Ownership is checked only at creation.** Payments does not appear to check ownership itself (VERIFY). A later change of
  ownership is not re-checked before items are sent.
- **No batch-total limit.** Only the item count and the per-item limit are enforced.
- **Group waiting** (section 5).
