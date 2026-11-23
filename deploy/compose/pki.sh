#!/usr/bin/env bash
# Makes a private certificate authority and one certificate per service, for the local stack's mutual-TLS mode
# (`up.sh mtls`). Development only: the keys are world-readable files on this machine, valid for a year, and signed by
# a CA whose key sits beside them. Production certificates come from the platform's PKI, are short-lived, and never
# touch a developer's disk.
#
#   deploy/compose/pki.sh <directory> <service-name>...
#
# Each certificate names its service in a URI subject alternative name, spiffe://forge/svc/<name>, which is where
# pkg/serve reads the caller's identity from. It is valid for 127.0.0.1 and localhost, as a server and as a client.
# An existing CA in the directory is kept and its certificates are renewed, so a restart does not strand a service
# with a certificate from a CA the others no longer trust.
set -euo pipefail

dir="${1:?usage: pki.sh <directory> <service-name>...}"
shift
mkdir -p "$dir"
cd "$dir"

if [ ! -f ca.key ] || [ ! -f ca.crt ]; then
  openssl ecparam -name prime256v1 -genkey -noout -out ca.key 2>/dev/null
  openssl req -x509 -new -key ca.key -sha256 -days 3650 -subj "/CN=forge-local-stack-ca" \
    -addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign" -out ca.crt 2>/dev/null
fi

for name in "$@"; do
  if ! [[ "$name" =~ ^[a-z][a-z0-9-]{1,62}$ ]]; then
    echo "pki: '$name' is not a service name" >&2
    exit 1
  fi
  # Made under temporary names and moved into place, so that a service that is running (and reads a renewed certificate
  # without restarting) never opens a file that is half-written or not yet readable.
  rm -f "$name.key.new" "$name.crt.new"
  openssl ecparam -name prime256v1 -genkey -noout -out "$name.key.new" 2>/dev/null
  openssl req -new -key "$name.key.new" -subj "/CN=$name" -out "$name.csr" 2>/dev/null
  ext="$(mktemp)"
  printf 'subjectAltName=URI:spiffe://forge/svc/%s,DNS:localhost,IP:127.0.0.1\nextendedKeyUsage=serverAuth,clientAuth\nkeyUsage=critical,digitalSignature\nbasicConstraints=critical,CA:FALSE\n' "$name" > "$ext"
  openssl x509 -req -in "$name.csr" -CA ca.crt -CAkey ca.key -CAcreateserial -days 365 -sha256 -extfile "$ext" -out "$name.crt.new" 2>/dev/null
  rm -f "$name.csr" "$ext"
  # The service images run as an unprivileged user that is not the owner of these files.
  chmod 0444 "$name.key.new" "$name.crt.new"
  mv -f "$name.key.new" "$name.key"
  mv -f "$name.crt.new" "$name.crt"
done
chmod 0444 ca.crt
chmod 0400 ca.key
echo "pki: a CA and $# service certificates in $dir"
