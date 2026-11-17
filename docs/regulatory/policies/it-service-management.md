> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# IT service management: change, release, incident, problem and capacity (Joint Standard 1 of 2023)

Register id: JS1-07.

**Change.** Every change is a pull request: reviewed by someone other than the author, passing the full gate (tests,
isolation, regulatory checks, vulnerability scan) before merge. Production changes need a second approver for the
release (PROPOSED). Emergency changes may skip the waiting period, never the gate, and are reviewed within 2 working days.

**Release.** Images are built from a tagged commit with an SBOM (`make sbom`); the release notes name the change
request and the approver. Database changes are backward compatible for one release, so a release can be rolled back.

**Incident.** `incident-response-plan.md`; material incidents are notified under Joint Notice 2 of 2026
(reg-reporting `internal/incident`).

**Problem.** Every incident of severity 2 or higher gets a root-cause analysis within 10 working days (PROPOSED), with
actions tracked to closure. This repository's commit history already records root causes for the defects found
(for example the backup loop and the simulator reference reuse).

**Capacity.** Ledger throughput is measured (`make load-ledger`: about 400 transfers a second on one machine); the
production cluster must be measured before launch. Alerts in `deploy/monitoring/alerts.csv` cover saturation. Review
quarterly against growth (PROPOSED).
