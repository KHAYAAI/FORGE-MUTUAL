#!/usr/bin/env bash
# Copies base backups and the WAL archive to an offsite, immutable bucket and verifies the copy. UNTESTED against a real
# bucket: none exists yet. Needs rclone and a remote configured for an S3-compatible store.
#
#   OFFSITE_REMOTE=forge-offsite:forge-backups scripts/offsite-sync.sh BASE_DIR WAL_DIR RECEIPT_FILE
#
# What the bucket must provide (Joint Standard 2 of 2024, 7.4.1(d); VERIFY-WORM):
#   - Object Lock in compliance mode, or an equivalent write-once setting, with retention at least as long as the backups
#     are kept locally plus a margin.
#   - A South African region unless counsel has approved otherwise (Directive 3/2018).
#   - Credentials for this job that can write and list but cannot delete or shorten retention.
#   - Encryption at rest. The backups hold personal data: encrypt them before upload as well (rclone crypt) with a key held
#     apart from the bucket credentials.
# The script only ever copies. It never deletes anything, locally or offsite. DRY_RUN=1 prints what would be copied.
set -euo pipefail
base="${1:?base backup directory}"; wal="${2:?WAL archive directory}"; receipt="${3:?receipt file}"
: "${OFFSITE_REMOTE:?set OFFSITE_REMOTE to the rclone remote and bucket, for example forge-offsite:forge-backups}"
command -v rclone >/dev/null || { echo "offsite-sync: rclone is not installed" >&2; exit 2; }
dry=(); [ "${DRY_RUN:-}" = 1 ] && dry=(--dry-run)

started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
rclone copy "${dry[@]}" --checksum "$base" "$OFFSITE_REMOTE/base"
rclone copy "${dry[@]}" --checksum "$wal" "$OFFSITE_REMOTE/wal"
if [ -z "${dry[*]:-}" ]; then
  rclone check --one-way "$base" "$OFFSITE_REMOTE/base"
  rclone check --one-way "$wal" "$OFFSITE_REMOTE/wal"
  printf '{"started":"%s","finished":"%s","remote":"%s","verified":true}\n' "$started" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$OFFSITE_REMOTE" > "$receipt"
  echo "offsite-sync: copied and verified; receipt $receipt"
fi
