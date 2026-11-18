> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# Secure configuration standard (Joint Standard 2 of 2024: configuration and patching)

Register ids: JS2-07, JS2-17. Each rule says how it is enforced. "Checked" means `scripts/check-secure-config.py`
fails the gate (`make test-regulatory`, part of `make ci`) when it is broken.

| # | Rule | Enforced by |
|---|---|---|
| 1 | Service images are distroless and run as a non-root user | Checked |
| 2 | Every third-party image is pinned to a version; never `latest` | Checked |
| 3 | No container runs privileged | Checked |
| 4 | Seccomp is relaxed only for TigerBeetle and its client (io_uring), with the reason written beside it | Checked |
| 5 | TLS 1.2 or later everywhere; certificate verification never disabled | Checked |
| 6 | Every service serves through `pkg/serve`: TLS, mutual TLS, request IDs, timeouts, and refusal of every development switch in production | Checked (no service listens without it) |
| 7 | Web apps send a nonce Content-Security-Policy, HSTS, nosniff and a referrer policy | Checked; browser tests also verify them |
| 8 | Secrets come from files (`NAME_FILE`); production refuses them in the plain environment | `pkg/serve` tests |
| 9 | Each service may call only the methods its code calls | `tools/isolation` (gate) |
| 10 | No shipped code calls a known-vulnerable function; dependencies are patched within PROPOSED 14 days of a fix (7 for critical) | `make vulncheck` (gate); the window is a process rule |
| 11 | A software bill of materials is produced for every release | `make sbom` (CycloneDX 1.5) |
| 12 | Production hosts: CIS benchmark for the chosen platform, no SSH by default, endpoint protection | **Not enforceable yet**: no production hosting (ADR-0002) |
