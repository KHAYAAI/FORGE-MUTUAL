# Open questions

Per ARCHITECTURE.md preamble, conflicts and unknowns are recorded here.

## VERIFY-IMG: image versions in `deploy/compose/docker-compose.yml`

Several services use `latest` or a major tag placeholder. Before the stack is relied on, a human must pin each image to an exact, current stable release and confirm the start command and environment variables against that version's documentation. The TigerBeetle command in particular depends on the pinned version.

## Phase 0 items still open

- ADR-0001 (Fineract vs in-house account service): spike not started.
- ADR-0020 (mutual vs bank) and ADR-0012 (sector appetite) are human decisions; nothing in this repo depends on them yet.
- `AGENTS.md` and `BUILD_PLAN.md` are referenced by the architecture document but are not in the repository.

## Ledger gateway (services/bank/ledger-gateway-bank)

- **VERIFY-TB.** The TigerBeetle adapter compiles against tigerbeetle-go v0.16.60, but no server has run it. Before sign-off, run `make test-ledger-tb` against a real cluster. Confirm: the cluster ID format; time-ordered ID use; whether `exists` inside a linked chain counts as success (the memory model assumes it does); the batch size limit (the gateway caps requests at 100 legs); and that account creation with `linked` is atomic across the batch.
- **VERIFY-KAFKA.** The Kafka publisher compiles against franz-go v1.18.1 but has not been run against a broker. Confirm acks, idempotence and the topic configuration.
- **VERIFY-IMG (ledger).** The gateway Dockerfile has not been built, because Docker is unavailable in this environment. Build and run it before relying on it.
- **Postgres version.** ARCHITECTURE.md names PostgreSQL 17. The gateway was tested on 16.15. Re-run `make test-ledger-pg` on 17.
- **Limits are placeholders.** `DefaultConfig` holds per-leg limits (R10M / $10M) and enabled codes so the gateway can run. Real limits belong in `tiers.yaml` and OPA. Confirm with the risk owner.
- **Caller identity.** Local development trusts the `X-Forge-Caller` header. Production must derive the caller from the mTLS workload identity (ARCHITECTURE.md section 5). Not implemented.
- **Outbox reconciler.** ARCHITECTURE.md 7.8 asks for a reconciler between TigerBeetle and the outbox. A transactional outbox makes gaps impossible inside the gateway, so this needs a decision: drop it from the spec, or keep it as a detector. Decide before Phase 1.
- **Retention window.** Idempotency records are purged after 30 days by default (`LEDGER_IDEMPOTENCY_RETENTION`). Confirm the window against the longest client retry, and against the 30-day minimum in ARCHITECTURE.md 7.6.
- **Tracing not built.** Metrics are built. OpenTelemetry tracing (section 22) is not.
