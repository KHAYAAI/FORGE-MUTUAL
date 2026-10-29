# Open questions

Per ARCHITECTURE.md preamble, conflicts and unknowns are recorded here.

## VERIFY-IMG: image versions in `deploy/compose/docker-compose.yml`

Several services use `latest` or a major tag placeholder. Before the stack is relied on, a human must pin each image to an exact, current stable release and confirm the start command and environment variables against that version's documentation. The TigerBeetle command in particular depends on the pinned version.

## Phase 0 items still open

- ADR-0001 (Fineract vs in-house account service): spike not started.
- ADR-0020 (mutual vs bank) and ADR-0012 (sector appetite) are human decisions; nothing in this repo depends on them yet.
- `AGENTS.md` and `BUILD_PLAN.md` are referenced by the architecture document but are not in the repository.
