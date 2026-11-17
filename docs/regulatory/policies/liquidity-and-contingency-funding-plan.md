> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# Liquidity risk framework and contingency funding plan (Banks Act regulations)

Register ids: LIC-04, BR-04.

**Measurement.** treasury-risk computes a 30-day liquidity coverage ratio daily from the ledger, with run-off rates by
kind of deposit (thresholds 24), and five stress scenarios: moderate and severe runs, the largest depositor leaving,
and credit losses of 10% and 25% (back office, *Risk*). Concentration and net-outflow alerts run every 15 minutes.

**Appetite (PROPOSED).** LCR at least 120% (minimum 100%); no single depositor above 10% of deposits; survive the
severe run for 30 days.

**Early warning.** Any ratio within 10% of its minimum, a concentration or outflow alert, or a failed scenario
escalates to the Head of Treasury the same day.

**Contingency funding (PROPOSED actions, in order).** 1. Stop new lending (switch the credit products off with
`pkg/licence`). 2. Draw any committed facility from the sponsor bank (to be negotiated). 3. Stop discretionary
payouts. 4. Inform the Prudential Authority and the sponsor. 5. Recovery plan (`recovery-plan.md`).

**Not yet built:** the report does not read undrawn overdrafts, notices given or instalments due (listed on the report);
there is no committed funding line.
