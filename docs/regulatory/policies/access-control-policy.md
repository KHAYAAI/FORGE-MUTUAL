> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# Access control and privileged access policy

Register ids: JS2-03, JS2-04, JS2-05. Joint Standard 2 of 2024, 8.3.

1. Sign-in is through the identity provider with multi-factor authentication. Services in production refuse tokens that do not show MFA (`OIDC_REQUIRE_AMR`).
2. Access is by role from policy; no shared accounts. Service accounts are separate and limited.
3. Anything that moves money, changes policy, or releases a held payment needs a maker and a different checker.
4. Privileged access (production data, keys, deploy) is granted for a purpose, time-limited, and logged. PROPOSED: reviewed monthly.
5. Joiners, movers, leavers: access is granted on appointment and removed the day someone leaves.
6. Review all access at least quarterly (PROPOSED) and record who reviewed it.
