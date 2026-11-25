# FORGE MUTUAL: the complete document

> All amounts are in rand, converted from the concept document's US-dollar figures at an assumed R18.00 per US dollar (ASSUMPTION; update with the rate at the time of use).
>
> Assembled 2026-11-25 from everything in this repository. Part I is new: it pulls the model, the money, the sustainability case and the
> status into one argument. Parts II to V reproduce every source document in full, so nothing is lost. **Nothing here is approved, audited
> or legal or financial advice.** Figures marked ASSUMPTION come from the concept document and have not been tested with customers; figures
> marked VERIFY are regulatory facts to confirm with the lawyer. Forge has no licence, no customers and holds no money.

## Contents

- **Part I. The whole picture** (this part)
  1. What Forge Mutual is, in one page
  2. Who it serves and why they would switch
  3. What it sells
  4. How it makes money
  5. What it costs to run
  6. Is it sustainable? The honest arithmetic
  7. What the regulator's own forms taught us
  8. The mutual question: why member-owned, and what it costs
  9. How it gets to market: sponsor first, licence second
  10. What is built, what is drafted, what is missing
  11. Risks and how each is handled
  12. Decisions only the founder can make
  13. Timeline
- **Part II. Business:** the concept and context; the bank model (Erebor and Investec); the draft business plan; wealth and digital-asset products
- **Part III. Legal form and governance:** the two legal forms; the draft constitution; the governance structure; the policy map to the regulator's structure; the service standard
- **Part IV. Licence application and regulation:** the licence pack tracker; the regulatory returns and their sources
- **Part V. Technology and operations:** the architecture; hosting; the project status (README)

---

# Part I. The whole picture

## 1. What Forge Mutual is, in one page

Forge Mutual is a proposed South African bank for technology companies and the people who build them. A company holds rand and US dollars
(and later euro and sterling) in one workspace, receives revenue from foreign customers, pays staff, suppliers and contractors at home and
abroad, and approves payments as a team. It is built on open-source software that runs in South Africa under Forge's control.

It is planned as a **mutual bank**: owned by its depositor-members, one member one vote, with surplus returned to members as better pricing or
rebates rather than paid to outside shareholders. The ordinary **bank** route under the Banks Act is kept open, and the software supports
both without change.

It reaches customers in two stages. First, **through a sponsor bank**, which holds the deposits and the licence while Forge runs the product
(the way Mercury began in the United States). Second, **on its own licence**, once it has capital, a board, members and a track record.

Its edge is specialism, not size: one all-in price for cross-border payments with the spread disclosed, a US-dollar account inside a South
African business workspace, team approvals, and controls built into the software. Its weaknesses are equally clear: capital, reach, and
speed to a licence, against Capitec, the large banks, Revolut, Wise and Lesaka with Bank Zero.

## 2. Who it serves and why they would switch

**First customers (decided):** technology companies.

| Segment | What they need | Why it matters |
|---|---|---|
| Startups and scale-ups | Fast onboarding, multi-currency treasury, payroll, approvals | Deposits swing with funding rounds |
| Profitable tech SMEs, agencies, studios | USD receipts, contractor payouts, FX | The most stable early base |
| AI, data and deep-tech firms | Large USD cloud bills and contractor payments | High FX volume |
| Founders personally | Personal multi-currency, later share-option and exit flows | Kept strictly separate from the company |

**Adjacent:** cross-border SMEs (importers, exporters, freelancers paid from abroad), which broaden the deposit base and do not follow funding cycles.
**Later:** platforms embedding Forge, funds, other African countries through partner banks, and a network of independent micro-lenders.

**Why they would switch.** Receiving USD from a foreign client and paying USD costs is slow, opaque and manual at the large banks, with a
published fee plus an undisclosed FX margin. Forge's test is simple and measurable: **if it cannot beat about 1% all-in cost with same-day
settlement on real customer statements, cross-border payments are not the wedge.** Realistically a company opens Forge as a *second*
account for foreign revenue first and moves its main account later.

**How big.** The serviceable market is an estimate: a few thousand startups and scale-ups plus several thousand active tech SMEs, agencies and
exporters (ASSUMPTION; earlier counts of 1,246 startups, 53,082 tech companies and 91,816 registered software companies are unverified).

