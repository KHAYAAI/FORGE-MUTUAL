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

**2026-11-03: sign-in uses WorkOS (AuthKit).**
Taken by the owner in session. The apps already verify tokens from any OIDC provider (`pkg/oidc`), so the choice changes
configuration and a claim mapping, not the verifier. **Not yet done:** no WorkOS account exists for this project, the
integration has not been run against WorkOS, and the claims the apps need (`forge_party` for a customer, `forge_roles`
for staff) are a proposal until a real token has been inspected. Details and the owner's steps: `docs/PROVIDERS.md`.

**2026-11-03: KYC and KYB use Didit, in a new account the owner creates.**
Taken by the owner in session, and it confirms the 2026-10-31 decision: a new organisation, not A-CARD. The owner signs
up with a personal email address (given in session, not recorded here). **Not yet done:** the account does not exist.
Sign-up needs an emailed code that only the owner can read, and the Didit and WorkOS connectors in the build session are
signed in to A-CARD, so they must not be used for this project. The adapter in `services/platform/onboarding` is still a
stub that refuses every verification. No verification session is approved until the new organisation exists.

## Open (need a named owner or a decision)

- **Provider accounts.** The WorkOS account and the Didit organisation are decided but not created. The build session's
  network policy also blocks the providers' hosts (`docs/PROVIDERS.md` lists them), so nothing can be run against either
  until the owner allows them.
- **How an applicant gets in.** Decide whether an applicant signs in with WorkOS before applying (recommended: a verified
  email, and rate-limiting by account) or applies anonymously and receives credentials after approval. The customer
  application flow cannot be designed until this is decided.

- **Sanctions list source and licence.** Recommendation in `docs/SANCTIONS_SOURCES.md`. Not decided.
- **Threshold owners.** Every placeholder, with its required owner role, is in `docs/THRESHOLDS.md`. No owner is named.
- **Commit attribution.** Commits carry `Co-Authored-By: Claude <noreply@anthropic.com>` and the session trailer, with
  no model name. The attribution reminder asked for the model name, but the system instructions forbid model
  identifiers in commits. Say if you want the name added.
- **Pull requests.** None opened. Not requested. Say if you want one.
- **TigerBeetle cluster.** Not run. The server binary and image cannot be obtained from this environment (see README).
- **WORM storage** for audit anchors and evidence packs (VERIFY-WORM).
- **Customer USD account flow** and the partner-bank router. Not started.
