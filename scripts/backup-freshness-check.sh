#!/usr/bin/env bash
# Checks that backups are recent. Exit 0 when fresh, 1 when stale or missing. Prints one line per check.
#
#   scripts/backup-freshness-check.sh BASE_DIR WAL_DIR [MAX_BASE_AGE_HOURS=30] [MAX_WAL_AGE_MINUTES=10]
#
# BASE_DIR holds base backups named base-YYYYMMDDTHHMMSSZ (what scripts/pg-basebackup-loop.sh writes). WAL_DIR is the WAL
# archive. A busy cluster switches WAL at least once a minute, so a WAL archive that is ten minutes old means archiving
# has stopped. An idle cluster may be quiet for longer: set MAX_WAL_AGE_MINUTES to suit (Joint Standard 2 of 2024, 7.4.1(c)).
# Run it from a scheduler and alert on a non-zero exit (docs/runbooks/README.md, backup-stale and wal-archive-stale).
set -euo pipefail
base="${1:?base backup directory}"; wal="${2:?WAL archive directory}"
max_base_h="${3:-30}"; max_wal_m="${4:-10}"
now="${NOW_EPOCH:-$(date -u +%s)}"
rc=0

latest="$(ls -d "$base"/base-* 2>/dev/null | grep -E '/base-[0-9]{8}T[0-9]{6}Z$' | sort | tail -1 || true)"
if [ -z "$latest" ]; then
  echo "backup-stale: no base backup in $base"; rc=1
else
  stamp="${latest##*/base-}"
  ts="$(date -u -d "${stamp:0:4}-${stamp:4:2}-${stamp:6:2} ${stamp:9:2}:${stamp:11:2}:${stamp:13:2}" +%s)"
  age_h=$(( (now - ts) / 3600 ))
  if [ "$age_h" -gt "$max_base_h" ]; then echo "backup-stale: newest base backup ${stamp} is ${age_h}h old (limit ${max_base_h}h)"; rc=1
  else echo "ok: newest base backup ${stamp} is ${age_h}h old"; fi
fi

newest_wal="$(find "$wal" -maxdepth 1 -type f -printf '%T@\n' 2>/dev/null | sort -n | tail -1 || true)"
if [ -z "$newest_wal" ]; then
  echo "wal-archive-stale: no WAL files in $wal"; rc=1
else
  age_m=$(( (now - ${newest_wal%.*}) / 60 ))
  if [ "$age_m" -gt "$max_wal_m" ]; then echo "wal-archive-stale: newest WAL file is ${age_m}m old (limit ${max_wal_m}m)"; rc=1
  else echo "ok: newest WAL file is ${age_m}m old"; fi
fi
exit "$rc"
