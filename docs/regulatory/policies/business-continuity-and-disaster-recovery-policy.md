> **DRAFT for board approval. Not approved. Not in force.** Prepared on 2026-11-18 from the repository and the regulatory register. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. Nothing here is legal advice.

# Business continuity and disaster recovery policy

| | |
|---|---|
| Owner | Business Continuity Manager (R28, VACANT) |
| Approved by | Board, on IT and Cyber Committee recommendation |
| Review | Yearly |
| Register rows | G4-02, JS1-12, OUT-05 |
| Version | 0.1 draft |

**Purpose.** Keeps critical services running, or restores them quickly, after any disruption.

## Business impact analysis

1. Critical services: payments in and out, the ledger, customer sign-in, sanctions screening, regulatory reporting.
2. PROPOSED recovery time objective: 4 hours for payments; 24 hours for reporting. Recovery point objective: zero for the ledger, 5 minutes for others.

## Recovery

1. A geographically separate disaster recovery site (JS1-12); not built until hosting is chosen.
2. Backups and point-in-time recovery are proven by a restore in the CI gate today; production needs off-site immutable storage.
3. The recovery plan covers outsourced material activities (OUT-05).

## Testing

1. Tabletop twice a year; full failover including the loss of a site at least yearly; results to the IT and Cyber Committee.

## Sign-off

| Role | Name | Signature | Date |
|---|---|---|---|
| Owner | VACANT | | |
| Approver (Board) | VACANT | | |
| Second line reviewer | VACANT | | |
