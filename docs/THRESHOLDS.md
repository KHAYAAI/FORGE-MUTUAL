# Placeholder thresholds and their owners

Every number below is a placeholder marked VERIFY. None may go live until its owner has confirmed it. The owner role
is what the architecture requires. The owner name is blank, because no person has been named for any of them.
Changing a value is a configuration or data change, so it needs no code change, but it does need the owner's sign-off
recorded here.

| # | Threshold | Current value | Where | Owner role | Owner | Status |
|---|---|---|---|---|---|---|
| 1 | Concentration: largest customer share of deposits | 2500 bps (25%) | `TREASURY_CONCENTRATION_BPS`, `services/bank/treasury-risk` | Head of Treasury | _unassigned_ | Placeholder |
| 2 | Net outflow: fall in deposits over about 24 hours | 2000 bps (20%) | `TREASURY_OUTFLOW_BPS` | Head of Treasury | _unassigned_ | Placeholder |
| 3 | Suspense ceiling before an alert | R10,000 (1,000,000 minor) | `TREASURY_SUSPENSE_MINOR` | Head of Operations | _unassigned_ | Placeholder |
| 4 | Case SLA: CRITICAL | 24 hours | `services/shared/compliance-engine/internal/cases` | MLRO (compliance officer) | _unassigned_ | Placeholder |
| 5 | Case SLA: WARNING | 72 hours | same | MLRO | _unassigned_ | Placeholder |
| 6 | Velocity: payouts in 24 hours | R50,000 | `internal/monitor` `DefaultConfig` | MLRO | _unassigned_ | Placeholder |
| 7 | Structuring band and count | R45,000 to R50,000, three payments in 7 days | same | MLRO | _unassigned_ | Placeholder |
| 8 | New beneficiary, large first payment | R20,000 | same | MLRO | _unassigned_ | Placeholder |
| 9 | Rapid in and out: share paid out within 24 hours | 90% | same | MLRO | _unassigned_ | Placeholder |
| 10 | Per-transaction, daily and step-up limits by tier | `policy/tiers.json` (standard and enhanced) | `policy/` | Risk owner | _unassigned_ | Placeholder (see `docs/QUESTIONS.md`) |
| 11 | KYC controlling threshold for the tier decision | 5000 bps | `services/platform/onboarding/internal/onboarding/tier.go` | Risk owner | _unassigned_ | Placeholder |
| 12 | Reconciliation: maximum age of a conclusive run | 30 minutes (`RECON_MAX_AGE`) | `services/bank/reconciliation` | Head of Payments operations | _unassigned_ | Operational; set for the 15-minute interval |
| 13 | Sanctions list maximum age | Set by the caller | `internal/sanctions` | MLRO | _unassigned_ | Recommend about one day |
| 14 | Payout batch: maximum items | 500 items (`PAYOUTS_MAX_ITEMS`) | `services/bank/payouts-service` | Head of Payments operations | _unassigned_ | Placeholder |
| 15 | Payout: maximum amount per item | R1,000,000 (100,000,000 minor, `PAYOUTS_MAX_ITEM_MINOR`) | same | Head of Payments operations | _unassigned_ | Placeholder |
| 16 | Payout: polls before an item stays open | 120 polls at 30 seconds (`PAYOUTS_MAX_POLLS`, `PAYOUTS_POLL_INTERVAL`) | same | Head of Payments operations | _unassigned_ | Placeholder |

**Not thresholds, but also owned:** the outbound reserve window of 7 days (the ledger's maximum pending timeout),
which is set by the ledger's limits, and the inbound credit lookback of 48 hours.

To close an item: the owner confirms the value in writing, the owner's name goes in the table, and the status moves to
"Confirmed". Record the date in `docs/DECISIONS.md`.
