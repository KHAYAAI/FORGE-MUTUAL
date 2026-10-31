# Sanctions list source and licence: recommendation

**Status: recommendation, not decided.** The owner must choose, and compliance counsel must sign off the licence. This
memo is written from secondary sources. The FIC, UN and OpenSanctions sites were not reachable from the build
environment, so each point below is marked as checked or unchecked. Compliance must check the unchecked points on the
primary sites before anything is bought or built.

## What each source is

| Source | What it gives | Licence (as reported) | Status |
|---|---|---|---|
| **FIC targeted financial sanctions (TFS) list**, South Africa | The list that South African accountable institutions must screen against, published by the Financial Intelligence Centre in Excel, PDF and XML. | Official publication. Obligations under FIC Act sections 26A to 26C and POCDATARA section 25. | Reported, not checked on the FIC site. |
| **UN Security Council consolidated list** | The source list. The FIC list is reported to replicate it, with identity details from FIC Director notices. Published in XML, PDF and HTML. | Reported as no open licence. The UN terms page was not reachable. | Reported, not checked. |
| **OpenSanctions** | Consolidated sanctions and PEP data, with bulk download and hosted screening. | Free for non-commercial use under CC BY-NC 4.0. A business needs a paid licence. Hosted screening is billed per query. Startup discounts exist. Prices are not published. | Checked against its own site (search results). |
| **Commercial screening vendors** (for example LSEG World-Check, Dow Jones, ComplyAdvantage) | Sanctions, PEP and adverse media with service levels and support. Common at banks. | Contract, priced on quote. | Not evaluated. |

## Recommendation

**For live launch, use three layers:**

1. **The FIC list, as the legal baseline, from day one.** It is the list the South African obligations name. It is
   free. Ingest the official file, and check its version and date every day.
2. **The UN consolidated list, as a cross-check.** Compare it with the FIC list. A gap means the FIC list is behind
   the UN, which is worth an alert. This is a control, not a second source of truth.
3. **A licensed data feed for PEP and adverse media, before live launch.** The official lists do not cover PEPs. For
   this layer, the realistic choice is the **OpenSanctions commercial licence**: its terms are published, it offers bulk
   data, and it is cheaper to start with than a tier-1 vendor. Get a written quote before choosing, because the price
   is not public.

**Industry standard later:** a tier-1 commercial vendor brings service levels, adverse-media coverage and a support
team. Mature banks usually run one. It is a reasonable upgrade once volumes or the regulator's expectations justify the
cost. It does not have to be in place for launch if counsel accepts layers 1 to 3 for the early stage.

## Engineering consequences (already in code, and what is missing)

- `internal/sanctions` refuses a list whose SHA-256 does not match, refuses unknown fields and empty lists, and
  fails closed when the list is older than its maximum age. Set the maximum age to about one day, to match the
  reported update timing (one source says within 24 hours of UN changes).
- **Not built:** the adapter that reads the FIC file into Forge's normalised format. It has to be written from the
  official file, so it waits for a sample of the real file. Writing it from a guessed format would be wrong.
- **Not built:** a licensed PEP feed. It waits for the licence decision.
- The payments service screens beneficiaries through `pkg/screening`. Until the compliance-engine is a running
  service, each caller still uses its own fixture list. That gap must close before launch.

## What compliance must check on the primary sites

- The FIC TFS page: the file formats, the update process and the update timing.
- The UN terms of use: whether the consolidated list can be redistributed, and whether an internal copy is allowed.
- The OpenSanctions commercial terms and a written quote for the business licence.
- Whether counsel accepts the three-layer design above as the screening control.
