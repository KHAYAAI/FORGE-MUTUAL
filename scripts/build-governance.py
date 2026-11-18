#!/usr/bin/env python3
"""Builds docs/governance/ROLES.md and POLICY_FRAMEWORK.md from roles.csv and policies.csv, and checks them.

With --check it fails if the generated files are stale, a policy file is unlisted, a listed file is missing, or a
register id does not exist. Run `make governance` after editing either CSV.
"""
import csv, os, sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
GOV = os.path.join(ROOT, "docs/governance")
POL = os.path.join(ROOT, "docs/regulatory/policies")
REG = os.path.join(ROOT, "docs/regulatory/register.csv")

def rows(p): return list(csv.DictReader(open(p, newline="")))

def roles_md(roles):
    out = ["# Roles roster", "",
           "> Generated from [`roles.csv`](roles.csv) by `scripts/build-governance.py`. Edit the CSV, then run `make governance`.",
           "> **Every seat is VACANT except the founder's.** Replace `VACANT` with the appointed person's name only after the",
           "> appointment is made in writing and, for a director or executive, the fit-and-proper assessment is done.", ""]
    filled = [r for r in roles if not r["holder"].startswith("VACANT")]
    out += [f"**{len(roles)} seats; {len(filled)} with a name; {len(roles)-len(filled)} vacant.**", "",
            "| # | Role | Line | Holder | Reports to | Priority |", "|---|---|---|---|---|---|"]
    out += [f"| {r['id']} | {r['role']} | {r['line']} | {r['holder']} | {r['reports_to']} | {r['priority']} |" for r in roles]
    out += ["", "## What each seat needs", ""]
    for r in roles:
        out += [f"### {r['id']} · {r['role']}", "",
                f"- **Holder:** {r['holder']} ({r['status']})",
                f"- **Must have:** {r['must_have']}",
                f"- **Nice to have:** {r['nice_to_have']}",
                f"- **May share the seat with:** {r['may_combine_with']}",
                f"- **Must never be the same person as:** {r['must_not_combine_with']}",
                f"- **Why it is required:** {r['required_by'].replace(';', ', ')}",
                f"- **When:** {r['priority']}", ""]
    return "\n".join(out)

def framework_md(pols):
    out = ["# Policy framework", "",
           "> Generated from [`policies.csv`](policies.csv) by `scripts/build-governance.py`. Every policy is a DRAFT; none is approved.", "",
           "Policies sit under the risk management framework and the risk appetite statement (see the hierarchy in",
           "[`STRUCTURE.md`](STRUCTURE.md#5-policy-hierarchy)). Each has one owner who keeps it current, one approver, and a",
           "review cycle. A policy takes effect only when its approver signs the sign-off block and the date is recorded.", "",
           "## Lifecycle", "", "```mermaid", "flowchart LR",
           '  D["Draft<br/>(owner)"] --> R["Second-line review<br/>(CRO / CCO)"] --> L["Legal review"] --> C["Committee recommends"] --> A["Approver adopts<br/>(signs, dates)"] --> P["Published to staff<br/>trained, attested"] --> V["Review cycle<br/>(yearly)"] --> D',
           "```", ""]
    by = {}
    for p in pols: by.setdefault(p["area"], []).append(p)
    for area, ps in by.items():
        out += [f"## {area}", "", "| Policy | Owner | Approver | Review | Register rows | Status |", "|---|---|---|---|---|---|"]
        out += [f"| [{p['title']}](../regulatory/policies/{p['file']}) | {p['owner']} | {p['approver']} | {p['review']} | {p['rows'].replace(';', ', ')} | {p['status']} |" for p in ps]
        out.append("")
    return "\n".join(out)

def main():
    roles, pols = rows(os.path.join(GOV, "roles.csv")), rows(os.path.join(GOV, "policies.csv"))
    reg = {r["id"] for r in rows(REG)}
    problems = []
    files = {f for f in os.listdir(POL) if f.endswith(".md") and f != "README.md"}
    listed = {p["file"] for p in pols}
    problems += [f"policy file not in policies.csv: {f}" for f in sorted(files - listed)]
    problems += [f"policies.csv lists a missing file: {f}" for f in sorted(listed - files)]
    for p in pols:
        problems += [f"{p['file']}: unknown register id {i}" for i in p["rows"].split(";") if i and i not in reg]
    for r in roles:
        problems += [f"{r['id']}: unknown register id {i}" for i in r["required_by"].split(";") if "-" in i and i.split("-")[0] in {x.split("-")[0] for x in reg} and i not in reg]
    want = {"ROLES.md": roles_md(roles) + "\n", "POLICY_FRAMEWORK.md": framework_md(pols) + "\n"}
    check = "--check" in sys.argv
    for name, text in want.items():
        path = os.path.join(GOV, name)
        if check:
            if not os.path.exists(path) or open(path).read() != text: problems.append(f"{name} is stale; run make governance")
        else: open(path, "w").write(text)
    for p in problems: print("FAIL", p)
    if problems: sys.exit(1)
    print(f"governance: {len(roles)} roles, {len(pols)} policies, all register ids known")

main()
