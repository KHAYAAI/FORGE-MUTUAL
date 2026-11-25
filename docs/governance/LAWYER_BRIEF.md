# Brief for a regulatory lawyer

> For **Tebogo Mvelase** to send. The aim is to find a South African financial-services regulatory lawyer to review
> the governance pack and advise on the legal form, then to help with the licence application. I can't appoint anyone
> or check a firm's current team, so verify every candidate yourself.
>
> **Revised 2026-11-24** to cover the draft constitution and business plan, which the first version predates. Documents to
> share under NDA with this brief: `docs/licence-application/CONSTITUTION_DRAFT.md`, `docs/licence-application/BUSINESS_PLAN_DRAFT.md`,
> `docs/governance/SPONSOR_BANK_PITCH.md` and the register. **All are drafts, written by an AI assistant from the repository; none is
> legal advice, and none has been sent to anyone.**

## What we need from them

**Phase 1: advice (fixed fee, about 4 to 6 weeks)**

1. **Legal form:** a mutual bank (planned) or a bank under the Banks Act (kept open). Confirm the current minimum
   capital, founder and board rules, member-share treatment and any amendments since 1993 (`LEGAL_FORMS.md`). Also: **is the mutual form
   available and sensible for a membership of technology businesses (and, later, savings groups and micro-lenders)?** Do member shares
   count towards the minimum capital, and what other capital instruments does the Prudential Authority accept for a mutual?
2. **The draft constitution** (`CONSTITUTION_DRAFT.md`). Please (a) add every provision the Mutual Banks Act and regulations require (marked
   `[ACT]`), (b) correct anything the Act forbids, and (c) advise on the 20 numbered decisions at the end. The ones that matter most:
   - **Who may be a member:** companies, co-operatives, stokvels and informal savings groups, micro-finance institutions, a founder and their
     company as separate members (decision 4).
   - **Return to members:** patronage rebates only, or a dividend on member shares (decision 2), and whether one share class is enough (6).
   - **Redemption:** the capital check and how long redemptions may be deferred (8); share nominal value and holding limits (7).
   - **Voting and meetings:** proxies and electronic voting, quorum, and the majorities for ordinary and special resolutions (5.5, 5.6, decisions
     10 to 12). The software already enforces one member one vote, more than half for ordinary and three quarters for special resolutions, a
     record date and a quorum; it will be changed to match whatever you advise.
   - **Board:** at least two employee directors (MR-03), independence, and the first board before members exist (13 to 15).
   - **Asset lock and winding up** (19), and the supermajority to protect mutuality (20).
3. **Money before registration.** Can prospective members or founders pay in share capital, or can anyone pay deposits or commitments, **before
   registration**, and under what conditions (escrow or trust, public-offer or prospectus rules, the prohibition on taking deposits without a
   licence)? The draft constitution and capital plan assume founders can subscribe; nothing confirms it (`BUSINESS_PLAN_DRAFT.md` section 9).
4. **Sponsor model:** what Forge may do before it holds a licence, under a sponsor bank's licence, and what the sponsor contract must cover
   (`SPONSOR_BANK_PITCH.md`, especially "Positions we hold": no exclusivity or non-compete that blocks our own application, and a contract
   that can be novated to the future bank). Can Forge call itself anything containing "bank" or "mutual" before authorisation, including in a
   pitch to a bank (LIC-17)? What wording is safe ("proposed mutual bank, not yet registered"?).
5. **Which built products need their own registration or approval.** The platform has built, and switched off, savings and notice deposits,
   working-capital credit, merchant acquiring, virtual debit cards and third-party offers. For each: can it be offered on a sponsor's licence,
   what must Forge itself hold (NCR, PASA, FSCA, other), and is any of it safe to include in a first launch? The first launch is intended to be
   accounts, payments and FX only.
6. **Related party: the platform company.** MYFORGE (Pty) Ltd, controlled by the founder, builds and runs the platform and would be the bank's
   principal technology supplier. Advise on the services agreement, a brand licence that cannot hold the mutual to ransom, director
   independence, disclosure to members, and how this appears in the outsourcing submission (OUT-01, OUT-06, LIC-10).
7. **Third-party offers and referral fees** (`services/platform/partner-offers/SPEC.md`). Please confirm: the wording of the statement
   customers must acknowledge; whether Forge may receive a referral fee from a licensed provider (and any cap or disclosure the law requires);
   whether introducing customers to financial-services or crypto-asset providers is itself regulated activity; and POPIA for sending a
   consenting customer's **name** to the provider (operator or responsible party, cross-border, retention, what withdrawing consent must do).
8. **Review the governance pack:** the structure (`STRUCTURE.md`), the 37 seats (`ROLES.md`), and the 58 draft policies
   (`POLICY_FRAMEWORK.md`). Do they meet the Banks Act or Mutual Banks Act regulations, King IV, Joint Standards 1
   and 2, Directive 3/2018 and the Guidance Notes?
