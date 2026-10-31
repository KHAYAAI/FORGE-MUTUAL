# compliance-engine (services/shared/compliance-engine)

Entity: shared. Architecture: ARCHITECTURE.md 15.1. v1 scope: "compliance core (sanctions, monitoring, goAML)".

**Status: building blocks, not a running service.** The sanctions loader, the monitoring rules and the case store are
built and tested. There is no ConnectRPC surface, no binary, no payment-event feed, and no goAML reporting.

## What exists

- **Sanctions** (`internal/sanctions`). Loads a normalised list only if its SHA-256 matches the publication, refuses
  unknown fields and empty lists, and builds a screener on the shared `pkg/screening` package. The screener fails
  closed with `ErrStale` once the list is older than its maximum age.
  - The official FIC list's format and channel are **unverified** (VERIFY-FIC). The adapter from the official format
    to this normalised form is not written.
  - Screening is done by each caller today through `pkg/screening`. It is not yet a central service with a
    screening history.
- **Monitoring** (`internal/monitor`). Four rules over a party's events: VELOCITY (24-hour payouts), STRUCTURING
  (three or more payments just under a reporting band in a week), NEW_BENEFICIARY_LARGE (a large first payment to a
  beneficiary), RAPID_IN_OUT (money in mostly paid out again within a day). Each rule is tested at its boundary.
  - Thresholds are **placeholders** (VERIFY).
  - Rules see only the events they are given. A "new" beneficiary is new relative to the supplied history.
- **Cases** (`internal/cases`). One open case per party and rule; a repeat finding does not open a second. SLA by
  severity (CRITICAL 24 hours, WARNING 72 hours, placeholders). Maker-checker: the person who opened a case cannot
  close it. A close needs a note of at least 20 characters. Overdue cases are listed.

## What is missing before this can run

1. **An event feed.** Monitoring needs each party's payment and deposit events. The broker has never run, and
   reading payment tables across services is not allowed. The feed is an open decision.
2. **A service surface and binary.** Screening, cases and the overdue list need a ConnectRPC API, caller
   allow-listing, and a loop that runs monitoring.
3. **Case events.** Cases are not yet written to the outbox.
4. **goAML.** Suspicious and threshold reports need the official schema (VERIFY-GOAML). Submission is refused until
   the schema is confirmed. Reporting is in `reg-reporting`, which is not started.
5. **Sanctions list source.** The publication channel, update cadence and licence are VERIFY items (OpenSanctions
   and FIC).

## Tests

`make test-compliance`. The case store tests need `COMPLIANCE_TEST_DSN` and skip without it.
