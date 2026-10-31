# Decisions and open items

Owner decisions that change behaviour, with the date, who took them and what they changed. `ARCHITECTURE.md` is not
edited by this log. Where a decision is stricter than the architecture, the entry says so, and the architecture needs
a revision at the next documentation pass.

## Taken

**2026-10-31: payments are held for any open reconciliation break (strict).**
While any monitored asset has a reconciliation break, every outbound payment is held, whatever its amount. There is no
threshold. Taken by the product owner in session.
- Changed: `policy/forge/recon` (the threshold rule is removed). `policy/forge/limits` already requires
  `break_active == false` before it allows anything, so the two rules now agree.
- Architecture: section 13 and invariant 9 describe holding payments above a threshold. This is stricter.

**2026-10-31: the dev stack runs a single-node Kafka.**
`deploy/compose/docker-compose.yml` runs `apache/kafka:3.9.0` in KRaft mode, the version the outbox integration test
has run against (`make test-outbox`). Topics are not auto-created. Taken in session.

**2026-10-31: `make ci` requires the database and broker suites to run.**
`make ci` refuses to pass unless every database and broker variable is set (`DB_TEST_VARS` in the Makefile).
`make ci-db` creates throwaway databases with `scripts/test-databases.sh` and runs the gate. The only opt-out is
`FORGE_SKIP_DB_TESTS=1`, which prints a warning and proves less. Taken in session, from the observation that a skipped
suite can hide a broken one.

**2026-10-31: the Didit integration uses a new organisation, not A-CARD.**
Taken in session. **Not yet done:** the connected Didit account lists only the A-CARD organisation, and no tool in this
environment can create an organisation. The owner must create it, grant access to the integration account, and give
me the sandbox application's identifier through the secret store, not in chat. No verification session has been
created, and creating one is not yet approved.

## Open (need a named owner or a decision)

- **Sanctions list source and licence.** Recommendation in `docs/SANCTIONS_SOURCES.md`. Not decided.
- **Threshold owners.** Every placeholder, with its required owner role, is in `docs/THRESHOLDS.md`. No owner is named.
- **Commit attribution.** Commits carry `Co-Authored-By: Claude <noreply@anthropic.com>` and the session trailer, with
  no model name. The attribution reminder asked for the model name, but the system instructions forbid model
  identifiers in commits. Say if you want the name added.
- **Pull requests.** None opened. Not requested. Say if you want one.
- **TigerBeetle cluster.** Not run. The server binary and image cannot be obtained from this environment (see README).
- **WORM storage** for audit anchors and evidence packs (VERIFY-WORM).
- **Customer USD account flow** and the partner-bank router. Not started.
