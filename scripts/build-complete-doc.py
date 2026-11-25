#!/usr/bin/env python3
"""Assembles docs/FORGE_MUTUAL_COMPLETE.md: Part I (docs/complete/00-synthesis.md) followed by every source document in full.
Run again whenever a source changes; `--check` fails if the assembled document is stale."""
import os, re, sys
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PARTS = [
    ("Part II. Business", [("The concept and business context", "docs/CONTEXT.md"), ("The bank model: Erebor and Investec", "docs/BANK_MODEL.md"),
        ("Draft business plan and public-interest motivation", "docs/licence-application/BUSINESS_PLAN_DRAFT.md"),
        ("Wealth and investment partnership", "docs/products/WEALTH_PARTNERSHIP.md"), ("Digital assets", "docs/products/DIGITAL_ASSETS.md")]),
    ("Part III. Legal form and governance", [("The two legal forms", "docs/governance/LEGAL_FORMS.md"), ("Draft constitution", "docs/licence-application/CONSTITUTION_DRAFT.md"),
        ("Governance structure", "docs/governance/STRUCTURE.md"), ("Policies mapped to the regulator's structure", "docs/governance/POLICY_MAP.md"),
        ("Relationship service standard", "docs/governance/SERVICE_STANDARD.md"), ("Brief for the regulatory lawyer", "docs/governance/LAWYER_BRIEF.md"),
        ("Sponsor bank pitch", "docs/governance/SPONSOR_BANK_PITCH.md")]),
    ("Part IV. Licence application and regulation", [("Licence application pack", "docs/licence-application/PACK.md"),
        ("Regulatory return forms and sources", "docs/regulatory/returns/SOURCES.md"), ("Regulatory returns built", "services/bank/reg-reporting/SPEC.md")]),
    ("Part V. Technology and operations", [("Architecture", "docs/ARCHITECTURE.md"), ("Hosting", "docs/HOSTING.md"), ("Project status (README)", "README.md")]),
]
def clean(text, src):
    base = os.path.dirname(src)
    def link(m):
        label, target = m.group(1), m.group(2)
        if re.match(r"^[a-z]+:", target) or target.startswith("#"): return m.group(0)
        path = os.path.normpath(os.path.join(base, target.split("#")[0]))
        return f"{label} (`{path}`)"
    text = re.sub(r"\[([^\]]+)\]\(([^)\s]+)\)", link, text)
    return re.sub(r"(?m)^(#{1,5}) ", lambda m: "#" + m.group(1) + " ", text)  # demote one level
out = [open(os.path.join(ROOT, "docs/complete/00-synthesis.md")).read()]
for part, docs in PARTS:
    out.append(f"\n---\n\n# {part}\n")
    for title, src in docs:
        out.append(f"\n## {title}\n\n*Source: `{src}`*\n\n" + clean(open(os.path.join(ROOT, src)).read(), src))
text = "\n".join(out)
dest = os.path.join(ROOT, "docs/FORGE_MUTUAL_COMPLETE.md")
if "--check" in sys.argv:
    if not os.path.exists(dest) or open(dest).read() != text: print("FAIL FORGE_MUTUAL_COMPLETE.md is stale"); sys.exit(1)
else:
    open(dest, "w").write(text)
print(f"complete document: {len(text.split())} words")
