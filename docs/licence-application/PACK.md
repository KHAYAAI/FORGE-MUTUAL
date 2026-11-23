# Licence application pack: index and tracker

> Generated from the regulatory register and [`sections.csv`](sections.csv) by `scripts/build-licence-pack.py`. **This is an index of
> what the application needs, not the application.** It says where each document is, who owns it, and what is still missing.
> Nothing in it is approved. Route: mutual bank (planned) or bank (kept open), see [`../governance/LEGAL_FORMS.md`](../governance/LEGAL_FORMS.md).

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

## A1 · Legal form and constitution

**Owner:** Head of Legal (R26) with the lawyer. Mutual bank constitution or company articles; see docs/governance/LEGAL_FORMS.md

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-01** Bank = public company under Companies Act 71 of 2008; mutual bank = juristic person under Mutual Banks Act, ow | Decision | Partial | [CONTEXT.md](../../docs/CONTEXT.md)<br>[licence-application-plan.md](../../docs/regulatory/policies/licence-application-plan.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md)<br>[legalform.go](../../pkg/banklayer/legalform.go)<br>[LEGAL_FORMS.md](../../docs/governance/LEGAL_FORMS.md) | Owner's direction 2026-11-18: mutual bank planned, bank kept open; both encoded as data. Legal advice needed before ADR-0020 closes |
| **LIC-07** At least seven founders with signed articles; depositor-members vote at general meetings; mutuality concept st | Document | Partial | [ARCHITECTURE.md](../../docs/ARCHITECTURE.md)<br>[shareholder-and-significant-ownership-policy.md](../../docs/regulatory/policies/shareholder-and-significant-ownership-policy.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md)<br>[membership](../../services/bank/membership/internal/membership) | Member register, shares, one-member-one-vote and the seven-founder check built and tested; constitution not drafted; module not wired to a service or the ledger |

## A2 · Name and naming authorisation

