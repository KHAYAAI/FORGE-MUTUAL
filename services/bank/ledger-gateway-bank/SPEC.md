# SPEC: ledger-gateway-bank

Status: draft for human review (ARCHITECTURE.md section 0, rule 3).
Entity: BANK. Sections referenced: ARCHITECTURE.md 2.2, 5, 7, 18.2, Appendix B.

## 1. Purpose

The only writer to the bank TigerBeetle cluster (`tb-bank`) (invariant 6). It accepts money
commands from services in the BANK entity, enforces the ledger rules, makes every command
idempotent, and publishes a committed-transfer event for each ledger write through an outbox.

It holds no business logic. Policy decisions (limits, sectors, step-up) happen before a call
reaches the gateway (ARCHITECTURE.md 7.7, "no business logic beyond enforcement").

## 2. Interface

ConnectRPC service `forge.ledger.v1.LedgerService` defined in `proto/forge/ledger/v1/ledger.proto`.
Over HTTP/1.1 and HTTP/2, JSON or binary (ConnectRPC).

| RPC | Mutates | Idempotency key |
|---|---|---|
| CreateAccounts | yes | required |
| CreateTransfers | yes | required |
| PostPending | yes | required |
| VoidPending | yes | required |
| GetBalances | no | n/a |
| LookupTransfers | no | n/a |

Money is always a decimal string of unsigned or signed integer minor units. Never a JSON number.

## 3. Enforcement (checked in this order, deny by default)

1. **Caller allow-list.** Each RPC is allowed only for callers listed in config. Unknown caller: `PERMISSION_DENIED`.
   Caller identity comes from the authenticated workload identity (mTLS SPIFFE ID in production;
   the `X-Forge-Caller` header in local development only).
2. **Entity match.** `entity_code` must equal the gateway's configured entity (`BANK`). Otherwise `ENTITY_MISMATCH`.
3. **Asset registry and cluster allow-list.** Asset must be in `pkg/money` registry and allowed for tb-bank
   (ZAR, USD in v1). Otherwise `ASSET_NOT_ALLOWED`. The ledger number is derived from the asset (ZAR 710, USD 840).
4. **Account codes and transfer codes.** Must belong to the Appendix B sets for tb-bank. Otherwise `CODE_NOT_ALLOWED`.
5. **Account directory.** Every account in a transfer must exist in the directory, belong to entity BANK,
   and hold the same asset. Otherwise `ACCOUNT_NOT_FOUND` or `ENTITY_MISMATCH`. No transfer spans two entities (invariant 7).
6. **Amount sanity.** Each leg amount must be > 0 and <= the configured per-asset maximum. Otherwise `AMOUNT_LIMIT_EXCEEDED`.
7. **Pending timeout.** Must be <= the configured maximum (default 7 days). Otherwise `TIMEOUT_LIMIT_EXCEEDED`.

## 4. Ledger semantics (tb-bank)

- Each transfer moves value between two accounts on the same ledger (one ledger per asset).
- Customer accounts carry `debits_must_not_exceed_credits`, so a debit that would make the balance negative is rejected
  with `INSUFFICIENT_FUNDS` (invariant 2).
- A multi-leg request is submitted as one TigerBeetle linked chain. If any leg fails, no leg is applied (invariant 3).
- Pending transfers reserve funds. Post and void reference pending transfer IDs. A pending transfer that is not
  posted or voided before its timeout expires automatically.
- Post applies the full pending amount in this version. Partial post is out of scope.

## 5. Idempotency (ARCHITECTURE.md 7.6)

- Record key: `(caller, rpc, idempotency_key)`. Stored in Postgres `idempotency_record` with a SHA-256 hash of the canonical request.
- Before any TigerBeetle call, the gateway writes the record as `PENDING` with all transfer IDs generated and stored.
- Same key, same hash, status `COMMITTED`: return the stored result with `STATUS_REPLAYED`. No money moves.
- Same key, same hash, status `PENDING` (a previous attempt crashed): resubmit with the stored IDs. TigerBeetle
  returns `exists` for legs already applied, which counts as success when fields match.
- Same key, different hash: `IDEMPOTENCY_CONFLICT` (Connect code `AlreadyExists`).
- Rejected requests are recorded as `COMMITTED` with a rejection result, so a replay returns the same rejection.
- Records are kept at least 30 days (purge job, outside this scope).

