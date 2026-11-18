# audit-service (services/shared/audit-service)

Entity: shared. Architecture: ARCHITECTURE.md 15.3. Invariant 14: every policy decision and money movement is logged,
and the audit log is append-only and hash-chained.

## Design

- **Chain.** Each entry stores `seq`, `at` (microseconds), `actor`, `action`, `entity`, the exact payload bytes, the
  payload hash, the previous entry's hash, and its own hash. `hash = SHA-256(prev || seq || at || actor || action ||
  entity || payload_hash)`, with length-prefixed strings. The genesis previous hash is 32 zero bytes.
- **Actor.** The actor is the authenticated caller (`X-Forge-Caller`), never a request field. A service can only write
  under its own identity.
- **Append-only at the database.** Triggers refuse `UPDATE`, `DELETE` and `TRUNCATE`. This stops ordinary mistakes and
  ordinary attackers. It does not stop someone who can disable triggers, which is why anchors exist.
- **Serialised writes.** Appends take a Postgres advisory lock, so concurrent writers cannot take the same sequence
  number or fork the chain. A test with ten concurrent writers checks this.
- **Anchors.** The head hash is written outside the database at `AUDIT_ANCHOR_EVERY` intervals, but only when the head
  has moved. A rewrite that reseals every hash passes the internal check and fails against an earlier anchor.

## Verifying

    forge-audit verify --anchors <anchor file>

Walks the whole chain, checks each anchor, and exits 0 when intact, 1 when broken or rewritten, 2 when it cannot run.
Run it on a schedule and on any incident. Anchors must be kept somewhere the database operator cannot write.

## Interfaces

ConnectRPC `forge.audit.v1.AuditService`: `Append` (any allow-listed caller), `Query` (paged, optional entity filter,
bounded to 500), `Head`.

## Limits, stated

- **Anchors are a local file in this build.** That gives no protection against someone who can rewrite the file.
  Production needs write-once storage with object lock (VERIFY-WORM). The service logs this warning at start.
- **No producer is wired yet.** Policy decisions already reach the outbox; they are not yet appended here. Money
  movements from the ledger are not appended here either. Both are required before invariant 14 holds end to end.
- **Payloads are stored as given.** Callers must keep personal data out of payloads. The service does not redact.
- **Retention and export** are not built. Entries are never deleted, so the table grows without bound until a
  retention decision is made (QUESTIONS.md).

## Configuration

| Variable | Meaning |
|---|---|
| `DATABASE_URL` | Postgres (required) |
| `AUDIT_CALLERS_FILE` | JSON allow-list (required) |
| `AUDIT_ANCHOR_FILE` | Anchor file (required) |
| `AUDIT_ANCHOR_EVERY` | Anchor interval (default 10m) |
| `AUDIT_ADDR` | Listen address (default `:8094`) |

## Tests

`make test-audit`. Database tests need `AUDIT_TEST_DSN` and skip without it. The tamper tests disable the trigger
on purpose, to show that the verifier catches what the trigger cannot.
