# deposits-service

Entity: BANK. Savings and notice deposits. API: `proto/forge/deposits/v1/deposits.proto`.

Status: built and tested against Postgres and a ledger fake that refuses overdrawing, and run on the local stack.
**Off in production** until `FORGE_APPROVAL_SAVINGS` and `FORGE_APPROVAL_NOTICE_DEPOSITS` name the approving
decisions (pkg/licence): interest-bearing deposits need treasury and regulatory sign-off.

## Products

| Product | Rate (placeholder) | Withdrawals |
|---|---|---|
| `savings` | 6.50% | at once |
| `notice-32` | 7.75% | 32 days' notice |
| `notice-90` | 8.50% | 90 days' notice |

Rates are threshold 17 in `docs/THRESHOLDS.md`; `DEPOSITS_RATES` overrides them.

## Rules

- A deposit account is a CUSTOMER_DEPOSIT ledger account (1001, debits may not exceed credits) with the product as its
  product ID. Opening is keyed by party and product: one each, and a repeat replays.
- Money moves only between the party's own rand current account (checked with consolidated-view) and its own deposit
  account, as INTERNAL_TRANSFER (12), under `deposits:<party>:<client key>`. The same key with a different request is 409.
- A withdrawal locks the account row, then checks the ledger balance less money already under notice, so concurrent
  requests cannot spend the same money. On a notice account it records a notice instead, due after the notice period,
  and the money is paid on the due date under `notice:<id>`; a notice can be cancelled until then.
- **Interest** accrues daily on the balance, ACT/365, in nano-cents (`internal/interest`), and is paid on the 1st from
  INTEREST_EXPENSE (2007) as INTEREST_CREDIT (65) under `interest:<account>:<YYYY-MM>`. What rounds away carries to the
  next month. The payment is recorded PENDING before the ledger is asked. Accrual uses the balance when the day is
  accrued, just after midnight; days the service missed (up to 7) are accrued late on the balance then, and marked.

## Not done

- Rates are placeholders; rate changes apply from the next accrual, with no notice to customers.
- No tax certificate (IT3(b)) or withholding.
- Accrual on the balance at accrual time, not the ledger's end-of-day history.
