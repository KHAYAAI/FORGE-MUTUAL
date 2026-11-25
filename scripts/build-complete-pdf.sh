#!/usr/bin/env bash
# Rebuilds docs/FORGE_MUTUAL_COMPLETE.md and the FORGE-style PDF from the sources. Needs pandoc, node with playwright, and network
# access to fonts.googleapis.com for Inter Tight and JetBrains Mono.
set -euo pipefail
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
mm=.cache/mermaid/node_modules/mermaid/dist/mermaid.min.js
if [ ! -f "$mm" ]; then mkdir -p .cache/mermaid && (cd .cache/mermaid && npm init -y >/dev/null && npm install --silent mermaid@10.9.1); fi
cp "$mm" "$tmp/mermaid.min.js"
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
import html as h
body = re.sub(r'<pre class="mermaid"><code>(.*?)</code></pre>', lambda m: '<div class="diagram"><pre class="mermaid">' + m.group(1) + '</pre></div>', body, flags=re.S)
tpl = open("docs/complete/pdf-template.html").read()
mermaid = """<script src="mermaid.min.js"></script><script>
mermaid.initialize({startOnLoad:false, theme:'base', securityLevel:'loose', flowchart:{htmlLabels:true, useMaxWidth:true}, sequence:{useMaxWidth:true},
 themeVariables:{fontFamily:'Inter Tight, Inter, sans-serif', fontSize:'13px', primaryColor:'#ece8e0', primaryBorderColor:'#26241f',
 primaryTextColor:'#0a0a0a', lineColor:'#6f6a62', secondaryColor:'#ffffff', tertiaryColor:'#f6f4ef', clusterBkg:'#f6f4ef', clusterBorder:'#c9c3b7'}});
window.diagramsDone = mermaid.run({querySelector:'pre.mermaid'}).then(()=>true).catch(e=>{return (e && (e.message||e.str||JSON.stringify(e))) || String(e)});
</script>"""
tpl = tpl.replace("</style>", ".diagram{margin:12pt 0;padding:12pt;border:1px solid var(--rule);page-break-inside:avoid;text-align:center}.diagram svg{max-width:100%;height:auto}pre.mermaid{background:none;padding:0}</style>")
tpl = tpl.replace("</body>", mermaid + "</body>")
open(f"{t}/doc.html", "w").write(tpl.replace("{{BODY}}", body))
PY
node scripts/complete-pdf.js "$tmp/doc.html" docs/FORGE_MUTUAL_COMPLETE.pdf
pdfinfo docs/FORGE_MUTUAL_COMPLETE.pdf | grep Pages