## 6. Events and outbox (ARCHITECTURE.md 7.8)

- In the same Postgres transaction that marks the idempotency record `COMMITTED`, the gateway inserts one `outbox` row per
  committed transfer with topic `forge.bank.ledger.transfer.committed.v1`.
- A relay publishes unpublished rows to Kafka (at-least-once) and marks them published. Consumers dedupe by `event_id`.
- Payload: `EventEnvelope` (ARCHITECTURE.md 18.2) with the transfer details as a Protobuf-encoded body.

## 7. Money-path rules (ARCHITECTURE.md 23.4 checklist)

- Integers only. No float anywhere in the path.
- Every mutating RPC has a replay test and a failure-injection test.
- Each TigerBeetle write is idempotent by transfer ID, so a crash at any step is recoverable.

## 8. Acceptance (from ARCHITECTURE.md section 25)

| Criterion | Status | Evidence |
|---|---|---|
| Property tests prove invariants 1 to 5 | Met, against the in-memory reference model | `internal/ledger` tests: conservation and no overdraft over 200 random runs; linked atomicity; replay; timeout release |
| Replay test | Met | Shared suite `deposit_replay_conflict`, run on memory and Postgres |
| Kill-and-restart test | Met for Postgres | `TestPendingRecordSurvivesRestart`; suite `crash_after_ledger_commit_is_recovered` |
| Benchmark report | Met for the reference model only | `docs/specs/ledger-gateway-bank/BENCHMARK.md` |
| Run against a live TigerBeetle cluster | Not met | No server binary (GitHub release downloads blocked) and no Docker. Adapter compiles; tagged suite ready. VERIFY-TB |
| Run against a live Kafka broker | Not met | Publisher compiles against franz-go. VERIFY-KAFKA |

Local Postgres was tested on 16.15, not the 17 named in ARCHITECTURE.md section 18.1. The schema uses only
standard features; the production image must be run against 17 before sign-off.

## 9. Decisions made while building (deviations and interpretations)

These were not in the original spec and should be confirmed by a human reviewer.

1. **Syntax errors are not recorded.** Malformed input (bad UUID, bad ID, missing key, wrong entity, negative amount) returns
   `INVALID_ARGUMENT` before any record is written, so the same key can be reused after fixing the request.
   Business rejections (insufficient funds, disabled code, unknown account) are recorded and replay the same rejection.
2. **Entity mismatch is `INVALID_ARGUMENT`**, not a rejection code. Section 3 step 2 is enforced before idempotency.
3. **Pending transfers must have a timeout.** A pending leg with no timeout is rejected (`TIMEOUT_REQUIRED`), so a hold
   cannot sit forever. Timeouts on non-pending transfers are rejected.
4. **Account creation is all-or-nothing.** The gateway links every account in one request, so a conflict on any account
   leaves none created.
5. **Post and void apply the full pending amount.** Partial post is out of scope.
6. **Account directory uses hex text for account IDs**, not `numeric(39,0)`. Hex matches the gateway's ID format and
   avoids a conversion layer. Store-side IDs are still 128-bit.
7. **Limits and enabled codes are placeholders.** The per-leg maximum (R10M / $10M), the 7-day pending maximum and the
   enabled account and transfer codes live in `DefaultConfig`. Real limits belong to `tiers.yaml` and OPA
   (ARCHITECTURE.md section 10). The code sets are the Appendix B subset enabled in v1.
8. **Batch size is capped at 100 legs.** This is a conservative local cap, not a TigerBeetle limit (VERIFY-TB).
9. **Caller identity is a header in development.** Production must take it from the mTLS workload identity. This is
   not implemented.
10. **Events are written in the same transaction as the record.** The payload is read back from the ledger for the
    timestamp, so each replay of a committed request adds no events.

## 10. Open items (see docs/QUESTIONS.md)

- VERIFY-TB: TigerBeetle version, ID scheme, linked chains across ledgers, batch limits, and whether `exists` inside a
  linked chain counts as success. The memory model assumes it does.
- VERIFY-KAFKA: Kafka broker configuration and franz-go behaviour under failure.
- The reconciler that republishes gaps between TigerBeetle and the outbox (ARCHITECTURE.md 7.8) is not built.
- Idempotency record purge after 30 days (ARCHITECTURE.md 7.6) is not built.
- Metrics and tracing (ARCHITECTURE.md section 22) are not wired. Only health and readiness endpoints exist.
