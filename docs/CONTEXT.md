# FORGE Mutual Bank: Business Context

Version 1.0 | 29 October 2026 | Status: concept, Phase 0
Place at `docs/CONTEXT.md`. Companion files: `AGENTS.md` (repository rules), `docs/ARCHITECTURE.md` (technical design).

> **What this file is for.** It explains what Forge Mutual Bank is, who it serves, how it makes money and why it is designed the way it is. Read it before writing specs, product copy or customer-facing features. It is context, not a source of regulatory fact: anything marked VERIFY must be confirmed by a human.
>
> **Status.** Forge is a proposal. It is not licensed, has no customers and holds no money. Figures marked unverified, illustrative or estimated must not appear in external material without checking.

---

## Contents

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

## 1. Summary

Forge Mutual Bank is a proposed South African bank and money platform for technology companies and the people who build them. It gives a company one workspace to hold rand and US dollars, receive revenue from foreign customers, pay staff, suppliers and contractors at home and abroad, and manage its money as a team.

It starts on a sponsor bank, which holds the deposits and carries the licence, the way Mercury started in the United States. Later, Forge seeks its own licence, as a mutual bank owned by its members or as a bank under the Banks Act. That decision is still open. Once Forge holds its own licence, the original vision returns: rand, tokenised deposits and, through a separate licensed entity, approved stablecoins, all inside one regulated institution.

The platform is built on open-source, self-hostable software, so the core can run in South Africa under Forge's control. A franchised network of independent micro-lenders (the MFI network) is planned for year 2, to extend Forge's infrastructure and funding to underserved borrowers.

---

## 2. The problem

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

## 3. What Forge is

### 3.1 Positioning

Forge is the money operating system and bank for technology companies and the people who build them, in South Africa first, then Africa through partner banks.

It is not a mass-market bank. Capitec has 26.6 million active clients and 686,000 business clients, so Forge cannot win on price or reach. Forge wins as a specialist.

### 3.2 Where the idea comes from

| Influence | What Forge takes | What Forge does not take |
|---|---|---|
| Mercury (US) | Software first on partner banks; a licence later; breadth beyond tech | Its scale or US market |
| Erebor (US) | A specialist, API-first bank for tech; on-chain money treated as native | Its capital (about $380M raised) or its customer base |
| Capitec | Simplicity and low cost to serve | Mass-market ambition |
| Discovery | Orchestration and consented, data-driven incentives | A large group customer base or years of heavy investment |
| Investec | Community, high value per client, founder-plus-company banking | A decades-old brand |
| Bank Zero | Distribution through partners (alliance banking) | Its zero-fee retail focus |
| Nubank | Credit as the long-term profit engine | Lending from its own balance sheet early |

### 3.3 Principles

1. **Bank first, crypto second.** On-chain money is a rail, never the book of record.
2. **One ledger per legal entity.** Every balance inside an entity sits in one place.
3. **Custody clarity.** Every balance shows which entity holds it and whether it is deposit-insured.
4. **Closed regulatory perimeter.** Activity stays inside a licensed entity or is handed off with disclosure.
5. **Customer simplicity.** No seed phrases or private keys for regulated balances.
6. **Open and sovereign.** Open-source, self-hostable components with licences checked; no hard dependency on one foreign cloud or vendor.
7. **Compliance built in,** not added afterwards.

---

## 4. Legal structure

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

## 5. The staged route and the mutual question

### 5.1 Three stages

| Stage | Bank layer | On-chain | What changes for customers |
|---|---|---|---|
| A (now) | Sponsor bank holds deposits; Forge runs a sub-ledger | None | ZAR and USD accounts, payments, FX, workspace |
| B (licensed) | Forge holds its own licence | Tokenised deposits inside the bank | Direct rails such as PayShap; tokenised rand deposits |
| C (cleared) | Unchanged | Stablecoin corridors via the separate digital-asset entity | Faster cross-border settlement where legally allowed |

The original idea, one member-owned bank holding rand, tokenised deposits and stablecoins together, is the destination. It cannot be the starting point, because Forge has no licence and stablecoins sit outside the payments law.

### 5.2 Mutual or bank (ADR-0020, open)

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

## 6. Target customers

### 6.1 First customer group: technology companies (ADR-0022, accepted)

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

### 6.2 Adjacent customers on the same product

Cross-border SMEs (importers, exporters, freelancers paid from abroad) are welcome on the same product. They broaden the deposit base, do not follow venture funding cycles and have the same cross-border pain.

### 6.3 Later customer groups

