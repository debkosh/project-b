// The form script (core/forms/page/forms.js, docs/password-autofill.md) in a real page: what
// it reports of a frame's fields, that it tells a press, a form sent and a form gone with
// only the watched fields' values, and that it fills what Go says and nothing a person
// couldn't use. The transport is this test's own: what the script sends is kept, and Go's
// messages are handed to it. No engine and no Go: the script alone.
// Run: node shell/e2e/form-script.cjs   (from the repo root)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { until } = lib;
const check = (name, ok, detail) => lib.check(name, ok, detail);

const script = fs.readFileSync(path.join(__dirname, '../../core/forms/page/forms.js'), 'utf8');
// The carrier's side, in the page: what the script sent, and a way to speak to it as Go does.
const transport = `
  window.__sent = [];
  globalThis.__pbFormsTransport = {
    send: (type, payload) => window.__sent.push([type, JSON.parse(JSON.stringify(payload))]),
    receive: (handler) => (window.__toScript = handler),
  };`;

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();
  await page.addInitScript(transport + script);
  const sent = (type) => page.evaluate((t) => window.__sent.filter((m) => m[0] === t).map((m) => m[1]), type);
  const last = async (type) => (await sent(type)).at(-1);
  const toScript = (type, payload) => page.evaluate(([t, p]) => window.__toScript(t, p), [type, payload]);
  // Each page is a real one of a site of its own, so the script starts with it as it does
  // in a browser, before the page has any field.
  let pages = 0;
  const open = async (html) => {
    const url = `https://forms.test/page-${++pages}`;
    await page.route(url, (route) => route.fulfill({ contentType: 'text/html', body: `<!doctype html><body>${html}</body>` }));
    await page.goto(url);
  };
  const byName = async (name) => (await last('fields')).fields.find((f) => f.name === name);

  // What it reports.
  await open(`
    <form id="login" action="/session" method="POST">
      <label for="u">Email address</label><input id="u" name="email" type="email" autocomplete="username" placeholder="you@example.com">
      <div><span>Password</span></div><input name="pass" type="password" maxlength="64">
      <input name="csrf" type="hidden" value="x"><input name="remember" type="checkbox">
      <input name="gone" type="text" style="display:none"><input name="off" type="text" disabled>
      <input name="ro" type="text" readonly value="kept" aria-label="Account number">
      <button>Sign in</button><input type="submit" value="Continue">
    </form>
    <input name="q" type="search" placeholder="Search"><textarea name="notes"></textarea><select name="country"><option>IN</option></select>`);
  check('a page with fields is told to Go, once it has settled', await until(async () => (await sent('fields')).length >= 1));
  const told = await last('fields');
  check('only inputs a role could be typed in are told: no hidden, checkbox, textarea or select', JSON.stringify(told.fields.map((f) => f.name)) === '["email","pass","gone","off","ro","q"]', JSON.stringify(told.fields.map((f) => f.name)));
  const email = await byName('email');
  check('a field is told with what the page calls it, and no value', email.tag === 'input' && email.type === 'email' && email.id === 'u' && email.autocomplete === 'username' && email.placeholder === 'you@example.com' &&
    email.label === 'Email address' && email.visible && email.focusable && email.empty && !('value' in email), JSON.stringify(email));
  const pass = await byName('pass');
  check('a field with no label is told with the text before it, and its longest length', pass.nearby === 'Password' && pass.maxLength === 64 && !pass.label, JSON.stringify(pass));
  check('a field out of sight is told as that, and one switched off as not to be used', (await byName('gone')).visible === false && (await byName('off')).focusable === false);
  const ro = await byName('ro');
  check('one that can\'t be typed in says so, and one holding something isn\'t empty', ro.readOnly === true && ro.empty === false && ro.ariaLabel === 'Account number', JSON.stringify(ro));
  check('each field names its form, and the form is told with where it goes and its buttons', email.form === pass.form && !(await byName('q')).form &&
    JSON.stringify(told.forms) === JSON.stringify([{ ref: email.form, action: '/session', method: 'post', buttons: ['Sign in', 'Continue'] }]), JSON.stringify(told.forms));

  // The same isn't told twice; a change is.
  let n = (await sent('fields')).length;
  await page.evaluate(() => document.body.append(document.createElement('p')));
  await page.waitForTimeout(400);
  check('a change to the page that changes no field tells nothing', (await sent('fields')).length === n);
  await page.locator('[name=pass]').evaluate((el) => (el.type = 'text'));
  check('a password shown as text is told as one that was a password', await until(async () => (await byName('pass')).wasPassword === true && (await byName('pass')).type === 'text'), JSON.stringify(await byName('pass')));
  await page.locator('[name=email]').fill('asha@example.org');
  check('a field that fills is told as no longer empty, still with no value', await until(async () => (await byName('email')).empty === false) && !JSON.stringify(await sent('fields')).includes('asha@example.org'));

  // A press.
  await page.locator('[name=pass]').click();
  const press = await last('pressed');
  const box = await page.locator('[name=pass]').boundingBox();
  check('a press on a field tells which, and where it is', !!press && press.ref === pass.ref && Math.abs(press.box.x - box.x) < 1 && Math.abs(press.box.y - box.y) < 1 && Math.abs(press.box.w - box.width) < 1 && press.box.h > 5, JSON.stringify(press));
  // A press on the page away from any field is told with no field, so a list goes.
  await page.mouse.click(600, 500);
  check('a press on the page that is on no field is told as one on none', await until(async () => (await last('pressed')).ref === ''), JSON.stringify(await last('pressed')));

  // A field the page focuses itself, as autofocus does, is told as well: the list may be
  // offered there, and nothing is filled until the person picks from it.
  await page.evaluate(() => document.querySelector('[name=email]').focus());
  check('a field the page focuses itself is told too, as an autofocus is', await until(async () => (await last('pressed')).ref === email.ref), JSON.stringify(await last('pressed')));

  // A form sent: only what Go watches, and only once watched.
  await page.evaluate(() => document.querySelector('form').addEventListener('submit', (e) => e.preventDefault()));
  await page.locator('[name=pass]').fill('correct horse');
  await page.locator('[name=pass]').press('Enter');
  await page.waitForTimeout(200);
  check('a form sent while nothing is watched tells nothing', (await sent('submitted')).length === 0);
  await toScript('watch', { refs: [email.ref, pass.ref] });
  await page.waitForTimeout(400);
  await page.getByRole('button', { name: 'Sign in' }).click();
  check('a form sent tells the watched fields\' values, and no other field\'s', await until(async () => (await sent('submitted')).length === 1) &&
    JSON.stringify(await last('submitted')) === JSON.stringify({ form: email.form, values: { [email.ref]: 'asha@example.org', [pass.ref]: 'correct horse' } }), JSON.stringify(await sent('submitted')));
  check('nothing the script ever sent holds the value of a field that isn\'t watched', !JSON.stringify(await page.evaluate(() => window.__sent)).includes('kept'));

  // Filling: what Go says, into fields a person could use, as their typing would.
  await open(`<form><input name="user"><input name="pass" type="password"><input name="hid" type="password" style="display:none"><input name="ro" type="password" readonly></form>
    <script>window.heard = []; document.addEventListener('input', (e) => window.heard.push(e.target.name + '=' + e.target.value));</script>`);
  await until(async () => (await sent('fields')).length >= 1);
  const [user, pw, hid, readonly] = await Promise.all(['user', 'pass', 'hid', 'ro'].map(byName));
  await toScript('fill', { values: { [user.ref]: 'asha', [pw.ref]: 'tr0ub4dor&3', [hid.ref]: 'nope', [readonly.ref]: 'nope', f9999: 'nope' } });
  check('a fill sets the fields named, and answers with those it filled', (await page.locator('[name=user]').inputValue()) === 'asha' && (await page.locator('[name=pass]').inputValue()) === 'tr0ub4dor&3' &&
    JSON.stringify((await last('filled')).refs.sort()) === JSON.stringify([user.ref, pw.ref].sort()), JSON.stringify(await last('filled')));
  check('a field out of sight, or one that can\'t be typed in, is never filled', (await page.locator('[name=hid]').inputValue()) === '' && (await page.locator('[name=ro]').inputValue()) === '');
  check('the page hears a fill as it hears typing', JSON.stringify(await page.evaluate(() => window.heard)) === '["user=asha","pass=tr0ub4dor&3"]', JSON.stringify(await page.evaluate(() => window.heard)));

  // While Go asks for a field's keys, its arrows, Escape, and Enter once an arrow has
  // moved, are told and kept from the page; nothing typed is told; and with no field
  // asked for, the keys are the page's own.
  await open(`<form onsubmit="event.preventDefault(); window.sent = (window.sent || 0) + 1"><input name="user"><input name="pass" type="password"><button>Go</button></form>
    <script>window.keys = []; document.addEventListener('keydown', (e) => window.keys.push(e.key));</script>`);
  await until(async () => (await sent('fields')).length >= 1);
  const ku = await byName('user');
  await page.locator('[name=user]').click();
  await page.keyboard.press('ArrowDown');
  check('with no field asked for, a key is the page\'s and nothing is told', (await sent('key')).length === 0 && JSON.stringify(await page.evaluate(() => window.keys)) === '["ArrowDown"]');
  await toScript('keys', { ref: ku.ref });
  await page.keyboard.press('Enter');
  check('Enter before any arrow is the page\'s: it sends the form, and isn\'t told', (await sent('key')).length === 0 && (await page.evaluate(() => window.sent)) === 1);
  await page.keyboard.type('ab');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowUp');
  await page.keyboard.press('Enter');
  await page.keyboard.press('Escape');
  check('the arrows, then Enter, and Escape are told, in order', JSON.stringify((await sent('key')).map((k) => k.key)) === '["ArrowDown","ArrowUp","Enter","Escape"]' && (await sent('key')).every((k) => k.ref === ku.ref), JSON.stringify(await sent('key')));
  check('and kept from the page, which heard only the Enter before and what was typed', JSON.stringify(await page.evaluate(() => window.keys)) === '["ArrowDown","Enter","a","b"]' && (await page.evaluate(() => window.sent)) === 1, JSON.stringify(await page.evaluate(() => window.keys)));
  check('nothing typed was told', !JSON.stringify(await page.evaluate(() => window.__sent)).includes('"ab"') && (await sent('key')).length === 4);
  await page.locator('[name=pass]').click();
  await page.keyboard.press('ArrowDown');
  check('a key in another field than the one asked for is the page\'s', (await sent('key')).length === 4);
  await toScript('keys', { ref: '' });
  await page.locator('[name=user]').click();
  await page.keyboard.press('ArrowDown');
  check('and once Go asks for none, the keys are the page\'s again', (await sent('key')).length === 4);

  // Fields in no form, sent by a script's own button; and a form that goes after its request.
  await open(`<input name="user" value="asha"><input name="pass" type="password" value="s3cret"><button id="go" type="button">Log in</button>
    <script>document.getElementById('go').addEventListener('click', () => setTimeout(() => document.querySelectorAll('input').forEach((i) => i.remove()), 300));</script>`);
  await until(async () => (await sent('fields')).length >= 1);
  const [u2, p2] = await Promise.all(['user', 'pass'].map(byName));
  await toScript('watch', { refs: [u2.ref, p2.ref] });
  await page.waitForTimeout(400);
  await page.locator('#go').click();
  check('fields in no form are sent by a press on a button in none', await until(async () => (await sent('submitted')).length === 1) &&
    JSON.stringify(await last('submitted')) === JSON.stringify({ form: '', values: { [u2.ref]: 'asha', [p2.ref]: 's3cret' } }), JSON.stringify(await sent('submitted')));
  check('and their going, soon after, is told with what they held', await until(async () => (await sent('gone')).length === 1) &&
    JSON.stringify((await last('gone')).values) === JSON.stringify({ [u2.ref]: 'asha', [p2.ref]: 's3cret' }), JSON.stringify(await sent('gone')));

  // A page built of its own elements keeps its fields in shadow trees: they are told,
  // pressed and filled like any other, and a tree added later is found.
  await open(`<login-box></login-box>
    <script>customElements.define('login-box', class extends HTMLElement { connectedCallback() {
      this.attachShadow({ mode: 'open' }).innerHTML = '<label for="u">Username</label><input id="u" name="user"><inner-pass></inner-pass>'; } });
    customElements.define('inner-pass', class extends HTMLElement { connectedCallback() {
      setTimeout(() => (this.attachShadow({ mode: 'open' }).innerHTML = '<input name="pass" type="password" aria-label="Password">'), 300); } });</script>`);
  check('fields in shadow trees are told, one in a tree inside another and added later too', await until(async () => JSON.stringify(((await last('fields')) || { fields: [] }).fields.map((f) => f.name)) === '["user","pass"]'), JSON.stringify(await last('fields')));
  const [su, sp] = await Promise.all(['user', 'pass'].map(byName));
  check('a field in a shadow tree has its label from that tree', su.label === 'Username' && sp.ariaLabel === 'Password', JSON.stringify([su, sp]));
  await page.locator('login-box').locator('[name=pass]').click();
  check('a press on one tells it', await until(async () => (await last('pressed'))?.ref === sp.ref), JSON.stringify(await last('pressed')));
  await toScript('fill', { values: { [su.ref]: 'asha', [sp.ref]: 'in-the-shadows' } });
  check('and a fill reaches it', (await page.locator('login-box').locator('[name=user]').inputValue()) === 'asha' && (await page.locator('login-box').locator('[name=pass]').inputValue()) === 'in-the-shadows');

  // A field in a frame of the page's own site is told where it is in the page, not in
  // its frame.
  await page.route('https://forms.test/inner', (route) => route.fulfill({ contentType: 'text/html', body: '<!doctype html><body style="margin:0"><input name="deep" style="position:absolute;left:30px;top:40px">' }));
  await open('<div style="height:100px"></div><iframe src="https://forms.test/inner" style="position:absolute;left:200px;top:150px;border:0" width="300" height="200"></iframe>');
  const inner = page.frames().find((f) => f.url().endsWith('/inner'));
  await inner.locator('[name=deep]').click();
  const deep = await until(async () => (await inner.evaluate(() => window.__sent.filter((m) => m[0] === 'pressed').map((m) => m[1]).at(-1))) || null);
  const where = await inner.evaluate(() => window.__sent.filter((m) => m[0] === 'pressed').map((m) => m[1]).at(-1));
  check('a press on a field in a frame of the page\'s own site is told where it is in the page', !!deep && Math.abs(where.box.x - 230) <= 1 && Math.abs(where.box.y - 190) <= 1, JSON.stringify(where));

  // A field kept only for a password manager to find, drawn nowhere, is told as neither
  // seen nor to be used.
  await open('<form><input name="email" type="email"><input name="hint" type="password" style="display:none" tabindex="0"><input name="off" type="password" style="position:absolute;left:-9999px"></form>');
  await until(async () => (await sent('fields')).length >= 1);
  check('a field drawn nowhere is neither seen nor to be used, and one off the page\'s edge isn\'t seen', (await byName('hint')).visible === false && (await byName('hint')).focusable === false && (await byName('off')).visible === false, JSON.stringify(await last('fields')));

  // A field focused the moment it is put on the page, as an autofocus and a quick press
  // are: its page's fields are told first, with that field among them, and then the press.
  await open('<p id="here"></p><script>setTimeout(() => { here.innerHTML = \'<form><input name="user" type="email"><input name="pw" type="password"></form>\'; here.querySelector("[name=pw]").focus(); }, 300);</script>');
  await until(async () => (await sent('pressed')).length >= 1);
  const order = await page.evaluate(() => window.__sent.map((m) => m[0]));
  const quick = (await sent('pressed'))[0];
  const first = (await sent('fields'))[0];
  check('a field focused as it appears is told among its page\'s fields before its press is', order.indexOf('fields') >= 0 && order.indexOf('fields') < order.indexOf('pressed') &&
    !!first && first.fields.some((f) => f.ref === quick.ref && f.name === 'pw'), JSON.stringify([order, quick, first && first.fields.map((f) => [f.ref, f.name])]));

  // A page with no field tells nothing.
  await open('<p>Nothing to fill in here.</p>');
  await page.waitForTimeout(500);
  check('a page with no field tells nothing at all', (await page.evaluate(() => window.__sent.length)) === 0, JSON.stringify(await page.evaluate(() => window.__sent)));

  await browser.close();
  await lib.done();
})().catch(async (e) => {
  console.log(`FAIL the run ended early -> ${e.stack}`);
  process.exit(1);
});
