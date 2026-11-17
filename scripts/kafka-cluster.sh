#!/usr/bin/env bash
# Starts or stops three Kafka 3.9 brokers in KRaft mode, each also a controller, for the cluster test
# (pkg/outbox/kafka_cluster_test.go). Topics default to three replicas and two in-sync replicas, the production setting:
# a write acknowledged with acks=all is on at least two brokers, and the cluster keeps accepting writes with one broker
# down, and refuses them with two down.
#
#   scripts/kafka-cluster.sh up|down|env
set -euo pipefail
image="${KAFKA_IMAGE:-apache/kafka:3.9.0}"
voters="1@127.0.0.1:9391,2@127.0.0.1:9392,3@127.0.0.1:9393"
case "${1:-}" in
  up)
    cluster_id="$(docker run --rm --entrypoint /opt/kafka/bin/kafka-storage.sh "$image" random-uuid)"
    for n in 1 2 3; do
      docker rm -f "forge-kafka3-$n" >/dev/null 2>&1 || true
      docker run -d --name "forge-kafka3-$n" --network host \
        -e KAFKA_NODE_ID="$n" -e KAFKA_PROCESS_ROLES=broker,controller -e CLUSTER_ID="$cluster_id" \
        -e KAFKA_LISTENERS="PLAINTEXT://127.0.0.1:929$n,CONTROLLER://127.0.0.1:939$n" \
        -e KAFKA_ADVERTISED_LISTENERS="PLAINTEXT://127.0.0.1:929$n" \
        -e KAFKA_CONTROLLER_LISTENER_NAMES=CONTROLLER -e KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT \
        -e KAFKA_CONTROLLER_QUORUM_VOTERS="$voters" -e KAFKA_INTER_BROKER_LISTENER_NAME=PLAINTEXT \
        -e KAFKA_DEFAULT_REPLICATION_FACTOR=3 -e KAFKA_MIN_INSYNC_REPLICAS=2 \
        -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=3 -e KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=3 \
        -e KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=2 -e KAFKA_AUTO_CREATE_TOPICS_ENABLE=false \
        -e KAFKA_UNCLEAN_LEADER_ELECTION_ENABLE=false -e KAFKA_HEAP_OPTS="-Xmx512m -Xms512m" \
        "$image" >/dev/null
    done
    for n in 1 2 3; do
      for _ in $(seq 1 60); do (echo > "/dev/tcp/127.0.0.1/929$n") 2>/dev/null && break; sleep 1; done
    done
    echo "kafka-cluster: three brokers on 127.0.0.1:9291-9293"
    ;;
  down)
    for n in 1 2 3; do docker rm -f "forge-kafka3-$n" >/dev/null 2>&1 || true; done
    ;;
  env)
    echo "export KAFKA_CLUSTER_BROKERS='127.0.0.1:9291,127.0.0.1:9292,127.0.0.1:9293'"
    echo "export KAFKA_CLUSTER_CONTAINERS='forge-kafka3-1,forge-kafka3-2,forge-kafka3-3'"
    ;;
  *) echo "usage: $0 up|down|env" >&2; exit 2 ;;
esac
