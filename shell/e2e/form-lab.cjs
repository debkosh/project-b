// The test bench for the form rules, end to end (docs/password-autofill.md, Carriers): pwlab
// writes out its test extension, a real Chromium loads it, and on pages of a local site the
// real form script, the extension's carrier and Go's rules work together: Go is told each
// frame's fields under the browser's own stamp, the popup shows the roles Go found and the
// test logins that match, a pick fills the form, a frame of another site is offered only
// its own site's logins, and a form sent is where saving would be offered, its password
// never told. Run: node shell/e2e/form-lab.cjs   (from the repo root; needs Go)

const fs = require('fs');
const os = require('os');
const path = require('path');
const http = require('http');
const { spawn, spawnSync } = require('child_process');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { until } = lib;
const check = (name, ok, detail) => lib.check(name, ok, detail);

const root = path.join(__dirname, '../..');
const work = fs.mkdtempSync(path.join(os.tmpdir(), 'pwlab-'));

(async () => {
  // Two local sites: the page's, and another's for a frame in it.
  const pages = {
    '/login': `<!doctype html><title>Sign in</title><form action="/session" method="post" onsubmit="event.preventDefault(); this.remove()">
      <label>Email <input name="email" type="email"></label><label>Password <input name="password" type="password"></label><button>Sign in</button></form>
      <iframe src="http://127.0.0.1:OTHER/widget" width="400" height="120"></iframe>`,
    '/widget': `<!doctype html><form><input name="user" autocomplete="username"><input name="pass" type="password" autocomplete="current-password"><button>Pay</button></form>`,
    '/search': `<!doctype html><form><input name="q" type="search" placeholder="Search"></form>`,
  };
  const serve = (other) => new Promise((done) => {
    const s = http.createServer((req, res) => {
      const body = pages[req.url.split('?')[0]];
      res.writeHead(body ? 200 : 404, { 'content-type': 'text/html' });
      res.end((body || '').replace('OTHER', String(other)));
    });
    s.listen(0, '127.0.0.1', () => done(s));
  });
  const frameSite = await serve(0);
  const site = await serve(frameSite.address().port);
  const here = `http://localhost:${site.address().port}`;
  const there = `http://127.0.0.1:${frameSite.address().port}`;

  // pwlab, built from this tree, with its test logins: one for each site, and one for
  // another host of the page's site that mustn't show on it (localhost has no other hosts).
  const bin = path.join(work, 'pwlab');
  const built = spawnSync('go', ['build', '-o', bin, './cmd/pwlab'], { cwd: path.join(root, 'core'), encoding: 'utf8' });
  if (built.status !== 0) {
    console.log(`FAIL pwlab builds -> ${built.stderr}`);
    process.exit(1);
  }
  fs.writeFileSync(path.join(work, 'logins.json'), JSON.stringify([
    { origin: here, username: 'asha@example.org', password: 'correct horse battery' },
    { origin: there, username: 'payer', password: 'frame-only-secret' },
    { origin: 'https://elsewhere.example', username: 'other', password: 'never-here' },
  ]));
  const port = 17391 + Math.floor(Math.random() * 500);
  const ext = path.join(work, 'extension');
  const said = [];
  const lab = spawn(bin, ['-addr', `127.0.0.1:${port}`, '-extension', ext, '-logins', path.join(work, 'logins.json')]);
  lab.stdout.on('data', (d) => said.push(...String(d).split('\n').filter(Boolean)));
  await until(() => said.some((l) => l.startsWith('pwlab on')), 15000);
  check('pwlab writes out the test extension, with the real form script in it', ['manifest.json', 'carrier.js', 'transport.js', 'popup.html', 'popup.js', 'forms.js'].every((f) => fs.existsSync(path.join(ext, f))) &&
    fs.readFileSync(path.join(ext, 'forms.js'), 'utf8') === fs.readFileSync(path.join(root, 'core/forms/page/matching.gen.js'), 'utf8') + '\n' + fs.readFileSync(path.join(root, 'core/forms/page/forms.js'), 'utf8') && fs.readFileSync(path.join(ext, 'carrier.js'), 'utf8').includes(`127.0.0.1:${port}`));

  const context = await chromium.launchPersistentContext(path.join(work, 'profile'), { channel: 'chromium', headless: true, args: [`--disable-extensions-except=${ext}`, `--load-extension=${ext}`] });
  const worker = context.serviceWorkers()[0] || await context.waitForEvent('serviceworker');
  const id = worker.url().split('/')[2];
  const ask = async (pathname, body) => (await fetch(`http://127.0.0.1:${port}${pathname}`, { method: 'POST', body: JSON.stringify(body) })).json();

  const page = await context.newPage();
  await page.goto(`${here}/login`);
  const tab = await worker.evaluate(async (url) => (await chrome.tabs.query({ url: `${url}/*` }))[0].id, here);
  const state = () => ask('/tab', { tab: String(tab) });
  const frameOf = async (origin) => (await state()).frames.find((f) => f.stamp.origin === origin);
  check('Go is told the page\'s fields and the frame\'s, each under the browser\'s own stamp', await until(async () => !!(await frameOf(here))?.found.length && !!(await frameOf(there))?.found.length, 15000), JSON.stringify(await state()));
  const top = await frameOf(here);
  const inner = await frameOf(there);
  check('the page is stamped the top frame of its own origin, and the frame as one under another site', top.stamp.top === true && top.stamp.crossOrigin === false && inner.stamp.top === false && inner.stamp.crossOrigin === true && inner.stamp.frame !== top.stamp.frame, JSON.stringify([top.stamp, inner.stamp]));
  check('Go finds a sign-in on the page: the email its username, the password its current password', top.found.length === 1 && top.found[0].kind === 'sign-in' &&
    JSON.stringify(top.found[0].roles.map((r) => r.role)) === '["username","current-password"]' && top.found[0].roles[0].field.includes('name=email'), JSON.stringify(top.found));
  check('each frame is offered its own origin\'s logins alone', JSON.stringify(top.found[0].logins.map((l) => l.username)) === '["asha@example.org"]' && JSON.stringify(inner.found[0].logins.map((l) => l.username)) === '["payer"]', JSON.stringify([top.found[0].logins, inner.found[0].logins]));
  check('and no password is in what the popup is told', !JSON.stringify(await state()).includes('correct horse') && !JSON.stringify(await state()).includes('frame-only-secret'));

  // The popup, opened as a page for this tab: it shows what Go found, and a pick fills.
  const popup = await context.newPage();
  await popup.goto(`chrome-extension://${id}/popup.html?tab=${tab}`);
  check('the popup shows the sign-in, its roles and the login to fill', await until(async () => (await popup.locator('.frame').count()) === 2) && (await popup.locator('body').innerText()).includes('sign-in') &&
    (await popup.getByRole('button', { name: 'Fill asha@example.org' }).isVisible()), await popup.locator('body').innerText());
  await page.bringToFront();
  await popup.getByRole('button', { name: 'Fill asha@example.org' }).click();
  check('a pick fills the page\'s form, and only it', await until(async () => (await page.locator('[name=email]').inputValue()) === 'asha@example.org' && (await page.locator('[name=password]').inputValue()) === 'correct horse battery') &&
    (await page.frameLocator('iframe').locator('[name=pass]').inputValue()) === '', await page.locator('[name=email]').inputValue());
  check('pwlab says the fields were filled', await until(() => said.some((l) => l.includes('filled 2 fields'))), said.slice(-4).join(' | '));

  // A form sent, and gone: where saving would be offered, the password told by its length.
  await page.locator('[name=password]').press('Enter');
  check('a form sent is where saving would be offered, with the username and the password\'s length alone', await until(() => said.some((l) => l.includes('a sign-in was sent') && l.includes('"asha@example.org"') && l.includes('a password of 21 letters')), 10000) &&
    !said.join('\n').includes('correct horse'), said.slice(-4).join(' | '));

  // A page with no role is told, and has nothing to offer.
  await page.goto(`${here}/search`);
  check('a search page has fields and no form with a role', await until(async () => { const f = await frameOf(here); return !!f && f.fields === 1 && f.found.length === 0; }, 10000), JSON.stringify(await state()));

  await context.close();
  lab.kill();
  site.close();
  frameSite.close();
  fs.rmSync(work, { recursive: true, force: true });
  await lib.done();
})().catch(async (e) => {
  console.log(`FAIL the run ended early -> ${e.stack}`);
  process.exit(1);
});
