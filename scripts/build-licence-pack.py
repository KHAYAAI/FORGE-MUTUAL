#!/usr/bin/env python3
"""Assembles docs/licence-application/PACK.md: every item the licence application needs, from the regulatory register and the
section map in docs/licence-application/sections.csv, with its owner, evidence and gap.

It is an index and a tracker, not the application: it points to the documents and says honestly which are drafts, which are missing
and which need a person or a regulator. `--check` fails if the pack is stale, a licensing row (LIC-*) is in no section, an evidence
path does not exist, or a section names an unknown register id. `make licence-pack` rebuilds it.
"""
import csv, os, sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
REG = os.path.join(ROOT, "docs/regulatory/register.csv")
SEC = os.path.join(ROOT, "docs/licence-application/sections.csv")
OUT = os.path.join(ROOT, "docs/licence-application/PACK.md")

def rows(p): return list(csv.DictReader(open(p, newline="")))

def build():
    reg = {r["id"]: r for r in rows(REG)}
    secs = rows(SEC)
    problems = []
    used = set()
    for s in secs:
        for i in s["register_ids"].split(";"):
            used.add(i)
            if i not in reg: problems.append(f"section {s['section']}: unknown register id {i}")
    for i in reg:
        if i.startswith("LIC-") and i not in used: problems.append(f"{i} is in no section of the pack")
    out = ["# Licence application pack: index and tracker", "",
           "> Generated from the regulatory register and [`sections.csv`](sections.csv) by `scripts/build-licence-pack.py`. **This is an index of",
           "> what the application needs, not the application.** It says where each document is, who owns it, and what is still missing.",
           "> Nothing in it is approved. Route: mutual bank (planned) or bank (kept open), see [`../governance/LEGAL_FORMS.md`](../governance/LEGAL_FORMS.md).", ""]
    counts = {}
    item_status = []
    for s in secs:
        for i in s["register_ids"].split(";"):
            if i in reg:
                counts[reg[i]["status"]] = counts.get(reg[i]["status"], 0) + 1
    total = sum(counts.values())
    out += [f"**{len(secs)} sections, {total} requirements:** " + ", ".join(f"{n} {k}" for k, n in sorted(counts.items())) + ". No requirement is `Evidenced`: nothing is approved and no seat except the founder's is filled.", "",
            "| Section | Title | Owner | Requirements | Open or external |", "|---|---|---|---|---|"]
    for s in secs:
        ids = s["register_ids"].split(";")
        openn = sum(1 for i in ids if i in reg and reg[i]["status"] in ("Open", "External"))
        out.append(f"| {s['section']} | {s['title']} | {s['owner_role']} | {len(ids)} | {openn} |")
    out.append("")
    for s in secs:
        out += [f"## {s['section']} · {s['title']}", "", f"**Owner:** {s['owner_role']}. {s['note']}", "",
                "| Requirement | Type | Status | Evidence | Gap or next step |", "|---|---|---|---|---|"]
        for i in s["register_ids"].split(";"):
            r = reg.get(i)
            if not r: continue
            ev = []
            for e in [x for x in r["evidence"].split(";") if x]:
                if not os.path.exists(os.path.join(ROOT, e)): problems.append(f"{i}: evidence path does not exist: {e}")
                ev.append(f"[{os.path.basename(e.rstrip('/')) or e}](../../{e})")
            gap = r["gap_or_next_step"].replace("|", "/")
            out.append(f"| **{i}** {r['requirement'][:110].replace('|','/')} | {r['type']} | {r['status']} | {'<br>'.join(ev) or '*none*'} | {gap[:240]} |")
        out.append("")
    out += ["## What no document can close", "",
            "- **Capital** (A3) and **founding members** (A4): money and people.", "- **Fit-and-proper** (B2): each person files, and the regulator assesses.",
            "- **Pre-application meeting** (F1) and every approval: the Prudential Authority and FSCA act.",
            "- **Approval of every policy**: a board that does not yet exist.", ""]
    return "\n".join(out) + "\n", problems

def main():
    text, problems = build()
    if "--check" in sys.argv:
        if not os.path.exists(OUT) or open(OUT).read() != text: problems.append("PACK.md is stale; run make licence-pack")
    else:
        open(OUT, "w").write(text)
    for p in problems: print("FAIL", p)
    if problems: sys.exit(1)
    print("licence pack: ok")

main()