| Segment | When | Why later |
|---|---|---|
| Fintech and regulated web3 firms | After launch | Needs enhanced due diligence and a stance on crypto (ADR-0012) |
| Platforms embedding Forge (BaaS) | Year 2 | Needs a mature core and partner contracts |
| Investors and funds | Year 2 | Fund structures need extra checks |
| MFI network customers | Year 2 pilot | Different customers, different risks (section 10) |
| Other African countries | Years 2 to 3 | Each needs a partner bank and its own regulatory work |

### 6.4 Not targeted

Mass-market retail, consumer lending, crypto traders looking for an exchange, and any sector excluded by the risk-appetite policy.

### 6.5 Size of the market

| Source | Count | What it means |
|---|---|---|
| StartupBlink | 1,246 startups | Venture-style core |
| Tracxn | 53,082 companies | Broad, includes companies beyond early stage |
| CIPC registry | 91,816 software companies | Mostly tiny or inactive |

These come from earlier notes and are unverified. The serviceable market is an estimate: a few thousand startups and scale-ups plus several thousand active tech SMEs, agencies and exporters.

**The funding backdrop is thin.** South African startups raised about $83M in the first half of 2026, against $327M in Egypt, $254M in Nigeria and $126M in Kenya. Startup runway alone cannot support a bank, which is why profitable tech SMEs, adjacent businesses and other African markets matter.

---

## 7. Products

### 7.1 v1

- ZAR business accounts, held at the sponsor bank with a Forge sub-ledger.
- A USD account through one partner bank, for receiving foreign revenue and paying USD costs.
- Cross-border payments and FX through an authorised-dealer partner, quoted as one all-in rand price with the spread disclosed.
- Domestic payments (EFT) through the sponsor.
- Business and personal identity checks through Didit.
- A web workspace: balances by currency and entity, payments, approvals, team roles, statements.
- A back-office console for Forge operations.
- Built-in compliance: sanctions screening, transaction monitoring, reporting to the Financial Intelligence Centre.
- Deposit concentration and outflow alerts, because tech deposits are volatile.

### 7.2 Later

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

### 7.3 Explicit non-goals

DeFi (swaps, staking, yield), self-custody products, Forge-issued stablecoins, a Forge card BIN, CBDC work and mass-market marketing.

---

## 8. Business model

### 8.1 Four product pillars

1. **Core money.** Multi-currency accounts, payments, FX and treasury.
2. **Orchestration.** One workspace where payments, FX, payroll, tax and accounting connect, with rules such as "move 10% of every receipt into a tax pot".
3. **Incentives and data.** With the customer's consent, revenue and usage data first feed dashboards, then transparent pricing and rewards, and later credit through partner capital.
4. **Community and founder-plus-company banking.** A founder community as a trusted distribution channel, and a personal tier for founders kept strictly separate from their company.

### 8.2 Revenue layers

| Layer | Source | Depends on | Phase |
|---|---|---|---|
| Subscriptions | Workspace tiers | Product depth and trust | Early |
| Transactions | FX spreads, payout fees, later card interchange | Authorised-dealer partner, sponsor and partner banks | Launch |
| Float income | Interest on balances, shared with the sponsor | Deposit size and sponsor terms | Launch |
| Platform fees | BaaS partners | Partners embedding Forge | Year 2 |
| Credit | Revenue-based financing on partner capital | Capital, registration, risk models | Year 3 or later |
| MFI platform | Software and service fees from MFIs | Profitable franchisees | Year 2 pilot |

**Pricing principles:** quote one all-in price; disclose the FX spread; no hidden margins; fees fall as volume grows. Pricing rules live in policy, not code, and any change to pricing, rewards or credit needs legal review.

### 8.3 Illustrative economics

These are assumptions, not data. Revenue per customer starts at $500, $900 or $1,400 a year and grows 10% a year; variable costs are 25% of revenue; operating costs grow from $2M to $16M over five years.

| Year 5 | Conservative | Base | Optimistic |
|---|---|---|---|
| Customers | 3,200 | 9,000 | 18,000 |
| Revenue | about $1.8M | about $8.9M | about $27.7M |
| Operating result | about -$4.6M | about -$3.3M | about +$4.8M |
| Deposits | about $48M | about $270M | about $900M |
| Peak funding need | about $17.7M | about $19.8M | about $16.8M |
| Customers to break even | about 10,900 | about 10,100 | about 10,400 |

**What this means**

- Break-even needs about 10,000 business customers, close to the whole South African serviceable market, so expansion is necessary.
- The funding need is similar across scenarios, because spending grows with success.
- Mercury earns about $2,170 per customer ($650M over about 300,000 customers, a calculation from reported figures). South African companies are smaller, so even the conservative case may be generous.
- The model excludes the sponsor bank's revenue share, licence capital, credit losses and compliance headcount. Real economics are harder.

