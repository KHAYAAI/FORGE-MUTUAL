> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# IT strategy (Joint Standard 1 of 2023)

Register id: JS1-01. Aligned to the business: a bank for people who build, run on open, verifiable technology.

1. **Correctness first.** Money lives in TigerBeetle, a ledger built for exactly-once double entry; every change to money
   is idempotent and reconciled. The gate refuses code that breaks an invariant.
2. **Small, isolated services.** Each owns one domain and may call only what it needs (least privilege, checked in CI).
3. **Regulated products off by default.** New products ship dark and are switched on by a recorded decision (`pkg/licence`).
4. **Open source and portable.** Postgres, Kafka, Temporal, TigerBeetle, OpenTelemetry: no single vendor holds the core,
   which keeps exit plans real (`exit-plans.md`).
5. **South African data residency** for core data (PROPOSED; ADR-0002 decides the host).
6. **Partners behind interfaces.** Sponsor bank, identity, KYC, acquirer and sanctions data are adapters, replaceable
   without changing the core.
7. **Next three years (PROPOSED):** production hosting and DR site; live providers; the own-licence route if ADR-0020
   chooses it; digital assets only after a CASP licence (ARCHITECTURE.md 16).
Reviewed yearly by the board with the IT risk report.
