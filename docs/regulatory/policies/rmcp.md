> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# Risk Management and Compliance Programme (FICA s.42)

Purpose: how Forge identifies, assesses, monitors, mitigates and manages the risk of money laundering, terrorist
financing and proliferation financing. Register ids: FIC-01 to FIC-10. Written from what the code does today; each
section says which parts are built and which are not. FICA references are from general knowledge and must be checked
against the current Act, the FIC's guidance notes and public compliance communications before approval.

## 1. Risk assessment
- **Clients.** PROPOSED risk ratings: *standard* (South African resident individual or company, simple ownership),
  *enhanced* (a controlling shareholder of 50% or more, a prominent influential person, a sector on the watch list,
  foreign ownership), *prohibited* (a sanctions match, a sector outside appetite, refusal to provide information).
  Built: the tier decision (`services/platform/onboarding/internal/onboarding/tier.go`); the sector list in policy
  (`policy/`). Not built: a rating that combines all factors.
- **Products.** Highest risk first: card acceptance (third-party funds), foreign-currency accounts and payments from
  abroad, bulk payouts, credit, deposits. New products are off in production until approved (`pkg/licence`), and the
  approval must include this assessment.
- **Channels.** Fully remote onboarding: identity verified by document and liveness (Didit), never face to face.
- **Geography.** Foreign payers and partner banks; sanctions screening of every party (fails closed).

## 2. Customer due diligence (s.21 to s.21H)
| Step | Standard | Enhanced | Built |
|---|---|---|---|
| Identify and verify | ID document and selfie with liveness | the same, plus source of funds | Yes (Didit adapter, untested live) |
| Company | registry check; each director verified | the same, plus ownership chain | Registry and directors yes; chain no |
| Beneficial owners | shareholders of 25% or more (PROPOSED) and anyone in control | 10% (PROPOSED) | Captured; threshold not enforced |
| Prominent persons | screened at onboarding | senior approval, source of wealth | Screening only |
| Ongoing | monitoring rules on every transaction | lower thresholds, yearly review | Rules yes; reviews no |
Doubtful or incomplete verification: the application is declined or held for review (`kyc_review`). A business
relationship is never opened before verification completes.

## 3. Sanctions (s.26A to s.28A)
Every party is screened at onboarding against the targeted financial sanctions list; an unavailable or stale list
declines (fail closed). To build: rescreening every party when the list changes, freezing a matched party's accounts,
and the s.28A report.

## 4. Monitoring and reporting (s.28, s.29)
Rules on the event bus open cases with SLA timers (thresholds are placeholders). A compliance officer decides each
case with maker-checker. Suspicious and unusual transaction reports go through goAML, which is refused until the schema
is verified. **Never tip off:** applicants and customers are never told a decline or a report relates to screening or
suspicion; the API and pages are built and tested to hide it.

## 5. Records (s.22, s.23)
CDD records, transactions and reports are kept for at least five years after the relationship or transaction ends,
then disposed of under the retention schedule (`data-retention-schedule.md`).

## 6. Governance and training (s.42A, s.43)
The board approves this programme and reviews it yearly. A money-laundering reporting officer (MLRO) at senior level
owns it, with a deputy. Staff are trained before handling customers and yearly after; the training plan must add AML.

## 7. Open items before approval
MLRO appointment; PEP and sanctions data sources; ownership-chain tracing; rescreening and freezing; goAML schema;
periodic CDD reviews; the combined client risk rating; FIC registration.
