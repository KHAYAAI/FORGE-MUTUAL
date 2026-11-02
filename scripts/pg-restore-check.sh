#!/usr/bin/env bash
# Proves a backup restores: checks the checksum, restores the dump into a scratch database on a server, and compares
# the row count of every table with the counts recorded at backup time. The scratch database is dropped afterwards.
#
#   scripts/pg-restore-check.sh <dump-file> <server-url-without-database>
#
# pg_restore must be the server's major version or newer than the dump's format; set PG_TOOLS_IMAGE to a postgres
# image of the server's version (for example postgres:17) to run it in a container, as pg-backup.sh does.
# Exits 0 only when the checksum matches and every table restores with exactly the recorded row count.
set -euo pipefail

dump="${1:?usage: pg-restore-check.sh <dump-file> <server-url>}"
server="${2:?usage: pg-restore-check.sh <dump-file> <server-url>}"
server="${server%/}"
scratch="restore_check_$(date -u +%s)_$$"
work="$(mktemp -d)"
trap 'psql "$server/postgres?sslmode=disable" -q -c "drop database if exists $scratch with (force)" >/dev/null 2>&1 || true; rm -rf "$work"' EXIT

(cd "$(dirname "$dump")" && sha256sum -c "$(basename "$dump").sha256" >/dev/null) || { echo "restore-check: checksum does not match"; exit 1; }

psql "$server/postgres?sslmode=disable" -q -v ON_ERROR_STOP=1 -c "create database $scratch" >/dev/null
if [ -n "${PG_TOOLS_IMAGE:-}" ]; then
  docker run --rm --network host -v "$(cd "$(dirname "$dump")" && pwd):/in:ro" "$PG_TOOLS_IMAGE" \
    pg_restore --no-owner --no-privileges --exit-on-error --dbname="$server/$scratch?sslmode=disable" "/in/$(basename "$dump")"
else
  server_major="$(psql "$server/postgres?sslmode=disable" -Atqc "show server_version_num" | awk '{print int($1/10000)}')"
  local_major="$(pg_restore --version | grep -oE '[0-9]+(\.[0-9]+)?' | head -1 | cut -d. -f1)"
  if [ "$local_major" != "$server_major" ]; then
    echo "restore-check: local pg_restore is major $local_major, the server is $server_major; set PG_TOOLS_IMAGE=postgres:$server_major" >&2
    exit 1
  fi
  pg_restore --no-owner --no-privileges --exit-on-error --dbname="$server/$scratch?sslmode=disable" "$dump"
fi

fail=0
while read -r table expected; do
  got="$(psql "$server/$scratch?sslmode=disable" -Atqc "select count(*) from public.\"$table\"")"
  if [ "$got" != "$expected" ]; then
    echo "restore-check: $table has $got rows, the backup recorded $expected"
    fail=1
  fi
done < "$dump.counts"

tables="$(wc -l < "$dump.counts")"
if [ "$fail" -ne 0 ]; then
  echo "restore-check: FAILED"
  exit 1
fi
echo "restore-check: ok, $tables tables restored with recorded row counts"
