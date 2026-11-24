# Placeholder thresholds and their owners

A mock roster of the 16 people who would own and approve these, with hiring advice, is in [`THRESHOLD_OWNERS.md`](THRESHOLD_OWNERS.md).

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

| 17 | Deposit rates: savings, 32-day and 90-day notice | 6.50%, 7.75%, 8.50% a year | `DEPOSITS_RATES`, `services/bank/deposits-service` | Head of Treasury | _unassigned_ | Placeholder |
| 18 | Credit terms: overdraft and term-loan rates, revenue-based fee and share | 16.75%, 14.50%; 8% fee, 10% of receipts | `internal/lending/lending.go` `DefaultProducts` | Risk owner (credit) | _unassigned_ | Placeholder |
| 19 | Credit limits per product | R5 000 to R500 000 (overdraft), R10 000 to R5 000 000 (loan), R10 000 to R2 000 000 (RBF) | same | Risk owner (credit) | _unassigned_ | Placeholder |
| 20 | Affordability: multiple of monthly inflow | 1 (overdraft), 3 (term loan), 2 (revenue-based) | `internal/credit/credit.go` `Affordability` | Risk owner (credit) | _unassigned_ | Placeholder; NCA affordability rules apply to persons |
| 21 | Card merchant discount rate | 2.75% | `ACQUIRING_MDR_BPS`, `services/bank/acquiring-service` | Head of Payments operations | _unassigned_ | Placeholder until an acquirer is chosen |

| 22 | Risk weights and credit conversion | cash at banks 20%, business loans 100%, personal loans 75%, acquirer 20%, undrawn 10% CCF | `services/bank/treasury-risk/internal/prudential` `DefaultParams` | Risk owner | _unassigned_ | Placeholder; BA 200 defines the real ones |
| 23 | Capital minimums | total capital 11.5%, leverage 4% | same | Risk owner and board | _unassigned_ | Placeholder (Directive 6/2017 to verify) |
| 24 | Liquidity run-off rates and LCR minimum | retail and savings 10%, business 40%, foreign 40%, inflows 50% capped at 75%; LCR 100% | same | Head of Treasury | _unassigned_ | Placeholder; BA 300 defines the real ones |
| 25 | Capital and gross income used by the report | configured (`TREASURY_CAPITAL_MINOR`, `TREASURY_GROSS_INCOME_MINOR`) | treasury-risk | CFO | _unassigned_ | Zero until capital is raised |

| 26 | Deposit insurance cover per depositor | R100 000 | `internal/prudential/scv.go` `CoverLimit` | CFO | _unassigned_ | Placeholder: confirm the CODI limit and which deposits qualify |
| 27 | Default notice timing | issued at 20 days overdue; 14 days to respond | `internal/lending/notices.go` | Risk owner (credit) and legal | _unassigned_ | Placeholder: verify against NCA s.129 and its timelines |
| 28 | Individual annual foreign allowance | R1 000 000 a calendar year (`FX_PERSON_ANNUAL_ALLOWANCE_MINOR`) | `services/bank/fx-service/internal/store` | MLRO and Head of Treasury | _unassigned_ | Placeholder: verify the single discretionary allowance and the tax-clearance route with an authorised dealer |
| 29 | Card limits: per transaction and per day | default R10 000 and R25 000; ceilings R50 000 and R100 000 (`CARDS_DEFAULT_*`, `CARDS_MAX_*`) | `services/bank/card-service/cmd/card-service/main.go` | Risk owner and Head of Product | _unassigned_ | Placeholder: set from fraud loss data and the programme manager's rules |
| 30 | Card blocked merchant categories and open cards per party | MCC 7995, 7801, 7802; 5 open cards (`CARDS_BLOCKED_MCC`, `CARDS_MAX_PER_PARTY`) | `services/bank/card-service/cmd/card-service/main.go` | CCO and Head of Product | _unassigned_ | Placeholder: confirm gambling categories with compliance; decide the policy on crypto-exchange merchants |
| 31 | Write-off timing and credit bureau rules | write off at 90 days past due after a default notice (`LENDING_WRITE_OFF_AFTER_DAYS`); adverse bureau record needs a written reason of 30+ characters | `services/bank/lending-service/internal/lending/writeoff.go`, `lending.go` | Risk owner (credit) and CFO | _unassigned_ | Placeholder: set from the credit policy and IFRS 9 staging; confirm NCA timing with the lawyer |
| 32 | Referral fee ceiling and delivery/register timing | largest fee an offer may allow R5 000 per introduction (`PARTNER_OFFERS_MAX_FEE`); register extract trusted for 48h (`LICENCE_REGISTER_MAX_AGE`); provider hidden if not confirmed in 35 days (`LICENCE_RECHECK_MAX_AGE`); delivery gives up after 6 attempts | `services/platform/partner-offers/internal/offers/offers.go`, `delivery.go`, `cmd/partner-offers/main.go` | CCO and CFO | _unassigned_ | Placeholder: set with the lawyer's view on referral fees and the real register's refresh rate |

Thresholds 17 to 21 and 32 belong to products that `pkg/licence` keeps off in production until a decision approves them
(`FORGE_APPROVAL_<PRODUCT>=DEC-nnnn`).

**Not thresholds, but also owned:** the outbound reserve window of 7 days (the ledger's maximum pending timeout),
which is set by the ledger's limits, and the inbound credit lookback of 48 hours.

To close an item: the owner confirms the value in writing, the owner's name goes in the table, and the status moves to
"Confirmed". Record the date in `docs/DECISIONS.md`.
