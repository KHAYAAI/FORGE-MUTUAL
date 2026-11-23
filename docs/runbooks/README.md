# Runbooks

One section per alert in `deploy/monitoring/alerts.csv`, under a heading with the alert's id. Each says what the alert
means, what to check first, what to do, and who to tell. They are written from how the code behaves today. Production does
not exist yet, so commands that depend on production infrastructure are described, not given.

**Who to tell.** Roles are not yet filled (see `docs/THRESHOLD_OWNERS.md`). Until they are, the person on call escalates to the
CEO. If an incident may be material (it disrupts operations, customers or the wider system severely), start
`docs/regulatory/policies/incident-response-plan.md` at once: the 24-hour clock starts when the incident is classified as
material, not when it is found.

## recon-break-open

**Means.** A reconciliation check found a difference between the books and the bank, the ledger and the general ledger, or
the in-flight terms. The policy rule is strict: while any break is open, every outbound payment is held, whatever its size
(`docs/DECISIONS.md`, 2026-10-31).

**Check first.** Which asset and which of the five checks. Whether the run was inconclusive because the ledger was moving
(that is not a break). Whether an inbound credit or a settled payment is in flight.

**Do.** Find the cause before releasing anything. Do not clear a break by hand. Fix the source (a missing booking, a
duplicated posting) through the normal workflow with maker-checker approval. Payments release when the next run is clean.

**Tell.** Operations lead at once. If customer funds may be wrong, treat it as a possible material incident.

## audit-chain-invalid

**Means.** `forge-audit verify` found a broken hash chain or a head that disagrees with its anchor. Either records were
altered, or the store or anchor is damaged.

**Check first.** Whether the failure is at one record or the whole chain. Whether the anchor file is the latest. Whether a
restore or migration happened just before.

**Do.** Preserve everything: take a copy of the database and the anchors before touching them. Do not repair the chain.
Compare with the last known-good anchor and with backups to find the first bad record. This is a security incident until
shown otherwise.

**Tell.** Security lead and CEO at once. Start the incident response plan.

## backup-stale

**Means.** The newest base backup is older than 30 hours (limit set when the check is scheduled).

**Check first.** Whether the backup service is running and its last log lines. Disk space on the backup volume. Whether
Postgres is reachable.

**Do.** Restart the backup service, confirm a new `base-<timestamp>` directory appears and passes its checksum. Until it
does, treat recovery as limited to the last good base backup plus archived WAL.

**Tell.** Engineering lead. Raise to security lead if the cause is not plain (for example, backups deleted).

## wal-archive-stale

**Means.** No WAL file has arrived in the archive for longer than the limit. Point-in-time recovery now stops at the last
archived segment, so every minute is data that could be lost.

**Check first.** Postgres `archive_command` failures in its log. Whether the archive volume is full or read-only.

**Do.** Fix the archive target first. Do not delete WAL that backups still need. Once archiving resumes, run
`scripts/pitr-check.sh` on the local stack as a proof of method, and on production once it exists.

**Tell.** Engineering lead.

## offsite-copy-stale

**Means.** No verified offsite copy is recent. Backups exist only where the database runs.

**Check first.** The receipt file from `scripts/offsite-sync.sh` and its time. The job's log. Credentials and the bucket's
retention setting.

**Do.** Re-run the job. A failed `rclone check` means a bad copy: keep the local data, do not delete anything locally until a
verified copy exists. Never loosen the bucket's retention to make a job pass.

**Tell.** Engineering lead. Security lead if credentials may be compromised.

## outbox-stuck

**Means.** Events are waiting in an outbox. The relay refuses to publish when Kafka cannot make a write durable (fewer
brokers in sync than required), by design, and tries again on its next pass.

**Check first.** Kafka brokers and in-sync replicas. The relay's log for the last error. The age of the oldest unpublished
row.

**Do.** Restore Kafka quorum. Do not publish by hand or reorder events. When the brokers recover the relay delivers in order
(proven by `make test-kafka-cluster`).

**Tell.** Engineering lead. If notifications or consolidated views are visibly stale for customers, tell operations.

## treasury-alert-raised

**Means.** Treasury risk raised a concentration, net outflow, liquidity or suspense alert. Thresholds are placeholders today
(`docs/THRESHOLDS.md`), so an alert is a prompt to look, not a measured breach.

**Check first.** Which alert, which asset, and the numbers it was raised on.

**Do.** Operations lead decides whether to act (for example slowing payouts). Record the decision. The alert clears itself when
the condition clears.

**Tell.** Operations lead; the named owner of the threshold once there is one.

## service-cert-expiring

**Means.** A service certificate expires within 7 days. Services speak mutual TLS and refuse callers without a valid
certificate, so an expired certificate takes the service out.

**Check first.** Which certificate and who issues it. Whether automatic renewal is configured and why it has not run.

**Do.** Renew before expiry. The local stack proves certificates can be renewed on disk without restarting services
(`deploy/compose/check-mtls.sh`); production needs the same with its own certificate authority.

**Tell.** Engineering lead.

## auth-refusal-spike

**Means.** Many sign-ins or tokens were refused in a short time, including refusals for missing multi-factor authentication
(`ErrMFARequired`). It may be an attack, or a misconfigured provider after a change.

**Check first.** Whether it follows a deploy or a provider setting change. The source addresses and accounts involved.

**Do.** If a provider change caused it, revert the change. If it is an attack, block the source and review privileged accounts.
Never lower the multi-factor requirement to stop the refusals.

**Tell.** Security lead.

## policy-bundle-refused

**Means.** The policy service refused a bundle whose signature does not verify. It keeps running the last good bundle.

**Check first.** Who published the bundle and when. Whether the signing key was rotated.

**Do.** Do not bypass the check. Rebuild and sign the bundle from the repository through the normal process. If the key may be
compromised, treat it as a security incident.

**Tell.** Security lead and compliance lead.

## trace-pipeline-down

**Means.** Spans are not reaching the collector, so one request can no longer be followed across services.

**Check first.** The collector's health and disk. Network policy between services and the collector.

**Do.** Restore the collector. Note the gap, because incident reviews will lack traces for it.

**Tell.** Engineering lead.

## incident-clock-due-soon

**Means.** A material incident has an obligation to the regulator within 6 hours or already overdue: immediate notification
(24 hours from classification), subsequent update (14 calendar days from notification) or the agreed full report.

**Check first.** Which obligation and its due time. Whether the template tabs are complete (`ImmediateMissing`, `UpdateMissing`).

**Do.** Complete the template, have a second person approve it, submit on the Umoja portal, and record the portal reference.
If late, still submit at once and record why. Do not wait to be complete: the template allows "To be determined" where the
update tab allows it, and the full report follows.

**Tell.** CEO and the board's risk committee chair.

## dependency-vulnerability

**Means.** `make vulncheck` or `npm audit` found a vulnerability that shipped code reaches.

**Check first.** Whether the vulnerable function is really called, the fixed version, and the exposure (internet-facing or internal).

**Do.** Upgrade and re-run the gate. If no fix exists, add a compensating control and a dated entry in the remediation register.
Joint Standard 2 of 2024, 7.7.6, requires severity, deadlines and tracking for every finding.

**Tell.** Engineering lead; security lead for anything reachable from the internet.
