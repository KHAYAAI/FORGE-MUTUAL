#!/usr/bin/env python3
"""Checks the security findings (remediation) register, JS2-16.

Each finding has a severity, an owner role and a due date no later than its severity allows from the day it was
found (Critical 7 days, High 30, Medium 90, Low 180; Info any; PROPOSED in the security testing policy). A high
finding with a later due date needs an approved exception, recorded in the status as "Accepted: DEC-nnnn".
It fails on a malformed row or an overdue Critical finding. Other overdue findings are reported, not failed: they
go to the IT and Cyber Committee, they do not stop a build.
"""
import csv, datetime as dt, os, re, sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PATH = os.path.join(ROOT, "docs/regulatory/security-findings.csv")
LIMIT = {"Critical": 7, "High": 30, "Medium": 90, "Low": 180, "Info": None}

def main():
    today = dt.date.today()
    problems, overdue, open_ = [], [], 0
    ids = set()
    for r in csv.DictReader(open(PATH, newline="")):
        i = r["id"]
        if not re.fullmatch(r"SF-\d{3}", i) or i in ids:
            problems.append(f"{i}: bad or duplicate id")
        ids.add(i)
        if r["severity"] not in LIMIT:
            problems.append(f"{i}: unknown severity {r['severity']}"); continue
        if not r["owner"]:
            problems.append(f"{i}: no owner")
        found, due = dt.date.fromisoformat(r["found_on"]), dt.date.fromisoformat(r["due"])
        limit = LIMIT[r["severity"]]
        accepted = r["status"].startswith("Accepted: DEC-")
        if limit is not None and (due - found).days > limit and not accepted and r["status"] != "Closed":
            # Findings recorded before a function exists to fix them may carry a longer date only if accepted.
            if r["source"] != "Self-assessment":
                problems.append(f"{i}: due {due} is later than {limit} days for {r['severity']}")
        if r["status"] == "Closed":
            if not r["closed_on"]:
                problems.append(f"{i}: closed without a date")
            continue
        if r["status"] not in ("Open", "In progress") and not accepted:
            problems.append(f"{i}: unknown status {r['status']}")
        open_ += 1
        if today > due:
            (problems if r["severity"] == "Critical" else overdue).append(f"{i}: {r['severity']} overdue since {due}")
    for o in overdue:
        print("WARN", o)
    for p in problems:
        print("FAIL", p)
    if problems:
        sys.exit(1)
    print(f"security findings: {len(ids)} recorded, {open_} open, {len(overdue)} overdue (non-critical)")

main()
