#!/usr/bin/env bash
# Checks, against the running stack started with `up.sh mtls`, that mutual TLS and the caller allow-lists refuse what
# they should. Each line is one claim. The script fails if any claim does not hold.
#
#   deploy/compose/check-mtls.sh
#
# It asks the ledger, because that is where a wrong answer costs money, and one other service to show that the answer is
# not the ledger's alone.
set -uo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
pki="$here/.pki"
ledger="https://127.0.0.1:8070/forge.ledger.v1.LedgerService"
party="https://127.0.0.1:8081/forge.party.v1.PartyService"
fail=0

[ -f "$pki/ca.crt" ] || { echo "no $pki/ca.crt: start the stack with 'deploy/compose/up.sh mtls' first" >&2; exit 2; }

# A second CA, which the stack does not trust, and a certificate from it that claims to be the payments service.
other="$(mktemp -d)"
trap 'rm -rf "$other"' EXIT
"$here/pki.sh" "$other" svc-payments >/dev/null

claim() { # claim "description" expected actual
  if [ "$2" = "$3" ]; then
    echo "ok    $1"
  else
    echo "FAIL  $1 (expected $2, got $3)"
    fail=1
  fi
}

# call IDENTITY URL BODY [extra curl args]: prints the HTTP status, or "refused" when no response came back at all.
call() {
  local id="$1" url="$2" body="$3"
  shift 3
  local code
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 --cacert "$pki/ca.crt" \
    --cert "$pki/$id.crt" --key "$pki/$id.key" -H 'content-type: application/json' "$@" -d "$body" "$url" 2>/dev/null)"
  [ "$code" = "000" ] && code="refused"
  echo "$code"
}
# The Connect error code in the response body, for telling "not allowed" from "not found".
reason() {
  local id="$1" url="$2" body="$3"
  shift 3
  curl -s --max-time 5 --cacert "$pki/ca.crt" --cert "$pki/$id.crt" --key "$pki/$id.key" \
    -H 'content-type: application/json' "$@" -d "$body" "$url" 2>/dev/null | sed -n 's/.*"code":"\([a-z_]*\)".*/\1/p'
}

balances='{"entityCode":"BANK","accountIds":[]}'
list='{"entityCode":"BANK","limit":5}'
transfer='{"idempotencyKey":"mtls-check","businessTxnId":"7d7b1c2e-5f0a-4c3b-9a1e-00000000aa01","entityCode":"BANK","legs":[],"actor":"check"}'

echo "Transport: a caller must present a certificate this stack's CA signed."
code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 --cacert "$pki/ca.crt" -H 'content-type: application/json' -d "$list" "$ledger/ListAccounts" 2>/dev/null)"
[ "$code" = "000" ] && code="refused"
claim "no client certificate: the ledger does not answer" refused "$code"
code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 --cacert "$pki/ca.crt" --cert "$other/svc-payments.crt" --key "$other/svc-payments.key" \
  -H 'content-type: application/json' -d "$list" "$ledger/ListAccounts" 2>/dev/null)"
[ "$code" = "000" ] && code="refused"
claim "a certificate from another CA that names itself svc-payments: the ledger does not answer" refused "$code"
code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 -H 'content-type: application/json' -H 'X-Forge-Caller: svc-payments' -d "$list" "http://127.0.0.1:8070/forge.ledger.v1.LedgerService/ListAccounts" 2>/dev/null)"
[ "$code" = "000" ] && code="refused"
claim "plain HTTP with the development caller header: not served" 400 "$code"

echo "Identity: the caller is the service its certificate names, and nothing a request says changes that."
claim "svc-stack-probe (allowed nothing) lists accounts: permission denied" permission_denied "$(reason svc-stack-probe "$ledger/ListAccounts" "$list")"
claim "svc-stack-probe adds 'X-Forge-Caller: svc-reconciliation', which may list accounts: still permission denied" permission_denied "$(reason svc-stack-probe "$ledger/ListAccounts" "$list" -H 'X-Forge-Caller: svc-reconciliation')"
claim "svc-stack-probe asks the party service for a party: permission denied" permission_denied "$(reason svc-stack-probe "$party/GetParty" '{"partyId":"0fcf8dbb-fd16-401a-98ed-1ebe93c558f3"}')"

echo "Least privilege: a service may call the methods its work needs, and no others."
claim "svc-reconciliation lists accounts: allowed" 200 "$(call svc-reconciliation "$ledger/ListAccounts" "$list")"
claim "svc-reconciliation creates a transfer: permission denied" permission_denied "$(reason svc-reconciliation "$ledger/CreateTransfers" "$transfer")"
claim "svc-consolidated-view posts a pending transfer: permission denied" permission_denied "$(reason svc-consolidated-view "$ledger/PostPending" "$transfer")"
claim "svc-treasury-risk creates accounts: permission denied" permission_denied "$(reason svc-treasury-risk "$ledger/CreateAccounts" "$transfer")"
claim "svc-onboarding creates a transfer: permission denied" permission_denied "$(reason svc-onboarding "$ledger/CreateTransfers" "$transfer")"
claim "svc-payments creates a transfer: allowed through to validation (no legs: invalid argument)" invalid_argument "$(reason svc-payments "$ledger/CreateTransfers" "$transfer")"
claim "svc-payments lists accounts, which it never needs: permission denied" permission_denied "$(reason svc-payments "$ledger/ListAccounts" "$list")"
claim "svc-workspace reads a party directly: permission denied" permission_denied "$(reason svc-workspace "$party/GetParty" '{"partyId":"0fcf8dbb-fd16-401a-98ed-1ebe93c558f3"}')"

echo "The two APIs for people: each serves only its own web app's server, whatever the request claims."
get() { # get IDENTITY URL [extra curl args]: the HTTP status of a GET
  local id="$1" url="$2"
  shift 2
  curl -s -o /dev/null -w '%{http_code}' --max-time 5 --cacert "$pki/ca.crt" --cert "$pki/$id.crt" --key "$pki/$id.key" "$@" "$url" 2>/dev/null
}
party_headers=(-H 'X-Forge-Party: 0fcf8dbb-fd16-401a-98ed-1ebe93c558f3' -H 'X-Forge-Actor: check')
staff_headers=(-H 'X-Forge-Staff: Naledi' -H 'X-Forge-Roles: compliance')
claim "the customer web app's server reads accounts from the customer API: served" 200 "$(get svc-web https://127.0.0.1:8300/v1/accounts "${party_headers[@]}")"
claim "the payments service sends the same request to the customer API: forbidden" 403 "$(get svc-payments https://127.0.0.1:8300/v1/accounts "${party_headers[@]}")"
claim "the customer web app's server asks the staff API for the review queue: forbidden" 403 "$(get svc-web https://127.0.0.1:8500/v1/kyc/reviews "${staff_headers[@]}")"
claim "the staff web app's server asks the staff API for the review queue: served" 200 "$(get svc-backoffice-web https://127.0.0.1:8500/v1/kyc/reviews "${staff_headers[@]}")"

if [ "$fail" = 0 ]; then
  echo "check-mtls: every claim holds"
else
  echo "check-mtls: at least one claim does not hold" >&2
fi
exit "$fail"
