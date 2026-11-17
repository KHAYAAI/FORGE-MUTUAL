> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# Data retention schedule (POPIA s.14; FICA s.22, s.23)

Purpose: how long each kind of record is kept, and what happens after. Register ids: POP-03, FIC-05. Periods are
PROPOSED and must be checked against every law that sets a minimum (FICA, the Banks Act and its returns, tax law, the
Companies Act, NCA) before approval. Where laws differ, the longest minimum applies.

| Record | Where | Kept for (PROPOSED) | Then |
|---|---|---|---|
| Customer identity and verification (CDD) | party-service, onboarding, Didit | 5 years after the relationship ends | Destroy; keep only a de-identified count |
| Declined applications | onboarding | 5 years after the decision | Destroy |
| Transactions and ledger entries | ledger (TigerBeetle), consolidated-view | 5 years after the transaction; the ledger itself is never altered | Archive the ledger; de-identify views |
| Suspicious transaction reports and cases | compliance-engine, reg-reporting | 5 years after the report | Destroy |
| Consents | consent-service | as long as the processing they cover, plus 5 years | Destroy |
| Credit applications and agreements | lending-service | 3 years after the agreement ends (NCA, PROPOSED; verify) | Destroy |
| Audit log | audit-service | 7 years (PROPOSED) | Archive to write-once storage |
| Application logs and traces | collector | 90 days | Delete |
| Backups and WAL archive | pg-backup | 14 days in production (2 on the local stack) | Delete |

**Not built:** an automated disposal job. Until one exists, nothing is deleted automatically, and this schedule is
applied by hand at the yearly review.
