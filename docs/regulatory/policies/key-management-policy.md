> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# Cryptographic key management policy

Purpose: Joint Standard 2 of 2024, 7.2.6 (key management). Register id: JS2-09. Inventory entries: KEY-01 to KEY-04.

## Rules
1. Keys are generated with approved libraries and never in source or logs. Secrets are supplied as files or from a secret manager.
2. Separate keys per purpose: service certificates, personal-data encryption, policy signing, webhook secrets.
3. Rotation. PROPOSED: certificates short-lived and renewed automatically; data-encryption keys yearly with re-encryption plan; signing key yearly or on suspected compromise.
4. Custody. No single person holds a whole production key. Policy signing key use needs two people. Where the key is in a managed KMS or HSM, access is limited and logged.
5. Backup of keys is separate from backup of data. Losing a data key means losing the data: test key recovery.
6. Compromise: treat as a security incident, rotate, assess exposure, follow the incident plan.

## Not decided
HSM or managed KMS (and its location) is undecided. Until decided, production must not go live with file-based keys.
