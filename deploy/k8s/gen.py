#!/usr/bin/env python3
"""Generates deploy/k8s/base from the compose stack and its caller grants, so ports, caller lists and service wiring
cannot drift from what the stack already runs and tests.

    python3 -I deploy/k8s/gen.py            # write deploy/k8s/base
    python3 -I deploy/k8s/gen.py --check    # fail if the files on disk are stale

These manifests are NOT tested against a cluster: this environment has no cluster, kubectl or kustomize. They are
checked by scripts/check-k8s.py (structure, security settings, grants). Every value that depends on a choice nobody has
made yet (hosting, brokers, database hosts, providers) is marked PLACEHOLDER; scripts/check-k8s.py counts them.
"""
import json, os, re, sys
import yaml

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
OUT = os.path.join(ROOT, "deploy/k8s/base")
COMPOSE = os.path.join(ROOT, "deploy/compose/docker-compose.yml")
CALLERS = os.path.join(ROOT, "deploy/compose/config/callers")

NS = "forge"
HEALTH_PORT = 8181
# Dev-only settings the stack uses. Production refuses most of them, and none belongs in a production manifest.
DEV_KEYS = {"FORGE_ENV", "ACQUIRER", "BANK_ALLOW_SIMULATION", "DIDIT_ENVIRONMENT", "KYC_PROVIDER", "LEDGER_STORE",
            "PAYMENTS_ALLOW_SIMULATION", "POLICY_DEV_UNSIGNED", "RAILS_SIMULATION", "BACKOFFICE_AUTH", "WORKSPACE_AUTH",
            "DATABASE_URL", "KAFKA_BROKERS", "OTEL_EXPORTER_OTLP_ENDPOINT", "TB_ADDRESSES", "TEMPORAL_HOST",
            "FX_RATE_ZAR_PER_USD", "FX_RATE_ZAR_PER_EUR", "FX_RATE_ZAR_PER_GBP", "FX_RATE_AS_OF", "TREASURY_FX_RATES",
            "BANK_CONFIG", "POLICY_BUNDLE_DIR", "BACKOFFICE_CLIENTS", "WORKSPACE_CLIENTS", "DIDIT_CALLBACK_URL",
            "SCREENING_LIST_AS_OF"}
# Services that talk to the outside world (providers, partners). Narrow these to the providers' addresses once chosen.
EXTERNAL = {"onboarding", "notifications", "payments", "rails", "partner-router", "acquiring", "compliance", "workspace", "backoffice"}
INGRESS = {"workspace", "backoffice"}  # reached from the customer and staff web apps, through the ingress controller
SECCOMP_EXCEPTION = {"ledger"}  # TigerBeetle's client needs io_uring; see the README for the profile to use


class Dumper(yaml.SafeDumper):
    # No anchors or aliases: every manifest is plain, so any YAML reader (kubectl, kustomize, reviewers) sees the same.
    def ignore_aliases(self, data):
        return True


def _str(dumper, data):
    # Multi-line text (the caller lists) as a literal block, so a reviewer can read it.
    return dumper.represent_scalar("tag:yaml.org,2002:str", data, style="|" if "\n" in data else None)


Dumper.add_representer(str, _str)


def dump(*docs):
    return "---\n".join(yaml.dump(d, Dumper=Dumper, sort_keys=False, default_flow_style=False, width=120) for d in docs)


def load_services():
    d = yaml.safe_load(open(COMPOSE))["services"]
    out = {}
    for name, s in d.items():
        img = str(s.get("image", ""))
        if not img.startswith("forge/") or name == "devidp":
            continue
        env = s.get("environment") or {}
        port = next((int(v[1:]) for v in env.values() if isinstance(v, str) and re.fullmatch(r":\d+", v)), None)
        if port is None:
            continue
        portkey = next(k for k, v in env.items() if isinstance(v, str) and v == f":{port}")
        callers = next((v for k, v in env.items() if k.endswith("_CALLERS_FILE")), None)
        # The identity the service presents is named by its certificate in the stack; callers' lists use the same names.
        ident = re.search(r"svc-[\w-]+", str(env.get("TLS_CERT_FILE", ""))).group(0)
        out[name] = dict(ident=ident, image=img.split("/")[1].split(":")[0], port=port, portkey=portkey, env=env,
                         callers=os.path.basename(callers) if callers else None)
    return out


