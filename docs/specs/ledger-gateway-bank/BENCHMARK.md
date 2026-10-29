# Benchmark: ledger-gateway-bank (P0-04)

Scope: the in-memory reference model (`internal/ledger.MemStore`), not TigerBeetle and not the
gateway over HTTP. The numbers bound the cost of the rule logic only. They are not a throughput
claim for the bank cluster. The cluster target (10k transfers per second, ARCHITECTURE.md P1-01) is
still VERIFY-TB.

## Command

```
cd services/bank/ledger-gateway-bank
go test -run xxx -bench BenchmarkMemStoreTransfers -benchtime 2s ./internal/ledger/
```

## Result

Environment: linux/amd64, 4 vCPU, Go 1.24.7, shared cloud container.

| Benchmark | ns/op (one batch of 100) | transfers/s | B/op | allocs/op |
|---|---|---|---|---|
| BenchmarkMemStoreTransfers | 137,218 | ~728,800 | 41,257 | 1 |

## What this does not show

- Durable throughput: TigerBeetle replication and disk are not in the path.
- Postgres cost: idempotency and outbox writes are one transaction per request, measured separately if needed.
- Network: gRPC/Connect HTTP round trips are not included.
