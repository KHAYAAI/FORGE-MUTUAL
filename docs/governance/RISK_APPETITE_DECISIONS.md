# Risk appetite: decisions for the owner

> For **Tebogo Mvelase** to complete. Each line is a number in
> [`risk-appetite-statement.md`](../regulatory/policies/risk-appetite-statement.md). For each one, pick an option or
> write your own in the last column, then sign at the bottom. These are not final until the board (once it exists)
> adopts them on the Chief Risk Officer's recommendation. Until then the code keeps using the placeholder values.
>
> Recommendations assume the **mutual bank** route. A mutual cannot quickly raise outside equity when it runs short,
> so it should hold more buffer than a bank of the same size. Where the bank route would change the answer, it says so.

## How to read the options

- **Cautious:** the safest choice; slower growth.
- **Recommended:** what I'd suggest for a new, small, tech-led mutual.
- **Bolder:** possible, but the regulator and a sponsor bank will ask harder questions.

## Capital

| # | Measure | Regulatory floor | Cautious | **Recommended** | Bolder | Why | Your decision |
|---|---|---|---|---|---|---|---|
| 1 | Total capital adequacy ratio | About 11.5% (code default; confirm with PA) | 20% | **18%** | 15% | New banks hold well above the floor until they have a track record; a mutual can't top up quickly. Bank route: 15% would be enough. | |
| 2 | Leverage ratio | 4% | 10% | **8%** | 6% | Simple backstop. Forge's assets are mostly cash at first, so this is cheap to meet. | |
| 3 | Capital above the legal minimum | R10m mutual / R250m bank | 3× minimum | **2× minimum (R20m)** | 1.5× | Operating losses in years 1 to 3 eat capital; this keeps the bank above the minimum while it grows. | |

## Liquidity

| # | Measure | Floor | Cautious | **Recommended** | Bolder | Why | Your decision |
|---|---|---|---|---|---|---|---|
| 4 | Liquidity coverage ratio | 100% | 200% | **150%** | 120% | Tech businesses move money fast, and deposits can leave in a day. | |
| 5 | Net stable funding ratio | 100% | 130% | **120%** | 110% | Little long-term lending at first, so easy to meet. | |
| 6 | Days survived under own stress | none | 90 | **60** | 30 | Gives time to act before a sponsor or the PA has to. | |

## Concentration

| # | Measure | Cautious | **Recommended** | Bolder | Why | Your decision |
|---|---|---|---|---|---|---|
| 7 | Largest single depositor as % of deposits (threshold 1) | 5% | **10%** for the first 2 years, then 5% | 15% | Early on a few large tech clients dominate. Cap them, and warn at two-thirds of the limit. | |
| 8 | Top 20 depositors as % of deposits | 30% | **40%** | 50% | Same reason. | |
| 9 | Deposits from the crypto sector as % of deposits | 10% | **20%** | 35% | Crypto flows are volatile and correlated. This is the tech and crypto leaning, but capped. | |
| 10 | Largest single credit exposure as % of capital | 10% | **15%** | 25% | Banks Act large-exposure rules apply above this; keep well inside them. | |

## Credit

| # | Measure | Cautious | **Recommended** | Bolder | Why | Your decision |
|---|---|---|---|---|---|---|
| 11 | Who we lend to in year 1 | Overdrafts to members with 12 months' history | **Tech businesses: overdrafts and working capital; no unsecured personal loans** | Add revenue-based financing to crypto businesses | Lend only where we can see the cash flows in our own accounts. | |
| 12 | Total lending as % of deposits | 20% | **35%** | 60% | A mutual earns from lending, but losses come out of members' capital. | |
| 13 | Non-performing loans | 2% | **4%** | 6% | Industry SME levels are higher; be stricter while the book is small. | |
| 14 | Lending to directors, staff and their relatives | None | **Normal terms, board-approved, max 2% of capital in total** | 5% | Related-party lending is a common failure in small and mutual banks. | |

## Crypto

| # | Statement | Cautious | **Recommended** | Bolder | Your decision |
|---|---|---|---|---|---|
| 15 | Which crypto businesses we bank | None at launch | **Only FSCA-licensed CASPs, with enhanced due diligence and travel-rule checks** | Also unlicensed crypto start-ups in a sandbox | |
| 16 | Crypto on our own balance sheet | Never | **None until a separate licence decision** | Hold treasury stablecoins | |
| 17 | Lending against crypto collateral | Never | **Not in year 1** | Up to 30% loan-to-value | |

## Zero tolerance (recommended to accept as written)

| # | Statement | Accept? |
|---|---|---|
| 18 | Knowingly dealing with a sanctioned person | |
| 19 | Tipping off a customer about a report or investigation | |
| 20 | Unreconciled ledger differences older than 1 business day | |
| 21 | Personal data outside South Africa without a legal basis | |
| 22 | Any production change without a second reviewer | |

## Technology and operations

| # | Measure | Cautious | **Recommended** | Bolder | Your decision |
|---|---|---|---|---|---|
| 23 | Recovery time for payments | 1 hour | **4 hours** | 8 hours | |
| 24 | Data loss allowed for the ledger | Zero | **Zero** | Zero (not negotiable) | |
| 25 | Critical security findings fixed within | 3 days | **7 days** | 14 days | |
| 26 | Operational losses a year, as % of income | 1% | **2%** | 5% | |
| 27 | Customer complaints resolved within | 10 days | **15 business days** | 20 days | |

## Escalation

| # | Rule | **Recommended** | Your decision |
|---|---|---|---|
| 28 | Early warning (amber) at | **80% of the limit** | |
| 29 | Breach (red) reported to the board chair and Risk Committee chair within | **1 business day** | |
| 30 | Who may approve a temporary breach | **Board only, for up to 30 days, with a plan** | |

## Sign-off

| | Name | Signature | Date |
|---|---|---|---|
| Founder (owner's view) | Tebogo Mvelase | | |
| Chief Risk Officer (recommends) | VACANT | | |
| Board (adopts) | VACANT | | |

After signing, the CRO updates `risk-appetite-statement.md`, `docs/THRESHOLDS.md` (thresholds 1 and 10 to 11) and
`prudential.DefaultParams` (rows 1, 2 and 4), and records the decision in `docs/DECISIONS.md`.
