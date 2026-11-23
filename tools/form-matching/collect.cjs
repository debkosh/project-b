// The form script, with the field matching before it as Go embeds them, run over a folder
// of saved pages in a real Chromium with the network shut: what it tells of each page, and
// the ref it gave each field that carries a hand-written label (data-manual-scoring, as
// DuckDuckGo's test-forms have). A JSON line a page. tools/form-matching.sh runs it.
// node tools/form-matching/collect.cjs <folder with index.json> > fields.jsonl
const fs = require('fs');
const path = require('path');
const root = path.join(__dirname, '../..');
const corpus = path.resolve(process.argv[2]);
const { chromium } = require(path.join(root, 'shell/node_modules/playwright'));
const page = (name) => fs.readFileSync(path.join(root, 'core/forms/page', name), 'utf8');
// The script names a field by a ref it keeps to itself: this run's copy of it shares them.
const marker = 'const refs = new WeakMap(); // element -> ref';
if (!page('forms.js').includes(marker)) throw new Error('core/forms/page/forms.js has no refs map where this looks for it');
const script = page('matching.gen.js') + '\n' + page('forms.js').replace(marker, marker + '\n    window.__pbRefs = refs;');
const transport = `window.__sent = []; globalThis.__pbFormsTransport = { send: (type, payload) => window.__sent.push([type, JSON.parse(JSON.stringify(payload))]), receive: () => {} };`;
(async () => {
  const index = JSON.parse(fs.readFileSync(path.join(corpus, 'index.json'), 'utf8'));
  const browser = await chromium.launch();
  const context = await browser.newContext();
  await context.addInitScript(transport + script);
  await context.route('**/*', (route) => {
    const u = new URL(route.request().url());
    const file = path.join(corpus, path.basename(decodeURIComponent(u.pathname)));
    if (u.host !== 'corpus.test' || !fs.existsSync(file)) return route.abort();
    route.fulfill({ contentType: 'text/html; charset=utf-8', body: fs.readFileSync(file) });
  });
  for (const entry of index) {
    const tab = await context.newPage();
    let out = { file: entry.html };
    try {
      await tab.goto('https://corpus.test/' + encodeURIComponent(entry.html), { waitUntil: 'domcontentloaded', timeout: 15000 });
      await tab.waitForTimeout(450);
      out = { ...out, ...(await tab.evaluate(() => ({
        told: (window.__sent || []).filter((m) => m[0] === 'fields').map((m) => m[1]).at(-1) || { fields: [], forms: [] },
        labelled: [...document.querySelectorAll('[data-manual-scoring]')].map((el) => ({ label: el.getAttribute('data-manual-scoring'), ref: (window.__pbRefs && window.__pbRefs.get(el)) || '' })),
      }))) };
    } catch (e) {
      out.error = String(e.message).split('\n')[0].slice(0, 160);
    }
    console.log(JSON.stringify(out));
    await tab.close();
  }
  await browser.close();
})();
