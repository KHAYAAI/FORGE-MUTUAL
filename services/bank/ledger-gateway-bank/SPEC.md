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
- **A refused transfer ID stays refused.** When TigerBeetle refuses a transfer for a reason that depended on the ledger's
  state at that moment (a balance too low, an account or a pending transfer that did not exist), it remembers the ID and
  answers `id_already_failed` to every later use of it. The gateway reports that as `PREVIOUSLY_REFUSED`. It can only be
  seen when the first answer was lost before the gateway recorded it (a crash, a timeout); otherwise the recorded
  rejection is replayed. A request once refused therefore cannot go through later behind the caller's back. To try
  again, the caller sends a new request under a new idempotency key.

## 5. Idempotency (ARCHITECTURE.md 7.6)

- Record key: `(caller, rpc, idempotency_key)`. Stored in Postgres `idempotency_record` with a SHA-256 hash of the canonical request.
- Before any TigerBeetle call, the gateway writes the record as `PENDING` with all transfer IDs generated and stored.
- Same key, same hash, status `COMMITTED`: return the stored result with `STATUS_REPLAYED`. No money moves.
- Same key, same hash, status `PENDING` (a previous attempt crashed): resubmit with the stored IDs. For a single
  transfer that was already applied, TigerBeetle answers `exists`. For a linked chain that was already applied it
  answers `exists` for the first leg and `linked_event_failed` for the rest, and changes nothing (measured; the model
  used to assume `exists` for every leg). The gateway reads both as "already applied", and then reads every transfer
  back from the ledger before it reports success: if one were missing, the request fails instead.
- If the earlier attempt was refused rather than applied, the resubmission answers `PREVIOUSLY_REFUSED` (section 4).
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
| Property tests prove invariants 1 to 5 | Met, on the model and on TigerBeetle | `internal/ledger` tests: conservation and no overdraft over 200 random runs; linked atomicity; replay; timeout release. The differential test holds the model to TigerBeetle (section 12) |
| Replay test | Met | Shared suite `deposit_replay_conflict`, run on memory, Postgres and TigerBeetle |
| Kill-and-restart test | Met | `TestPendingRecordSurvivesRestart`; suite `crash_after_ledger_commit_is_recovered`, `crash_after_a_linked_chain_committed_is_recovered`, `a_refused_request_stays_refused_when_its_answer_was_lost`; for the ledger itself, `TestClusterSurvivesLosingReplicas` |
| Benchmark report | Met for the reference model only | `docs/specs/ledger-gateway-bank/BENCHMARK.md` |
| Run against a live TigerBeetle cluster | Met, on a development-mode cluster | TigerBeetle 0.16.60, one replica and three replicas, 2026-11-16 (section 12). Not yet on production hardware or a production configuration |
| Run against a live Kafka broker | Met | The outbox suite publishes to Kafka 3.9 in the gate, and `pkg/outbox/kafka_cluster_test.go` runs the relay on three brokers while one and then two are killed. The ledger gateway's own publisher has the same settings (acks=all, a 20-second delivery deadline) but is not itself run against the cluster |

The gate runs against Postgres 17.

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
9. **Caller identity is a header in development.** In production it is the service named in the caller's client
   certificate (`pkg/serve`), and the header is ignored.
10. **Events are written in the same transaction as the record.** The payload is read back from the ledger for the
    timestamp, so each replay of a committed request adds no events.

## 10. Open items (see docs/QUESTIONS.md)

- VERIFY-TB, what is left: a production cluster (six replicas on dedicated hosts, without `--development`), its batch
  limits, an upgrade rehearsal, and how fast a replica repairs and state-syncs on that hardware (section 12). Answered on
  2026-11-16: the version (0.16.60), the ID scheme, and `exists` inside a linked chain, which does **not** count as
  success: it breaks the chain. The model and this document said otherwise, and both are corrected.
- VERIFY-KAFKA, what is left: TLS and SASL to the brokers, and the production cluster's topic settings. Found on a three-broker cluster: with no quorum the client retried a record for ever; both publishers now have a delivery deadline.
- **Outbox reconciler (ARCHITECTURE.md 7.8) is not built, and I recommend changing that section.** The gateway writes
  each outbox row in the same transaction as the record it belongs to, so a committed transfer without an event
  cannot happen inside the gateway. A reconciler that compares TigerBeetle with the outbox would only detect writes
  made outside the gateway, and the one-writer rule (invariant 6) forbids those. A human should decide between
  dropping the reconciler from the spec or keeping it as a detector.