9. **Regulatory register:** check the 114 requirements (`docs/regulatory/register.csv`) and add the laws not yet read:
   FICA in full, POPIA, the National Payment System Act, exchange control, the National Credit Act, the CODI
   (deposit insurance) rules, COFI if in force, and the FSCA's crypto-asset rules. Row OUT-02 (20 working days' notice to the PA before material
   outsourcing) is typed "Bank" only: **does it also apply on the mutual route?**
10. **Crypto:** what a bank or mutual bank may do for FSCA-licensed crypto-asset service providers, and the
    PA's current position on banks holding or lending against crypto assets.
11. **Credit:** NCR registration, and the default notice and statement wording we built (NCA s.108 and s.129).
12. **Digital assets:** the eight questions in `docs/products/DIGITAL_ASSETS.md`: whether a tokenised deposit is a deposit, the
    FSCA's position, reserve and capital treatment, the travel rule, and exchange control.
13. **Outsourcing and cloud:** which of our providers (WorkOS for sign-in, likely US processing; Didit for identity checks, location
    unconfirmed; hosting; a sanctions-list provider) are material, what notices to the PA are needed, and what data may flow to each.

**Phase 2: the application (quoted separately)**

- Founding documents: finalise the mutual constitution (or company articles), member or shareholder agreements, and the
  **services agreement and brand licence between MYFORGE (Pty) Ltd and the bank**.
- The pre-application meeting with the Prudential Authority, and FSCA concurrence.
- DI or BA application forms and fit-and-proper filings (DI 020 / BA 020).
- Provider contracts: sponsor bank, partner banks, acquirer, identity and KYC providers.
- Customer terms, privacy notice, PAIA manual, credit agreements.

## What to look for

- A partner who has taken a new bank, mutual bank or co-operative bank through PA registration in the last 10 years,
  or acted on a sponsor-bank (banking-as-a-service) arrangement.
- **Experience drafting a mutual or co-operative constitution** (not only company articles), and with member-share capital.
- Current relationships with the Prudential Authority and the FSCA; fintech and crypto-asset experience.
- **No conflict:** they must not act for the sponsor bank we choose, or be the firm a sponsor uses for its own due diligence.
- Willing to quote phase 1 as a fixed fee.
- Can work from documents in a GitHub repository, or will accept a PDF export.

## Where to look

Large South African firms with financial-services regulatory practices: Bowmans, Webber Wentzel, ENSafrica,
Cliffe Dekker Hofmeyr, Werksmans, and the Johannesburg offices of Norton Rose Fulbright, Baker McKenzie and Herbert
Smith Freehills. Smaller fintech-focused boutiques often cost less for phase 1. Ask for a short shortlist through:

- the founders of recently licensed South African digital banks, who often name their counsel;
- the Banking Association South Africa;
- the Intergovernmental Fintech Working Group's innovation hub contacts;
- your accountants, since the audit firms work with these lawyers on licence applications.

The firm names are from general knowledge. Check each firm's current team and conflicts before you approach them.

## First email (to adapt)

> Subject: Regulatory advice: proposed mutual bank (pre-licence)
>
> Dear [name],
>
> I am the founder of Forge, a technology company (MYFORGE (Pty) Ltd) that is building the platform for a proposed technology-led mutual bank in South
> Africa. Nothing is registered or licensed yet. We plan to begin on a sponsor bank's licence and to apply for registration as a mutual bank, keeping a
> Banks Act registration as an alternative.
>
> We have built the banking platform and drafted a governance pack: a board and committee structure, 37 roles, 58 draft policies and a register of 114
> regulatory requirements. We have also prepared draft versions of a mutual-bank constitution and a business plan, and a pitch for a sponsor bank.
> They are working drafts, written to give counsel something to correct; we do not treat any of them as advice.
>
> We are looking for counsel to: (1) advise on the legal form, member-share capital, and what we may do and say before registration; (2) review and
> correct the draft constitution; (3) advise on the sponsor model, the platform company as a related party, and referral fees and data sharing for
> third-party offers; (4) review the governance pack and register; and (5) quote for the licence application.
>
> Could you let me know whether you have capacity and no conflict (in particular, that you do not act for a bank we may approach as a sponsor), and
> send a fixed-fee estimate for the first phase? I can share the documents under NDA.
>
> Kind regards,
> Tebogo Mvelase, Founder

## Tracking

| Firm | Contacted | Conflict check | Phase 1 quote | Notes |
|---|---|---|---|---|
| | | | | |
| | | | | |
| | | | | |

When one is appointed, set seat R26 in `roles.csv` to the firm and partner's name and run `make governance`.
