# SPEC: party-service

Status: built and tested locally. Human review required before any real customer (ARCHITECTURE.md section 0, rule 3).
Entity: PLAT. Sections referenced: ARCHITECTURE.md 2.2 (invariants 10, 11, 12), 11.4, 18.3, Appendix C.

## 1. Purpose

The system of record for parties: persons, companies and funds. It holds their relationships (director, shareholder,
UBO, signatory, employee) and their lifecycle status. It moves no money.

## 2. Interface

ConnectRPC `forge.party.v1.PartyService`: CreateParty, GetParty, AddRelationship, ListRelationships, SetStatus,
SetTier. Caller allow-list applies to every RPC (deny by default, `pkg/callerauth`).

## 3. Rules

- **Lifecycle.** PROSPECT -> KYC_PENDING -> ACTIVE -> RESTRICTED | FROZEN -> CLOSED. Anything not listed is refused
  with `FAILED_PRECONDITION`. CLOSED is terminal. The table is `internal/lifecycle`, with unit tests.
- **Founder and company separation (invariant 12).** A relationship is allowed only when the party types fit:
  directors, signatories and employees are persons; a company can never hold them. UBO is a person only, and needs a
  share (`percentage_bps` > 0). Shareholder may be a person, company or fund, and needs a share. A party cannot relate
  to itself. Each active relationship type between two parties is unique.
- **National IDs (invariant 11 and section 18.3).** Persons' national IDs are sealed with AES-256-GCM and never stored
  in plaintext. A keyed HMAC index detects duplicates without decryption. Responses show only the masked value (last
  four characters). Keys come from `PARTY_SEAL_KEY` and `PARTY_INDEX_KEY` and must be set.
- **Idempotent creates and relationships.** The same key with the same request returns the same result with no second
  event. The same key with a different request returns `ALREADY_EXISTS`.
- **Events.** Every status change, including creation, writes `forge.plat.party.status_changed.v1` to the outbox in
  the same transaction as the change. A refused transition writes nothing.
- **Concurrency.** Status changes lock the row, and the update is conditional on the status still being the one read.

## 4. Tests

`PARTY_TEST_DSN` must point at a throwaway database. Covered: sealing (no plaintext in the column, masked output),
idempotent creates (replay and conflict), duplicate detection for national IDs and registration numbers, the lifecycle
table with its event count, founder and company separation, relationship replay and conflicts, tier validation, and
input validation. The sealer and lifecycle packages are unit-tested without a database.

## 5. Deviations and open items

- **Keys are environment variables in this build.** Production must use OpenBao transit (VERIFY-OPENBAO).
- **Relationship changes emit no event.** ARCHITECTURE.md Appendix C does not list one; add it if the catalogue
  changes.
- **Creating a party does not check that its tier exists in policy.** Tiers are validated against the two labels
  policy-service knows about, but nothing reads back from policy yet.
- **Percentage thresholds for UBO are not encoded.** The legal threshold is a VERIFY item (ARCHITECTURE.md 27,
  item 17 and the FIC Act).
- **Not built:** Keycloak-driven owner roles (P1-03), relationship-level audit events, and the party read model
  consolidated-view consumes.
