# lending-service

Entity: BANK. Overdraft facilities, working-capital term loans and revenue-based financing, with a credit desk for
staff. API: `proto/forge/lending/v1/lending.proto` (`LendingService`, `CreditDeskService`).

Status: built and tested against Postgres and a ledger fake, and run on the local stack. **Off in production** until
`FORGE_APPROVAL_OVERDRAFT`, `FORGE_APPROVAL_TERM_LOANS` and `FORGE_APPROVAL_REVENUE_BASED` name approving decisions:
lending needs a credit policy, National Credit Act registration (for lending to persons and small businesses), and
capital. The terms and the affordability rule are placeholders (thresholds 18 to 20).

## Flow

1. **Apply.** The customer names a product, amount, term (loans) and the rand account to pay into and collect from.
   The application is assessed at once on the account's last 90 days of money received (codes 10, 52, 62 only; internal
   moves and loan payouts do not count), against the placeholder affordability rule.
2. **Decide.** A credit officer (back-office role `credit`) approves, possibly for less, or declines, with reasons. The
   applicant can never decide their own application (checked in code and by a database constraint). The note is for
   staff; the customer never sees it.
3. **Accept.** The customer accepts the approved offer, which shows the total repayable. Only then is a CUSTOMER_LOAN
   account (1020) opened and the money paid out as LOAN_DISBURSEMENT (66), under the facility's key: once.
4. **Repay.**
   - Term loan: equal monthly instalments (annuity, `internal/credit`, exact rationals, interest rounded down),
     collected from the current account: principal to the loan account (67), interest to INTEREST_INCOME (2008, code
     68), as one chain. An instalment the account cannot cover goes into ARREARS, is retried daily, and the facility
     reports days past due. Early settlement must be in full; future interest is waived.
   - Overdraft: draws up to the limit, checked under a row lock. Interest accrues daily on the drawn balance and is
     charged monthly by adding it to the drawn balance.
   - Revenue-based: a fixed fee; each payment received after acceptance is swept at the agreed share, split between
     fee and principal in proportion, until both are paid. The sweep cursor moves only after the sweep is posted.

## Not done

- No credit bureau check, no NCA pre-agreement statement document, no collections process beyond retrying, no
  write-off (WRITE_OFF exists in the ledger, with dual control upstream, but is not wired here).
- Overdraft interest is charged without regard to the limit; a facility can exceed its limit by its interest.
