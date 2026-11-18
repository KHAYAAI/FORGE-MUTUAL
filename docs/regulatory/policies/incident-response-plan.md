> **DRAFT for board approval. Not approved. Not in force.** Prepared by Claude from the repository and the regulatory register on 2026-11-17. Every figure marked PROPOSED is a suggestion for the owner to confirm or change. A policy takes effect only when the board (or, before a board exists, the named accountable person) approves it and records the date. Approver: ________  Date: ________

# Incident response plan

Purpose: detect, classify, contain and report IT and cyber incidents, and notify the regulators on time (Joint Notice 2 of 2026; Joint Standard 2 of 2024, 7.6 and 7.3). Register ids: JN-01, JN-02, JN-03, JS2-14, JS2-18.

## Roles (to be named)
- Incident lead: engineering lead. Decides containment.
- Materiality decision-maker: CEO, with the compliance lead. Classifies an incident as material or not and records the criteria used.
- Approver of regulatory notifications: a second person who is not the preparer.
- Communications: CEO.

## Steps
1. **Detect and log.** Any person who suspects an incident reports it to the incident lead at once. Record it with `incident.Detect` (services/bank/reg-reporting/internal/incident): time detected and who found it.
2. **Contain.** Stop the damage first: revoke sessions and keys, isolate services. Preserve evidence; do not repair or delete logs or the audit chain.
3. **Classify.** The materiality decision-maker decides material or not material and records the criteria. The 24-hour clock runs from the decision that it is material. Decide promptly; an unreasonable delay in classifying is itself a finding.
4. **Notify (within 24 clock hours of the material classification).** Complete Annexure A tabs 2 and 3 (and tab 4 for a cyber incident). Unknown fields: say so; do not leave blank. A second person approves. Submit on the PA's Umoja portal and record the portal reference (`Notify`).
5. **Update (within 14 calendar days of notification).** Complete tab 5 with impact, root cause and recovery (`SendUpdate`).
6. **Full report.** Agree a date with the PA and deliver it (`AgreeFullReport`, `SendFullReport`).
7. **Review.** Within 30 days: what failed, what changes, who owns each change. Feed lessons into the risk register.

## Customer and third-party notice
If personal information may have been accessed, assess notice to the Information Regulator and affected people (POPIA s22; legal review needed, not yet done). Notify sponsor bank, partner banks and providers as their agreements require (agreements not yet written).

## Testing
Run a tabletop exercise at least yearly and after any material change. PROPOSED: first exercise before any customer money is held. Record the date and findings.

## Known limits
- The code does not submit to Umoja; a person does (VERIFY-UMOJA).
- Reporting deadlines in the code count 14 days as 14×24 hours; confirm with the PA.
