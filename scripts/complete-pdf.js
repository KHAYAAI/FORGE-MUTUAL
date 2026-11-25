const { chromium } = require('playwright');
(async () => {
  const b = await chromium.launch(); const p = await b.newPage();
  await p.goto('file://' + process.argv[2], { waitUntil: 'networkidle' });
  await p.evaluate(() => document.fonts.ready);
  await p.pdf({ path: process.argv[3], format: 'A4', printBackground: true, preferCSSPageSize: true, displayHeaderFooter: true, headerTemplate: '<div></div>',
    footerTemplate: '<div style="width:100%;margin:0 18mm;display:flex;justify-content:space-between;font-family:monospace;font-size:6.5px;letter-spacing:1.5px;color:#6f6a62;text-transform:uppercase"><span>© MMXXVI MYFORGE (PTY) LTD · FORGE MUTUAL</span><span>The complete document · <span class="pageNumber"></span> / <span class="totalPages"></span></span></div>' });
  await b.close();
})();
