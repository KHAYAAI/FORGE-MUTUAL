> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# Data strategy and classification policy

Purpose: classify data, say how each class is protected, and keep one inventory. Register ids: JS2-02, JS2-06. Inventory: `docs/regulatory/assets.csv`.

## Classes
| Class | Examples | Minimum protection |
|---|---|---|
| Restricted | Balances, national IDs, KYC data, keys, audit log | Encrypted at rest where personal; mTLS in transit; named access; every access logged |
| Confidential | Policy rules, source code, aggregate treasury data | Access by role; encrypted in transit |
| Internal | Runbooks, internal docs | Staff only |
| Public | Marketing, published terms | None |

## Rules
- Events on the bus carry party IDs, not personal data.
- National IDs are stored encrypted with a keyed index for lookup; contacts are sealed.
- Retention: PROPOSED five years after a relationship ends for records FICA requires; shorter for the rest. Legal to confirm periods; deletion must preserve the audit chain.
- Personal information is processed under POPIA: purpose limitation, minimisation, consent records (consent-service).
- Data used for regulatory returns passes through an evidence pack with a digest and a second approver.

## Data owners
Each asset in the inventory has a named owner. Owners are not yet named.
