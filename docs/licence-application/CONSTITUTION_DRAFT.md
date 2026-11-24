# Forge Mutual Bank: draft constitution (rules)

> **DRAFT FOR THE LAWYER. Not legal advice. Not approved. Not in force. Not for signature.**
> Prepared 2026-11-24 from `docs/governance/LEGAL_FORMS.md`, `docs/CONTEXT.md` and the rules the membership module already enforces
> (`services/bank/membership`). It is a first working text so that the regulatory lawyer has something to correct instead of starting from
> nothing. **The Mutual Banks Act 124 of 1993 and its regulations set mandatory content and process for a mutual bank's constitution; this draft
> does not reproduce them, and every place where the Act decides is marked `[ACT]`.** The lawyer must add those provisions, delete anything
> the Act forbids, and confirm every `VERIFY`. Square brackets `[ ]` are decisions the founder or the founding members must make.
>
> Register rows: LIC-01, LIC-07, MR-03. Pack section: A1. ADR-0020 stays open until the lawyer advises.

## How to read this draft

| Marker | Meaning |
|---|---|
| `[ACT]` | The Mutual Banks Act or its regulations decide this. The lawyer supplies the text. |
| `[DECISION n]` | A choice for the founder or founding members. Listed in the table at the end. |
| `VERIFY` | A fact or figure from the repository that must be confirmed against the current law. |
| **Code:** | What the software already does, so that the constitution and the system agree. If the constitution changes, the code changes (or the other way round); neither is allowed to drift. |

## 1. Name and nature

1.1 The name of the mutual bank is `[Forge Mutual Bank]` `[DECISION 1]`. **The words "bank" and "mutual bank" are not used in any public name, marketing or
website until the Prudential Authority has authorised them** (register LIC-17; `docs/regulatory/policies/naming-and-marketing-policy.md`). Until then the
organisation is referred to by its registered name only.

1.2 It is a mutual bank, a juristic person registered under the Mutual Banks Act. `[ACT]` (registration, juristic personality, the registrar's role).

1.3 It is owned by its members, who are its depositors who hold member shares (clause 4). It has no outside shareholders and does not pay a return to
anyone for providing capital beyond what clause 12 allows. `[ACT]` `[DECISION 2]` (whether any member share carries a dividend or only a rebate).

1.4 Its purpose is `[to provide banking services to its members, especially technology businesses, the people who build them, and savings and credit groups, on
fair, transparent terms, and to return its surplus to members]`. `[DECISION 3]` (the lawyer to align with the Act's description of a mutual bank's objects).

## 2. Definitions `[ACT]`

Terms to be defined with the Act's own wording: Act, Registrar, Prudential Authority, member, member share, founder member, general meeting, special resolution,
board, employee director, independent director, surplus, reserves, record date. The definitions of "member" and "member share" are the ones the software
uses; see clauses 3 and 4.

## 3. Members

3.1 **Who may be a member.** A person who holds an account with the bank and holds at least the minimum member share holding. Natural persons and juristic persons
may be members, including `[companies, close corporations, co-operatives, savings and credit groups, stokvels, and micro-finance institutions]`. `[DECISION 4]`
(whether stokvels and informal groups can be members directly or only through a registered entity; whether a business and its founder are separate members).

3.2 **Admission.** Admission is by application, subject to the bank's customer due diligence (identity, sanctions and anti-money-laundering checks under the
Financial Intelligence Centre Act). A person who fails those checks cannot be admitted. The board may refuse an application only on stated, non-discriminatory
grounds recorded in the minutes. `[ACT]`

3.3 **Founder members.** There are at least **seven** founder members, who sign the founding documents. The application to the Prudential Authority is not made
until there are at least seven. `[ACT]` `VERIFY` (the figure of seven and what "founder" means).
**Code:** `membership.ReadyToApply` refuses until the register holds the founding number of active members (`Profile.MinFounders = 7` in `pkg/banklayer/legalform.go`).

3.4 **Ending membership.** A member ceases to be a member on resignation, on closing their last account and redeeming their shares, on death or dissolution, or
by the board's removal for `[a serious breach of the rules or fraud]`, with a right to be heard and to appeal to a general meeting. `[ACT]` `[DECISION 5]`.

3.5 **Register of members.** The bank keeps a register of members showing name, identity or registration number, date of admission, shares held, and date of
ceasing. The register is personal information and is protected under the Protection of Personal Information Act. A member may inspect their own entry. `[ACT]`
(who else may inspect the register, and what must be public).
**Code:** the member register, admission date and status are held by the membership module.