def by_port(svcs):
    return {s["port"]: n for n, s in svcs.items()}


def labels(name):
    return {"app.kubernetes.io/name": name, "app.kubernetes.io/part-of": "forge"}


def manifests(name, s, svcs):
    ports = by_port(svcs)
    env, vols, mounts = [], [], []
    add = lambda k, v: env.append({"name": k, "value": str(v)})
    add("FORGE_ENV", "production")
    add("HEALTH_ADDR", f":{HEALTH_PORT}")
    add(s["portkey"], f":{s['port']}")
    add("TLS_CERT_FILE", "/tls/tls.crt"); add("TLS_KEY_FILE", "/tls/tls.key"); add("TLS_CA_FILE", "/tls/ca.crt")
    add("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector.forge-observability.svc.cluster.local:4318")
    if "KAFKA_BROKERS" in s["env"]:
        add("KAFKA_BROKERS", "PLACEHOLDER-kafka-bootstrap.forge-data.svc.cluster.local:9093")
    if "DATABASE_URL" in s["env"]:
        add("DATABASE_URL_FILE", "/secrets/DATABASE_URL")
    if "TB_ADDRESSES" in s["env"]:
        add("TB_ADDRESSES", "PLACEHOLDER-tigerbeetle-0.forge-data:3001,tigerbeetle-1.forge-data:3001,tigerbeetle-2.forge-data:3001")
    if "TEMPORAL_HOST" in s["env"]:
        add("TEMPORAL_HOST", "PLACEHOLDER-temporal-frontend.forge-data.svc.cluster.local:7233")
    callees = set()
    for k, v in s["env"].items():
        if k in DEV_KEYS or k.startswith("TLS_") or k.startswith("DEVIDP") or k == s["portkey"]:
            continue
        if k.endswith("_CALLERS_FILE"):
            add(k, "/config/callers.json")
            continue
        if k.endswith("_FILE"):
            add(k, f"/secrets/{os.path.basename(str(v))}")
            continue
        if k in ("DIDIT_API_KEY", "DIDIT_WEBHOOK_SECRET"):
            add(k + "_FILE", f"/secrets/{k}")  # a secret: a file, never the environment
            continue
        m = re.fullmatch(r"\$\{STACK_SCHEME:-http\}://127\.0\.0\.1:(\d+)(.*)", str(v))
        if k.endswith("_URL") and m and int(m.group(1)) in ports:
            callee = ports[int(m.group(1))]
            callees.add(callee)
            add(k, f"https://{callee}.{NS}.svc.cluster.local:{m.group(1)}{m.group(2)}")
            continue
        if k.endswith("_URL") or k.startswith("DIDIT_"):
            add(k, f"PLACEHOLDER-{k.lower()}")
            continue
        add(k, v)
    cfg = None
    if s["callers"]:
        cfg = json.dumps(json.load(open(os.path.join(CALLERS, s["callers"]))), indent=2, sort_keys=True)
        vols.append({"name": "callers", "configMap": {"name": f"{name}-callers"}})
        mounts.append({"name": "callers", "mountPath": "/config", "readOnly": True})
    vols.append({"name": "tls", "secret": {"secretName": f"{name}-tls"}})
    mounts.append({"name": "tls", "mountPath": "/tls", "readOnly": True})
    vols.append({"name": "secrets", "secret": {"secretName": f"{name}-secrets"}})
    mounts.append({"name": "secrets", "mountPath": "/secrets", "readOnly": True})
    vols.append({"name": "tmp", "emptyDir": {"sizeLimit": "64Mi"}})
    mounts.append({"name": "tmp", "mountPath": "/tmp"})
    seccomp = {"type": "Unconfined"} if name in SECCOMP_EXCEPTION else {"type": "RuntimeDefault"}
    container = {
        "name": name, "image": f"registry.PLACEHOLDER/forge/{s['image']}:TAG",
        "ports": [{"name": "https", "containerPort": s["port"]}, {"name": "health", "containerPort": HEALTH_PORT}],
        "env": env,
        "readinessProbe": {"httpGet": {"path": "/readyz", "port": "health"}, "periodSeconds": 5, "failureThreshold": 3},
        "livenessProbe": {"httpGet": {"path": "/livez", "port": "health"}, "periodSeconds": 10, "failureThreshold": 3},
        "startupProbe": {"httpGet": {"path": "/livez", "port": "health"}, "periodSeconds": 2, "failureThreshold": 60},
        "resources": {"requests": {"cpu": "100m", "memory": "128Mi"}, "limits": {"cpu": "1", "memory": "512Mi"}},
        "securityContext": {"allowPrivilegeEscalation": False, "readOnlyRootFilesystem": True, "runAsNonRoot": True,
                            "capabilities": {"drop": ["ALL"]}, "seccompProfile": seccomp},
        "volumeMounts": mounts,
    }
    lab = labels(name)
    docs = [
        {"apiVersion": "v1", "kind": "ServiceAccount", "metadata": {"name": name, "namespace": NS, "labels": lab}, "automountServiceAccountToken": False},
    ]
    if cfg:
        docs.append({"apiVersion": "v1", "kind": "ConfigMap", "metadata": {"name": f"{name}-callers", "namespace": NS, "labels": lab},
                     "data": {"callers.json": cfg + "\n"}})
    docs += [
        {"apiVersion": "cert-manager.io/v1", "kind": "Certificate", "metadata": {"name": f"{name}-tls", "namespace": NS, "labels": lab},
         "spec": {"secretName": f"{name}-tls", "duration": "24h", "renewBefore": "8h",
                  "privateKey": {"algorithm": "ECDSA", "size": 256, "rotationPolicy": "Always"},
                  "uris": [f"spiffe://forge/svc/{s['ident']}"], "dnsNames": [f"{name}.{NS}.svc.cluster.local"],
                  "usages": ["server auth", "client auth", "digital signature"],
                  "issuerRef": {"name": "forge-service-ca", "kind": "ClusterIssuer", "group": "cert-manager.io"}}},
        {"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {"name": name, "namespace": NS, "labels": lab},
         "spec": {"replicas": 2, "revisionHistoryLimit": 5, "selector": {"matchLabels": lab},
                  "strategy": {"type": "RollingUpdate", "rollingUpdate": {"maxUnavailable": 0, "maxSurge": 1}},
                  "template": {"metadata": {"labels": lab},
                               "spec": {"serviceAccountName": name, "automountServiceAccountToken": False,
                                        "securityContext": {"runAsNonRoot": True, "runAsUser": 65532, "runAsGroup": 65532, "fsGroup": 65532},
                                        "terminationGracePeriodSeconds": 30,
                                        "topologySpreadConstraints": [{"maxSkew": 1, "topologyKey": "topology.kubernetes.io/zone",
                                                                       "whenUnsatisfiable": "ScheduleAnyway", "labelSelector": {"matchLabels": lab}}],
                                        "containers": [container], "volumes": vols}}}},
        {"apiVersion": "v1", "kind": "Service", "metadata": {"name": name, "namespace": NS, "labels": lab},
         "spec": {"selector": lab, "ports": [{"name": "https", "port": s["port"], "targetPort": "https"}]}},
        {"apiVersion": "policy/v1", "kind": "PodDisruptionBudget", "metadata": {"name": name, "namespace": NS, "labels": lab},
         "spec": {"minAvailable": 1, "selector": {"matchLabels": lab}}},
    ]
    # Who may call this service: exactly the services its caller list names (the same list the service enforces).
    callers = []
    if s["callers"]:
        callers = [c for c in json.load(open(os.path.join(CALLERS, s["callers"])))]
    idents = {v["ident"]: n for n, v in svcs.items()}
    # Identities with no deployment (the stack's journey tests, the Fineract bridge) get no network path.
    frm = [{"podSelector": {"matchLabels": {"app.kubernetes.io/name": idents[c]}}} for c in sorted(callers) if c in idents]
    if name in INGRESS:
        frm.append({"namespaceSelector": {"matchLabels": {"forge.io/ingress": "true"}}})
    rules = [{"from": frm, "ports": [{"protocol": "TCP", "port": s["port"]}]}] if frm else []
    egress = [
        {"to": [{"namespaceSelector": {}, "podSelector": {"matchLabels": {"k8s-app": "kube-dns"}}}], "ports": [{"protocol": "UDP", "port": 53}, {"protocol": "TCP", "port": 53}]},
        {"to": [{"namespaceSelector": {"matchLabels": {"kubernetes.io/metadata.name": "forge-observability"}}}], "ports": [{"protocol": "TCP", "port": 4318}]},
    ]
    data = [{"protocol": "TCP", "port": p} for p in (5432, 9093, 7233, 3001)]
    egress.append({"to": [{"namespaceSelector": {"matchLabels": {"kubernetes.io/metadata.name": "forge-data"}}}], "ports": data})
    if callees:
        egress.append({"to": [{"podSelector": {"matchLabels": {"app.kubernetes.io/name": c}}} for c in sorted(callees)],
                       "ports": [{"protocol": "TCP", "port": svcs[c]["port"]} for c in sorted(callees)]})
    if name in EXTERNAL:
        # PLACEHOLDER: narrow to the providers' addresses once they are chosen.
        egress.append({"to": [{"ipBlock": {"cidr": "0.0.0.0/0", "except": ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]}}], "ports": [{"protocol": "TCP", "port": 443}]})
    docs.append({"apiVersion": "networking.k8s.io/v1", "kind": "NetworkPolicy", "metadata": {"name": name, "namespace": NS, "labels": lab},
                 "spec": {"podSelector": {"matchLabels": lab}, "policyTypes": ["Ingress", "Egress"], "ingress": rules, "egress": egress}})
    return docs


