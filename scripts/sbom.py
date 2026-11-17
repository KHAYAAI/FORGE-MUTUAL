#!/usr/bin/env python3
"""Writes a software bill of materials for everything Forge ships: every Go module's dependencies and both web apps'
production npm packages, as CycloneDX 1.5 JSON. Run from the repository root: python3 -I scripts/sbom.py [out.json]
Needs the Go toolchain and the apps' package-lock.json files; it reads, it changes nothing."""
import json, os, subprocess, sys, datetime
root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
components, seen = [], set()

def add(name, version, kind, purl, used_by):
    key = (kind, name, version)
    if key in seen:
        return
    seen.add(key)
    components.append({"type": "library", "name": name, "version": version, "purl": purl,
                       "properties": [{"name": "forge:used-by", "value": used_by}]})

# Go: each module in go.work, as built for an image (GOWORK=off), excluding Forge's own modules.
mods = [l.strip() for l in open(os.path.join(root, "go.work")) if l.strip().startswith("./")]
for m in mods:
    d = os.path.join(root, m)
    if not os.path.exists(os.path.join(d, "go.mod")):
        continue
    out = subprocess.run(["go", "list", "-m", "-json", "all"], cwd=d, capture_output=True, text=True,
                         env={**os.environ, "GOWORK": "off", "GOFLAGS": "-mod=mod"})
    if out.returncode != 0:
        sys.exit(f"sbom: go list failed in {m}: {out.stderr.strip()[:300]}")
    dec = json.JSONDecoder(); s = out.stdout; i = 0
    while i < len(s):
        while i < len(s) and s[i].isspace(): i += 1
        if i >= len(s): break
        obj, i = dec.raw_decode(s, i)
        if obj.get("Main") or obj["Path"].startswith("github.com/khayaai/forge-mutual"):
            continue
        v = obj.get("Version", "")
        add(obj["Path"], v, "go", f"pkg:golang/{obj['Path']}@{v}", m[2:])

# npm: production packages of each app.
for app in ("apps/web", "apps/backoffice"):
    lock = json.load(open(os.path.join(root, app, "package-lock.json")))
    for path, p in lock.get("packages", {}).items():
        if not path or p.get("dev") or "version" not in p:
            continue
        name = path.split("node_modules/")[-1]
        add(name, p["version"], "npm", f"pkg:npm/{name}@{p['version']}", app)

components.sort(key=lambda c: (c["purl"].split("/")[0], c["name"], c["version"]))
bom = {"bomFormat": "CycloneDX", "specVersion": "1.5", "version": 1,
       "metadata": {"timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds"),
                    "component": {"type": "application", "name": "forge-mutual"}},
       "components": components}
out = sys.argv[1] if len(sys.argv) > 1 else os.path.join(root, "sbom.cdx.json")
json.dump(bom, open(out, "w"), indent=1)
print(f"sbom: {len(components)} components ({sum(1 for c in components if c['purl'].startswith('pkg:golang'))} Go, "
      f"{sum(1 for c in components if c['purl'].startswith('pkg:npm'))} npm) written to {os.path.relpath(out, root)}")
