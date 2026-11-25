> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository, the regulatory register and the PA's published return forms on 2026-11-24. Values marked PROPOSED are suggestions for the owner. Approver: ________  Date: ________

# Deposit insurance policy (CODI)

Register ids: DI-01, DI-02. PA structure: PA-14 (deposit insurance). Sources: Financial Sector Regulation Act Chapter 12A, the Corporation for
Deposit Insurance (CODI) standards, and Mutual Banks Directive 1 of 2024 (regulatory requirements relating to covered deposits). Not legal advice.

1. **Purpose.** Make sure every depositor knows what is covered, and that the bank can give CODI an accurate single customer view at any time.
2. **Membership.** On registration the mutual bank becomes a member of the deposit insurance scheme and pays its levies (VERIFY the levy basis).
   Before registration, deposits are held by the sponsor bank and are covered, if at all, under the sponsor's membership; Forge tells customers so.
3. **Disclosure.** Every balance shows whether it is covered and up to what limit (DI-02); member shares are **not** deposits and are not covered,
   and members are told so in writing before they buy them (constitution draft clause 4.5).
4. **Single customer view.** The treasury-risk service builds the single customer view from the ledger (`services/bank/treasury-risk/internal/prudential/scv.go`);
   it is tested monthly and given to CODI in the format CODI prescribes (VERIFY the format).
5. **Reporting.** The covered-deposit exposure is reported as the directive requires (DI 100 and DI 500 line items named in Directive 1 of 2024;
   VERIFY after the 2026 move of DI returns into a directive).
6. **Owner and review.** The chief financial officer owns this policy; reviewed yearly and whenever CODI changes its standards.
