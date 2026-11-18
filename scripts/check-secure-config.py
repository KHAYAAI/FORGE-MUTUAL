#!/usr/bin/env python3
"""Checks the secure-configuration standard (docs/regulatory/policies/secure-configuration-standard.md) where a
machine can: container users and base images, pinned third-party images, no privileged containers, seccomp relaxed
only where documented, TLS 1.2 or later, every service serving through pkg/serve, and the web apps' security headers.
Run from the repository root: python3 -I scripts/check-secure-config.py"""
import glob, os, re, sys
root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
read = lambda p: open(os.path.join(root, p)).read()
errs, checks = [], 0

def check(ok, msg):
    global checks
    checks += 1
    if not ok:
        errs.append(msg)

# 1. Service images: distroless, non-root.
df = read("deploy/docker/service.Dockerfile")
check("distroless" in df, "service image is not distroless")
check(re.search(r"^USER\s+nonroot", df, re.M) is not None, "service image does not run as nonroot")

# 2-4. Compose: pinned third-party images, nothing privileged, seccomp relaxed only for TigerBeetle and its client.
compose = read("deploy/compose/docker-compose.yml")
for img in re.findall(r"^\s*image:\s*(\S+)", compose, re.M):
    if img.startswith("forge/"):
        continue
    img = re.sub(r"^\$\{[A-Z_]+:-(.+)\}$", r"\1", img)
    check(":" in img and not img.endswith(":latest"), f"third-party image not pinned to a version: {img}")
check("privileged: true" not in compose, "a container runs privileged")
SECCOMP_ALLOWED = {"tigerbeetle", "tigerbeetle-format", "ledger"}
service = None
for line in compose.splitlines():
    m = re.match(r"^  ([a-z0-9-]+):\s*$", line)
    if m:
        service = m.group(1)
    if "seccomp=unconfined" in line:
        check(service in SECCOMP_ALLOWED, f"seccomp relaxed for {service}, which is not on the documented list")

# 5. TLS: never below 1.2.
for p in ("pkg/serve/serve.go", "pkg/serve/outbound.go"):
    src = read(p)
    check("MinVersion: tls.VersionTLS12" in src or "MinVersion: tls.VersionTLS13" in src, f"{p}: no TLS 1.2 minimum")
    check("InsecureSkipVerify: true" not in src, f"{p}: certificate verification disabled")

# 6. Every service serves through pkg/serve, which applies TLS, mutual TLS and the production refusals.
for main in sorted(glob.glob(os.path.join(root, "services/*/*/cmd/*/main.go"))):
    src = open(main).read()
    rel = os.path.relpath(main, root)
    if "ListenAndServe" in src and "serve.New" not in src:
        errs.append(f"{rel}: listens without pkg/serve")
    checks += 1

# 7. Web apps: a nonce Content-Security-Policy and the other security headers.
for app in ("apps/web", "apps/backoffice"):
    proxy = read(f"{app}/proxy.ts")
    check("nonce" in proxy and "Content-Security-Policy" in proxy, f"{app}: no nonce Content-Security-Policy")
    conf = read(f"{app}/next.config.ts")
    for h in ("Strict-Transport-Security", "X-Content-Type-Options", "Referrer-Policy"):
        check(h in conf or h in proxy, f"{app}: missing {h}")

print(f"{checks} secure-configuration checks, {len(errs)} problems")
for e in errs:
    print(" -", e)
sys.exit(1 if errs else 0)
