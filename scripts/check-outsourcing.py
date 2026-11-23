#!/usr/bin/env python3
"""Checks the outsourcing register and works out when the Prudential Authority must be told.

Guidance Note 5/2014 (OUT-02): notify the PA at least 20 working days before a material outsourcing starts. For each
material provider with a planned start date this computes the latest notice date (weekends and South African public
holidays skipped) and writes it into pa_notice_due. It fails when:
  - a material provider is in use without a notice recorded, a contract or a met contract schedule;
  - the notice date has passed and no notice is recorded;
  - a material provider has no named owner role.
Usage: check-outsourcing.py [--today YYYY-MM-DD] [--write]
"""
import csv, datetime as dt, os, sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PATH = os.path.join(ROOT, "docs/regulatory/outsourcing-register.csv")
NOTICE_WORKING_DAYS = 20

def easter(y):
    a, b, c = y % 19, y // 100, y % 100
    d, e = b // 4, b % 4
    f = (b + 8) // 25
    g = (b - f + 1) // 3
    h = (19 * a + b - d - g + 15) % 30
    i, k = c // 4, c % 4
    l = (32 + 2 * e + 2 * i - h - k) % 7
    m = (a + 11 * h + 22 * l) // 451
    month = (h + l - 7 * m + 114) // 31
    return dt.date(y, month, (h + l - 7 * m + 114) % 31 + 1)

def holidays(y):
    fixed = [(1, 1), (3, 21), (4, 27), (5, 1), (6, 16), (8, 9), (9, 24), (12, 16), (12, 25), (12, 26)]
    days = {dt.date(y, m, d) for m, d in fixed}
    # Public Holidays Act: a holiday on a Sunday moves to the Monday.
    days |= {d + dt.timedelta(days=1) for d in list(days) if d.weekday() == 6}
    e = easter(y)
    days |= {e - dt.timedelta(days=2), e + dt.timedelta(days=1)}  # Good Friday, Family Day
    return days

def working_days_before(start, n):
    d, hol = start, holidays(start.year) | holidays(start.year - 1)
    while n > 0:
        d -= dt.timedelta(days=1)
        if d.weekday() < 5 and d not in hol:
            n -= 1
    return d

def main():
    today = dt.date.today()
    if "--today" in sys.argv:
        today = dt.date.fromisoformat(sys.argv[sys.argv.index("--today") + 1])
    rows = list(csv.DictReader(open(PATH, newline="")))
    problems = []
    for r in rows:
        material = r["material"].startswith("Yes")
        if not material:
            continue
        name = r["provider"]
        if "VACANT" not in r["owner"] and "(" not in r["owner"]:
            problems.append(f"{name}: owner must name a role")
        if r["planned_start"]:
            due = working_days_before(dt.date.fromisoformat(r["planned_start"]), NOTICE_WORKING_DAYS)
            r["pa_notice_due"] = due.isoformat()
            if today > due and not r["pa_notified"]:
                problems.append(f"{name}: PA notice was due by {due} and none is recorded")
        if r["status"].startswith("In use"):
            if not r["pa_notified"]:
                problems.append(f"{name}: in use without a PA notice")
            if r["contract"] in ("", "None") or r["contract_schedule_met"] != "Yes":
                problems.append(f"{name}: in use without a contract meeting provider-contract-schedule.md")
    if "--write" in sys.argv:
        w = csv.DictWriter(open(PATH, "w", newline=""), fieldnames=list(rows[0].keys()), lineterminator="\n")
        w.writeheader(); w.writerows(rows)
    for p in problems:
        print("FAIL", p)
    if problems:
        sys.exit(1)
    m = sum(r["material"].startswith("Yes") for r in rows)
    print(f"outsourcing: {len(rows)} providers, {m} material, 0 problems")

if __name__ == "__main__":
    main()
