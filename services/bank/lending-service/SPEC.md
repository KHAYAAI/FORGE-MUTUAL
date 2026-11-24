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

## Credit controls

- **Credit bureau check.** Every application asks the bureau (`BUREAU`), with the customer's consent (`credit_bureau_check`, scope
  `enquiry`). No consent means no enquiry; no answer from the bureau means no assessment (fail closed); an adverse record needs a
  written reason of at least 30 characters to approve. Production refuses the simulator and, with no real bureau, refuses applications.
- **Pre-agreement statement and quotation (NCA s.92).** `GetPreAgreement` generates it from the approved terms; the customer acknowledges
  its hash in `AcceptOffer`, and a changed statement invalidates the acknowledgement. It says plainly that Forge is not yet registered as
  a credit provider until `NCR_REGISTRATION` is set.
- **Write-off** (`WriteOffFacility`, credit desk). An active facility at least 90 days past due (`LENDING_WRITE_OFF_AFTER_DAYS`; threshold 31),
  after a default notice has been issued, written off by two different people, neither the applicant. The loan account is credited and
  WRITE_OFF (2006) debited with transfer code 91, once; the status becomes WRITTEN_OFF, collection stops, and it shows on the statement.

## Not done

- No collections process beyond retrying and default notices, and no recoveries after a write-off (a written-off debt is still owed in law).
- A real credit bureau: only the simulator is built (VERIFY-BUREAU), and production refuses it and refuses every application until a real
  adapter exists. The pre-agreement wording is a draft that the lawyer must confirm against NCA s.92 and its regulations.
- The customer app does not yet fetch the pre-agreement statement or send its hash; `LENDING_REQUIRE_PRE_AGREEMENT` is on in production,
  so the web app must do this before credit is switched on there.
- Overdraft interest is charged without regard to the limit; a facility can exceed its limit by its interest.
