# FORGE MUTUAL: the complete document

> Money is shown in rand first, with the concept document's original US-dollar figure in brackets, converted at an assumed R18.00 per US dollar (ASSUMPTION; update with the rate at the time of use).
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

From the concept document (ASSUMPTION): operating costs grow from about **R36 million (US$2 million) to R288 million (US$16 million) a year over five years**, and variable
costs are about **25% of revenue**. That model leaves out four costs that the regulatory work has since made concrete:

1. **The sponsor bank's share** of interest and fees during the sponsor stage (unknown until a term sheet).
2. **Compliance and control headcount:** money-laundering reporting officer, compliance, risk, internal audit, IT risk; a board with
   independent directors; an external auditor. Most of the 37 governance seats carry a salary or a fee.
3. **Credit losses** once lending is switched on.
4. **The cost of holding liquid assets and a reserve balance** once Forge is a registered mutual bank (section 7).

Technology is not the dominant cost: the stack is open source and self-hosted, and much of it is built.

## 6. Is it sustainable? The honest arithmetic

**The concept document's scenarios (ASSUMPTION, in rand with US dollars in brackets):**

| Year 5 | Conservative | Base | Optimistic |
|---|---|---|---|
| Customers | 3,200 | 9,000 | 18,000 |
| Revenue | about R32.4 million (US$1.8 million) | about R160 million (US$8.9 million) | about R499 million (US$27.7 million) |
| Operating result | about -R82.8 million (-US$4.6 million) | about -R59.4 million (-US$3.3 million) | about +R86.4 million (+US$4.8 million) |
| Deposits | about R864 million (US$48 million) | about R4.86 billion (US$270 million) | about R16.2 billion (US$900 million) |
| Peak funding need | about R319 million (US$17.7 million) | about R356 million (US$19.8 million) | about R302 million (US$16.8 million) |
| Customers to break even | about 10,900 | about 10,100 | about 10,400 |

Revenue per customer is assumed at R9,000 (US$500), R16,200 (US$900) or R25,200 (US$1,400) a year, growing 10% a year. For comparison, Mercury earns about R39,100 (US$2,170) per customer;
South African companies are smaller, so even the conservative case may be generous.

**What the arithmetic says.**

- **Break-even needs about 10,000 business customers** in every scenario. That is close to the whole South African serviceable market. So
  sustainability depends on widening the base: cross-border SMEs, founders' personal accounts, platforms embedding Forge, other African
  markets through partner banks, and the micro-lender network.
- **The funding need is similar in every scenario (about R270 million (US$15 million) to R360 million (US$20 million))**, because spending grows with success. The minimum
  regulatory capital (R10 million, VERIFY) is not the need.
- **Revenue per customer is the lever.** At R16,200 (US$900) a year, break-even is about 10,000 customers; every R1,800 (US$100) more per customer brings it
  materially closer. The biggest per-customer earners are FX volume and, later, credit.
- **Float income is real but shared.** On a sponsor, interest on balances is split with the sponsor; on its own licence Forge keeps it, but
  must then hold liquid assets and a reserve that earn little (section 7).

**A simple way to reason about one customer (illustrative, ASSUMPTION):**

  revenue per customer per year = subscription + (FX volume × spread) + (payments × fee) + (average balance × net interest share)

For a tech SME converting R9 million (US$500,000) a year at a 0.6% disclosed spread, paying a R150 subscription a month, making 100 cross-border payments
at a modest fee, and holding an average R1 million balance at a 1% net share, the spread (about R54,000 (US$3,000)), subscription (about R1,800) and float
(about R10,000) together exceed the R16,200 (US$900) base case. This is why the plan's first proof point is measured FX economics on real statements.

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


---

# Part II. Business


## The concept and business context

*Source: `docs/CONTEXT.md`*

## FORGE Mutual Bank: Business Context

Version 1.0 | 29 October 2026 | Status: concept, Phase 0
Place at `docs/CONTEXT.md`. Companion files: `AGENTS.md` (repository rules), `docs/ARCHITECTURE.md` (technical design).

> **What this file is for.** It explains what Forge Mutual Bank is, who it serves, how it makes money and why it is designed the way it is. Read it before writing specs, product copy or customer-facing features. It is context, not a source of regulatory fact: anything marked VERIFY must be confirmed by a human.
>
> **Status.** Forge is a proposal. It is not licensed, has no customers and holds no money. Figures marked unverified, illustrative or estimated must not appear in external material without checking.

---

### Contents

1. Summary
2. The problem
3. What Forge is
4. Legal structure
5. The staged route and the mutual question
6. Target customers
7. Products
8. Business model
9. Distribution
10. The MFI network
11. Market and competition
12. What makes Forge different
13. Regulatory landscape
14. Technology in brief
15. Relationship to the wider FORGE ecosystem
16. Risks
17. Roadmap
18. Decisions and open questions
19. Facts to verify and corrected figures
20. Glossary

---

### 1. Summary

Forge Mutual Bank is a proposed South African bank and money platform for technology companies and the people who build them. It gives a company one workspace to hold rand and US dollars, receive revenue from foreign customers, pay staff, suppliers and contractors at home and abroad, and manage its money as a team.

It starts on a sponsor bank, which holds the deposits and carries the licence, the way Mercury started in the United States. Later, Forge seeks its own licence, as a mutual bank owned by its members or as a bank under the Banks Act. That decision is still open. Once Forge holds its own licence, the original vision returns: rand, tokenised deposits and, through a separate licensed entity, approved stablecoins, all inside one regulated institution.

The platform is built on open-source, self-hostable software, so the core can run in South Africa under Forge's control. A franchised network of independent micro-lenders (the MFI network) is planned for year 2, to extend Forge's infrastructure and funding to underserved borrowers.

---

### 2. The problem

**For South African technology companies:**

- **Foreign revenue is slow and expensive to receive.** Software firms, agencies and studios often earn in USD from foreign clients. Converting it involves bank fees, a foreign-exchange margin that is not shown on the statement, and settlement of two business days or more.
- **Paying abroad is the same in reverse.** Cloud bills, SaaS tools and offshore contractors are paid in dollars. Each payment carries a published fee (roughly R800 to R900 on a R250,000 transfer at the large banks, from earlier notes, unverified) plus the hidden margin.
- **Exchange-control paperwork is manual.** Purpose codes, invoices and supporting documents add friction to every cross-border payment.
- **Business banking is built for other customers.** The large banks serve corporates; Capitec and the digital banks serve retail and small traders. Few products are designed for a fast, digital company that runs on software and earns in several currencies.
- **Money tools are fragmented.** Banking, FX, payroll, accounting and approvals sit in separate systems joined by spreadsheets.

**For the wider economy (the MFI network, later):**

- Many people and micro-businesses lack affordable credit. About 10 million South Africans are estimated to be over-indebted, and unregistered lenders charge far more than registered ones.
- Small, registered lenders struggle with the fixed cost of software, compliance and funding.

---

### 3. What Forge is

#### 3.1 Positioning

Forge is the money operating system and bank for technology companies and the people who build them, in South Africa first, then Africa through partner banks.

It is not a mass-market bank. Capitec has 26.6 million active clients and 686,000 business clients, so Forge cannot win on price or reach. Forge wins as a specialist.

#### 3.2 Where the idea comes from

| Influence | What Forge takes | What Forge does not take |
|---|---|---|
| Mercury (US) | Software first on partner banks; a licence later; breadth beyond tech | Its scale or US market |
| Erebor (US) | A specialist, API-first bank for tech; on-chain money treated as native | Its capital (about R6.84 billion (US$380 million) raised) or its customer base |
| Capitec | Simplicity and low cost to serve | Mass-market ambition |
| Discovery | Orchestration and consented, data-driven incentives | A large group customer base or years of heavy investment |
| Investec | Community, high value per client, founder-plus-company banking | A decades-old brand |
| Bank Zero | Distribution through partners (alliance banking) | Its zero-fee retail focus |
| Nubank | Credit as the long-term profit engine | Lending from its own balance sheet early |

#### 3.3 Principles

1. **Bank first, crypto second.** On-chain money is a rail, never the book of record.
2. **One ledger per legal entity.** Every balance inside an entity sits in one place.
3. **Custody clarity.** Every balance shows which entity holds it and whether it is deposit-insured.
4. **Closed regulatory perimeter.** Activity stays inside a licensed entity or is handed off with disclosure.
5. **Customer simplicity.** No seed phrases or private keys for regulated balances.
6. **Open and sovereign.** Open-source, self-hostable components with licences checked; no hard dependency on one foreign cloud or vendor.
7. **Compliance built in,** not added afterwards.

---

### 4. Legal structure

Forge is one product built on separate legal entities with separate books. Customers see one login and one account view.

| Entity | Role | Holds | Never holds |
|---|---|---|---|
| **MYFORGE (Pty) Ltd** (platform company) | Builds and runs the product: app, workspace, APIs, customer and consent data, consolidated view | No customer money | Deposits or crypto |
| **Bank layer** | Takes deposits and runs payments. A sponsor bank now; Forge Mutual Bank or a Forge bank later | ZAR deposits; USD and other currencies via partner banks | Crypto or stablecoins |
| **Digital-asset entity** (deferred) | Stablecoin custody and on/off-ramp, licensed by the FSCA as a crypto asset service provider | Stablecoins | Deposits |
| **MFI network** (year 2) | Independent, separately licensed lenders on the Forge platform and brand | Each MFI's own loan book and, if a registered co-operative, member savings | Another MFI's money or data |

Earlier documents called the platform company "Forge Technologies (Pty) Ltd". The registered company is MYFORGE (Pty) Ltd.

**Why separate entities:**

- South African regulators say stablecoins are not money or funds under the payments law, so a deposit-taker holding crypto balances is a prudential problem.
- On a sponsor bank, the sponsor's books are the legal record and Forge's ledger is a sub-ledger, reconciled daily.
- Independent MFIs avoid holding-out risk, where customers or regulators treat the bank as responsible for a lender it does not control.
- The pattern exists elsewhere: Infinite runs hybrid fiat and stablecoin business accounts powered by Erebor, with the technology company and the bank kept separate.

Value moves between the bank and the digital-asset entity only through a formal, reconciled settlement step, never as one combined transfer.

---

### 5. The staged route and the mutual question

#### 5.1 Three stages

| Stage | Bank layer | On-chain | What changes for customers |
|---|---|---|---|
| A (now) | Sponsor bank holds deposits; Forge runs a sub-ledger | None | ZAR and USD accounts, payments, FX, workspace |
| B (licensed) | Forge holds its own licence | Tokenised deposits inside the bank | Direct rails such as PayShap; tokenised rand deposits |
| C (cleared) | Unchanged | Stablecoin corridors via the separate digital-asset entity | Faster cross-border settlement where legally allowed |

The original idea, one member-owned bank holding rand, tokenised deposits and stablecoins together, is the destination. It cannot be the starting point, because Forge has no licence and stablecoins sit outside the payments law.

#### 5.2 Mutual or bank (ADR-0020, open)

The code supports both. Everything licence-specific sits behind one interface (`BankLayer`), and everything mutual-specific sits in a membership module that is designed but not built.

**What a true mutual would help with**

- **Trust.** A bank owned by its depositors is a credible story in a market tired of fees.
- **The MFI network.** MFIs and co-operatives could become members of the bank, so the network owns its funder (an apex structure, like European co-operative banking groups).
- **Redistribution.** Surplus returns to members as better pricing or rebates, not to outside shareholders.
- **Communities.** Member ownership fits stokvels and savings groups.
- **Possibly a lower capital hurdle** than a commercial bank (the earlier R10M figure is unverified).

**What it costs**

- It cannot easily raise venture equity or be sold.
- It grows more slowly.
- It needs a licence before it can launch as itself.

**Current lean:** a sponsor-led platform now, with a mutual or co-operative structure introduced for the MFI network later. The name "Forge Mutual Bank" is kept as the brand and the long-term destination.

---

### 6. Target customers

#### 6.1 First customer group: technology companies (ADR-0022, accepted)

| Segment | Who | What they need | Notes |
|---|---|---|---|
| Startups and scale-ups | SaaS, marketplaces, consumer apps | Fast onboarding, multi-currency treasury, payroll, team approvals | Deposits swing with funding cycles |
| Profitable tech SMEs | Software houses, agencies, studios | USD receipts from foreign clients, contractor payouts, FX | The most stable early base |
| AI, data and deep-tech | AI labs, hardware and data companies | Large USD cloud spend, contractor payments | Sector eligibility policy applies |
| Founders personally | Founders, executives, senior engineers | Personal multi-currency, later ESOP and exit flows | Always kept separate from the company |

**Why tech companies first:** they earn and spend in foreign currency, adopt software quickly, recommend tools to each other, and are underserved by retail-oriented business banking. Tech is also the brand: a specialist bank for builders.

**The core jobs Forge must do well in v1**

1. Receive USD from a foreign customer and see it arrive quickly.
2. Pay a USD cost (cloud, SaaS, a contractor) without converting to rand first.
3. Convert USD to rand for payroll at a clearly disclosed price.
4. Pay a foreign supplier from rand with one all-in quoted cost.
5. Pay local staff and suppliers.
6. Approve payments as a team, with roles and an audit trail.

#### 6.2 Adjacent customers on the same product

Cross-border SMEs (importers, exporters, freelancers paid from abroad) are welcome on the same product. They broaden the deposit base, do not follow venture funding cycles and have the same cross-border pain.

#### 6.3 Later customer groups

| Segment | When | Why later |
|---|---|---|
| Fintech and regulated web3 firms | After launch | Needs enhanced due diligence and a stance on crypto (ADR-0012) |
| Platforms embedding Forge (BaaS) | Year 2 | Needs a mature core and partner contracts |
| Investors and funds | Year 2 | Fund structures need extra checks |
| MFI network customers | Year 2 pilot | Different customers, different risks (section 10) |
| Other African countries | Years 2 to 3 | Each needs a partner bank and its own regulatory work |

#### 6.4 Not targeted

Mass-market retail, consumer lending, crypto traders looking for an exchange, and any sector excluded by the risk-appetite policy.

#### 6.5 Size of the market

| Source | Count | What it means |
|---|---|---|
| StartupBlink | 1,246 startups | Venture-style core |
| Tracxn | 53,082 companies | Broad, includes companies beyond early stage |
| CIPC registry | 91,816 software companies | Mostly tiny or inactive |

These come from earlier notes and are unverified. The serviceable market is an estimate: a few thousand startups and scale-ups plus several thousand active tech SMEs, agencies and exporters.

**The funding backdrop is thin.** South African startups raised about R1.49 billion (US$83 million) in the first half of 2026, against R5.89 billion (US$327 million) in Egypt, R4.57 billion (US$254 million) in Nigeria and R2.27 billion (US$126 million) in Kenya. Startup runway alone cannot support a bank, which is why profitable tech SMEs, adjacent businesses and other African markets matter.

---

### 7. Products

#### 7.1 v1

- ZAR business accounts, held at the sponsor bank with a Forge sub-ledger.
- A USD account through one partner bank, for receiving foreign revenue and paying USD costs.
- Cross-border payments and FX through an authorised-dealer partner, quoted as one all-in rand price with the spread disclosed.
- Domestic payments (EFT) through the sponsor.
- Business and personal identity checks through Didit.
- A web workspace: balances by currency and entity, payments, approvals, team roles, statements.
- A back-office console for Forge operations.
- Built-in compliance: sanctions screening, transaction monitoring, reporting to the Financial Intelligence Centre.
- Deposit concentration and outflow alerts, because tech deposits are volatile.

#### 7.2 Later

| Product | When |
|---|---|
| EUR and GBP accounts | After the USD pattern is proven |
| Bulk payouts and accounting integrations | Late v1 or year 2 |
| Cards, through a rented issuer processor | Year 2 |
| BaaS partner API | Year 2 |
| Mobile app | Year 2 |
| Partner-funded credit (revenue-based financing) | Year 3 or later |
| PayShap direct participation | Own licence only |
| Tokenised rand deposits | Own licence only |
| Stablecoin corridors | Digital-asset entity, once legally cleared |

#### 7.3 Explicit non-goals

DeFi (swaps, staking, yield), self-custody products, Forge-issued stablecoins, a Forge card BIN, CBDC work and mass-market marketing.

---

### 8. Business model

#### 8.1 Four product pillars

1. **Core money.** Multi-currency accounts, payments, FX and treasury.
2. **Orchestration.** One workspace where payments, FX, payroll, tax and accounting connect, with rules such as "move 10% of every receipt into a tax pot".
3. **Incentives and data.** With the customer's consent, revenue and usage data first feed dashboards, then transparent pricing and rewards, and later credit through partner capital.
4. **Community and founder-plus-company banking.** A founder community as a trusted distribution channel, and a personal tier for founders kept strictly separate from their company.

#### 8.2 Revenue layers

| Layer | Source | Depends on | Phase |
|---|---|---|---|
| Subscriptions | Workspace tiers | Product depth and trust | Early |
| Transactions | FX spreads, payout fees, later card interchange | Authorised-dealer partner, sponsor and partner banks | Launch |
| Float income | Interest on balances, shared with the sponsor | Deposit size and sponsor terms | Launch |
| Platform fees | BaaS partners | Partners embedding Forge | Year 2 |
| Credit | Revenue-based financing on partner capital | Capital, registration, risk models | Year 3 or later |
| MFI platform | Software and service fees from MFIs | Profitable franchisees | Year 2 pilot |

**Pricing principles:** quote one all-in price; disclose the FX spread; no hidden margins; fees fall as volume grows. Pricing rules live in policy, not code, and any change to pricing, rewards or credit needs legal review.

#### 8.3 Illustrative economics

Money is shown in rand first, with the concept document's original US-dollar figure in brackets, converted at an assumed R18.00 per US dollar (ASSUMPTION; update with the rate at the time of use).

These are assumptions, not data. Revenue per customer starts at R9,000 (US$500), R16,200 (US$900) or R25,200 (US$1,400) a year and grows 10% a year; variable costs are 25% of revenue; operating costs grow from R36 million (US$2 million) to R288 million (US$16 million) over five years.

| Year 5 | Conservative | Base | Optimistic |
|---|---|---|---|
| Customers | 3,200 | 9,000 | 18,000 |
| Revenue | about R32.4 million (US$1.8 million) | about R160 million (US$8.9 million) | about R499 million (US$27.7 million) |
| Operating result | about -R82.8 million (-US$4.6 million) | about -R59.4 million (-US$3.3 million) | about +R86.4 million (+US$4.8 million) |
| Deposits | about R864 million (US$48 million) | about R4.86 billion (US$270 million) | about R16.2 billion (US$900 million) |
| Peak funding need | about R319 million (US$17.7 million) | about R356 million (US$19.8 million) | about R302 million (US$16.8 million) |
| Customers to break even | about 10,900 | about 10,100 | about 10,400 |

**What this means**

- Break-even needs about 10,000 business customers, close to the whole South African serviceable market, so expansion is necessary.
- The funding need is similar across scenarios, because spending grows with success.
- Mercury earns about R39,100 (US$2,170) per customer (R11.7 billion (US$650 million) over about 300,000 customers, a calculation from reported figures). South African companies are smaller, so even the conservative case may be generous.
- The model excludes the sponsor bank's revenue share, licence capital, credit losses and compliance headcount. Real economics are harder.

The customer interviews (section 17) replace these assumptions with real prices and balances.

---

### 9. Distribution

| Channel | How it works | Phase |
|---|---|---|
| Design partners | About 20 tech companies recruited through customer interviews | Now |
| Community and referrals | A founder community (cohorts, events, introductions); referral credits from a capped pool | Early |
| Ecosystem partners | Accelerators, venture funds, accounting firms, cloud marketplaces; co-branded onboarding | Early to mid |
| Sponsor and partner banks | Rails, deposits and foreign-currency accounts delivered through banks | From launch |
| Embedded (BaaS) | Accounting, payroll and marketplace platforms offer Forge accounts inside their software | Year 2 |
| MFI network | Independent licensed lenders on the Forge brand and platform | Year 2 pilot |

Partners lower acquisition cost but hold the customer relationship, so each needs a revenue-share agreement.

**Realistic entry point:** a company rarely moves its main bank account to a new provider. Forge's first role is a second account for foreign revenue and payments, earning trust before becoming the main account.

---

### 10. The MFI network

#### 10.1 The idea

A franchise-style network of independent micro-lenders using Forge's platform, brand and funding, modelled on the McDonald's system rather than the burger: franchisees pay for a tested way of operating, not just a name.

| What Forge supplies | Detail |
|---|---|
| Brand and operating manual | How to run a compliant micro-lender |
| The platform | White-label lending software built on Fineract, with limits enforced in software so a lender cannot offer a product, price or sector its policy forbids |
| Funding | Wholesale credit from the bank layer, plus development-finance funding |
| Shared compliance | Identity checks, sanctions screening, reporting |
| Credit data | Shared scoring that improves as lenders join (a credit bureau is a regulated business and needs its own registration; VERIFY) |

Natural first members are already-registered lenders, savings co-operatives, stokvels, church groups and community savings clubs that have trust but lack infrastructure and funding.

#### 10.2 Structure: official but arm's-length

- Each MFI is its own legal entity, registered or licensed in its own right: a credit provider registered with the National Credit Regulator, or a co-operative financial institution if it holds member savings.
- It uses the Forge brand under a licence agreement.
- The bank is its funder, not its owner. If ADR-0020 chooses a mutual, MFIs could become members of the bank.
- Fees come from services to the lending business, never from recruiting more franchisees, to avoid pyramid-scheme concerns.
- An unofficial spin-off is ruled out because of holding-out risk.

#### 10.3 What it could change

- Cheaper, registered credit replacing unregistered lenders.
- Local ownership, so lending profits stay in communities.
- Formal savings that build assets, not just debt.
- Repayment history that lets a small loan grow into a larger one, which is an incentive to build a business.

#### 10.4 What it will not do on its own

- Randomised studies of microcredit in seven countries found no significant effect on average household income, though they showed more business choice and expansion for households that already ran businesses.
- Franchising spreads credit risk across many operators; it does not remove it.
- Margins are capped and over-indebtedness is high.
- The co-operative route for member savings is small: 6 co-operative banks and 24 co-operative financial institutions held R594M of deposits in 2023/24.
- Funding illiquid microloans with volatile tech deposits recreates the funding mismatch behind bank runs, so exposure is capped and term or development-finance funding is preferred.

#### 10.5 How it is tested

Pilot with three to five already-registered lenders. Measure portfolio at risk, cost per loan, the loan book each lender needs to break even, the rate borrowers pay before and after, borrowers' business revenue, and whether pooled data improves default prediction. Open the franchise only if pilot lenders are profitable.

An alternative to test first is the agent-network model used in Nigeria (Moniepoint, OPay, PalmPay), where local agents handle onboarding and payments and lending comes later.

---

### 11. Market and competition

#### 11.1 Competitors

| Competitor | Threat | Why |
|---|---|---|
| Capitec | High | 686,000 business clients, up 123% in a year; a stokvel account for roughly 800,000 stokvels; price and reach |
| Lesaka and Bank Zero | High for small business | Lesaka is buying Bank Zero (about R700M deposits, break-even expected December 2026) and plans FX and cross-border services |
| Revolut | Medium to high | Filed a South African banking licence application in November 2025 |
| Wise | High on price | Conditional authorised-dealer approval, for personal customers first |
| Large banks (FNB, Absa, Standard Bank, Nedbank) | Medium | Trust and balance sheets; slow and opaque on cross-border |
| Payment gateways (Paystack, Stitch, Ozow, Peach, Yoco) | Medium | Own the checkout; could also be partners |
| Remittance players (Mama Money, Mukuru, Shyft) | Low to medium | Personal corridors |
| Crypto exchanges (Luno, VALR) | Low | Not banks |

#### 11.2 Models, not rivals

| | Erebor | Mercury | Forge |
|---|---|---|---|
| Customers | Tech, crypto, defence | Startups and tech, then broader | Tech companies, then adjacent SMEs |
| Structure | Own national charter (opened February 2026) | Partner banks; conditional charter approval April 2026 | Sponsor bank, licence later |
| Scale | R72.9 billion (US$4.05 billion) deposits by end of June 2026 | 300,000+ customers, about R11.7 billion (US$650 million) annualised revenue | Concept |
| Capital | About R6.84 billion (US$380 million) raised | Large | Far smaller |
| Stack | Proprietary | Proprietary | Open-source, self-hostable |

Erebor found demand for crypto-backed lending lower than expected, which is a reminder that on-chain features are not yet why businesses choose a bank.

---

### 12. What makes Forge different

**Where Forge can win**

- One all-in rand price for cross-border payments, with same-day settlement and exchange-control paperwork handled automatically.
- A USD account built into a South African business workspace.
- One workspace connecting payments, FX, payroll and accounting, which a retail-style business account does not offer.
- Open-source, self-hostable infrastructure, which matters for sovereignty and lowers vendor lock-in.
- Software-enforced quality across an MFI network, if it works.

**Where Forge loses**

- Price and reach, against Capitec.
- Trust and balance sheet, against the large banks.
- Capital, against everyone.
- Speed to a licence, against Revolut and Lesaka.

**The test that decides the wedge:** if Forge cannot beat roughly 1% all-in cost with same-day settlement on real customer statements, cross-border payments are not the wedge.

---

### 13. Regulatory landscape

All points need confirmation by counsel (VERIFY).

| Area | Position | Implication for Forge |
|---|---|---|
| Stablecoins | On 28 May 2026 the Reserve Bank and FSCA said crypto assets, including stablecoins, are not money or funds under the National Payment System Act. Foreign-currency stablecoins will not be approved for domestic payments; rand-pegged stablecoins are under study | No stablecoin dependence in v1; a separate entity later |
| Crypto licensing | The FSCA licenses crypto asset service providers under FAIS | The digital-asset entity needs its own licence |
| Exchange control | Cross-border flows need an authorised dealer; draft capital-flow regulations were published in 2026 | An authorised-dealer partner for FX in v1 |
| Banking licence | Prudential Authority licence under the Banks Act or the Mutual Banks Act | Sponsor bank first |
| Payment rails | Direct PayShap participation is for licensed banks | Rails through the sponsor until licensed |
| Deposit insurance | CODI covers bank deposits, not third-party stablecoins | Every balance shows its insurance status |
| Anti-money laundering | FIC Act: KYC, sanctions, reporting through goAML; a Travel Rule for crypto transfers | Compliance built into the platform |
| Data | POPIA governs consent, purpose, retention and cross-border transfer | Consent required for every data use |
| Credit | National Credit Act registration for lenders and credit bureaus | Applies to the MFI network and any credit product |
| Co-operatives | A deposit-taking co-operative must register at 200 members and R1M of deposits | Applies to savings co-op MFIs |

---

### 14. Technology in brief

Full detail is in `docs/ARCHITECTURE.md`.

- **Ledger:** TigerBeetle, one cluster per legal entity, written by one gateway only.
- **Core banking:** Apache Fineract for customers, products and loans; it holds no authoritative balances.
- **Workflows:** Temporal, so every multi-step money operation can be reversed cleanly.
- **Rules:** Open Policy Agent for limits, sectors, permissions and pricing.
- **Events and data:** Kafka, PostgreSQL, OpenTelemetry.
- **Identity:** Keycloak; Didit for identity checks.
- **Bank-layer interface:** a sponsor implementation now and an own-licence implementation later, so moving to Forge's own licence is a configuration change and a migration, not a rewrite.
- **Rules enforced in code:** money as integers, idempotent commands, no crypto in the bank ledger, daily reconciliation against the sponsor, an append-only audit log, consent for data, and default deny.

---

### 15. Relationship to the wider FORGE ecosystem

Forge Mutual Bank sits inside the FORGE ecosystem operated by MYFORGE (Pty) Ltd. Related products include FORGE Payments, the Agent Credit Bureau, FORGE Custody, FORGE Wallet and Enterprise Treasury, built on the Revenue Ontology event ledger.

ADR-0021 decides how they connect. The current lean:

| Existing asset | Proposed role in Forge Mutual Bank |
|---|---|
| TigerBeetle (new) | The bank's ledger; banking needs strict double-entry guarantees |
| Revenue Ontology | Analytics and event enrichment feeding data features, not the bank ledger |
| Hyperswitch fork | Merchant payment acceptance later, if Forge offers it |
| FORGE Custody | Custody for the digital-asset entity, after its external MPC audit and move off the single shared key |
| FORGE Wallet | A candidate for the digital-asset entity's customer wallets, later |
| Agent Credit Bureau and Agent Lending Network | A separate initiative; the MFI network here serves people and micro-businesses, while the agent lending network serves AI agents. Overlap in lenders, governance and data should be decided explicitly |

A short audit of each (task P0-11 in the architecture) decides reuse, adaptation or replacement.

---

### 16. Risks

| Risk | Why it matters | Mitigation |
|---|---|---|
| Sponsor dependence | The sponsor holds the licence, the books and much of the margin | Two or three sponsors; terms signed early |
| Partner-bank dependence (USD) | Losing the partner removes a core v1 product | A second partner as soon as volume allows |
| Licensing sequence | Bank, crypto and authorised-dealer routes run on different timelines | Start in week one; a lawyer on retainer |
| Stablecoin rules | Outside the payments law and changing | Deferred and feature-flagged |
| Deposit concentration | Tech deposits are correlated and volatile (the Silicon Valley Bank lesson) | Diversified customers, concentration limits, stress tests |
| Unit economics | About 10,000 customers to break even, before the sponsor's share | Rebuild from interview data |
| Competition | Capitec, Lesaka-Bank Zero, Revolut and Wise are all moving in | Specialise; do not compete on price |
| Entity confusion | Customers may not know which entity holds what | Entity and insurance status on every balance |
| MFI risks | Funding mismatch, contagion, over-indebtedness | Pilot first, caps, software-enforced limits |
| Data and consent | POPIA applies to every data feature | Consent required, purposes limited, decisions explainable |
| Focus | A bank, a platform, a digital-asset entity and an MFI network are four businesses | Strict sequencing |
| Capital | About R270 million (US$15 million) to R360 million (US$20 million) before profit in the base case | Milestone funding; a sale as an acceptable outcome |
| AI-built software | Defects in money-path code | Tests decide; two human reviews; no real data in development |

---

### 17. Roadmap

| Stage | Timing (estimate) | What happens |
|---|---|---|
| Phase 0 | Weeks 1 to 6 | Customer interviews, regulatory lawyer, sponsor and partner-bank talks, technical spikes, ecosystem audit |
| Phase 1 | Weeks 7 to 18 | Prototype on simulators: onboarding, ledger, ZAR and USD accounts, payments, compliance, workspace |
| Gate 1 | | Ledger decision made; a full demo reconciles |
| Phase 2 | Weeks 19 to 34 | Sponsor and partner-bank sandboxes, FX, reporting, recovery drill |
| Phase 3 | Weeks 35 to 52 | Production, audits, closed launch with about 20 design partners |
| Gate 2 | | Sponsor, partner and regulator sign-off |
| Year 2 | | MFI pilot, BaaS pilot, cards, digital-asset entity application, EUR and GBP |
| Gate 3 | | Pilot MFIs profitable; shared data improves credit scores |
| Year 3 and beyond | | Second country by partner bank; franchise opens if the pilot pays; own licence or a sale |

Estimates assume six to eight people working with coding agents. Calendar time to a sponsor-led launch is about 12 to 18 months; an own licence is about 24 to 42 months or more (VERIFY).

**Three proof points that would make Forge significant**

1. Cross-border payments under about 1% all-in cost with same-day settlement, proven on real statements from 15 to 20 companies.
2. One franchised MFI that is profitable, with defaults in line with expectations.
3. Evidence that pooled data improves credit scores across independent lenders.

---

### 18. Decisions and open questions

| ADR | Decision | Status |
|---|---|---|
| 0019 | Separate legal entities with separate books | Accepted in principle |
| 0020 | Bank layer legal form: mutual or bank, and when | Open; both supported in code; lean is sponsor now, mutual or co-operative for the MFI network later |
| 0021 | Relationship to ForgePay, FORGE Custody and the wider ecosystem | Open; audit in Phase 0 |
| 0022 | First customer group | **Accepted: technology companies** |
| 0012 | Sectors Forge will serve (crypto, defence, gambling and others) | Open; a human risk-appetite decision |
| 0024 | USD partner bank and account model | Open |

**Open business questions**

- What will South African tech companies pay, and what balances will they hold?
- Will they move a second account, and later the main account?
- What revenue share will the sponsor and partner banks require?
- Which sponsor and USD partner banks will work with an unlicensed platform?

---

### 19. Facts to verify and corrected figures

**Corrected from earlier documents**

- Capitec has 26.6M active clients, not 22M.
- GoTyme has more than 12.5M customers, per its chief executive.
- Bank Zero's 275,000 direct and 500,000 alliance customers are unsupported. Sources show over 40,000 funded accounts (April 2025), about R700M deposits and its first alliance partner only in April 2026.
- Erebor raised about R6.84 billion (US$380 million) in total, not R11.4 billion (US$635 million).
- One account holding rand, USDC and tokenised deposits with full deposit insurance is not currently achievable. Third-party stablecoins are not bank liabilities and are not deposit-insured, and only a licensed bank can issue tokenised deposits.

**To verify before relying on**

- Mutual bank capital (earlier R10M figure), licensing timeline (earlier 12 to 24 months) and the Prudential Authority's posture.
- CODI coverage and limits (earlier R100,000 figure) and their application to business and tokenised deposits.
- Exchange-control rules for residents' foreign-currency accounts and inbound USD revenue.
- Authorised-dealer partner responsibilities, including Balance of Payments reporting.
- Large-bank cross-border fees and FX margins, from current price guides and real statements.
- MFI categories, franchise and pyramid-scheme rules, and credit bureau registration.
- Whether Didit meets FIC Act and POPIA requirements.
- The licence and maintenance status of every open-source component.

---

### 20. Glossary

| Term | Meaning |
|---|---|
| AD partner | An authorised dealer in foreign exchange that executes FX and cross-border payments |
| BaaS | Banking-as-a-Service: other companies offering Forge accounts inside their own products |
| CASP | Crypto asset service provider, licensed by the FSCA |
| CODI | Corporation for Deposit Insurance |
| Design partner | An early customer who uses the product and shapes it in exchange for close support |
| MFI | Microfinance institution: a small, registered lender, sometimes a savings co-operative |
| Mutual bank | A bank owned by its members (depositors), regulated under the Mutual Banks Act |
| Partner bank | A bank that holds foreign-currency accounts for Forge's customers |
| Sponsor bank | A licensed bank that holds deposits and provides payment rails until Forge has its own licence |
| Sub-ledger | Forge's record of each customer's balance inside the sponsor's pooled account |
| Tokenised deposit | A licensed bank's own deposit liability represented on a blockchain |
| Wedge | The first, narrow product and customer group used to enter a market |


## The bank model: Erebor and Investec

*Source: `docs/BANK_MODEL.md`*

## Our bank model: Erebor for the technology, Investec for the service

> **DRAFT, owner's strategy note.** Forge Mutual borrows ideas from two banks and copies neither. This page says
> which ideas, from where, how far we have taken them, and where we deliberately differ. Forge is **not a licensed
> bank**, has no customers, and is not connected to either institution. Facts about Erebor and Investec come from
> their public pages and press coverage (linked at the end); they were not checked with either bank, and both change.
> Anything about what Forge has built is checked against this repository.

### In one picture

```mermaid
flowchart LR
  subgraph EREBOR["Erebor (US): the technology angle"]
    E1["A bank for technology companies:<br/>AI, defence, crypto, manufacturing"]
    E2["Deposits, credit, treasury,<br/>payments, stablecoin services"]
    E3["The charter as infrastructure:<br/>other companies build on it"]
  end
  subgraph INVESTEC["Investec (SA): the service angle"]
    I1["One banker, one relationship"]
    I2["Banking, lending, FX and investing<br/>in one place, local and offshore"]
    I3["Quality of service as the product"]
  end
  subgraph FORGE["Forge Mutual (planned)"]
    F1["Technology businesses first (ADR-0022)"]
    F2["Business accounts, ZAR + USD/EUR/GBP,<br/>FX, savings, credit, card acceptance"]
    F3["BankLayer: sponsor bank now,<br/>own licence later"]
    F4["Member-owned (mutual route)"]
  end
  E1 --> F1
  E2 --> F2
  E3 --> F3
  I2 --> F2
  I1 -. "not built yet" .-> F2
  I3 -. "target" .-> F4
```

### What we take from Erebor (the technology angle)

Erebor is a US national bank, chartered in 2025, built for companies in AI, defence, crypto and manufacturing. It
offers deposits, credit, treasury management and payments, plans stablecoin services, and lets other companies build on
its charter.

| Idea | What it means for Forge | Where we are |
|---|---|---|
| **A bank that starts with technology companies** | Our first customers are technology businesses and their founders, not the general public (ADR-0022). The front door, the product order and the risk appetite follow from that. | Built into the product choices; sector appetite (ADR-0012) still to be decided by the owner. |
| **Software-first operations** | Everything is an API and a workflow: onboarding, KYC, payments, FX, reconciliation. A ledger on TigerBeetle, Temporal workflows, an audit log that cannot be edited, and thresholds as data. | Built and tested on the local stack (`docs/ARCHITECTURE.md`). |
| **Treasury for companies, not only accounts** | Multi-currency accounts, FX with a disclosed spread, bulk payouts, merchant acquiring, savings and notice deposits, and a prudential view (capital, liquidity, concentration). | Built; each product is off in production until a licence decision approves it (`pkg/licence`). |
| **Lending that reads our own data** | Overdrafts, working capital and revenue-based financing, decided from the cash flow we can see in the customer's accounts. | Built (`services/bank/lending-service`); needs credit policy, NCA registration and a bureau before use. |
| **The charter as infrastructure** | `BankLayer` hides who holds the licence. Today a sponsor bank holds it and Forge sits on its rails; later Forge holds its own licence, and the rest of the code does not change. | Built as an interface, a simulator and a contract suite. No sponsor chosen. |
| **Crypto-friendly, but regulated** | We bank licensed crypto businesses. | Policy only (see "Where we differ"). |
| **A front door that looks like a modern product** | The apply flow and public pages take their look from erebor.bank's restraint (light, plain, one accent colour); the sygnia.co.za site was used for the later restyle. Look and feel only: no text, images or marks were copied. | Built (`apps/web`). |

### What we take from Investec (the service angle)

Investec is a South African banking and wealth group. Its private bank gives each client a dedicated banker, a 24/7
support centre, local and offshore banking, investment advice, estate and tax planning, and travel and lifestyle
benefits; one page cites a qualifying income of about R800 000 a year (indicative, and changes).

| Idea | What it means for Forge | Where we are |
|---|---|---|
| **Everything in one place** | One login for rand accounts, foreign-currency accounts, FX, savings, credit and card acceptance, with a single view of what the customer owns across them. | Built: accounts, FX, deposits, lending, acquiring, consolidated view and a single customer view. |
| **Local and offshore together** | USD, EUR and GBP accounts at a partner bank, FX against the rand, inbound payments recorded for SARB reporting, and a per-person annual foreign allowance. | Built; needs an authorised-dealer arrangement and the SARB reporting channel. |
| **Credit as part of the relationship** | Statements, arrears tracking and default notices (NCA s.108 and s.129) in the same app as the account. | Built; legal review of the wording needed. |
| **Service as the differentiator** | A named person who knows the customer, fast answers, complaints handled properly. | **Not built.** There is no relationship-manager role in the roster and no service model yet; see "Gaps". |
| **Tiers by customer** | KYC tiers and limits by tier (thresholds 10 and 11) rather than one product for everyone. | Built as data; the values are placeholders. |

