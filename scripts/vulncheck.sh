#!/usr/bin/env bash
# Checks every Go module in the repository against the Go vulnerability database, and both web apps' production
# dependencies against npm's advisory database. It fails if any module CALLS vulnerable code (govulncheck follows the
# calls, so a vulnerability in a dependency this code never reaches is reported by it but does not fail the check), or
# if either app has a known vulnerability in what it ships.
#
#   scripts/vulncheck.sh           every module and both apps
#   scripts/vulncheck.sh go        Go modules only
#   scripts/vulncheck.sh npm       the web apps only
#
# Needs govulncheck (go install golang.org/x/vuln/cmd/govulncheck@latest), and reaches vuln.go.dev and
# registry.npmjs.org. A machine that cannot reach them cannot run this check, and must not pretend it passed.
set -uo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
what="${1:-all}"
fail=0

if [ "$what" = all ] || [ "$what" = go ]; then
  vc="$(command -v govulncheck || true)"
  [ -n "$vc" ] || vc="$(go env GOPATH)/bin/govulncheck"
  if [ ! -x "$vc" ]; then
    echo "vulncheck: govulncheck is not installed: go install golang.org/x/vuln/cmd/govulncheck@latest" >&2
    exit 2
  fi
  # Each module is checked on its own, as it is built and shipped, not through the workspace.
  mapfile -t modules < <(find . -name go.mod -not -path '*/node_modules/*' -exec dirname {} \; | sort)
  for m in "${modules[@]}"; do
    out="$(cd "$m" && GOWORK=off GOFLAGS=-mod=mod "$vc" -tags tbintegration,temporaldev ./... 2>&1)"
    code=$?
    if [ "$code" = 0 ]; then
      echo "ok    $m"
    elif echo "$out" | grep -q "^Vulnerability #"; then
      echo "FAIL  $m calls vulnerable code:"
      echo "$out" | grep -E "^Vulnerability #|^    Found in:|^    Fixed in:|^    More info:|^      #1:" | sed 's/^/        /'
      fail=1
    else
      echo "FAIL  $m could not be checked:"
      echo "$out" | tail -5 | sed 's/^/        /'
      fail=1
    fi
  done
fi

if [ "$what" = all ] || [ "$what" = npm ]; then
  for app in apps/web apps/backoffice; do
    out="$(cd "$app" && npm audit --omit=dev 2>&1)"
    if echo "$out" | grep -q "found 0 vulnerabilities"; then
      echo "ok    $app (production dependencies)"
    else
      echo "FAIL  $app has a known vulnerability in what it ships:"
      echo "$out" | head -20 | sed 's/^/        /'
      fail=1
    fi
  done
fi

if [ "$fail" = 0 ]; then
  echo "vulncheck: nothing that is shipped calls or contains a known vulnerability"
fi
exit "$fail"