## 3. What it sells

| Product | State | Needs before customers can have it |
|---|---|---|
| ZAR business and personal accounts | Built | A sponsor bank |
| USD, EUR and GBP accounts, inbound SWIFT, FX conversion | Built | A partner bank and an authorised dealer |
| Domestic payments, payouts and batch payments with approvals | Built | Sponsor rails |
| Team workspace, roles, approvals, statements | Built | Live sign-in (WorkOS) |
| Savings and 32- and 90-day notice deposits with daily interest | Built, switched off | A licence decision |
| Overdrafts, working-capital loans, revenue-based financing | Built, switched off | NCR registration, a credit policy, capital |
| Merchant card acceptance: payment links and point of sale | Built, switched off | An acquiring bank |
| Virtual debit cards | Built, switched off | A card programme manager and processor |
| Third-party investment and crypto offers (provided, licensed and monitored by the third party, never by Forge) | Built, switched off | A licensed provider and legal review |
| Deposit tokens and on-chain settlement | Built behind a licence gate | Own licence and legal clearance |
| Relationship-manager service standard | Written | Staff |

Non-goals: DeFi, self-custody, Forge-issued stablecoins, a Forge card scheme, mass-market marketing.

## 4. How it makes money

Forge earns from six sources, phased in as each partner or licence arrives.

| Source | How | When | Depends on |
|---|---|---|---|
| **Subscriptions** | Monthly workspace tiers | Early | Product depth and trust |
| **FX spread** | A disclosed margin on each conversion | Launch | Authorised-dealer partner |
| **Payment and payout fees** | Per cross-border payment, per batch | Launch | Sponsor and partner banks |
| **Net interest (float)** | Interest earned on balances; on a sponsor this is shared with the sponsor | Launch | Deposit size and sponsor terms |
| **Card and acquiring fees** | Interchange on cards; fees on merchant takings | Year 2 | Programme manager, acquirer |
| **Platform fees** | Platforms embedding Forge accounts | Year 2 | Partner contracts |
| **Credit** | Interest on overdrafts, working-capital loans and revenue-based financing | Year 3 or later | Capital, NCR registration, credit policy |
| **Referral fees** | From licensed third parties, capped per offer and told to the customer | When offers are on | Provider and legal review |
| **MFI platform fees** | Software and service fees from independent micro-lenders | Year 2 pilot | Profitable franchisees |

**Pricing principles:** one all-in price; the FX spread disclosed; no hidden margins; fees fall as volume grows; any change to pricing,
rewards or credit needs legal review.

**What a mutual changes.** A mutual does not pay dividends to outside owners. Surplus first goes to the reserves the Act requires; the rest
returns to members as lower fees, better rates or a rebate in proportion to their business with the bank. So the goal is not maximum
profit but **enough surplus to grow capital** (which a mutual must mostly build from retained earnings) and then to share the rest.

## 5. What it costs to run

From the concept document (ASSUMPTION): operating costs grow from about **R36 million to R288 million a year over five years**, and variable
costs are about **25% of revenue**. That model leaves out four costs that the regulatory work has since made concrete:

1. **The sponsor bank's share** of interest and fees during the sponsor stage (unknown until a term sheet).
2. **Compliance and control headcount:** money-laundering reporting officer, compliance, risk, internal audit, IT risk; a board with
   independent directors; an external auditor. Most of the 37 governance seats carry a salary or a fee.
3. **Credit losses** once lending is switched on.
4. **The cost of holding liquid assets and a reserve balance** once Forge is a registered mutual bank (section 7).

Technology is not the dominant cost: the stack is open source and self-hosted, and much of it is built.

## 6. Is it sustainable? The honest arithmetic

**The concept document's scenarios (ASSUMPTION, converted to rand):**

| Year 5 | Conservative | Base | Optimistic |
|---|---|---|---|
| Customers | 3,200 | 9,000 | 18,000 |
| Revenue | about R32.4 million | about R160 million | about R499 million |
| Operating result | about -R82.8 million | about -R59.4 million | about +R86.4 million |
| Deposits | about R864 million | about R4.86 billion | about R16.2 billion |
| Peak funding need | about R319 million | about R356 million | about R302 million |
| Customers to break even | about 10,900 | about 10,100 | about 10,400 |

