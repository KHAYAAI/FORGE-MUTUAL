> **DRAFT for board approval. Not approved. Not in force.** Prepared on 2026-11-18 from the services in this repository. Every recovery objective is PROPOSED and must be confirmed against the risk appetite (`RISK_APPETITE_DECISIONS.md` rows 23 and 24).

# Business impact analysis

| | |
|---|---|
| Owner | Business Continuity Manager (R28, VACANT) |
| Approved by | IT and Cyber Committee |
| Review | Yearly, and after any major change |
| Register rows | G4-02, JS1-12, OUT-05 |
| Version | 0.1 draft |

**Purpose.** Ranks Forge's services by the harm an outage does, and sets the recovery time objective (RTO, how long it
may be down) and recovery point objective (RPO, how much data may be lost) that the continuity plan, the DR site and
provider contracts must meet.

## Services

| Service | What it does | Harm if down 1 hour | Harm if down 1 day | Depends on | RTO | RPO | Tier |
|---|---|---|---|---|---|---|---|
| Ledger (`ledger-gateway-bank`, TigerBeetle) | Book of record for every balance | Payments stop | Customers cannot see or move money; regulatory breach | TigerBeetle cluster, Postgres | 1 hour | **0** | 1 |
| Payments in and out (`payments-orchestrator`, `rails-gateway`, `payouts-service`) | EFT, PayShap, payouts | Payments queue | Salaries and suppliers unpaid; reputational damage | Sponsor bank, ledger | 4 hours | 0 | 1 |
| Sign-in (WorkOS, `workspace-api`) | Customers and staff log in | Customers locked out | Same, plus support load | WorkOS | 4 hours | 5 min | 1 |
| Sanctions screening and monitoring (`compliance-engine`) | Blocks listed persons; opens cases | Payments held (fails closed) | Payments held; AML breach risk | Sanctions list provider | 4 hours | 5 min | 1 |
| Reconciliation (`reconciliation`) | Proves the ledger matches the banks | Payments held on a break | Breaks grow; threshold 12 breached | Sponsor statements | 24 hours | 0 | 2 |
| Onboarding and KYC (`onboarding`, Didit) | New customers | New customers wait | Growth stops | Didit, Temporal | 24 hours | 5 min | 2 |
| FX, deposits, lending, acquiring | Products beyond the core | Feature unavailable | Customer harm, missed instalments | Partner banks, acquirer | 24 hours | 0 (money); 5 min (other) | 2 |
| Back office (`backoffice-api`, `apps/backoffice`) | Staff operations | Manual work queues | Cases and approvals delayed | Sign-in | 24 hours | 5 min | 2 |
| Regulatory reporting (`reg-reporting`, `treasury-risk`) | Returns, prudential report | None | Late returns if over a due date | Ledger | 72 hours | 0 | 3 |
| Notifications | Email and SMS | Customers not told | Same | Email and SMS provider | 72 hours | 1 hour | 3 |

## What follows from it

1. Tier 1 services run in two availability zones in production and fail over to a geographically separate DR site
   (JS1-12) within their RTO. Not built: no production host is chosen (ADR-0002).
2. Backups and point-in-time recovery are proven by a restore in the CI gate; production needs off-site immutable
   copies (VERIFY-WORM).
3. Every Tier 1 and Tier 2 provider contract carries clause 9 of `provider-contract-schedule.md` with these objectives.
4. The yearly DR test covers the loss of a whole site and at least one Tier 1 provider (`business-continuity-and-disaster-recovery-policy.md`).

## Sign-off

| Role | Name | Signature | Date |
|---|---|---|---|
| Owner | VACANT | | |
| Approver (IT and Cyber Committee) | VACANT | | |
| Second line reviewer | VACANT | | |
