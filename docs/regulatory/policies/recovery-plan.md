> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# Business continuity, disaster recovery and recovery plan

Register ids: JS2-12, JS2-13, BR-04. This is a draft with PROPOSED objectives. Recovery and resolution planning for a deposit-taker has its own requirements (PA guidance, not yet read in full).

## Business impact (initial)
| Function | Impact of loss | PROPOSED RTO | PROPOSED RPO |
|---|---|---|---|
| Ledger and payments | Customers cannot pay; money at risk | 4 hours | Zero for committed transfers (ledger is replicated) |
| Sign-in and workspace | Customers locked out | 4 hours | 15 minutes |
| Compliance and screening | Cannot onboard or release payments | 8 hours | 15 minutes |
| Audit log | Evidence at risk | 24 hours | Zero (anchored) |
| Back office | Operations slow | 24 hours | 1 hour |

These numbers are proposals. The board sets them; the architecture then has to meet them (it does not yet: there is no production environment).

## Recovery capabilities today
Postgres base backups and WAL archive give point-in-time recovery, proven on the local stack (`scripts/pitr-check.sh`). Freshness is checkable (`scripts/backup-freshness-check.sh`). Offsite immutable copy: script written, untested, no bucket. Ledger replication and multi-region: not built.

## Drills
Restore drill before launch and then quarterly (PROPOSED); failover drill yearly. Record results. A backup that has not been restored is not a backup.

## Recovery and resolution plan
Not started. Needs: critical functions, wind-down options, funding, stakeholder map, and PA input on form and timing.
