#!/usr/bin/env bash
# Starts or stops a local TigerBeetle for tests, in Docker. Development only: it uses --development, which relaxes the
# direct-I/O requirement and shrinks the caches. A production cluster is six replicas on dedicated NVMe hosts across
# sites, without that flag (services/bank/ledger-gateway-bank/SPEC.md section 8).
#
#   scripts/tigerbeetle.sh single up|down     one replica on 127.0.0.1:3033 (what `make ci-db` uses)
#   scripts/tigerbeetle.sh cluster up|down    three replicas on 127.0.0.1:3041-3043 (what the failure test kills)
#   scripts/tigerbeetle.sh env single|cluster prints the variables the tests read
#
# The server must be the same version as the Go client in services/bank/ledger-gateway-bank/go.mod, or newer: a client
# that is newer than its cluster is refused. TB_IMAGE overrides the image.
#
# Containers run with --network host (this also works on machines without a bridge network), seccomp=unconfined
# (TigerBeetle uses io_uring, which Docker's default profile blocks) and IPC_LOCK (it locks its memory).
set -euo pipefail

image="${TB_IMAGE:-ghcr.io/tigerbeetle/tigerbeetle:0.16.60}"
single_name="forge-tb"
single_addr="127.0.0.1:3033"
single_cluster=1
cluster_prefix="forge-tb3"
cluster_addrs="127.0.0.1:3041,127.0.0.1:3042,127.0.0.1:3043"
cluster_id=7

run_replica() { # name volume cluster replica count addresses
  local name="$1" volume="$2" cluster="$3" replica="$4" count="$5" addrs="$6"
  local file="/data/${cluster}_${replica}.tigerbeetle"
  docker rm -f "$name" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
  docker run --rm --security-opt seccomp=unconfined -v "$volume:/data" "$image" \
    format "--cluster=$cluster" "--replica=$replica" "--replica-count=$count" --development "$file" >/dev/null 2>&1
  docker run -d --name "$name" --network host --security-opt seccomp=unconfined --cap-add IPC_LOCK \
    -v "$volume:/data" "$image" start "--addresses=$addrs" --development "$file" >/dev/null
}

wait_port() { # host:port
  local host="${1%:*}" port="${1#*:}"
  for _ in $(seq 1 60); do
    if (echo > "/dev/tcp/$host/$port") 2>/dev/null; then return 0; fi
    sleep 0.5
  done
  echo "tigerbeetle did not start listening on $1" >&2
  return 1
}

print_env() {
  case "$1" in
    single)
      echo "export TB_ADDRESSES='$single_addr'"
      echo "export TB_CLUSTER_ID='$single_cluster'"
      ;;
    cluster)
      echo "export TB_ADDRESSES='$cluster_addrs'"
      echo "export TB_CLUSTER_ID='$cluster_id'"
      echo "export TB_CLUSTER_CONTAINERS='$cluster_prefix-0,$cluster_prefix-1,$cluster_prefix-2'"
      ;;
    *) echo "usage: $0 env single|cluster" >&2; exit 2 ;;
  esac
}

case "${1:-} ${2:-}" in
  "single up")
    run_replica "$single_name" "$single_name-data" "$single_cluster" 0 1 "$single_addr"
    wait_port "$single_addr"
    echo "tigerbeetle: one replica on $single_addr (cluster $single_cluster)"
    ;;
  "single down")
    docker rm -f "$single_name" >/dev/null 2>&1 || true
    docker volume rm "$single_name-data" >/dev/null 2>&1 || true
    ;;
  "cluster up")
    for r in 0 1 2; do
      run_replica "$cluster_prefix-$r" "$cluster_prefix-$r" "$cluster_id" "$r" 3 "$cluster_addrs"
    done
    IFS=, read -ra addrs <<< "$cluster_addrs"
    for a in "${addrs[@]}"; do wait_port "$a"; done
    echo "tigerbeetle: three replicas on $cluster_addrs (cluster $cluster_id)"
    ;;
  "cluster down")
    for r in 0 1 2; do
      docker rm -f "$cluster_prefix-$r" >/dev/null 2>&1 || true
      docker volume rm "$cluster_prefix-$r" >/dev/null 2>&1 || true
    done
    ;;
  "env single" | "env cluster")
    print_env "$2"
    ;;
  *)
    echo "usage: $0 single up|down | cluster up|down | env single|cluster" >&2
    exit 2
    ;;
esac
