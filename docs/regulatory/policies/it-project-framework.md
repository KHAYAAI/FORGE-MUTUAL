> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# IT programme and project framework (Joint Standard 1 of 2023)

Register id: JS1-10.

1. **Initiation.** A one-page brief: the business need, the regulatory rows it touches (`register.csv`), the risk owner.
2. **Security requirements** are written before building and approved by the CISO, independently of the delivery team
   (PROPOSED: a named reviewer who did not write the code).
3. **Design.** Changes to money, identity or data residency get a written design (an ADR) reviewed before build.
4. **Build and test.** Tests first for money movement; the full gate on every change; stack journeys for customer flows.
5. **Acceptance.** The business owner accepts against the brief; the risk owner signs the residual risk.
6. **Go-live.** For a regulated product, a recorded decision (`FORGE_APPROVAL_<PRODUCT>`) switches it on.
7. **Post-implementation review** within 30 days (PROPOSED).
