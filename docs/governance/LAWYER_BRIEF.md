# Brief for a regulatory lawyer

> For **Tebogo Mvelase** to send. The aim is to find a South African financial-services regulatory lawyer to review
> the governance pack and advise on the legal form, then to help with the licence application. I can't appoint anyone
> or check a firm's current team, so verify every candidate yourself.

## What we need from them

**Phase 1: advice (fixed fee, about 4 to 6 weeks)**

1. **Legal form:** a mutual bank (planned) or a bank under the Banks Act (kept open). Confirm the current minimum
   capital, founder and board rules, member-share treatment and any amendments since 1993 (`LEGAL_FORMS.md`).
2. **Sponsor model:** what Forge may do before it holds a licence, under a sponsor bank's licence, and what the sponsor
   contract must cover. Can Forge call itself anything containing "bank" or "mutual" before authorisation?
3. **Review the governance pack:** the structure (`STRUCTURE.md`), the 35 seats (`ROLES.md`), and the 52 draft policies
   (`POLICY_FRAMEWORK.md`). Do they meet the Banks Act or Mutual Banks Act regulations, King IV, Joint Standards 1
   and 2, Directive 3/2018 and the Guidance Notes?
4. **Regulatory register:** check the 114 requirements (`docs/regulatory/register.csv`) and add the laws not yet read:
   FICA in full, POPIA, the National Payment System Act, exchange control, the National Credit Act, the CODI
   (deposit insurance) rules, COFI if in force, and the FSCA's crypto-asset rules.
5. **Crypto:** what a bank or mutual bank may do for FSCA-licensed crypto-asset service providers, and the
   PA's current position on banks holding or lending against crypto assets.
6. **Credit:** NCR registration, and the default notice and statement wording we built (NCA s.108 and s.129).
7. **Outsourcing and cloud:** which of our providers (WorkOS, Didit, hosting) are material, and what notices to the PA
   are needed.

**Phase 2: the application (quoted separately)**

- Founding documents (mutual constitution or company articles, shareholder or member agreements).
- The pre-application meeting with the Prudential Authority, and FSCA concurrence.
- DI or BA application forms and fit-and-proper filings (DI 020 / BA 020).
- Provider contracts: sponsor bank, partner banks, acquirer, identity and KYC providers.
- Customer terms, privacy notice, PAIA manual, credit agreements.

## What to look for

- A partner who has taken a new bank, mutual bank or co-operative bank through PA registration in the last 10 years,
  or acted on a sponsor-bank (banking-as-a-service) arrangement.
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
> I am the founder of Forge Mutual, a proposed technology-led mutual bank in South Africa. We plan to begin on a sponsor
> bank's licence and apply for registration as a mutual bank, keeping a Banks Act registration as an alternative.
>
> We have built the banking platform and a draft governance pack: the board and committee structure, a roster of 35
> roles, 52 draft policies and a register of 114 regulatory requirements. We are looking for counsel to:
> (1) advise on the legal form and the sponsor model; (2) review the governance pack and register; and (3) quote for
> the licence application.
>
> Could you let me know whether you have capacity and no conflict, and send a fixed-fee estimate for the first phase?
> I can share the documents under NDA.
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
