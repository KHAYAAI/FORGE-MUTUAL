# Regulatory requirement register

`register.csv` lists what South African banking regulation asks of a bank or mutual bank, one requirement per row,
and where this repository stands against each. It is a gap list. It does not say Forge complies with anything.

Check it with `python3 -I scripts/check-regulatory-register.py`. That checks the columns, the allowed values, unique
ids and that every evidence path exists. It does not check that the evidence satisfies the requirement.

## Columns

| Column | Meaning |
|---|---|
| `id` | Stable id. Prefix names the source: LIC licensing guide, BR Banks regulations, MR Mutual Banks regulations, D3 Directive 3/2018, G52 Guidance Note 5/2018, OUT Guidance Note 5/2014, G4 Guidance Note 4/2017, JN Joint Notice 2/2026 and template, JS2 Joint Standard 2/2024, RET returns, GAP not yet read. |
| `source`, `ref` | The document and the regulation, paragraph or section. |
| `route` | `Bank` (Banks Act), `Mutual` (Mutual Banks Act), `Both`. `Sponsor-flowdown` is reserved for requirements that reach Forge only through a sponsor bank contract. No row uses it yet. |
| `type` | `Code` (work in this repository), `Document` (policy, plan or filing a person must own), `External` (a third party or regulator acts), `Decision` (an owner decision). |
| `status` | `Open`, `Partial`, `Evidenced` (needs a path in `evidence`), `External`. No row is `Evidenced` yet. |
| `evidence` | Repository paths, separated by `;`, that a reviewer should open. |
| `gap_or_next_step` | What is missing. |
| `read_basis` | `Full` (read in full), `Excerpt` (searched and read in part), `Secondary` (quoted by a vendor checklist, verify against the original), `Not read`. |

## Limits

- The Mutual Banks regulations used here are the original 1993 gazette (R.2508), read by OCR. Confirm the current text
  and any amendments with the Prudential Authority.
- The Banks regulations (R.1029, 2012) are 1,221 pages. The risk, governance, internal audit and returns rules were read.
  The form-completion instructions for BA 100 to BA 950 were not.
- Guidance Notes 5/2014 and 5/2018 were read in part. Rows marked `Secondary` rely on an Oracle checklist that quotes
  them.
- Joint Standard 1 of 2023 (IT governance and risk) has not been read. FICA, POPIA, the National Payment System Act,
  exchange control, the National Credit Act and deposit insurance are not covered by any source read (`GAP-01`, `GAP-02`).
- `Partial` means some related control exists in the repository according to its own README and specs. It has not been
  independently tested against the requirement.
- Forge is not licensed. While it operates on a sponsor bank, most of these obligations bind the sponsor, and reach Forge
  through its contract.
