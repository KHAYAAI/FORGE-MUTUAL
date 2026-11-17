#!/usr/bin/env python3
"""Check deploy/monitoring/alerts.csv: values, runbook headings and register ids.
Run: python3 -I scripts/check-monitoring.py   (from the repository root)"""
import csv, os, re, sys
H = ["id","what_it_catches","signal","signal_status","severity","page","register_ids"]
root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
rows = list(csv.reader(open(os.path.join(root, "deploy/monitoring/alerts.csv"), newline="")))
reg = {r["id"] for r in csv.DictReader(open(os.path.join(root, "docs/regulatory/register.csv"), newline=""))}
heads = set(re.findall(r"^## ([a-z0-9-]+)\s*$", open(os.path.join(root, "docs/runbooks/README.md")).read(), re.M))
errs, seen = [], set()
if rows[0] != H: errs.append("header mismatch")
for i, r in enumerate(rows[1:], 2):
    if len(r) != len(H): errs.append(f"line {i}: {len(r)} columns"); continue
    d = dict(zip(H, r))
    if d["id"] in seen: errs.append(f"{d['id']}: duplicate")
    seen.add(d["id"])
    if d["signal_status"] not in {"exists","needs-wiring","missing"}: errs.append(f"{d['id']}: bad signal_status")
    if d["severity"] not in {"critical","high","medium","low"}: errs.append(f"{d['id']}: bad severity")
    if d["page"] not in {"yes","no"}: errs.append(f"{d['id']}: bad page")
    if d["id"] not in heads: errs.append(f"{d['id']}: no runbook heading")
    for x in filter(None, d["register_ids"].split(";")):
        if x not in reg: errs.append(f"{d['id']}: unknown register id {x}")
for h in sorted(heads - seen): errs.append(f"runbook for an alert that is not in the catalogue: {h}")
print(f"{len(seen)} alerts checked, {len(errs)} problems")
for e in errs: print(" -", e)
sys.exit(1 if errs else 0)
