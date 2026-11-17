> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# Exit plans for providers (Directive 3 of 2018; Guidance Note 5 of 2014)

Register id: D3-06. One line each; the outsourcing register (`../outsourcing-register.csv`) holds the full list.

| Provider | If it fails or we leave | Data back | Time (PROPOSED) |
|---|---|---|---|
| Sponsor bank | Move to another sponsor through the `BankLayer` interface; customers' balances are in our own ledger | Statements and the pooled account | 6 months, needs a contract with a second sponsor |
| Didit | Another KYC provider behind `kyc.Provider`; existing verdicts stay in our records | Session records and documents, by contract | 1 month |
| WorkOS | Any OIDC provider; the apps use standard OIDC | User list export | 1 month |
| Cloud host | Infrastructure as code on another host; open-source stack | Backups, off-site | 3 months |
| Acquirer | Another acquirer behind `acquiring.Acquirer` | Transaction history | 3 months |
| Sanctions data | Another list provider behind `pkg/screening` | n/a | 2 weeks |