### Where we differ, on purpose

| | Erebor | Investec | **Forge Mutual** |
|---|---|---|---|
| Ownership | Shareholders | Listed group | **Depositor-members** (mutual bank planned; bank kept open): `docs/governance/LEGAL_FORMS.md` |
| Home market | United States | South Africa and UK | South Africa |
| Customers | Technology companies, high-net-worth | Affluent individuals, business, corporates | Technology businesses and their founders first |
| Crypto | Plans to hold crypto on its own balance sheet for stablecoin services and fees (per its charter filing) | Not the core offer | **No crypto on our balance sheet** until a separate licence decision. Banks FSCA-licensed crypto businesses only, with enhanced checks (risk appetite worksheet, rows 15 to 17). |
| Stablecoins | A stated focus | Not the core offer | **Not built.** Tokenised deposits are a designed capability (`BankLayer` capabilities), switched off. |
| Wealth management | Limited | Core | **Not planned.** We do not give investment, estate or tax advice. |
| Deposit protection | FDIC-insured | Deposit insurance (South Africa) | Deposit insurance reporting is designed; applies only once licensed. |
| Status | Chartered and operating | Operating | **Pre-licence**, sponsor-bank route first |

### Gaps against the model

Honest list of what the two banks do that Forge does not yet:

1. **Relationship managers: standard written, no one hired.** Seat R37 and `docs/governance/SERVICE_STANDARD.md`; the client
   record, queue and reporting it needs are not built.
2. **Card issuing: built, not usable.** `services/bank/card-service` issues virtual debit cards on a customer's account
   and approves each purchase against the ledger in real time (holds, limits, clearing, reversal, refund), tested against
   a processor simulator, and wired to the customer app (`/cards`) but switched off. It needs a card programme manager or sponsor,
   a processor, and the sponsor's and regulator's agreement before it can be switched on.
3. **Investments and wealth: a referral partnership, not a product.** `docs/products/WEALTH_PARTNERSHIP.md` sets out the
   introducer model, consent and a recommendation to wait until after the pilot. **The introducer layer is built and switched off**
   (`services/platform/partner-offers`): every product is provided, licensed and monitored by a third party and the code
   says so in every offer; no provider is chosen.
4. **Stablecoin or on-chain settlement: decision memo and invariants written, no service.** The architecture forbids token
   assets in the bank ledger without an own licence, and puts digital assets in a separate entity. `docs/products/DIGITAL_ASSETS.md`
   sets out three options and recommends a licensed-partner route first; `pkg/digitalassets` holds the tested reserve,
   wallet and travel-rule checks any future build must pass.
5. **No branches, ATMs or cash, by design.** `docs/regulatory/policies/cash-policy.md` says so and says what must be true
   before that changes (register row FIC-07).
6. **No licence, no sponsor, no customers.** Cannot be built. Everything above is software and documents until that changes;
   see `docs/governance/SPONSOR_BANK_PITCH.md` and `docs/governance/LAWYER_BRIEF.md` for the route.

### Why this model

- **Erebor shows there is demand** for a bank that understands technology companies, and that software can be the
  bank's operating model. Its regulatory route (a US national charter) does not transfer to South Africa; ours is a
  sponsor bank first, then a mutual or bank registration.
- **Investec shows what a premium relationship looks like** in the South African market and what clients will pay for.
  Our cost base is software, not branches, so a smaller team may be able to serve fewer, better-understood customers.
- **The mutual form is our own idea**: members own the bank, so surplus goes back to them. It suits founders' networks,
  co-operatives and microfinance institutions, and it is the part neither bank has.

### Sources

