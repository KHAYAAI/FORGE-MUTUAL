#!/usr/bin/env bash
# Rebuilds docs/FORGE_MUTUAL_COMPLETE.md and the FORGE-style PDF from the sources. Needs pandoc, node with playwright, and network
# access to fonts.googleapis.com for Inter Tight and JetBrains Mono.
set -euo pipefail
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
python3 -I scripts/build-complete-doc.py
pandoc docs/FORGE_MUTUAL_COMPLETE.md -f gfm -t html5 -o "$tmp/body.html"
python3 -I - "$tmp" <<'PY'
import re, sys
t = sys.argv[1]
body = open(f"{t}/body.html").read()
body = re.sub(r'<h1[^>]*>FORGE MUTUAL: the complete\s+document</h1>.*?<hr\s*/?>', '', body, count=1, flags=re.S)
n = [0]
def part(m):
    n[0] += 1
    title = re.sub('<[^>]+>', '', m.group(2)).replace("\n", " ")
    eyebrow, _, rest = title.partition('. ')
    return f'<section class="part"><div class="eyebrow"><span>{n[0]:02d}</span><i></i>{eyebrow.upper()}</div><h1{m.group(1)}>{rest or title}</h1></section>'
body = re.sub(r'<h1([^>]*)>(.*?)</h1>', part, body, flags=re.S)
tpl = open("docs/complete/pdf-template.html").read()
open(f"{t}/doc.html", "w").write(tpl.replace("{{BODY}}", body))
PY
node scripts/complete-pdf.js "$tmp/doc.html" docs/FORGE_MUTUAL_COMPLETE.pdf
pdfinfo docs/FORGE_MUTUAL_COMPLETE.pdf | grep Pages
