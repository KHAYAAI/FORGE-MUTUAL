#!/usr/bin/env bash
# Starts the local stack. It generates this machine's secrets into deploy/compose/.env (created once, never
# committed), sets the time-stamped settings (the screening list and the FX rate are only valid for a day), builds
# the images, starts everything, and waits until every service answers on its port.
#
#   deploy/compose/up.sh            start (or re-start) the stack
#   deploy/compose/up.sh oidc       also start the development identity provider (tools/devidp)
#   deploy/compose/up.sh mtls       every service speaks TLS, requires a client certificate, and takes the caller's
#                                   identity from it (pki.sh makes a local CA and the certificates in .pki/)
#
# The words can be combined. Images are built with `docker build --network host`, the one form that works on machines
# without a bridge network and does not need a BuildKit entitlement. Build logs go to a temporary directory, named on
# failure.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../.." && pwd)"
cd "$here"
compose=(docker compose -f docker-compose.yml)
oidc=0
mtls=0
for word in "$@"; do
  case "$word" in
    oidc) oidc=1 ;;
    mtls) mtls=1 ;;
    *) echo "up: unknown word '$word' (oidc, mtls)" >&2; exit 2 ;;
  esac
done
[ "$oidc" = 1 ] && compose+=(--profile oidc)

env_file=".env"
touch "$env_file"

# Mutual TLS. Without it the stack is plain HTTP and callers name themselves in a header, which is development only.
# The .pki directory is always there, because every service mounts it.
mkdir -p .pki
sed -i -e '/^STACK_TLS=/d' -e '/^STACK_SCHEME=/d' "$env_file"
scheme=http
if [ "$mtls" = 1 ]; then
  ./pki.sh .pki svc-ledger svc-party svc-consent svc-policy svc-onboarding svc-consolidated-view svc-notifications \
    svc-audit svc-fx svc-rails svc-partner-router svc-payments svc-payouts svc-reconciliation svc-treasury-risk \
    svc-compliance svc-backoffice-api svc-workspace svc-web svc-backoffice-web svc-stack-journeys svc-stack-probe
  printf 'STACK_TLS=1\nSTACK_SCHEME=https\n' >> "$env_file"
  scheme=https
fi

