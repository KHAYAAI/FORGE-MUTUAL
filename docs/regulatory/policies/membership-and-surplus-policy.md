> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository, the regulatory register and the PA's published return forms on 2026-11-24. Values marked PROPOSED are suggestions for the owner. Approver: ________  Date: ________

# Membership and surplus distribution policy

Register ids: LIC-07. PA structure: PA-06 (capital) and PA-17 (licensing and mutual form). Must be read with the draft constitution
(`docs/licence-application/CONSTITUTION_DRAFT.md`); where they differ, the constitution as finally adopted wins.

1. **Purpose.** Admit members fairly, keep member shares from weakening capital, and return surplus to members only when the bank can afford it.
2. **Admission.** A member must hold an account and the minimum member share holding, and pass customer due diligence (FICA). Refusals are
   recorded with reasons. The membership service enforces the founding number and the register (`services/bank/membership`).
3. **Member shares.** Issued only against full payment, never transferable except by redemption, one vote per member. PROPOSED: no member holds
   more than 5% of member shares.
4. **Redemption.** Refused when it would take capital below the requirement (DI 400) or the board's buffer, and deferred under the constitution's
   limit; the software refuses it with a capital check.
5. **Surplus.** First to the reserves the Act requires and the board sets; then, if the bank would still meet its capital, liquidity and
   large-exposure limits, returned to members as rebates by a weight the members approve (fees paid, PROPOSED). **Profit counts as capital only once
   the board has appropriated it to a reserve by formal resolution** (DI 400 footnote a); this policy requires that resolution at each year end.
6. **Approval.** A distribution is proposed by the board on the CFO's recommendation, approved by the members' meeting, and paid by maker-checker.
7. **Review.** Yearly, and when the PA publishes its capital standard for mutual banks.
