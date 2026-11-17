#!/usr/bin/env python3
"""Check docs/regulatory/assets.csv: fields, allowed values, repo paths, review dates, and completeness.
Completeness: every service directory under services/*/* must appear as a 'service' row, so a new service cannot be
added without being classified. Joint Standard 2 of 2024, paragraphs 7.1.1(d) and 7.1.2: the inventory states location,
ownership and responsibilities and is reviewed at least every two years. This repository sets six months.
Run: python3 -I scripts/check-asset-register.py   (from the repository root)"""
import csv, datetime, os, sys
H = ["id","name","kind","description","classification","data_held","location","owner","criticality","third_party","repo_path","last_reviewed","next_review"]
KIND = {"service","application","datastore","backup","secret","source","provider"}
CLASS = {"Public","Internal","Confidential","Restricted"}
CRIT = {"Critical","High","Medium","Low"}
root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
today = datetime.date.fromisoformat(os.environ.get("CHECK_DATE", datetime.date.today().isoformat()))
rows = list(csv.reader(open(os.path.join(root, "docs/regulatory/assets.csv"), newline="")))
errs = []
if rows[0] != H: errs.append("header mismatch")
seen, services = set(), set()
for i, r in enumerate(rows[1:], 2):
    if len(r) != len(H): errs.append(f"line {i}: {len(r)} columns"); continue
    d = dict(zip(H, r))
    if d["id"] in seen: errs.append(f"{d['id']}: duplicate id")
    seen.add(d["id"])
    for k, allowed in (("kind", KIND), ("classification", CLASS), ("criticality", CRIT), ("third_party", {"Y","N"})):
        if d[k] not in allowed: errs.append(f"{d['id']}: bad {k} {d[k]!r}")
    for k in ("name","description","data_held","location","owner"):
        if not d[k].strip(): errs.append(f"{d['id']}: {k} is empty")
    if not os.path.exists(os.path.join(root, d["repo_path"])): errs.append(f"{d['id']}: missing path {d['repo_path']}")
    try:
        lr, nr = datetime.date.fromisoformat(d["last_reviewed"]), datetime.date.fromisoformat(d["next_review"])
        if nr <= lr: errs.append(f"{d['id']}: next_review is not after last_reviewed")
        if (nr - lr).days > 730: errs.append(f"{d['id']}: review interval over two years (Joint Standard 2, 7.1.2)")
        if nr < today: errs.append(f"{d['id']}: review overdue since {nr}")
    except ValueError:
        errs.append(f"{d['id']}: bad date")
    if d["kind"] == "service": services.add(d["repo_path"])
actual = {f"services/{g}/{s}" for g in os.listdir(os.path.join(root, "services")) if os.path.isdir(os.path.join(root, "services", g))
          for s in os.listdir(os.path.join(root, "services", g)) if os.path.isdir(os.path.join(root, "services", g, s))}
for p in sorted(actual - services): errs.append(f"service not in the register: {p}")
for p in sorted(services - actual): errs.append(f"register lists a service that does not exist: {p}")
print(f"{len(seen)} assets checked, {len(errs)} problems")
for e in errs: print(" -", e)
sys.exit(1 if errs else 0)
