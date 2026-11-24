#!/usr/bin/env bash
# Downloads the Prudential Authority's published return forms and completion instructions into .cache/return-forms and
# checks each file against the SHA-256 recorded in docs/regulatory/returns/SOURCES.md. A changed hash means the PA has
# published a new version: re-read it before trusting the line mapping in services/bank/reg-reporting/internal/returns.
# Needs network access to www.resbank.co.za.
set -euo pipefail
out="${1:-.cache/return-forms}"
mkdir -p "$out"
base="https://www.resbank.co.za/content/dam/sarb/publications/prudential-authority"
fetch() { # name sha256 path
  curl -fsSL -m 300 -o "$out/$1" "$base/$3"
  got=$(sha256sum "$out/$1" | cut -d' ' -f1)
  if [ "$got" != "$2" ]; then echo "CHANGED $1: expected $2, got $got" >&2; status=1; else echo "ok $1"; fi
}
status=0
fetch di-annexures.zip 146d62dab990ad45c449bdde3d9f90ffff4c48efcf249130d41504c9944e82b5 \
  "pa-documents-issued-for-consultation/2025/proposed-directive-for-mutual-banks-on-di-returns/Annexures%20to%20the%20proposed%20directive.zip"
fetch di-proposed-directive.pdf e9ddb08cde133c2eb2ab5a1fbe64959cb13789015c974a1cb985db870fb52b70 \
  "pa-documents-issued-for-consultation/2025/proposed-directive-for-mutual-banks-on-di-returns/Proposed%20Directive%20for%20Mutual%20banks%20on%20DI%20Returns.pdf"
fetch mb-directive-1-2025.pdf 8a2513bbdfbe2ae4635ebfd9f02989a430f28079fed6ad4c0d293af37a9ec821 \
  "pa-deposit-takers/mutual-bank-directive/2025/d1-of-2025/D1-2025%20-%20Directive%20to%20Repeal%20DI%20returns%20from%20Regulations.pdf"
fetch ba-directive-1-2025.pdf fe671a0e3c3211cc6e6a48481429e79f5d4fc9bde9b88dc49f781102de16a081 \
  "pa-deposit-takers/banks-directives/2025/d1-of-2025/D1%20-%202025%20-%20Directive%20returns%20to%20be%20submitted%20to%20the%20PA.pdf"
fetch ba-annexures.zip 58e1c9e03c390a6e6ff425012a9befd84b9e74b7be59a7226ae2f14c868a7231 \
  "pa-deposit-takers/banks-directives/2025/d1-of-2025/Returns%20-%20Annexures.zip"
exit $status
