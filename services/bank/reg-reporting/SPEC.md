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

## Not started

- Balance of Payments data handed to the AD partner (VERIFY the division of responsibility).
- Prudential returns, which apply only once Forge is the licensed bank (Stage B).
- The report builders that read compliance cases and ledger balances into a pack.
- Any binary, schedule or ConnectRPC API.

## Tests

`make test-reg-reporting`.