**Owner:** Chief Compliance Officer (R04). No 'bank' or 'mutual bank' in public naming until authorised

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-17** 'Bank', 'mutual bank' and 'building society' in a business name need PA authorisation | Decision | Open | [CONTEXT.md](../../docs/CONTEXT.md)<br>[naming-and-marketing-policy.md](../../docs/regulatory/policies/naming-and-marketing-policy.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Do not use 'bank' in public naming until authorised |

## A3 · Capital and proof of funds

**Owner:** CFO (R03) and the founder. R10m (mutual) or R250m (bank) with written proof

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-02** Minimum capital R250m (bank) or R10m (mutual bank), with written proof (Banks Act s.70 / Mutual Banks Act s.48 | External | Open | [legalform.go](../../pkg/banklayer/legalform.go)<br>[prudential](../../services/bank/treasury-risk/internal/prudential) | Minimum per form is checked in the prudential report (INITIAL_CAPITAL breach); no capital raised or evidenced |

## A4 · Founders and shareholders or members

**Owner:** Company Secretary (R17). Register of founders and members; fit and proper for significant holders; membership module checks seven founders

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-05** Significant shareholders fit and proper; >15% acquisition needs permission (Banks Act s.37, s.42); Form BA 125 | Document | Partial | [shareholder-and-significant-ownership-policy.md](../../docs/regulatory/policies/shareholder-and-significant-ownership-policy.md)<br>[fit-and-proper-procedure.md](../../docs/regulatory/policies/fit-and-proper-procedure.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Shareholder register and fit-and-proper files not started |
| **LIC-07** At least seven founders with signed articles; depositor-members vote at general meetings; mutuality concept st | Document | Partial | [ARCHITECTURE.md](../../docs/ARCHITECTURE.md)<br>[shareholder-and-significant-ownership-policy.md](../../docs/regulatory/policies/shareholder-and-significant-ownership-policy.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md)<br>[membership](../../services/bank/membership/internal/membership) | Member register, shares, one-member-one-vote and the seven-founder check built and tested; constitution not drafted; module not wired to a service or the ledger |

## B1 · Board and executive officers

**Owner:** Board chair (R05) and Nominations Committee. Composition and independence; at least two employee directors on the mutual route

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-06** Board per Act/regulations; directors and executive officers fit and proper (BA 020 / DI 020); accountability,  | Document | Partial | [THRESHOLD_OWNERS.md](../../docs/THRESHOLD_OWNERS.md)<br>[board-charter.md](../../docs/regulatory/policies/board-charter.md)<br>[fit-and-proper-procedure.md](../../docs/regulatory/policies/fit-and-proper-procedure.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). No board; BA/DI 020 not started |
| **MR-03** At least two board members must be employees of the bank | Decision | Open | [board-charter.md](../../docs/regulatory/policies/board-charter.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Governance design not started |

## B2 · Fit-and-proper statements (DI 020 / BA 020)

**Owner:** Company Secretary (R17). See docs/governance/FIT_AND_PROPER_PACK.md; nobody has filed

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **BR-06** Directors' and executive officers' statements (BA 020) and the prescribed forms list | Document | Partial | [FIT_AND_PROPER_PACK.md](../../docs/governance/FIT_AND_PROPER_PACK.md) | Filing pack and tracker for every director and executive officer (DI 020 / BA 020); nobody has filed; form content VERIFY |

## B3 · Directors' report on internal control

**Owner:** Board (Audit Committee). Yearly report to the Registrar

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **MR-02** Directors report annually to the Registrar on effectiveness of internal controls over financial and regulatory | Document | Partial | [internal-control-over-financial-reporting.md](../../docs/regulatory/policies/internal-control-over-financial-reporting.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Not started |

## C1 · Business plan and public-interest motivation

**Owner:** Founder and CFO. Outline only; see docs/regulatory/policies/business-plan-outline.md

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-15** Business plan, public-interest motivation, rationale, group structure chart, dividend policy, memorandum (or a | Document | Partial | [CONTEXT.md](../../docs/CONTEXT.md) | CONTEXT.md is the base for the business plan; others not started |

## C2 · Financial projections and pro forma returns

**Owner:** CFO (R03) and Regulatory Reporting (R22). make proforma builds DI/BA 100 and 200 from projections; other forms outstanding; figures are illustrative

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-08** Systems must produce all statutory and prudential information accurately and on time from the first day of ope | Code | Partial | [trialbalance.go](../../services/bank/treasury-risk/internal/prudential/trialbalance.go)<br>[returns](../../services/bank/reg-reporting/internal/returns) | Trial balance and balance sheet from the ledger with a double-entry check; the returns framework exists; no form builder (line definitions unread) |
| **LIC-11** Pro forma BA 100/110/120/130/300/350/700 (bank) or DI 100/110/200/300/400/430/700 (mutual bank), proposed for  | Code | Partial | [returns](../../services/bank/reg-reporting/internal/returns)<br>[returns-proforma](../../services/bank/reg-reporting/cmd/returns-proforma)<br>[proforma](../../docs/regulatory/proforma) | Pro forma balance sheet and income statement for both forms built from projections (make proforma); projection is illustrative; other forms and line mapping to form instructions outstanding |
| **BR-07** Returns BA 100 through BA 950, each with its own directives and interpretations (BA 200 alone is about 340 pag | Code | Partial | [SPEC.md](../../services/bank/reg-reporting/SPEC.md)<br>[statements.go](../../services/bank/reg-reporting/internal/returns/statements.go) | BA 100 and BA 120 built from the ledger trial balance, balanced and reconciled; about 17 other BA forms and the mapping to official line numbers outstanding |
| **MR-06** Returns DI 100, 110, 200, 300, 310, 400, 401, 402, 403, 410, 420, 430, 500-520, 600, 700-702 plus Chapter V re | Code | Partial | [SPEC.md](../../services/bank/reg-reporting/SPEC.md)<br>[statements.go](../../services/bank/reg-reporting/internal/returns/statements.go) | DI 100 and DI 200 (form numbers VERIFY) built from the ledger trial balance; remaining DI forms and line mapping outstanding |
| **RET-01** Certification and rendition of returns, auditor reports and reconciliation of selected returns | Code | Partial | [SPEC.md](../../services/bank/reg-reporting/SPEC.md)<br>[returns.go](../../services/bank/reg-reporting/internal/returns/returns.go) | Preparer, different approver, evidence digest and auditor report enforced in code; the PA certification form itself not built |
| **MR-05** Auditor reports on about 20 DI forms; DI 900 must reconcile to DI 100, auditor reports within 90 days of year  | Code | Partial | [returns.go](../../services/bank/reg-reporting/internal/returns/returns.go) | Audited returns cannot be submitted without an independent auditor's report recorded; which forms need one, and the report wording, VERIFY |

## D1 · Risk management framework and policies

**Owner:** Chief Risk Officer (R02). Risk framework and appetite drafted; appetite numbers await the owner

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-13** Risk-by-risk policy and quantified effect for solvency, liquidity, counterparty, interest-rate, market, credit | Document | Partial | [risk-management-framework.md](../../docs/regulatory/policies/risk-management-framework.md)<br>[risk-appetite-statement.md](../../docs/regulatory/policies/risk-appetite-statement.md)<br>[credit-risk-policy.md](../../docs/regulatory/policies/credit-risk-policy.md)<br>[market-and-interest-rate-risk-policy.md](../../docs/regulatory/policies/market-and-interest-rate-risk-policy.md)<br>[operational-risk-policy.md](../../docs/regulatory/policies/operational-risk-policy.md)<br>[capital-management-policy.md](../../docs/regulatory/policies/capital-management-policy.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). No risk policies written |
| **BR-01** Board-owned risk management covering the listed risk types: capital, compliance, concentration, counterparty,  | Document | Partial | [SPEC.md](../../services/bank/treasury-risk/SPEC.md) | Only a few risks have any code; need a risk register with policy and owner per risk |
| **BR-02** Board-approved policies to identify, measure, monitor, control, price, mitigate and report each risk | Document | Partial | [risk-management-framework.md](../../docs/regulatory/policies/risk-management-framework.md)<br>[committee-terms-of-reference.md](../../docs/regulatory/policies/committee-terms-of-reference.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). No board-approved policies |
| **MR-01** Risks to manage: solvency, liquidity, credit, currency, market, interest-rate, counterparty, technological, op | Document | Partial | [risk-management-framework.md](../../docs/regulatory/policies/risk-management-framework.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Version is the original 1993 gazette; confirm current text with the PA |

## D2 · Capital adequacy and liquidity

**Owner:** Head of Treasury (R13). Capital and leverage ratios and the liquidity coverage ratio are computed from the ledger (treasury-risk); concentration policy and liquidity plan drafted

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-03** Total capital ratio >= 11.5%, leverage ratio >= 4% (Directive 6 of 2017); higher ratio possible for new banks | Code | Partial | [prudential](../../services/bank/treasury-risk/internal/prudential) | Capital adequacy and leverage computed from the ledger daily (back office Risk); weights and minimums PROPOSED (thresholds 22, 23); no capital raised, so both ratios show in breach |
| **LIC-04** Prudent funding profile, buffer of high-quality liquid assets, liquidity risk framework and stress testing | Code | Partial | [prudential](../../services/bank/treasury-risk/internal/prudential)<br>[liquidity-and-contingency-funding-plan.md](../../docs/regulatory/policies/liquidity-and-contingency-funding-plan.md) | 30-day LCR and five stress scenarios computed from the ledger; draft liquidity plan; no committed funding line; undrawn limits and notices not yet read |
| **LIC-12** Policy on maximum deposit from a single depositor relative to total deposits | Document | Partial | [SPEC.md](../../services/bank/treasury-risk/SPEC.md)<br>[concentration-policy.md](../../docs/regulatory/policies/concentration-policy.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Concentration alerts exist but no board-approved policy |
| **BR-04** Liquidity stress testing, contingency funding plans, liquid-asset cushion | Document | Partial | [prudential](../../services/bank/treasury-risk/internal/prudential)<br>[liquidity-and-contingency-funding-plan.md](../../docs/regulatory/policies/liquidity-and-contingency-funding-plan.md) | Stress testing and a draft contingency funding plan exist; run-off rates PROPOSED; plan not approved |

## D3 · Internal audit

**Owner:** Head of Internal Audit (R11). Charter drafted; 12 months of findings need a running audit function

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **BR-05** Internal audit is a permanent function; outsourcing it needs prior written Registrar approval; scope includes  | Document | Partial | [SPEC.md](../../services/shared/audit-service/SPEC.md)<br>[internal-audit-charter.md](../../docs/regulatory/policies/internal-audit-charter.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Audit log is a control, not an internal audit function |
| **LIC-14** Last 12 months of internal-audit findings on deposit book, advances book and large exposures; any non-complian | Document | Partial | [internal-audit-charter.md](../../docs/regulatory/policies/internal-audit-charter.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Nothing to report yet; needs an internal audit function |

## D4 · Business continuity and recovery

**Owner:** Business Continuity Manager (R28). Business impact analysis and recovery plan drafted; no tested DR site

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-09** Contingency plans so business continues after a disaster | Document | Partial | [pitr-check.sh](../../scripts/pitr-check.sh) | PITR proven locally; no production DR plan or drill |
| **D3-06** Contingency plans; IP and data rights protected; data usable, readable and portable after termination | Document | Partial | [exit-plans.md](../../docs/regulatory/policies/exit-plans.md) | Draft exit plan per provider; no provider contracts to hold exit terms |
| **JS1-11** Recovery priorities, RTO and RPO set; business impact assessment covering third-party dependencies | Document | Partial | [recovery-plan.md](../../docs/regulatory/policies/recovery-plan.md) | Proposed values only; board must set |
| **JS2-13** Set RPO and RTO from nature, scale, complexity and risk | Document | Partial | [recovery-plan.md](../../docs/regulatory/policies/recovery-plan.md) | Draft with proposed RPO and RTO; board must set |

## E1 · IT systems and material outsourcing submission

**Owner:** CTO (R10) and CISO (R12). Assembled from the IT policies and docs/ARCHITECTURE.md

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-10** Detailed discussion of IT systems, material outsourcing and third-party supplier operational risk, with monito | Document | Partial | [ARCHITECTURE.md](../../docs/ARCHITECTURE.md)<br>[it-risk-management-framework.md](../../docs/regulatory/policies/it-risk-management-framework.md)<br>[outsourcing-policy.md](../../docs/regulatory/policies/outsourcing-policy.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). No written IT and outsourcing submission; see OUT-* rows |
| **JS1-01** Governing body ultimately responsible; approves an IT strategy aligned to business strategy, reviewed at least | Document | Partial | [it-strategy.md](../../docs/regulatory/policies/it-strategy.md) | Draft IT strategy; no board to approve it |
| **JS1-02** IT risk management framework approved by governing body, reviewed annually; material IT risks identified, asse | Document | Partial | [risk-register.md](../../docs/regulatory/policies/risk-register.md) | Starter register exists; technology risk not rated; framework not approved |
| **JS1-03** Dedicated IT risk function accountable to the governing body, led by a suitably skilled senior manager | Decision | Open | [it-risk-management-framework.md](../../docs/regulatory/policies/it-risk-management-framework.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Needs a hire or appointment |
| **JS1-05** Risk register maintained; IT risk metrics and profile reported to governing body | Document | Partial | [risk-register.md](../../docs/regulatory/policies/risk-register.md) | No metrics reporting; no board |
| **JS1-06** Staff, providers and contractors screened, fit and proper, bound to confidentiality; training reviewed at leas | Document | Partial | [staff-screening.md](../../docs/regulatory/policies/staff-screening.md)<br>[training-plan.md](../../docs/regulatory/policies/training-plan.md) | Draft screening procedure and training plan; no staff yet |
| **JS1-10** IT programme and project management framework; security requirements approved independently of development; re | Document | Partial | [it-project-framework.md](../../docs/regulatory/policies/it-project-framework.md) | Draft framework; no independent security approver named |
| **JS1-12** Geographically separate DR site; IT resilience plan tested at least annually including total primary-site loss | Code | Open | [business-impact-analysis.md](../../docs/regulatory/policies/business-impact-analysis.md) | No production hosting chosen (ADR-0002); nothing built |
| **JS2-01** Board accountable; cyber function with direct line to board; board-approved strategy reviewed at least annuall | Document | Partial | [cyber-strategy.md](../../docs/regulatory/policies/cyber-strategy.md) | Draft strategy written; not board-approved |
| **JS2-09** Cryptographic key management: generation, distribution, renewal, revocation, destruction, tamper-resistant pro | Document | Partial | [key-management-policy.md](../../docs/regulatory/policies/key-management-policy.md) | Draft policy written; not approved; KMS or HSM undecided |
| **JS2-10** Cyber awareness training at least annually; board training | Document | Partial | [training-plan.md](../../docs/regulatory/policies/training-plan.md) | Draft plan only; no training delivered; not approved |
| **JS2-14** Cyber incident response and management plan, tested against plausible threats; customer communication strategy | Document | Partial | [incident-response-plan.md](../../docs/regulatory/policies/incident-response-plan.md) | Draft plan written; never tested |

## E2 · Cloud and offshoring

**Owner:** CTO (R10) and Head of Legal (R26). Hosting direction in docs/HOSTING.md; PA notice 20 working days before production

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **D3-01** Board-approved data strategy and data governance framework | Document | Partial | [ARCHITECTURE.md](../../docs/ARCHITECTURE.md)<br>[data-strategy.md](../../docs/regulatory/policies/data-strategy.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Architecture states SA data residency; no board-approved strategy |
| **D3-02** Cloud/offshoring policy aligned to strategy and risk appetite; oversight inside governance structures | Document | Partial | [ARCHITECTURE.md](../../docs/ARCHITECTURE.md)<br>[cloud-and-offshoring-policy.md](../../docs/regulatory/policies/cloud-and-offshoring-policy.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). ADR-0002 (hosting) open |
| **D3-03** Risk-appetite check and due diligence before each cloud or offshoring initiative | Document | Partial | [PROVIDERS.md](../../docs/PROVIDERS.md)<br>[cloud-and-offshoring-policy.md](../../docs/regulatory/policies/cloud-and-offshoring-policy.md)<br>[provider-due-diligence-procedure.md](../../docs/regulatory/policies/provider-due-diligence-procedure.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Not done for WorkOS, Didit or any host; check where each processes data |
| **D3-05** No obstruction of regulator access or forensic audits by cloud or offshoring arrangements | Document | Partial | [cloud-and-offshoring-policy.md](../../docs/regulatory/policies/cloud-and-offshoring-policy.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Needs contract terms with every provider |
| **D3-07** Every arrangement in a documented, legally binding agreement | External | Partial | [provider-contract-schedule.md](../../docs/regulatory/policies/provider-contract-schedule.md) | 16 minimum clauses defined and enforced through the outsourcing register; no contracts signed |

## E3 · Outsourcing register and contracts

**Owner:** COO (R08) and Head of Legal (R26). docs/regulatory/outsourcing-register.csv computes notice dates

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **OUT-01** Core banking IT and financial-reporting IT are material functions; outsourcing needs prior written approval | Decision | Open | [ARCHITECTURE.md](../../docs/ARCHITECTURE.md) | Hosting choice (ADR-0002) decides whether this is triggered |
| **OUT-02** Notify the PA at least 20 working days before material outsourcing, and before offshoring | External | Partial | [outsourcing-register.csv](../../docs/regulatory/outsourcing-register.csv)<br>[check-outsourcing.py](../../scripts/check-outsourcing.py) | Register computes the latest PA notice date (20 working days, SA holidays) per material provider and the gate fails if one is missed or a provider is used without notice; no start dates yet |
| **OUT-03** Board-approved outsourcing policy, plan and risk assessment, due diligence, binding contract, monitoring, cont | Document | Partial | [outsourcing-policy.md](../../docs/regulatory/policies/outsourcing-policy.md)<br>[provider-due-diligence-procedure.md](../../docs/regulatory/policies/provider-due-diligence-procedure.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Not started |
| **OUT-04** Materiality assessment of every outsourcing arrangement (eleven factors) | Document | Partial | [outsourcing-register.csv](../../docs/regulatory/outsourcing-register.csv) | Register of providers with materiality; the eleven-factor assessment per provider is not done |
| **OUT-05** Recovery plan must cover outsourced material activities | Document | Partial | [business-continuity-and-disaster-recovery-policy.md](../../docs/regulatory/policies/business-continuity-and-disaster-recovery-policy.md)<br>[recovery-plan.md](../../docs/regulatory/policies/recovery-plan.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). No recovery plan |
| **OUT-06** Contract minimum topics (about 30), no provider right to cancel on resolution, regulator and auditor access | External | Partial | [provider-contract-schedule.md](../../docs/regulatory/policies/provider-contract-schedule.md) | Minimum clauses drafted from the guidance as read; verify against the full text |

## E4 · Cyber resilience and testing

**Owner:** CISO (R12). No penetration test yet; findings in docs/regulatory/security-findings.csv

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **G4-01** Assess policies against the CPMI-IOSCO cyber-resilience guidance (governance, identification, protection, dete | Document | Partial | [threat-intelligence-and-security-testing-policy.md](../../docs/regulatory/policies/threat-intelligence-and-security-testing-policy.md)<br>[STRUCTURE.md](../../docs/governance/STRUCTURE.md) | Draft written, not approved; seat holders VACANT (docs/governance/roles.csv). Self-assessment not done |
| **G4-02** Recovery time objectives from a business impact assessment; penetration testing by reputable, accredited exter | External | Partial | [business-impact-analysis.md](../../docs/regulatory/policies/business-impact-analysis.md) | Business impact analysis with RTO and RPO per service; no penetration test yet (SF-001) |
| **JS2-15** Threat-intelligence process and participation in information sharing | External | Partial | [threat-intelligence-and-security-testing-policy.md](../../docs/regulatory/policies/threat-intelligence-and-security-testing-policy.md) | Process drafted; no feed or sharing group joined yet (SF-004) |
| **JS2-16** Test all controls; vulnerability assessments; penetration test of internet-facing critical systems annually or | External | Partial | [security-findings.csv](../../docs/regulatory/security-findings.csv)<br>[check-security-findings.py](../../scripts/check-security-findings.py) | Remediation register with severity deadlines checked by the gate; no penetration test or simulation yet (SF-001) |

## F1 · Regulator engagement

**Owner:** Founder and CCO. Pre-application meeting with the PA; FSCA concurrence

| Requirement | Type | Status | Evidence | Gap or next step |
|---|---|---|---|---|
| **LIC-16** Optional pre-application meeting; FSCA concurrence before any licence (FSRA s.126); registration within 12 mon | External | Open | *none* | No PA contact yet |

## What no document can close

- **Capital** (A3) and **founding members** (A4): money and people.
- **Fit-and-proper** (B2): each person files, and the regulator assesses.
- **Pre-application meeting** (F1) and every approval: the Prudential Authority and FSCA act.
- **Approval of every policy**: a board that does not yet exist.

