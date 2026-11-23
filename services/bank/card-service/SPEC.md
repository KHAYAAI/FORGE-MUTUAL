# card-service

Entity: BANK. Virtual debit cards on a customer's own rand account, and the real-time decision on every purchase.
API: `proto/forge/cards/v1/cards.proto`. Processor webhook: `POST /webhooks/processor`.

Status: built and tested against Postgres and a ledger fake (11 tests, including a concurrent-purchase race and
recovery from an unreachable ledger), and wired into the local stack with a processor simulator. **Not usable in
production**: no card processor or programme manager is chosen (VERIFY-PROCESSOR), production refuses the simulator, and
`FORGE_APPROVAL_CARDS` must name the approving decision (`pkg/licence`). **Wired to the customer app, but not switched on**: the workspace API has the card routes and the web app a `/cards` page, but `CARDS_URL`
is unset (so the routes answer "not offered"), no navigation link points to the page, and the stack runs with the processor simulator. Only workspace-api may call it
(`deploy/compose/config/callers/cards.json`).

## Rules

- **Card numbers never reach Forge.** The processor creates the card and holds the number; Forge keeps a token, the last
  four digits and the expiry. The cardholder sees the number on the processor's page (`DisplayCard` returns its address).
  That keeps Forge out of PCI DSS cardholder-data scope for issuing, as for acquiring.
- **Approve only what the account can cover, at the moment of purchase.** An authorisation is approved only if the card
  is active, the party is ACTIVE (a frozen party cannot spend), the currency is rand, the merchant category is not
  blocked, the amount is within the per-transaction and daily limits, and the ledger can hold the amount. The hold is a
  pending transfer (code 50, CARD_AUTH_HOLD) from the customer's account to CARD_SCHEME_SETTLEMENT (3030), keyed by the
  processor's authorisation id, so asking again never holds twice.
- **A decline never leaves money held.** Declines are recorded with a reason for the cardholder (never a
  financial-crime reason). If the ledger cannot be asked the answer is "declined, temporarily unavailable", and
  `Recover` later releases anything that was held for it.
- **Limits are decided under one lock per card**, with the new authorisation recorded in the same step, so purchases at
  the same moment cannot together pass a limit only one fits under. The daily limit runs on South African time.
- **Clearing** posts the hold when the amounts match. A smaller clearing releases the hold and posts the smaller amount
  (code 51). A larger one posts nothing and goes to an operator (`EXCEPTION: OVER_CAPTURE`). Clearing for a declined or
  reversed authorisation also goes to an operator. A hold that expired is posted straight from the account if the funds
  are there. **Reversal** releases the hold. **Refund** credits the account, up to what was captured less what was
  already refunded, once per refund id.
- A customer sees and changes only their own cards: another party's card looks absent. A closed card stays closed and is
  closed at the processor too. Limits can be lowered or raised only up to the bank's ceilings.

## Ledger

Account code 3030 CARD_SCHEME_SETTLEMENT (system account); transfer codes 50 CARD_AUTH_HOLD and 51 CARD_SETTLE. The
service may create accounts, create transfers, post and void pending transfers, and nothing else on the ledger.

## Not done

- A real processor or programme-manager adapter, its webhook signature and its settlement and clearing files.
- Reconciliation of CARD_SCHEME_SETTLEMENT against the processor's settlement, and the daily net settlement to the
  sponsor pool.
- Physical cards, PIN, 3-D Secure and tokenised wallets (Apple Pay, Google Pay): the processor's.
- Foreign-currency purchases (declined today), cash withdrawals, chargebacks and disputes, card fees and interchange.
- A back-office view of cards, and a navigation link to `/cards` (add it when cards are switched on).
- Card limits and blocked categories are placeholders (`docs/THRESHOLDS.md`, thresholds 29 and 30).
- Confirmation from the sponsor bank and the Prudential Authority that Forge may issue cards at all.