# Secrets: generated once per machine. Existing values are kept, so the stack's data stays readable across restarts.
ensure() { # ensure NAME VALUE_COMMAND
  local name="$1"; shift
  if ! grep -q "^$name=" "$env_file" 2>/dev/null; then
    printf '%s=%s\n' "$name" "$("$@")" >> "$env_file"
  fi
}
b64key() { head -c 32 /dev/urandom | base64 -w0; }
hex32() { head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'; }
ensure PARTY_SEAL_KEY b64key
ensure PARTY_INDEX_KEY b64key
ensure NOTIFICATIONS_KEY b64key
ensure KYC_WEBHOOK_SECRET hex32

# The services read these as files (NAME_FILE), as production requires, never from the environment. .secrets/ is
# written from .env on every start, readable by the services' unprivileged user, and never committed.
mkdir -p .secrets
for name in PARTY_SEAL_KEY PARTY_INDEX_KEY NOTIFICATIONS_KEY KYC_WEBHOOK_SECRET; do
  value="$(grep "^$name=" "$env_file" | head -1 | cut -d= -f2-)"
  rm -f ".secrets/$name.new"
  printf '%s' "$value" > ".secrets/$name.new"
  chmod 0444 ".secrets/$name.new"
  mv -f ".secrets/$name.new" ".secrets/$name"
done

# Time-stamped settings, rewritten on every start.
now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
sed -i -e '/^SCREENING_LIST_AS_OF=/d' -e '/^FX_RATE_AS_OF=/d' "$env_file"
printf 'SCREENING_LIST_AS_OF=%s\nFX_RATE_AS_OF=%s\n' "$now" "$now" >> "$env_file"

# Images: name, module path, command directory under cmd/, and CGO (1 only for the ledger: the TigerBeetle client
# links a native library and its Go files are excluded without cgo).
images=(
  "forge/party-service:dev services/platform/party-service party-service"
  "forge/consent-service:dev services/platform/consent-service consent-service"
  "forge/policy-service:dev services/shared/policy-service policy-service"
  "forge/onboarding:dev services/platform/onboarding onboarding"
  "forge/consolidated-view:dev services/platform/consolidated-view consolidated-view"
  "forge/notifications:dev services/platform/notifications notifications"
  "forge/audit-service:dev services/shared/audit-service audit-service"
  "forge/ledger-gateway-bank:dev services/bank/ledger-gateway-bank ledger-gateway-bank 1"
  "forge/fx-service:dev services/bank/fx-service fx-service"
  "forge/rails-gateway:dev services/bank/rails-gateway rails-gateway"
  "forge/partner-bank-router:dev services/bank/partner-bank-router partner-bank-router"
  "forge/payments-orchestrator:dev services/bank/payments-orchestrator payments-orchestrator"
  "forge/payouts-service:dev services/bank/payouts-service payouts-service"
  "forge/reconciliation:dev services/bank/reconciliation reconciliation"
  "forge/treasury-risk:dev services/bank/treasury-risk treasury-risk"
  "forge/compliance-engine:dev services/shared/compliance-engine compliance-engine"
  "forge/backoffice-api:dev services/shared/backoffice-api backoffice-api"
  "forge/workspace-api:dev services/platform/workspace-api workspace-api"
)
[ "$oidc" = 1 ] && images+=("forge/devidp:dev tools/devidp devidp")

logs="$(mktemp -d)"
build_one() {
  local image module cmd cgo
  read -r image module cmd cgo <<<"$1"
  cgo="${cgo:-0}"
  local log="$logs/$(echo "$image" | tr '/:' '__').log"
  if ! docker build --network host -f "$root/deploy/docker/service.Dockerfile" \
      --build-arg MODULE="$module" --build-arg CMD="$cmd" --build-arg CGO_ENABLED="$cgo" -t "$image" "$root" >"$log" 2>&1; then
    echo "up: build failed for $image; log: $log" >&2
    return 1
  fi
}
export -f build_one
export root logs
printf '%s\n' "${images[@]}" | xargs -P 4 -I{} bash -c 'build_one "$1"' _ {} || exit 1
echo "up: images built"

"${compose[@]}" up -d --no-build --wait --wait-timeout 600 postgres kafka kafka-init temporal tigerbeetle >/dev/null
"${compose[@]}" up -d --no-build --wait-timeout 600 >/dev/null

# Every Go service answers HTTP on its port. A reply of any status means the process is listening and serving. With
# mutual TLS the probe presents its own certificate, svc-stack-probe, which no service allows to call anything: it can
# complete a handshake and nothing more.
probe=(curl -s -o /dev/null -w '%{http_code}' --max-time 2 -X POST)
[ "$mtls" = 1 ] && probe+=(--cacert .pki/ca.crt --cert .pki/svc-stack-probe.crt --key .pki/svc-stack-probe.key)
declare -A ports=(
  [ledger]=8070 [party]=8081 [consent]=8082 [onboarding]=8083 [policy]=8084 [payments]=8085 [rails]=8086
  [partner-router]=8099 [fx]=8100 [payouts]=8200 [recon]=8093 [treasury]=8095 [consolidated]=8096
  [notifications]=8097 [audit]=8094 [compliance]=8400 [backoffice]=8500 [workspace]=8300
)
failed=()
for svc in "${!ports[@]}"; do
  up=0
  for _ in $(seq 1 60); do
    code="$("${probe[@]}" "$scheme://127.0.0.1:${ports[$svc]}/__probe" 2>/dev/null || true)"
    if [ -n "$code" ] && [ "$code" != "000" ]; then up=1; break; fi
    sleep 2
  done
  [ "$up" = 1 ] || failed+=("$svc")
done

if [ "${#failed[@]}" -gt 0 ]; then
  echo "up: not answering after two minutes: ${failed[*]}" >&2
  echo "up: logs: docker compose -f $here/docker-compose.yml logs <service>" >&2
  exit 1
fi
rm -rf "$logs"
echo "up: the stack is running. Customer workspace API: $scheme://127.0.0.1:8300  Back-office API: $scheme://127.0.0.1:8500"
[ "$mtls" = 1 ] && echo "up: mutual TLS is on. Callers need a certificate from .pki/ (see deploy/compose/README.md)."
exit 0
