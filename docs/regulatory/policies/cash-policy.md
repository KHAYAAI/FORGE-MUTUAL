> **DRAFT for board approval. Not approved. Not in force.** Prepared on 2026-11-23. Figures marked PROPOSED are for the owner. Nothing here is legal advice.

# Cash policy

| | |
|---|---|
| Owner | Money Laundering Reporting Officer (R06, VACANT) |
| Approved by | Board, on Risk and Capital Management Committee recommendation |
| Review | Yearly, and before any cash channel is added |
| Register rows | FIC-07, FIC-10 |
| Version | 0.1 draft |

**Purpose.** Forge takes no cash. This policy says so, says what would have to be true before that changes, and keeps
cash-threshold reporting (FIC Act s.28) ready if it ever does.

## Position

1. **Forge has no branches, ATMs, tills or cash-handling staff, and accepts no cash.** No employee may take cash from a
   customer for any reason, including as a favour or to pass on.
2. A customer who needs to deposit cash does so at the sponsor bank's or a partner's channel, which is that
   institution's transaction and reporting duty (to be confirmed with the sponsor, register row FIC-07). The money reaches
   Forge as an ordinary inbound payment.
3. Cash withdrawals: none. Cards issued by Forge, if ever switched on, may not be used at ATMs (`card-service`: cash
   withdrawals are not supported).

## If cash enters through a partner

An inbound credit whose source is a cash deposit is higher risk and is treated so:

- It carries a channel flag from the partner; **a credit with no flag cannot be assumed not to be cash**: ask the
  partner whether the flag exists before relying on this policy.
- Structuring (several deposits just under a threshold) is already a monitoring rule (`docs/THRESHOLDS.md`, threshold 7);
  cash-flagged credits are in scope of that rule from the first rand.
- If Forge is the reporting institution for any cash transaction, the MLRO files a cash threshold report with the FIC
  through goAML within the legal time, and the compliance engine opens a case for every cash credit above the threshold
  (R49 999.99 at last reading; VERIFY the current figure). The goAML route is not confirmed (VERIFY-GOAML).

## Before any cash channel is added

The owner, the CCO and the MLRO must all sign off, after: the sponsor bank and lawyer confirm who is the accountable
institution; the monitoring rule and the reporting step are built and tested; staff are trained; and the risk appetite
statement is amended. Until then, no product or marketing may mention cash.

## What is built

Structuring monitoring and case opening exist; a cash-flag field and a cash-threshold report do not. Nothing in the
platform can take cash, by design.

## Sign-off

| Role | Name | Signature | Date |
|---|---|---|---|
| Owner | VACANT | | |
| Approver (Board) | VACANT | | |
| Second line reviewer | VACANT | | |
