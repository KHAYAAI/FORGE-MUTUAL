> **DRAFT for board approval. Not approved. Not in force.** Prepared on 2026-11-18. Nothing here is legal advice; the Head of Legal (or external counsel) settles the wording.

# Provider contract schedule: minimum clauses

| | |
|---|---|
| Owner | Head of Legal (R26, VACANT) |
| Approved by | Board, on Audit Committee recommendation |
| Review | Yearly |
| Register rows | D3-05, D3-07, OUT-03, OUT-06, JN-04, POP-08 |
| Version | 0.1 draft |

**Purpose.** Every material provider contract must contain these clauses, or a written, approved reason why not. The
outsourcing register records `contract_schedule_met = Yes` only when the Head of Legal has checked each one, and
`scripts/check-outsourcing.py` refuses a material provider in use without it.

## Clauses

| # | Clause | What it must say | Source |
|---|---|---|---|
| 1 | Scope and service levels | The services, measurable service levels, reporting, and remedies for missing them | OUT-06 |
| 2 | Regulator access | The PA, FSCA, FIC and Information Regulator, and Forge's auditors, may access premises, people, systems and records, including for forensic audit, without obstruction | D3-05, OUT-06 |
| 3 | Audit rights | Forge and its internal and external auditors may audit, or rely on independent reports (SOC 2 Type II, ISO 27001) with the right to audit on cause | OUT-06 |
| 4 | Data location | Where data is processed and stored; no move without Forge's prior written consent; South Africa only for classes so marked in `data-classes.csv` | D3-03, POP-08 |
| 5 | Personal information | Operator duties under POPIA s.20 and s.21; transborder transfer basis under s.72 | POP-08 |
| 6 | Security | Minimum controls, encryption, access logging, vulnerability management, penetration testing | JS2 |
| 7 | Incident notice | The provider tells Forge of an incident affecting Forge **within 4 hours** (PROPOSED) of detection, with enough detail for Forge to meet its own notice duties; the provider's notice to a regulator does not replace Forge's | JN-04 |
| 8 | Sub-contracting | No material sub-contracting without consent; the same clauses flow down | OUT-06 |
| 9 | Business continuity | The provider keeps and tests a continuity plan that meets Forge's recovery objectives (`business-impact-analysis.md`) and shares the results | G4-02, OUT-05 |
| 10 | Resolution and recovery | The provider may not terminate or change terms because Forge enters recovery or resolution, and continues service to a resolution authority or successor | OUT-06 |
| 11 | Termination and exit | Termination rights for Forge, including on regulator instruction; exit assistance; return and certified deletion of data | OUT-06, `exit-plans.md` |
| 12 | Confidentiality | Customer confidentiality survives the contract | OUT-06 |
| 13 | Compliance with law | The provider complies with the laws that apply to the service, including FICA and sanctions | OUT-06 |
| 14 | Governing law and disputes | South African law and courts, or a stated arbitration seat | D3-07 |
| 15 | Insurance and liability | Professional indemnity and cyber cover; liability caps that do not defeat the clauses above | OUT-06 |
| 16 | Binding agreement | A signed, legally binding agreement, not click-through terms, for every material arrangement | D3-07 |

## Sign-off

| Role | Name | Signature | Date |
|---|---|---|---|
| Owner | VACANT | | |
| Approver (Board) | VACANT | | |
| Second line reviewer | VACANT | | |
