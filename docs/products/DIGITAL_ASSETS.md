# Digital assets: stablecoins, deposit tokens and on-chain settlement

> **DRAFT decision memo.** Gap 4 of `docs/BANK_MODEL.md`: Erebor is described as seeking to facilitate stablecoin
> transactions, and our tech-and-crypto leaning invites the same. This memo says what we could do, what stops us, what
> is built, and what to decide. **Nothing here moves customer money.** Not legal advice. Every regulatory statement is
> from general knowledge and must be confirmed by the lawyer and the Prudential Authority and FSCA before anything is built.

## Why it is not built

Two rules in `docs/ARCHITECTURE.md` stop it, on purpose:

- **Rule 8:** the bank's ledger rejects stablecoin assets, and rejects tokenised-deposit assets unless an own licence is
  approved (`licence.bank.own`) and the asset itself is approved.
- **Rule 6:** only one gateway writes to each ledger cluster, and digital assets belong to a **separate entity** (`DAST`)
  with its own ledger cluster, signer and on-chain services. The bank perimeter holds ZAR and foreign-currency deposits.

A service that minted tokens against customer deposits inside the bank entity would break both, and would do so before
anyone has asked the regulator whether it is allowed. So the decision comes first.

## The three things "digital assets" could mean

| Option | What it is | Where it lives | Licence and regulator | Effort |
|---|---|---|---|---|
| **A. Bank customers send and receive stablecoins through a licensed partner** | Forge never touches the token. A licensed crypto-asset service provider (CASP) or exchange does the conversion; Forge sends and receives rand and verifies the counterparty. | Bank entity, ordinary payments | The partner's licences; Forge's screening and travel-rule duties | Small: counterparty screening, travel rule, partner integration |
| **B. A separate Forge digital-asset entity** | Custody and on/off-ramp for a third-party stablecoin (an issuer per ADR-0005). | `DAST`, own ledger, signer, on-chain services | FSCA as a CASP, plus the PA's view of the group; intercompany settlement with the bank | Large: 6 to 12 months of engineering, custody controls, a smart-contract-free design but signing infrastructure |
| **C. Tokenised deposits** | The bank issues an on-chain token that is a claim on a deposit, 1:1 with a liability (account 7001). | Bank entity, own licence only | The PA: a deposit-taking and deposit-insurance question (is a token a deposit?); FSCA if it is also a crypto asset | Largest, and the least certain legally |

A Forge-issued stablecoin (not a deposit token) is a fourth possibility the architecture lists as deferred; it carries
the heaviest regulation and is not recommended.

## Recommendation

**Do A first, when a sponsor bank and a pilot exist; do not start B or C until the regulator has been asked.**
Reasons: A is what customers most plausibly want (moving rand to and from a stablecoin for a technology business), needs
no new licence for Forge, and fits the risk appetite (`RISK_APPETITE_DECISIONS.md` rows 15 to 17: bank only
FSCA-licensed crypto businesses, no crypto on our own balance sheet). B and C change what kind of institution Forge is.

## Questions for the lawyer and the regulator

1. Is a tokenised deposit a deposit under the Banks Act (or Mutual Banks Act), and is it covered by deposit insurance?
2. Is it also a crypto asset under the FSCA's declaration, needing a CASP licence?
3. May a mutual bank hold crypto-asset reserves, or only the reserves that back a token? Which assets may be reserves?
4. What is the capital treatment of crypto-asset exposure and of token liabilities?
5. What does the FIC's travel-rule directive require for each transfer, and from what value?
6. Does the sponsor bank permit crypto-related activity at all, and for which customers?
7. Which stablecoin issuers are acceptable to the regulators, and under what disclosures to customers?
8. Exchange control: how do transfers of crypto assets abroad interact with the single discretionary and foreign
   investment allowances (`docs/THRESHOLDS.md`, threshold 28)?

## Controls any option must have

| Control | Status |
|---|---|
| **1:1 reserves with no tolerance:** every token in circulation is covered by reserve assets; a shortfall of one unit halts issuance and redemption and goes to the board | **Written and tested** (`pkg/digitalassets.Attest`); not connected to anything |
| **Wallets:** supported chain, valid address, proof the customer controls it, screened within 24 hours and clear, else no transfer | **Written and tested** (`Wallet.Usable`) |
| **Travel rule:** originator and beneficiary details travel with a transfer; the originator's identifier from 1 000 US dollars (VERIFY) | **Written and tested** (`TravelRule`) |
| Sanctions and illicit-exposure screening of addresses | Interface only: the screening package works on names; address screening needs a provider (Didit offers wallet screening; unverified here, not used) |
| Key custody: hardware security modules, multi-party signing, no single person able to move funds | Not designed in detail |
| Daily proof of reserves published, and an independent attestation | Not built |
| Smart-contract audit, if a contract is deployed at all (option C) | Not started |
| Incident response for a chain event (a stablecoin depeg, an issuer freeze) | Not written |
| Customer disclosures: a token is not a deposit unless the PA says so; not covered by insurance unless it is | Not written |

## What is built

- **Option A's front door:** `services/platform/partner-offers` can present a **licensed crypto-asset service provider's** on/off-ramp as a
  third-party product (category `CRYPTO_ASSETS`, CASP licence required). Forge never touches the token. Every offer states that
  it is provided, licensed and monitored by the provider, and the whole layer is **switched off**. No provider is chosen.
- `pkg/digitalassets`: `Attest` (reserve cover), `Wallet.Usable` (fail-closed wallet checks) and `TravelRule`. It
  holds no state, calls no network and writes to no ledger. It exists so that whoever builds A, B or C starts from tested
  invariants, and so CI can hold them to those invariants.
- The `digital_assets` licence product (`pkg/licence`): when any service uses it, production will refuse it without a
  `FORGE_APPROVAL_DIGITAL_ASSETS` decision. No service uses it yet.
- The architecture's reserved codes (account 7001 TOKENISED_DEPOSIT_LIABILITY; transfer codes 40 and 41) remain
  **unregistered in the ledger gateway**, so the bank ledger still refuses them.

## What is not built, deliberately

A token issuer or minter; a chain watcher or executor; a signer; a wallet registry; a digital-asset ledger cluster;
any chain connection; stablecoin custody; an on/off-ramp. Each depends on the decision below.

## Decision needed from the owner

1. Agree the sequence: **A after the pilot**, B and C only after a written view from the PA and FSCA.
2. Put the eight questions above in the lawyer's brief (`docs/governance/LAWYER_BRIEF.md`).
3. Decide whether Forge will ever hold crypto itself, or only bank businesses that do (the risk appetite worksheet).
