#!/usr/bin/env bash
# Backs up one Postgres database: a custom-format dump, its SHA-256, and the exact row count of every table in the
# public schema (the restore check compares against these counts).
#
#   scripts/pg-backup.sh <postgres-url-of-the-database> <output-dir>
#
# pg_dump must be the server's major version: a newer server cannot be dumped by an older client. If the local
# pg_dump is a different major version, set PG_TOOLS_IMAGE to a postgres image of the server's version (for example
# postgres:17) and the dump runs in a container with the host network.
set -euo pipefail

url="${1:?usage: pg-backup.sh <postgres-url> <output-dir>}"
out="${2:?usage: pg-backup.sh <postgres-url> <output-dir>}"
mkdir -p "$out"

server_major="$(psql "$url" -Atqc "show server_version_num" | awk '{print int($1/10000)}')"
name="$(psql "$url" -Atqc "select current_database()")"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
dump="$out/$name-$stamp.dump"

run_pg() { # run_pg <tool> [args...]: the tool, local or in the matching image
  local tool="$1"; shift
  if [ -n "${PG_TOOLS_IMAGE:-}" ]; then
    docker run --rm --network host -v "$(cd "$out" && pwd):/out" "$PG_TOOLS_IMAGE" "$tool" "$@"
  else
    local local_major
    local_major="$("$tool" --version | grep -oE '[0-9]+(\.[0-9]+)?' | head -1 | cut -d. -f1)"
    if [ "$local_major" != "$server_major" ]; then
      echo "pg-backup: local $tool is major $local_major, the server is $server_major; set PG_TOOLS_IMAGE=postgres:$server_major" >&2
      exit 1
    fi
    "$tool" "$@"
  fi
}

# The dump is written into the output directory; with a container the path inside it is /out.
if [ -n "${PG_TOOLS_IMAGE:-}" ]; then
  run_pg pg_dump --format=custom --no-owner --no-privileges --dbname="$url" --file="/out/$(basename "$dump")"
else
  run_pg pg_dump --format=custom --no-owner --no-privileges --dbname="$url" --file="$dump"
fi

(cd "$out" && sha256sum "$(basename "$dump")" > "$(basename "$dump").sha256")

psql "$url" -At -F ' ' -c "select table_name from information_schema.tables where table_schema = 'public' and table_type = 'BASE TABLE' order by 1" \
  | while read -r table; do
      count="$(psql "$url" -Atqc "select count(*) from public.\"$table\"")"
      echo "$table $count"
    done > "$dump.counts"

echo "backup: $dump ($(stat -c %s "$dump") bytes, $(wc -l < "$dump.counts") tables, server $server_major)"
