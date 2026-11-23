# Kubernetes manifests (draft, never applied to a cluster)

> **Status.** Generated, checked on paper, **not run**: this environment has no cluster, `kubectl` or `kustomize`, and
> no hosting provider is chosen (ADR-0002). Treat these as a starting point for the infrastructure work, not as
> something that has been deployed. They also do not build the images: see "What is missing".

```sh
python3 -I deploy/k8s/gen.py            # regenerate deploy/k8s/base from the compose stack and its caller grants
python3 -I scripts/check-k8s.py --selftest   # structure, hardening, grants; runs in `make test-regulatory`
```

## What is here

21 Go services, each with: a ServiceAccount, a Deployment (2 replicas, rolling update with no unavailable pods), a
Service, a PodDisruptionBudget, a cert-manager Certificate, a NetworkPolicy, and its caller list as a ConfigMap.

| Property | How |
|---|---|
| Mutual TLS | Each service gets a 24-hour certificate naming `spiffe://forge/svc/<identity>`, renewed at 16 hours, from a `forge-service-ca` ClusterIssuer; the services reload it without a restart (`pkg/serve`). |
| Least privilege between services | The ConfigMap is the same caller list the stack is tested with (`deploy/compose/config/callers`); the network policy lets in only the services that list names. |
| Hardened pods | Non-root, read-only filesystem, no privilege escalation, all capabilities dropped, default seccomp, no service-account token, restricted pod security standard on the namespace, default-deny network policy. |
| Secrets | Mounted as files (`NAME_FILE`), never as environment values; production refuses the plain environment (`pkg/serve/secrets.go`). |
| Probes | `/readyz`, `/livez` on port 8181, plain HTTP, serving nothing else (`HEALTH_ADDR`). Readiness turns off before shutdown so traffic drains. |
| No dev switches | The generator drops every simulator and development switch; the checker fails if one appears. |

## What is missing, and who decides

Each is a `PLACEHOLDER` in the manifests (46 today). None can be filled until the decision is made.

1. **Where it runs** (ADR-0002): cloud account and region, cluster, registry, ingress, storage classes.
2. **The data layer.** Not in these manifests, because it should be managed or run by an operator, not hand-written:
   Postgres (TLS, per-service databases, backups), Kafka (TLS and SASL, three or more brokers), Temporal, and
   TigerBeetle (six replicas on dedicated hosts, `--development` off).
3. **Secrets delivery.** Each service expects a Secret `<name>-secrets` holding its `*_FILE` secrets and
   `DATABASE_URL`. Fill them from a secret store (External Secrets Operator, Vault, or the cloud's driver).
4. **A certificate authority.** The `forge-service-ca` ClusterIssuer, with the CA bundle that goes in each
   `<name>-tls` secret as `ca.crt`.
5. **Provider addresses.** Services that call outside (onboarding, payments, rails, partner-router, acquiring,
   notifications, compliance) have a 443-to-the-internet egress rule; narrow it to the providers' addresses once chosen.
6. **The ledger's seccomp profile.** TigerBeetle's client needs `io_uring`; the ledger runs `Unconfined`, the only
   exception the checker allows. Replace it with a custom profile that permits `io_uring_setup`, `io_uring_enter` and
   `io_uring_register` only.
7. **Images.** `deploy/docker/service.Dockerfile` builds every Go service (the stack builds them all); they still need
   pushing to a registry, and the two web apps (`apps/web`, `apps/backoffice`) need Deployments of their own.
8. **A rate source.** The FX service reads static rates in the stack; production needs a market-data feed
   (VERIFY, `docs/QUESTIONS.md`).
9. **Observability backend.** Services export traces to `otel-collector.forge-observability:4318`; the collector, a
   tracing and metrics backend, dashboards and alerts (`deploy/monitoring/alerts.csv`) are not set up, nor an on-call rota.

## First steps once hosting is chosen

1. Create the cluster and namespaces `forge`, `forge-data`, `forge-observability` and a namespace for the ingress
   controller labelled `forge.io/ingress=true`.
2. Install cert-manager, create the `forge-service-ca` issuer, and install the secret store integration.
3. Stand up Postgres, Kafka, Temporal and TigerBeetle in `forge-data`; run `scripts/pitr-check.sh` against the
   production Postgres.
4. Fill the placeholders, apply `deploy/k8s/overlays/production`, and run `make test-stack-mtls` against it.
