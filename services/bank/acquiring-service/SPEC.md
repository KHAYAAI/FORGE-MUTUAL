# acquiring-service

Entity: BANK. Card acceptance for merchants: payment links and point-of-sale payment requests, the payer's checkout,
the acquirer's webhook, next-day settlement and refunds. API: `proto/forge/acquiring/v1/acquiring.proto`.

Status: built and tested against Postgres and a ledger fake, and run on the local stack with the acquirer simulator.
**Not usable in production**: no acquiring bank is chosen (VERIFY-ACQUIRER), production refuses the simulator, and
`FORGE_APPROVAL_ACQUIRING` must name the approving decision (pkg/licence).

## Rules

- **Card details never reach Forge.** The payer is sent to the acquirer's hosted checkout. Forge holds only what the
  merchant asked for and what the acquirer reported, which keeps Forge out of PCI DSS cardholder-data scope.
- A link has a 160-bit random token, which is the payer's only credential: the public checkout shows only the
  merchant's display name, the amount, the description and the reference. The web app forwards checkout calls with
  no identity at all, and refuses a malformed token before calling anything.
- An approval arrives on `POST /webhooks/acquirer`, signed (the simulator: HMAC-SHA256 in `X-Forge-Signature`). It
  pays a link once, and only for the link's own amount. Anything unsigned is 401.
- **Settlement:** every day's takings settle the next day, per merchant and settlement account: gross from
  ACQUIRER_SETTLEMENT (3020) as ACQUIRING_SETTLEMENT (52), less the merchant discount to fees (FEE, 30), as one chain
  under the settlement's key. The grouping is recorded before the ledger is asked.
- **Refunds** are for settled payments, up to what is left. The merchant's account is debited first (REVERSAL, 90)
  under the refund's key, then the acquirer refunds the card; a retry with the same key finishes an unconfirmed
  refund and never debits twice.

## Not done

- A real acquirer adapter, its webhook signature and its settlement file; reconciliation of ACQUIRER_SETTLEMENT
  against the acquirer's payouts; chargebacks; card-present terminals (point-of-sale here is a payment page shown at
  the counter); 3-D Secure is the acquirer's.
