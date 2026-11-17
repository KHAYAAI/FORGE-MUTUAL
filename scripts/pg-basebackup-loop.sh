#!/usr/bin/env bash
# Runs in the stack's pg-backup container. Takes a base backup now and then every BACKUP_INTERVAL_SECONDS, verifies
# each one against its manifest, keeps BACKUP_KEEP_DAYS of them, and removes archived WAL older than the oldest kept
# backup needs. A failed backup stops the container, so it is restarted and the failure shows in its log.
set -euo pipefail
interval="${BACKUP_INTERVAL_SECONDS:-86400}"
keep_days="${BACKUP_KEEP_DAYS:-14}"
while true; do
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  dir="/backups/base-$stamp"
  pg_basebackup -h 127.0.0.1 -p 5433 -U postgres -D "$dir.partial" -Ft -z -X none --checkpoint=fast --manifest-checksums=SHA256
  mkdir -p "$dir.verify"
  tar -xzf "$dir.partial/base.tar.gz" -C "$dir.verify"
  pg_verifybackup -n -m "$dir.partial/backup_manifest" "$dir.verify" >/dev/null
  rm -rf "$dir.verify"
  mv "$dir.partial" "$dir"
  echo "pg-backup: base backup $dir taken and verified"
  find /backups -maxdepth 1 -name 'base-*' -type d -mtime "+$keep_days" -exec rm -rf {} +
  oldest="$(ls -d /backups/base-* 2>/dev/null | grep -E "/base-[0-9]{8}T[0-9]{6}Z$" | sort | head -1)"
  if [ -n "$oldest" ]; then
    # pg_basebackup writes the label as "backup_label" (no "./"). A cleanup that cannot run is logged and the loop goes
    # on: exiting here would restart the container, which takes a new base backup at once, over and over.
    first_wal="$(tar -xzOf "$oldest/base.tar.gz" backup_label 2>/dev/null | sed -n 's/^START WAL LOCATION: .* (file \([0-9A-F]*\))$/\1/p' || true)"
    if [ -n "$first_wal" ]; then
      pg_archivecleanup /wal-archive "$first_wal" || echo "pg-backup: WAL cleanup failed; the archive keeps growing until it works" >&2
    else
      echo "pg-backup: no backup_label in $oldest; WAL not cleaned up" >&2
    fi
  fi
  sleep "$interval"
done