## 4. Member shares and capital

4.1 **One class of member share** `[DECISION 6]` (a second class for founders, for non-voting capital, or for institutional members is a decision with legal
consequences and is not assumed here). Each member share has a nominal value of `[R100]`. A member holds at least `[1]` and at most `[a limit so that no member holds
more than 5% of all member shares]` `[DECISION 7]`.

4.2 Member shares are issued only against full payment. They are not transferable except by redemption to the bank. They carry **one vote per member**, not per share.

4.3 **Redemption.** A member may ask to redeem shares when they leave or reduce their holding. The bank must refuse a redemption if paying it would take the bank
below the capital the Act and the Prudential Authority require, and may defer redemptions for `[DECISION 8: the longest deferral]`. `[ACT]`
**Code:** `MoveShares` redeems only after a capital check and refuses with `ErrCapitalAtRisk`; redemption of more than held is refused (`ErrTooFewShares`).

4.4 **Minimum capital.** The bank maintains at least the minimum capital the Act requires, currently understood to be **R10 million** at registration. `VERIFY` `[ACT]`.
Proof of that capital is part of the application (register LIC-02). The founders' capital plan is in `BUSINESS_PLAN_DRAFT.md` section 9.
**Code:** the prudential report raises an INITIAL_CAPITAL breach below the minimum for the legal form.

4.5 No member share is a deposit and none is covered by deposit insurance. The member is told so in writing before buying shares. `[ACT]`

## 5. General meetings

5.1 The bank holds an annual general meeting within `[six months]` after its financial year end, and other meetings when the board calls them or when
`[a stated number or percentage of members]` ask in writing. `[ACT]` `[DECISION 9]`.

5.2 **Notice.** At least `[21]` days' written notice, sent to each member's registered address and electronic address, stating the time, place (or
electronic means) and the resolutions. `[ACT]` `VERIFY`.

5.3 **Record date.** Only a person who was a member on the record date set in the notice may vote. **Code:** the module treats the day the notice was sent as the record date, and a member admitted after it is refused
(`ErrNotEligible`). If the constitution sets a different record date, the module changes with it.

5.4 **Quorum.** A meeting is quorate when at least `[the greater of 25 members or 5% of members]` are present or voting. `[DECISION 10]`.
**Code:** `Decide` returns INQUORATE when voters are fewer than the quorum fixed when the meeting was called. Abstentions count towards the quorum and not
towards the majority.

5.5 **Voting.** One member, one vote. Voting may be in person, by proxy `[DECISION 11: whether proxies are allowed and how many a person may hold]`, or
electronically by a method that proves the voter is a member and records one vote only. **Code:** a second vote by the same member on the same resolution is
refused (`ErrAlreadyVoted`); a closed resolution takes no more votes (`ErrClosed`).

5.6 **Majorities.** An ordinary resolution passes on **more than half** of the votes cast for or against. A special resolution passes on **at least three quarters**
of the votes cast for or against. `[ACT]` `VERIFY` (whether the Act or regulations set other thresholds).
**Code:** `Decide` applies exactly these two rules.

5.7 **What needs a special resolution:** amending this constitution, changing the objects, merging, converting to another form, winding up, and
`[DECISION 12: any other matter]`. Amendments take effect only after any approval the Act requires. `[ACT]`

5.8 Results and minutes are kept, and members may inspect them. Resolutions of the first general meeting adopt this constitution and elect the first board
`[or the founders appoint the first board until the first general meeting]` `[DECISION 13]`.

## 6. The board `[ACT]`

6.1 The board has at least `[7]` directors `[DECISION 14]`, elected by the members at a general meeting, serving `[three-year]` terms, with `[one third]` retiring
each year.

6.2 **Employee directors.** **At least two** directors are employees of the bank. `[ACT]` (MR-03, regulation 38; `VERIFY`).

6.3 **Independence.** At least `[a majority of the non-executive directors]` are independent of management and of any significant supplier `[DECISION 15]`. A
director who is an officer or owner of **MYFORGE (Pty) Ltd** (the platform company that builds and runs the product) is not independent on any matter in which
MYFORGE has an interest (clause 10).

6.4 **Fit and proper.** Every director and executive officer is fit and proper and has filed the Prudential Authority's statement (form DI 020 for a mutual
bank) before taking office. Nobody serves whose statement has not been accepted. `[ACT]`
**Code:** the filing pack and tracker are `docs/governance/FIT_AND_PROPER_PACK.md`.

