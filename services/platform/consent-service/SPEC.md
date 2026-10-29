# SPEC: consent-service

Status: built and tested locally. Human review required before any real customer (ARCHITECTURE.md section 0, rule 3).
Entity: PLAT. Sections referenced: ARCHITECTURE.md 2.2 (invariant 11), 11.5, 18.2, Appendix C.

## 1. Purpose

Records what a party has consented to, and answers whether a particular use of their data is covered. Every use of
third-party data must pass `Check` (invariant 11). A revocation takes effect on the next check, because `Check` reads
the stored consents at call time and keeps no cache.

## 2. Interface

ConnectRPC `forge.consent.v1.ConsentService`: Grant, Revoke, Check, List. Caller allow-list applies to each RPC.

## 3. Rules

- **Grant** needs a party UUID, a purpose (3 to 64 characters: lower-case letters, digits, dots, underscores), one to
  16 unique scopes, one to 16 data sources, an evidence reference (up to 256 characters) and an actor. An expiry must
  be in the future. A repeated key with the same request returns the same consent with no second event.
- **Revoke** sets `revoked_at` once. Revoking an already-revoked consent changes nothing and writes no event. Consents
  are never deleted.
- **Check** returns allowed only when a consent for that party and purpose is not revoked, not expired, and lists the
  scope. Otherwise it returns `no_valid_consent`. It never returns an error for a denial.
- **Events.** A grant writes `forge.plat.consent.granted.v1`. A revocation writes `forge.plat.consent.revoked.v1`.
  Both go to the outbox in the same transaction as the change.

## 4. Tests

`CONSENT_TEST_DSN` must point at a throwaway database. Covered: purpose and scope isolation, revocation effective on
the next check with one revoke event even after a repeated revoke, expiry (inserted directly, because grants refuse a
past expiry), idempotent grants with conflict detection, input validation, and listing that keeps revoked consents.

## 5. Deviations and open items

- **No check that the party exists.** Consent records a party UUID; the caller (onboarding) creates the party first.
  An existence check would make consent depend on party-service, so it is not done.
- **Policy `forge.consent` is not written.** `Check` is the decision, so a second copy in Rego would drift.
- **Retention.** POPIA retention for consent evidence is a VERIFY item (ARCHITECTURE.md 27, item 8). No purge exists.
