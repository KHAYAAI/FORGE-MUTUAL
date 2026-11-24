# reg-reporting (services/bank/reg-reporting)

Entity: BANK. Architecture: ARCHITECTURE.md 15.2. v1 scope: goAML reports and evidence packs.

**Status: the evidence pack is built; goAML submission is deliberately refused.** There is no binary and no
ConnectRPC surface yet.

## Evidence packs (`internal/evidence`)

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

## goAML (`internal/goaml`)

`Submit` always returns `ErrSchemaUnverified`. The official goAML schema and the submission process are unverified
(VERIFY-GOAML), so no report can reach the regulator from this code. This is intentional. A report that cannot be
validated must not be sent. A test asserts the refusal on every payload.

## Incident notification (`internal/incident`)

Joint Notice 2 of 2026. Models an IT or cyber incident from detection through classification, the 24-hour immediate notice,
the 14-day update and the full report, with deadline tracking (`Obligations`) and a CSV export of the Annexure A tabs. The
institution classifies materiality; the 24 clock hours run from that decision. Notify needs complete tabs and an approver who
differs from the preparer. It does not submit to the PA's Umoja portal (VERIFY-UMOJA); a person does. The 14 days are counted
as 14x24 hours from notification (confirm with the PA).

## Prudential returns (`internal/returns`)

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
- **Still to build:** DI 500 (credit risk, with the asset-quality and sector distribution of loans) and DI 900 (institutional and
  maturity breakdown, a statistical return of 8 pages). BA returns stay on this repository's own
  references (`statements.go`) while the bank route is only kept open.

## Not started

- Balance of Payments data handed to the AD partner (VERIFY the division of responsibility).
- The 13 DI forms still to build, the BA forms if the bank route is chosen, and the submission channel (the PA's Umoja portal).
- The report builders that read compliance cases and ledger balances into a pack.
- Any binary, schedule or ConnectRPC API.

## Tests

`make test-reg-reporting`.
