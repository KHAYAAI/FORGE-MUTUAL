#!/usr/bin/env bash
# Proves point-in-time recovery on the running local stack. It does not touch the stack's own data directory: the
# restore goes into a new, throwaway Postgres.
#
#   1. writes a row, then waits for the scheduled backup service to take a base backup after it;
#   2. writes a second row, notes the time T, and then does damage: writes a third row and drops a table;
#   3. restores the latest base backup into a fresh Postgres, replays the WAL archive up to T, and checks that the
#      first two rows are there, the third is not, and the dropped table is back.
#
#   scripts/pitr-check.sh        (with the stack up: deploy/compose/up.sh)
set -euo pipefail

compose=(docker compose -f "$(cd "$(dirname "$0")/.." && pwd)/deploy/compose/docker-compose.yml")
url="postgres://postgres:forge-local-only@127.0.0.1:5433/postgres?sslmode=disable"
q() { psql "$url" -Atqc "$1"; }
restored=forge-pitr-check
port=5499
docker rm -f "$restored" >/dev/null 2>&1 || true
docker volume rm "$restored" >/dev/null 2>&1 || true
trap 'docker rm -f "$restored" >/dev/null 2>&1 || true; docker volume rm "$restored" >/dev/null 2>&1 || true' EXIT

run_id="$(date +%s)"
q "create table if not exists pitr_check (run bigint, label text, at timestamptz default clock_timestamp())"
q "create table if not exists pitr_victim (run bigint)"
q "insert into pitr_victim values ($run_id)"
q "insert into pitr_check (run, label) values ($run_id, 'before-backup')"

# A base backup that starts after the first row. The backup service takes one at start; this asks for one now by
# restarting it, which is what its schedule does every BACKUP_INTERVAL_SECONDS.
before="$(date -u +%Y%m%dT%H%M%SZ)"
sleep 1
"${compose[@]}" restart pg-backup >/dev/null
backup=""
for _ in $(seq 1 120); do
  latest="$("${compose[@]}" exec -T pg-backup sh -c 'ls -d /backups/base-* 2>/dev/null | grep -E "/base-[0-9]{8}T[0-9]{6}Z$" | sort | tail -1' | tr -d '\r')"
  if [ -n "$latest" ] && [[ "${latest#/backups/base-}" > "$before" ]]; then backup="$latest"; break; fi
  sleep 1
done
[ -n "$backup" ] || { echo "pitr-check: the backup service took no base backup within two minutes" >&2; exit 1; }
echo "pitr-check: base backup $backup"

q "insert into pitr_check (run, label) values ($run_id, 'kept')"
target="$(q "select clock_timestamp()")"
sleep 2
q "insert into pitr_check (run, label) values ($run_id, 'after-target')"
q "drop table pitr_victim"
# Close the current WAL segment so the archive has everything up to now, without waiting for archive_timeout.
q "select pg_switch_wal()" >/dev/null
sleep 3
echo "pitr-check: restoring to $target, before a later row and a dropped table"

# The restore: base backup into a fresh data directory, the archive mounted read-only, recovery to the target time.
docker volume create "$restored" >/dev/null
docker run --rm -v forge-local_pg-base:/backups:ro -v "$restored":/var/lib/postgresql/data postgres:17 bash -c "
  set -e
  tar -xzf $backup/base.tar.gz -C /var/lib/postgresql/data
  cat >> /var/lib/postgresql/data/postgresql.auto.conf <<CONF
restore_command = 'cp /wal-archive/%f %p'
recovery_target_time = '$target'
recovery_target_action = 'promote'
archive_mode = off
CONF
  touch /var/lib/postgresql/data/recovery.signal
  chown -R postgres:postgres /var/lib/postgresql/data
  chmod 0700 /var/lib/postgresql/data"
docker run -d --name "$restored" --network host -v "$restored":/var/lib/postgresql/data -v forge-local_pg-wal:/wal-archive:ro \
  postgres:17 postgres -p "$port" >/dev/null
rurl="postgres://postgres:forge-local-only@127.0.0.1:$port/postgres?sslmode=disable"
for _ in $(seq 1 120); do
  if [ "$(psql "$rurl" -Atqc "select not pg_is_in_recovery()" 2>/dev/null)" = "t" ]; then break; fi
  sleep 1
done
[ "$(psql "$rurl" -Atqc "select not pg_is_in_recovery()" 2>/dev/null)" = "t" ] || {
  echo "pitr-check: the restored database did not finish recovery" >&2; docker logs "$restored" 2>&1 | tail -20 >&2; exit 1; }

labels="$(psql "$rurl" -Atqc "select string_agg(label, ',' order by at) from pitr_check where run = $run_id")"
victim="$(psql "$rurl" -Atqc "select count(*) from pitr_victim where run = $run_id" 2>/dev/null || echo missing)"
echo "pitr-check: restored rows: $labels; the dropped table holds $victim row(s)"
q "drop table if exists pitr_check" >/dev/null
if [ "$labels" = "before-backup,kept" ] && [ "$victim" = "1" ]; then
  echo "pitr-check: ok: restored to the moment chosen, with the later damage undone"
else
  echo "pitr-check: FAIL: expected rows before-backup,kept and the dropped table with 1 row" >&2
  exit 1
fi
