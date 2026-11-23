#!/usr/bin/env bash
# Tests scripts/backup-freshness-check.sh against throwaway directories. No Docker, no database.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"; chk="$here/backup-freshness-check.sh"
t="$(mktemp -d)"; trap 'rm -rf "$t"' EXIT
fail=0
expect() { # name expected_rc base wal base_h wal_m
  local rc=0; "$chk" "$3" "$4" "$5" "$6" >"$t/out" 2>&1 || rc=$?
  if [ "$rc" != "$2" ]; then echo "FAIL $1: rc $rc, want $2"; cat "$t/out"; fail=1; else echo "ok   $1"; fi
}
stamp() { date -u -d "$1" +%Y%m%dT%H%M%SZ; }
mkdir -p "$t/fresh/base-$(stamp '2 hours ago')" "$t/fresh-wal"; touch "$t/fresh-wal/00000001"
expect "fresh base and WAL pass" 0 "$t/fresh" "$t/fresh-wal" 30 10
mkdir -p "$t/old/base-$(stamp '40 hours ago')"
expect "a 40-hour-old base backup is stale" 1 "$t/old" "$t/fresh-wal" 30 10
mkdir -p "$t/oldwal"; touch -d '30 minutes ago' "$t/oldwal/00000001"
expect "a 30-minute-old WAL archive is stale" 1 "$t/fresh" "$t/oldwal" 30 10
expect "a longer WAL limit accepts it" 0 "$t/fresh" "$t/oldwal" 30 45
mkdir -p "$t/none" "$t/nonewal"
expect "no backups at all fails" 1 "$t/none" "$t/nonewal" 30 10
mkdir -p "$t/junk/base-notadate"
expect "a badly named directory is not a backup" 1 "$t/junk" "$t/fresh-wal" 30 10
exit "$fail"
