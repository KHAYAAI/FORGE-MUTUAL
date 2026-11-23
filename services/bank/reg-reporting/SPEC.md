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

## Prudential returns framework (`internal/returns`)

The frame only: line computation from named inputs, form arithmetic checks, reconciliation to other sources, preparer plus
a different approver, tied to an evidence digest. It holds no line definitions. `Submittable` returns `ErrUnverified` until each
line is verified against the form instructions by a named person (VERIFY-FORMS). Nothing is sent to the PA.

## Not started

- Balance of Payments data handed to the AD partner (VERIFY the division of responsibility).
- The prudential return line definitions (BA 100 to BA 950, DI forms) and their submission channel.
- The report builders that read compliance cases and ledger balances into a pack.
- Any binary, schedule or ConnectRPC API.

## Tests

`make test-reg-reporting`.
