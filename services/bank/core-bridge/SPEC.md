# core-bridge (services/bank/core-bridge)

Entity: BANK. Architecture: ARCHITECTURE.md 8 (Fineract and core-bridge) and 13 (the GL reconciliation). Phase 1.

## What it does

- **Mirrors committed transfers.** Each committed, non-pending ledger transfer for a mirrored code is queued as a journal in
  the same transaction that records the ledger event. A hold is not a posting, so it is not queued.
- **Posts to Fineract, once.** The poster sends each journal to Fineract with the ledger transfer id as the reference.
  The chart is per asset and per code (`1001-ZAR`, `1001-USD`), so a currency never mixes into another's balance.
- **Reconciles the general ledger.** Every interval (default 24 hours) it compares each mirrored GL account with the ledger's
  posted balance. A difference is a break, and the run names the account and the size. The run refuses to compare while
  journals are still queued, and says so.

## The rule that matters most: check before a retry

Fineract **accepts a repeated reference number**. A second journal with the same reference is posted again (verified against
a running Fineract; see the live test). So a retry after an unknown outcome could count money twice in the general ledger.

- A 4xx from Fineract means nothing was written: the journal fails at once, with the reason kept.
- A timeout, a network failure or a 5xx means the outcome is unknown. The journal stays pending. The next attempt first looks
  for the journal by its reference (`FindByReference`) and records it if it is there, instead of writing it again.
- `finfake` tests this: Fineract accepts the write and then fails the response. The retry records the posting; Fineract holds
  one. Disabling the check makes the test fail with two postings (mutation check).

## Verified against a real Fineract

The client was tested against `apache/fineract` (image digest
`sha256:3bf81a6a0f073801ee12cba2922541bb7d354f363e109393c10f3ca1ca1cd4d6`), running on this environment's Postgres 17. The live
test creates its own GL accounts, posts a journal, and checks that the balances and the reference lookup agree. Verified:

- `POST /journalentries` with `officeId`, `transactionDate` in `dd MMMM yyyy`, `currencyCode`, `referenceNumber`, and `debits`
  and `credits` with `glAccountId` and a decimal `amount` in major units.
- A repeated `referenceNumber` is accepted (see above).
- Balances: `GET /journalentries?glAccountId=` lists entries with `entryType.code` and `amount`. The bridge sums them exactly,
  parsing amounts as decimal strings, never as floats.
- Currencies must be enabled for the organisation (`PUT /currencies`) before journals in them are accepted.
- The default tenant's database user is `root` in the image's tenant configuration. This environment created that role; a
  real deployment must configure its own tenant database credentials.

## Not built, and must be before cutover

- **Opening balances.** Core-bridge mirrors transfers from the moment it starts. Ledger balances that existed before that
  point are not in Fineract, so the first reconciliation will show breaks equal to them. A cutover runbook must post an
  opening-balance journal for every GL account before the bridge goes live. This is not built.
- **Fineract's version is pinned by digest here, not by a deployment decision.** Confirm the version production uses.
- **Secrets.** `FINERACT_USER` and `FINERACT_PASSWORD` come from the secret store. The image's default credentials
  (`mifos`/`password`) must not reach production.
- **Duplicate detection is by reconciliation.** The check-before-retry prevents the usual duplicate. If a duplicate still
  occurs (for example, two bridge instances posting at once), the GL reconciliation shows it as a break. One poster runs at a
  time (an advisory lock), but that is a guard against misconfiguration, not a proof.

## Limits, stated

- **Lookup cost.** `FindByReference` scans the GL account's journal entries, page by page. It runs only after an unknown
  outcome. When volumes grow, Fineract's own query or an index on the bridge side is needed.
- **Mirrored codes only.** A transfer touching a code outside the chart is recorded as FAILED, for an operator to resolve. The
  chart is in `internal/chart` and must match Appendix B.
- **Journal date** is the ledger event's time, in UTC.

## Configuration

| Variable | Meaning |
|---|---|
| `DATABASE_URL` | Postgres for the queue and the cache (required) |
| `KAFKA_BROKERS` | Broker list (required) |
| `LEDGER_URL` | Ledger gateway, as `svc-core-bridge` (required) |
| `FINERACT_URL` | Fineract API base, for example `http://fineract:8080/fineract-provider/api/v1` (required) |
| `FINERACT_TENANT` | Tenant identifier (default `default`) |
| `FINERACT_USER`, `FINERACT_PASSWORD` | From the secret store (required) |
| `CORE_BRIDGE_OFFICE_ID` | Fineract office for the journals (default 1) |
| `CORE_BRIDGE_POST_INTERVAL` | Posting interval (default 5s) |
| `CORE_BRIDGE_RECON_INTERVAL` | Reconciliation interval (default 24h) |
| `CORE_BRIDGE_ADDR` | Health listener (default `:8098`) |

## Tests

`make test-core-bridge`. Store-backed tests need `CORE_BRIDGE_TEST_DSN`. The live test needs `FINERACT_TEST_URL`,
`FINERACT_TEST_USER` and `FINERACT_TEST_PASSWORD`.