Revenue per customer is assumed at R9,000, R16,200 or R25,200 a year, growing 10% a year. For comparison, Mercury earns about R39,100 per customer;
South African companies are smaller, so even the conservative case may be generous.

**What the arithmetic says.**

- **Break-even needs about 10,000 business customers** in every scenario. That is close to the whole South African serviceable market. So
  sustainability depends on widening the base: cross-border SMEs, founders' personal accounts, platforms embedding Forge, other African
  markets through partner banks, and the micro-lender network.
- **The funding need is similar in every scenario (about R270 million to R360 million)**, because spending grows with success. The minimum
  regulatory capital (R10 million, VERIFY) is not the need.
- **Revenue per customer is the lever.** At R16,200 a year, break-even is about 10,000 customers; every R1,800 more per customer brings it
  materially closer. The biggest per-customer earners are FX volume and, later, credit.
- **Float income is real but shared.** On a sponsor, interest on balances is split with the sponsor; on its own licence Forge keeps it, but
  must then hold liquid assets and a reserve that earn little (section 7).

**A simple way to reason about one customer (illustrative, ASSUMPTION):**

  revenue per customer per year = subscription + (FX volume × spread) + (payments × fee) + (average balance × net interest share)

For a tech SME converting R9 million a year at a 0.6% disclosed spread, paying a R150 subscription a month, making 100 cross-border payments
at a modest fee, and holding an average R1 million balance at a 1% net share, the spread (about R54,000), subscription (about R1,800) and float
(about R10,000) together exceed the R16,200 base case. This is why the plan's first proof point is measured FX economics on real statements.

**What makes it sustainable as a mutual.**

1. **Retained surplus builds capital.** A mutual cannot easily raise outside equity, so it must earn and keep enough to grow its capital
   in step with deposits (capital adequacy, DI 400). Distributions only come after that.
2. **Low cost to serve.** Software does the work of branches; open source avoids licence fees; controls are automated.
3. **Diversified income.** Fees and FX do not depend on rates; float and credit do. The mix reduces dependence on any one.
4. **Diversified deposits.** Technology deposits are correlated and volatile (the Silicon Valley Bank lesson). Concentration limits,
   stress tests and adjacent SME customers spread the risk.
5. **Sponsor first.** Revenue starts before the licence, and the track record shortens the licence process.

**Where it could fail.** Too few customers at too low revenue per customer; a sponsor that takes most of the margin; correlated deposit
outflows in a funding drought; capital raised too slowly to grow; the cost of compliance arriving before the revenue.

## 7. What the regulator's own forms taught us

Building all 27 mutual-bank returns from the Prudential Authority's published forms produced findings that change the model:

1. **A balance at a sponsor bank is not a liquid asset.** Under the Mutual Banks Act only Reserve Bank notes and coin, gold, Reserve Bank
   clearing balances, Treasury bills and government, Reserve Bank and Land Bank securities count. A registered Forge would need about **5% of
   adjusted liabilities in those instruments plus a 2.5% average reserve balance with the Reserve Bank** (VERIFY the current rates). The
   reserve earns nothing; Treasury bills earn less than loans. On R1 billion of deposits that is roughly R25 million held at no return plus
   R50 million in low-yield securities: a real cost the model must carry.
2. **Profit only counts as capital once the board appropriates it to a reserve** by formal resolution (DI 400). The governance calendar must
   include that resolution every year.
3. **Concentration on the sponsor.** Every rand placed with one bank is a single exposure to that bank, many times 10% of capital (DI 510).
   Placements must be spread across banks and within the large-exposure limits.
4. **The foreign-currency open position is limited to 10% of capital** (DI 600). With small capital, even modest mismatches between
   customers' USD balances and USD held at the partner bank breach it, so FX positions must be matched daily.
5. **Investment limits** (DI 700, sections 55 and 56) mean no property, shares or subsidiaries beyond what capital allows.

