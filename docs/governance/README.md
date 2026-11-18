# Governance pack

> **DRAFT. Nothing here is approved, and only one seat is filled.** This is the structure, roster and policy set a
> South African bank is expected to have, cut down to fit Forge. It is the to-do list for building the institution
> around the software. Have a regulatory lawyer confirm it before it goes to the Prudential Authority.

| File | What it is |
|---|---|
| [`STRUCTURE.md`](STRUCTURE.md) | Diagrams: board and committees, executive chart, three lines of defence, policy hierarchy, approval matrix, hiring by stage |
| [`ROLES.md`](ROLES.md) / [`roles.csv`](roles.csv) | All 36 seats, who holds each (Tebogo Mvelase as founder and CEO; everyone else **VACANT**), what each seat needs, and which seats may never be shared |
| [`POLICY_FRAMEWORK.md`](POLICY_FRAMEWORK.md) / [`policies.csv`](policies.csv) | All 52 policies, their owner, approver, review cycle and the register rows they answer |
| [`../regulatory/policies/`](../regulatory/policies/) | The policy drafts themselves, each with a sign-off block |
| [`LEGAL_FORMS.md`](LEGAL_FORMS.md) | Mutual bank (planned) and bank (kept open) side by side, with both licence paths |
| [`RISK_APPETITE_DECISIONS.md`](RISK_APPETITE_DECISIONS.md) | 30 risk appetite numbers with cautious, recommended and bolder options, for the owner to decide |
| [`LAWYER_BRIEF.md`](LAWYER_BRIEF.md) | What to ask a regulatory lawyer, how to choose one, where to look, and a first email |
| [`../regulatory/register.csv`](../regulatory/register.csv) | The 114 regulatory requirements, and where we stand on each |

`make governance` rebuilds `ROLES.md` and `POLICY_FRAMEWORK.md` from the two CSVs. The gate (`make test-regulatory`)
fails if they are out of date, if a policy file is not listed, or if a register id does not exist.

## Filling a seat

1. Make the appointment in writing (board resolution for directors and executives).
2. Complete the fit-and-proper assessment (`../regulatory/policies/fit-and-proper-procedure.md`); for a director or
   executive officer, file BA 020 / DI 020 with the Prudential Authority.
3. Replace `VACANT - to be appointed` with the person's name in `roles.csv`, set `status` to `Appointed YYYY-MM-DD`,
   run `make governance`, and put the same name against their thresholds in `docs/THRESHOLDS.md`.

## Everything we will need

### People (see `ROLES.md`)

- **Now:** a regulatory lawyer, a sponsor-bank relationship.
- **Before the first customer (sponsor model):** MLRO and deputy, Chief Compliance Officer, Chief Risk Officer, CTO,
  CISO, Deputy Information Officer, Head of Payments Operations.
- **Before the licence application:** board chair and at least 4 independent directors, CFO, Financial Controller,
  regulatory reporting, Company Secretary, internal audit (in-house or outsourced with PA approval), Head of Treasury,
  Head of IT Risk, external auditors.
- **Before credit and the public launch:** Head of Credit and credit officers, Head of Product, customer service and
  complaints, fraud lead, business continuity manager, Head of People.

### Decisions only the owner can make

- [~] Legal form: **mutual bank planned, bank kept open** (owner's direction 2026-11-18; ADR-0020 open until legal advice). See `LEGAL_FORMS.md`.
- [ ] Sponsor bank, or straight to a licence.
- [ ] Hosting provider and region (ADR-0002). Decides material outsourcing and offshoring notices.
- [ ] Risk appetite: complete `RISK_APPETITE_DECISIONS.md` (30 numbers, each with a recommendation).
- [ ] Sector appetite (ADR-0012), including how far to go with crypto businesses.
- [ ] Which products to switch on first (`new-product-approval-policy.md`).

### Money

- [ ] Initial capital with written proof (LIC-02).
- [ ] Budget for the first 18 months of salaries for the seats above, audit fees, legal fees, the penetration test,
  provider fees and production infrastructure.

### Registrations and accounts

- [ ] Company registration (CIPC) in the chosen legal form; a name that does not use "bank" until authorised.
- [ ] Prudential Authority pre-application meeting; FSCA concurrence.
- [ ] FIC registration as an accountable institution and goAML access (or through the sponsor).
- [ ] Information Officer registered with the Information Regulator.
- [ ] NCR registration as a credit provider, before any lending.
- [ ] Authorised-dealer arrangement for FX and SARB cross-border reporting.
- [ ] Ombudsman for Banking Services membership.
- [ ] PASA participation, or through the sponsor, for payments.
- [ ] Corporation for Deposit Insurance membership once licensed.

### Providers and contracts (each needs due diligence and a contract, `provider-due-diligence-procedure.md`)

- [ ] Sponsor bank; partner bank for USD, EUR and GBP; acquiring bank.
- [ ] WorkOS (sign-in) and Didit (KYC/KYB): accounts on the Forge email, data location confirmed.
- [ ] Sanctions and PEP list source; credit bureau; hosting; email and SMS; whistleblowing hotline; penetration-testing firm.

### Documents still to write beyond these drafts

- [ ] Articles of association (or mutual bank constitution) and shareholder agreement.
- [ ] Financial projections for 3 years, with the capital plan.
- [ ] Customer terms and conditions, fee schedule, PAIA manual, credit agreements and pre-agreement statements.
- [ ] Pro forma returns (BA or DI forms) and the first ICAAP.
- [ ] Employment contracts, delegations of authority and job descriptions for each seat.