The customer interviews (section 17) replace these assumptions with real prices and balances.

---

## 9. Distribution

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

## 10. The MFI network

### 10.1 The idea

A franchise-style network of independent micro-lenders using Forge's platform, brand and funding, modelled on the McDonald's system rather than the burger: franchisees pay for a tested way of operating, not just a name.

| What Forge supplies | Detail |
|---|---|
| Brand and operating manual | How to run a compliant micro-lender |
| The platform | White-label lending software built on Fineract, with limits enforced in software so a lender cannot offer a product, price or sector its policy forbids |
| Funding | Wholesale credit from the bank layer, plus development-finance funding |
| Shared compliance | Identity checks, sanctions screening, reporting |
| Credit data | Shared scoring that improves as lenders join (a credit bureau is a regulated business and needs its own registration; VERIFY) |

Natural first members are already-registered lenders, savings co-operatives, stokvels, church groups and community savings clubs that have trust but lack infrastructure and funding.

### 10.2 Structure: official but arm's-length

- Each MFI is its own legal entity, registered or licensed in its own right: a credit provider registered with the National Credit Regulator, or a co-operative financial institution if it holds member savings.
- It uses the Forge brand under a licence agreement.
- The bank is its funder, not its owner. If ADR-0020 chooses a mutual, MFIs could become members of the bank.
- Fees come from services to the lending business, never from recruiting more franchisees, to avoid pyramid-scheme concerns.
- An unofficial spin-off is ruled out because of holding-out risk.

### 10.3 What it could change

- Cheaper, registered credit replacing unregistered lenders.
- Local ownership, so lending profits stay in communities.
- Formal savings that build assets, not just debt.
- Repayment history that lets a small loan grow into a larger one, which is an incentive to build a business.

### 10.4 What it will not do on its own

- Randomised studies of microcredit in seven countries found no significant effect on average household income, though they showed more business choice and expansion for households that already ran businesses.
- Franchising spreads credit risk across many operators; it does not remove it.
- Margins are capped and over-indebtedness is high.
- The co-operative route for member savings is small: 6 co-operative banks and 24 co-operative financial institutions held R594M of deposits in 2023/24.
- Funding illiquid microloans with volatile tech deposits recreates the funding mismatch behind bank runs, so exposure is capped and term or development-finance funding is preferred.

### 10.5 How it is tested

Pilot with three to five already-registered lenders. Measure portfolio at risk, cost per loan, the loan book each lender needs to break even, the rate borrowers pay before and after, borrowers' business revenue, and whether pooled data improves default prediction. Open the franchise only if pilot lenders are profitable.

An alternative to test first is the agent-network model used in Nigeria (Moniepoint, OPay, PalmPay), where local agents handle onboarding and payments and lending comes later.

---

## 11. Market and competition

### 11.1 Competitors

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

### 11.2 Models, not rivals

| | Erebor | Mercury | Forge |
|---|---|---|---|
| Customers | Tech, crypto, defence | Startups and tech, then broader | Tech companies, then adjacent SMEs |
| Structure | Own national charter (opened February 2026) | Partner banks; conditional charter approval April 2026 | Sponsor bank, licence later |
| Scale | $4.05bn deposits by end of June 2026 | 300,000+ customers, about $650M annualised revenue | Concept |
| Capital | About $380M raised | Large | Far smaller |
| Stack | Proprietary | Proprietary | Open-source, self-hostable |

Erebor found demand for crypto-backed lending lower than expected, which is a reminder that on-chain features are not yet why businesses choose a bank.

---

## 12. What makes Forge different

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

## 13. Regulatory landscape

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

## 14. Technology in brief

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

## 15. Relationship to the wider FORGE ecosystem

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

## 16. Risks

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
| Capital | About $15M to $20M before profit in the base case | Milestone funding; a sale as an acceptable outcome |
| AI-built software | Defects in money-path code | Tests decide; two human reviews; no real data in development |

---

## 17. Roadmap

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

## 18. Decisions and open questions

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

## 19. Facts to verify and corrected figures

**Corrected from earlier documents**

- Capitec has 26.6M active clients, not 22M.
- GoTyme has more than 12.5M customers, per its chief executive.
- Bank Zero's 275,000 direct and 500,000 alliance customers are unsupported. Sources show over 40,000 funded accounts (April 2025), about R700M deposits and its first alliance partner only in April 2026.
- Erebor raised about $380M in total, not $635M.
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

## 20. Glossary

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