- Tracing (ARCHITECTURE.md section 22) is not wired. Metrics are: `ledger_rpc_total{rpc,outcome}`,
  `ledger_rpc_duration_seconds{rpc}` and `ledger_rejections_total{rpc,code}`, served at `/metrics`.
  Labels never carry account, party or transfer IDs.

## 11. Retention (built)

- `RunRetention` deletes COMMITTED idempotency records whose result was stored more than `LEDGER_IDEMPOTENCY_RETENTION`
  ago, every hour. The default is 720 hours and the server refuses anything shorter.
- PENDING records are never deleted. A purged key that is replayed would move money again, so the retention must
  exceed every client's retry window. Clients must not reuse a key after that window.

## 12. TigerBeetle: what was run, what it showed, and how to run it

Run on 2026-11-16 against TigerBeetle 0.16.60 (`ghcr.io/tigerbeetle/tigerbeetle:0.16.60`) with the Go client v0.16.60,
in Docker, in development mode, on one machine. The client must never be newer than the cluster: upgrade the cluster
first, then the client.

```sh
scripts/tigerbeetle.sh single up && eval "$(scripts/tigerbeetle.sh env single)" && make test-ledger-tb
make test-ledger-tb-cluster   # three replicas, taken apart while in use
```

`make ci` requires `TB_ADDRESSES` and `TB_CLUSTER_ID`, and `make ci-db` starts a replica for it, so the gate runs the
first of these every time.

**The gateway suite passes** on one replica and on three, including the crash cases.

**The in-memory model is held to TigerBeetle.** Every other suite in the repository runs on the model, on the claim that
it behaves as TigerBeetle does. `TestMemStoreAgreesWithTigerBeetle` sends both the same random batches (linked chains,
pending transfers with posts and voids, exact replays, the same ID with a changed field, IDs that failed before, missing
accounts, overdrafts) and requires the same outcome for every event, the same four balances on every account, and the
same stored transfers. On its first run it found the model wrong in four ways, all now fixed:

1. A transfer ID that was refused for a state-dependent reason is refused for good (`id_already_failed`). The model
   looked at it afresh, so a refused request could succeed on a retry. This is the one that mattered (section 4).
2. A linked chain that is sent again answers `exists` then `linked_event_failed`, not `exists` for every leg (section 5).
3. A post or void with empty user data is stored with the pending transfer's, and is compared that way on a replay.
4. The adapter did not map `id_already_failed`, the `exists_with_different_user_data` results, or
   `pending_transfer_not_pending`; they all came back as `invalid_transfer`.

After the fixes: 48,000 batches (about 120,000 events) on four seeds, no difference. The seed is random in the gate and
printed, and `TB_DIFF_SEED` repeats a run. Removing either of the first two fixes makes a test fail.

**Three replicas, taken apart** (`TestClusterSurvivesLosingReplicas`):

- One replica killed, primary or not: the cluster carries on. A new primary took over in about five seconds, and the
  model and the cluster still agreed on every outcome and balance.
- That replica started again and another killed: the cluster carries on, which it can only do if the first caught up.
- A second replica killed, so one of three is left: nothing is accepted. With the replicas started again, a transfer
  that was tried during the outage and retried under the same ID was applied exactly once.

**What running it changed in the code.**

- The TigerBeetle client never gives up. Without a quorum it waits for the cluster for as long as it takes, and the
  adapter ignored the request's deadline, so a gateway request would have hung for ever. The adapter now stops waiting
  at the caller's deadline or after 12 seconds, whichever is first, and answers `ErrUnavailable`, which says plainly
  that the outcome is unknown and the same IDs must be sent again. At most 512 calls may be waiting; beyond that, calls
  are refused at once.
- `/readyz` now asks the ledger as well as the database, so a gateway whose cluster has no quorum reports not ready.

**What it showed that still needs measuring on production hardware.** A replica that was down while about 800
operations were committed replayed them at two or three a second after it restarted (a replica that was much further
behind used state sync instead, which took seconds). While a replica is still catching up, losing a second one leaves
the cluster without a working majority until the first finishes. That is safe (requests answer "unavailable" and
nothing is lost), but it is an outage. This was a development build with extra verification on one shared machine, so
the rate says nothing about production; the point is that the rate must be measured there, and that a three-replica
cluster does not tolerate a second failure during a repair. TigerBeetle recommends six replicas for production.

**Not verified:** a production configuration (no `--development`, direct I/O, dedicated NVMe hosts, six replicas across
sites), throughput and latency on it, an upgrade from one release to the next, and restoring a cluster from its data
files.
