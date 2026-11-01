#!/usr/bin/env bash
# Creates the throwaway databases that the database-backed test suites need, then writes .test-dbs.env, which
# `make ci-db` sources. Every database named here is DROPPED and recreated. Never point FORGE_PG_HOST_DSN at a server
# that holds data you care about.
#
#   FORGE_PG_HOST_DSN   server URI without a database, default the local compose Postgres
#   FORGE_KAFKA_BROKERS broker list for the outbox suite, default localhost:9092
set -euo pipefail

host="${FORGE_PG_HOST_DSN:-postgres://postgres:forge-local-only@127.0.0.1:5432}"
brokers="${FORGE_KAFKA_BROKERS:-localhost:9092}"
root="$(cd "$(dirname "$0")/.." && pwd)"
out="$root/.test-dbs.env"

databases=(
  ledger_pg_test recon_test payments_test inbound_test audit_test treasury_test
  compliance_test policy_test party_test consent_test outbox_test fx_test payouts_test
)

for db in "${databases[@]}"; do
  psql "$host/postgres?sslmode=disable" -v ON_ERROR_STOP=1 -q \
    -c "drop database if exists $db with (force)" -c "create database $db" >/dev/null
done

{
  echo "# Written by scripts/test-databases.sh. Throwaway; do not commit."
  echo "export LEDGER_TEST_DSN='$host/ledger_pg_test?sslmode=disable'"
  echo "export RECON_TEST_DSN='$host/recon_test?sslmode=disable'"
  echo "export PAYMENTS_TEST_PG='$host/payments_test?sslmode=disable'"
  echo "export INBOUND_TEST_DSN='$host/inbound_test?sslmode=disable'"
  echo "export AUDIT_TEST_DSN='$host/audit_test?sslmode=disable'"
  echo "export TREASURY_TEST_DSN='$host/treasury_test?sslmode=disable'"
  echo "export COMPLIANCE_TEST_DSN='$host/compliance_test?sslmode=disable'"
  echo "export POLICY_TEST_DSN='$host/policy_test?sslmode=disable'"
  echo "export PARTY_TEST_DSN='$host/party_test?sslmode=disable'"
  echo "export CONSENT_TEST_DSN='$host/consent_test?sslmode=disable'"
  echo "export ONBOARDING_TEST_PG='$host/postgres?sslmode=disable'"
  echo "export OUTBOX_TEST_DSN='$host/outbox_test?sslmode=disable'"
  echo "export FX_TEST_DSN='$host/fx_test?sslmode=disable'"
  echo "export PAYOUTS_TEST_DSN='$host/payouts_test?sslmode=disable'"
  echo "export KAFKA_TEST_BROKERS='$brokers'"
} > "$out"
echo "wrote $out for ${#databases[@]} databases and broker $brokers"
