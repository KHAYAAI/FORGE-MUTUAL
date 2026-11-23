#!/usr/bin/env python3
"""Check docs/regulatory/data-classes.csv (Guidance Note 5 of 2018, 4.1: classification, residency, cloud model per class,
security rules, breach process): fields, allowed classes, that every place named exists, and that Restricted data is
never allowed onto public SaaS except where the row says the provider holds it under contract.
Run: python3 -I scripts/check-data-classes.py   (from the repository root)"""
import csv, os, sys
H = ["id","data","classification","personal_info","residency","cloud_model","security_rules","held_in","retention","breach_notice"]
CLASS = {"Public","Internal","Confidential","Restricted"}
root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
rows = list(csv.reader(open(os.path.join(root, "docs/regulatory/data-classes.csv"), newline="")))
errs = [] if rows[0] == H else ["header mismatch"]
ids = set()
for r in rows[1:]:
    if len(r) != len(H):
        errs.append(f"{r[:1]}: {len(r)} columns"); continue
    d = dict(zip(H, r))
    if d["id"] in ids: errs.append(f"{d['id']}: duplicate")
    ids.add(d["id"])
    if d["classification"] not in CLASS: errs.append(f"{d['id']}: bad classification {d['classification']!r}")
    for k in H[1:]:
        if not d[k].strip(): errs.append(f"{d['id']}: {k} is empty")
    for p in filter(None, (x.strip() for x in d["held_in"].split(";"))):
        if not os.path.exists(os.path.join(root, p)): errs.append(f"{d['id']}: missing path {p}")
    if d["classification"] == "Restricted" and "SaaS" in d["cloud_model"] and "contract" not in d["cloud_model"]:
        errs.append(f"{d['id']}: Restricted data allowed on SaaS without a contract")
print(f"{len(ids)} data classes checked, {len(errs)} problems")
for e in errs: print(" -", e)
sys.exit(1 if errs else 0)