def shared(names):
    ns = {"apiVersion": "v1", "kind": "Namespace", "metadata": {"name": NS, "labels": {
        "pod-security.kubernetes.io/enforce": "restricted", "pod-security.kubernetes.io/audit": "restricted", "pod-security.kubernetes.io/warn": "restricted"}}}
    deny = {"apiVersion": "networking.k8s.io/v1", "kind": "NetworkPolicy", "metadata": {"name": "default-deny", "namespace": NS},
            "spec": {"podSelector": {}, "policyTypes": ["Ingress", "Egress"]}}
    return {"namespace.yaml": dump(ns, deny)}


def build():
    svcs = load_services()
    files = shared(svcs)
    for n, s in sorted(svcs.items()):
        files[f"{n}.yaml"] = dump(*manifests(n, s, svcs))
    kust = {"apiVersion": "kustomize.config.k8s.io/v1beta1", "kind": "Kustomization", "resources": sorted(files)}
    files["kustomization.yaml"] = dump(kust)
    return files


def overlay():
    imgs = sorted({s["image"] for s in load_services().values()})
    k = {"apiVersion": "kustomize.config.k8s.io/v1beta1", "kind": "Kustomization", "resources": ["../../base"],
         "images": [{"name": f"registry.PLACEHOLDER/forge/{i}", "newName": f"registry.PLACEHOLDER/forge/{i}", "newTag": "TAG"} for i in imgs]}
    header = ("# Production overlay: pins every image. CI replaces the registry and tag from the release; nothing else\n"
              "# should change here. PLACEHOLDER values in ../../base wait on the decisions listed in ../../README.md.\n")
    return header + dump(k)