The returns code reports every one of these as a shortfall and refuses to let the declaration go out unless it is stated.

## 8. The mutual question

**Why member-owned.** Trust in a fee-weary market; surplus returned to the people who bank with it; a natural home for savings groups,
stokvels, co-operatives and micro-lenders as members; a lower minimum capital (R10 million against R250 million for a bank, VERIFY).

**What it costs.** No venture equity and no sale; slower growth; capital built mostly from retained surplus and member shares; a membership,
voting and distribution apparatus (built in software: member register, shares with a capital check on redemption, one member one vote,
quorum and majorities, distributions to the cent).

**The open question (ADR-0020).** Whether a mutual suits a membership of technology businesses, and whether members can pay in share capital
before registration, is for the regulatory lawyer. The draft constitution sets out twenty decisions for the founder and the lawyer.

## 9. How it gets to market

| Stage | Bank layer | What changes for customers |
|---|---|---|
| A (sponsor) | A sponsor bank holds deposits and the licence; Forge runs a reconciled sub-ledger | ZAR and USD accounts, payments, FX, workspace |
| B (own licence) | Forge Mutual Bank holds its own licence | Direct payment rails, tokenised deposits, own balance sheet |
| C (cleared) | Unchanged | Stablecoin corridors through a separate licensed entity, if legally cleared |

The software isolates every route-specific detail behind one interface, so moving from the sponsor to Forge's own books is a configuration
change and a migration, not a rebuild. The sponsor agreement must not stop Forge applying for its own licence and must be transferable
to the future bank.

## 10. What is built, what is drafted, what is missing

**Built and tested (local stack, every outside party simulated):** the full banking platform listed in section 3; double-entry ledger on
TigerBeetle; reconciliation; sanctions screening and monitoring; mutual TLS between services; encrypted personal data (AES-256-GCM);
backups proven by a restore; all 27 mutual-bank regulatory returns (15 computed, 9 checked nil returns, 3 not applicable); the DI 099
declaration; a licence gate that keeps unapproved products off in production.

**Drafted (none approved):** 58 policies mapped to the regulator's structure; the constitution; the business plan; the lawyer brief and
sponsor pitch; the licence-application pack (59 requirements tracked); the governance structure with 37 roles.

**Missing:** a lawyer; capital; a board and executives (1 of 37 roles filled); six more founders; a sponsor bank; live WorkOS, Didit and a
licensed sanctions list; production hosting; a penetration test; customer evidence; a real financial model.

## 11. Risks and how each is handled

| Risk | Handling |
|---|---|
| Sponsor dependence | Two or three sponsors; terms that allow exit and transfer to the future bank |
| USD partner dependence | A second partner as volume allows |
| Licensing sequence | Lawyer first; sponsor stage earns revenue meanwhile |
| Deposit concentration and volatility | Concentration limits, stress tests, adjacent customers, liquidity ladder (DI 300) |
| Liquid asset and reserve cost | Priced into the model; Treasury bills held in proportion |
| Unit economics | Rebuilt from interview and pilot data; widen the customer base |
| Competition | Specialise; do not compete on price or reach |
| Capital | Milestone funding; retained surplus; member shares |
| Customer confusion about who holds money | Every balance shows its holder and insurance status |
| Data and consent | Consent enforced in code; data never sold |
| Software defects in money code | Tests decide; ledger invariants; two-person approvals |

## 12. Decisions only the founder can make

Legal form (with the lawyer); the 20 constitution decisions; who the founders, directors and executives are; how capital is raised; which
sponsor; owners for 32 thresholds; sector appetite (ADR-0012); pricing; whether to switch on each built product.

## 13. Timeline (estimates; confirm with the lawyer)

| Milestone | When, if started now |
|---|---|
| Lawyer engaged, legal form decided | 1 to 2 months |
| Sponsor signed, providers live, hosting up, key people hired | 6 to 12 months |
| Pilot with about 20 design partners | 9 to 15 months |
| Licence application complete (capital, board, founders, approved policies) | 9 to 15 months |
| Registration by the Prudential Authority | a further 18 to 24 months or more |
| Own licence | roughly 2.5 to 3.5 years |

The rest of this document is the detail behind every line above.
