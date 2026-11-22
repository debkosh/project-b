// Filling a saved login on a phone, in the Mobile shell on the mock engine
// (docs/password-autofill.md). As password-fill.cjs, this test plays the page's form
// script. The shell shows the site's saved logins as chips in a bar over the foot of the
// page, its own area; a tap on a chip fills; a press elsewhere on the page takes the bar
// away; a frame of another site says whose logins they are; Manage at the bar's end
// opens Settings at Passwords.
// Run: node shell/e2e/password-fill-mobile.cjs   (from the repo root, after `npm run build` in shell/)

const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { regions, settle, until } = lib;
const check = (name, ok, detail) => lib.check(name, ok, detail);

(async () => {
  await lib.start(['-use=mobile']);
  await lib.fire('logins', JSON.stringify([
    { Origin: 'https://shop.example', Username: 'asha@example.org', Password: 'correct horse battery' },
    { Origin: 'https://shop.example', Username: 'work', Password: 'second-one' },
    { Origin: 'https://accounts.shop.example', Username: 'sibling', Password: 'third-one' },
    { Origin: 'https://pay.example', Username: 'payer', Password: 'frame-secret' },
  ]));
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser, { viewport: { width: 412, height: 860 }, hasTouch: true });
  const shot = (name) => w.page.screenshot({ path: path.join(lib.out, `password-fill-mobile-${name}.png`) });
  await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'navigate', text: 'https://shop.example/login' }) });
  await until(async () => (await regions()).tabs.some((t) => t.url === 'https://shop.example/login'));

  const page = { frame: 'main', document: 'doc-1', origin: 'https://shop.example', top: true, secure: true, x: 0, y: 0 };
  const tell = (frame, kind, payload) => lib.fire('form', JSON.stringify({ frame, kind, payload }));
  const fields = { fields: [
    { ref: 'f1', form: 'F1', tag: 'input', type: 'email', name: 'email', visible: true, focusable: true, empty: true },
    { ref: 'f2', form: 'F1', tag: 'input', type: 'password', name: 'password', visible: true, focusable: true, empty: true },
  ] };
  const box = { x: 40, y: 300, w: 300, h: 44 };
  const bar = w.ui.getByRole('group', { name: 'Saved passwords for shop.example' });
  const chips = bar.locator('.chip');
  const names = async () => (await chips.allInnerTexts()).map((t) => t.replace(/\s+/g, ' ').trim());
  const fills = async () => ((await regions()).formsSent || []).filter((m) => m.kind === 'fill');

  await tell(page, 'fields', fields);
  await settle(w.page);
  check('a page\'s fields alone bring no bar', (await bar.count()) === 0);
  await tell(page, 'pressed', { ref: 'f2', box });
  check('a press on the password field brings the site\'s logins as chips, its own first, then another host\'s said to be saved for it', await until(() => bar.isVisible()) &&
    JSON.stringify(await names()) === '["asha@example.org","work","sibling saved for accounts.shop.example"]', JSON.stringify(await names()));
  check('no password is in the shell', !(await w.ui.locator('body').innerHTML()).includes('correct horse'));
  const frame = await w.page.locator('iframe.shell').boundingBox();
  const at = await bar.boundingBox();
  const sizes = await chips.evaluateAll((all) => all.map((c) => Math.round(c.getBoundingClientRect().height)));
  check('the bar lies over the foot of the page, the window\'s width, each chip 40px in its 56', Math.abs(at.width - 412) <= 1 && at.height >= 56 && sizes.every((h) => h === 40), JSON.stringify({ at, sizes }));
  await settle(w.page);
  check('and is the shell\'s own area, so a tap on it never reaches the page', await until(async () => (await regions({ x: at.x - frame.x + 30, y: at.y - frame.y + 28 })).hit === 'shell'), (await regions({ x: at.x - frame.x + 30, y: at.y - frame.y + 28 })).hit);
  const manage = bar.getByRole('button', { name: 'Manage passwords' });
  const manageBox = await manage.boundingBox();
  check('Manage is at the bar\'s end, in words, 48px tall, apart from the chips by a hairline', (await manage.innerText()) === 'Manage' && manageBox.height >= 48 && manageBox.width >= 48 &&
    (await manage.evaluate((b) => getComputedStyle(b).borderLeftWidth)) === '1px' && (await manage.locator('svg').count()) === 0, JSON.stringify(manageBox));
  check('the row of chips fades at its end, where one runs off', (await bar.locator('.chips').evaluate((c) => getComputedStyle(c).maskImage || getComputedStyle(c).webkitMaskImage)).includes('linear-gradient'));
  await shot('chips');
  // With the keyboard up, the bars under the page step aside and the bar is at the foot of
  // the window, which is the keyboard's top.
  await lib.fire('keyboard', 'up');
  check('with the keyboard up the bar is at the foot of the window, just above it', await until(async () => { const b = await bar.boundingBox(); return !!b && Math.abs(b.y + b.height - (frame.y + frame.height)) <= 1; }), JSON.stringify(await bar.boundingBox()));
  await shot('keyboard-up');
  // A finger down on a chip takes the keyboard down, and the bar goes down with it before
  // the finger is up: the tap still picks, though the chip is no longer under the finger.
  const cdp = await w.page.context().newCDPSession(w.page);
  const held = await chips.nth(1).boundingBox();
  const finger = { x: Math.round(held.x + held.width / 2), y: Math.round(held.y + held.height / 2) };
  await w.page.waitForTimeout(600);
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [finger] });
  await lib.fire('keyboard', 'down');
  check('the bar goes down with the keyboard while the finger is on a chip', await until(async () => Math.abs((await chips.nth(1).boundingBox()).y - held.y) > 40), JSON.stringify([held, await chips.nth(1).boundingBox()]));
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
  check('and the finger coming up still fills that chip\'s login', await until(async () => (await fills()).length === 1) && JSON.stringify((await fills())[0].payload) === '{"values":{"f1":"work","f2":"second-one"}}', JSON.stringify(await fills()));
  check('and the bar goes', await until(async () => (await bar.count()) === 0));
  await settle(w.page);
  check('and no click follows it, onto what the bar left under the finger: the address field isn\'t opened', (await w.ui.locator('input:focus').count()) === 0);
  // The bar again, for a tap with the keyboard down.
  await tell(page, 'pressed', { ref: 'f2', box });
  await until(() => bar.isVisible());

  // The login just used is the first chip now: the second is the other one.
  await w.page.waitForTimeout(600);
  await chips.nth(1).tap(); // does: passwordPick
  check('a tap on a chip fills its username and password into the fields', await until(async () => (await fills()).length === 2) && JSON.stringify((await fills())[1].payload) === '{"values":{"f1":"asha@example.org","f2":"correct horse battery"}}', JSON.stringify(await fills()));
  check('and the bar goes', await until(async () => (await bar.count()) === 0));

  await tell(page, 'pressed', { ref: 'f1', box });
  await until(() => bar.isVisible());
  await tell(page, 'pressed', { ref: '', box: { x: 0, y: 0, w: 0, h: 0 } });
  check('a press elsewhere on the page takes the bar away, with nothing filled', await until(async () => (await bar.count()) === 0) && (await fills()).length === 2);

  // A frame of another site: its own site's logins, and whose they are.
  const framed = { frame: 'pay', document: 'doc-9', origin: 'https://pay.example', top: false, crossOrigin: true, secure: true, x: 20, y: 200 };
  await tell(framed, 'fields', fields);
  await tell(framed, 'pressed', { ref: 'f1', box: { x: 10, y: 10, w: 200, h: 30 } });
  const other = w.ui.getByRole('group', { name: 'Saved passwords for pay.example' });
  check('a frame of another site is offered its own site\'s logins, and the bar says whose', await until(() => other.isVisible()) &&
    (await other.locator('.whose').innerText()).replace(/\s+/g, ' ').trim() === 'This sign-in is from pay.example, shown inside shop.example.' && JSON.stringify(await other.locator('.chip').allInnerTexts()) === '["payer"]', await other.innerText().catch(() => ''));
  await shot('framed');
  await w.page.waitForTimeout(600);
  await other.getByRole('button', { name: 'Manage passwords' }).tap(); // does: passwordsManage
  check('Manage opens Settings at Passwords, and the bar goes', await until(() => w.settings.getByRole('heading', { name: 'Passwords', exact: true }).isVisible(), 10000) && (await other.count()) === 0);

  check('no errors in the console', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.stop();
  await lib.done();
})().catch(async (e) => {
  console.log(`FAIL the run ended early -> ${e.stack}`);
  process.exit(1);
});
