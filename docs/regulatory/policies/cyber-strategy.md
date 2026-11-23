> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# Cyber and information security strategy and framework

Purpose: the board-approved strategy and framework Joint Standard 2 of 2024 (paragraphs 4 to 6, 7) expects. Register ids: JS2-01 to JS2-17 (board accountability: JS2-01).

## Principles
1. Customer money and data are protected before features ship.
2. Controls are enforced in code and checked in the build, not left to memory (the `make ci` gate, signed policy bundles, mTLS between services, hash-chained audit).
3. Least privilege, with a second person for anything that moves money or changes policy (maker-checker).
4. Assume breach: log everything that matters, protect the logs, practise recovery.
5. Providers are risks we manage, not assurances we inherit.

## Governance
- The board approves this strategy and reviews it at least yearly. PROPOSED: a board risk committee receives a cyber report each quarter.
- Accountable executive: CEO until a CISO or equivalent is appointed (to be named).
- Independent assurance: penetration test before launch and yearly; internal audit once the structure exists. None done yet.

## Control areas and where each stands
| Area (Joint Standard 2) | Position today | Next |
|---|---|---|
| Asset inventory (7.1) | `docs/regulatory/assets.csv` with a build check | Owners named; production assets added |
| Cryptography and keys (7.2.6) | AES-256-GCM for personal data; keys from files | Key management policy; HSM or managed KMS decision |
| Access control and MFA (8.3) | OIDC with enforced MFA claim in production; maker-checker | Confirm provider sends the claim; privileged access reviews |
| Monitoring (7.3) | Alert catalogue and runbooks | A real alerting backend, on-call rota |
| Backup and recovery (7.4) | Local base backups and WAL archive, PITR proven locally | Offsite immutable copy, restore drills, RPO and RTO |
| Secure development (7.7) | Vulnerability check, tests, review in CI | Dependency policy, annual pen test, finding tracker |
| Incident response | Plan and notification workflow | Tabletop exercise |
| Third parties | Draft outsourcing policy and register | Contracts with required clauses |
| People | Not started | Training plan, vetting |

## Measures (PROPOSED)
Time to classify an incident; open critical findings; backup freshness; restore drill pass rate; share of staff with current training.
