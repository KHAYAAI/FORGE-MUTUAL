# Threshold owners: mock roster and hiring advice

> **MOCK. Every name below is invented.** This page shows the shape of the team that must own Forge's original 16 placeholder
> thresholds (`docs/THRESHOLDS.md`). Replace each name with a real, appointed person. No threshold may go live on a
> mock name.

## Why 16 people, not 5

The register needs five owner roles, but one person per role is not enough for a bank:

- **Four eyes.** A threshold change is proposed by its owner and approved by a second person who did not propose it.
  The approver is from risk or compliance (the second line), not the business (the first line).
- **Cover.** The MLRO and payments operations need a named deputy for leave and incidents. A case SLA of 24 hours
  cannot wait for one person's holiday to end.
- **Independence.** Internal audit reviews the thresholds and the changes to them, and must own none of them.
- **Regulators expect named people.** In South Africa the Prudential Authority assesses the key people of a bank as fit
  and proper, and the FIC Act requires an accountable institution to appoint a compliance officer. Confirm the exact
  requirements with a regulatory lawyer. This page is not legal advice.

## The mock roster

| # | Mock name | Role | Line | What they own or approve |
|---|---|---|---|---|
| 1 | Ayanda Mokoena | Chief Executive Officer | Executive | Accountable to the board for the risk appetite; signs the threshold register once a year |
| 2 | Pieter van der Berg | Chief Financial Officer | First | Approves the treasury thresholds (1, 2) |
| 3 | Lerato Khumalo | Head of Treasury | First | Owns 1 and 2: concentration and net outflow |
| 4 | Sipho Ndlovu | Chief Risk Officer | Second | Approves 10 and 11; chairs the threshold review |
| 5 | Fatima Patel | Head of Product and Credit Risk ("Risk owner") | First | Owns 10 and 11: transaction limits by tier, the KYC tier threshold |
| 6 | Nomvula Dube | MLRO and FIC compliance officer | Second | Owns 4 to 9 and 13: case SLAs, monitoring rules, sanctions list age |
| 7 | Johan Botha | Deputy MLRO | Second | Covers for the MLRO; approves the MLRO's own changes when the CCO is away |
| 8 | Thabo Molefe | Financial Crime Analytics Lead | First | Tunes the monitoring rules (6 to 9) with data and proposes changes to the MLRO |
| 9 | Zanele Mahlangu | Chief Compliance Officer | Second | Approves 4 to 9 and 13 |
| 10 | Kagiso Sithole | Head of Operations | First | Owns 3: suspense ceiling |
| 11 | Priya Naidoo | Head of Payments Operations | First | Owns 12 and 14 to 16: reconciliation age, payout limits and polling |
| 12 | Bongani Zulu | Payments Operations Lead | First | Covers for the Head of Payments Operations |
| 13 | Megan Jacobs | Chief Technology Officer | First | Confirms that each value is technically safe (12, 14, 16); owns the change process in code and config |
| 14 | Tshepo Mabaso | Chief Information Security Officer | Second | Approves who can change a threshold, and reviews the audit trail of changes |
| 15 | Ruth Abrahams | Information Officer (POPIA) | Second | Consulted where a threshold changes what personal data is collected or kept (10, 11) |
| 16 | Daniel Okafor | Head of Internal Audit | Third | Reviews the register and every change each year; owns nothing |

## Each threshold, owner and approver

| # | Threshold | Owner (proposes) | Approver (second pair of eyes) |
|---|---|---|---|
| 1 | Concentration: largest customer share of deposits | Head of Treasury (3) | CFO (2), with CRO (4) |
| 2 | Net outflow over 24 hours | Head of Treasury (3) | CFO (2), with CRO (4) |
| 3 | Suspense ceiling | Head of Operations (10) | CFO (2) |
| 4 | Case SLA: critical | MLRO (6) | CCO (9) |
| 5 | Case SLA: warning | MLRO (6) | CCO (9) |
| 6 | Velocity: payouts in 24 hours | MLRO (6), on analysis by (8) | CCO (9) |
| 7 | Structuring band and count | MLRO (6), on analysis by (8) | CCO (9) |
| 8 | New beneficiary, large first payment | MLRO (6), on analysis by (8) | CCO (9) |
| 9 | Rapid in and out | MLRO (6), on analysis by (8) | CCO (9) |
| 10 | Limits by tier | Risk owner (5) | CRO (4), MLRO (6) consulted |
| 11 | KYC tier threshold | Risk owner (5) | CRO (4), MLRO (6) consulted |
| 12 | Reconciliation: maximum age of a run | Head of Payments Operations (11) | CFO (2), CTO (13) consulted |
| 13 | Sanctions list maximum age | MLRO (6) | CCO (9) |
| 14 | Payout batch: maximum items | Head of Payments Operations (11) | CRO (4), CTO (13) consulted |
| 15 | Payout: maximum amount per item | Head of Payments Operations (11) | CRO (4), MLRO (6) consulted |
| 16 | Payout: polls before an item stays open | Head of Payments Operations (11) | CTO (13) |

## What kind of people to look for

**Hire first, before any customer:** the MLRO (6), the Chief Risk Officer (4) and the Chief Compliance Officer (9).
Regulators and sponsor banks ask for them by name, and they own or approve 10 of the 16 thresholds. A small bank can
start with one person as CCO and MLRO only if the regulator accepts it; then the deputy MLRO (7) becomes essential,
because someone else must approve the MLRO's own changes.

- **MLRO and deputy (6, 7):** South African AML experience at a bank or payment provider; has filed reports with the
  FIC through goAML; knows the FIC Act and its guidance notes. Senior enough to refuse the CEO.
- **CRO and Risk owner (4, 5):** has run a risk appetite framework and set transaction limits at a bank or e-money
  issuer; comfortable with data, not only policy.
- **CCO (9):** regulatory relationships with the Prudential Authority and the FSCA; has taken a firm through a licence
  application or a sponsor-bank due diligence.
- **Head of Treasury and CFO (2, 3):** liquidity and ALM at a bank; knows the prudential returns.
- **Operations and payments (10, 11, 12):** has run payment operations on South African rails (EFT, RTC, PayShap) with
  a sponsor bank; owns reconciliation and incidents.
- **Financial crime analytics (8):** builds and tunes monitoring rules from data, and can show the false-positive rate
  of a change before it is made.
- **CTO and CISO (13, 14):** have run regulated production systems; the CISO knows the Prudential Authority's
  cybersecurity standards and has run a penetration test programme.
- **Information Officer (15):** POPIA experience; registered with the Information Regulator.
- **Internal audit (16):** independent of everyone above; can be outsourced to a firm at the start.

**Where people can share roles at the start:** the CEO can be the Information Officer; internal audit can be
outsourced; the deputy roles (7, 12) can be part-time or contracted. The MLRO, the CRO and the person who approves
the MLRO's changes cannot be the same person.

## To make it real

1. Appoint each person in writing, and put their real name in `docs/THRESHOLDS.md`.
2. Each owner confirms their values in writing, and the approver countersigns.
3. Record the date in `docs/DECISIONS.md` and move the status to "Confirmed".
