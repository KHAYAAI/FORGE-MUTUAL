> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Values marked PROPOSED are suggestions for the owner. Approver: ________  Date: ________

# Secure development and open-source policy

Register ids: JS2-07, JS2-17.

1. **Review.** Every change is reviewed by someone other than its author before merge. The CI gate (`make ci`) must pass: lint, vet, tests, isolation check, vulnerability check.
2. **No secrets in the repository.** Secrets come from files or a secret manager. Scan for them in CI (PROPOSED; not wired yet).
3. **Money paths.** Only the ledger gateway writes the ledger. Changes to money movement, policy rules or authentication need a second reviewer who is named on the change.
4. **Dependencies.** Add a dependency only when needed, check its licence and maintenance, and pin the version. Vulnerability checks run in CI and daily (PROPOSED). Critical fixes within 7 days, high within 30 (PROPOSED); exceptions are recorded with a reason and an expiry.
5. **Open-source licences.** Permissive licences (MIT, BSD, Apache-2.0) are allowed. Copyleft (GPL, AGPL) needs legal review before use in anything shipped. Record the licence of each direct dependency (PROPOSED: generate a list in CI).
6. **Testing.** New behaviour has tests. Security-relevant behaviour has tests that prove refusal (as the MFA and goAML tests do).
7. **Releases.** Built from a tagged commit by CI, with the policy bundle signed. No manual production changes.
8. **Penetration testing.** Independent test before launch and yearly; findings tracked with severity and deadline (Joint Standard 2, 7.7.6).
