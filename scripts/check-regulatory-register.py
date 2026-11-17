#!/usr/bin/env python3
"""Check docs/regulatory/register.csv: columns, allowed values, unique ids, and that every evidence path exists.
Run: python3 -I scripts/check-regulatory-register.py   (from the repository root)"""
import csv, os, sys
HEAD = ["id","source","ref","requirement","route","type","status","evidence","gap_or_next_step","read_basis"]
ROUTE = {"Both","Bank","Mutual","Sponsor-flowdown"}
TYPE = {"Code","Document","External","Decision"}
STATUS = {"Evidenced","Partial","Open","External"}
BASIS = {"Full","Excerpt","Secondary","Not read"}
root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
rows = list(csv.reader(open(os.path.join(root,"docs/regulatory/register.csv"), newline="")))
errs = []
if rows[0] != HEAD: errs.append("header mismatch")
seen = set()
for i, r in enumerate(rows[1:], 2):
    if len(r) != len(HEAD): errs.append(f"line {i}: {len(r)} columns"); continue
    d = dict(zip(HEAD, r))
    if d["id"] in seen: errs.append(f"line {i}: duplicate id {d['id']}")
    seen.add(d["id"])
    for k, allowed in (("route",ROUTE),("type",TYPE),("status",STATUS),("read_basis",BASIS)):
        if d[k] not in allowed: errs.append(f"{d['id']}: bad {k} {d[k]!r}")
    for p in filter(None, (x.strip() for x in d["evidence"].split(";"))):
        if not os.path.exists(os.path.join(root, p)): errs.append(f"{d['id']}: missing evidence path {p}")
    if d["status"] == "Evidenced" and not d["evidence"]: errs.append(f"{d['id']}: Evidenced needs evidence")
print(f"{len(seen)} requirements checked, {len(errs)} problems")
for e in errs: print(" -", e)
sys.exit(1 if errs else 0)
