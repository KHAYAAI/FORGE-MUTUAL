# Governance and organisation structure

> **DRAFT. Not approved.** This is the structure most South African banks use: a board with its committees, an
> executive committee, and three lines of defence. It is cut down to fit a small, technology-led bank that starts on
> a sponsor bank and applies for its own licence later. Every seat except the founder's is **VACANT**. The roster with
> what each seat needs is in [`roles.csv`](roles.csv) and [`ROLES.md`](ROLES.md). Nothing here is legal advice; a
> regulatory lawyer must confirm it against the Banks Act (or Mutual Banks Act), the Companies Act and the Prudential
> Authority's current requirements.

**Founder:** Tebogo Mvelase, proposed Chief Executive Officer and executive director. The fit-and-proper assessment
(BA 020 / DI 020) has not been done.

## 1. The whole structure

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

## 2. Executive organisation chart

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

## 3. Three lines of defence

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

## 4. Committees: who sits, how often, what they decide

| Committee | Chair | Members | Meets | Decides or recommends | Terms of reference |
|---|---|---|---|---|---|
| Board | Independent chair (VACANT) | All directors | Quarterly, plus ad hoc | Strategy, risk appetite, policies, capital, appointments | [board charter](../regulatory/policies/board-charter.md) |
| Audit Committee | Independent NED, CA(SA) (VACANT) | 3 independent NEDs | Quarterly | Financial statements, returns, internal and external audit, internal controls | [terms](../regulatory/policies/committee-terms-of-reference.md#audit-committee) |
| Risk and Capital Management Committee | Independent NED (VACANT) | 3 NEDs, CEO, CRO, CFO attend | Quarterly | Risk appetite, ICAAP, ILAAP, recovery plan, risk policies, stress tests | [terms](../regulatory/policies/committee-terms-of-reference.md#risk-and-capital-management-committee) |
| IT and Cyber Committee | Independent NED, technology (VACANT) | 2 NEDs, CTO, CISO, Head of IT Risk attend | Quarterly | IT strategy, cyber posture, material IT changes, outsourcing of IT | [terms](../regulatory/policies/committee-terms-of-reference.md#it-and-cyber-committee) |
| Remuneration Committee | Independent NED (VACANT) | 3 NEDs | Twice a year | Pay of executives and control functions; clawback | [terms](../regulatory/policies/committee-terms-of-reference.md#remuneration-committee) |
| Directors' Affairs and Nominations | Board chair (VACANT) | 3 NEDs | Twice a year | Board composition, fit and proper, succession, evaluations | [terms](../regulatory/policies/committee-terms-of-reference.md#directors-affairs-and-nominations-committee) |
| Social and Ethics Committee | Independent NED (VACANT) | 1 NED, 2 others | Twice a year | Companies Act s.72 duties; conduct and customer outcomes | [terms](../regulatory/policies/committee-terms-of-reference.md#social-and-ethics-committee) |
| Board Credit Committee | Independent NED (VACANT) | 2 NEDs, CEO, CRO, Head of Credit | Monthly once credit is live | Large exposures and facilities above the management mandate | [terms](../regulatory/policies/committee-terms-of-reference.md#board-credit-committee) |
| Executive Committee | CEO Tebogo Mvelase | All executives | Weekly | Running the bank within the appetite | [terms](../regulatory/policies/committee-terms-of-reference.md#executive-committee) |
| ALCO | CFO (VACANT) | CEO, CRO, Head of Treasury | Monthly | Liquidity, funding, interest-rate risk, FX positions, rates paid | [terms](../regulatory/policies/committee-terms-of-reference.md#alco) |
| Credit Committee | Head of Credit (VACANT) | CRO, CFO, credit officers | Weekly | Facilities within mandate; arrears and write-offs | [terms](../regulatory/policies/committee-terms-of-reference.md#credit-committee) |
| Operational Risk and Control | CRO (VACANT) | COO, CTO, CISO, CCO | Monthly | Incidents, losses, control failures, risk register | [terms](../regulatory/policies/committee-terms-of-reference.md#operational-risk-and-control-committee) |
| Financial Crime Committee | MLRO (VACANT) | CCO, COO, Fraud lead | Monthly | Monitoring rules, case backlog, exits, sanctions | [terms](../regulatory/policies/committee-terms-of-reference.md#financial-crime-committee) |
| Product Approval Committee | Head of Product (VACANT) | CRO, CCO, CFO, COO, CTO | As needed | New products and changes; signs the licence-gate decision `DEC-nnnn` | [terms](../regulatory/policies/committee-terms-of-reference.md#product-approval-committee) |
| Change Advisory Board | CTO (VACANT) | CISO, Head of IT Risk, COO | Weekly | Production changes; emergency changes after the fact | [terms](../regulatory/policies/committee-terms-of-reference.md#change-advisory-board) |
| Data Governance Council | Head of Data (VACANT) | CTO, CISO, CCO, Information Officer | Quarterly | Data classes, retention, residency, access | [terms](../regulatory/policies/committee-terms-of-reference.md#data-governance-council) |

## 5. Policy hierarchy

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

The full list of policies, their owners and approvers is in [`POLICY_FRAMEWORK.md`](POLICY_FRAMEWORK.md).

## 6. Who can approve what

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

## 7. How the structure grows

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

## 8. Rules for sharing seats while small

- The CEO may not also be the chair, the CRO, the MLRO or the head of internal audit.
- The CRO may not be the CFO or run any first-line function.
- The MLRO and CCO may be one person only if the Prudential Authority and the sponsor bank accept it; then the Deputy
  MLRO must be appointed, because someone else must approve the MLRO's own changes.
- Internal audit may be outsourced, but only with the PA's prior written approval (register row BR-05), and never to
  the external auditor.
- At least two board members must be employees (register row MR-03 for a mutual bank): the CEO and the CFO.
- A majority of the board, and every chair of the audit and risk committees, should be independent non-executives.
