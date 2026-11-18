> **DRAFT for board approval. Not approved. Not in force.** Prepared on 2026-11-18 from the repository and the regulatory register. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. Nothing here is legal advice.

# Risk management framework

| | |
|---|---|
| Owner | Chief Risk Officer (R02, VACANT) |
| Approved by | Board, on Risk and Capital Management Committee recommendation |
| Review | Yearly |
| Register rows | BR-02, MR-01, LIC-13 |
| Version | 0.1 draft |

**Purpose.** Describes how Forge identifies, measures, monitors, controls, prices, mitigates and reports every material risk (Banks Act regulation 39; Mutual Banks regulations).

## Risk taxonomy

1. Solvency and capital risk: `capital-management-policy.md`.
2. Liquidity and funding risk: `liquidity-and-contingency-funding-plan.md`.
3. Credit and counterparty risk, including concentration: `credit-risk-policy.md`, `concentration-policy.md`.
4. Market risk: currency and interest rate in the banking book: `market-and-interest-rate-risk-policy.md`.
5. Operational risk, including fraud, process and people: `operational-risk-policy.md`.
6. Technology, cyber and data risk: `it-risk-management-framework.md`, `cyber-strategy.md`, `data-strategy.md`.
7. Third-party and outsourcing risk: `outsourcing-policy.md`, `cloud-and-offshoring-policy.md`.
8. Compliance and conduct risk, including financial crime: `compliance-charter.md`, `rmcp.md`, `complaints-and-fair-treatment-policy.md`.
9. Strategic and reputational risk, crypto-asset risk: this framework and the risk appetite statement.

## Process

1. Identify: every new product, provider, change and incident is assessed against the taxonomy.
2. Measure: limits are numbers in `docs/THRESHOLDS.md` and the prudential calculations in `services/bank/treasury-service/internal/prudential`.
3. Monitor: key risk indicators reported monthly to the executive committee and quarterly to the board.
4. Control: controls in code (ledger invariants, limits, licence gate, maker-checker) are tested in the CI gate.
5. Report: the CRO reports to the Risk and Capital Management Committee directly.

## Roles

1. First line owns its risks and controls. Second line (CRO, CCO, MLRO, CISO) sets the rules and challenges. Third line (internal audit) assures both.
2. The CRO has a veto over any product or limit change that would breach the appetite, overridable only by the board.

## Risk register

1. Kept in `risk-register.md`; every risk has an owner, inherent and residual ratings and its controls.

## Sign-off

| Role | Name | Signature | Date |
|---|---|---|---|
| Owner | VACANT | | |
| Approver (Board) | VACANT | | |
| Second line reviewer | VACANT | | |