- [OCC: conditional approval of the Erebor Bank charter](https://www.occ.gov/news-issuances/news-releases/2025/nr-occ-2025-101.html)
- [FDIC: Erebor Bank deposit insurance approval](https://fdic.gov/news/press-releases/2025/fdic-approves-deposit-insurance-application-erebor-bank-na-columbus-ohio)
- [Banking Dive: Erebor receives national bank charter](https://bankingdive.com/news/erebor-bank-receives-national-bank-charter/811724/)
- [Investec: why become a Private Banking client](https://www.investec.com/en_za/banking/private-bank-account/discover-the-benefits-of-private-banking.html)
- [Investec: private banking for individuals factsheet](https://www.investec.com/content/dam/south-africa/myself/private-banking/factsheets/private_banking_for_individuals_factsheet.pdf)

Erebor's product scope in this page comes from its charter filing as reported by the press, not from the OCC document
itself; check the primary source before relying on a detail.


## Draft business plan and public-interest motivation

*Source: `docs/licence-application/BUSINESS_PLAN_DRAFT.md`*

## Forge Mutual Bank: draft business plan and public-interest motivation

> **DRAFT. Not approved. Not for submission.** Prepared 2026-11-24 by Claude from `docs/CONTEXT.md`, `docs/BANK_MODEL.md`, `docs/governance/`, the regulatory register
> and the repository. It follows `docs/regulatory/policies/business-plan-outline.md` and register row LIC-15 (business plan, public-interest motivation, rationale,
> group structure chart, dividend policy, constitution, auditor names, fees). **Every number here is either taken from the concept document (itself unverified) or
> marked as a gap. There are no founder-supplied financials in this repository and none are invented here.** The CFO (a vacant seat) and an accountant must replace
> sections 8 to 10 with real work before anything is submitted. Nothing in this document is legal or financial advice.

**Status in plain words.** Forge Mutual Bank has no licence, no customers and holds no money. A sponsor-bank launch is planned first; the mutual bank is the
destination, and the ordinary bank route is kept open (ADR-0020, open). The founder is Tebogo Mvelase (R01). Thirty-six of the thirty-seven governance seats are
vacant.

| Marker | Meaning |
|---|---|
| **GAP** | Needed for the application and not available in the repository. A person must supply it. |
| **VERIFY** | A fact from the concept documents to be confirmed against a source before it is relied on. |
| **ASSUMPTION** | A figure that is a modelling assumption, not data. |

### 1. Summary

Forge is a South African bank and money platform for technology companies and the people who build them. A company holds rand and US dollars in one workspace,
receives foreign revenue, pays staff, suppliers and contractors at home and abroad, and runs approvals as a team. It starts on a sponsor bank, which holds the
deposits, then applies for its own licence as a **mutual bank owned by its depositor-members**. Its edge is a specialist service on open, self-hostable software,
not price or reach (CONTEXT s3, s12).

**What exists today:** a working software platform (customer front door, ledger, payments, multi-currency accounts, savings, lending, merchant acquiring, card issuing
and third-party offers, the last two switched off), a compliance layer (sanctions screening, monitoring, regulatory reporting shells), a governance pack of 55 draft
policies and 37 defined roles, and both legal forms built into the code. **What does not exist:** capital, a board, a sponsor bank contract, members, a licence, and
customers.

### 2. Legal and ownership structure (group chart)

```mermaid
flowchart TB
  M["Members<br/>(depositors who hold member shares; at least 7 founders)"] --> MB["Forge Mutual Bank<br/>(mutual bank, to be registered; not yet)"]
  F["Founder: Tebogo Mvelase"] --> MY["MYFORGE (Pty) Ltd<br/>platform company: builds and runs the product<br/>holds no customer money"]
  MY -- "services agreement, arm's length,<br/>approved by independent directors" --> MB
  SB["Sponsor bank (not chosen)<br/>holds deposits until the licence"] -. "today" .-> MY
  MB -. "later, separate entity" .-> DA["Digital-asset entity (deferred)<br/>FSCA licence needed"]
  MB -- "funder, not owner" --> MFI["Independent MFIs (year 2 pilot)<br/>each separately licensed"]
```

- **Entities and what each may hold** are in CONTEXT s4: the platform company holds no money; the bank layer holds deposits; a separate digital-asset entity (deferred) would
  hold stablecoins; MFIs are independent. Value moves between them only through a reconciled settlement step.
- **Ownership of the mutual:** no outside shareholders; members hold member shares, one member one vote (`CONSTITUTION_DRAFT.md`).
- **Related party risk (to confront, not hide):** the founder controls MYFORGE, which would be the bank's most important supplier. The services agreement, fees, the brand
  licence and the exit terms are a first-order governance matter for the board's independent directors and for the Prudential Authority (outsourcing, register OUT-01).
- **GAP:** the legal advice that the mutual form is available and appropriate for a technology-business membership (ADR-0020; `docs/governance/LAWYER_BRIEF.md`).
- **GAP:** shareholder and controller details for MYFORGE, and fit-and-proper information for every director and executive (`docs/governance/FIT_AND_PROPER_PACK.md`).

### 3. Route and licence type

| Stage | Bank layer | What changes for customers | Needs |
|---|---|---|---|
| A (now) | Sponsor bank holds deposits; Forge runs a sub-ledger and reconciles daily | ZAR and USD accounts, payments, FX, workspace | A signed sponsor bank; an authorised-dealer partner for FX; FICA registration |
| B (licensed) | Forge holds its own licence | Direct payment rails (for example PayShap); tokenised deposits inside the bank | The licence in this application |
| C (cleared) | Unchanged | Stablecoin corridors through a separate entity | A legal clearance that does not exist today |

The application is for **registration as a mutual bank** (planned) or a **bank** (kept open), made while stage A runs. The licence is not a launch prerequisite; a
sponsor bank is. The business plan therefore has to show the regulator two things: the sponsor-stage operation that already runs, and why the bank-stage operation is
better for members than staying on a sponsor.

### 4. Market and customers

**First customer group (ADR-0022, accepted): technology companies.** Startups and scale-ups, profitable tech SMEs, AI and deep-tech firms, and founders personally (kept
separate from their companies). Adjacent customers: cross-border SMEs such as exporters, importers and freelancers paid from abroad (CONTEXT s6).

**Their core jobs:** receive USD from a foreign customer and see it arrive quickly; pay a USD cost without converting first; convert USD to rand for payroll at a
disclosed price; pay a foreign supplier with one all-in quoted cost; pay local staff; approve payments as a team with an audit trail.

**Size of the market (VERIFY, from earlier notes):** about 1,246 startups (StartupBlink), 53,082 technology companies (Tracxn), 91,816 registered software companies
(CIPC, mostly tiny or inactive). The serviceable market is **an estimate of a few thousand startups and scale-ups plus several thousand active tech SMEs, agencies and
exporters**. Startup funding is thin: about R1.49 billion (US$83 million) in the first half of 2026, against R5.89 billion (US$327 million) in Egypt, R4.57 billion (US$254 million) in Nigeria and R2.27 billion (US$126 million) in Kenya (VERIFY).

**GAP:** none of this is yet tested with real customers. The plan's Phase 0 calls for about twenty customer interviews and a design-partner cohort; they have not been run.
The wedge test is stated in CONTEXT s12: if Forge cannot beat roughly **1% all-in cost with same-day settlement** on real customer statements, cross-border payments are
not the wedge.

**Competition (VERIFY):** Capitec (26.6M active clients, 686,000 business clients), Lesaka with Bank Zero, Revolut (filed for a South African banking licence in
November 2025), Wise, the large banks, payment gateways and crypto exchanges. Forge cannot win on price, reach, balance sheet or speed to a licence; it can win as a
specialist (CONTEXT s11 to s12).

### 5. Products and services

v1 (sponsor stage): ZAR business accounts; a USD account through one partner bank; cross-border payments and FX through an authorised-dealer partner; domestic payments;
identity checks; a web workspace with roles and approvals; a back-office console; built-in sanctions screening, transaction monitoring and FIC reporting; deposit
concentration alerts.

Built but **not offered, because each needs a licence or approval that does not exist**: savings and notice deposits, working-capital loans, overdrafts and revenue-based
financing, merchant acquiring, virtual debit cards, and third-party investment and crypto-asset introductions. The software refuses to run any of them in production without
a named approving decision (`pkg/licence`). **Every third-party product is provided, licensed and monitored by the third party, never by Forge.**

Explicit non-goals (CONTEXT s7.3): DeFi, self-custody, Forge-issued stablecoins, a Forge card BIN, CBDC work and mass-market marketing.

### 6. Public-interest motivation (LIC-15)

*A draft argument for the lawyer and the founder to test. Each claim needs evidence; none is yet evidenced.*

1. **Cross-border friction for the people who earn foreign currency for South Africa.** Receiving USD revenue and paying foreign costs is slow, opaque and manual at
   present (CONTEXT s2). A specialist service with one all-in price and disclosed spreads is a consumer-protection benefit. *Evidence needed: fee and settlement data from real
   customer statements.*
2. **Member ownership and fair pricing.** A bank owned by its depositors returns surplus as better pricing or rebates instead of outside dividends, and has a credible answer
   to fee mistrust. *Evidence needed: the constitution's surplus rule and a pricing policy approved by members.*
3. **Access for groups that banks serve poorly.** Savings groups, stokvels, co-operatives and micro-lenders could become members and own their funder; the planned MFI network
   would offer cheaper registered credit in place of unregistered lenders (CONTEXT s2, s10). *Evidence needed: the pilot in section 12, with measured default, cost and
   borrower-rate results. The concept document itself records that randomised studies of microcredit found no significant average effect on household income, so this claim must
   be made carefully.*
4. **Competition and local control.** A new entrant on open, self-hostable software that can run in South Africa under local control reduces dependence on foreign vendors
   (CONTEXT s3.3 principle 6). *Evidence needed: the hosting decision (ADR-0002) and the outsourcing register.*
5. **Safety designed in.** Reconciliation against the sponsor, a ledger with one writer, an append-only audit log, a default-deny rule engine, and consent for data use are
   already built and tested. *Evidence: the test gate and the regulatory register.*

**GAP:** a regulator will want proof that the new bank is needed and not just wanted: market data, member commitments (letters of intent from prospective founders and
members), and a statement of how the bank differs from existing mutual and co-operative banks. None is in the repository.

### 7. Governance, board and key persons

The target governance is in `docs/governance/STRUCTURE.md`, `ROLES.md` and `POLICY_FRAMEWORK.md`: a board with committees (audit, risk and capital, nominations and
remuneration), three lines of defence, 58 draft policies and 37 defined seats.

| Item | State |
|---|---|
| Board | **None.** The constitution draft sets the shape: at least two employee directors (MR-03), a majority-independent non-executive group, directors elected by members. |
| Seat holders | 1 of 37 named (founder, Tebogo Mvelase, as R01). The accountable roles for compliance, money laundering, risk, finance, audit, legal, company secretary, treasury, IT risk and security are **vacant**. |
| Fit and proper | The filing pack (DI 020 for a mutual bank) and tracker exist; nobody has filed. |
| Independence | The founder's control of the platform supplier makes independent directors and an independent chair essential, not optional. |

**GAP:** names, curricula vitae, qualifications and references for each director and executive. A business plan with an empty board will not be accepted.

### 8. Financial projections

Money is shown in rand first, with the concept document's original US-dollar figure in brackets, converted at an assumed R18.00 per US dollar (ASSUMPTION; update with the rate at the time of use).

> **This section is a placeholder by design.** The repository holds an *illustrative* pro forma and the concept document's *illustrative* economics. They are shown below
> so that nobody mistakes them for forecasts, and so the gaps are visible.

**What the concept document assumes (CONTEXT s8.3; ASSUMPTION, in US dollars, not converted):**

| Year 5 | Conservative | Base | Optimistic |
|---|---|---|---|
| Customers | 3,200 | 9,000 | 18,000 |
| Revenue | about R32.4 million (US$1.8 million) | about R160 million (US$8.9 million) | about R499 million (US$27.7 million) |
| Operating result | about -R82.8 million (-US$4.6 million) | about -R59.4 million (-US$3.3 million) | about +R86.4 million (+US$4.8 million) |
| Deposits | about R864 million (US$48 million) | about R4.86 billion (US$270 million) | about R16.2 billion (US$900 million) |
| Peak funding need | about R319 million (US$17.7 million) | about R356 million (US$19.8 million) | about R302 million (US$16.8 million) |
| Customers to break even | about 10,900 | about 10,100 | about 10,400 |

The model assumes revenue per customer of R9,000 (US$500), R16,200 (US$900) or R25,200 (US$1,400) a year growing 10% a year, variable costs of 25% of revenue and operating costs rising from R36 million (US$2 million) to R288 million (US$16 million).
It **excludes** the sponsor bank's revenue share, licence capital, credit losses and compliance headcount, so the real position is harder.

**What the repository's pro forma shows (`docs/regulatory/proforma/`, illustrative, rand):** deposits of R150M, R600M and R1.5bn at the end of years 1 to 3, with small
surpluses. It exists to show the shape of the DI 100 and DI 200 forms. **It does not agree with the concept document's economics** (for example, year-5 base deposits of about
R4.86 billion (US$270 million) against R1.5bn at year 3). One consistent model must replace both.

**Three weak points a regulator will test:**

1. **Break-even needs about 10,000 business customers**, which is close to the whole serviceable market in section 4. Either the plan needs expansion (other African markets,
   adjacent customers, the MFI network) or the economics need to be rebuilt. The concept document says the same.
2. **Volatile deposits.** Technology deposits swing with funding cycles and are correlated (the Silicon Valley Bank lesson, CONTEXT s16). The plan needs concentration limits,
   stress tests and a liquidity buffer sized to that, and a mutual's members can redeem shares (constitution clause 4.3).
3. **Capital.** The minimum for a mutual bank is understood to be R10M (VERIFY) and for a bank R250M (VERIFY), but the concept document puts the funding need at about
   **R270 million (US$15 million) to R360 million (US$20 million) before profit in the base case**. The minimum is not the need.

4. **Liquid assets and the reserve balance.** Building DI 310 from the official form showed that a balance with a sponsor or partner bank is
   **not** a liquid asset under the Mutual Banks Act (only Reserve Bank notes and coin, gold, Reserve Bank clearing balances, Treasury bills,
   government, Reserve Bank and Land Bank securities count). As a registered mutual bank Forge would need, on the form's figures (VERIFY the
   current rates), about **5% of adjusted liabilities in liquid assets plus a 2.5% average reserve balance with the Reserve Bank**, which earn
   little or nothing. The model must hold them and price their cost; today's ledger has no account for any of them, and the returns code reports
   the shortfall.

5. **Concentration on the sponsor bank.** DI 510 counts a balance with a bank as an exposure to that bank. With all customer money placed at
   one sponsor, that single exposure is many times ten per cent of capital. The model must test it against the large-exposure limits that apply
   to a registered mutual bank (VERIFY the limits with the lawyer) and plan how placements are spread.

**GAP, to be built with the CFO and an accountant:** a consistent three-to-five-year model (balance sheet, income statement, capital and liquidity, with customer, deposit,
lending and cost drivers), a sponsor-bank revenue share, stress scenarios, and the pro forma returns (DI 100, 110, 200, 300, 400, 430, 700) regenerated from that model
(`make proforma`). DI 100 and DI 200 now use the forms' own line numbers (`docs/regulatory/returns/`); the transcription needs a second person's check.

### 9. Capital and funding plan

| | Amount | Source | State |
|---|---|---|---|
| Minimum registered capital (mutual bank) | **R10M** (VERIFY) | Founder capital and member shares | **GAP: nothing raised** |
| Minimum registered capital (bank option) | **R250M** (VERIFY) | Investors | **GAP** |
| Funding need before profit (base case) | about **R270 million (US$15 million) to R360 million (US$20 million)** (ASSUMPTION) | To be decided: members, development finance, founder, strategic partners | **GAP** |
| Loss-absorbing buffer above the minimum | to be set by the CRO and CFO | | **GAP** |

A mutual cannot easily raise venture equity or be sold (CONTEXT s5.2), so the plan must say where the money comes from. Options to evaluate with the lawyer and an
accountant (none is chosen): founder capital; member shares from prospective members (each priced and capped by the constitution); development-finance institutions; a
subordinated or other capital instrument that the Prudential Authority accepts for a mutual (`VERIFY`); revenue from the sponsor-stage business.
**Proof of funds** in writing is a licensing requirement (LIC-02) and nothing here replaces it.

### 10. Risk management and compliance framework

- **Risks to manage (MR-01, JS1):** solvency, liquidity, credit, currency, market, interest-rate, counterparty, technological and operational. Drafts: `risk-management-framework.md`,
  `liquidity-and-contingency-funding-plan.md`, `concentration-policy.md`, `it-risk-management-framework.md`, `outsourcing-policy.md`.
- **Compliance:** FICA (KYC, sanctions screening, goAML reporting, Travel Rule where crypto transfers arise), POPIA, exchange control (through an authorised dealer), National
  Credit Act (for any lending), and deposit insurance (CODI) disclosure. Counsel has not reviewed any of it.
- **Controls built into the software:** reconciliation against the sponsor, one ledger writer, an append-only audit log, default-deny rules, maker-checker approvals and
  consent for personal data. Evidence is the test gate and the register.
- **Status:** all 58 policies are drafts; none is approved because no body exists to approve them.

### 11. Systems, security and outsourcing

Summarised from `docs/ARCHITECTURE.md`: TigerBeetle ledger, Apache Fineract core, Temporal workflows, Open Policy Agent rules, Kafka events, PostgreSQL, mutual TLS between services,
secrets from files, security scanning and a published recovery procedure. Hosting plan: Fly.io for a synthetic-data preview; AWS (Cape Town proposed) for production
(`docs/HOSTING.md`); **production hosting does not exist yet and nothing has been applied to a cloud account.**

**Material outsourcing (OUT-01, E1 to E3 in the pack):** the platform company (core and reporting IT), the sponsor bank, cloud hosting, identity verification (Didit), sign-in
(WorkOS), and others in `docs/regulatory/outsourcing-register.csv`. The Prudential Authority needs prior notice of material outsourcing (20 working days is the working
assumption, VERIFY).
**GAP:** an independent penetration test, a production disaster-recovery site and test (JS1-12), and a named IT risk function (JS1-03).

### 12. Staged plan and milestones

| Stage | Timing (estimate; VERIFY) | Exit test |
|---|---|---|
| Today | now | Software built and tested on a local stack; every external party simulated |
| Legal and people | next 3 months | Lawyer's advice; ADR-0020 closed; seat holders named; seven founders identified |
| Sponsor and partners | months 3 to 9 | Signed sponsor bank, USD partner, authorised-dealer partner; sanctions list licensed |
| Production readiness | months 6 to 12 | Hosting applied; penetration test passed; Prudential Authority outsourcing notice made |
| Closed launch | months 9 to 15 | About twenty design-partner customers; the 1% wedge test run on real statements |
| Application | when capital, board, founders and policies are in place | Pre-application meeting; filing |
| Registration | **typically 18 to 24 months or more from a complete application** (general knowledge; confirm with the lawyer) | Licence and Financial Sector Conduct Authority concurrence |
| After registration | | Migrate from the sponsor's books (the migration runbook is **not yet written**; `LEGAL_FORMS.md` refers to one that does not exist); consider the MFI pilot and cards |

**Proof points that would make Forge significant (CONTEXT s17):** cross-border payments under about 1% all-in cost with same-day settlement on real statements from fifteen to
twenty companies; one franchised MFI that is profitable; evidence that pooled data improves credit scores across independent lenders.

### 13. Dividend and surplus policy

Surplus is applied first to required reserves, then returned to members as rebates, better rates or (only if the lawyer advises it is compatible with the form) a limited
dividend, by decision of the members' meeting, and only if the bank would still meet its capital and liquidity requirements. See `CONSTITUTION_DRAFT.md` clause 8 and decision
2 there.

### 14. Other items the application needs (LIC-15)

| Item | State |
|---|---|
| Constitution or articles | Draft: `CONSTITUTION_DRAFT.md` (for the lawyer) |
| Group structure chart | Section 2 above (to be redrawn with real ownership percentages) |
| Auditor names (form DI 005 for a mutual bank) | **GAP**: no auditor appointed; one registered for banks is needed |
| Fees and tariffs | **GAP**: a fee schedule approved by the board and reviewed by the lawyer (CONTEXT s8.2 sets the principle: one all-in price, disclosed spreads) |
| Public-interest motivation | Section 6, to be evidenced |
| Pro forma returns | Illustrative only (`docs/regulatory/proforma/`) until section 8 is built |
| Pre-application meeting and FSCA concurrence (LIC-16) | **GAP**: not requested |

### What must happen before this plan can be submitted

1. The lawyer's advice, and ADR-0020 closed.
2. A CFO and an accountant build the model in section 8 and the capital plan in section 9; the founder confirms every ASSUMPTION or replaces it with data.
3. A board, executives and independent directors are named, with fit-and-proper statements.
4. At least seven founding members and written commitments for capital.
5. Customer evidence: interviews, letters of intent and a design-partner cohort.
6. A signed sponsor bank and the other partners in section 12.
7. The policies in section 10 approved by the board once it exists.
8. The document reviewed by the lawyer for statements about regulation, and by the founder for statements about intent.

*Nothing in this plan has been sent to any regulator, investor or customer.*


## Wealth and investment partnership

*Source: `docs/products/WEALTH_PARTNERSHIP.md`*

## Investments and wealth: a partnership, not a product

> **DRAFT.** The introducer layer is now built (`services/platform/partner-offers`, `SPEC.md` there) and is **switched off**: every
> product is provided, licensed and monitored by a third party, and the code says so in every offer. No partner is chosen.
> Investec's model combines banking with investment, estate and tax advice. Forge will not provide those
> itself: they need separate licences, people and a different risk culture. The realistic route is an **introducer
> (referral) partnership** with a licensed provider. Nothing here is built and no partner is approached. Not legal advice;
> the lawyer (`docs/governance/LAWYER_BRIEF.md`) settles every regulatory point below.

### Position

| | Forge | Partner |
|---|---|---|
| Deposits, payments, FX, credit, card acceptance | Yes | No |
| Investment advice and management, unit trusts, tax and estate planning | **No** | Yes, under its own FSP licence |
| Crypto-asset advice or custody | **No** | A licensed CASP, if at all |
| Holding a client's investments | **No** | Partner or its custodian |

### Why not build it

- Advising on or managing investments is a regulated financial service (Financial Advisory and Intermediary Services
  Act: an authorised FSP, key individuals and representatives with regulatory exams). Confirm the current rules and
  whether a bank can introduce clients under its own licence or a representative arrangement.
- It adds conduct risk (suitability, disclosure, complaints) that a small bank should not carry in its first years.
- The client's need is a good answer, not necessarily ours: a partner can give it.

### How a referral would work

```mermaid
flowchart LR
  C["Client asks, or the relationship<br/>manager sees surplus cash"] --> RM["Relationship manager:<br/>explains the partner, no advice"]
  RM --> CONSENT["Client consents to share<br/>named data with the partner<br/>(consent-service)"]
  CONSENT --> P["Partner's own onboarding, KYC,<br/>suitability and contract"]
  P --> OUT["Client invests with the partner;<br/>funds leave Forge by a normal payment"]
  OUT --> FEE["Partner pays Forge a disclosed<br/>referral fee (if lawful)"]
```

Rules (PROPOSED):

1. The client always deals with the partner under the partner's terms. Forge's name is not on the product.
2. **Consent first.** Data goes to the partner only for the purpose, fields and time the client agreed, recorded in the
   consent service and revocable (POPIA; `docs/regulatory/data-classes.csv`).
3. Referral fees are disclosed to the client and approved by the Compliance Officer; no fee is tied to a product that is
   not suitable. Confirm the legal basis with the lawyer.
4. Forge makes no statement about returns and gives no comparison between products.
5. A transfer to the partner is an ordinary payment, screened and limited like any other.
6. The partner is a material outsourcing candidate if Forge relies on it for a client service
   (`docs/regulatory/policies/provider-due-diligence-procedure.md`, `provider-contract-schedule.md`).
7. Forge keeps a register of referrals and a complaints route that reaches the partner.

### Choosing a partner

What to look for: authorised FSP with a clean regulatory record; custody with a recognised custodian; transparent
fees; technology that can be reached by API; comfortable with technology founders; a stance on crypto assets that
matches our appetite (`docs/governance/RISK_APPETITE_DECISIONS.md`, rows 15 to 17). Start with two or three
conversations; the sponsor bank may have a preferred partner.

### What would need building, later

| Item | Status |
|---|---|
| A referral register, the customer's page and a compliance back-office screen | The register, the rules, the workspace routes, the customer page `/invest` with its consent step, and the compliance screen `/partners` are built |
| A consent purpose "share with an investment partner" in the consent service | Used by `partner-offers` as purpose `share_with_partner`, scope `referral:<provider_id>`; the customer screen (`/invest`) asks, and the sender sends only the listed fields (today the customer's name) |
| An API link to a chosen partner | Not built; depends on the partner |
| A referral-fee ledger entry (income) | Not built; use FEE income (2002) |

### Decision needed

Whether to pursue this at all before the sponsor bank and first pilot. Recommendation: **no**. Revisit after the pilot,
when clients with surplus cash have actually asked for it.


## Digital assets

*Source: `docs/products/DIGITAL_ASSETS.md`*

## Digital assets: stablecoins, deposit tokens and on-chain settlement

> **DRAFT decision memo.** Gap 4 of `docs/BANK_MODEL.md`: Erebor is described as seeking to facilitate stablecoin
> transactions, and our tech-and-crypto leaning invites the same. This memo says what we could do, what stops us, what
> is built, and what to decide. **Nothing here moves customer money.** Not legal advice. Every regulatory statement is
> from general knowledge and must be confirmed by the lawyer and the Prudential Authority and FSCA before anything is built.

### Why it is not built

Two rules in `docs/ARCHITECTURE.md` stop it, on purpose:

- **Rule 8:** the bank's ledger rejects stablecoin assets, and rejects tokenised-deposit assets unless an own licence is
  approved (`licence.bank.own`) and the asset itself is approved.
- **Rule 6:** only one gateway writes to each ledger cluster, and digital assets belong to a **separate entity** (`DAST`)
  with its own ledger cluster, signer and on-chain services. The bank perimeter holds ZAR and foreign-currency deposits.

A service that minted tokens against customer deposits inside the bank entity would break both, and would do so before
anyone has asked the regulator whether it is allowed. So the decision comes first.

### The three things "digital assets" could mean

| Option | What it is | Where it lives | Licence and regulator | Effort |
|---|---|---|---|---|
| **A. Bank customers send and receive stablecoins through a licensed partner** | Forge never touches the token. A licensed crypto-asset service provider (CASP) or exchange does the conversion; Forge sends and receives rand and verifies the counterparty. | Bank entity, ordinary payments | The partner's licences; Forge's screening and travel-rule duties | Small: counterparty screening, travel rule, partner integration |
| **B. A separate Forge digital-asset entity** | Custody and on/off-ramp for a third-party stablecoin (an issuer per ADR-0005). | `DAST`, own ledger, signer, on-chain services | FSCA as a CASP, plus the PA's view of the group; intercompany settlement with the bank | Large: 6 to 12 months of engineering, custody controls, a smart-contract-free design but signing infrastructure |
| **C. Tokenised deposits** | The bank issues an on-chain token that is a claim on a deposit, 1:1 with a liability (account 7001). | Bank entity, own licence only | The PA: a deposit-taking and deposit-insurance question (is a token a deposit?); FSCA if it is also a crypto asset | Largest, and the least certain legally |

A Forge-issued stablecoin (not a deposit token) is a fourth possibility the architecture lists as deferred; it carries
the heaviest regulation and is not recommended.

### Recommendation

**Do A first, when a sponsor bank and a pilot exist; do not start B or C until the regulator has been asked.**
Reasons: A is what customers most plausibly want (moving rand to and from a stablecoin for a technology business), needs
no new licence for Forge, and fits the risk appetite (`RISK_APPETITE_DECISIONS.md` rows 15 to 17: bank only
FSCA-licensed crypto businesses, no crypto on our own balance sheet). B and C change what kind of institution Forge is.

### Questions for the lawyer and the regulator

1. Is a tokenised deposit a deposit under the Banks Act (or Mutual Banks Act), and is it covered by deposit insurance?
2. Is it also a crypto asset under the FSCA's declaration, needing a CASP licence?
3. May a mutual bank hold crypto-asset reserves, or only the reserves that back a token? Which assets may be reserves?
4. What is the capital treatment of crypto-asset exposure and of token liabilities?
5. What does the FIC's travel-rule directive require for each transfer, and from what value?
6. Does the sponsor bank permit crypto-related activity at all, and for which customers?
7. Which stablecoin issuers are acceptable to the regulators, and under what disclosures to customers?
8. Exchange control: how do transfers of crypto assets abroad interact with the single discretionary and foreign
   investment allowances (`docs/THRESHOLDS.md`, threshold 28)?

### Controls any option must have

| Control | Status |
|---|---|
| **1:1 reserves with no tolerance:** every token in circulation is covered by reserve assets; a shortfall of one unit halts issuance and redemption and goes to the board | **Written and tested** (`pkg/digitalassets.Attest`); not connected to anything |
| **Wallets:** supported chain, valid address, proof the customer controls it, screened within 24 hours and clear, else no transfer | **Written and tested** (`Wallet.Usable`) |
| **Travel rule:** originator and beneficiary details travel with a transfer; the originator's identifier from 1 000 US dollars (VERIFY) | **Written and tested** (`TravelRule`) |
| Sanctions and illicit-exposure screening of addresses | Interface only: the screening package works on names; address screening needs a provider (Didit offers wallet screening; unverified here, not used) |
| Key custody: hardware security modules, multi-party signing, no single person able to move funds | Not designed in detail |
| Daily proof of reserves published, and an independent attestation | Not built |
| Smart-contract audit, if a contract is deployed at all (option C) | Not started |
| Incident response for a chain event (a stablecoin depeg, an issuer freeze) | Not written |
| Customer disclosures: a token is not a deposit unless the PA says so; not covered by insurance unless it is | Not written |

### What is built

- **Option A's front door:** `services/platform/partner-offers` can present a **licensed crypto-asset service provider's** on/off-ramp as a
  third-party product (category `CRYPTO_ASSETS`, CASP licence required). Forge never touches the token. Every offer states that
  it is provided, licensed and monitored by the provider, and the whole layer is **switched off**. No provider is chosen.
- `pkg/digitalassets`: `Attest` (reserve cover), `Wallet.Usable` (fail-closed wallet checks) and `TravelRule`. It
  holds no state, calls no network and writes to no ledger. It exists so that whoever builds A, B or C starts from tested
  invariants, and so CI can hold them to those invariants.
- The `digital_assets` licence product (`pkg/licence`): when any service uses it, production will refuse it without a
  `FORGE_APPROVAL_DIGITAL_ASSETS` decision. No service uses it yet.
- The architecture's reserved codes (account 7001 TOKENISED_DEPOSIT_LIABILITY; transfer codes 40 and 41) remain
  **unregistered in the ledger gateway**, so the bank ledger still refuses them.

### What is not built, deliberately

A token issuer or minter; a chain watcher or executor; a signer; a wallet registry; a digital-asset ledger cluster;
any chain connection; stablecoin custody; an on/off-ramp. Each depends on the decision below.

### Decision needed from the owner

1. Agree the sequence: **A after the pilot**, B and C only after a written view from the PA and FSCA.
2. Put the eight questions above in the lawyer's brief (`docs/governance/LAWYER_BRIEF.md`).
3. Decide whether Forge will ever hold crypto itself, or only bank businesses that do (the risk appetite worksheet).


---

# Part III. Legal form and governance


## The two legal forms

*Source: `docs/governance/LEGAL_FORMS.md`*

## Two legal forms: mutual bank (planned) and bank (kept open)

> **DRAFT.** Owner's direction on 2026-11-18 (Tebogo Mvelase): plan for a **mutual bank**, and keep the **bank**
> route ready in case capital, investors or the regulator point that way. Not a final decision: ADR-0020 stays open
> until the regulatory lawyer has advised (see `LAWYER_BRIEF.md` (`docs/governance/LAWYER_BRIEF.md`)). Every figure is from the
> regulatory register and must be confirmed against the current law.

### How both are kept open

Nothing in the software branches on the legal form. The differences are data in one place,
`pkg/banklayer/legalform.go` (`pkg/banklayer/legalform.go`), and every service reads them from there:

```mermaid
flowchart LR
  CFG["FORGE_LEGAL_FORM<br/>mutual (default) or bank"] --> PROF["banklayer.Profile<br/>capital minimum, owners, voting rule,<br/>founders, forms, returns"]
  PROF --> TR["treasury-risk<br/>INITIAL_CAPITAL breach:<br/>R10m mutual · R250m bank"]
  PROF --> MEM["membership<br/>members, shares, AGM votes, distributions<br/>(refuses everything if bank)"]
  PROF --> CAP["Capabilities<br/>MemberShares, BookOfRecord<br/>(read by OPA as data)"]
  PROF --> RR["reg-reporting<br/>DI returns or BA returns<br/>(builders not yet written)"]
  PROF --> GOV["Governance pack<br/>DI 020 or BA 020, board rules"]
```

Switching from one to the other is a configuration change plus the legal work; no code changes.

### Side by side

| | Mutual bank (planned) | Bank (option) |
|---|---|---|
| Law | Mutual Banks Act 124 of 1993 | Banks Act 94 of 1990, Companies Act 71 of 2008 |
| Owned by | Depositor-members, who hold member shares | Shareholders of a public company |
| Minimum initial capital | **R10 million** (verify current figure) | **R250 million** (verify) |
| Founders | At least **7**, signing the founding documents | One or more shareholders |
| Voting | One member, one vote (PROPOSED; the constitution decides) | By shares |
| Ownership approval | Members' rules in the constitution | PA permission above 15% (Banks Act s.37; verify thresholds) |
| Employee directors | At least **2** (MR-03) | No fixed minimum; King IV and PA expectations |
| Fit and proper form | DI 020 | BA 020 |
| Application forms | DI 100, 110, 200, 300, 400, 430, 700 | BA 100, 110, 120, 130, 300, 350, 700 |
| Returns | DI 100 to DI 900 (about 18) | BA 100 to BA 950 (about 19) |
| Venture capital | No: cannot easily raise outside equity or be sold | Yes |
| Surplus | Back to members as rebates, better rates or dividends | To shareholders |
| Fit for Forge | Member ownership, MFIs and co-operatives as members, stokvels, lower capital | Faster growth, investors, exit |
| In the code | `MemberShares = true`; membership module active | `MemberShares = false`; membership refuses |

### Path for each

```mermaid
flowchart TB
  NOW["Today: software built, sponsor-bank model,<br/>founder Tebogo Mvelase"] --> LAW["Regulatory lawyer advises on form<br/>(LAWYER_BRIEF.md)"]
  LAW --> M1 & B1
  subgraph MUTUAL["Mutual bank route (planned)"]
    M1["Recruit 7+ founding members<br/>(membership.ReadyToApply)"] --> M2["Draft the constitution:<br/>voting, shares, distributions"]
    M2 --> M3["Raise R10m+ in member shares<br/>and founder capital"]
    M3 --> M4["Board with 2+ employee directors;<br/>DI 020 for each"]
    M4 --> M5["DI application forms;<br/>pre-application meeting with PA"]
  end
  subgraph BANK["Bank route (option)"]
    B1["Incorporate a public company"] --> B2["Shareholder agreement;<br/>s.37 approvals for 15%+ holders"]
    B2 --> B3["Raise R250m+"]
    B3 --> B4["Board; BA 020 for each"]
    B4 --> B5["BA application forms;<br/>pre-application meeting with PA"]
  end
  M5 & B5 --> LIC["PA registration (FSCA concurrence)<br/>then CODI, FIC, SARB, PASA"]
  LIC --> MIG["Move customers from the sponsor to Forge's books<br/>(runbook not yet written)"]
```

### What changes in the governance pack

- **Mutual:** the general meeting of members elects directors; the membership module keeps the register, shares and
  votes; distributions need board approval (maker-checker in code); seat R36, member relations, is needed.
- **Bank:** the shareholder register replaces the member register; the ownership policy applies the s.37 approvals;
  R36 is not needed.
- Everything else (board, committees, three lines of defence, policies, risk appetite) is the same for both.

### What is built and what is not

| Item | Mutual | Bank |
|---|---|---|
| Legal-form profile and selection (`FORGE_LEGAL_FORM`) | Built, tested | Built, tested |
| Minimum capital breach in the prudential report | Built, tested | Built, tested |
| Member register, shares (issue, redeem with capital check), meetings, one-member-one-vote, quorum and majorities, distributions to the cent | Built, tested against Postgres; not yet wired to a running service or the ledger (8001/8002, transfer 80) | Not needed |
| Shareholder register | Not needed | Kept by the company secretary (a document, not code) |
| Return builders (DI or BA) | Not built | Not built |
| Application form builders | Not built | Not built |


## Draft constitution

*Source: `docs/licence-application/CONSTITUTION_DRAFT.md`*

## Forge Mutual Bank: draft constitution (rules)

> **DRAFT FOR THE LAWYER. Not legal advice. Not approved. Not in force. Not for signature.**
> Prepared 2026-11-24 from `docs/governance/LEGAL_FORMS.md`, `docs/CONTEXT.md` and the rules the membership module already enforces
> (`services/bank/membership`). It is a first working text so that the regulatory lawyer has something to correct instead of starting from
> nothing. **The Mutual Banks Act 124 of 1993 and its regulations set mandatory content and process for a mutual bank's constitution; this draft
> does not reproduce them, and every place where the Act decides is marked `[ACT]`.** The lawyer must add those provisions, delete anything
> the Act forbids, and confirm every `VERIFY`. Square brackets `[ ]` are decisions the founder or the founding members must make.
>
> Register rows: LIC-01, LIC-07, MR-03. Pack section: A1. ADR-0020 stays open until the lawyer advises.

### How to read this draft

| Marker | Meaning |
|---|---|
| `[ACT]` | The Mutual Banks Act or its regulations decide this. The lawyer supplies the text. |
| `[DECISION n]` | A choice for the founder or founding members. Listed in the table at the end. |
| `VERIFY` | A fact or figure from the repository that must be confirmed against the current law. |
| **Code:** | What the software already does, so that the constitution and the system agree. If the constitution changes, the code changes (or the other way round); neither is allowed to drift. |

### 1. Name and nature

1.1 The name of the mutual bank is `[Forge Mutual Bank]` `[DECISION 1]`. **The words "bank" and "mutual bank" are not used in any public name, marketing or
website until the Prudential Authority has authorised them** (register LIC-17; `docs/regulatory/policies/naming-and-marketing-policy.md`). Until then the
organisation is referred to by its registered name only.

1.2 It is a mutual bank, a juristic person registered under the Mutual Banks Act. `[ACT]` (registration, juristic personality, the registrar's role).

1.3 It is owned by its members, who are its depositors who hold member shares (clause 4). It has no outside shareholders and does not pay a return to
anyone for providing capital beyond what clause 12 allows. `[ACT]` `[DECISION 2]` (whether any member share carries a dividend or only a rebate).

1.4 Its purpose is `[to provide banking services to its members, especially technology businesses, the people who build them, and savings and credit groups, on
fair, transparent terms, and to return its surplus to members]`. `[DECISION 3]` (the lawyer to align with the Act's description of a mutual bank's objects).

### 2. Definitions `[ACT]`

Terms to be defined with the Act's own wording: Act, Registrar, Prudential Authority, member, member share, founder member, general meeting, special resolution,
board, employee director, independent director, surplus, reserves, record date. The definitions of "member" and "member share" are the ones the software
uses; see clauses 3 and 4.

### 3. Members

3.1 **Who may be a member.** A person who holds an account with the bank and holds at least the minimum member share holding. Natural persons and juristic persons
may be members, including `[companies, close corporations, co-operatives, savings and credit groups, stokvels, and micro-finance institutions]`. `[DECISION 4]`
(whether stokvels and informal groups can be members directly or only through a registered entity; whether a business and its founder are separate members).

3.2 **Admission.** Admission is by application, subject to the bank's customer due diligence (identity, sanctions and anti-money-laundering checks under the
Financial Intelligence Centre Act). A person who fails those checks cannot be admitted. The board may refuse an application only on stated, non-discriminatory
grounds recorded in the minutes. `[ACT]`

3.3 **Founder members.** There are at least **seven** founder members, who sign the founding documents. The application to the Prudential Authority is not made
until there are at least seven. `[ACT]` `VERIFY` (the figure of seven and what "founder" means).
**Code:** `membership.ReadyToApply` refuses until the register holds the founding number of active members (`Profile.MinFounders = 7` in `pkg/banklayer/legalform.go`).

3.4 **Ending membership.** A member ceases to be a member on resignation, on closing their last account and redeeming their shares, on death or dissolution, or
by the board's removal for `[a serious breach of the rules or fraud]`, with a right to be heard and to appeal to a general meeting. `[ACT]` `[DECISION 5]`.

3.5 **Register of members.** The bank keeps a register of members showing name, identity or registration number, date of admission, shares held, and date of
ceasing. The register is personal information and is protected under the Protection of Personal Information Act. A member may inspect their own entry. `[ACT]`
(who else may inspect the register, and what must be public).
**Code:** the member register, admission date and status are held by the membership module.

### 4. Member shares and capital

4.1 **One class of member share** `[DECISION 6]` (a second class for founders, for non-voting capital, or for institutional members is a decision with legal
consequences and is not assumed here). Each member share has a nominal value of `[R100]`. A member holds at least `[1]` and at most `[a limit so that no member holds
more than 5% of all member shares]` `[DECISION 7]`.

4.2 Member shares are issued only against full payment. They are not transferable except by redemption to the bank. They carry **one vote per member**, not per share.

4.3 **Redemption.** A member may ask to redeem shares when they leave or reduce their holding. The bank must refuse a redemption if paying it would take the bank
below the capital the Act and the Prudential Authority require, and may defer redemptions for `[DECISION 8: the longest deferral]`. `[ACT]`
**Code:** `MoveShares` redeems only after a capital check and refuses with `ErrCapitalAtRisk`; redemption of more than held is refused (`ErrTooFewShares`).

4.4 **Minimum capital.** The bank maintains at least the minimum capital the Act requires, currently understood to be **R10 million** at registration. `VERIFY` `[ACT]`.
Proof of that capital is part of the application (register LIC-02). The founders' capital plan is in `BUSINESS_PLAN_DRAFT.md` section 9.
**Code:** the prudential report raises an INITIAL_CAPITAL breach below the minimum for the legal form.

4.5 No member share is a deposit and none is covered by deposit insurance. The member is told so in writing before buying shares. `[ACT]`

### 5. General meetings

5.1 The bank holds an annual general meeting within `[six months]` after its financial year end, and other meetings when the board calls them or when
`[a stated number or percentage of members]` ask in writing. `[ACT]` `[DECISION 9]`.

5.2 **Notice.** At least `[21]` days' written notice, sent to each member's registered address and electronic address, stating the time, place (or
electronic means) and the resolutions. `[ACT]` `VERIFY`.

5.3 **Record date.** Only a person who was a member on the record date set in the notice may vote. **Code:** the module treats the day the notice was sent as the record date, and a member admitted after it is refused
(`ErrNotEligible`). If the constitution sets a different record date, the module changes with it.

5.4 **Quorum.** A meeting is quorate when at least `[the greater of 25 members or 5% of members]` are present or voting. `[DECISION 10]`.
**Code:** `Decide` returns INQUORATE when voters are fewer than the quorum fixed when the meeting was called. Abstentions count towards the quorum and not
towards the majority.

5.5 **Voting.** One member, one vote. Voting may be in person, by proxy `[DECISION 11: whether proxies are allowed and how many a person may hold]`, or
electronically by a method that proves the voter is a member and records one vote only. **Code:** a second vote by the same member on the same resolution is
refused (`ErrAlreadyVoted`); a closed resolution takes no more votes (`ErrClosed`).

5.6 **Majorities.** An ordinary resolution passes on **more than half** of the votes cast for or against. A special resolution passes on **at least three quarters**
of the votes cast for or against. `[ACT]` `VERIFY` (whether the Act or regulations set other thresholds).
**Code:** `Decide` applies exactly these two rules.

5.7 **What needs a special resolution:** amending this constitution, changing the objects, merging, converting to another form, winding up, and
`[DECISION 12: any other matter]`. Amendments take effect only after any approval the Act requires. `[ACT]`

5.8 Results and minutes are kept, and members may inspect them. Resolutions of the first general meeting adopt this constitution and elect the first board
`[or the founders appoint the first board until the first general meeting]` `[DECISION 13]`.

### 6. The board `[ACT]`

6.1 The board has at least `[7]` directors `[DECISION 14]`, elected by the members at a general meeting, serving `[three-year]` terms, with `[one third]` retiring
each year.

6.2 **Employee directors.** **At least two** directors are employees of the bank. `[ACT]` (MR-03, regulation 38; `VERIFY`).

6.3 **Independence.** At least `[a majority of the non-executive directors]` are independent of management and of any significant supplier `[DECISION 15]`. A
director who is an officer or owner of **MYFORGE (Pty) Ltd** (the platform company that builds and runs the product) is not independent on any matter in which
MYFORGE has an interest (clause 10).

6.4 **Fit and proper.** Every director and executive officer is fit and proper and has filed the Prudential Authority's statement (form DI 020 for a mutual
bank) before taking office. Nobody serves whose statement has not been accepted. `[ACT]`
**Code:** the filing pack and tracker are `docs/governance/FIT_AND_PROPER_PACK.md`.

6.5 **Committees.** The board has at least an audit committee, a risk and capital committee, and a nominations and remuneration committee, each with written
terms and a majority of independent non-executive members. `[ACT]` (any committee the Act requires, such as a directors' affairs or credit committee).

6.6 **Duties and conflicts.** Directors act in the interests of the members as a whole. A director discloses any interest in a matter before the board, takes no part in
the decision, and the interest is recorded. `[ACT]`

6.7 **Removal and vacancies.** A director may be removed by ordinary resolution of the members, after being heard. The board may fill a casual vacancy until the next
annual general meeting. `[ACT]`

### 7. Executive management and key persons

7.1 The board appoints a chief executive officer, a chief financial officer, a chief risk officer, a chief compliance officer and the other executive officers the
Act requires, and the heads of the control functions listed in `docs/governance/ROLES.md`. Each is fit and proper (clause 6.4). `[ACT]`

7.2 The chief executive and the chief accounting officer certify the bank's regulatory returns (form DI 099). `[ACT]` (MR-04).

7.3 The control functions (risk, compliance, internal audit) report to the board's committees, not to the executive they oversee.

### 8. Surplus, reserves and rebates

8.1 The bank's surplus is first applied to the reserves the Act and the Prudential Authority require, and to any reserve the board sets for the bank's safety and growth.
`[ACT]`

8.2 The balance may be returned to members as `[better rates, lower fees, a patronage rebate in proportion to each member's business with the bank, or a dividend on
member shares]`. `[DECISION 2]`. A return is decided by the general meeting on the board's recommendation, and is paid only if the bank would still meet its capital
and liquidity requirements afterwards. `[ACT]`

8.3 **Allocation rule (what the software does).** A rebate is split in proportion to a stated weight (fees paid, average balance or shares held), to the cent, so
the parts always add up to the pool, with leftover cents to the largest remainders and ties broken by member number. **Code:** `membership.Allocate`. The weight
is chosen by `[DECISION 16]`. A distribution is approved by one person and released by another (maker-checker).

### 9. Accounts, audit and reporting

9.1 Financial year: `[1 March to end February]` `[DECISION 17]`. Annual financial statements are audited by an auditor registered under the Act and approved by the
Prudential Authority where required. `[ACT]`

9.2 The directors report each year to the Registrar on the effectiveness of internal control over financial and regulatory reporting and compliance. `[ACT]` (MR-02,
regulation 37(5)).

9.3 Returns to the Prudential Authority reflect the management accounts presented to the board, and the auditor reports on the returns the regulations name. `[ACT]`
(MR-04, MR-05).

### 10. Related parties and the platform company

10.1 The bank's technology is built and run by **MYFORGE (Pty) Ltd** under a written services agreement. That agreement is on arm's-length terms, is approved by the
independent directors, is disclosed to members, and is part of the outsourcing register submitted to the Prudential Authority (register OUT-01, LIC-10).

10.2 The bank, not MYFORGE, is responsible for every outsourced function. The Prudential Authority and the bank's auditor have access to MYFORGE's relevant records and premises.
`[ACT]` (the Prudential Authority's outsourcing directive; `VERIFY`).

10.3 No founder, director or executive receives a benefit from MYFORGE that is connected with the bank's business unless it is disclosed to the board and members.

10.4 The brand "Forge" belongs to `[MYFORGE (Pty) Ltd]` and is licensed to the bank. The licence ends on terms that let the bank continue to serve members. `[DECISION 18]`.

### 11. The sponsor-bank stage and transition

11.1 Until the bank holds its own licence, no customer money is held by the bank; it is held by a licensed sponsor bank under a written agreement, and the bank
describes the arrangement honestly to every customer. `[ACT]` (nothing in this constitution is a licence or a promise of one).

11.2 On registration, customers and members migrate from the sponsor's books to the bank's own books under a migration runbook (**not yet written**), with
notice, reconciliation and the Prudential Authority's approval.

### 12. Member protection

12.1 **Complaints.** Members have a written complaints procedure, a named officer, a time limit for a reply, and the right to take a complaint to the relevant
ombud or regulator. `[ACT]`

12.2 **Information.** Each member receives, free, the annual financial statements, the notice and minutes of meetings, and a plain statement of what is and is not
covered by deposit insurance. `[ACT]` `VERIFY` (the deposit insurance scheme's application to a mutual bank's members and their shares).

12.3 **Data.** Personal information is used only for the purposes the member has been told, with the consents and rights required by law. The bank never sells
member data. Third-party offers shown to members are provided, licensed and monitored by the third party, never by the bank, and details go to a provider only with
the member's consent.

### 13. Winding up, merger and conversion `[ACT]`

13.1 The Act decides how a mutual bank is wound up, merged or converted, and what happens to surplus assets after members' shares and deposits are repaid. **Assets are
not distributed to members beyond the nominal value of their shares** unless the Act requires or permits otherwise. `[DECISION 19]` (an "asset lock", such as giving any
residual to a named public-benefit body or to member-owned institutions).

### 14. Amendments

14.1 This constitution may be amended only by special resolution (clause 5.6) and only with any approval the Act requires. `[ACT]`

14.2 A draft amendment is sent to members with the notice of the meeting. A change that would reduce the one-member-one-vote rule or move assets to outsiders needs
`[DECISION 20: a higher majority]`.

### 15. Founding page (to be completed by the founders)

| # | Founder member | Identity or registration number | Date admitted | Member shares | Signature |
|---|---|---|---|---|---|
| 1 | Tebogo Mvelase | _to complete_ | | | |
| 2 | | | | | |
| 3 | | | | | |
| 4 | | | | | |
| 5 | | | | | |
| 6 | | | | | |
| 7 | | | | | |

**Only one founder is named so far.** The remaining six are people the founder must recruit and who must each pass the checks in clause 3.2 and file the
statements in clause 6.4. Nobody is added to this table by Claude or by software.

### Decisions for the founder and founding members

| # | Decision | Why it matters | Suggested default (PROPOSED) |
|---|---|---|---|
| 1 | The registered name | The Act and the Prudential Authority control "bank" in a name | Keep "Forge" as the working brand; no "bank" until authorised |
| 2 | Return to members: rebate only, or dividends too | A dividend on shares can pull the structure towards a company; the lawyer must advise | Patronage rebate and better rates only |
| 3 | Objects | Defines who the bank exists for | Technology businesses first; savings and credit groups next |
| 4 | Who may be a member (groups, stokvels, MFIs, founders and their companies) | Drives whether the MFI-network idea works under the Act | Registered entities only; individuals as separate members |
| 5 | Grounds and process for removing a member | Fairness and legal challenge | Fraud, serious breach, failed due diligence; hearing and appeal |
| 6 | One share class or more | Capital raising and control | One class |
| 7 | Nominal value and holding limits | Capital depth against concentration of influence | R100 nominal; 5% cap on shares held |
| 8 | Longest deferral of redemptions | Protects capital in stress; affects member trust | To be set with the capital plan and the lawyer |
| 9 | When the AGM is held, and how many members can call a meeting | Member rights | Within six months of year end; 5% of members |
| 10 | Quorum | A meeting that cannot reach quorum cannot decide | Greater of 25 members or 5% |
| 11 | Proxies and electronic voting | Participation across the country; fraud risk | Electronic voting with proof of membership; limit proxies |
| 12 | Other special-resolution matters | Protects members against changes of character | Anything that changes member rights |
| 13 | First board: elected at the first meeting, or appointed by the founders until then | Who runs it before there are members | Founders appoint; members confirm at the first meeting |
| 14 | Board size and terms | Cost and independence | 7 directors, three-year terms, a third retiring each year |
| 15 | Independence standard | Governance quality and the Prudential Authority's view | Majority of non-executives independent |
| 16 | Rebate weight | Fairness between small and large members | Fees paid |
| 17 | Financial year end | Audit timetable | To be set with the auditor |
| 18 | Ownership and licence of the Forge brand | A mutual cannot be controlled through its brand | Licence from MYFORGE with a no-hold-up clause |
| 19 | Asset lock on winding up | Mutual character | Residual to member-owned or public-benefit bodies |
| 20 | Supermajority for changes that remove member rights | Stops a later majority undoing mutuality | 90% of votes cast |

### What this draft does not do

- It does not satisfy the Act. The lawyer must add every `[ACT]` provision and check the figures marked `VERIFY`.
- It does not choose the legal form. If the lawyer advises the bank route (a public company), this becomes articles of association and a shareholders'
  agreement instead (see `docs/governance/LEGAL_FORMS.md`).
- It does not name anyone other than the founder, and nothing in it is signed.
- It is not in force, and nothing in the software depends on it being approved. The code's rules in the **Code:** notes are the same either way and will be
  aligned with the final text.


## Governance structure

*Source: `docs/governance/STRUCTURE.md`*

## Governance and organisation structure

> **DRAFT. Not approved.** This is the structure most South African banks use: a board with its committees, an
> executive committee, and three lines of defence. It is cut down to fit a small, technology-led bank that starts on
> a sponsor bank and applies for its own licence later. Every seat except the founder's is **VACANT**. The roster with
> what each seat needs is in `roles.csv` (`docs/governance/roles.csv`) and `ROLES.md` (`docs/governance/ROLES.md`). Nothing here is legal advice; a
> regulatory lawyer must confirm it against the Banks Act (or Mutual Banks Act), the Companies Act and the Prudential
> Authority's current requirements.

**Founder:** Tebogo Mvelase, proposed Chief Executive Officer and executive director. The fit-and-proper assessment
(BA 020 / DI 020) has not been done.

### 1. The whole structure

```mermaid
flowchart TB
  SH["Shareholders / depositor-members<br/>(general meeting)"]
  PA["Prudential Authority<br/>FSCA, FIC, Information Regulator,<br/>SARB FinSurv, NCR"]
  SPON["Sponsor bank<br/>(until Forge holds a licence)"]
  BOARD["Board of directors<br/>Chair: VACANT (independent)<br/>CEO: Tebogo Mvelase · CFO: VACANT<br/>4+ independent non-executives: VACANT"]
  SH --> BOARD
  PA -. supervises .-> BOARD
  SPON -. programme oversight .-> BOARD

  subgraph BC["Board committees (chaired by independent non-executives)"]
    AUD["Audit Committee"]
    RCC["Risk and Capital<br/>Management Committee"]
    ITC["IT and Cyber<br/>Committee"]
    REM["Remuneration<br/>Committee"]
    NOM["Directors' Affairs and<br/>Nominations Committee"]
    SEC["Social and Ethics<br/>Committee"]
    BCC["Board Credit<br/>Committee"]
  end
  BOARD --> AUD & RCC & ITC & REM & NOM & SEC & BCC

  EXCO["Executive Committee<br/>Chair: CEO Tebogo Mvelase"]
  BOARD --> EXCO

  subgraph MC["Management committees"]
    ALCO["ALCO<br/>(assets, liabilities, liquidity)"]
    CRC["Credit Committee"]
    ORC["Operational Risk and<br/>Control Committee"]
    FCC["Financial Crime Committee"]
    PAC["Product Approval Committee"]
    CAB["Change Advisory Board"]
    DGC["Data Governance Council"]
    IMC["Incident Management Team"]
  end
  EXCO --> ALCO & CRC & ORC & FCC & PAC & CAB & DGC & IMC

  IA["Internal Audit (third line)<br/>VACANT"]
  IA == reports directly ==> AUD
  EA["External auditors<br/>VACANT"] -.-> AUD
```

### 2. Executive organisation chart

Solid lines are management reporting. Dotted lines are the direct, unfiltered access a control function has to its
board committee: the CRO, the MLRO and the head of internal audit can go to the board without the CEO.

```mermaid
flowchart TB
  CEO["Chief Executive Officer<br/><b>Tebogo Mvelase</b>"]
  CEO --> CFO["Chief Financial Officer<br/>VACANT"]
  CEO --> CRO["Chief Risk Officer<br/>VACANT"]
  CEO --> CCO["Chief Compliance Officer<br/>VACANT"]
  CEO --> COO["Chief Operating Officer<br/>VACANT"]
  CEO --> CTO["Chief Technology Officer<br/>VACANT"]
  CEO --> CRED["Head of Credit<br/>VACANT"]
  CEO --> PROD["Head of Product<br/>VACANT"]
  CEO --> LEG["Head of Legal<br/>VACANT (external counsel at start)"]
  CEO --> HR["Head of People<br/>VACANT"]
  CEO --> IO["Information Officer (POPIA)<br/><b>Tebogo Mvelase</b> by default"]

  CFO --> TRE["Head of Treasury<br/>VACANT"]
  CFO --> FC["Financial Controller<br/>VACANT"]
  CFO --> RR["Regulatory Reporting<br/>VACANT"]
  CRO --> CISO["CISO<br/>VACANT"]
  CRO --> ITR["Head of IT Risk<br/>VACANT"]
  CRO --> DATA["Head of Data and Data Protection<br/>VACANT (Deputy Information Officer)"]
  CCO --> MLRO["MLRO<br/>VACANT"]
  MLRO --> DMLRO["Deputy MLRO<br/>VACANT"]
  MLRO --> FCA["Financial Crime Analytics<br/>VACANT"]
  COO --> PAY["Head of Payments Operations<br/>VACANT"]
  COO --> CS["Customer Service and Complaints<br/>VACANT"]
  COO --> FR["Fraud Lead<br/>VACANT"]
  COO --> BCM["Business Continuity Manager<br/>VACANT"]
  CRED --> CO["Credit Officers<br/>VACANT"]

  BRCC(["Board Risk and Capital Committee"])
  BAUD(["Board Audit Committee"])
  CRO -.-> BRCC
  MLRO -.-> BRCC
  CISO -.-> BRCC
  CCO -.-> BAUD
  HIA["Head of Internal Audit<br/>VACANT"] ==> BAUD
  SECY["Company Secretary<br/>VACANT"] --> BOARDC(["Board chair"])
```

### 3. Three lines of defence

```mermaid
flowchart LR
  subgraph L1["First line: owns and runs the risk"]
    direction TB
    a1["Operations and payments"]
    a2["Treasury"]
    a3["Credit"]
    a4["Technology (CTO)"]
    a5["Product"]
    a6["Finance"]
  end
  subgraph L2["Second line: sets the rules and challenges"]
    direction TB
    b1["Risk (CRO)"]
    b2["Compliance (CCO)"]
    b3["Financial crime (MLRO)"]
    b4["Information security (CISO)"]
    b5["IT risk"]
    b6["Data protection"]
  end
  subgraph L3["Third line: independent assurance"]
    direction TB
    c1["Internal audit"]
  end
  L1 -- "reports risks, incidents, breaches" --> L2
  L2 -- "policy, limits, challenge" --> L1
  L3 -- "audits both" --> L1 & L2
  L3 ==> AC(["Board Audit Committee"])
  L2 ==> RC(["Board Risk and Capital Committee"])
  EXT["External audit, regulators"] -.-> AC
```

How it maps to this repository: first-line controls are code (the ledger, limits, the licence gate, maker-checker in
the back office); second-line rules are data owned by a named person (`docs/THRESHOLDS.md`, the policy engine,
screening lists); third-line evidence is the append-only audit log and the evidence packs.

### 4. Committees: who sits, how often, what they decide

| Committee | Chair | Members | Meets | Decides or recommends | Terms of reference |
|---|---|---|---|---|---|
| Board | Independent chair (VACANT) | All directors | Quarterly, plus ad hoc | Strategy, risk appetite, policies, capital, appointments | board charter (`docs/regulatory/policies/board-charter.md`) |
| Audit Committee | Independent NED, CA(SA) (VACANT) | 3 independent NEDs | Quarterly | Financial statements, returns, internal and external audit, internal controls | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| Risk and Capital Management Committee | Independent NED (VACANT) | 3 NEDs, CEO, CRO, CFO attend | Quarterly | Risk appetite, ICAAP, ILAAP, recovery plan, risk policies, stress tests | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| IT and Cyber Committee | Independent NED, technology (VACANT) | 2 NEDs, CTO, CISO, Head of IT Risk attend | Quarterly | IT strategy, cyber posture, material IT changes, outsourcing of IT | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| Remuneration Committee | Independent NED (VACANT) | 3 NEDs | Twice a year | Pay of executives and control functions; clawback | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| Directors' Affairs and Nominations | Board chair (VACANT) | 3 NEDs | Twice a year | Board composition, fit and proper, succession, evaluations | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| Social and Ethics Committee | Independent NED (VACANT) | 1 NED, 2 others | Twice a year | Companies Act s.72 duties; conduct and customer outcomes | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| Board Credit Committee | Independent NED (VACANT) | 2 NEDs, CEO, CRO, Head of Credit | Monthly once credit is live | Large exposures and facilities above the management mandate | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| Executive Committee | CEO Tebogo Mvelase | All executives | Weekly | Running the bank within the appetite | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| ALCO | CFO (VACANT) | CEO, CRO, Head of Treasury | Monthly | Liquidity, funding, interest-rate risk, FX positions, rates paid | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| Credit Committee | Head of Credit (VACANT) | CRO, CFO, credit officers | Weekly | Facilities within mandate; arrears and write-offs | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| Operational Risk and Control | CRO (VACANT) | COO, CTO, CISO, CCO | Monthly | Incidents, losses, control failures, risk register | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| Financial Crime Committee | MLRO (VACANT) | CCO, COO, Fraud lead | Monthly | Monitoring rules, case backlog, exits, sanctions | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| Product Approval Committee | Head of Product (VACANT) | CRO, CCO, CFO, COO, CTO | As needed | New products and changes; signs the licence-gate decision `DEC-nnnn` | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| Change Advisory Board | CTO (VACANT) | CISO, Head of IT Risk, COO | Weekly | Production changes; emergency changes after the fact | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |
| Data Governance Council | Head of Data (VACANT) | CTO, CISO, CCO, Information Officer | Quarterly | Data classes, retention, residency, access | terms (`docs/regulatory/policies/committee-terms-of-reference.md`) |

### 5. Policy hierarchy

```mermaid
flowchart TB
  RAF["Risk management framework and risk appetite statement<br/>(board)"]
  RAF --> POL["Policies<br/>(board or a board committee approves)"]
  POL --> STD["Standards<br/>(executive committee approves)"]
  STD --> PROC["Procedures and runbooks<br/>(the function head approves)"]
  PROC --> CTRL["Controls in code and configuration<br/>(this repository, CI gate, thresholds)"]
  CTRL -. evidence .-> REG["Regulatory register<br/>docs/regulatory/register.csv"]
  REG -. gaps .-> RAF
```

The full list of policies, their owners and approvers is in `POLICY_FRAMEWORK.md` (`docs/governance/POLICY_FRAMEWORK.md`).

### 6. Who can approve what

| Decision | Proposes | Approves | Second pair of eyes | Recorded in |
|---|---|---|---|---|
| Risk appetite statement | CRO | Board, on Risk Committee recommendation | Internal audit reviews yearly | Board minutes, `docs/DECISIONS.md` |
| A policy | Policy owner | Committee in `POLICY_FRAMEWORK.md` | Second line | Policy header |
| A threshold value | Owner in `THRESHOLDS.md` | Approver in `THRESHOLD_OWNERS.md` | Never the proposer | `docs/DECISIONS.md` |
| A new product or turning one on (`FORGE_APPROVAL_*`) | Head of Product | Product Approval Committee, then Risk Committee for credit and deposits | CCO and CRO | `docs/DECISIONS.md` as `DEC-nnnn` |
| A credit facility | Credit officer | Within mandate: credit officer; above: Credit Committee; large exposure: Board Credit Committee | CRO for large exposures | Lending service decision record |
| A material outsourcing | Function head | Board, after PA notice where required | CRO, Head of Legal | Outsourcing register |
| A production change | Engineer | Change Advisory Board (two-person review in code) | CISO for security changes | Git history, CI gate |
| A suspicious transaction report | Analyst | MLRO | Deputy MLRO | Compliance case |
| Freezing or unfreezing a customer | Compliance engine / analyst | MLRO | CCO | Party status history, case |
| Writing off a loan | Head of Credit | Credit Committee | CFO | Ledger, decision record |

### 7. How the structure grows

Not every seat is needed on day one. The order follows what must exist before each stage.

```mermaid
gantt
  title Seats to fill by stage (indicative, dates to be set by the owner)
  dateFormat YYYY-MM-DD
  axisFormat %b %Y
  section Stage 0 · Now
  Founder and CEO (Tebogo Mvelase)            :done, s0a, 2026-11-01, 30d
  Head of Legal (external counsel)             :s0b, 2026-12-01, 30d
  Sponsor bank relationship                    :s0c, 2026-12-01, 60d
  section Stage 1 · Before the first customer (sponsor model)
  MLRO and Deputy MLRO                         :s1a, 2027-01-01, 60d
  Chief Compliance Officer                     :s1b, 2027-01-01, 60d
  Chief Risk Officer                           :s1c, 2027-01-01, 60d
  CTO and CISO                                 :s1d, 2027-01-15, 60d
  Deputy Information Officer                   :s1e, 2027-02-01, 30d
  Head of Payments Operations                  :s1f, 2027-02-01, 45d
  section Stage 2 · Before the licence application
  Board chair and 4 independent directors      :s2a, 2027-03-01, 120d
  CFO, Financial Controller, Regulatory reporting :s2b, 2027-03-01, 90d
  Company Secretary                            :s2c, 2027-03-01, 30d
  Head of Internal Audit (or outsourced)       :s2d, 2027-04-01, 60d
  Head of Treasury, Head of IT Risk            :s2e, 2027-04-01, 60d
  External auditors                            :s2f, 2027-05-01, 60d
  section Stage 3 · Before credit and the public launch
  Head of Credit and credit officers           :s3a, 2027-07-01, 60d
  Head of Product, Customer Service, Fraud     :s3b, 2027-07-01, 60d
  Business Continuity Manager, Head of People  :s3c, 2027-08-01, 45d
```

### 8. Rules for sharing seats while small

- The CEO may not also be the chair, the CRO, the MLRO or the head of internal audit.
- The CRO may not be the CFO or run any first-line function.
- The MLRO and CCO may be one person only if the Prudential Authority and the sponsor bank accept it; then the Deputy
  MLRO must be appointed, because someone else must approve the MLRO's own changes.
- Internal audit may be outsourced, but only with the PA's prior written approval (register row BR-05), and never to
  the external auditor.
- At least two board members must be employees (register row MR-03 for a mutual bank): the CEO and the CFO.
- A majority of the board, and every chair of the audit and risk committees, should be independent non-executives.


## Policies mapped to the regulator's structure

*Source: `docs/governance/POLICY_MAP.md`*

## Policies mapped to the Prudential Authority's structure

> Generated by `scripts/build-policy-map.py` from `pa-framework.csv` (`docs/governance/pa-framework.csv`) and `policies.csv` (`docs/governance/policies.csv`). Every policy is
> a DRAFT. **Announced** areas follow the seven prudential standards for mutual banks that the PA says it has developed but has not
> published (PA annual reports 2024/25 and 2025/26): when they are published, each policy in that area must be checked against the text.
> **Published** areas cite instruments already in force.

| Area | Regulator's instruments | Status | Policies |
|---|---|---|---|
| **PA-01** Governance and risk management | PA prudential standard for mutual banks: governance and risk management (announced, not published); Mutual Banks regulations 37 and 38 (risks to manage, at least two employee directors); King IV | announced | Board charter (`docs/regulatory/policies/board-charter.md`)<br>Terms of reference for board and management committees (`docs/regulatory/policies/committee-terms-of-reference.md`)<br>Conflicts of interest and related-party policy (`docs/regulatory/policies/conflicts-of-interest-policy.md`)<br>Code of conduct and ethics (`docs/regulatory/policies/code-of-conduct.md`)<br>Whistleblowing policy (`docs/regulatory/policies/whistleblowing-policy.md`)<br>Remuneration policy (`docs/regulatory/policies/remuneration-policy.md`)<br>Risk management framework (`docs/regulatory/policies/risk-management-framework.md`)<br>Risk appetite statement (`docs/regulatory/policies/risk-appetite-statement.md`)<br>Risk register (initial, complete list) (`docs/regulatory/policies/risk-register.md`)<br>New product approval policy (`docs/regulatory/policies/new-product-approval-policy.md`) |
| **PA-02** Operational risk | PA prudential standard for mutual banks: operational risk management (announced) | announced | Operational risk policy (`docs/regulatory/policies/operational-risk-policy.md`)<br>Business impact analysis (`docs/regulatory/policies/business-impact-analysis.md`)<br>Business continuity, disaster recovery and recovery plan (`docs/regulatory/policies/recovery-plan.md`)<br>Cash policy (`docs/regulatory/policies/cash-policy.md`) |
| **PA-03** Credit risk | PA prudential standard for mutual banks: credit risk (announced); DI 500 and DI 510 instructions (Annexure Q2) | announced | Credit risk policy (`docs/regulatory/policies/credit-risk-policy.md`)<br>Large exposures and investment limits policy (`docs/regulatory/policies/large-exposures-policy.md`) |
| **PA-04** Liquidity risk | PA prudential standard for mutual banks: liquidity risk (announced); Mutual Banks Act s.50 liquid assets; DI 300 and DI 310; SARB Act s.10A reserve balance | announced | Liquidity risk framework and contingency funding plan (Mutual Banks Act s.50 and Banks Act regulations) (`docs/regulatory/policies/liquidity-and-contingency-funding-plan.md`)<br>Deposit concentration and liquidity limits policy (`docs/regulatory/policies/concentration-policy.md`) |
| **PA-05** Interest rate risk in the banking book | PA prudential standard for mutual banks: IRRBB (announced); DI 410; Banks Directive 3 of 2026 as the nearest published comparison | announced | Market and interest rate risk policy (`docs/regulatory/policies/market-and-interest-rate-risk-policy.md`) |
| **PA-06** Capital | PA prudential standard for mutual banks: capital (announced); Mutual Banks Act s.48; regulation 23; DI 400 | announced | Capital management policy and ICAAP (`docs/regulatory/policies/capital-management-policy.md`)<br>Membership and surplus distribution policy (`docs/regulatory/policies/membership-and-surplus-policy.md`) |
| **PA-07** Returns and reporting | PA prudential standard for mutual banks: economic returns (announced); proposed directive on DI returns (13 August 2025); Mutual Banks regulations 2, 6 and 7 (returns reflect management accounts; DI 099) | announced | Regulatory returns policy (`docs/regulatory/policies/regulatory-reporting-policy.md`)<br>Internal control over financial reporting (`docs/regulatory/policies/internal-control-over-financial-reporting.md`) |
| **PA-08** Currency risk | Mutual Banks regulation 31(6) (effective net open position limit); DI 600; exchange control | published | Market and interest rate risk policy (`docs/regulatory/policies/market-and-interest-rate-risk-policy.md`) |
| **PA-09** Investment and exposure limits | Mutual Banks Act ss.55 and 56; DI 700; DI 505 and DI 510 | published | Large exposures and investment limits policy (`docs/regulatory/policies/large-exposures-policy.md`)<br>Capital management policy and ICAAP (`docs/regulatory/policies/capital-management-policy.md`) |
| **PA-10** Fitness and propriety | Mutual banks circular MB C2-2023; form DI 020; Mutual Banks Act | published | Fit-and-proper procedure (`docs/regulatory/policies/fit-and-proper-procedure.md`)<br>Staff, contractor and provider screening (Joint Standard 1 of 2023; FICA s.42A) (`docs/regulatory/policies/staff-screening.md`)<br>Shareholders, significant ownership and founders (`docs/regulatory/policies/shareholder-and-significant-ownership-policy.md`) |
| **PA-11** Outsourcing and third parties | PA Guidance Note 5 of 2014; Directive 3 of 2018 (cloud and offshoring); Prudential Communication 3 of 2025 (third-party risk, mutual banks) | published | Outsourcing and third-party policy (`docs/regulatory/policies/outsourcing-policy.md`)<br>Cloud computing and offshoring of data policy (`docs/regulatory/policies/cloud-and-offshoring-policy.md`)<br>Provider due diligence procedure (`docs/regulatory/policies/provider-due-diligence-procedure.md`)<br>Exit plans for providers (Directive 3 of 2018; Guidance Note 5 of 2014) (`docs/regulatory/policies/exit-plans.md`)<br>Provider contract schedule: minimum clauses (`docs/regulatory/policies/provider-contract-schedule.md`) |
| **PA-12** IT risk and cyber resilience | Joint Standard 1 of 2023 (IT governance and risk management); Joint Standard 2 of 2024 (cybersecurity and cyber resilience); Joint Notice 2 of 2026 (incident reporting) | published | IT governance and risk management framework (`docs/regulatory/policies/it-risk-management-framework.md`)<br>IT strategy (Joint Standard 1 of 2023) (`docs/regulatory/policies/it-strategy.md`)<br>IT service management: change, release, incident, problem and capacity (Joint Standard 1 of 2023) (`docs/regulatory/policies/it-service-management.md`)<br>IT programme and project framework (Joint Standard 1 of 2023) (`docs/regulatory/policies/it-project-framework.md`)<br>Cyber and information security strategy and framework (`docs/regulatory/policies/cyber-strategy.md`)<br>Access control and privileged access policy (`docs/regulatory/policies/access-control-policy.md`)<br>Cryptographic key management policy (`docs/regulatory/policies/key-management-policy.md`)<br>Secure development and open-source policy (`docs/regulatory/policies/secure-development-policy.md`)<br>Secure configuration standard (Joint Standard 2 of 2024: configuration and patching) (`docs/regulatory/policies/secure-configuration-standard.md`)<br>Threat intelligence and security testing policy (`docs/regulatory/policies/threat-intelligence-and-security-testing-policy.md`)<br>Incident response plan (`docs/regulatory/policies/incident-response-plan.md`)<br>Regulatory incident notification procedure (`docs/regulatory/policies/regulatory-incident-notification-procedure.md`)<br>Business continuity and disaster recovery policy (`docs/regulatory/policies/business-continuity-and-disaster-recovery-policy.md`)<br>Cyber awareness and training plan (`docs/regulatory/policies/training-plan.md`) |
| **PA-13** Financial crime | FIC Act s.42 (risk management and compliance programme), ss.28 and 29 (cash threshold and suspicious reports); PA AML/CFT communications | published | Compliance charter (`docs/regulatory/policies/compliance-charter.md`)<br>Risk Management and Compliance Programme (FICA s.42) (`docs/regulatory/policies/rmcp.md`)<br>Cash threshold reporting procedure (`docs/regulatory/policies/cash-threshold-reporting-procedure.md`) |
| **PA-14** Deposit insurance | FSR Act Chapter 12A; CODI standards; Mutual Banks Directive 1 of 2024 (covered deposits) | published | Deposit insurance policy (CODI) (`docs/regulatory/policies/deposit-insurance-policy.md`) |
| **PA-15** Conduct and data protection | POPIA; PAIA; Treating Customers Fairly; FSCA conduct standards | published | Complaints and treating customers fairly policy (`docs/regulatory/policies/complaints-and-fair-treatment-policy.md`)<br>Data strategy and classification policy (`docs/regulatory/policies/data-strategy.md`)<br>Data retention schedule (POPIA s.14; FICA s.22, s.23) (`docs/regulatory/policies/data-retention-schedule.md`)<br>Privacy notice (POPIA s.18): customer-facing draft (`docs/regulatory/policies/privacy-notice.md`)<br>POPIA compliance framework and PAIA manual outline (`docs/regulatory/policies/popia-compliance-framework.md`)<br>Naming, marketing and public statements policy (`docs/regulatory/policies/naming-and-marketing-policy.md`) |
| **PA-16** Internal audit and assurance | Mutual Banks regulation 37(5) (directors' report on internal control); regulation 6 (auditor's reports on returns) | published | Internal audit charter (`docs/regulatory/policies/internal-audit-charter.md`)<br>Internal control over financial reporting (`docs/regulatory/policies/internal-control-over-financial-reporting.md`) |
| **PA-17** Licensing and mutual form | Mutual Banks Act (registration, founders, member shares); PA licensing guide | published | Licence application plan (`docs/regulatory/policies/licence-application-plan.md`)<br>Business plan outline (application pack) (`docs/regulatory/policies/business-plan-outline.md`)<br>Membership and surplus distribution policy (`docs/regulatory/policies/membership-and-surplus-policy.md`)<br>Shareholders, significant ownership and founders (`docs/regulatory/policies/shareholder-and-significant-ownership-policy.md`) |

17 areas; 58 policies, each under at least one area.


## Relationship service standard

*Source: `docs/governance/SERVICE_STANDARD.md`*

## Client service standard (relationship managers)

> **DRAFT for the Head of Product and the owner.** Investec's signature is a named banker who knows the client
> (`docs/BANK_MODEL.md`). This is the standard we would hold ourselves to. Every number is PROPOSED. The seat is
> R37 in `roles.csv` and is **vacant**; no relationship manager exists and no tooling supports one yet.

### Who gets a relationship manager

| Client | Service | Why |
|---|---|---|
| Business with deposits over R1 000 000, credit over R500 000, or a founder who asks | **Named relationship manager** | The relationship is the product. |
| Every other business | **Pooled service team**, one named lead | A person who knows the account without a one-to-one ratio. |
| Individual (founder) | Served through their business's manager | One conversation, not two. |

PROPOSED ratio: one relationship manager to 40 to 60 business clients. Credit decisions stay with credit officers
(`R16`); a relationship manager introduces and explains, and never approves or promises credit.

### The standard

| Moment | Commitment (PROPOSED) |
|---|---|
| A client writes or calls | Acknowledged in **1 business hour**; answered or a firm date given in **1 business day** |
| Urgent (payment held, account frozen, suspected fraud) | A person responds in **30 minutes**, 06:00 to 22:00 every day; out of hours through the 24/7 line |
| A complaint | Acknowledged in 1 business day; resolved in **15 business days**; the Ombudsman for Banking Services named in every final reply (`complaints-and-fair-treatment-policy.md`) |
| Onboarding | A named person from the first call; decision communicated within 2 business days of a complete application |
| Review | Every client contacted at least **twice a year**; the top 20% by deposits quarterly |
| Fees or terms change | Notice at least 60 days before (confirm the legal minimum with the lawyer) |

### What a relationship manager may and may not do

- **May:** explain products and fees; introduce credit, foreign currency and card acceptance; help with onboarding;
  chase an internal team; arrange a callback from compliance; refer to a partner for investments (`docs/products/WEALTH_PARTNERSHIP.md`).
- **May not:** give investment, tax or legal advice; approve credit; release a held payment or unfreeze a customer;
  tell a client that a report or investigation exists or why they were declined (no tipping off); take cash; share one
  client's information with another.
- **Must:** record every material conversation in the client file; escalate any suspicion to the MLRO the same day;
  use the client's consented contact channels only.

### Vulnerable clients and conduct

Treating Customers Fairly applies. A relationship manager who sees signs of distress, confusion, coercion or fraud
risk pauses the request and escalates to the Customer Service lead and, if financial crime is suspected, the MLRO.
Training: onboarding week, then yearly (`training-plan.md`).

### What we would measure

Response times against the table; complaints per 1 000 clients and days to resolve; client retention; share of clients
with a review in the year; the quality score from a short client survey after each resolved request. Reported monthly
to the executive committee, quarterly to the Social and Ethics Committee.

### What is needed to run it

1. Hire the seat (R37), with Customer Service (R19) in place first.
2. A client record: who the manager is, contact history, preferences, consents. **Not built.** The party service holds
   identity and status only; a relationship record would be a new table and a back-office screen.
3. A queue and clock for the commitments above. **Not built.**
4. Reporting from items 2 and 3. **Not built.**

Until then this is a promise on paper, and nothing should be advertised.


## Brief for the regulatory lawyer

*Source: `docs/governance/LAWYER_BRIEF.md`*

## Brief for a regulatory lawyer

> For **Tebogo Mvelase** to send. The aim is to find a South African financial-services regulatory lawyer to review
> the governance pack and advise on the legal form, then to help with the licence application. I can't appoint anyone
> or check a firm's current team, so verify every candidate yourself.
>
> **Revised 2026-11-24** to cover the draft constitution and business plan, which the first version predates. Documents to
> share under NDA with this brief: `docs/licence-application/CONSTITUTION_DRAFT.md`, `docs/licence-application/BUSINESS_PLAN_DRAFT.md`,
> `docs/governance/SPONSOR_BANK_PITCH.md` and the register. **All are drafts, written by an AI assistant from the repository; none is
> legal advice, and none has been sent to anyone.**

### What we need from them

**Phase 1: advice (fixed fee, about 4 to 6 weeks)**

1. **Legal form:** a mutual bank (planned) or a bank under the Banks Act (kept open). Confirm the current minimum
   capital, founder and board rules, member-share treatment and any amendments since 1993 (`LEGAL_FORMS.md`). Also: **is the mutual form
   available and sensible for a membership of technology businesses (and, later, savings groups and micro-lenders)?** Do member shares
   count towards the minimum capital, and what other capital instruments does the Prudential Authority accept for a mutual?
2. **The draft constitution** (`CONSTITUTION_DRAFT.md`). Please (a) add every provision the Mutual Banks Act and regulations require (marked
   `[ACT]`), (b) correct anything the Act forbids, and (c) advise on the 20 numbered decisions at the end. The ones that matter most:
   - **Who may be a member:** companies, co-operatives, stokvels and informal savings groups, micro-finance institutions, a founder and their
     company as separate members (decision 4).
   - **Return to members:** patronage rebates only, or a dividend on member shares (decision 2), and whether one share class is enough (6).
   - **Redemption:** the capital check and how long redemptions may be deferred (8); share nominal value and holding limits (7).
   - **Voting and meetings:** proxies and electronic voting, quorum, and the majorities for ordinary and special resolutions (5.5, 5.6, decisions
     10 to 12). The software already enforces one member one vote, more than half for ordinary and three quarters for special resolutions, a
     record date and a quorum; it will be changed to match whatever you advise.
   - **Board:** at least two employee directors (MR-03), independence, and the first board before members exist (13 to 15).
   - **Asset lock and winding up** (19), and the supermajority to protect mutuality (20).
3. **Money before registration.** Can prospective members or founders pay in share capital, or can anyone pay deposits or commitments, **before
   registration**, and under what conditions (escrow or trust, public-offer or prospectus rules, the prohibition on taking deposits without a
   licence)? The draft constitution and capital plan assume founders can subscribe; nothing confirms it (`BUSINESS_PLAN_DRAFT.md` section 9).
4. **Sponsor model:** what Forge may do before it holds a licence, under a sponsor bank's licence, and what the sponsor contract must cover
   (`SPONSOR_BANK_PITCH.md`, especially "Positions we hold": no exclusivity or non-compete that blocks our own application, and a contract
   that can be novated to the future bank). Can Forge call itself anything containing "bank" or "mutual" before authorisation, including in a
   pitch to a bank (LIC-17)? What wording is safe ("proposed mutual bank, not yet registered"?).
5. **Which built products need their own registration or approval.** The platform has built, and switched off, savings and notice deposits,
   working-capital credit, merchant acquiring, virtual debit cards and third-party offers. For each: can it be offered on a sponsor's licence,
   what must Forge itself hold (NCR, PASA, FSCA, other), and is any of it safe to include in a first launch? The first launch is intended to be
   accounts, payments and FX only.
6. **Related party: the platform company.** MYFORGE (Pty) Ltd, controlled by the founder, builds and runs the platform and would be the bank's
   principal technology supplier. Advise on the services agreement, a brand licence that cannot hold the mutual to ransom, director
   independence, disclosure to members, and how this appears in the outsourcing submission (OUT-01, OUT-06, LIC-10).
7. **Third-party offers and referral fees** (`services/platform/partner-offers/SPEC.md`). Please confirm: the wording of the statement
   customers must acknowledge; whether Forge may receive a referral fee from a licensed provider (and any cap or disclosure the law requires);
   whether introducing customers to financial-services or crypto-asset providers is itself regulated activity; and POPIA for sending a
   consenting customer's **name** to the provider (operator or responsible party, cross-border, retention, what withdrawing consent must do).
8. **Review the governance pack:** the structure (`STRUCTURE.md`), the 37 seats (`ROLES.md`), and the 58 draft policies
   (`POLICY_FRAMEWORK.md`). Do they meet the Banks Act or Mutual Banks Act regulations, King IV, Joint Standards 1
   and 2, Directive 3/2018 and the Guidance Notes?
9. **Regulatory register:** check the 114 requirements (`docs/regulatory/register.csv`) and add the laws not yet read:
   FICA in full, POPIA, the National Payment System Act, exchange control, the National Credit Act, the CODI
   (deposit insurance) rules, COFI if in force, and the FSCA's crypto-asset rules. Row OUT-02 (20 working days' notice to the PA before material
   outsourcing) is typed "Bank" only: **does it also apply on the mutual route?**
10. **Crypto:** what a bank or mutual bank may do for FSCA-licensed crypto-asset service providers, and the
    PA's current position on banks holding or lending against crypto assets.
11. **Credit:** NCR registration, and the default notice and statement wording we built (NCA s.108 and s.129).
12. **Digital assets:** the eight questions in `docs/products/DIGITAL_ASSETS.md`: whether a tokenised deposit is a deposit, the
    FSCA's position, reserve and capital treatment, the travel rule, and exchange control.
13. **Outsourcing and cloud:** which of our providers (WorkOS for sign-in, likely US processing; Didit for identity checks, location
    unconfirmed; hosting; a sanctions-list provider) are material, what notices to the PA are needed, and what data may flow to each.

**Phase 2: the application (quoted separately)**

- Founding documents: finalise the mutual constitution (or company articles), member or shareholder agreements, and the
  **services agreement and brand licence between MYFORGE (Pty) Ltd and the bank**.
- The pre-application meeting with the Prudential Authority, and FSCA concurrence.
- DI or BA application forms and fit-and-proper filings (DI 020 / BA 020).
- Provider contracts: sponsor bank, partner banks, acquirer, identity and KYC providers.
- Customer terms, privacy notice, PAIA manual, credit agreements.

### What to look for

- A partner who has taken a new bank, mutual bank or co-operative bank through PA registration in the last 10 years,
  or acted on a sponsor-bank (banking-as-a-service) arrangement.
- **Experience drafting a mutual or co-operative constitution** (not only company articles), and with member-share capital.
- Current relationships with the Prudential Authority and the FSCA; fintech and crypto-asset experience.
- **No conflict:** they must not act for the sponsor bank we choose, or be the firm a sponsor uses for its own due diligence.
- Willing to quote phase 1 as a fixed fee.
- Can work from documents in a GitHub repository, or will accept a PDF export.

### Where to look

Large South African firms with financial-services regulatory practices: Bowmans, Webber Wentzel, ENSafrica,
Cliffe Dekker Hofmeyr, Werksmans, and the Johannesburg offices of Norton Rose Fulbright, Baker McKenzie and Herbert
Smith Freehills. Smaller fintech-focused boutiques often cost less for phase 1. Ask for a short shortlist through:

- the founders of recently licensed South African digital banks, who often name their counsel;
- the Banking Association South Africa;
- the Intergovernmental Fintech Working Group's innovation hub contacts;
- your accountants, since the audit firms work with these lawyers on licence applications.

The firm names are from general knowledge. Check each firm's current team and conflicts before you approach them.

### First email (to adapt)

> Subject: Regulatory advice: proposed mutual bank (pre-licence)
>
> Dear [name],
>
> I am the founder of Forge, a technology company (MYFORGE (Pty) Ltd) that is building the platform for a proposed technology-led mutual bank in South
> Africa. Nothing is registered or licensed yet. We plan to begin on a sponsor bank's licence and to apply for registration as a mutual bank, keeping a
> Banks Act registration as an alternative.
>
> We have built the banking platform and drafted a governance pack: a board and committee structure, 37 roles, 58 draft policies and a register of 114
> regulatory requirements. We have also prepared draft versions of a mutual-bank constitution and a business plan, and a pitch for a sponsor bank.
> They are working drafts, written to give counsel something to correct; we do not treat any of them as advice.
>
> We are looking for counsel to: (1) advise on the legal form, member-share capital, and what we may do and say before registration; (2) review and
> correct the draft constitution; (3) advise on the sponsor model, the platform company as a related party, and referral fees and data sharing for
> third-party offers; (4) review the governance pack and register; and (5) quote for the licence application.
>
> Could you let me know whether you have capacity and no conflict (in particular, that you do not act for a bank we may approach as a sponsor), and
> send a fixed-fee estimate for the first phase? I can share the documents under NDA.
>
> Kind regards,
> Tebogo Mvelase, Founder

### Tracking

| Firm | Contacted | Conflict check | Phase 1 quote | Notes |
|---|---|---|---|---|
| | | | | |
| | | | | |
| | | | | |

When one is appointed, set seat R26 in `roles.csv` to the firm and partner's name and run `make governance`.


## Sponsor bank pitch

*Source: `docs/governance/SPONSOR_BANK_PITCH.md`*

## Sponsor bank: pitch, questions and checklist

> **DRAFT for Tebogo Mvelase to adapt. Revised 2026-11-24 after a review against `CONSTITUTION_DRAFT.md` and `BUSINESS_PLAN_DRAFT.md`.** A sponsor
> bank is the fastest legal route to customers: it holds the licence and the deposits, and Forge sits on its rails (`BankLayer`,
> `docs/ARCHITECTURE.md` section 5.2). Nothing here has been sent. I can't confirm which South African banks offer such arrangements, or on what
> terms; the shortlist at the end is a list of kinds of counterparty to approach, to be checked by you and the lawyer (`LAWYER_BRIEF.md`).
>
> **Before this goes anywhere:** the lawyer must confirm the wording marked ⚖ below (how Forge may describe itself before authorisation, LIC-17) and
> review the positions in "Positions we hold". Do not send the illustrative pro forma as a forecast.

### The pitch in one page

**Who we are.** The contracting company today is **MYFORGE (Pty) Ltd**, which builds and runs the Forge platform; its founder is Tebogo Mvelase. MYFORGE
intends to support the formation of a **proposed mutual bank, not yet registered** ⚖ (with a bank registration as the alternative; `LEGAL_FORMS.md`). **No
bank, mutual bank or other regulated entity exists yet, and MYFORGE is not licensed.** Until a licence is granted we would operate as a **programme on
your licence**: you hold the deposits and carry the regulatory relationship; MYFORGE builds and runs the customer experience. We do not use "bank" in our
public name and will not until the Prudential Authority authorises it.

**Who we serve.** Technology businesses and their founders: companies that need rand and foreign-currency accounts, payments, FX and treasury, through
software (`BANK_MODEL.md`). The wider product set (savings, credit, card acceptance, cards) is built but **switched off**: each would be a separate
decision with you and with the regulator, and none is assumed in this approach.

**What we bring you.**

1. **A working platform, tested only against simulators.** Accounts, payments, reconciliation, multi-currency, FX and the workspace run end to end on a
   local stack, and the software gate passes. **Every outside party is simulated**: no sponsor bank, no payment rail, no live identity provider and no
   real customer has been connected. There is no production hosting. The code is real; the integrations are not yet proven.
2. **Controls you can inspect.** Every balance is reconciled to bank statements; payments are held while a reconciliation break is open; limits and monitoring
   rules are data owned by named people; every decision goes to an append-only audit log; customers are re-screened daily and a match opens a critical
   case and freezes the customer. **The screening list is a development fixture of one name.** The sanctions-list source and licence are undecided
   (`docs/SANCTIONS_SOURCES.md`), so this control is built but not live-grade.
3. **A governance pack with an honest status:** structure, 37 seats, 58 draft policies and a 114-line regulatory register. **None of the policies is
   approved and 36 of the 37 seats are vacant.** We expect you to find gaps; we would rather you find them than a regulator.
4. **A way out.** `BankLayer` isolates every route-specific detail so that moving to our own licence later is a migration, not a rebuild (**the
   migration runbook is not yet written**).

**What we ask of you.** A sponsor sandbox and wire formats; a programme agreement; your onboarding standards; and a view on which products you would
carry. Start with the core: ZAR and USD accounts, payments and FX.

**What we are not.** Licensed, funded, or staffed beyond the founder. We have no capital raised, no board, no approved policy, no signed provider, no
penetration test (SF-001), no production environment and no customers. Every external connection is a simulator today. Say all of this first; they will
ask, and each point is also in the readiness table below.

### Things to disclose up front (so they do not look hidden)

| Topic | What is true | Why a sponsor cares |
|---|---|---|
| Third-party offers | A layer can introduce customers to investment and crypto-asset products that a licensed third party provides. It is **off by default and off in production** until a named decision approves it, no provider is chosen, and every offer states it is provided, licensed and monitored by the third party, not by Forge or you. | Conduct and reputational risk to the programme; holding-out risk; FAIS and CASP licensing of the provider |
| Referral fees | Forge may receive a referral fee from a provider, set per offer and stated to the customer in the statement they acknowledge; it is booked only against a bank receipt, once, within the stated cap, by two people. No fee is earned today. | Conflict of interest and disclosure; where the money lands (it would reach Forge through you) |
| Customer names to providers | If a customer consents, **their name only** is sent to the provider they chose, signed and over https, and never anything else; withdrawing stops anything unsent but cannot recall what was sent. | POPIA, your own data-sharing and outsourcing rules, whether your customer data may be shared at all |
| Sign-in and identity providers | WorkOS (sign-in, likely US processing) and Didit (identity checks, location unconfirmed) are the chosen providers. **Neither account exists yet and nothing has run against them**; the apps use development sign-in only. | Outsourcing, cross-border data, and whether you will allow our providers under your policy |
| Sanctions list | A one-name fixture today; a licensed source is undecided. | Your regulatory responsibility for screening under your licence |
| The contracting company | MYFORGE (Pty) Ltd, whose founder will also found the proposed mutual bank; MYFORGE would be the bank's main technology supplier. | Related-party and concentration risk, and who stands behind the contract |

### Positions we hold (confirm with the lawyer before the first meeting)

These protect the plan to apply for our own licence. A sponsor will not always accept all of them; we should know which we can give up and which we cannot.

1. **No exclusivity or non-compete that stops us applying for, or operating under, our own licence.** A clause that treats Forge as a competitor if it
   becomes a bank would block the application and is **not acceptable**. Accept a reasonable duty not to solicit your customers by improper means.
2. **The contract must be assignable (novated) to the future bank, or to a successor MYFORGE nominates, on notice and the Prudential Authority's approval**,
   without the sponsor being able to refuse or re-price it as a result.
3. **A defined exit:** return of data in an agreed format, orderly migration of customers to our own books, a transition period at the current price, and
   no termination by you on account of our application. Match this with the contract schedule in `docs/regulatory/policies/provider-contract-schedule.md`
   (incident notice within 4 hours, exit assistance, no termination on recovery or resolution).
4. **MYFORGE signs, not a pretend bank.** The agreement says in terms that MYFORGE is not licensed and does not hold customer money.
5. **Customers are portable.** On exit, customers can choose to move; we do not lock them in, and we do not take their money.
6. **No promise on capital or timing.** We do not give guarantees about when or whether we will be registered.
7. **Independent legal advice.** The lawyer who advises us does not act for you (`LAWYER_BRIEF.md`).

### Questions to ask them

| Topic | Question |
|---|---|
| Model | Do you offer a programme or banking-as-a-service arrangement for a pre-licence fintech? Are customers yours or ours, in the deposits and in the contract? |
| Contracting party | You would contract with MYFORGE (Pty) Ltd, an unlicensed technology company. What do you need from it and its founder: guarantees, capital, insurance, a director's undertaking? |
| Assignment | Will you agree that the contract can be novated to our own bank, or a successor, when and if we are registered? On what conditions? |
| Exclusivity | Is exclusivity or a non-compete a condition? We cannot agree to anything that stops us applying for our own licence. What is the narrowest restriction you need? |
| Eligibility | What do you need to see before a first meeting: capital, directors fit and proper, policies, a security review? We have none of these yet. |
| Controls | Whose KYC, sanctions screening and transaction monitoring count? May we use our own provider (Didit) under your policy? Which sanctions list do you require, and may we use yours? Who files STRs with the FIC? |
| Payments | EFT, RTC and PayShap access: through you, in what formats (VERIFY-SPONSOR), with what cut-offs and fees? |
| Foreign currency | Do you provide an authorised-dealer arrangement and SARB reporting, or do we need a separate one? |
| Deposits | Pooled account or sub-accounts per customer? How is deposit insurance treated? What is the customer-money protection if MYFORGE fails? |
| Credit | We are not registered as a credit provider and have no balance sheet. **Which, if any, credit products would you carry on your own licence and balance sheet, and which can we not offer at all until we are registered?** (We do not assume any.) |
| Third-party offers | May we show customers products from licensed third parties, under what disclosure, and may we receive a referral fee that reaches us through you? Or do you prohibit it? |
| Data and providers | May customer data flow to WorkOS (sign-in, likely US) and Didit (identity)? To a product provider with the customer's consent? Which of our providers do you treat as material outsourcing? |
| Economics | Fees, revenue share, minimum volumes, term, exclusivity, exit and notice. |
| Oversight | Audit rights, reporting we owe you, incident notice times (we propose 4 hours; `provider-contract-schedule.md`), your regulator notices. |
| Exit | Data return, customer migration to our own licence, transition price and period. We will not accept a non-compete. |

### What they will want from us, and where it is

| They will ask for | Where it is | Ready? |
|---|---|---|
| Business plan and projections | `docs/licence-application/BUSINESS_PLAN_DRAFT.md` (outline: `docs/regulatory/policies/business-plan-outline.md`) | Draft plan with **no real projections**; **do not share `docs/regulatory/proforma/` as a forecast** (illustrative, and it disagrees with the `CONTEXT.md` economics) |
| Legal form and constitution | `docs/licence-application/CONSTITUTION_DRAFT.md`, `LEGAL_FORMS.md` | Draft for the lawyer; ADR-0020 open; one founder named |
| Directors and key people | `docs/governance/ROLES.md`, `FIT_AND_PROPER_PACK.md` | Founder only; MLRO and risk and compliance heads **vacant** |
| Policies (AML, risk, outsourcing, security) | `docs/regulatory/policies/` | Drafts, none approved |
| RMCP (FICA risk management and compliance programme) | `docs/regulatory/policies/rmcp.md` | Draft |
| Sanctions screening | `docs/SANCTIONS_SOURCES.md` | Recommendation only; source and licence **not chosen**; screening runs on a one-name fixture |
| Security evidence | `docs/regulatory/security-findings.csv`, CI gate output | **No penetration test yet** (SF-001) |
| Architecture and resilience | `docs/ARCHITECTURE.md`, `business-impact-analysis.md`, `deploy/k8s/` | Documented; **no production hosting** |
| Outsourcing register | `docs/regulatory/outsourcing-register.csv` | Drafted; several providers and locations undecided or unconfirmed |
| Capital | `LEGAL_FORMS.md`, `BUSINESS_PLAN_DRAFT.md` section 9 | **None raised** |

### Process

```mermaid
flowchart LR
  A["Lawyer advises<br/>(LAWYER_BRIEF.md)"] --> B["Shortlist 3 to 5<br/>counterparties"]
  B --> C["Intro meetings:<br/>pitch + questions"]
  C --> D["NDA, share the governance pack<br/>and architecture"]
  D --> E["Their due diligence:<br/>sandbox, security review,<br/>policies, people"]
  E --> F["Term sheet,<br/>then programme agreement<br/>(positions we hold)"]
  F --> G["Provider contract schedule met,<br/>PA notice if material (OUT-02)"]
  G --> H["Pilot with friendly customers"]
```

### Who to approach

Kinds of counterparty, not recommendations; verify each with the lawyer before contact.

- **South African banks that run banking-as-a-service or partner programmes for fintechs**, including the large retail banks and the digital banks.
- **Banks with a business or private-bank focus on technology clients**, the Investec-type relationship (`BANK_MODEL.md`); they may value the clients we would
  bring.
- **Co-operative banks and mutual banks**, where the mutual model is already understood.
- A bank **outside** South Africa is not an option for rand deposits; foreign-currency partners are a separate search (`docs/PROVIDERS.md`).

### Tracking

| Counterparty | Contact | First meeting | NDA | Pack shared | Their questions | Status |
|---|---|---|---|---|---|---|
| | | | | | | |
| | | | | | | |
| | | | | | | |


---

# Part IV. Licence application and regulation


## Licence application pack

*Source: `docs/licence-application/PACK.md`*

## Licence application pack: index and tracker

> Generated from the regulatory register and `sections.csv` (`docs/licence-application/sections.csv`) by `scripts/build-licence-pack.py`. **This is an index of
> what the application needs, not the application.** It says where each document is, who owns it, and what is still missing.
> Nothing in it is approved. Route: mutual bank (planned) or bank (kept open), see `../governance/LEGAL_FORMS.md` (`docs/governance/LEGAL_FORMS.md`).

**18 sections, 59 requirements:** 7 Open, 52 Partial. No requirement is `Evidenced`: nothing is approved and no seat except the founder's is filled.

| Section | Title | Owner | Requirements | Open or external |
|---|---|---|---|---|
| A1 | Legal form and constitution | Head of Legal (R26) with the lawyer | 2 | 0 |
| A2 | Name and naming authorisation | Chief Compliance Officer (R04) | 1 | 1 |
| A3 | Capital and proof of funds | CFO (R03) and the founder | 1 | 1 |
| A4 | Founders and shareholders or members | Company Secretary (R17) | 2 | 0 |
| B1 | Board and executive officers | Board chair (R05) and Nominations Committee | 2 | 1 |
| B2 | Fit-and-proper statements (DI 020 / BA 020) | Company Secretary (R17) | 1 | 0 |
| B3 | Directors' report on internal control | Board (Audit Committee) | 1 | 0 |
| C1 | Business plan and public-interest motivation | Founder and CFO | 1 | 0 |
| C2 | Financial projections and pro forma returns | CFO (R03) and Regulatory Reporting (R22) | 6 | 0 |
| D1 | Risk management framework and policies | Chief Risk Officer (R02) | 4 | 0 |
| D2 | Capital adequacy and liquidity | Head of Treasury (R13) | 4 | 0 |
| D3 | Internal audit | Head of Internal Audit (R11) | 2 | 0 |
| D4 | Business continuity and recovery | Business Continuity Manager (R28) | 4 | 0 |
| E1 | IT systems and material outsourcing submission | CTO (R10) and CISO (R12) | 12 | 2 |
| E2 | Cloud and offshoring | CTO (R10) and Head of Legal (R26) | 5 | 0 |
| E3 | Outsourcing register and contracts | COO (R08) and Head of Legal (R26) | 6 | 1 |
| E4 | Cyber resilience and testing | CISO (R12) | 4 | 0 |
| F1 | Regulator engagement | Founder and CCO | 1 | 1 |

### A1 · Legal form and constitution

**Owner:** Head of Legal (R26) with the lawyer. Mutual bank constitution or company articles; draft in docs/licence-application/CONSTITUTION_DRAFT.md; see docs/governance/LEGAL_FORMS.md

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-01** Bank = public company under Companies Act 71 of 2008; mutual bank = juristic person under Mutual Banks Act, ow | Decision | Partial | CONTEXT.md (`docs/CONTEXT.md`)<br>licence-application-plan.md (`docs/regulatory/policies/licence-application-plan.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`)<br>legalform.go (`pkg/banklayer/legalform.go`)<br>LEGAL_FORMS.md (`docs/governance/LEGAL_FORMS.md`)<br>CONSTITUTION_DRAFT.md (`docs/licence-application/CONSTITUTION_DRAFT.md`) | Owner's direction 2026-11-18: mutual bank planned, bank kept open; both encoded as data. Draft constitution for the lawyer written 2026-11-24 (not legal advice; Act provisions marked ACT). Legal advice needed before ADR-0020 closes |
| **LIC-07** At least seven founders with signed articles; depositor-members vote at general meetings; mutuality concept st | Document | Partial | ARCHITECTURE.md (`docs/ARCHITECTURE.md`)<br>shareholder-and-significant-ownership-policy.md (`docs/regulatory/policies/shareholder-and-significant-ownership-policy.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`)<br>membership (`services/bank/membership/internal/membership`)<br>CONSTITUTION_DRAFT.md (`docs/licence-application/CONSTITUTION_DRAFT.md`) | Member register, shares, one-member-one-vote and the seven-founder check built and tested; draft constitution for the lawyer written (20 decisions listed, Act provisions to be added); only one founder named; module not wired to a service or |

### A2 · Name and naming authorisation

**Owner:** Chief Compliance Officer (R04). No 'bank' or 'mutual bank' in public naming until authorised

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-17** 'Bank', 'mutual bank' and 'building society' in a business name need PA authorisation | Decision | Open | CONTEXT.md (`docs/CONTEXT.md`)<br>naming-and-marketing-policy.md (`docs/regulatory/policies/naming-and-marketing-policy.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Do not use 'bank' in public naming until authorised |

### A3 · Capital and proof of funds

**Owner:** CFO (R03) and the founder. R10m (mutual) or R250m (bank) with written proof

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-02** Minimum capital R250m (bank) or R10m (mutual bank), with written proof (Banks Act s.70 / Mutual Banks Act s.48 | External | Open | legalform.go (`pkg/banklayer/legalform.go`)<br>prudential (`services/bank/treasury-risk/internal/prudential`) | Minimum per form is checked in the prudential report (INITIAL_CAPITAL breach); no capital raised or evidenced |

### A4 · Founders and shareholders or members

**Owner:** Company Secretary (R17). Register of founders and members; fit and proper for significant holders; membership module checks seven founders

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-05** Significant shareholders fit and proper; >15% acquisition needs permission (Banks Act s.37, s.42); Form BA 125 | Document | Partial | shareholder-and-significant-ownership-policy.md (`docs/regulatory/policies/shareholder-and-significant-ownership-policy.md`)<br>fit-and-proper-procedure.md (`docs/regulatory/policies/fit-and-proper-procedure.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Shareholder register and fit-and-proper files not started |
| **LIC-07** At least seven founders with signed articles; depositor-members vote at general meetings; mutuality concept st | Document | Partial | ARCHITECTURE.md (`docs/ARCHITECTURE.md`)<br>shareholder-and-significant-ownership-policy.md (`docs/regulatory/policies/shareholder-and-significant-ownership-policy.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`)<br>membership (`services/bank/membership/internal/membership`)<br>CONSTITUTION_DRAFT.md (`docs/licence-application/CONSTITUTION_DRAFT.md`) | Member register, shares, one-member-one-vote and the seven-founder check built and tested; draft constitution for the lawyer written (20 decisions listed, Act provisions to be added); only one founder named; module not wired to a service or |

### B1 · Board and executive officers

**Owner:** Board chair (R05) and Nominations Committee. Composition and independence; at least two employee directors on the mutual route

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-06** Board per Act/regulations; directors and executive officers fit and proper (BA 020 / DI 020); accountability,  | Document | Partial | THRESHOLD_OWNERS.md (`docs/THRESHOLD_OWNERS.md`)<br>board-charter.md (`docs/regulatory/policies/board-charter.md`)<br>fit-and-proper-procedure.md (`docs/regulatory/policies/fit-and-proper-procedure.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). No board; BA/DI 020 not started |
| **MR-03** At least two board members must be employees of the bank | Decision | Open | board-charter.md (`docs/regulatory/policies/board-charter.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Governance design not started |

### B2 · Fit-and-proper statements (DI 020 / BA 020)

**Owner:** Company Secretary (R17). See docs/governance/FIT_AND_PROPER_PACK.md; nobody has filed

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **BR-06** Directors' and executive officers' statements (BA 020) and the prescribed forms list | Document | Partial | FIT_AND_PROPER_PACK.md (`docs/governance/FIT_AND_PROPER_PACK.md`) | Filing pack and tracker for every director and executive officer (DI 020 / BA 020); nobody has filed; form content VERIFY |

### B3 · Directors' report on internal control

**Owner:** Board (Audit Committee). Yearly report to the Registrar

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **MR-02** Directors report annually to the Registrar on effectiveness of internal controls over financial and regulatory | Document | Partial | internal-control-over-financial-reporting.md (`docs/regulatory/policies/internal-control-over-financial-reporting.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Not started |

### C1 · Business plan and public-interest motivation

**Owner:** Founder and CFO. Draft in docs/licence-application/BUSINESS_PLAN_DRAFT.md; outline in docs/regulatory/policies/business-plan-outline.md

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-15** Business plan, public-interest motivation, rationale, group structure chart, dividend policy, memorandum (or a | Document | Partial | CONTEXT.md (`docs/CONTEXT.md`)<br>BUSINESS_PLAN_DRAFT.md (`docs/licence-application/BUSINESS_PLAN_DRAFT.md`)<br>CONSTITUTION_DRAFT.md (`docs/licence-application/CONSTITUTION_DRAFT.md`) | Draft business plan, public-interest motivation, group chart and dividend policy written 2026-11-24 from CONTEXT; projections, capital plan, board, auditor and fees are gaps; the illustrative pro forma and CONTEXT economics disagree and mus |

### C2 · Financial projections and pro forma returns

**Owner:** CFO (R03) and Regulatory Reporting (R22). make proforma builds DI/BA 100 and 200 from projections; other forms outstanding; figures are illustrative

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-08** Systems must produce all statutory and prudential information accurately and on time from the first day of ope | Code | Partial | trialbalance.go (`services/bank/treasury-risk/internal/prudential/trialbalance.go`)<br>returns (`services/bank/reg-reporting/internal/returns`) | Trial balance and balance sheet from the ledger with a double-entry check; the returns framework exists; no form builder (line definitions unread) |
| **LIC-11** Pro forma BA 100/110/120/130/300/350/700 (bank) or DI 100/110/200/300/400/430/700 (mutual bank), proposed for  | Code | Partial | returns (`services/bank/reg-reporting/internal/returns`)<br>returns-proforma (`services/bank/reg-reporting/cmd/returns-proforma`)<br>proforma (`docs/regulatory/proforma`) | Pro forma DI 100 and DI 200 on the official line numbers, and BA 100/120 summaries, built from projections (make proforma); projection is illustrative; remaining forms outstanding (docs/regulatory/returns/catalogue.csv) |
| **BR-07** Returns BA 100 through BA 950, each with its own directives and interpretations (BA 200 alone is about 340 pag | Code | Partial | SPEC.md (`services/bank/reg-reporting/SPEC.md`)<br>statements.go (`services/bank/reg-reporting/internal/returns/statements.go`) | BA 100 and BA 120 built from the ledger trial balance, balanced and reconciled; about 17 other BA forms and the mapping to official line numbers outstanding |
| **MR-06** Returns DI 100, 110, 200, 300, 310, 400, 401, 402, 403, 410, 420, 430, 500-520, 600, 700-702 plus Chapter V re | Code | Partial | SPEC.md (`services/bank/reg-reporting/SPEC.md`)<br>di.go (`services/bank/reg-reporting/internal/returns/di.go`)<br>dicontrol.go (`services/bank/reg-reporting/internal/returns/dicontrol.go`)<br>catalogue.csv (`docs/regulatory/returns/catalogue.csv`)<br>SOURCES.md (`docs/regulatory/returns/SOURCES.md`) | Official DI forms obtained from the PA (proposed directive, 13 Aug 2025). DI 100, DI 200, DI 110, DI 300, DI 310, DI 400, DI 410, DI 500, DI 505, DI 510, DI 600, DI 700, DI 900 and DI 930 built on the forms' own line numbers (transcription  |
| **RET-01** Certification and rendition of returns, auditor reports and reconciliation of selected returns | Code | Partial | SPEC.md (`services/bank/reg-reporting/SPEC.md`)<br>returns.go (`services/bank/reg-reporting/internal/returns/returns.go`) | Preparer, different approver, evidence digest and auditor report enforced in code; the PA certification form itself not built |
| **MR-05** Auditor reports on about 20 DI forms; DI 900 must reconcile to DI 100, auditor reports within 90 days of year  | Code | Partial | returns.go (`services/bank/reg-reporting/internal/returns/returns.go`) | Audited returns cannot be submitted without an independent auditor's report recorded; which forms need one, and the report wording, VERIFY |

### D1 · Risk management framework and policies

**Owner:** Chief Risk Officer (R02). Risk framework and appetite drafted; appetite numbers await the owner

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-13** Risk-by-risk policy and quantified effect for solvency, liquidity, counterparty, interest-rate, market, credit | Document | Partial | risk-management-framework.md (`docs/regulatory/policies/risk-management-framework.md`)<br>risk-appetite-statement.md (`docs/regulatory/policies/risk-appetite-statement.md`)<br>credit-risk-policy.md (`docs/regulatory/policies/credit-risk-policy.md`)<br>market-and-interest-rate-risk-policy.md (`docs/regulatory/policies/market-and-interest-rate-risk-policy.md`)<br>operational-risk-policy.md (`docs/regulatory/policies/operational-risk-policy.md`)<br>capital-management-policy.md (`docs/regulatory/policies/capital-management-policy.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). No risk policies written |
| **BR-01** Board-owned risk management covering the listed risk types: capital, compliance, concentration, counterparty,  | Document | Partial | SPEC.md (`services/bank/treasury-risk/SPEC.md`) | Only a few risks have any code; need a risk register with policy and owner per risk |
| **BR-02** Board-approved policies to identify, measure, monitor, control, price, mitigate and report each risk | Document | Partial | risk-management-framework.md (`docs/regulatory/policies/risk-management-framework.md`)<br>committee-terms-of-reference.md (`docs/regulatory/policies/committee-terms-of-reference.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). No board-approved policies |
| **MR-01** Risks to manage: solvency, liquidity, credit, currency, market, interest-rate, counterparty, technological, op | Document | Partial | risk-management-framework.md (`docs/regulatory/policies/risk-management-framework.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Version is the original 1993 gazette; confirm current text with the PA |

### D2 · Capital adequacy and liquidity

**Owner:** Head of Treasury (R13). Capital and leverage ratios and the liquidity coverage ratio are computed from the ledger (treasury-risk); concentration policy and liquidity plan drafted

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-03** Total capital ratio >= 11.5%, leverage ratio >= 4% (Directive 6 of 2017); higher ratio possible for new banks | Code | Partial | prudential (`services/bank/treasury-risk/internal/prudential`) | Capital adequacy and leverage computed from the ledger daily (back office Risk); weights and minimums PROPOSED (thresholds 22, 23); no capital raised, so both ratios show in breach |
| **LIC-04** Prudent funding profile, buffer of high-quality liquid assets, liquidity risk framework and stress testing | Code | Partial | prudential (`services/bank/treasury-risk/internal/prudential`)<br>liquidity-and-contingency-funding-plan.md (`docs/regulatory/policies/liquidity-and-contingency-funding-plan.md`) | 30-day LCR and five stress scenarios computed from the ledger; draft liquidity plan; no committed funding line; undrawn limits and notices not yet read |
| **LIC-12** Policy on maximum deposit from a single depositor relative to total deposits | Document | Partial | SPEC.md (`services/bank/treasury-risk/SPEC.md`)<br>concentration-policy.md (`docs/regulatory/policies/concentration-policy.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Concentration alerts exist but no board-approved policy |
| **BR-04** Liquidity stress testing, contingency funding plans, liquid-asset cushion | Document | Partial | prudential (`services/bank/treasury-risk/internal/prudential`)<br>liquidity-and-contingency-funding-plan.md (`docs/regulatory/policies/liquidity-and-contingency-funding-plan.md`) | Stress testing and a draft contingency funding plan exist; run-off rates PROPOSED; plan not approved |

### D3 · Internal audit

**Owner:** Head of Internal Audit (R11). Charter drafted; 12 months of findings need a running audit function

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **BR-05** Internal audit is a permanent function; outsourcing it needs prior written Registrar approval; scope includes  | Document | Partial | SPEC.md (`services/shared/audit-service/SPEC.md`)<br>internal-audit-charter.md (`docs/regulatory/policies/internal-audit-charter.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Audit log is a control, not an internal audit function |
| **LIC-14** Last 12 months of internal-audit findings on deposit book, advances book and large exposures; any non-complian | Document | Partial | internal-audit-charter.md (`docs/regulatory/policies/internal-audit-charter.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Nothing to report yet; needs an internal audit function |

### D4 · Business continuity and recovery

**Owner:** Business Continuity Manager (R28). Business impact analysis and recovery plan drafted; no tested DR site

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-09** Contingency plans so business continues after a disaster | Document | Partial | pitr-check.sh (`scripts/pitr-check.sh`) | PITR proven locally; no production DR plan or drill |
| **D3-06** Contingency plans; IP and data rights protected; data usable, readable and portable after termination | Document | Partial | exit-plans.md (`docs/regulatory/policies/exit-plans.md`) | Draft exit plan per provider; no provider contracts to hold exit terms |
| **JS1-11** Recovery priorities, RTO and RPO set; business impact assessment covering third-party dependencies | Document | Partial | recovery-plan.md (`docs/regulatory/policies/recovery-plan.md`) | Proposed values only; board must set |
| **JS2-13** Set RPO and RTO from nature, scale, complexity and risk | Document | Partial | recovery-plan.md (`docs/regulatory/policies/recovery-plan.md`) | Draft with proposed RPO and RTO; board must set |

### E1 · IT systems and material outsourcing submission

**Owner:** CTO (R10) and CISO (R12). Assembled from the IT policies and docs/ARCHITECTURE.md

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-10** Detailed discussion of IT systems, material outsourcing and third-party supplier operational risk, with monito | Document | Partial | ARCHITECTURE.md (`docs/ARCHITECTURE.md`)<br>it-risk-management-framework.md (`docs/regulatory/policies/it-risk-management-framework.md`)<br>outsourcing-policy.md (`docs/regulatory/policies/outsourcing-policy.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). No written IT and outsourcing submission; see OUT-* rows |
| **JS1-01** Governing body ultimately responsible; approves an IT strategy aligned to business strategy, reviewed at least | Document | Partial | it-strategy.md (`docs/regulatory/policies/it-strategy.md`) | Draft IT strategy; no board to approve it |
| **JS1-02** IT risk management framework approved by governing body, reviewed annually; material IT risks identified, asse | Document | Partial | risk-register.md (`docs/regulatory/policies/risk-register.md`) | Starter register exists; technology risk not rated; framework not approved |
| **JS1-03** Dedicated IT risk function accountable to the governing body, led by a suitably skilled senior manager | Decision | Open | it-risk-management-framework.md (`docs/regulatory/policies/it-risk-management-framework.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Needs a hire or appointment |
| **JS1-05** Risk register maintained; IT risk metrics and profile reported to governing body | Document | Partial | risk-register.md (`docs/regulatory/policies/risk-register.md`) | No metrics reporting; no board |
| **JS1-06** Staff, providers and contractors screened, fit and proper, bound to confidentiality; training reviewed at leas | Document | Partial | staff-screening.md (`docs/regulatory/policies/staff-screening.md`)<br>training-plan.md (`docs/regulatory/policies/training-plan.md`) | Draft screening procedure and training plan; no staff yet |
| **JS1-10** IT programme and project management framework; security requirements approved independently of development; re | Document | Partial | it-project-framework.md (`docs/regulatory/policies/it-project-framework.md`) | Draft framework; no independent security approver named |
| **JS1-12** Geographically separate DR site; IT resilience plan tested at least annually including total primary-site loss | Code | Open | business-impact-analysis.md (`docs/regulatory/policies/business-impact-analysis.md`) | No production hosting chosen (ADR-0002); nothing built |
| **JS2-01** Board accountable; cyber function with direct line to board; board-approved strategy reviewed at least annuall | Document | Partial | cyber-strategy.md (`docs/regulatory/policies/cyber-strategy.md`) | Draft strategy written; not board-approved |
| **JS2-09** Cryptographic key management: generation, distribution, renewal, revocation, destruction, tamper-resistant pro | Document | Partial | key-management-policy.md (`docs/regulatory/policies/key-management-policy.md`) | Draft policy written; not approved; KMS or HSM undecided |
| **JS2-10** Cyber awareness training at least annually; board training | Document | Partial | training-plan.md (`docs/regulatory/policies/training-plan.md`) | Draft plan only; no training delivered; not approved |
| **JS2-14** Cyber incident response and management plan, tested against plausible threats; customer communication strategy | Document | Partial | incident-response-plan.md (`docs/regulatory/policies/incident-response-plan.md`) | Draft plan written; never tested |

### E2 · Cloud and offshoring

**Owner:** CTO (R10) and Head of Legal (R26). Hosting direction in docs/HOSTING.md; PA notice 20 working days before production

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **D3-01** Board-approved data strategy and data governance framework | Document | Partial | ARCHITECTURE.md (`docs/ARCHITECTURE.md`)<br>data-strategy.md (`docs/regulatory/policies/data-strategy.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Architecture states SA data residency; no board-approved strategy |
| **D3-02** Cloud/offshoring policy aligned to strategy and risk appetite; oversight inside governance structures | Document | Partial | ARCHITECTURE.md (`docs/ARCHITECTURE.md`)<br>cloud-and-offshoring-policy.md (`docs/regulatory/policies/cloud-and-offshoring-policy.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). ADR-0002 (hosting) open |
| **D3-03** Risk-appetite check and due diligence before each cloud or offshoring initiative | Document | Partial | PROVIDERS.md (`docs/PROVIDERS.md`)<br>cloud-and-offshoring-policy.md (`docs/regulatory/policies/cloud-and-offshoring-policy.md`)<br>provider-due-diligence-procedure.md (`docs/regulatory/policies/provider-due-diligence-procedure.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Not done for WorkOS, Didit or any host; check where each processes data |
| **D3-05** No obstruction of regulator access or forensic audits by cloud or offshoring arrangements | Document | Partial | cloud-and-offshoring-policy.md (`docs/regulatory/policies/cloud-and-offshoring-policy.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Needs contract terms with every provider |
| **D3-07** Every arrangement in a documented, legally binding agreement | External | Partial | provider-contract-schedule.md (`docs/regulatory/policies/provider-contract-schedule.md`) | 16 minimum clauses defined and enforced through the outsourcing register; no contracts signed |

### E3 · Outsourcing register and contracts

**Owner:** COO (R08) and Head of Legal (R26). docs/regulatory/outsourcing-register.csv computes notice dates

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **OUT-01** Core banking IT and financial-reporting IT are material functions; outsourcing needs prior written approval | Decision | Open | ARCHITECTURE.md (`docs/ARCHITECTURE.md`) | Hosting choice (ADR-0002) decides whether this is triggered |
| **OUT-02** Notify the PA at least 20 working days before material outsourcing, and before offshoring | External | Partial | outsourcing-register.csv (`docs/regulatory/outsourcing-register.csv`)<br>check-outsourcing.py (`scripts/check-outsourcing.py`) | Register computes the latest PA notice date (20 working days, SA holidays) per material provider and the gate fails if one is missed or a provider is used without notice; no start dates yet |
| **OUT-03** Board-approved outsourcing policy, plan and risk assessment, due diligence, binding contract, monitoring, cont | Document | Partial | outsourcing-policy.md (`docs/regulatory/policies/outsourcing-policy.md`)<br>provider-due-diligence-procedure.md (`docs/regulatory/policies/provider-due-diligence-procedure.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Not started |
| **OUT-04** Materiality assessment of every outsourcing arrangement (eleven factors) | Document | Partial | outsourcing-register.csv (`docs/regulatory/outsourcing-register.csv`) | Register of providers with materiality; the eleven-factor assessment per provider is not done |
| **OUT-05** Recovery plan must cover outsourced material activities | Document | Partial | business-continuity-and-disaster-recovery-policy.md (`docs/regulatory/policies/business-continuity-and-disaster-recovery-policy.md`)<br>recovery-plan.md (`docs/regulatory/policies/recovery-plan.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). No recovery plan |
| **OUT-06** Contract minimum topics (about 30), no provider right to cancel on resolution, regulator and auditor access | External | Partial | provider-contract-schedule.md (`docs/regulatory/policies/provider-contract-schedule.md`) | Minimum clauses drafted from the guidance as read; verify against the full text |

### E4 · Cyber resilience and testing

**Owner:** CISO (R12). No penetration test yet; findings in docs/regulatory/security-findings.csv

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **G4-01** Assess policies against the CPMI-IOSCO cyber-resilience guidance (governance, identification, protection, dete | Document | Partial | threat-intelligence-and-security-testing-policy.md (`docs/regulatory/policies/threat-intelligence-and-security-testing-policy.md`)<br>STRUCTURE.md (`docs/governance/STRUCTURE.md`) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Self-assessment not done |
| **G4-02** Recovery time objectives from a business impact assessment; penetration testing by reputable, accredited exter | External | Partial | business-impact-analysis.md (`docs/regulatory/policies/business-impact-analysis.md`) | Business impact analysis with RTO and RPO per service; no penetration test yet (SF-001) |
| **JS2-15** Threat-intelligence process and participation in information sharing | External | Partial | threat-intelligence-and-security-testing-policy.md (`docs/regulatory/policies/threat-intelligence-and-security-testing-policy.md`) | Process drafted; no feed or sharing group joined yet (SF-004) |
| **JS2-16** Test all controls; vulnerability assessments; penetration test of internet-facing critical systems annually or | External | Partial | security-findings.csv (`docs/regulatory/security-findings.csv`)<br>check-security-findings.py (`scripts/check-security-findings.py`) | Remediation register with severity deadlines checked by the gate; no penetration test or simulation yet (SF-001) |

### F1 · Regulator engagement

**Owner:** Founder and CCO. Pre-application meeting with the PA; FSCA concurrence

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-16** Optional pre-application meeting; FSCA concurrence before any licence (FSRA s.126); registration within 12 mon | External | Open | *none* | No PA contact yet |

### What no document can close

- **Capital** (A3) and **founding members** (A4): money and people.
- **Fit-and-proper** (B2): each person files, and the regulator assesses.
- **Pre-application meeting** (F1) and every approval: the Prudential Authority and FSCA act.
- **Approval of every policy**: a board that does not yet exist.



## Regulatory return forms and sources

*Source: `docs/regulatory/returns/SOURCES.md`*

## Regulatory return forms: where they come from

Read 2026-11-24. `scripts/fetch-return-forms.sh` downloads every file below and fails if its SHA-256 has changed, which
means the Prudential Authority (PA) has published a new version and the line mapping must be re-read.

### What changed in the law

- **Banks (BA returns).** The Minister removed every BA return and its completion instructions from the Regulations relating to
  Banks with effect from 1 February 2025. The PA now prescribes them in **Directive 1 of 2025** (ref. 15/8/1/3, dated 29 January 2025),
  with each form and its instructions as an annexure (50 enclosures: Annexure 1A is the list of returns, frequencies and deadlines).
- **Mutual banks (DI returns).** Mutual Banks **Directive 1 of 2025** (ref. 15/8/2/3, dated 13 August 2025) gives notice that every DI
  return and its instructions are being removed from the Regulations relating to Mutual Banks and will be issued as PA directives. The PA
  then published a **proposed directive** (dated 13 August 2025, comments by 26 September 2025, proposed start 1 December 2025) carrying
  every DI form and instruction as annexures. Paragraph 1.5.3 of the directive says the forms themselves are not changed by the move.
  A law-firm note and the consolidated regulations indicate the deletion took effect through GN 7414 of 29 April 2026; **whether the
  proposed directive has been issued in final form was not confirmed** (VERIFY with the lawyer or the PA).
- The DI forms in the proposed directive are scans of the Government Gazette pages (DI 100: Gazette 22738, 5 October 2001; DI 200:
  Gazette 15381, 28 December 1993). The line numbers and labels in `services/bank/reg-reporting/internal/returns/forms/` were transcribed
  from those scans by reading them, not by optical character recognition. **A second person must check the transcription against the
  scans** before any return is filed.

### Files

| File | Publisher, date | SHA-256 |
|---|---|---|
| [Annexures to the proposed directive (DI forms)](https://www.resbank.co.za/content/dam/sarb/publications/prudential-authority/pa-documents-issued-for-consultation/2025/proposed-directive-for-mutual-banks-on-di-returns/Annexures%20to%20the%20proposed%20directive.zip) | PA, 2025 | `146d62dab990ad45c449bdde3d9f90ffff4c48efcf249130d41504c9944e82b5` |
| [Proposed Directive for Mutual banks on DI Returns](https://www.resbank.co.za/en/home/publications/publication-detail-pages/prudential-authority/pa-documents-issued-for-consultation/2025/Proposed-Directive-for-Mutual-banks-on-DI-Returns) | PA, 13 August 2025 | `e9ddb08cde133c2eb2ab5a1fbe64959cb13789015c974a1cb985db870fb52b70` |
| [Mutual Banks Directive 1 of 2025](https://www.resbank.co.za/en/home/publications/publication-detail-pages/prudential-authority/pa-deposit-takers/Mutual-Banks-Directive/2025/D1-2025-Mutual-banks) | PA, 13 August 2025 | `8a2513bbdfbe2ae4635ebfd9f02989a430f28079fed6ad4c0d293af37a9ec821` |
| [Banks Directive 1 of 2025](https://www.resbank.co.za/en/home/publications/publication-detail-pages/prudential-authority/pa-deposit-takers/banks-directives/2025/Directive-returns-to-be-submitted-to-the-PA) | PA, 29 January 2025 | `fe671a0e3c3211cc6e6a48481429e79f5d4fc9bde9b88dc49f781102de16a081` |
| Banks Directive 1 of 2025, Returns - Annexures.zip (same page) | PA, 2025 | `58e1c9e03c390a6e6ff425012a9befd84b9e74b7be59a7226ae2f14c868a7231` |

Also published and not yet used: the PA's [Banks Returns (Umoja)](https://www.resbank.co.za/en/home/what-we-do/Prudentialregulation/banks-returns-umoja)
page, with per-form instruction bundles and a submission schedule, and a proposed directive for BA returns from 1 July 2026.

### Catalogue

`catalogue.csv` (`docs/regulatory/returns/catalogue.csv`) lists all 27 DI forms and 27 BA forms with the title, frequency, deadline and source annexure, and how Forge
treats each: **built**, **to-build**, **nil** (the form applies but Forge has nothing to report, and the code checks that), or
**not-applicable** (the form is marked N/A for a mutual bank in the Republic, or Forge has no foreign operations).


## Regulatory returns built

*Source: `services/bank/reg-reporting/SPEC.md`*

## reg-reporting (services/bank/reg-reporting)

Entity: BANK. Architecture: ARCHITECTURE.md 15.2. v1 scope: goAML reports and evidence packs.

**Status: the evidence pack is built; goAML submission is deliberately refused.** There is no binary and no
ConnectRPC surface yet.

### Evidence packs (`internal/evidence`)

Each regulatory run produces one pack: the run and report, the period, the code and policy versions, every input with
its digest and row count, the queries used, and who prepared and approved it.

- **Stable digest.** The pack's JSON is canonical and inputs are sorted by name, so the same inputs always give the
  same digest. Changing any byte, including a count or a query, changes it.
- **Maker-checker.** The preparer cannot approve the pack. A pack without approval is refused when approval is required,
  and allowed as a draft otherwise.
- **Verification.** `Verify` checks stored bytes against an expected digest before it decodes them, and refuses unknown
  fields.
- **Storage.** The architecture asks for WORM storage (VERIFY-WORM). This package produces the bytes and the digest;
  it does not store them. Storage is not built.

### goAML (`internal/goaml`)

`Submit` always returns `ErrSchemaUnverified`. The official goAML schema and the submission process are unverified
(VERIFY-GOAML), so no report can reach the regulator from this code. This is intentional. A report that cannot be
validated must not be sent. A test asserts the refusal on every payload.

### Incident notification (`internal/incident`)

Joint Notice 2 of 2026. Models an IT or cyber incident from detection through classification, the 24-hour immediate notice,
the 14-day update and the full report, with deadline tracking (`Obligations`) and a CSV export of the Annexure A tabs. The
institution classifies materiality; the 24 clock hours run from that decision. Notify needs complete tabs and an approver who
differs from the preparer. It does not submit to the PA's Umoja portal (VERIFY-UMOJA); a person does. The 14 days are counted
as 14x24 hours from notification (confirm with the PA).

### Prudential returns (`internal/returns`)

The frame: line computation from named inputs, form arithmetic checks, reconciliation to other sources, preparer plus a different
approver, tied to an evidence digest. `Submittable` returns `ErrUnverified` until every line has a named checker. Nothing is sent to
the PA.

**The official forms.** The PA moved every return out of the regulations into directives: BA returns in Banks Directive 1 of 2025, DI
returns in a proposed mutual-bank directive of 13 August 2025 whose annexures carry every DI form and instruction. Sources, hashes and
how to re-fetch them: `docs/regulatory/returns/SOURCES.md` and `scripts/fetch-return-forms.sh`. `docs/regulatory/returns/catalogue.csv`
lists all 27 DI and 27 BA forms with frequency, deadline and how Forge treats each; a test keeps it and the code in step.

- **DI 100 and DI 200** (`di.go`) use the forms' own line numbers (76 and 66 lines, in `forms/*.csv`, transcribed from the Gazette
  scans by reading them; a second person must check them). Each total follows the form's own arithmetic. Ledger positions (asset,
  code, product, net in rand) are placed on leaf lines by `mapDI100` and `mapDI200`; anything without a place is a problem, never
  dropped. DI 100 must balance (line 76 = line 34) and its line 32 must equal DI 200 line 66 (`CrossCheck`). Only the total columns are
  computed; the maturity split, averages, yields, risk-weight columns and cost of funds are not (`UnbuiltColumns`). Judgements in the
  mapping (balances with the sponsor as interbank placements, FX fees as foreign-exchange earnings, DI 200 item 45 as 46 + 49 + 50 so
  items 47 and 48 are not counted twice, the signs of items 58 to 63) are stated in the code and need the auditor's view.
- **DI 400** (`di400.go`), capital adequacy: Division I weights the quarter's average DI 100 assets and undrawn facilities by the J2
  table (judgements: banks 20% if South African or OECD, mortgages 100%); Division II excludes profit not appropriated by formal board
  resolution, as the form's footnote requires; secondary capital is capped at primary; line 44 is the risk-asset ratio.
- **DI 310** (`di310.go`), minimum reserve balance and liquid assets: only the instruments the Act names count, so a sponsor balance is
  not a liquid asset and the platform as built reports a shortfall. `Shortfalls` reports capital, liquid-asset and reserve shortfalls,
  and the DI 099 declaration refuses to go out unqualified while any exists.
- **Nil returns** (`dicontrol.go`): DI 402, 420, 430, 520, 525, 701, 702, 910 and 920 are filed as nil, and only when every ledger position
  maps to a known DI 100 line (an unknown code could be exactly what the nil denies). DI 525 also needs a named person, because the
  ledger cannot show that no borrower is a local government.
- **Not applicable:** DI 099A, 401 and 403 (no off-shore or foreign operations, no subsidiaries).
- **DI 099 control sheet:** two different officers (chief executive and chief accounting officer), only clean returns for the period,
  the form-specific paragraphs only for forms in the set, and every qualification stated.
- **DI 110** (`di110.go`) feeds DI 400 line 2: undrawn facilities and the open FX position. **DI 600** (`di600.go`) is in US cents, on the
  1993 currency columns (euro under "other"); the aggregate open position is the larger of longs and shorts, limited to 10% of capital.
- **DI 300** (`di300.go`) ladders the DI 100 lines by contractual maturity: deposits by notice period, loans from the lending schedule
  (which must add up to DI 100), the rest "other"; line 19 (the ALCO behavioural model) is not built.
- **DI 930** (`di930.go`): the predominant rate per category (most accounts, ties to the larger balance) with its effective date.
- **DI 510 and DI 505** (`di510.go`): exposures per legal entity above 10% of capital, and the check that a transaction crossed the
  prescribed percentage. Balances with the sponsor and partner banks are exposures to those banks.
- **DI 410** (`di410.go`): repricing gap and the 12-month income effect of a 1% move by the standard gap method.
- **DI 700** (`di700.go`): the section 55 and 56 limits; a breach is a shortfall that qualifies DI 099.
- **DI 500** (`di500.go`): overdue loans and their ageing, assets by profitability (the instructions' rule), loans by sector.
- **DI 900** (`di900.go`): 229 lines by institutional sector, reconciled to DI 100 (line 80 = line 223); only line totals are computed.
- **All 27 DI forms are covered.** Still open for every form: a second person's check of the transcription, and the data feeds
  (report builders) that read the services into these inputs. BA returns stay on this repository's own
  references (`statements.go`) while the bank route is only kept open.

### Not started

- Balance of Payments data handed to the AD partner (VERIFY the division of responsibility).
- The 13 DI forms still to build, the BA forms if the bank route is chosen, and the submission channel (the PA's Umoja portal).
- The report builders that read compliance cases and ledger balances into a pack.
- Any binary, schedule or ConnectRPC API.

### Tests

`make test-reg-reporting`.


---

# Part V. Technology and operations


## Architecture

*Source: `docs/ARCHITECTURE.md`*

## FORGE: Technical Architecture

Version 1.1 | 27 October 2026 | Status: concept, Phase 0

**v1.1 changes:** ADR-0022 decided (technology companies first, so a USD account via a partner bank is in v1). ADR-0020 handled by building both routes behind a `BankLayer` interface: sponsor-led now, own licence (mutual or bank) later, with own-licence and membership modules designed but not built. See sections 1, 5.2 to 5.5, 12.3, 17.4 and 25.
Place at `docs/ARCHITECTURE.md`. Companion files: `AGENTS.md` (v2.1, repository rules), `docs/adr/`, `docs/specs/<service>/SPEC.md`.

> **Precedence.** `AGENTS.md` governs scope and rules of work. This file governs technical design. Both supersede the technical sections of `BUILD_PLAN.md` v1.1. If any two conflict, stop and record it in `docs/QUESTIONS.md`.
>
> **VERIFY.** Anything marked VERIFY (law, licence terms, vendor behaviour, library versions, file formats) must be confirmed by a human before it is relied on. Coding agents must not invent these facts.

---

### Contents

0. How to use this document with Claude Code
1. Context: what is being built and why
2. Architectural principles and invariants
3. System context
4. Logical architecture
5. Legal-entity boundaries in code
6. Service catalogue
7. Ledger design (TigerBeetle)
8. Core banking (Fineract) and core-bridge
9. Workflows and sagas (Temporal)
10. Policy as code (OPA)
11. Identity, onboarding, party and consent
12. Payments, rails and FX
13. Reconciliation
14. Intercompany settlement
15. Compliance and regulatory reporting
16. Digital-asset entity (deferred)
17. MFI platform (year 2)
18. Data architecture and events
19. APIs
20. Security architecture
21. Infrastructure, environments and operations
22. Observability
23. Testing strategy and quality gates
24. Repository layout
25. Build sequence for Claude Code
26. Architecture decisions (ADRs)
27. VERIFY register
- Appendix A: Open-source component register
- Appendix B: Ledger codes
- Appendix C: Event catalogue
- Appendix D: Glossary

---

### 0. How to use this document with Claude Code

1. Put `AGENTS.md` at the repository root and this file at `docs/ARCHITECTURE.md`. Claude Code reads `AGENTS.md` (or `CLAUDE.md`) automatically; point it to the relevant sections of this file per task.
2. Work one task at a time from section 25. Each task names the sections it needs. Do not paste the whole file into every prompt.
3. Spec first: every service gets `docs/specs/<service>/SPEC.md` before code. The spec is a deliverable that a human reviews.
4. Tests first: ledger invariants as property tests, idempotency tests and failure-path tests before features.
5. CI is the arbiter, not the agent's report. `make ci` must pass.
6. Agents never touch production, real keys, real customer data or real money. Everything runs against simulators, sandboxes and testnets.
7. Money-path changes (anything that moves money, touches keys, or changes permissions) need two human reviews.

**Kickoff prompt (once per session)**

> You are building Forge. Read `AGENTS.md` and `docs/ARCHITECTURE.md` sections 1, 2 and 5. Your task is `<ID>` from section 25. Write or update the service SPEC.md first, then tests, then code. Never invent regulatory, vendor or format facts; record unknowns in `docs/QUESTIONS.md` marked VERIFY. Stop and ask when a task touches an open ADR (section 26).

**Task prompt**

> Task `<ID>`: `<title>`. Read sections `<n>`. Acceptance criteria: `<criteria>`. Deliver code, tests, SPEC.md updates, runbook notes and a PR description with `make ci` output.

**Review prompt (second agent, read-only)**

> Review the PR for task `<ID>` against the invariants in section 2 and the checklist in section 23.4. List defects, missing tests, risky assumptions and unverified regulatory claims. Do not modify code.

---

### 1. Context: what is being built and why

#### 1.1 The product

Forge is a business money platform and bank for technology companies and the people who build them, starting in South Africa. **Technology companies are the first customer group (ADR-0022, accepted):** startups and scale-ups, profitable software houses and agencies, AI and deep-tech firms. The first product is ZAR and USD business accounts with cross-border payments and receipts, in one workspace for accounts, payments, FX, payroll and accounting. Adjacent cross-border SMEs are welcome on the same product to diversify deposits. Later, an MFI network extends reach to underserved borrowers.

#### 1.2 The staged route

| Stage | Bank layer | On-chain | What the code must support |
|---|---|---|---|
| A (now) | Sponsor bank holds deposits; Forge runs a sub-ledger | None in v1 | Sponsor adapter, three-way reconciliation, sub-ledger semantics |
| B (licensed) | Forge Mutual Bank or own bank licence | Tokenised deposits inside the bank perimeter | Swap the sponsor adapter for direct rails; enable tokenised-deposit asset behind a licence flag |
| C (cleared) | Unchanged | Stablecoin corridors via the separate digital-asset entity | Digital-asset services and intercompany settlement |

The open-source core is identical across all three stages. Design so that Stage B and C are configuration and new adapters, not rewrites. The switch from Stage A to Stage B is a change of `BankLayer` implementation (section 5.2), not a rewrite.

**Stage B has two possible legal forms (ADR-0020, open):** a mutual bank owned by its depositors, or a bank under the Banks Act. The code supports both: everything licence-specific sits behind `BankLayer`, and everything mutual-specific sits in a membership module that is designed now and built only if ADR-0020 chooses a mutual (section 5.4).

#### 1.3 Legal entities

| Entity | Code | Holds | Never holds |
|---|---|---|---|
| Forge Technologies (Pty) Ltd | `PLAT` | Product, workspace, party and consent data, consolidated view | Customer money, crypto |
| Bank layer (sponsor now; own licence later, mutual or bank per ADR-0020) | `BANK` | ZAR and foreign-currency deposits, payments | Crypto, stablecoins |
| Digital-asset entity (deferred) | `DAST` | Stablecoin custody, on/off-ramp | Deposits |
| MFI tenants (year 2) | `MFI:<tenant>` | Each MFI's own loan book and, if a registered co-op, member savings | Another tenant's data or money |

#### 1.4 v1 scope

In v1:
- ZAR business accounts at the sponsor, with a Forge sub-ledger.
- **A USD account via one partner bank**, mirrored in the bank ledger, for receiving foreign revenue and paying USD costs (cloud, contractors, SaaS).
- Cross-border payouts and FX via an authorised-dealer partner.
- KYB/KYC via Didit; party and consent services; founder-and-company separation.
- Compliance core (sanctions, monitoring, goAML); concentration and outflow alerts (tech deposits are volatile).
- Web workspace (balances by currency, payments, approvals, team roles) and back-office.
- The `BankLayer` interface with `SponsorBankLayer` as the only implementation.

Deferred: everything in the digital-asset entity, tokenised deposits, cards, PayShap direct participation, USSD, WhatsApp, mobile, BaaS partner API, MFI platform, credit bureau, lending, DeFi, self-custody, Forge-issued stablecoins, `OwnLicenceBankLayer`, the membership module. EUR and GBP accounts follow USD once the partner-bank pattern is proven.

---

### 2. Architectural principles and invariants

#### 2.1 Principles

1. **Commands go down, events go up.** Services never read another service's database. They call APIs or consume events.
2. **One authority per fact.** Balances: TigerBeetle. Customers and products: Fineract and party-service. Permissions: Keycloak plus OPA. Consent: consent-service.
3. **Legal entities are hard boundaries.** Separate ledgers, databases, credentials and namespaces.
4. **Everything money-moving is a durable workflow.** Every step has a compensation.
5. **Rules are data.** Limits, sectors, pricing and eligibility live in OPA bundles, never hard-coded.
6. **Swappable vendors.** Every external dependency (sponsor bank, authorised dealer, KYC, sanctions data, HSM, hosting) sits behind an interface with a simulator.
7. **Fail closed.** Stale sanctions lists, missing consent, reconciliation breaks and unknown assets all deny.
8. **Open and sovereign.** Self-hostable open-source components with licences recorded; no hard dependency on one foreign cloud.

#### 2.2 Invariants (CI enforces; a change that breaks one does not merge)

**Money**
1. Conservation per asset per ledger: sum of debits equals sum of credits. Money is created or destroyed only through audited mint/burn or external settlement accounts.
2. No negative customer balances unless the product allows overdraft and the account carries the flag.
3. Multi-leg operations within one TigerBeetle cluster are atomic (linked transfers).
4. Every money-moving command carries an idempotency key; replays return the original result and never move money twice.
5. Money is integers in minor units. No floating point anywhere for money.

**Legal entities**
6. One writer per ledger cluster: only `ledger-gateway-bank` writes to the bank cluster; only `ledger-gateway-dasset` writes to the digital-asset cluster; only `ledger-gateway-mfi` writes to the MFI cluster.
7. No transfer spans two entities' ledgers. Cross-entity value moves only via `intercompany-settlement`.
8. The bank cluster rejects third-party stablecoin asset codes. Tokenised-deposit assets are rejected unless `licence.bank.own = true` and the asset is approved.
9. Daily reconciliation of the bank sub-ledger against sponsor and partner-bank statements and against Fineract. A break alarms and blocks outbound payments above a policy threshold until resolved.

**Customers and data**
10. Every balance shown to a customer carries its holding entity and deposit-insurance status.
11. No third-party data is accessed, stored or used without a recorded, purpose-bound, revocable consent.
12. Founder and company accounts are never commingled; person-to-entity transfers use transfer code `63`, are explicit, logged and policy-approved.
13. MFI tenants are isolated: no read or write crosses tenants.

**Control**
14. Every policy decision and money movement is logged with inputs and policy version; the audit log is append-only and hash-chained.
15. Default deny for every permission, limit, asset, sector and feature flag.
16. When on-chain is enabled: only `signer` signs; keys never leave HSM/MPC in production; ledger leads, chain follows.

---

### 3. System context

```mermaid
flowchart TB
  subgraph Users
    C[Business customers<br/>finance leads, founders]
    O[Forge operations staff]
    P[BaaS partners - year 2]
    M[MFI operators - year 2]
  end

  subgraph Forge["Forge platform (this repository)"]
    PLAT[Platform services<br/>Forge Technologies]
    BANK[Bank-layer services<br/>sub-ledger]
    DAST[Digital-asset services<br/>deferred]
    MFI[MFI platform<br/>deferred]
  end

  subgraph External
    SB[Sponsor bank<br/>accounts, EFT, PayShap, statements]
    AD[Authorised-dealer partner<br/>FX, cross-border payout, BoP reporting]
    PB[Partner bank<br/>USD account - v1]
    KYC[Didit<br/>KYC and KYB]
    SAN[Sanctions data<br/>FIC TFS list, UN, others]
    FIC[FIC goAML]
    ACC[Accounting platforms<br/>Xero, Sage - later]
    CH[Blockchains and custodian<br/>deferred]
  end

  C --> PLAT
  O --> PLAT
  P -.-> PLAT
  M -.-> MFI
  PLAT --> BANK
  BANK <--> SB
  BANK <--> AD
  BANK <--> PB
  PLAT <--> KYC
  BANK --> SAN
  BANK --> FIC
  PLAT -.-> ACC
  DAST -.-> CH
  BANK <-.->|intercompany settlement| DAST
```

**External systems and how Forge connects**

| System | Purpose | Interface (VERIFY exact format after contract) | Simulator |
|---|---|---|---|
| Sponsor bank | Holds deposits in pooled account(s); EFT and PayShap; statements | API, SFTP or ISO 20022 (pain.001 out, pain.002/camt.054 in, camt.053 statements) | `tools/sim/sponsor` |
| Authorised-dealer partner | FX quotes, conversion, cross-border payout, Balance of Payments reporting | Partner API | `tools/sim/ad-partner` |
| Partner bank (USD, v1) | Foreign-currency account: receive foreign revenue, pay USD costs | Partner API, statements (camt.053 or partner format, VERIFY) | `tools/sim/partner-bank` |
| Didit | Identity and business verification | REST API plus signed webhooks | `FakeKycProvider` |
| Sanctions data | Screening lists | Files or API; FIC targeted financial sanctions list is mandatory (VERIFY sources and licences) | Fixture lists |
| FIC goAML | STR, CTR and other reports | goAML XML, official schema (VERIFY) | Schema validation only |

---

### 4. Logical architecture

```mermaid
flowchart TB
  subgraph Edge["Channels and edge"]
    WEB[web workspace<br/>Next.js]
    BO[back-office<br/>Next.js]
    GW[Envoy Gateway<br/>mTLS, JWT, rate limits]
    KC[Keycloak]
  end

  subgraph Platform["services/platform"]
    ONB[onboarding]
    PARTY[party-service]
    CONS[consent-service]
    WS[workspace-api]
    VIEW[consolidated-view]
    NOTIF[notifications]
  end

  subgraph Shared["services/shared"]
    POL[policy-service<br/>OPA]
    COMP[compliance-engine]
    AUD[audit-service]
    ICS[intercompany-settlement]
  end

  subgraph Bank["services/bank"]
    PAY[payments-orchestrator<br/>Temporal]
    POUT[payouts-service]
    FX[fx-service]
    LGB[ledger-gateway-bank]
    CB[core-bridge]
    RAIL[rails-gateway]
    PBR[partner-bank-router]
    REC[reconciliation]
    TR[treasury-risk]
    REG[reg-reporting]
    TBB[(TigerBeetle<br/>bank cluster)]
    FIN[(Fineract)]
  end

  WEB --> GW
  BO --> GW
  GW --> KC
  GW --> WS
  GW --> ONB
  WS --> PAY
  WS --> VIEW
  ONB --> PARTY
  ONB --> CONS
  PAY --> POL
  PAY --> COMP
  PAY --> FX
  PAY --> LGB
  PAY --> RAIL
  POUT --> PAY
  LGB --> TBB
  CB --> FIN
  RAIL --> PBR
  REC --> LGB
  REC --> CB

  KAFKA{{Kafka}}
  LGB -. events .-> KAFKA
  PAY -. events .-> KAFKA
  KAFKA -. events .-> CB
  KAFKA -. events .-> VIEW
  KAFKA -. events .-> COMP
  KAFKA -. events .-> AUD
  KAFKA -. events .-> TR
  KAFKA -. events .-> NOTIF
```

**Layers**

| Layer | Responsibility | Key components |
|---|---|---|
| Channels and edge | User interfaces, TLS termination, authentication, rate limits | Next.js apps, Envoy Gateway, Keycloak |
| Platform | Customers, parties, consent, workspace, read models; moves no money | onboarding, party-service, consent-service, workspace-api, consolidated-view |
| Orchestration | Durable multi-step operations with compensation | Temporal workers in payments-orchestrator, payouts-service, onboarding |
| Policy and compliance | Allow/deny/step-up decisions; screening; monitoring | policy-service (OPA), compliance-engine |
| Bank layer | Sub-ledger, rails, FX, reconciliation, reporting | ledger-gateway-bank, TigerBeetle, core-bridge, Fineract, rails-gateway, fx-service |
| Digital-asset layer (deferred) | Separate ledger, signing, chain I/O | ledger-gateway-dasset, signer, onchain-watcher, onchain-executor |
| Backbone | Events, read models, audit, telemetry | Kafka, PostgreSQL, OpenTelemetry, audit-service |

---

### 5. Legal-entity boundaries in code

Entity boundaries are enforced at five levels, so a bug at one level does not breach the boundary.

| Level | Enforcement |
|---|---|
| Repository | Services live in `services/platform`, `services/bank`, `services/dasset`, `services/mfi`, `services/shared`. A CI check (`make test-isolation`) rejects imports of another entity's internal packages and any shared database DSN. |
| Kubernetes | One namespace per entity (`forge-plat`, `forge-bank`, `forge-dasset`, `forge-mfi`, `forge-shared`). Cilium network policies default deny; only listed service-to-service calls are allowed. |
| Identity | Each service has its own workload identity (SPIFFE-style ID via service account and mTLS). Credentials are scoped per entity in OpenBao paths `secret/<entity>/<service>`. |
| Data | One PostgreSQL cluster per entity (CloudNativePG), one database per service. One TigerBeetle cluster per money-holding entity. |
| Ledger semantics | The bank gateway rejects non-fiat assets; cross-entity value moves only via `intercompany-settlement` with matching legs. |

**Shared services** (`policy-service`, `compliance-engine`, `audit-service`, `intercompany-settlement`) serve all entities but partition data by `entity_code` and hold no balances.

#### 5.2 The bank-layer abstraction (ADR-0020)

The two routes, sponsor-led and own licence, share about 90% of the architecture. Everything above the bank layer is identical: platform, workflows, policy, compliance, ledger, reconciliation and apps. Everything that differs sits behind one interface in `pkg/banklayer`, defined in Phase 0 and used by every bank-layer service from day one.

```go
package banklayer

// BankLayer is the only place that knows whether Forge runs on a sponsor bank
// or under its own licence. No other package may branch on the licence route.
type BankLayer interface {
    // Accounts
    OpenAccount(ctx context.Context, req OpenAccountRequest) (Account, error)
    CloseAccount(ctx context.Context, accountID string, reason string) error

    // Movement of money outside Forge's books
    Rails() []rails.Rail                     // sponsor-routed now, direct later
    ForeignCurrency() []fx.ForeignAccountProvider // partner banks for USD (v1), EUR, GBP

    // Evidence for reconciliation
    StatementSource() StatementSource        // sponsor and partner statements now; own settlement accounts later

    // Regulatory duties that depend on who holds the licence
    DepositInsurance() InsuranceReporter     // sponsor-reported now; Forge reports to CODI later
    PrudentialReporter() PrudentialReporter  // no-op now; regulatory returns later

    // What this route allows; read by OPA as data
    Capabilities() Capabilities
}

type Capabilities struct {
    Route                 Route // ROUTE_SPONSOR, ROUTE_OWN_LICENCE
    LegalForm             LegalForm // LEGAL_FORM_NONE, LEGAL_FORM_MUTUAL, LEGAL_FORM_BANK
    DirectClearing        bool  // PayShap and clearing participation
    TokenisedDeposits     bool  // only true under own licence, after approval
    MemberShares          bool  // only true for a mutual
    LegalBookOfRecord     BookOfRecord // BOOK_SPONSOR or BOOK_FORGE
}
```

**Implementations**

| Implementation | When | What it does |
|---|---|---|
| `SponsorBankLayer` | v1 | Accounts are sub-ledger accounts inside the sponsor's pooled account; rails route through the sponsor; statements come from the sponsor and the USD partner bank; deposit insurance is the sponsor's duty; prudential reporting is a no-op; `LegalBookOfRecord = BOOK_SPONSOR` |
| `OwnLicenceBankLayer` | Stage B, designed now | Accounts are Forge's own deposit accounts; direct clearing; Forge's own settlement accounts at the Reserve Bank or a clearing bank; Forge reports to CODI and files prudential returns; `LegalBookOfRecord = BOOK_FORGE` |
| `SimBankLayer` | Always | Drives local, dev and CI through the simulators |

**Rules for agents**

1. No service outside `pkg/banklayer` and its adapters may check which route is active. Behaviour that varies by route is expressed as `Capabilities` and evaluated by OPA (`forge.banklayer` package).
2. Both real implementations must pass the same contract test suite (`make test-banklayer`), with route-specific tests on top.
3. The active implementation is chosen by configuration (`config/banklayer.yaml`), never by code changes.
4. Migration from sponsor to own licence is a documented, rehearsed runbook (`docs/runbooks/banklayer-migration.md`): open Forge-held accounts, move balances customer by customer with matching ledger legs, switch rails, reconcile, close sponsor sub-ledger accounts.

#### 5.3 Own-licence components (designed now, built at Stage B)

These are needed for any own licence, mutual or not. Write their specs in Phase 2 so the v1 code does not block them; build them only when the licence path is confirmed.

| Component | Purpose | Effort (6–8 person team) | Mutual-specific |
|---|---|---|---|
| `prudential-reporting` | Regulatory returns, capital and liquidity calculations | 3–5 months | No |
| `direct-rails` | PayShap and clearing participation, certification | 4–6 months plus the certification queue | No |
| `deposit-insurance` | CODI depositor data and reporting | 1–2 months | No |
| `treasury-liquidity` | Liquidity management, stress testing, investment of surplus | 2–3 months (extends `treasury-risk`) | No |
| `tokenised-deposits` | The original on-chain vision, inside the bank perimeter | 3–4 months plus a contract audit | No |
| `membership` | Member register, shares, voting, rebates or dividends, AGM records | 2–3 months | **Yes** |

Each has a `docs/specs/<component>/SPEC.md` and a stub package with interfaces only. CI fails if any stub is wired into a running service before its ADR is accepted.

#### 5.4 The membership module (mutual only, designed not built)

Built only if ADR-0020 chooses a mutual. Design now so that party and account data can support it later without migration.

**Data model (spec only)**

| Table | Fields |
|---|---|
| `member` | `member_id`, `party_id`, `member_class` (individual, business, MFI, co-operative), `admitted_at`, `status` |
| `member_share` | `member_id`, `share_class`, `units`, `paid_up_minor`, `issued_at`, `redeemed_at` |
| `member_vote` | `resolution_id`, `member_id`, `weight`, `choice`, `cast_at`, `channel` |
| `resolution` | `resolution_id`, `meeting_id`, `text_ref`, `voting_rule`, `result` |
| `meeting` | `meeting_id`, `type` (AGM, special), `notice_sent_at`, `quorum`, `minutes_ref` |
| `member_distribution` | `period`, `member_id`, `basis` (rebate on fees, interest uplift, dividend), `amount_minor`, `paid_transfer_id` |

**Design rules**

- Voting rules (one member one vote, or weighted) are data in OPA, set by the bank's constitution (VERIFY Mutual Banks Act requirements).
- Member share capital is booked in the bank ledger with dedicated account codes (`8001 MEMBER_SHARE_CAPITAL`, `8002 MEMBER_DISTRIBUTIONS_PAYABLE`), reserved now.
- Distributions are calculated by a workflow, approved by maker-checker, and paid as ledger transfers with code `80 MEMBER_DISTRIBUTION`.
- Party data already separates persons, companies and funds, so MFIs and co-operatives can become members without schema changes.

#### 5.5 Why both routes are kept open

**What a true mutual would help with**

- Trust: a bank owned by its depositors is a credible story in a market tired of fees.
- The MFI network: MFIs and co-operatives could become members of the bank, an apex structure where the network owns its funder, much like European co-operative banking groups.
- Redistribution: surplus returns to members through better pricing or rebates, not to outside shareholders.
- Stokvels and communities: member ownership fits savings groups culturally.
- Possibly a lower capital hurdle than a commercial bank (the earlier R10M figure is unverified).

**What it costs:** it cannot easily raise venture equity or be sold, it grows more slowly, and it needs a licence before launch.

**Current lean (a hybrid):** a sponsor-led platform now, with a mutual or co-operative structure introduced for the MFI network later (section 17.4). The code supports any outcome because the route lives behind `BankLayer` and the mutual-specific parts live in the membership module.

---

### 6. Service catalogue

Language defaults: Go for anything on the money path; TypeScript for UIs and (later) chain tooling. Every service has `SPEC.md`, `README.md`, `Dockerfile`, Helm chart, `/healthz`, `/readyz`, `/metrics`, and OpenTelemetry instrumentation.

#### 6.1 Platform (`services/platform`, entity PLAT)

| Service | Lang | Responsibility | Interfaces | Store | Phase |
|---|---|---|---|---|---|
| onboarding | Go | KYB/KYC workflows (Temporal); creates party, consent, accounts and limits | REST (edge), Didit webhooks, Temporal | Postgres | 1 |
| party-service | Go | Parties (person, company, fund), relationships (director, shareholder, UBO), roles | ConnectRPC | Postgres | 1 |
| consent-service | Go | Consent purposes, scopes, expiry, revocation, audit | ConnectRPC; emits events | Postgres | 1 |
| workspace-api | Go | Backend-for-frontend for the customer workspace | REST (OpenAPI) | Postgres (session-free; reads views) | 1 |
| consolidated-view | Go | Read models: balances by entity, statements, activity | ConnectRPC; consumes events | Postgres | 1 |
| notifications | Go | Email, SMS and push templates | Consumes events | Postgres | 1 |
| baas-api | Go | Partner tenancy, OAuth clients, webhooks, sandbox | REST, OAuth2 | Postgres | Year 2 |
| integrations-hub | Go | Accounting/payroll connectors, token vault | ConnectRPC | Postgres, OpenBao | 2+ |

#### 6.2 Bank layer (`services/bank`, entity BANK)

| Service | Lang | Responsibility | Interfaces | Store | Phase |
|---|---|---|---|---|---|
| ledger-gateway-bank | Go | Only writer to the bank TigerBeetle cluster; accounts, transfers, holds, balances, idempotency, outbox | ConnectRPC | TigerBeetle, Postgres | 0–1 |
| core-bridge | Go | Projects ledger events into Fineract; nightly GL reconciliation | Consumes events; Fineract REST | Postgres | 1 |
| payments-orchestrator | Go | Temporal workflows: domestic, cross-border, refund | ConnectRPC; Temporal | Postgres | 1–2 |
| payouts-service | Go | Bulk payouts (contractors, suppliers) as child workflows | ConnectRPC | Postgres | 2 |
| fx-service | Go | Quotes, spreads, quote lock, booking legs | ConnectRPC; AD partner adapter | Postgres | 2 |
| rails-gateway | Go | `Rail` interface; sponsor adapter; ISO 20022 messages; RailSim | ConnectRPC | Postgres, object storage | 1–2 |
| partner-bank-router | Go | Routes foreign-currency operations to partner banks by country, currency, cost and health; USD account in v1 | ConnectRPC | Postgres | 1–2 |
| reconciliation | Go | Statement ingestion (camt.053), three-way recon, breaks, circuit breaker | Batch, ConnectRPC | Postgres, object storage | 1–2 |
| treasury-risk | Go | Deposit concentration, outflow velocity, liquidity alerts, stress scenarios | Consumes events | Postgres | 1 |
| reg-reporting | Go | goAML reports, evidence packs; BoP data handoff to AD partner | Batch | Postgres, WORM storage | 2 |

#### 6.3 Shared (`services/shared`)

| Service | Lang | Responsibility | Phase |
|---|---|---|---|
| policy-service | Go + Rego | OPA decision point; signed bundle loading; decision log | 1 |
| compliance-engine | Go | Sanctions screening, monitoring rules, alerts, cases | 1–2 |
| audit-service | Go | Append-only, hash-chained audit log; WORM anchors; verify CLI | 1 |
| intercompany-settlement | Go | Two-sided, arm's-length settlement between entities; daily reconciliation | Stage C (design now) |

#### 6.4 Digital-asset (`services/dasset`, entity DAST, deferred)

ledger-gateway-dasset (Go), signer (Go), onchain-watcher (TS), onchain-executor (TS), travel-rule (Go). See section 16.

#### 6.5 MFI platform (`services/mfi`, year 2)

ledger-gateway-mfi (Go), mfi-tenant-service (Go), mfi-loans (Fineract multi-tenant via core-bridge-mfi), mfi-portal (Next.js). See section 17.

#### 6.6 Apps (`apps/`)

| App | Stack | Users | Phase |
|---|---|---|---|
| web | Next.js, TypeScript, Tailwind | Customer workspace | 1 |
| backoffice | Next.js | Ops: KYC review, cases, payment repair, recon dashboard, maker-checker | 1 |
| dev-portal | Docs site | BaaS partners | Year 2 |
| mfi-portal | Next.js | MFI operators | Year 2 |

API clients are generated from OpenAPI and Protobuf. No hand-written fetch wrappers.

---

### 7. Ledger design (TigerBeetle)

TigerBeetle is the authority for balances and transfers. Read the TigerBeetle documentation for the pinned version before implementing (VERIFY field names, limits and recommended ID scheme for the version in use).

#### 7.1 Clusters

| Cluster | Entity | Writer | Stage |
|---|---|---|---|
| `tb-bank` | BANK | ledger-gateway-bank | A |
| `tb-dasset` | DAST | ledger-gateway-dasset | C |
| `tb-mfi` | MFI tenants | ledger-gateway-mfi | Year 2 |

Production: 3 to 6 replicas across failure domains, dedicated nodes, NVMe storage, never shared with other workloads. Development: 1 replica.

#### 7.2 Ledgers and asset registry

In TigerBeetle a transfer can only move value between accounts on the same `ledger`. Forge uses one `ledger` number per asset (and, in the MFI cluster, per tenant and asset), which gives hard isolation for free.

Registry lives in `proto/forge/assets/v1/assets.proto` and `config/assets.yaml`:

| Cluster | Ledger | Asset | Scale | Status |
|---|---|---|---|---|
| tb-bank | 710 | ZAR | 2 | Active |
| tb-bank | 840 | USD (partner-bank mirror) | 2 | Active v1 |
| tb-bank | 978 | EUR (partner-bank mirror) | 2 | Later |
| tb-bank | 826 | GBP (partner-bank mirror) | 2 | Later |
| tb-bank | 7100 | ZAR tokenised deposit | 2 | Reserved; rejected unless own licence |
| tb-dasset | 1001 | USD stablecoin (issuer per ADR-0005) | token decimals, commonly 6 | Deferred |
| tb-dasset | 1002 | Rand stablecoin | token decimals | Deferred |
| tb-mfi | `tenant_index * 1000 + iso_numeric` | ZAR per tenant | 2 | Year 2 |

Using ISO 4217 numeric codes for fiat makes ledgers self-describing. The gateway validates every request against the registry and the cluster's allowed set.

#### 7.3 Account fields

| Field | Use |
|---|---|
| `id` (u128) | Generated by the gateway (see 7.6) |
| `ledger` (u32) | Asset (and tenant in tb-mfi) |
| `code` (u16) | Account type (Appendix B) |
| `user_data_128` | Internal owner ID (party, system or tenant UUID) |
| `user_data_64` | Product ID |
| `user_data_32` | Entity/tenant partition ID |
| `flags` | Customer: `debits_must_not_exceed_credits` + `history`. System accounts per Appendix B. Overdraft products omit the debit constraint and carry an explicit limit enforced by policy. |

A Postgres `account_directory` maps business identity to TigerBeetle IDs:

```sql
create table account_directory (
  tb_account_id   numeric(39,0) primary key,
  entity_code     text not null,          -- BANK, DAST, MFI:<tenant>
  owner_party_id  uuid,                   -- null for system accounts
  asset           text not null,          -- ZAR, USD...
  ledger          bigint not null,
  code            int not null,
  product_id      text,
  status          text not null,          -- open, frozen, closed
  insured         boolean not null,       -- disclosure (invariant 10)
  created_at      timestamptz not null default now()
);
```

#### 7.4 Transfer fields

| Field | Use |
|---|---|
| `id` (u128) | Gateway-generated, bound to the idempotency key (7.6) |
| `debit_account_id`, `credit_account_id` | Same `ledger` |
| `amount` (u128) | Minor units |
| `ledger`, `code` | Asset and transfer type (Appendix B) |
| `user_data_128` | Business transaction UUID (traces every transfer to a workflow) |
| `user_data_64` | Workflow step number |
| `user_data_32` | Entity/tenant partition |
| `flags` | `linked`, `pending`, `post_pending_transfer`, `void_pending_transfer` as needed |
| `timeout` | Seconds for pending transfers, so stale holds expire |

#### 7.5 Patterns

**Holds (two-phase).** A payment reserves funds with a pending transfer when accepted; posts when the rail confirms; voids on failure, cancellation or timeout.

**Linked chains.** Multi-leg operations (fees plus principal, FX legs on mirror ledgers) are submitted as one linked chain within one cluster. Linked chains may include transfers on different ledgers of the same cluster (VERIFY for pinned version).

**Fees.** Always a separate transfer to `FEES_INCOME`, never netted silently.

**FX inside tb-bank** (when Forge holds USD mirrors), one linked chain:
1. customer ZAR → `FX_POSITION` ZAR (ledger 710)
2. `FX_POSITION` USD → customer USD (ledger 840)
3. customer ZAR → `FEES_INCOME` ZAR (spread or fee, explicit)

**Cross-border payout from ZAR** (no USD held): customer ZAR → `AD_PARTNER_SETTLEMENT` ZAR as a pending transfer; post when the partner confirms execution. The FX happens at the partner.

**USD payout from the customer's USD account** (v1): customer USD → `PARTNER_BANK_MIRROR` USD (ledger 840) as a pending transfer; post when the partner bank confirms. No FX.

**Inbound USD** (v1, the common tech-company case: foreign customers paying a South African software firm): partner bank notifies the credit → `PARTNER_BANK_MIRROR` USD → customer USD. Converting to ZAR is a separate, customer-initiated FX booking. Exchange-control reporting for inbound flows is handled with the partner (VERIFY).

**Sponsor sub-ledger.** All customer deposits are liabilities against `SPONSOR_POOLED` (code 3003), which mirrors the sponsor's pooled account. Inbound EFT: `SPONSOR_POOLED` → customer. Outbound: customer → `RAIL_SETTLEMENT_<rail>` (pending), post on confirmation, then settle `RAIL_SETTLEMENT` → `SPONSOR_POOLED` when the sponsor statement shows the debit.

#### 7.6 Identifiers and idempotency

1. Every money-moving request carries `Idempotency-Key` (edge) / `idempotency_key` (internal).
2. The gateway writes `idempotency_record(key, request_hash, tb_transfer_ids, status, result)` to Postgres before submitting to TigerBeetle.
3. Transfer IDs are generated once (time-ordered, ULID-style, as TigerBeetle recommends for performance; VERIFY) and stored in the record.
4. A retry with the same key and same hash returns the stored result; if the transfer was submitted but the result was lost, the gateway resubmits the same IDs; TigerBeetle returns `exists`, which the gateway treats as success when fields match.
5. Same key with a different payload returns `409 IDEMPOTENCY_CONFLICT`.
6. Records are kept for at least 30 days.

#### 7.7 Gateway API (ConnectRPC, `proto/forge/ledger/v1/ledger.proto`)

```protobuf
syntax = "proto3";
package forge.ledger.v1;

message Money {
  string asset = 1;          // "ZAR"
  string amount_minor = 2;   // decimal string of an unsigned integer; never float
}

message TransferLeg {
  string debit_account_id = 1;
  string credit_account_id = 2;
  Money amount = 3;
  uint32 code = 4;
}

message CreateTransfersRequest {
  string idempotency_key = 1;
  string business_txn_id = 2;     // UUID
  string entity_code = 3;         // must match the gateway's entity
  repeated TransferLeg legs = 4;  // submitted as one linked chain
  bool pending = 5;
  uint32 pending_timeout_seconds = 6;
  string actor = 7;               // service or user
  string reason = 8;
}

message CreateTransfersResponse {
  repeated string transfer_ids = 1;
  Status status = 2;              // OK, REPLAYED, REJECTED
  repeated string rejection_codes = 3;
}

service LedgerService {
  rpc CreateAccounts(CreateAccountsRequest) returns (CreateAccountsResponse);
  rpc CreateTransfers(CreateTransfersRequest) returns (CreateTransfersResponse);
  rpc PostPending(PostPendingRequest) returns (CreateTransfersResponse);
  rpc VoidPending(VoidPendingRequest) returns (CreateTransfersResponse);
  rpc GetBalances(GetBalancesRequest) returns (GetBalancesResponse);
  rpc LookupTransfers(LookupTransfersRequest) returns (LookupTransfersResponse);
}
```

**Gateway enforcement:** asset registry and cluster allow-list; entity match; caller allow-list per RPC (deny by default); amount sanity limits; tenant partition check; no business logic beyond enforcement.

**Batching:** coalesce concurrent requests into batches within a few milliseconds, never splitting a linked chain.

#### 7.8 Outbox and events

After a successful commit the gateway writes `outbox(event_id, payload)` in the same Postgres transaction as the idempotency record update, then a relay publishes `forge.bank.ledger.transfer.committed.v1` to Kafka. A reconciler compares TigerBeetle lookups with published events and republishes gaps. Consumers are idempotent by `event_id`.

#### 7.9 Read model

`consolidated-view` builds balances and statements in Postgres from committed-transfer events for fast UI queries. The ledger authorises; if the read model and the ledger disagree, the ledger wins and an alert fires.

---

### 8. Core banking (Fineract) and core-bridge

**Decision (ADR-0001, to confirm in Phase 0 spike):** TigerBeetle owns balances. Fineract owns customer records, product definitions, fee schedules, loan schedules and interest logic, and a statutory general ledger as a projection.

- `core-bridge` consumes `transfer.committed` events and posts matching journal entries to Fineract via REST, deduplicated by `transfer_id`.
- `projection_state(transfer_id, status, fineract_txn_id, attempts)`; retries with backoff; poison events to a dead-letter topic for human review.
- Nightly `Reconcile(date)`: per-account balances and GL totals, Fineract versus TigerBeetle. Drift opens an incident.
- Fineract balances are informational. No feature authorises from them.
- Fineract configuration (products, charges, accounting rules) is code in `core/fineract/` applied by a repeatable provisioning job.

**Phase 0 spike must prove:** projection throughput and latency, replay and out-of-order handling, reconciliation accuracy, and developer effort.

**Fallback (ADR-0001-B):** if the spike fails, replace Fineract with a thin Go account and product service on Postgres, and defer loan accounting. The MFI platform (section 17) would then need its own loan engine decision.

---

### 9. Workflows and sagas (Temporal)

Temporal (Go SDK) runs every multi-step operation. Workflows are deterministic, versioned and replay-tested. Activities are idempotent.

#### 9.1 Workflow catalogue

| Workflow | Owner | Phase |
|---|---|---|
| `OnboardBusiness` | onboarding | 1 |
| `OnboardPerson` | onboarding | 1 |
| `DomesticPayment` | payments-orchestrator | 1 (sim), 2 (sponsor) |
| `CrossBorderPayment` | payments-orchestrator | 2 |
| `BulkPayout` (parent) | payouts-service | 2 |
| `InboundCredit` | payments-orchestrator | 1 |
| `Refund` / `Return` | payments-orchestrator | 2 |
| `DailyReconciliation` | reconciliation | 1 |
| `IntercompanySettlement` | intercompany-settlement | Stage C |

#### 9.2 Standard step pattern

`validate → policy check → compliance pre-check → reserve (pending) → execute on rail/partner → await confirmation (signal or poll, with timeout) → post → notify → emit events`

Each step after reserve has a compensation: void pending, request recall, reverse posting with code `90 REVERSAL`, open a repair case.

#### 9.3 Cross-border payment sequence

```mermaid
sequenceDiagram
  autonumber
  participant U as Customer (web)
  participant WS as workspace-api
  participant FX as fx-service
  participant WF as CrossBorderPayment (Temporal)
  participant POL as policy-service
  participant CMP as compliance-engine
  participant LG as ledger-gateway-bank
  participant AD as AD partner adapter

  U->>WS: Request quote (amount, currency, beneficiary, invoice)
  WS->>FX: GetQuote
  FX->>AD: Partner quote
  AD-->>FX: Rate, fees, expiry
  FX-->>WS: All-in ZAR cost, spread disclosed, quote_id, expiry
  U->>WS: Confirm (Idempotency-Key, quote_id)
  WS->>WF: Start workflow (workflow ID = idempotency key)
  WF->>POL: Check limits, sector, step-up
  POL-->>WF: allow / deny / step-up
  WF->>CMP: Screen beneficiary and country, capture exchange-control data
  CMP-->>WF: clear / hold
  WF->>LG: Pending transfer: customer ZAR -> AD_PARTNER_SETTLEMENT (+ fee leg)
  LG-->>WF: transfer_ids
  WF->>AD: Execute payout (quote_id, beneficiary, purpose code)
  AD-->>WF: Accepted (partner_ref)
  AD-->>WF: Signal: executed / rejected
  alt executed
    WF->>LG: Post pending
    WF-->>U: Notify success, proof of payment
  else rejected or timeout
    WF->>LG: Void pending
    WF-->>U: Notify failure with reason
  end
```

Quote rules: the quote shown to the customer is stored with `quote_id`; execution after expiry re-quotes and requires re-confirmation.

#### 9.4 Workflow IDs and idempotency

Workflow ID = `<type>:<idempotency_key>`, with Temporal's ID reuse policy set to reject duplicates while running and return the existing result after completion. Activities pass the same idempotency keys to the ledger and partners.

---

### 10. Policy as code (OPA)

`policy-service` embeds OPA as a Go library (or runs OPA sidecars; decide in ADR-0010-B), loads signed bundles from `policy/`, and logs every decision to `forge.shared.policy.decision.v1`.

**Packages**

| Package | Decides |
|---|---|
| `forge.limits` | Per-transaction, daily and monthly limits by tier and asset; step-up thresholds |
| `forge.eligibility.sector` | Allowed, restricted and blocked sectors (ADR-0012) |
| `forge.assets` | Which assets each entity and product may hold |
| `forge.perimeter` | What each tenant or partner may do |
| `forge.entity` | Cross-entity rules; person-to-entity transfers (code 63) |
| `forge.consent` | Whether a data use is covered by a valid consent |
| `forge.recon` | Payment thresholds blocked during a reconciliation break |
| `forge.mfi` | MFI product, pricing caps and affordability rules (year 2) |
| `forge.banklayer` | What the active route allows (from `Capabilities`): direct clearing, tokenised deposits, member shares |

**Example (Rego v1)**

```rego
package forge.limits

import rego.v1

default decision := {"allow": false, "reason": "default_deny"}

tier := data.tiers[input.party.tier]

within_limits if {
    input.amount_minor <= tier.per_txn[input.asset]
    input.velocity.day_total_minor + input.amount_minor <= tier.daily[input.asset]
}

decision := {"allow": true, "stepup": true, "reason": "above_stepup"} if {
    input.party.status == "active"
    not input.screening.hit
    not input.recon.break_active
    within_limits
    input.amount_minor > tier.stepup[input.asset]
}

decision := {"allow": true, "stepup": false, "reason": "ok"} if {
    input.party.status == "active"
    not input.screening.hit
    not input.recon.break_active
    within_limits
    input.amount_minor <= tier.stepup[input.asset]
}
```

Every package has `opa test` unit tests in CI. Bundles are versioned, signed and the version is recorded in each decision log.

---

### 11. Identity, onboarding, party and consent

#### 11.1 Keycloak

| Realm | Users | Auth |
|---|---|---|
| `customers` | Business users and founders | Passkeys (WebAuthn) preferred, OTP fallback, device binding, step-up for risky actions |
| `staff` | Forge operations | MFA required; RBAC with segregation of duties; maker-checker for sensitive actions |
| `partners` | BaaS and MFI operators (year 2) | OAuth2 client credentials, optional mTLS, scoped permissions |

Tokens are validated at Envoy (JWT) and again in services for authorisation context. Business roles (owner, approver, viewer) live in party-service and are evaluated by OPA.

#### 11.2 Didit integration (`KycProvider` interface)

```go
type KycProvider interface {
    StartPersonVerification(ctx context.Context, partyID string, opts PersonOpts) (Session, error)
    StartBusinessVerification(ctx context.Context, partyID string, opts BusinessOpts) (Session, error)
    HandleWebhook(ctx context.Context, payload []byte, signature string) (Result, error)
    GetResult(ctx context.Context, sessionID string) (Result, error)
}
```

Implementations: `DiditProvider`, `FakeKycProvider`. Verify webhook signatures; store only what is needed; encrypt PII at rest with OpenBao transit keys; apply POPIA retention. VERIFY Didit coverage against FIC Act and POPIA.

#### 11.3 Onboarding workflow (`OnboardBusiness`)

1. Create prospect party and company party.
2. Record consents (terms, data processing, optional data sharing).
3. KYB on company; KYC on directors and UBOs (Didit).
4. Sanctions screening on all parties.
5. Risk scoring; sector eligibility (OPA).
6. Manual review if needed (back-office task, maker-checker).
7. Create Fineract client and product (core-bridge), then ledger accounts (gateway), with `insured` flag set.
8. Set tier and limits; emit `forge.plat.party.onboarded.v1`.

Party lifecycle: `PROSPECT → KYC_PENDING → ACTIVE → RESTRICTED → FROZEN → CLOSED`, with allowed actions per state enforced by policy.

#### 11.4 party-service

Parties: `person`, `company`, `fund`. Relationships: `director`, `shareholder` (with percentage), `ubo`, `signatory`, `employee`. Founder and company are always separate parties with separate accounts (invariant 12).

#### 11.5 consent-service

Consent record: `consent_id, party_id, purpose, scopes[], data_sources[], granted_at, expires_at, revoked_at, evidence_ref`. Every data access calls `CheckConsent(purpose, scope)`; revocation emits `consent.revoked` and connectors stop within minutes.

---

### 12. Payments, rails and FX

#### 12.1 rails-gateway

```go
type Rail interface {
    Name() string
    Quote(ctx context.Context, req QuoteRequest) (QuoteResult, error)
    Initiate(ctx context.Context, req PaymentInstruction) (InitiateResult, error) // idempotent by instruction ID
    Status(ctx context.Context, instructionID string) (StatusResult, error)
    Cancel(ctx context.Context, instructionID string) error
    HandleInbound(ctx context.Context, msg InboundMessage) ([]RailEvent, error)
}
```

Implementations: `SponsorEFT`, `SponsorPayShap`, `RailSim`. Later: `DirectPayShap` (Stage B, licensed bank).

- **ISO 20022:** generate Go types from official XSDs or use a maintained library (for example moov-io/iso20022, Apache 2.0, VERIFY). Messages: `pain.001` (initiation), `pain.002` (status), `camt.054` (notifications), `camt.053` (statements). Store raw messages in object storage for audit.
- **Sponsor connectivity:** API, SFTP or ISO 20022 over a secure channel, decided after the agreement. Everything runs against `RailSim` until then.
- **RailSim** simulates acceptance, delays, rejections, returns, duplicates and out-of-order messages, with configurable fault injection.

#### 12.2 fx-service

- Quote model: mid rate, spread, fees, all-in ZAR cost, validity window. Store the exact quote shown to the customer.
- `FxProvider` interface: `AuthorisedDealerPartner`, `PartnerBank`, `Sim`.
- Booking: v1 books ZAR to the partner settlement account; with USD mirrors, books linked legs (7.5).
- Exchange-control data capture: purpose codes, invoice, beneficiary details passed to the AD partner for Balance of Payments reporting (VERIFY formats with the partner).

#### 12.3 partner-bank-router (v1, USD)

Technology companies earn in USD and pay USD costs, so a USD account is part of v1.

- `ForeignAccountProvider` interface, reached through `BankLayer.ForeignCurrency()`: `OpenAccount`, `GetAccountDetails` (account number, routing details to share with payers), `InitiatePayout`, `HandleInbound`, `Statement`.
- Implementations: one partner bank in v1 (VERIFY candidates and terms), plus `Sim`.
- Routes by currency, cost and health score, with failover once a second partner exists (the Mercury lesson: partner banks can be lost).
- Partner balances are mirrored in tb-bank with code `5001 PARTNER_BANK_MIRROR` per currency and reconciled daily against partner statements.
- Accounts are either named accounts per customer or virtual accounts under a pooled partner account; the partner's model decides (VERIFY), and the mirror design supports both.
- Exchange-control rules for South African residents holding foreign-currency accounts apply (VERIFY with counsel and the partner).

#### 12.4 Product journeys for technology companies (v1)

| Journey | Flow |
|---|---|
| Receive USD from a foreign customer | Share USD account details → inbound credit → USD balance → optional conversion to ZAR |
| Pay a USD cost (cloud, SaaS, contractor) | USD balance → USD payout via partner bank |
| Convert USD to ZAR for payroll | Quote with disclosed spread → linked FX legs (section 7.5) |
| Pay a foreign supplier from ZAR | `CrossBorderPayment` via the AD partner (section 9.3) |
| Pay local staff and suppliers | ZAR EFT via the sponsor |
| Approve payments as a team | Maker-checker roles in party-service, enforced by OPA |

---

### 13. Reconciliation

Reconciliation is a first-class product feature, not a back-office afterthought.

| Recon | Sources | Frequency | On break |
|---|---|---|---|
| Sponsor three-way | tb-bank `SPONSOR_POOLED` and customer balances; sponsor `camt.053`; Fineract GL | Daily (intraday where available) | Alarm; block outbound payments above `forge.recon` threshold; open case |
| Rail settlement | `RAIL_SETTLEMENT_*` accounts vs sponsor debits/credits | Daily | Age-based alerts on unmatched items |
| AD partner | `AD_PARTNER_SETTLEMENT` vs partner reports | Daily | Case; hold new payouts on large breaks |
| Partner bank | `PARTNER_BANK_MIRROR` vs partner statements | Daily | Case |
| Read model | consolidated-view vs ledger | Hourly sample, nightly full | Rebuild read model; ledger wins |
| Fineract | GL projection vs ledger | Nightly | Incident |
| Intercompany | Matching legs across entities | Daily (Stage C) | Case; pause settlement |
| Chain (deferred) | Custody addresses vs `tb-dasset`; token supply vs liability | Hourly | Circuit breaker on withdrawals |

**Sponsor three-way equation (simplified):**

```
sponsor_statement_closing_balance
  = sum(customer_balances) + sum(fees_income_unswept) + suspense
    + in_flight_inbound - in_flight_outbound   (from pending transfers and rail settlement accounts)
```

Every break gets a `recon_break` row with amount, age, owner and resolution. `SUSPENSE` (code 2005) holds unmatched receipts and alerts by age.

---

### 14. Intercompany settlement

Design now, build in Stage C.

- Each entity holds a ledger account representing the receivable/payable with the other (`INTERCO_DUE_FROM_<entity>`, `INTERCO_DUE_TO_<entity>`).
- A cross-entity customer action (for example, buy a stablecoin with rand) becomes two independent, linked-by-ID operations: rand moves in tb-bank (customer → `INTERCO_DUE_TO_DAST`), and the asset moves in tb-dasset (`INTERCO_DUE_FROM_BANK` side → customer), with a shared `settlement_id`.
- Net positions settle in fiat between the entities' real bank accounts at an agreed cut-off, at arm's length, with documented pricing.
- Daily reconciliation ensures every leg has its counterpart; unmatched legs pause new cross-entity activity.
- The customer sees one action; the books show two entities.

---

### 15. Compliance and regulatory reporting

#### 15.1 compliance-engine

- **Sanctions screening:** at onboarding, periodically, and on every external payment beneficiary. Lists include the FIC targeted financial sanctions list and others counsel requires (VERIFY). Fail closed if a list is stale. Matching engine options: in-house fuzzy matching or yente (MIT) with a commercial data licence where required (VERIFY OpenSanctions licence for commercial use).
- **Transaction monitoring:** data-driven rules consuming ledger and payment events: velocity, structuring, new beneficiary, high-risk geography, rapid in-out, mule patterns, dormant-account reactivation. Each rule has synthetic scenario tests. Evaluate open-source engines (for example Jube, AGPL; Marble, licence VERIFY) before committing.
- **Alerts and cases:** case API for back-office with SLA timers, notes and evidence attachments.
- **Risk scoring:** party and transaction scores, explainable with reason codes.

#### 15.2 reg-reporting

- goAML XML for suspicious and threshold reports, from official schemas stored in `docs/compliance/` (VERIFY current schema and submission process).
- Balance of Payments data handed to the AD partner for cross-border flows (VERIFY division of responsibility).
- Prudential returns only once Forge is the licensed bank (Stage B).
- Each run produces an immutable evidence pack (inputs, queries, versions, sign-off) in WORM storage.

#### 15.3 audit-service

Append-only entries `(seq, ts, actor, action, entity, payload_hash, prev_hash, hash)`; periodic anchor hash exported to WORM storage; `forge-audit verify` CLI.

---

### 16. Digital-asset entity (deferred, Stage C)

Built only after the CASP licence and legal sign-off. Design the interfaces now so v1 code does not block it.

- **Ledger:** `tb-dasset`, own gateway, own Postgres, own namespace.
- **Custody model (ADR-0006):** licensed custodian first, or HSM/MPC-backed treasury. FORGE Custody (from the openfireblocks work) is a candidate implementation behind the `Custody` interface, after its external MPC audit (ADR-0021).
- **signer (Go):** gRPC over mTLS, callable only by onchain-executor and ops tooling. Before signing: OPA check (destination allow-list, amount and velocity, approved asset, dual control above threshold), confirm a live ledger hold, write an audit entry. Backends: SoftHSM2 (dev), PKCS#11 HSM (prod), optional MPC provider.
- **onchain-watcher (TS, viem):** detects deposits, tracks confirmation depth, handles reorgs, at least two RPC providers with quorum on critical reads, cursor persisted in Postgres.
- **onchain-executor (TS, viem):** builds and broadcasts withdrawals already reserved in the ledger, manages nonce and gas, never holds keys, never writes to the ledger.
- **travel-rule (Go):** adapter interface; providers per ADR-0008.
- **Contracts (Foundry, OpenZeppelin, Safe):** only if Forge issues tokens (Stage B tokenised deposits); unit, fuzz and invariant tests, Slither and Echidna, external audit before mainnet.
- **Hourly reconciliation** with a circuit breaker that pauses withdrawals on any mismatch.

---

### 17. MFI platform (year 2)

Forge Technologies hosts the platform as a software provider; each MFI is an independent legal entity and owns its books. The bank layer may fund an MFI as a customer.

#### 17.1 Tenancy model

| Concern | Design |
|---|---|
| Ledger | `tb-mfi` cluster; each tenant gets its own ledger numbers (`tenant_index * 1000 + iso_numeric`), so TigerBeetle itself prevents cross-tenant transfers |
| Core | Fineract multi-tenancy (one tenant schema per MFI) for clients, loan products and schedules (VERIFY Fineract multi-tenant operation at target scale) |
| Data | Separate Postgres database per tenant, or schema-per-tenant with row-level security; decide in ADR-0023 |
| Policy | Per-tenant OPA data; `forge.mfi` enforces product limits, pricing caps, affordability checks and sector rules |
| Identity | Keycloak `partners` realm, tenant-scoped roles |
| Branding | Tenant config in `mfi-tenant-service`; disclosures always name the MFI as the lender, never the bank |

#### 17.2 Wholesale funding

The bank lends to the MFI as a business customer in tb-bank. In the MFI's tenant ledger the facility is a liability (`MFI_FUNDING_LIABILITY`). Disbursement moves real money from the bank customer account to the MFI's operating account; the tenant ledger records it. Exposure limits per MFI and in aggregate live in `forge.mfi` and `treasury-risk`.

#### 17.3 Credit data

A shared credit bureau is a separate regulated business (VERIFY registration under the National Credit Act). Until registered, the platform only offers per-tenant scoring on that tenant's own data, plus reporting to existing registered bureaus.

#### 17.4 The apex option (mutual or co-operative, ADR-0020)

If ADR-0020 introduces a mutual or co-operative structure for the network, MFIs and co-operatives become members of the funding institution, so the network owns its funder.

- The MFI tenant's company party becomes a `member` (section 5.4) with `member_class = MFI` or `co-operative`.
- Wholesale funding terms, rebates and voting weights come from the membership module and OPA data, not from code.
- Surplus returned to MFIs as rebates or distributions is paid as ledger transfers (code `80`) to the MFI's account in tb-bank, then recorded in its tenant ledger.
- Nothing in the tenancy model (17.1) changes: each MFI remains a separate legal entity with isolated books.

---

### 18. Data architecture and events

#### 18.1 Stores

| Store | Use | Licence |
|---|---|---|
| TigerBeetle | Balances and transfers | Apache 2.0 |
| PostgreSQL 17 (CloudNativePG) | Service data, read models, idempotency, outbox, audit | PostgreSQL / Apache 2.0 |
| Valkey | Rate limits, short-lived state | BSD |
| Kafka (Strimzi, KRaft) | Events | Apache 2.0 |
| Object storage (SeaweedFS or Ceph RGW) | Documents, raw rail messages, statements, evidence packs, WORM exports | Apache 2.0 / LGPL |
| ClickHouse + dbt-core (later) | Analytics on consented data | Apache 2.0 |
| OpenSearch | Logs and search | Apache 2.0 |

One database per service; no cross-service database access.

#### 18.2 Event envelope

```protobuf
message EventEnvelope {
  string event_id = 1;         // UUID, unique
  string event_type = 2;       // forge.bank.ledger.transfer.committed
  uint32 schema_version = 3;
  string entity_code = 4;      // PLAT, BANK, DAST, MFI:<tenant>
  string correlation_id = 5;   // end-to-end business flow
  string causation_id = 6;     // event or command that caused this
  string producer = 7;         // service name
  google.protobuf.Timestamp occurred_at = 8;
  bytes payload = 9;           // typed Protobuf message
}
```

Topic naming: `forge.<entity>.<domain>.<entity_name>.<event>.v<N>`, for example `forge.bank.ledger.transfer.committed.v1`. Schemas in `proto/`, registered in Apicurio; `buf breaking` runs in CI. Delivery is at-least-once; consumers are idempotent by `event_id`.

#### 18.3 PII and retention

PII lives only in party-service, onboarding and compliance-engine, encrypted at field level for ID numbers. Events carry party IDs, not PII. Retention schedules per data class follow FIC Act record-keeping and POPIA (VERIFY periods).

---

### 19. APIs

| Surface | Style | Auth | Notes |
|---|---|---|---|
| Customer edge | REST, OpenAPI 3.1 in `api/openapi/` | Keycloak JWT (customers realm) | `Idempotency-Key` required on every money-moving POST |
| Back-office edge | REST, OpenAPI | Keycloak JWT (staff realm), maker-checker | All actions audited |
| Internal | ConnectRPC (Protobuf) over mTLS | Workload identity | Deadlines and retries with backoff |
| Webhooks in | HTTPS | HMAC or signature per vendor | Replay protection by event ID and timestamp |
| Partner (year 2) | REST, OpenAPI | OAuth2 client credentials | HMAC-signed outbound webhooks with retries |

**Error model:** RFC 9457 problem details at the edge; stable error codes (`INSUFFICIENT_FUNDS`, `LIMIT_EXCEEDED`, `STEP_UP_REQUIRED`, `IDEMPOTENCY_CONFLICT`, `QUOTE_EXPIRED`, `COMPLIANCE_HOLD`, `RECON_BREAK_ACTIVE`).

**Money in APIs:** amounts as strings of integer minor units plus an asset code. Never JSON numbers for money.

**Example: create cross-border payment**

```http
POST /v1/payments/cross-border
Idempotency-Key: 0f9c2c3e-...
Authorization: Bearer <jwt>

{
  "quote_id": "q_01J...",
  "source_account_id": "acc_...",
  "beneficiary_id": "ben_...",
  "invoice_ref": "INV-2026-1043",
  "purpose_code": "VERIFY"
}
```

---

### 20. Security architecture

#### 20.1 Controls

| Area | Control |
|---|---|
| Transport | TLS everywhere; mTLS between services (Cilium or Envoy); cert-manager for certificates |
| Workload identity | Per-service identities; short-lived credentials |
| Secrets | OpenBao; External Secrets Operator; never in code, env files or logs |
| Encryption | Envelope encryption for PII via OpenBao transit; field-level for ID numbers; encrypted volumes |
| Keys (deferred) | Non-extractable keys in certified HSM or vetted MPC; documented key ceremonies and rotation |
| Access | Least privilege; staff RBAC with segregation of duties; break-glass with audit |
| Supply chain | Pinned dependencies and lockfiles; SBOM (Syft); scanning (Trivy, Grype, OSV-Scanner); signed images (cosign); Kyverno admits only signed images; no new money-path dependency younger than 14 days without review |
| Code | Semgrep SAST; gitleaks; two human reviews on the money path |
| Runtime | Falco; Kubernetes pod security (non-root, read-only root filesystem, resource limits); network default deny |
| Edge | Rate limits, WAF rules at Envoy, bot detection on public endpoints |
| Data residency | Production data in South Africa unless counsel approves otherwise (VERIFY POPIA cross-border rules and Prudential Authority outsourcing requirements) |

#### 20.2 Threat modelling

Each service has `docs/specs/<service>/THREATS.md` (STRIDE) before Phase 2. Priority threats: account takeover and authorised push payment fraud; insider misuse of back-office; replay and double-spend; reconciliation tampering; sponsor message spoofing; webhook forgery; data exfiltration of PII; dependency compromise; agent-introduced defects.

#### 20.3 External assurance (human-led, schedule early)

Penetration test before pilot; ISO 27001 readiness; PCI scope assessment only if card data is touched (design to stay out of scope via an issuer processor); smart-contract audit before any mainnet deployment.

---

### 21. Infrastructure, environments and operations

#### 21.1 Environments

| Env | Purpose | Data |
|---|---|---|
| `local` | Developer and agent work: Docker Compose or kind + Tilt | Synthetic |
| `dev` | Integration | Synthetic |
| `staging` | Production-like scale, chaos, load, soak | Synthetic only |
| `prod` | Live (after sign-offs) | Real |

Separate clusters, secrets and HSM partitions per environment.

#### 21.2 Platform

Kubernetes (Talos or RKE2), Cilium (network policy, mTLS option), Envoy Gateway, Argo CD (GitOps, app-of-apps), Argo Rollouts (progressive delivery for money-path services), Kyverno (admission policy), cert-manager, External Secrets, Velero (backups), Strimzi (Kafka), CloudNativePG (Postgres), Temporal (self-hosted, Postgres persistence), OpenTofu and Helm for infrastructure as code.

#### 21.3 Hosting (ADR-0002)

Options: sovereign colocation in South Africa, a South African region of a managed cloud with strong controls, or hybrid. Self-hosting everything increases cost and operational risk for a small team. Decide explicitly; keep cloud-specific code behind interfaces; document Prudential Authority and sponsor-bank outsourcing implications (VERIFY).

#### 21.4 Local stack (`make up`)

TigerBeetle (1 replica), PostgreSQL, Kafka (KRaft), Keycloak, OPA, Temporal (+ UI), OpenBao (dev mode), SoftHSM2, Fineract, Valkey, SeaweedFS, OpenTelemetry Collector, SigNoz or Grafana stack, simulators (sponsor, AD partner, Didit fake).

#### 21.5 Resilience

| Component | Target (VERIFY feasibility) |
|---|---|
| Ledger | RPO 0 (replication), RTO 60 minutes |
| Other services | RPO 5 minutes, RTO 60 minutes |

Quarterly TigerBeetle and Postgres recovery drills; chaos experiments each sprint in staging (pod kill, network partition, clock skew, sponsor outage, Kafka broker loss). Runbooks in `docs/runbooks/`.

#### 21.6 Delivery pipeline

GitHub Actions or Forgejo: lint, unit, integration (testcontainers), contract tests, `buf lint` and `buf breaking`, `opa test`, SAST, dependency and image scans, secret scan, SBOM, image signing, licence check, isolation check, recon tests. Database migrations are forward-only, reviewed, and run as pre-deploy jobs with online-safe patterns.

---

### 22. Observability

- OpenTelemetry SDK in every service; OTel Collector to SigNoz, or to Prometheus, Tempo and Loki (VERIFY Grafana-family licences for your use).
- Golden signals per service; business metrics: payment success rate, time to settle, reconciliation breaks and age, pending-hold ageing, quote expiry rate, onboarding conversion, alert backlog.
- Trace context propagated through ConnectRPC, Kafka headers and Temporal.
- SLOs with alerting; synthetic transactions every minute in staging and prod.
- No PII in logs or traces; correlation IDs instead.

---

### 23. Testing strategy and quality gates

#### 23.1 Test levels

| Level | What | Tools |
|---|---|---|
| Unit | Money maths, policy, mappers | `go test`, Vitest, `opa test` |
| Property-based | Conservation, no negatives, idempotent replay, linked atomicity | `rapid` (Go), `fast-check` (TS) |
| Integration | Real Postgres, Kafka, TigerBeetle, Keycloak, Temporal | testcontainers |
| Workflow | Temporal replay tests on every workflow version | Temporal test suite |
| Contract | Protobuf compatibility, OpenAPI conformance | `buf breaking`, Schemathesis |
| Simulation | Rails, AD partner, KYC with fault injection | `tools/sim` |
| Isolation | No cross-entity imports, DSNs, credentials or writes | `make test-isolation` |
| Bank layer | Every `BankLayer` implementation passes the same contract suite; no route checks outside `pkg/banklayer` | `make test-banklayer` |
| Reconciliation | Three-way recon on generated scenarios | `make test-recon` |
| Consent | Denied without consent; stops after revocation | `make test-consent` |
| End-to-end | Customer journeys in staging | Playwright |
| Load | Ledger and API throughput | k6 (AGPL; dev tool, VERIFY policy) |
| Chaos | Failure injection | Chaos Mesh |
| Security | SAST, DAST, dependencies, secrets | Semgrep, OWASP ZAP, OSV-Scanner, gitleaks |

#### 23.2 Merge gates

All unit, integration and contract tests green; no new high-severity findings; money-path coverage at least 85%; every new topic or API has a schema and docs; every money-moving endpoint has an idempotency test and a failure-injection test.

#### 23.3 Promotion gates

- To staging: clean reconciliation for 7 days on synthetic traffic; chaos suite passes; load targets met.
- To production: external audit findings closed or accepted; sponsor and regulator sign-off (human); runbooks rehearsed.

#### 23.4 Human review checklist (money path)

1. Does it break any invariant in section 2?
2. Is every money-moving call idempotent, with tests?
3. Are failure and compensation paths tested?
4. Are policy checks, limits and audit entries present?
5. Does it cross an entity or tenant boundary?
6. Are amounts integers in minor units everywhere, including APIs and events?
7. Does it touch keys, permissions or perimeter rules?
8. Is every regulatory or vendor claim sourced and marked VERIFY?

---

### 24. Repository layout

```
forge/
  AGENTS.md
  Makefile
  docs/
    ARCHITECTURE.md
    BUILD_PLAN.md            # v1.1, superseded where noted
    QUESTIONS.md
    adr/
    specs/<service>/SPEC.md, THREATS.md
    runbooks/
    compliance/              # official schemas, VERIFY register
  proto/forge/
    assets/v1/  ledger/v1/  events/v1/  party/v1/  consent/v1/
    payments/v1/  fx/v1/  policy/v1/  recon/v1/
  api/openapi/               # edge REST specs
  config/
    assets.yaml  tiers.yaml  feature-flags.yaml
  policy/                    # Rego bundles + tests
  services/
    platform/  onboarding  party-service  consent-service  workspace-api
               consolidated-view  notifications
    bank/      ledger-gateway-bank  core-bridge  payments-orchestrator
               payouts-service  fx-service  rails-gateway
               partner-bank-router  reconciliation  treasury-risk  reg-reporting
    shared/    policy-service  compliance-engine  audit-service
               intercompany-settlement
    dasset/    (deferred) ledger-gateway-dasset  signer  onchain-watcher
               onchain-executor  travel-rule
    mfi/       (year 2) ledger-gateway-mfi  mfi-tenant-service  core-bridge-mfi
  pkg/                       # shared Go: money, idempotency, otel, authz, events
    banklayer/               # BankLayer interface, Capabilities, contract tests
      sponsor/               # SponsorBankLayer (v1)
      ownlicence/            # OwnLicenceBankLayer (stub, Stage B)
      sim/                   # SimBankLayer
  services/bank/stage-b/     # designed, not built: specs + interface stubs only
    prudential-reporting  direct-rails  deposit-insurance
    treasury-liquidity  tokenised-deposits  membership
  apps/
    web  backoffice  dev-portal  mfi-portal
  core/
    fineract/                # config as code, provisioning
    keycloak/                # realm exports, themes
  contracts/                 # deferred
  platform/
    helm/  tofu/  argocd/  kyverno/  observability/
  tools/
    sim/sponsor  sim/ad-partner  sim/partner-bank  sim/kyc
    loadtest/  recon-scenarios/
```

**Makefile targets:** `up`, `down`, `proto`, `lint`, `test`, `test-int`, `test-recon`, `test-isolation`, `test-consent`, `test-policy`, `test-banklayer`, `test-contracts`, `sim`, `load`, `demo-phase0`, `demo-phase1`, `ci`.

**Go conventions:** Go modules per service plus shared `pkg/`; connect-go for RPC; pgx and sqlc for Postgres; franz-go for Kafka (VERIFY choice); Temporal Go SDK; TigerBeetle Go client pinned to the server version; `pkg/money` with integer-only arithmetic and explicit rounding modes.

**Commits and branches:** conventional commits; trunk-based with short-lived branches; PRs under about 500 changed lines.

---

### 25. Build sequence for Claude Code

Workstreams: **A** ledger and core, **B** identity and onboarding, **C** payments and rails, **E** compliance and reporting, **F** apps, **G** platform and security, **Q** quality. Each task: write SPEC.md, tests, code, docs; `make ci` green.

#### Phase 0: foundations and spikes (weeks 1–6)

| ID | WS | Task | Sections | Acceptance |
|---|---|---|---|---|
| P0-01 | G | Scaffold monorepo per section 24; Makefile; CI skeleton; buf; lint configs; AGENTS.md v2.1; isolation check | 5, 21.6, 24 | `make ci` green on empty services; isolation check fails on a planted cross-entity import |
| P0-02 | G | Local stack (`make up`) per 21.4 with health checks | 21.4 | All components healthy; `make down` cleans up |
| P0-03 | A | `pkg/money`, asset registry proto and `config/assets.yaml` (ZAR and USD active) | 7.2, 18 | Property tests: no floats, overflow checks, scale handling |
| P0-03B | A | `pkg/banklayer`: interface, `Capabilities`, `SimBankLayer`, contract test suite, `forge.banklayer` OPA package, `config/banklayer.yaml` | 5.2 | `make test-banklayer` green on `SimBankLayer`; a CI check fails if any package outside `pkg/banklayer` reads the active route |
| P0-04 | A | ledger-gateway-bank spike: accounts, linked transfers, pending/post/void, idempotency records, outbox | 7 | Property tests prove invariants 1–5; replay test; kill-and-restart test; benchmark report |
| P0-05 | A | Fineract spike: products as code; core-bridge projection; nightly recon prototype | 8 | Report on throughput, failure handling and accuracy; ADR-0001 recommendation |
| P0-06 | C | RailSim, sponsor simulator and partner-bank (USD) simulator; ISO 20022 pain.001/pain.002/camt.053 handling | 12.1, 12.3 | Simulated payment accepted, delayed, rejected, returned; inbound USD credit simulated; messages validate against XSDs |
| P0-07 | A | Sponsor three-way reconciliation prototype on generated statements | 13 | Recon detects planted breaks; equation holds on clean data |
| P0-08 | E | OPA skeleton: limits, assets, entity, recon packages, with tests | 10 | `opa test` green; decision events emitted |
| P0-09 | G | Observability baseline | 22 | One trace spans gateway, policy and a workflow |
| P0-10 | Q | ADRs 0001, 0002, 0005, 0010, 0012, 0019–0024 drafted; ADR-0022 recorded as accepted | 26 | Each ADR has context, options, owner, status |
| P0-11 | Q | ForgePay and FORGE Custody audit for ADR-0021: what can sit behind which interface | 26 | Short report per component: reuse, adapt or replace, with gaps |

**Exit gate:** ADR-0001 accepted; `make demo-phase0` shows a simulated deposit, a payment and a clean reconciliation.

#### Phase 1: prototype (weeks 7–18)

| ID | WS | Task | Acceptance |
|---|---|---|---|
| P1-01 | A | Productionise ledger-gateway-bank: batching, authz, metrics, outbox relay, directory | 10k transfers/second on reference hardware in staging (VERIFY target); chaos tests pass |
| P1-02 | A | core-bridge projection and nightly GL recon | Zero drift across a 7-day soak; replay and out-of-order tests |
| P1-03 | B | Keycloak realms, passkeys, step-up, staff maker-checker | Auth journeys e2e tested; sensitive admin actions need two roles |
| P1-04 | B | party-service and consent-service | Founder/company separation tests; consent denial and revocation tests |
| P1-05 | B | onboarding with FakeKycProvider, then Didit sandbox | Business onboards; accounts created with insured flag; limits by tier |
| P1-06 | E | policy-service in the request path; compliance-engine v0 (sanctions, 10 monitoring rules, alerts API) | Fail-closed on stale lists; rules tested on synthetic scenarios |
| P1-07 | C | payments-orchestrator: `InboundCredit` and `DomesticPayment` on RailSim, all through `BankLayer` | Idempotent; compensation verified; workflow replay tests |
| P1-07B | C | `SponsorBankLayer` against the sponsor simulator; partner-bank-router with USD mirror and inbound USD on the simulator | `make test-banklayer` green for sponsor and sim; USD mirror recon green |
| P1-08 | A | reconciliation service: daily sponsor recon, breaks, circuit breaker via `forge.recon` | Planted break blocks large payments until resolved |
| P1-09 | E | audit-service, treasury-risk v0 | Hash chain verifies; concentration alerts fire on synthetic stress |
| P1-10 | F | web workspace MVP and back-office MVP | Playwright journeys pass; every balance shows entity and insurance status |

**Exit gate:** a business onboards, receives a simulated deposit, pays a domestic supplier, and everything reconciles across ledger, Fineract and the sponsor simulator.

#### Phase 2: pilot build (weeks 19–34)

| ID | WS | Task | Acceptance |
|---|---|---|---|
| P2-01 | C | Sponsor adapter against the sponsor's sandbox | Sponsor certification test cases pass |
| P2-02 | C | fx-service and `CrossBorderPayment` with the AD partner sandbox | Quote with disclosed spread; exchange-control data captured; failure paths tested |
| P2-03 | C | payouts-service (bulk contractor and supplier payouts) | Partial-failure handling; idempotent batches |
| P2-04 | C | USD account against the partner bank's sandbox; USD to ZAR conversion; USD payouts | Partner certification cases pass; mirror recon green; quotes disclose spread |
| P2-09 | Q | Stage B specs: `OwnLicenceBankLayer`, prudential reporting, direct rails, deposit insurance, treasury-liquidity, tokenised deposits, membership (sections 5.3, 5.4) | Specs reviewed; interface stubs compile; CI blocks wiring them in before their ADRs are accepted |
| P2-05 | E | compliance-engine v1, case SLAs; reg-reporting v1 (goAML drafts, evidence packs) | Test submissions validate against official schemas |
| P2-06 | G | Threat models; DR design; first recovery drill | Drill report with measured RPO and RTO |
| P2-07 | Q | Load, chaos and soak suites nightly in staging | Trend dashboards; no unexplained drift |
| P2-08 | F | Team permissions, approvals, bulk payment UI, statements | Maker-checker enforced; audit trail complete |

**Exit gate:** pilot-ready in staging; penetration test scheduled; sponsor and regulator conversations documented.

#### Phase 3: hardening and limited launch (weeks 35–52)

| ID | WS | Task | Acceptance |
|---|---|---|---|
| P3-01 | G | Production environment, GitOps, backups, runbooks | Dry-run release and rollback |
| P3-02 | Q | External audit and penetration test fixes | High and critical findings closed |
| P3-03 | E | Monitoring tuned on pilot-like data; live reporting | Reports generated from production-like data |
| P3-04 | G | Operations readiness: on-call, incident response, vendor management | Game days completed |

**Exit gate:** closed launch with about 20 design partners and tight limits, after licence, sponsor and regulator sign-offs (human decisions).

#### Year 2 (design now, build later)

MFI platform (section 17, including the apex option in 17.4), BaaS API, digital-asset entity (section 16) once licensed, cards via a rented issuer processor, intercompany settlement (section 14), EUR and GBP accounts.

#### Stage B (when the own-licence path is confirmed)

Build the section 5.3 components in this order: `OwnLicenceBankLayer`, deposit-insurance, prudential-reporting, treasury-liquidity, direct-rails, a rehearsal of the migration runbook, membership (only if mutual), then tokenised-deposits. With a 6–8 person team, expect about 12–18 months of engineering, with licence and certification queues setting the real date.

#### Engineering estimates

| Milestone | Solo founder + Claude Code | 2–3 engineers + Claude Code | 6–8 people + Claude Code |
|---|---|---|---|
| Phase 0 | 6–10 weeks | 4–6 weeks | 4–6 weeks |
| Phase 1 prototype on simulators | 4–7 months | 3–4 months | About 3 months |
| Phase 2 pilot build | Not realistic solo | 5–7 more months | About 4 more months |
| Phase 3 limited launch | Not realistic solo | 6–9 more months | About 4 more months |
| Total to closed launch | Prototype only | About 15–20 months | About 12 months |

These are estimates, not sourced figures. A sponsor-led launch is realistically 12–18 months of calendar time; an own licence is about 24–42+ months (VERIFY). The USD account adds about 2–3 months compared with a ZAR-only v1.

---

### 26. Architecture decisions (ADRs)

| ADR | Decision | Status | Needed by |
|---|---|---|---|
| 0001 | Fineract as projection vs in-house account service | Spike in Phase 0 | End of Phase 0 |
| 0002 | Hosting: sovereign colo, SA cloud region, hybrid | Open | Phase 0 |
| 0005 | Chain and stablecoin(s) | Deferred | Stage C |
| 0006 | Custody model | Deferred | Stage C |
| 0007 | HSM vendor or MPC provider | Deferred | Stage C |
| 0008 | Travel Rule provider | Deferred | Stage C |
| 0009 | Card issuer processor | Open | Year 2 |
| 0010 | Observability stack and licences | Open | Phase 0 |
| 0011 | Didit scope and fallback KYC provider | Open | Phase 1 |
| 0012 | Sector eligibility and risk appetite | Open (human) | Phase 0 |
| 0019 | Two-entity (multi-entity) structure | Accepted in principle | Phase 0 |
| 0020 | Bank layer legal form: mutual vs bank, and when | Open (human). Both routes supported behind `BankLayer`; current lean is sponsor now, with a mutual or co-operative structure for the MFI network later | Before Stage B |
| 0021 | Relationship to ForgePay and FORGE Custody | Open; audit in P0-11. Lean: keep TigerBeetle as bank ledger; reuse Revenue Ontology for analytics; Hyperswitch fork later for merchant acceptance; FORGE Custody for the digital-asset entity after its MPC audit | Phase 0 |
| 0022 | First customer group | **Accepted: technology companies**, with adjacent cross-border SMEs on the same product | Done |
| 0023 | MFI tenancy and data isolation model | Open | Year 2 |
| 0024 | Partner bank for USD accounts, and account model (named vs virtual) | Open | Phase 1 |

---

### 27. VERIFY register (human-owned)

1. Current SARB and FSCA position on stablecoins and tokenised deposits; IFWG findings on rand-pegged stablecoins.
2. Exchange-control treatment of cross-border flows and the AD partner's responsibilities (including BoP reporting).
3. FIC Act obligations, goAML schemas and submission process, sanctions lists, Travel Rule directive.
4. CODI coverage, limits and application to business deposits and tokenised deposits.
5. Mutual bank capital, licensing steps and PASA direct-participation rules.
6. Sponsor-bank interface formats and certification.
7. Didit coverage against FIC and POPIA.
8. POPIA: consent, cross-border transfer, retention, automated decisions.
9. Prudential Authority outsourcing and cloud requirements for the hosting choice.
10. National Credit Act registration for MFIs and any credit bureau.
11. Co-operative financial institution rules for MFIs taking member savings.
12. Licence and maintenance status of every component in Appendix A.
13. TigerBeetle version-specific behaviour: ID recommendations, batch limits, linked chains across ledgers, backup and recovery.
14. Sanctions data licences for commercial use.
15. Throughput and RPO/RTO targets against actual hardware.
16. Exchange-control rules for residents' foreign-currency accounts and inbound USD revenue; partner-bank candidates and terms.
17. Mutual Banks Act requirements for member shares, voting and distributions (only if ADR-0020 chooses a mutual).

---

### Appendix A: Open-source component register

All licences VERIFY before commitment. "Avoid" means do not adopt without an approving ADR.

| Layer | Component | Role | Licence | Notes |
|---|---|---|---|---|
| Ledger | TigerBeetle | Balances and transfers | Apache 2.0 | Pin client to server version |
| Core | Apache Fineract | Customers, products, loans, GL projection | Apache 2.0 | Multi-tenant; started as microfinance software |
| Workflows | Temporal (server + Go SDK) | Durable sagas | MIT | Self-hosted on Postgres |
| Events | Apache Kafka via Strimzi | Event streaming | Apache 2.0 | Avoid Redpanda (BSL) |
| Schemas | Protobuf, Buf, connect-go | Contracts and RPC | Apache 2.0 | |
| Schema registry | Apicurio Registry | Event schemas | Apache 2.0 | |
| Database | PostgreSQL 17, CloudNativePG | Service data | PostgreSQL, Apache 2.0 | |
| Go data | pgx, sqlc | Postgres access | MIT | |
| Kafka client | franz-go | Go Kafka client | BSD-3 | VERIFY |
| Cache | Valkey | Rate limits | BSD | |
| Identity | Keycloak | Auth, passkeys, OAuth2 | Apache 2.0 | Avoid ZITADEL (AGPL) |
| KYC/KYB | Didit | Verification | Commercial | Behind `KycProvider` |
| Policy | Open Policy Agent | Rules as code | Apache 2.0 | |
| Gateway | Envoy Gateway | Edge | Apache 2.0 | |
| Secrets | OpenBao | Secrets, transit encryption | MPL 2.0 | Avoid Vault (BSL) |
| Keys | SoftHSM2 (dev), PKCS#11 HSM (prod) | Signing keys | BSD / hardware | Deferred with DAST |
| ISO 20022 | moov-io/iso20022 or XSD codegen | Payment messages | Apache 2.0 | VERIFY fit |
| Sanctions matching | yente, or in-house | Screening | MIT | Data licences VERIFY |
| Monitoring rules | In-house, or Jube / Marble | Transaction monitoring | AGPL / VERIFY | Evaluate first |
| Orchestration | Kubernetes (Talos or RKE2) | Platform | Apache 2.0 | |
| Network | Cilium | Network policy, mTLS option | Apache 2.0 | |
| GitOps | Argo CD, Argo Rollouts | Delivery | Apache 2.0 | |
| Admission | Kyverno | Policy | Apache 2.0 | |
| Certificates | cert-manager | TLS | Apache 2.0 | |
| Secrets sync | External Secrets Operator | Secrets | Apache 2.0 | |
| Backup | Velero | Backups | Apache 2.0 | |
| IaC | OpenTofu, Helm | Infrastructure | MPL 2.0, Apache 2.0 | |
| Object storage | SeaweedFS or Ceph RGW | Documents, WORM | Apache 2.0 / LGPL | Avoid MinIO (AGPL) |
| Observability | OpenTelemetry, SigNoz or Prometheus/Tempo/Loki | Telemetry | Apache / MIT / AGPL parts | VERIFY Grafana-family terms |
| Search | OpenSearch | Logs, search | Apache 2.0 | |
| Analytics (later) | ClickHouse, dbt-core | Metrics | Apache 2.0 | |
| Frontend | Next.js, React, Tailwind | Apps | MIT | |
| Testing | testcontainers, rapid, fast-check, Playwright, Schemathesis | Tests | MIT / MPL / Apache | |
| Load and chaos | k6, Chaos Mesh | Load, chaos | AGPL / Apache | k6 is a dev tool |
| Security | Semgrep CE, Trivy, Grype, Syft, OSV-Scanner, gitleaks, cosign, OWASP ZAP, Falco | Supply chain and runtime | Mostly Apache / MIT / LGPL | |
| Chain (deferred) | viem, Foundry, OpenZeppelin, Safe, Slither, Echidna | Chain I/O, contracts | MIT / LGPL / AGPL (tools) | Stage C |
| Payments hub (later) | Mojaloop | Only if Forge runs a hub | Apache 2.0 | Deferred |
| Cards (later) | Rented issuer processor; jPOS | Issuing | Commercial / AGPL | Avoid jPOS without licence |

---

### Appendix B: Ledger codes

**Account codes (tb-bank)**

| Code | Name | Purpose | Flags |
|---|---|---|---|
| 1001 | CUSTOMER_DEPOSIT | Customer fiat deposit | `debits_must_not_exceed_credits`, `history` |
| 1010 | CUSTOMER_OVERDRAFT | Overdraft-enabled deposit (later) | `history`; limit via policy |
| 1020 | CUSTOMER_LOAN | Amount a customer owes on a loan or overdraft facility (debit balance) | `history`; lending-service only |
| 2001 | OPERATING | Forge operating | — |
| 2002 | FEES_INCOME | Fees and spread income | — |
| 2003 | FX_POOL | Per-asset conversion pool | — |
| 2004 | FX_POSITION | Net FX position | Monitored |
| 2005 | SUSPENSE | Unmatched receipts, repairs | Age alerts |
| 2006 | WRITE_OFF | Loss recognition | Dual control |
| 2007 | INTEREST_EXPENSE | Deposit interest paid to customers | deposits-service only |
| 2008 | INTEREST_INCOME | Loan interest charged to customers | lending-service only |
| 3001 | RAIL_SETTLEMENT_EFT | EFT settlement with sponsor | — |
| 3002 | RAIL_SETTLEMENT_PAYSHAP | PayShap settlement with sponsor | — |
| 3003 | SPONSOR_POOLED | Mirror of the sponsor pooled account | Reconciled daily |
| 3010 | AD_PARTNER_SETTLEMENT | Cross-border payouts via AD partner | Reconciled daily |
| 3020 | ACQUIRER_SETTLEMENT | Card takings due from the acquiring bank | Reconciled daily |
| 3030 | CARD_SCHEME_SETTLEMENT | Card purchases owed to the scheme or processor | Reconciled daily |
| 5001 | PARTNER_BANK_MIRROR | Balance at a partner bank, per currency | Reconciled daily |
| 5002 | REWARDS_POOL | Capped incentives (later) | `debits_must_not_exceed_credits` |
| 6001 | INTERCO_DUE_TO_DAST | Payable to digital-asset entity | Stage C |
| 6002 | INTERCO_DUE_FROM_DAST | Receivable from digital-asset entity | Stage C |
| 7001 | TOKENISED_DEPOSIT_LIABILITY | Outstanding tokenised deposits | Stage B only |
| 8001 | MEMBER_SHARE_CAPITAL | Member shares (mutual only) | Reserved; Stage B |
| 8002 | MEMBER_DISTRIBUTIONS_PAYABLE | Rebates and distributions owed to members | Reserved; Stage B |

**Transfer codes**

| Code | Name | Code | Name |
|---|---|---|---|
| 10 | DEPOSIT_IN | 50 | CARD_AUTH_HOLD |
| 11 | PAYMENT_OUT | 51 | CARD_SETTLE |
| 12 | INTERNAL_TRANSFER | 60 | PAYOUT_BULK |
| 13 | CROSS_BORDER_OUT | 61 | REWARD_CREDIT |
| 20 | FX_LEG | 62 | INVOICE_PAYMENT |
| 21 | FX_SPREAD | 63 | PERSON_ENTITY_TRANSFER (policy-approved) |
| 30 | FEE | 64 | PARTNER_BANK_SWEEP |
| 31 | RAIL_SETTLE | 70 | INTERCO_SETTLEMENT |
| 52 | ACQUIRING_SETTLEMENT | 65 | INTEREST_CREDIT |
| 66 | LOAN_DISBURSEMENT | 67 | LOAN_REPAYMENT |
| 68 | INTEREST_CHARGE | | |
| 40 | CHAIN_WITHDRAWAL (deferred) | 90 | REVERSAL |
| 41 | CHAIN_DEPOSIT (deferred) | 91 | ADJUSTMENT (dual control) |
| 80 | MEMBER_DISTRIBUTION (mutual only, reserved) | | |

Flag sets per code are finalised in ADR-0001 and `docs/specs/ledger-gateway-bank/ACCOUNTS.md`.

---

### Appendix C: Event catalogue (v1)

| Topic | Producer | Notes |
|---|---|---|
| `forge.bank.ledger.transfer.committed.v1` | ledger-gateway-bank | Every committed transfer |
| `forge.bank.ledger.hold.posted.v1` / `.voided.v1` / `.expired.v1` | ledger-gateway-bank | Two-phase outcomes |
| `forge.plat.party.onboarded.v1` / `.status_changed.v1` | onboarding, party-service | Drives accounts and limits |
| `forge.plat.consent.granted.v1` / `.revoked.v1` | consent-service | Stops data use on revoke |
| `forge.bank.payment.requested.v1` / `.settled.v1` / `.failed.v1` | payments-orchestrator | Saga lifecycle |
| `forge.bank.fx.quote.issued.v1` / `.booked.v1` | fx-service | Quote audit |
| `forge.bank.rail.message.sent.v1` / `.received.v1` | rails-gateway | ISO 20022 metadata |
| `forge.bank.recon.completed.v1` / `.break_opened.v1` / `.break_resolved.v1` | reconciliation | Drives circuit breaker |
| `forge.bank.risk.alert.v1` | treasury-risk | Concentration, outflows |
| `forge.shared.compliance.alert.raised.v1` / `.case.closed.v1` | compliance-engine | Back-office |
| `forge.shared.policy.decision.v1` | policy-service | Inputs, result, policy version |
| `forge.shared.audit.entry.v1` | all | Hash-chained by audit-service |

---

### Appendix D: Glossary

| Term | Meaning |
|---|---|
| AD partner | Authorised dealer in foreign exchange that executes FX and cross-border payouts and handles Balance of Payments reporting |
| BoP | Balance of Payments reporting for cross-border flows |
| CASP | Crypto asset service provider, licensed by the FSCA under FAIS |
| CODI | Corporation for Deposit Insurance |
| Entity | A legal entity with its own books (PLAT, BANK, DAST, MFI tenant) |
| Hold | A pending (two-phase) transfer that reserves funds |
| Ledger (TigerBeetle) | A numbered partition per asset; transfers cannot cross ledgers |
| Minor units | Smallest integer unit of an asset (cents for ZAR) |
| Saga | A multi-step workflow where each step has a compensation |
| Sponsor bank | A licensed bank holding deposits and providing rails until Forge is licensed |
| Sub-ledger | Forge's ledger of customer balances inside the sponsor's pooled account |
| Tenant | An MFI hosted on the platform, isolated from all others |
| Tokenised deposit | A licensed bank's own deposit liability represented on-chain (Stage B only) |
| VERIFY | A fact a human must confirm before it is relied on |


## Hosting

*Source: `docs/HOSTING.md`*

## Hosting: Fly.io for the preview, AWS for production

> **DRAFT, owner's direction of 2026-11-18 and later (Tebogo Mvelase): run on Fly.io for now, ultimately on AWS.**
> A direction, not a final decision: ADR-0002 closes only after the checks below. Everything about Fly.io and AWS
> here is from general knowledge and the Fly MCP guide, not tested by us; each point marked VERIFY needs checking.

### The rule that makes both safe

**Fly.io holds no real customer data and no real money.** It hosts the pre-licence preview: the local stack running
for demonstrations, sponsor-bank due diligence and the first sandbox runs against WorkOS and Didit, with synthetic
people only. Forge has no licence, so no customer money may be held anywhere anyway (`README.md`, Launch readiness).

The day a real customer's personal information goes in, the hosting becomes a material outsourcing and cloud
arrangement under Directive 3/2018, Guidance Note 5/2014 and Joint Standard 2 of 2024: due diligence
(`provider-due-diligence-procedure.md`), a contract meeting `provider-contract-schedule.md`, an entry in the
outsourcing register, and notice to the Prudential Authority at least 20 working days before it starts
(`scripts/check-outsourcing.py` computes the date). That day should be **AWS**, not Fly.io.

### Why AWS is the destination (VERIFY each)

- A South African region (Cape Town, `af-south-1`) keeps personal data in South Africa, which the data classes require
  for several kinds of data (`docs/regulatory/data-classes.csv`) and which POPIA and Directive 3 reward.
- Managed Kubernetes (EKS), Postgres (RDS or Aurora), Kafka (MSK), a secret store and a certificate authority exist
  as services, and AWS publishes independent audit reports a sponsor bank will recognise.
- Our manifests (`deploy/k8s`) are Kubernetes, so they apply to EKS with the 9 placeholders filled
  (`deploy/k8s/README.md`).

### What Fly.io is good for now

- Cheap, quick, close to the stack we already run: containers as Machines, private networking between apps, volumes,
  and a managed Postgres.
- A throwaway place to prove that the services start, find each other and pass the journeys somewhere that is not a
  laptop, before AWS is paid for.

### What Fly.io is not (VERIFY each)

- **Not the deployment target of `deploy/k8s`.** Fly runs apps and Machines, described by a `fly.toml` per app, not
  our Kubernetes manifests. Moving between them is a re-description of the same containers, not a rebuild.
- **Not a place for TigerBeetle in production.** It needs `io_uring`, dedicated hosts and a six-replica cluster;
  whether Fly's Machines allow `io_uring` must be tested before relying on it, even for the preview. Fallback: run the
  ledger on the in-memory model for the preview only (`LEDGER_STORE=memory`), which production refuses.
- **No managed Kafka or Temporal.** Either run them as Machines, or use hosted ones; neither is verified here.
- **A South African region** may or may not be offered; if not, the preview must stay synthetic-only.

### What this session can and cannot do on Fly.io today

The Fly connection in this session is signed in to your own Fly organisation, `tebogo-mvelase`, as admin, but its
grant is narrow:

| Can do | Cannot do |
|---|---|
| Create and read **Managed Postgres** clusters and databases; run read-only SQL | Create apps, deploy, or manage Machines, volumes or app secrets (no such scopes; the guide says deployment workflows and app creation are not included) |
| Read organisation and billing context | Anything outside this organisation |
| Create up to **3 Sprites** named with the prefix `a-card` (a sandbox product, not a place to run a bank) | Create a Sprite with any other name |

So the **database layer** could be created from here; **deploying the services** cannot, without something more:

1. **A Fly deploy token** for a Forge-only app group, given through the secure credential form (never pasted in chat),
   and the environment allowed to reach `api.fly.io` and `registry.fly.io` so `flyctl` can push images. This session
   has Docker, so it can build the images (`deploy/docker/service.Dockerfile`).
2. **Your say-so on cost**: every Machine and Postgres cluster bills.

**One question to settle first.** The Fly grant is named for A-CARD (Sprites prefix `a-card`), although the
organisation is yours. You ruled out A-CARD accounts for this project. Confirm whether `tebogo-mvelase` is the
organisation you want Forge in, or whether you will create a separate Forge organisation.

### Cost, and what is already in the Fly organisation

**Found, not created by Forge:** a Managed Postgres cluster named `durin-staging-db` (plan `dev-256`, Frankfurt, ready)
already exists in `tebogo-mvelase`, created on 2026-11-23 at 21:05 UTC, during this session but not by it. "Durin" is
not a Forge name, so the organisation is evidently shared with another project. That is a second reason to give Forge
its own Fly organisation. It was not touched, and nothing of Forge's should be created next to it until you decide.

**Prices Fly returns for Managed Postgres** (read from the account, US dollars a month):

| Plan | CPU | Memory | $/month |
|---|---|---|---|
| dev-256 | 1 shared | 256 MB | 4 |
| basic | 2 shared | 1 GB | 38 |
| starter | 2 shared | 2 GB | 72 |
| launch | 2 performance | 8 GB | 282 |
| scale | 4 performance | 32 GB | 962 |
| Performance | 8 performance | 64 GB | 1 922 |

One cluster can hold many databases, so the preview needs one cluster, not 20. **`starter` ($72) is the smallest I would
run all 20 service databases on.** Machine, volume and bandwidth prices were not available to me and are not in this table.

**Proposed ceiling for the preview: US$300 a month** (PROPOSED, for you to accept or change). Rough build-up, with
everything except the Postgres row an unverified estimate from general knowledge:

| Item | Estimate $/month |
|---|---|
| Managed Postgres `starter`, one cluster | 72 (from Fly) |
| 21 services, one small Machine each | 60 to 100 |
| Kafka, Temporal and a ledger Machine with a volume | 40 to 80 |
| Volumes, bandwidth, spare | 30 |
| **Total** | **about 200 to 280** |

Set a billing alert at 50% and 80% of the ceiling in the Fly dashboard, and stop the preview (scale to zero) outside
working hours. If the ceiling is not accepted, nothing is created.

### WorkOS and Didit

| | State | What it means |
|---|---|---|
| WorkOS connector in this session | Connected, but to the **A-CARD** team (signed in as the A-CARD address, projects FORGE, OpenFireblocks, Neo-Lloyds, OPEN PRIVY, A-CARD's Project). It already holds a project called "FORGE". | **Not used.** Your decision was a new WorkOS account on the Forge email. Nothing was read beyond who the connection is. |
| Didit connector | Reaches only A-CARD (recorded earlier). | **Not used**, not even to list. |
| `workos-forge-mutual` and `didit` in `.mcp.json` | Cannot connect: the session's network refuses `mcp.workos.com` and `mcp.didit.me`. | Needs the hosts allowed *and* your sign-in; see below. |

What you do, in this order:

1. Create the **WorkOS** and **Didit** accounts on the Forge email, if not done (`docs/PROVIDERS.md` has the steps).
2. At [claude.ai/customize/connectors](https://claude.ai/customize/connectors), add a connector for each, signing in
   with the **Forge** accounts, not A-CARD. If your claude.ai account already holds the A-CARD ones under the same name,
   remove or disable those first, or add the new ones under different names (for example "WorkOS Forge"). VERIFY that
   two connectors for one server can coexist.
3. **Start a new session** (connectors are read when a session starts). I will first confirm which organisation each
   one reaches, and stop if it is A-CARD.
4. Say yes to allowing the two hosts in this environment's network settings if the `.mcp.json` route is wanted; the
   claude.ai connector route needs no host change.

Then the work waiting on them starts: configure WorkOS AuthKit, build the Didit KYC and KYB workflows in its sandbox,
register the webhook, and run an onboarding end to end with simulated outcomes.

### AWS

The AWS connection in this session needs you to sign in again (`/mcp`, or the connector settings), and I do not know
which AWS account it reaches. Once it is back I will read who the caller is (read-only) and stop if it is not an account
you intend for Forge. Nothing is created in AWS without your say-so; a Terraform description of the landing zone
(`deploy/aws`: accounts, `af-south-1` network, EKS, RDS, MSK, TigerBeetle hosts, KMS, WORM storage, a private CA,
organisation guardrails) is **written and checked on paper**; it has never been planned or applied. The open question
it raises is disaster recovery: AWS has one region in South Africa as far as we know, so a geographically separate
site is a decision for the lawyer and the regulator (`deploy/aws/README.md`, finding 1).

### Plan

```mermaid
flowchart LR
  A["Now: local stack<br/>(everything works here)"] --> B["Fly.io preview<br/>synthetic data only<br/>demos, sponsor diligence,<br/>sandbox runs"]
  B --> C["AWS af-south-1<br/>staging, then production<br/>deploy/k8s + data layer"]
  D["Lawyer + PA notice<br/>20 working days"] --> C
  E["Sponsor bank signed,<br/>policies approved,<br/>pen test"] --> C
  C --> F["Pilot with real customers"]
```

| Step | Who | Needs |
|---|---|---|
| Choose the Fly organisation (a separate Forge one is recommended) and the cost ceiling (US$300 proposed) | You | A decision; nothing is created until then |
| Fly deploy token; host allowlist | You (credential form, environment settings) | The token, approval of the settings change |
| Build images; create preview apps and Postgres | Me | The two above |
| Reconnect AWS; confirm the account | You, then me | Sign-in |
| Terraform for the AWS landing zone, on paper | Me | **Done**: `deploy/aws/` |
| PA notice for AWS production hosting | CCO (vacant), lawyer | Appointments; a start date |


## Project status (README)

*Source: `README.md`*

## FORGE-MUTUAL

FORGE Mutual Bank: the bank platform behind Forge Technologies. Source of truth for scope and design:
`docs/CONTEXT.md` (`docs/CONTEXT.md`) and `docs/ARCHITECTURE.md` (`docs/ARCHITECTURE.md`). Open questions and
items that need a human decision are in `docs/QUESTIONS.md` (`docs/QUESTIONS.md`).

> **Status: concept, pre-licence.** Forge is unlicensed and has no customers. Nothing here moves real money.
> Every external integration is either a simulator or unverified (see *Blocked* below).

### Launch readiness

**Not ready to take real customers.** The software is built, tested and runs end to end on a local stack, including
the customer front door: a person or a company signs in, applies, verifies and gets an account. What stands between
this and a launch is mostly not code: a banking licence or sponsor bank, accounts with live providers, owner decisions,
production infrastructure and an independent security review. Assessed 2026-11-23.

**Scorecard.** Regulatory register (`docs/regulatory/register.csv`, 114 requirements): 100 Partial, 10 Open, 4 waiting
on an external party, none fully met. The 10 open rows all wait on a person, capital or a hosting choice. Governance pack (`docs/governance/`): 58 draft policies, none approved;
37 seats, 1 named (founder Tebogo Mvelase), 36 vacant. Thresholds: 31 placeholders, none with a named owner. Legal form: mutual bank planned, bank kept open (`docs/governance/LEGAL_FORMS.md`); both built into the code. Licence: none.
Production infrastructure: none. Software gate (`make ci-db`): passing.

#### Hosting

Fly.io for the synthetic-data preview, AWS (Cape Town proposed) for production: `docs/HOSTING.md` (`docs/HOSTING.md`).
The AWS landing zone is written as Terraform and checked on paper, never applied: `deploy/aws` (`deploy/aws/README.md`).

#### The model

How the bank is shaped by Erebor (technology) and Investec (service), and where it differs: `docs/BANK_MODEL.md` (`docs/BANK_MODEL.md`).


#### Readiness by stage

| Stage | Where we are | What is missing |
|---|---|---|
| **Conversations** (sponsor banks, a regulatory lawyer, the Prudential Authority's pre-application meeting, investors, first hires) | **Ready to start.** A working platform, a governance pack (structure, 37 seats, 58 draft policies), a 114-line regulatory register, a bank-model note, a legal-forms comparison (mutual planned, bank open), a sponsor pitch, a lawyer brief and a risk appetite worksheet. | The founder's own fit-and-proper file; real financial projections (the pro formas use illustrative figures); a named lawyer and a sponsor shortlist; the owner's answers to the risk appetite worksheet. |
| **Licence application** (Prudential Authority registration as a mutual bank or a bank) | **Not ready.** Policies and structure are drafted, but nothing is approved. For the mutual route the official forms are now in hand (27 DI forms, from the PA): DI 100, DI 200, DI 110, DI 300 (liquidity ladder), DI 310 (liquid assets), DI 400 (capital), DI 410 (interest-rate risk), DI 500 (credit risk), DI 505 and DI 510 (large exposures), DI 600 (currency risk), DI 700 (investment limits), DI 900 (institutional breakdown) and DI 930 (interest rates) are built on the forms' own line numbers, the DI 099 control sheet is built, 9 forms are nil returns and 3 do not apply; none is left to build (`docs/regulatory/returns/catalogue.csv`). The line transcription still needs a second person's check. | Capital (R10m mutual, R250m bank); at least 7 founding members for a mutual; a board with independent directors; fit-and-proper filings for every director and executive; approved policies; a business plan with real projections; the lawyer's advice on the legal form. Typically 18 to 24 months or more from a complete application (general knowledge; confirm with the lawyer). |
| **Live launch** (real customers, through a sponsor bank first) | **Not ready.** The software passes its gate on a local stack, with every external party simulated. | A signed sponsor bank; the MLRO, risk and compliance heads; approved policies; production hosting and a Prudential Authority notice 20 working days ahead; a penetration test; live WorkOS and Didit; a licensed sanctions list; lawyer-reviewed terms and privacy notice; FIC registration; a pilot. Roughly 6 to 12 months if the first steps start now (general knowledge). |

#### Licence application pack

`docs/licence-application/PACK.md` (`docs/licence-application/PACK.md`) is generated (`make licence-pack`) from the regulatory register: 18 sections, 59
requirements, each with its owner, evidence and gap. It is an index of what the application needs, not the application.
Two working drafts sit beside it, both for the lawyer and the founder to correct and neither approved: a
draft constitution (`docs/licence-application/CONSTITUTION_DRAFT.md`) (20 decisions listed; the Act's own provisions still to be added) and a
draft business plan (`docs/licence-application/BUSINESS_PLAN_DRAFT.md`) (projections, capital plan, board and auditor are gaps).

#### Policies against the regulator's structure

`docs/governance/POLICY_MAP.md` (`docs/governance/POLICY_MAP.md`) places all 58 draft policies under 17 areas: the seven prudential standards the PA has
announced for mutual banks (not yet published) and the instruments already in force (Mutual Banks Act and regulations, Joint Standards, guidance
notes, FICA, CODI, POPIA). Three gaps it found are now drafted: large exposures, deposit insurance, and membership and surplus.

#### Governance and people

The board, committees, executive chart, three lines of defence and every seat the bank needs are drawn in
`docs/governance/STRUCTURE.md` (`docs/governance/STRUCTURE.md`). The roster of 37 seats with what each needs is
`docs/governance/ROLES.md` (`docs/governance/ROLES.md`); everything still needed (people, decisions, money,
registrations, providers, documents) is the checklist in `docs/governance/README.md` (`docs/governance/README.md`).

#### What is left before real customers

Ticked items are done. Each open item says who must act: **Owner** (a decision or an account only a person can make),
**External** (a third party) or **Engineering** (work in this repository).

**1. Licence and regulators: the hard gate**

- [ ] **Owner/External.** A licence or a sponsor bank. No customer money may be held until one of the two is in place.
  ADR-0020 (legal form of the bank layer) is open.
- [ ] **External.** Sponsor bank sandbox and wire formats (VERIFY-SPONSOR), partner and FIC formats (VERIFY-PARTNER,
  VERIFY-FIC), and the goAML schema (VERIFY-GOAML; submission is refused until it is confirmed).
- [ ] **Owner/External.** Regulatory review of the policy, screening and reporting rules.
- [ ] **Owner.** Name an owner for each of the 31 placeholder thresholds (`docs/THRESHOLDS.md`), and decide ADR-0012
  (sector appetite).
- [x] Daily re-screening of every customer: a match opens a critical case and freezes the customer.
- [x] A per-person annual foreign allowance on FX (threshold 28), monthly credit statements and default notices
  (threshold 27), and a data-classification register checked by the gate.
- [ ] **Owner.** Choose and license the sanctions list source (`docs/SANCTIONS_SOURCES.md`). Screening runs on a
  one-name fixture today.

**2. Identity: sign-in and KYC/KYB**

- [x] Sign-in through OIDC, built and tested against a local identity provider.
- [x] The Didit adapter, built from Didit's documentation; its webhook signature matches Didit's reference code.
- [x] Both MCP servers are in `.mcp.json` (`workos-forge-mutual`, `didit`).
- [ ] **Owner.** Sign in to the WorkOS and Didit MCP servers with the new Forge account (not A-CARD): run `/mcp` in
  Claude Code. The Didit connection on the cloud sessions reaches only A-CARD and must not be used.
- [ ] **Engineering, once connected.** Configure WorkOS AuthKit (redirect addresses, roles, a `forge_party` claim for
  approved applicants). Build the Didit KYC and KYB workflows in its sandbox, register the webhook, and run an
  onboarding end to end against Didit with simulated outcomes. Then repeat in Didit's live application.

**3. Customer front door**

- [x] Public application endpoints (`POST /v1/applications/person|business`, `GET /v1/applications/{kind}`), for
  signed-in applicants only, rate-limited, with no decline reason shown (no tipping-off).
- [x] The `OnboardPerson` workflow: personal account, KYC, screening, account opening.
- [x] Apply, status and welcome pages, restyled; Playwright and accessibility tests.
- [x] A personal application journey on the running stack (`make test-stack`): sign in, apply, verify, approved,
  account opened in the ledger and shown to its owner; and a decline, shown without a reason. The one stand-in is the
  identity provider, whose signed verdict the test delivers until Didit is connected. It found two bugs, both fixed:
  every personal application stopped at verification, and the stand-in provider gave no verification link.
- [x] The onboarding SPEC, the workspace-api SPEC and its OpenAPI describe the application routes and `OnboardPerson`.
- [x] The signed-in customer pages share the front door's style.
- [ ] **Engineering.** Give an approved director a login, and create the Fineract client the design lists.
- [ ] **Owner.** Terms and conditions, privacy notice and fee schedule, reviewed by a lawyer, linked from the forms.
- [ ] **Owner.** Decide how soon an account reaches its owner. Today reconciliation names the owner every 15 minutes
  by default.

**3a. Products beyond v1: built, and off in production until approved**

Each is a working service on the local stack, tested against Postgres, and refused in production by `pkg/licence`
until `FORGE_APPROVAL_<PRODUCT>` names the decision that approved it (`docs/DECISIONS.md`). Their rates, fees, limits
and affordability rule are placeholders 17 to 21 in `docs/THRESHOLDS.md`.

- [x] **Currencies:** USD, EUR and GBP accounts at the partner bank, one per currency, with the reference payers
  quote; FX between the rand and each; payments received from abroad booked and recorded for SARB cross-border
  reporting with the reason the customer gives (`services/bank/payments-orchestrator`, `onboarding` OpenCurrencyAccount).
- [x] **Savings and notice deposits:** daily interest, paid monthly; 32- and 90-day notice (`services/bank/deposits-service`).
- [x] **Credit:** overdrafts, working-capital loans and revenue-based financing, decided by a credit officer in the back
  office (`services/bank/lending-service`).
- [x] **Card payments:** payment links and point-of-sale requests, next-day settlement, refunds; card details never
  reach Forge (`services/bank/acquiring-service`).
- [ ] **Owner/External.** For currencies: an authorised-dealer arrangement and the SARB reporting format and channel
  (VERIFY-FINSURV); nothing is submitted yet.
- [ ] **Owner/External.** For savings: treasury and regulatory sign-off of the products and rates.
- [ ] **Owner/External.** For credit: a credit policy and committee, NCA registration, a credit bureau, capital.
  Built, against simulators and with draft wording: a credit bureau check (consent, fail closed, an adverse record needs a written
  reason), the NCA pre-agreement statement (generated, acknowledged by hash), and write-offs (two people, after a default notice, 90 days
  past due). Not built: a real bureau, collections beyond retrying, recoveries after a write-off.
- [x] **Card issuing:** virtual debit cards with real-time approval against the ledger (`services/bank/card-service`),
  tested against a processor simulator; wired to the customer app (`/cards`) but not switched on (its navigation link appears only while it is on).
- [x] **Third-party offers** (`services/platform/partner-offers`): introduces investment and crypto-asset products that a licensed
  provider supplies. **Every product is provided, licensed and monitored by a third party, never by Forge**, and the code states
  it in every offer. Wired to the customer app (`/invest`, with a consent step) and to a compliance screen in the back office (`/partners`), but **switched off**; no provider chosen. The navigation links appear only while the product is on. Built behind that switch: a signed, consent-checked sender of the customer's name to the provider, an automatic licence re-check against a regulator register extract (suspends on mismatch, never trusts a stale file), and a referral-fee ledger entry booked only against a bank statement line, within the cap the customer was told, by two people. The real regulator register source and a real provider are not yet available.
- [ ] **Owner/External.** For card issuing: a programme manager or sponsor, a processor, and the regulator's view.
- [ ] **Owner/External.** For card payments: an acquiring bank (VERIFY-ACQUIRER) and its adapter; chargebacks;
  reconciliation of acquirer payouts.

**4. Production infrastructure: none exists yet**

- [x] Kubernetes manifests for the 23 services (`deploy/k8s`): mutual TLS certificates, caller lists, network policies,
  hardened pods, health probes. Generated and checked on paper; **never applied to a cluster**.
- [ ] **Owner/Engineering.** A cloud account and region (ADR-0002), and the data layer and secret store the manifests
  expect (`deploy/k8s/README.md` lists 9 things still to decide). Nothing provisions production yet.
- [ ] **Engineering.** A production TigerBeetle cluster (six replicas on dedicated hosts), measured, with an upgrade
  rehearsal.
- [ ] **Engineering.** Production Temporal, Kafka (TLS and SASL, more than one broker) and Postgres (managed, with TLS).
- [ ] **Engineering.** A secret store delivering the `*_FILE` secrets; a certificate authority issuing the service
  certificates.
- [ ] **Engineering.** Off-site, immutable storage for backups, the WAL archive, audit anchors and evidence packs
  (VERIFY-WORM).
- [ ] **Engineering.** A tracing backend, metrics, dashboards, alerts and an on-call rota.
- [x] A health listener that needs no client certificate (`HEALTH_ADDR`, `/livez`, `/readyz`), tested in `pkg/serve`.

**5. Assurance before go-live**

- [ ] **External.** An independent security review and penetration test.
- [x] Returns: balance sheet and income statement for both forms (BA 100/120, DI 100/200) from the ledger, with
  auditor sign-off; pro formas from projections (`make proforma`). Other forms not built.
- [x] Outsourcing notice dates, provider contract clauses, a business impact analysis and a security findings
  register, each checked by the gate.
- [ ] **Engineering.** A load test through the whole payment path, on production hardware.
- [ ] **Owner/Engineering.** An incident and recovery drill on production: a restore, the loss of a site.
- [ ] **Owner.** A pilot with a small number of friendly customers before opening applications to the public.

#### Next steps, in order

1. **Owner:** sign in to the WorkOS and Didit MCP servers with the Forge account.
2. **Engineering:** configure WorkOS and Didit, and run the first live sandbox onboarding.
3. **Owner:** name the threshold owners (a mock roster with hiring advice is in
   `docs/THRESHOLD_OWNERS.md` (`docs/THRESHOLD_OWNERS.md`)), choose the sanctions source, and start the licence or sponsor conversation.
4. **Engineering:** production infrastructure as code, then the production clusters, measured.
5. **External:** sponsor sandbox, security review, then a pilot.

**Verified**

- The full gate (`make ci-db`) passes on the head of the branch, last run 2026-11-23: lint, 83 test packages against real Postgres 17, Kafka 3.9 and TigerBeetle 0.16.60, the
  entity-isolation check and the caller-list check.
- 477 Go test functions in 37 modules. Playwright: 70 tests across the two apps, including the accessibility scans
  and the security-header checks.
- The local stack starts from nothing in one command (`make up`). `make test-stack` passes on a fresh stack: a customer
  onboarded through the real workflow sees the account it opened, and a compliance person decides a real review and
  the onboarding completes.
- The Didit adapter's webhook signature reproduces Didit's own published reference code on over 180 vectors, and the
  onboarding suites pass against a real Temporal dev server (`make test-temporal`, which the gate does not run), including
  a whole onboarding completed by Didit-signed deliveries. It has **not** run against Didit itself.
- **The ledger runs on TigerBeetle 0.16.60**, and the gate tests against it on every run. The in-memory model that every
  other suite relies on is held to the real ledger: the same random batches go to both, and every outcome, balance and
  stored transfer must match (48,000 batches on four seeds, after the test found and fixed four differences). A
  three-replica cluster was taken apart while in use: it survived losing its primary, used a restarted replica to
  survive losing another, accepted nothing without a quorum, and applied a transfer retried across the outage exactly
  once. On the local stack a balance survived a restart of the ledger service and of TigerBeetle. All of this is a
  development-mode cluster on one machine (`services/bank/ledger-gateway-bank/SPEC.md` section 12).
- **Mutual TLS runs end to end on the stack** (`deploy/compose/up.sh mtls`, `make test-stack-mtls`). Every service
  speaks TLS and requires a client certificate, and the journeys pass through it, browser to ledger. Checked against the
  running services: a caller with no certificate, or one from another authority, gets no answer; the caller is the
  service its certificate names, and a header that says otherwise is ignored; each of the two APIs for people serves
  only its own web app's server. Every certificate was renewed on disk while the stack ran, and every service presented
  its new one within seconds, with no restart.
- **Caller lists are least privilege, and stay that way.** Each service may call only the methods its code calls: 83
  unneeded grants were removed (reconciliation could create transfers and approve payments; every reader of the ledger
  could write to it). `make ci` fails on a grant the caller's code does not use (`tools/isolation`).
- Production refusals, request IDs and security headers are unit-tested (`pkg/serve`, `pkg/callerauth`).
- **No shipped code calls a known vulnerability** (`make vulncheck`, part of the gate): every Go module is scanned
  against the Go vulnerability database and both apps against npm's. Getting there meant moving from Go 1.24.7 (out of
  support, 41 reachable standard-library vulnerabilities) to Go 1.27.4, and upgrading pgx, klauspost/compress,
  golang.org/x/net, OpenTelemetry and gRPC.
- **Secrets come from files, not the environment.** Every secret a service reads (database URLs, encryption keys,
  webhook secrets, API keys) may be given as `NAME_FILE`, a mounted file, which is how Kubernetes secrets, Vault Agent
  and cloud secret drivers deliver them. Production refuses to start with any of them in the plain environment. On the
  stack, the generated keys are files, and `docker inspect` shows only their paths.
- **Traces reach a collector and follow a request across services.** On the stack all 18 services export spans to
  an OpenTelemetry collector. The trace context crosses every service call and goes into Temporal workflows and out
  through their activities, so one onboarding is one trace through onboarding, party, consent, policy and the
  ledger. Not done: a tracing backend to read them in, and sampling settings for production volume.
- **Point-in-time recovery works.** The stack's Postgres archives every WAL segment (at least once a minute), and a
  backup service takes a verified base backup on a schedule and keeps 14 days. `scripts/pitr-check.sh` writes rows,
  picks a moment, then adds a row and drops a table, and restores a fresh copy to that moment: the rows from before
  are there, the later row is not, and the dropped table is back. Passed twice on the running stack.
- **Kafka survives losing a broker, and refuses writes it cannot make durable.** Against three brokers (three
  replicas, two in sync required) the outbox relay lost a broker mid-run with no event lost or reordered; with two of
  three down it refused to publish and left the events in the outbox; on recovery it delivered them. That test found
  the relay hung for ever, inside its database transaction, when the brokers had no quorum: the Kafka client retried a
  record with no deadline, and its own timeout did not stop it once the brokers had lost their controller majority.
  Both publishers now return at the caller's deadline whatever the client does, and the relay retries on its next pass.
  Passed four runs in a row (`make test-kafka-cluster`).
- **Only the page's own scripts run.** Both apps send a Content-Security-Policy with a fresh nonce on every page, and
  every script Next.js renders carries it; `'unsafe-inline'` is gone from `script-src`. A script tag injected into a
  page's HTML does not run (tested in both apps; those tests fail under the old policy). Styles keep
  `'unsafe-inline'`, since React sets style attributes; an injected style cannot run code.
- **Under load, and through crashes, no money is made, lost or moved twice** (`make load-ledger`). 32 clients sent
  about 400 transfers a second for a minute to the ledger on TigerBeetle, with refusals, retries of the same request,
  the ledger service killed and then TigerBeetle killed. Afterwards every account held exactly what the accepted
  transfers say, no customer was negative, and the balances summed to zero; every retry got the first answer back.
  p50 about 60 ms, p99 under 200 ms on one shared machine, with development-mode TigerBeetle.
- Both web apps are on Next.js 16.4.6. 16.4.1 was in the range of a published advisory (GHSA-mfr8-8qww-v76h, moderate;
  the apps do not use the affected feature). `npm audit --omit=dev` reports no known vulnerability in either app.
- A KYC decision is recorded before the onboarding is told, and is redelivered until it is. Shown on the stack: with
  Temporal stopped, a decision was recorded and the reviewer was told it would be retried; once Temporal was back, the
  onboarding completed within 20 seconds with no further action.

**Blocked by people** (`docs/DECISIONS.md`, `docs/QUESTIONS.md`, `docs/THRESHOLDS.md`)

- Threshold owners: the 31 placeholder thresholds each need a named owner who confirms them. None is named.
- Sanctions list source and licence: undecided. Screening runs on a one-name fixture.
- Identity provider: decided on 2026-11-03 (WorkOS AuthKit), but the account does not exist and nothing has run against
  it. The apps sign in with development sign-in only.
- Didit for KYC and KYB: decided on 2026-11-03, in a new account the owner creates. The organisation does not exist. The
  adapter is built from Didit's documentation and tested without an account (its webhook signature is checked against
  Didit's own reference code), but it has never run against Didit, so the local stack uses the fake provider. The
  owner's steps are in `docs/PROVIDERS.md`.
- ADR-0020 (legal form of the bank layer) and ADR-0012 (sector appetite): open human decisions.
- goAML schema (VERIFY-GOAML): submission is refused until it is confirmed.
- WORM storage for audit anchors and evidence packs (VERIFY-WORM): local files only.
- Regulatory review of the policy and reporting rules: not started, and not in this repository.

**Blocked by external systems**

- TigerBeetle runs, in development mode only (since 2026-11-16; see *Verified*). Not done: a production cluster (six
  replicas on dedicated hosts, without `--development`), its throughput, an upgrade rehearsal, and the repair rate of a
  restarted replica on that hardware.
- Sponsor bank: no sandbox, and the wire format is unverified (VERIFY-SPONSOR). Partner and FIC formats too
  (VERIFY-PARTNER, VERIFY-FIC).
- Kafka: verified as a three-broker cluster in tests, but the stack runs one broker, and TLS and SASL to the brokers
  are not set up.
- Temporal runs as a single development server. No production cluster is set up.
- core-bridge's posting path was checked once against a live Fineract. It is not on the stack.

**Engineering gaps**: see *What is left before real customers* above.

### Built and tested

Each service is its own Go module. `make ci-db` creates throwaway databases on the compose Postgres 17, then runs the
full gate: lint, every module's tests against real Postgres and a real Kafka broker, and the isolation check. Every
database-backed suite runs; the only opt-in suite is the live Fineract test, which needs `FINERACT_TEST_URL`.

- **Ledger gateway** (`services/bank/ledger-gateway-bank`): the only writer to the bank ledger. Linked transfer chains,
  pending, post and void, idempotency, transactional outbox, and `ListAccounts` over the account directory. Passes one
  shared behaviour suite in memory, against Postgres and against TigerBeetle. A request the ledger refused stays
  refused if it is retried after its answer was lost (`PREVIOUSLY_REFUSED`), and a ledger call gives up at a deadline
  instead of waiting for a cluster that has no quorum.
- **Policy service** (`services/shared/policy-service`, `policy/`): embedded OPA v1.13.0, Ed25519-signed bundles, every
  decision audited to the outbox before it is returned. The break rule is strict: while any reconciliation break is
  open, every payment is held (`docs/DECISIONS.md`).
- **Party and consent services** (`services/platform/party-service`, `consent-service`): national IDs encrypted
  (AES-256-GCM) with a keyed index, founder and company separation enforced, consent checked live.
- **Onboarding** (`services/platform/onboarding`): Temporal workflow `OnboardBusiness`, tested on a real Temporal dev
  server and run on the local stack. A provider's REVIEW verdict parks the workflow until a compliance person decides
  it; the decision is recorded once, then delivered to the workflow until it has been told. On the local stack KYC uses a **fake provider**. The Didit
  adapter is built, with a recorded session per subject and signed-webhook verification, and tested without an account; it has never run
  against Didit (see *Blocked*).
- **Rails gateway** (`services/bank/rails-gateway`) and **payments orchestrator** (`services/bank/payments-orchestrator`):
  domestic payment saga with reserve, initiate, settle or release, and repair. Settlement posts the hold, then records
  the sponsor's debit as a clearing leg into the pool. Each rail status is recorded when it is seen. Inbound credits
  are booked once, to a customer or to suspense. Rails are simulated.
- **Reconciliation** (`services/bank/reconciliation`): five checks per asset. A moving ledger makes a run inconclusive.
  The in-flight terms come from payments: inbound credits the bank has reported but the books do not yet hold, and
  outbound payments the bank has settled but the books have not yet cleared. The gate fails closed and is tested
  against Postgres.
- **Audit service** (`services/shared/audit-service`): append-only hash-chained log. Triggers refuse update, delete and
  truncate. Head anchors detect a full rewrite that reseals every hash. `forge-audit verify` checks the chain.
- **Treasury risk** (`services/bank/treasury-risk`): concentration, net outflow, liquidity and suspense alerts. Each
  alert is raised once and cleared once, with one outbox event per transition. Thresholds are placeholders.
- **Outbox and Kafka** (`pkg/outbox`): the relay publishes to a real broker. Tests show that events arrive in order, and
  that during a broker outage every event stays unpublished for the next pass.
- **BankLayer** (`pkg/banklayer`): the interface, a sponsor simulator, an HTTP client, a contract suite that the
  simulator and the client both pass, config-driven bootstrap that refuses own-licence, and a CI check that only
  `pkg/banklayer` may read the licence route.
- **Core bridge** (`services/bank/core-bridge`): mirrors committed ledger transfers into Apache Fineract. Before a retry
  it looks up the reference, so a lost response cannot post the same journal twice. Reconciles 14 general-ledger
  accounts (seven codes in ZAR and USD) against the ledger, and refuses to run while a posting is pending. The suite
  runs against a stateful Fineract fake. The posting path was also checked once against a live Fineract.
- **Partner-bank router** (`services/bank/partner-bank-router`): routes USD operations by partner health and cost. A
  payout is pinned to the partner it was first sent to, so a retry never reaches a second partner. Partners are
  simulated.
- **Consolidated view** (`services/platform/consolidated-view`): balances and activity per party, projected idempotently
  from transfers and reconciled to the ledger. Drift is counted, not hidden. A party sees only its own accounts.
- **Notifications** (`services/platform/notifications`): email and SMS from platform events. Contacts are sealed with
  AES-256-GCM. Failed sends are retried, then marked failed. Only a development file sink exists; no provider is chosen
  (VERIFY).
- **FX service** (`services/bank/fx-service`): quotes with a disclosed spread, rounding against the customer, a 60-second
  single-use quote lock, and a saga across two ledgers with holds, reversal and a resume loop. One worker runs a deal at
  a time, enforced by an advisory lock. The rate is static configuration (VERIFY).
- **Payouts** (`services/bank/payouts-service`): bulk ZAR payouts. A checker other than the maker approves the batch. Each
  item is sent to payments under a key derived from the batch and the item, so a retry or a resume cannot pay it twice. A
  refused item is final; an unreachable one is marked uncertain and sent again on resume. Tested against Postgres and in
  Temporal's test environment. **Not yet run against a Temporal server** (see its SPEC).
- **Workspace API** (`services/platform/workspace-api`): the customer REST surface, with the OpenAPI contract in
  `openapi/workspace.yaml`. The party comes from the session and request bodies are strict. Only development sign-in
  exists. Its downstream calls are tested with fakes, and it runs against the real services on the local stack.
- **Back office** (`services/shared/backoffice-api`, `apps/backoffice`): staff tools for compliance cases (close as a second
  person), payment approval and repair, and the reconciliation gate and breaks. Each action checks the role before any
  downstream call. The cases service is part of compliance-engine and is tested against Postgres. Development sign-in only.
  The app has a KYC review screen, which decides real reviews on the local stack. Twenty-two Playwright journeys pass
  against the production build, including the accessibility and security-header checks. One more runs against the
  local stack with nothing mocked.
- **Customer web app** (`apps/web`): accounts with entity and insurance on every balance, activity, payments, FX with a
  disclosure that must be accepted, and bulk payouts with maker-checker. Twenty-two Playwright journeys pass against the
  production build, with the API mocked in the browser. Two run against the local stack, where an account opened
  by a real onboarding is visible to its owner. Development sign-in only. See its SPEC.

### Partly built

- **Compliance engine** (`services/shared/compliance-engine`): a sanctions loader that checks integrity and freshness
  and fails closed, monitoring rules over the event bus that open cases, and a case store with maker-checker and SLA
  timers, served to the back office. Cases open from settled payments and customer credits on the local stack. The
  rules' thresholds are placeholders (`docs/THRESHOLDS.md`). The list source is a recommendation
  (`docs/SANCTIONS_SOURCES.md`), not a decision; the local stack uses a one-name fixture.
- **Reg-reporting** (`services/bank/reg-reporting`): immutable evidence packs with stable digests and maker-checker.
  goAML submission is **deliberately refused** until its schema is verified (VERIFY-GOAML). No report builders yet.

### Not started (v1 scope)

Every v1 service in `docs/ARCHITECTURE.md` (`docs/ARCHITECTURE.md`) section 6 now exists, and the back office has its
KYC review screen and live cases. The customer application flow and the `OnboardPerson`
workflow were built on 2026-11-17. Not yet built: moving the payments orchestrator's rails from the rails-gateway simulator onto
`BankLayer`, and the core-bridge on the local stack (it needs a live Apache Fineract there).

### Production configuration

Done and tested: TLS and mutual TLS between services, on the running stack as well as in unit tests (`pkg/serve`:
production refuses to start without certificates, the caller is the service named in its certificate and not a header,
a renewed certificate is loaded without a restart, and the two APIs for people serve only the web apps named in
`WORKSPACE_CLIENTS` and `BACKOFFICE_CLIENTS`, which production requires); each web app's server presents its own
certificate to its API and refuses a plain-HTTP API address in production; request IDs, timeouts and security headers on
every service and both web apps (the Content-Security-Policy runs only scripts carrying a per-request nonce); a
production refusal of every development switch (memory ledger, simulators, unsigned policy, fake KYC); OpenTelemetry
spans, with OTLP export when `OTEL_EXPORTER_OTLP_ENDPOINT` is set; a backup and restore check that proves a dump
restores (`scripts/pg-backup.sh`, `scripts/pg-restore-check.sh`); and an axe accessibility scan of every page in both
apps, failing on serious and critical violations.

Not done: under mutual TLS a health check needs a certificate,
because there is no separate health listener; no secret store is wired to deliver the secret files. Backups are
scheduled and point-in-time recovery is proven on the stack (see *Verified*).

### Blocked

- **Didit (KYC/KYB)**: the decision is a new organisation, not A-CARD. That organisation does not exist yet, and no tool
  in this environment can create one. The owner must create it (by adding Didit's own connector and signing up on its
  page with the personal email) and grant access. The adapter is built, but no verification session has been created, no
  delivery from Didit has been checked, and none is approved.
- **TigerBeetle: unblocked on 2026-11-16**, by allowing one host (`pkg-containers.githubusercontent.com`, which serves
  the image's layers) in the build environment. What is still blocked is a production cluster: hardware, sites and an
  operator. Nothing in this repository provisions one.
- **Kafka: runs in the dev stack, but only single-node.** `apache/kafka:3.9.0` in KRaft mode is verified by the outbox
  tests and by the local stack (on port 9192, so it does not clash with a broker on 9092). Not verified: a multi-broker cluster, TLS and SASL, topic creation policy in production, and the Kafka wiring of
  the other services' relays.
- **Sponsor, partner-bank and FIC list formats**: unverified (VERIFY-SPONSOR, VERIFY-PARTNER, VERIFY-FIC). The simulators
  define Forge's own wire formats. The government and UN sites were not reachable from the build environment, so the
  sanctions facts in `docs/SANCTIONS_SOURCES.md` are secondary and need checking.
- **goAML XML**: the official schema is unverified (VERIFY-GOAML). Submission is refused until it is confirmed.
- **WORM storage** for audit anchors and evidence packs (VERIFY-WORM). Local files are used for development only.
- **Threshold owners**: every placeholder needs a named owner (`docs/THRESHOLDS.md`). None is named yet.
- **Sanctions list source and licence**: a recommendation exists; the decision and the licence do not.
- **ADR-0020** (legal form of the bank layer) is open, so the own-licence route is not built.
- **Reconciliation's window**: between the bank debiting a payment and payments seeing it, the run breaks and the gate
  closes until a later clean run. That is fail-closed by design. It is documented in the reconciliation spec.

### Running it

```sh
make up               # the local stack: builds every service and starts it (deploy/compose/README.md)
make test-stack       # the customer and back-office journeys against the running stack, with nothing mocked
make ci-db            # the full gate, with throwaway databases and a throwaway TigerBeetle replica in Docker
                      # (defaults: Postgres on 5432, broker on 9092; for the stack's own, set
                      # FORGE_PG_HOST_DSN=postgres://postgres:forge-local-only@127.0.0.1:5433 and
                      # FORGE_KAFKA_BROKERS=127.0.0.1:9192)
make test-ledger-tb-cluster  # three TigerBeetle replicas, taken apart while in use (not part of the gate)
make ci               # the same gate, if you have set every database and broker variable yourself
make test-outbox      # the outbox against a real broker
make test-recon       # reconciliation (serial: packages share a database)
make test-banklayer   # BankLayer contract suite, bootstrap, simulator, route check
make test-audit       # audit log, anchors and service
make test-treasury    # treasury alerts
make test-compliance  # sanctions, monitoring and cases
make test-policy      # policy service and Rego tests
make test-platform    # party, consent and onboarding
make test-bank        # rails gateway and payments orchestrator
```

**`make ci` refuses to pass unless the database and broker suites are configured.** Set them with `make ci-db`, or set
every variable in `DB_TEST_VARS` (see the Makefile). The only opt-out is `FORGE_SKIP_DB_TESTS=1`, which prints a warning
and proves less. Each suite still reads its own variable, and a test that cannot reach its database fails rather than
skips.

The sponsor simulator (`tools/sim/sponsor`) refuses to start unless `SPONSOR_SIMULATION=1` is set.

### Decisions and open items

See `docs/DECISIONS.md` (`docs/DECISIONS.md`) for every owner decision, what it changed, and what is still open. The
threshold register is `docs/THRESHOLDS.md` (`docs/THRESHOLDS.md`). The sanctions recommendation is
`docs/SANCTIONS_SOURCES.md` (`docs/SANCTIONS_SOURCES.md`).

### Conventions

- Money is integer minor units in 64-bit integers. ZAR and USD use scale 2. Ratios are basis points.
- Only `pkg/banklayer` may read the licence route. Behaviour that depends on it is data for OPA.
- Every money-moving command carries an idempotency key. Replays return the original result.
- Money-path changes need two human reviews (`docs/ARCHITECTURE.md` section 23.4).
- Anything marked VERIFY in the docs is unconfirmed and must be checked by a human before it is relied on.