def main():
    files = build()
    ov = os.path.join(ROOT, "deploy/k8s/overlays/production/kustomization.yaml")
    if "--check" in sys.argv and (not os.path.exists(ov) or open(ov).read() != overlay()):
        print("FAIL stale: overlays/production/kustomization.yaml\nrun: python3 -I deploy/k8s/gen.py")
        sys.exit(1)
    if "--check" not in sys.argv:
        os.makedirs(os.path.dirname(ov), exist_ok=True)
        open(ov, "w").write(overlay())
    if "--check" in sys.argv:
        bad = [n for n, t in files.items() if not os.path.exists(os.path.join(OUT, n)) or open(os.path.join(OUT, n)).read() != t]
        extra = [f for f in os.listdir(OUT) if f.endswith(".yaml") and f not in files] if os.path.isdir(OUT) else []
        for n in bad + extra:
            print("FAIL stale or unexpected:", n)
        if bad or extra:
            print("run: python3 -I deploy/k8s/gen.py")
            sys.exit(1)
        print(f"k8s manifests up to date: {len(files)} files")
        return
    os.makedirs(OUT, exist_ok=True)
    for f in os.listdir(OUT):
        if f.endswith(".yaml") and f not in files:
            os.remove(os.path.join(OUT, f))
    for n, t in files.items():
        open(os.path.join(OUT, n), "w").write(t)
    print(f"wrote {len(files)} files to {OUT}")


if __name__ == "__main__":
    main()