6.5 **Committees.** The board has at least an audit committee, a risk and capital committee, and a nominations and remuneration committee, each with written
terms and a majority of independent non-executive members. `[ACT]` (any committee the Act requires, such as a directors' affairs or credit committee).

6.6 **Duties and conflicts.** Directors act in the interests of the members as a whole. A director discloses any interest in a matter before the board, takes no part in
the decision, and the interest is recorded. `[ACT]`

6.7 **Removal and vacancies.** A director may be removed by ordinary resolution of the members, after being heard. The board may fill a casual vacancy until the next
annual general meeting. `[ACT]`

## 7. Executive management and key persons

7.1 The board appoints a chief executive officer, a chief financial officer, a chief risk officer, a chief compliance officer and the other executive officers the
Act requires, and the heads of the control functions listed in `docs/governance/ROLES.md`. Each is fit and proper (clause 6.4). `[ACT]`

7.2 The chief executive and the chief accounting officer certify the bank's regulatory returns (form DI 099). `[ACT]` (MR-04).

7.3 The control functions (risk, compliance, internal audit) report to the board's committees, not to the executive they oversee.

## 8. Surplus, reserves and rebates

8.1 The bank's surplus is first applied to the reserves the Act and the Prudential Authority require, and to any reserve the board sets for the bank's safety and growth.
`[ACT]`

8.2 The balance may be returned to members as `[better rates, lower fees, a patronage rebate in proportion to each member's business with the bank, or a dividend on
member shares]`. `[DECISION 2]`. A return is decided by the general meeting on the board's recommendation, and is paid only if the bank would still meet its capital
and liquidity requirements afterwards. `[ACT]`

8.3 **Allocation rule (what the software does).** A rebate is split in proportion to a stated weight (fees paid, average balance or shares held), to the cent, so
the parts always add up to the pool, with leftover cents to the largest remainders and ties broken by member number. **Code:** `membership.Allocate`. The weight
is chosen by `[DECISION 16]`. A distribution is approved by one person and released by another (maker-checker).

## 9. Accounts, audit and reporting

9.1 Financial year: `[1 March to end February]` `[DECISION 17]`. Annual financial statements are audited by an auditor registered under the Act and approved by the
Prudential Authority where required. `[ACT]`

9.2 The directors report each year to the Registrar on the effectiveness of internal control over financial and regulatory reporting and compliance. `[ACT]` (MR-02,
regulation 37(5)).

9.3 Returns to the Prudential Authority reflect the management accounts presented to the board, and the auditor reports on the returns the regulations name. `[ACT]`
(MR-04, MR-05).

## 10. Related parties and the platform company

10.1 The bank's technology is built and run by **MYFORGE (Pty) Ltd** under a written services agreement. That agreement is on arm's-length terms, is approved by the
independent directors, is disclosed to members, and is part of the outsourcing register submitted to the Prudential Authority (register OUT-01, LIC-10).

10.2 The bank, not MYFORGE, is responsible for every outsourced function. The Prudential Authority and the bank's auditor have access to MYFORGE's relevant records and premises.
`[ACT]` (the Prudential Authority's outsourcing directive; `VERIFY`).

10.3 No founder, director or executive receives a benefit from MYFORGE that is connected with the bank's business unless it is disclosed to the board and members.

10.4 The brand "Forge" belongs to `[MYFORGE (Pty) Ltd]` and is licensed to the bank. The licence ends on terms that let the bank continue to serve members. `[DECISION 18]`.

## 11. The sponsor-bank stage and transition

11.1 Until the bank holds its own licence, no customer money is held by the bank; it is held by a licensed sponsor bank under a written agreement, and the bank
describes the arrangement honestly to every customer. `[ACT]` (nothing in this constitution is a licence or a promise of one).

11.2 On registration, customers and members migrate from the sponsor's books to the bank's own books under a migration runbook (**not yet written**), with
notice, reconciliation and the Prudential Authority's approval.

## 12. Member protection

12.1 **Complaints.** Members have a written complaints procedure, a named officer, a time limit for a reply, and the right to take a complaint to the relevant
ombud or regulator. `[ACT]`

12.2 **Information.** Each member receives, free, the annual financial statements, the notice and minutes of meetings, and a plain statement of what is and is not
covered by deposit insurance. `[ACT]` `VERIFY` (the deposit insurance scheme's application to a mutual bank's members and their shares).

12.3 **Data.** Personal information is used only for the purposes the member has been told, with the consents and rights required by law. The bank never sells
member data. Third-party offers shown to members are provided, licensed and monitored by the third party, never by the bank, and details go to a provider only with
the member's consent.

## 13. Winding up, merger and conversion `[ACT]`

13.1 The Act decides how a mutual bank is wound up, merged or converted, and what happens to surplus assets after members' shares and deposits are repaid. **Assets are
not distributed to members beyond the nominal value of their shares** unless the Act requires or permits otherwise. `[DECISION 19]` (an "asset lock", such as giving any
residual to a named public-benefit body or to member-owned institutions).

## 14. Amendments

14.1 This constitution may be amended only by special resolution (clause 5.6) and only with any approval the Act requires. `[ACT]`

14.2 A draft amendment is sent to members with the notice of the meeting. A change that would reduce the one-member-one-vote rule or move assets to outsiders needs
`[DECISION 20: a higher majority]`.

## 15. Founding page (to be completed by the founders)

| # | Founder member | Identity or registration number | Date admitted | Member shares | Signature |
|---|---|---|---|---|---|
| 1 | Tebogo Mvelase | _to complete_ | | | |
| 2 | | | | | |
| 3 | | | | | |
| 4 | | | | | |
| 5 | | | | | |
| 6 | | | | | |
| 7 | | | | | |

**Only one founder is named so far.** The remaining six are people the founder must recruit and who must each pass the checks in clause 3.2 and file the
statements in clause 6.4. Nobody is added to this table by Claude or by software.

## Decisions for the founder and founding members

| # | Decision | Why it matters | Suggested default (PROPOSED) |
|---|---|---|---|
| 1 | The registered name | The Act and the Prudential Authority control "bank" in a name | Keep "Forge" as the working brand; no "bank" until authorised |
| 2 | Return to members: rebate only, or dividends too | A dividend on shares can pull the structure towards a company; the lawyer must advise | Patronage rebate and better rates only |
| 3 | Objects | Defines who the bank exists for | Technology businesses first; savings and credit groups next |
| 4 | Who may be a member (groups, stokvels, MFIs, founders and their companies) | Drives whether the MFI-network idea works under the Act | Registered entities only; individuals as separate members |
| 5 | Grounds and process for removing a member | Fairness and legal challenge | Fraud, serious breach, failed due diligence; hearing and appeal |
| 6 | One share class or more | Capital raising and control | One class |
| 7 | Nominal value and holding limits | Capital depth against concentration of influence | R100 nominal; 5% cap on shares held |
| 8 | Longest deferral of redemptions | Protects capital in stress; affects member trust | To be set with the capital plan and the lawyer |
| 9 | When the AGM is held, and how many members can call a meeting | Member rights | Within six months of year end; 5% of members |
| 10 | Quorum | A meeting that cannot reach quorum cannot decide | Greater of 25 members or 5% |
| 11 | Proxies and electronic voting | Participation across the country; fraud risk | Electronic voting with proof of membership; limit proxies |
| 12 | Other special-resolution matters | Protects members against changes of character | Anything that changes member rights |
| 13 | First board: elected at the first meeting, or appointed by the founders until then | Who runs it before there are members | Founders appoint; members confirm at the first meeting |
| 14 | Board size and terms | Cost and independence | 7 directors, three-year terms, a third retiring each year |
| 15 | Independence standard | Governance quality and the Prudential Authority's view | Majority of non-executives independent |
| 16 | Rebate weight | Fairness between small and large members | Fees paid |
| 17 | Financial year end | Audit timetable | To be set with the auditor |
| 18 | Ownership and licence of the Forge brand | A mutual cannot be controlled through its brand | Licence from MYFORGE with a no-hold-up clause |
| 19 | Asset lock on winding up | Mutual character | Residual to member-owned or public-benefit bodies |
| 20 | Supermajority for changes that remove member rights | Stops a later majority undoing mutuality | 90% of votes cast |

## What this draft does not do

- It does not satisfy the Act. The lawyer must add every `[ACT]` provision and check the figures marked `VERIFY`.
- It does not choose the legal form. If the lawyer advises the bank route (a public company), this becomes articles of association and a shareholders'
  agreement instead (see `docs/governance/LEGAL_FORMS.md`).
- It does not name anyone other than the founder, and nothing in it is signed.
- It is not in force, and nothing in the software depends on it being approved. The code's rules in the **Code:** notes are the same either way and will be
  aligned with the final text.
