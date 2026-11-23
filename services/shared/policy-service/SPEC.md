# SPEC: policy-service

Status: built and tested locally. Human review required before any real customer (ARCHITECTURE.md section 0, rule 3).
Entity: shared (serves every entity, holds no balances). Sections referenced: ARCHITECTURE.md 2.2, 10, 6.3, Appendix C.

## 1. Purpose

Answers allow, deny and step-up decisions for other services. OPA is embedded as a Go library
(`github.com/open-policy-agent/opa`). The rules are Rego in `policy/` at the repository root. Every decision is
written to the audit outbox (`forge.shared.policy.decision.v1`) before it is returned (invariant 14).

## 2. Interface

ConnectRPC `forge.policy.v1.PolicyService/Evaluate`. The request carries a decision point, the entity, a caller
correlation id, and an input object. The response carries allow, stepup, reason, bundle version and decision id.

| Decision point | Rego query | Decides |
|---|---|---|
| `limits` | `data.forge.limits.decision` | Per-transaction, daily and step-up limits by tier; party status; screening; recon break |
| `assets` | `data.forge.assets.decision` | Whether an entity may hold an asset |
| `entity` | `data.forge.entity.decision` | Cross-entity transfers (denied) and person-to-entity code 63 (needs approval and a verified relationship) |
| `recon` | `data.forge.recon.decision` | Payments held while a sponsor reconciliation break is open |
| `sector` | `data.forge.sector.decision` | Sector eligibility. Only sectors in `sectors.json` pass (ADR-0012 open) |

Money in the input is a decimal string (`amount_minor: "20000000"`). Rego converts it with `to_number`. A float
on the wire would lose precision above about 9x10^15 minor units and breaks invariant 5.

## 3. Fail-closed rules

- Unknown decision point: `INVALID_ARGUMENT`, nothing audited.
- No verified bundle loaded: deny with reason `policy_bundle_unavailable`, audited.
- Rule undefined, errored or returning the wrong shape: deny with reason `evaluation_error`, audited.
- Audit write fails: `UNAVAILABLE`. The decision is not returned, so an unaudited allow cannot happen.
- Missing input fields are denials. Rego conditions compare with `== true` or `== false` explicitly, so an absent
  field never passes as false.
- Caller allow-list, deny by default (`pkg/callerauth`).

## 4. Bundle signing and verification

- A bundle is `policy/`: Rego under `forge/`, and JSON data at the top level (each file becomes `data.<name>`).
- `manifest.json` lists the SHA-256 of every bundle file. `manifest.sig` is an Ed25519 signature over the exact
  manifest bytes. Test files, `README.md`, `manifest.*` are outside the bundle.
- Startup refuses to run if the signature fails, a file is altered, or a file is missing or extra.
- Development can set `POLICY_DEV_UNSIGNED=true`, which skips verification. The service refuses that setting when
  `POLICY_TRUSTED_KEYS` is also set, so dev mode cannot bypass a configured trust root.
- Signing: `policy-sign -dir policy -version <v>` with `POLICY_SIGNING_KEY` from the secret store. The key must
  never be committed (see the Makefile `policy-manifest` target).

## 5. Tests

- `internal/engine`: every Rego test in `policy/` runs through OPA's tester, using the same data the service loads.
  Mutation check: breaking a rule fails the suite.
  Decision table: sample inputs for every decision point, including fail-closed cases.
  A signed copy of the real bundle is verified, compiled and evaluated.
- `internal/bundle`: signature checks, altered, extra and missing files, wrong key, dev-mode guard.
- `internal/service` (needs `POLICY_TEST_DSN`): every decision is audited, denials are audited, no-bundle denial is
  audited, unknown points write nothing, an unaudited decision is not returned.
- Smoke run: the signed bundle was served over HTTP, a tampered rule stopped startup.

## 6. Versions and deviations

- **OPA is pinned to v1.13.0.** Later v1 releases require Go 1.25 or 1.26. This environment has Go 1.24.7. Move
  the pin when the toolchain moves.
- **`tester.Run` does not load the root JSON files as data**, so the Rego tests use the bundle loader's data
  instead. Tests run against the production data path.
- **Limits in `tiers.json` and the sector list in `sectors.json` are placeholders.** The risk owner and ADR-0012
  must confirm them before any real onboarding.
- **Not built:** `forge.banklayer` (Stage B), `forge.consent` (consent checks are answered by consent-service
  directly, so a second copy would drift), and hot reload of a new bundle (`Swap` exists; nothing calls it yet).

## 7. Open items

- VERIFY-KAFKA: audit topic delivery to a real broker.
- The Rego rules have not been reviewed by counsel or the risk owner (ARCHITECTURE.md section 0).
