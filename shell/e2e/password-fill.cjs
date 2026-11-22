// Filling a saved login, in the Standard shell on the mock engine (docs/password-autofill.md).
// The mock runs no page, so this test plays the form script of a page's frame, as an engine
// would carry it: the frame's fields, then a press on one. The shell lists the site's saved
// logins beside the field, outside the page and as its own area; a press on one fills, with
// the password going to the page and never to the shell; a press elsewhere on the page, the
// tab going on, and Escape each take the list away; a frame of another site says whose
// logins they are; Manage passwords… opens Settings at Passwords.
// Run: node shell/e2e/password-fill.cjs   (from the repo root, after `npm run build` in shell/)

const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { regions, settle, until } = lib;
const check = (name, ok, detail) => lib.check(name, ok, detail);

(async () => {
  await lib.start(['-use=standard']);
  await lib.fire('logins', JSON.stringify([
    { Origin: 'https://shop.example', Username: 'asha@example.org', Password: 'correct horse battery' },
    { Origin: 'https://shop.example', Username: 'work', Password: 'second-one' },
    { Origin: 'https://accounts.shop.example', Username: 'sibling', Password: 'third-one' },
    { Origin: 'https://pay.example', Username: 'payer', Password: 'frame-secret' },
    { Origin: 'https://a-very-long-host-name-of-the-single-sign-on.identity.long.example', Username: 'a.person.with.a.very.long.address.for.their.account@mail.long.example', Password: 'long-one' },
    { Origin: 'https://www.long.example', Username: 'short', Password: 'other-long' },
    { Origin: 'http://plain.example', Username: 'plain-user', Password: 'plain-one' },
  ]));
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser);
  const shot = (name) => w.page.screenshot({ path: path.join(lib.out, `password-fill-${name}.png`) });
  await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'navigate', text: 'https://shop.example/login' }) });
  await until(async () => (await regions()).tabs.some((t) => t.url === 'https://shop.example/login'));

  // The page's form script, played: its frame, and what it tells.
  const page = { frame: 'main', document: 'doc-1', origin: 'https://shop.example', top: true, secure: true, x: 0, y: 0 };
  const tell = (frame, kind, payload) => lib.fire('form', JSON.stringify({ frame, kind, payload }));
  const fields = { fields: [
    { ref: 'f1', form: 'F1', tag: 'input', type: 'email', name: 'email', visible: true, focusable: true, empty: true },
    { ref: 'f2', form: 'F1', tag: 'input', type: 'password', name: 'password', visible: true, focusable: true, empty: true },
  ] };
  const box = { x: 120, y: 160, w: 280, h: 36 };
  const list = w.ui.getByRole('menu', { name: 'Saved passwords for shop.example' });
  const rows = list.getByRole('menuitem');
  const names = async () => (await rows.allInnerTexts()).map((t) => t.replace(/\s+/g, ' ').trim());

  await tell(page, 'fields', fields);
  await settle(w.page);
  check('a page\'s fields alone bring no list', (await list.count()) === 0);
  await tell(page, 'pressed', { ref: 'f2', box });
  check('a press on the password field lists the site\'s logins, its own first, then another host\'s said to be saved for it, then Manage passwords…', await until(() => list.isVisible()) &&
    JSON.stringify(await names()) === '["asha@example.org ••••••••","work ••••••••","sibling •••••••• saved for accounts.shop.example","Manage passwords…"]', JSON.stringify(await names()));
  const key = await rows.first().locator('svg.key').evaluate((k) => ({ w: k.getBoundingClientRect().width, stroke: getComputedStyle(k).strokeWidth, box: k.getAttribute('viewBox') }));
  check('each login has the key, a true 16px at a 1.5px stroke', key.w === 16 && key.stroke === '1.5px' && key.box === '0 0 16 16', JSON.stringify(key));
  const lines = await rows.nth(2).evaluate((r) => { const n = r.querySelector('.name').getBoundingClientRect(), d = r.querySelector('.dots').getBoundingClientRect(), f = r.querySelector('.for').getBoundingClientRect(); return { sameLine: Math.abs(n.top - d.top) < 6, under: f.top >= n.bottom - 1, wide: f.right >= d.right - 1, cut: r.querySelector('.for').scrollWidth > r.querySelector('.for').clientWidth }; });
  check('where a login was saved for is on a line of its own under the username, the row\'s width, and not cut off', lines.sameLine && lines.under && lines.wide && !lines.cut, JSON.stringify(lines));
  check('no password is in the shell', !(await w.ui.locator('body').innerHTML()).includes('correct horse') && !(await w.ui.locator('body').innerHTML()).includes('second-one'));
  // Where it is: under the field, in the window, and the shell's own area there.
  const bounds = (await regions()).visible[0].rect;
  const at = await list.boundingBox();
  const frame = await w.page.locator('iframe.shell').boundingBox();
  const left = at.x - frame.x;
  const top = at.y - frame.y;
  check('it hangs under the field, at its left edge', Math.abs(left - (bounds.x + box.x)) <= 1 && Math.abs(top - (bounds.y + box.y + box.h + 4)) <= 1 && at.width >= box.w, JSON.stringify({ left, top, bounds, at }));
  await settle(w.page);
  check('and is the shell\'s own area over the page, so a press on it never reaches the page', await until(async () => (await regions({ x: left + 20, y: top + 20 })).hit === 'shell'), (await regions({ x: left + 20, y: top + 20 })).hit);
  await shot('list');

  // A press in its first half second is no pick; one after it fills.
  await tell(page, 'pressed', { ref: 'f2', box });
  await rows.nth(1).click();
  await w.page.waitForTimeout(200);
  const fills = async () => ((await regions()).formsSent || []).filter((m) => m.kind === 'fill');
  check('a press on a login in the list\'s first half second fills nothing', (await fills()).length === 0 && (await list.isVisible()));
  await w.page.waitForTimeout(500);
  await rows.nth(1).click(); // does: passwordPick
  check('a press on a login fills its username and password into the fields, in that document', await until(async () => (await fills()).length === 1) &&
    JSON.stringify((await fills())[0]) === JSON.stringify({ document: 'doc-1', frame: 'main', kind: 'fill', payload: { values: { f1: 'work', f2: 'second-one' } }, tab: (await regions()).active }), JSON.stringify((await regions()).formsSent));
  check('and the list goes', await until(async () => (await list.count()) === 0));

  // What takes the list away with nothing filled.
  const up = async () => {
    await tell(page, 'pressed', { ref: 'f1', box });
    await until(() => list.isVisible());
  };
  await up();
  await tell(page, 'pressed', { ref: '', box: { x: 0, y: 0, w: 0, h: 0 } });
  check('a press elsewhere on the page takes it away', await until(async () => (await list.count()) === 0));
  await up();
  await w.ui.locator('body').dispatchEvent('pointerdown'); // does: passwordDismiss
  check('a press on the shell outside it takes it away', await until(async () => (await list.count()) === 0));
  await up();
  await rows.first().focus();
  await w.page.keyboard.press('Escape');
  check('Escape takes it away', await until(async () => (await list.count()) === 0));
  await up();
  await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'navigate', text: 'https://shop.example/elsewhere' }) });
  check('the tab going on to another page takes it away', await until(async () => (await list.count()) === 0));
  check('none of those filled anything', (await fills()).length === 1);

  // The page keeps the keyboard: the field's arrow keys, Enter and Escape come through its
  // form script, and Go says which login they are at.
  await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'navigate', text: 'https://shop.example/login' }) });
  await up();
  const asked = ((await regions()).formsSent || []).filter((m) => m.kind === 'keys').at(-1);
  check('while the list shows, Go asks the form script for that field\'s keys', JSON.stringify(asked.payload) === '{"ref":"f1"}', JSON.stringify(asked));
  const pressKey = (k) => tell(page, 'key', { ref: 'f1', key: k });
  const current = async () => (await list.locator('[aria-current]').allInnerTexts()).map((t) => t.replace(/\s+/g, ' ').trim());
  check('no login is chosen as the list comes up', (await current()).length === 0);
  await pressKey('ArrowDown');
  check('ArrowDown in the field goes to the first login, marked as a row under the pointer is', await until(async () => (await current()).length === 1 && (await current())[0] === (await names())[0]) && (await names())[0].startsWith('work') &&
    (await list.locator('.login.at').evaluate((r) => getComputedStyle(r).backgroundColor)) !== 'rgba(0, 0, 0, 0)', JSON.stringify(await current()));
  // One current login: the pointer moving over a row takes it there, and a key takes it on.
  await rows.nth(1).hover(); // does: passwordAt
  check('the pointer moving over a login makes it the one, and no other is marked', await until(async () => (await current()).length === 1 && (await current())[0] === (await names())[1]) &&
    (await list.locator('.login').evaluateAll((all) => all.filter((r) => getComputedStyle(r).backgroundColor !== 'rgba(0, 0, 0, 0)').length)) === 1, JSON.stringify(await current()));
  await pressKey('ArrowUp');
  check('a key moves it on from there, the pointer still lying on its row, and still one row is marked', await until(async () => (await current())[0] === (await names())[0]) &&
    (await list.locator('.login').evaluateAll((all) => all.filter((r) => getComputedStyle(r).backgroundColor !== 'rgba(0, 0, 0, 0)').length)) === 1, JSON.stringify(await current()));
  await pressKey('ArrowUp');
  check('ArrowUp from the first goes round to the last', await until(async () => (await current())[0]?.startsWith('sibling')), JSON.stringify(await current()));
  await shot('keys');
  await pressKey('Enter');
  check('Enter fills the login the keys are at, and the list goes', await until(async () => (await fills()).length === 2) && (await fills())[1].payload.values.f1 === 'sibling' && await until(async () => (await list.count()) === 0), JSON.stringify(await fills()));
  await up();
  await pressKey('Escape');
  check('Escape in the field takes the list away, with nothing more filled', await until(async () => (await list.count()) === 0) && (await fills()).length === 2);
  check('and the field\'s keys are the page\'s again', JSON.stringify(((await regions()).formsSent || []).filter((m) => m.kind === 'keys').at(-1).payload) === '{"ref":""}');

  // A frame of another site in the page: its own site's logins, and whose they are, placed
  // by where the frame is in the page.
  const framed = { frame: 'pay', document: 'doc-9', origin: 'https://pay.example', top: false, crossOrigin: true, secure: true, x: 300, y: 200 };
  await tell(framed, 'fields', fields);
  await tell(framed, 'pressed', { ref: 'f1', box: { x: 10, y: 10, w: 200, h: 30 } });
  const other = w.ui.getByRole('menu', { name: 'Saved passwords for pay.example' });
  check('a frame of another site is offered its own site\'s logins, and the list says whose', await until(() => other.isVisible()) &&
    (await other.innerText()).replace(/\s+/g, ' ').startsWith('This sign-in is from pay.example, shown inside shop.example. payer') && (await other.locator('.whose b').innerText()) === 'pay.example', await other.innerText().catch(() => ''));
  const far = await other.boundingBox();
  check('placed by where the frame is in the page', Math.abs(far.x - frame.x - (bounds.x + 310)) <= 1 && Math.abs(far.y - frame.y - (bounds.y + 210 + 30 + 4)) <= 1, JSON.stringify(far));
  await shot('framed');
  // A frame with a certificate that doesn't check is offered nothing.
  await tell({ ...framed, frame: 'bad', document: 'doc-10', secure: false }, 'fields', fields);
  await tell({ ...framed, frame: 'bad', document: 'doc-10', secure: false }, 'pressed', { ref: 'f1', box });
  check('a frame whose certificate doesn\'t check is offered nothing, and the list there goes', await until(async () => (await other.count()) === 0));

  // A long username and a long host: the username is cut at its end, the host in its
  // middle, keeping the site.
  await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'navigate', text: 'https://www.long.example/' }) });
  await until(async () => (await regions()).tabs.some((t) => t.url === 'https://www.long.example/'));
  const long = { ...page, frame: 'long', document: 'doc-20', origin: 'https://www.long.example' };
  await tell(long, 'fields', fields);
  await tell(long, 'pressed', { ref: 'f1', box });
  const longList = w.ui.getByRole('menu', { name: 'Saved passwords for www.long.example' });
  check('a host too long for the row loses its middle and keeps its site', await until(() => longList.isVisible()) &&
    (await longList.locator('.for').innerText()) === 'saved for a-very-lo…y.long.example' && (await longList.locator('.for').evaluate((f) => f.scrollWidth <= f.clientWidth)) &&
    (await longList.locator('.for').getAttribute('title')) === 'a-very-long-host-name-of-the-single-sign-on.identity.long.example', await longList.innerText().catch(() => ''));
  check('and a username too long for it is cut at its end, the dots still beside it', await longList.getByRole('menuitem').nth(1).evaluate((r) => { const n = r.querySelector('.name'), d = r.querySelector('.dots').getBoundingClientRect(); return n.scrollWidth > n.clientWidth && d.right <= r.getBoundingClientRect().right && d.width > 20; }));
  await shot('long');
  await tell(long, 'pressed', { ref: '', box: { x: 0, y: 0, w: 0, h: 0 } });
  await until(async () => (await longList.count()) === 0);

  // A page that isn't https: its own logins are offered, under a line that says so.
  await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'navigate', text: 'http://plain.example/' }) });
  await until(async () => (await regions()).tabs.some((t) => t.url === 'http://plain.example/'));
  const plain = { ...page, frame: 'plain', document: 'doc-30', origin: 'http://plain.example', secure: false };
  await tell(plain, 'fields', fields);
  await tell(plain, 'pressed', { ref: 'f2', box });
  const plainList = w.ui.getByRole('menu', { name: 'Saved passwords for plain.example' });
  check('on a page that isn\'t https the list says so, over the site\'s own logins', await until(() => plainList.isVisible()) &&
    (await plainList.innerText()).replace(/\s+/g, ' ').startsWith('This page isn\'t secure: what you fill could be read on the way. plain-user'), await plainList.innerText().catch(() => ''));
  check('and a list on an https page has no such line', !(await list.count()) || !(await list.innerText()).includes('secure'));
  await shot('not-secure');
  await tell(plain, 'pressed', { ref: '', box: { x: 0, y: 0, w: 0, h: 0 } });
  await until(async () => (await plainList.count()) === 0);
  await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'navigate', text: 'https://shop.example/login' }) });
  await until(async () => (await regions()).tabs.some((t) => t.url === 'https://shop.example/login'));

  // Manage passwords… opens Settings at Passwords.
  await tell(framed, 'pressed', { ref: 'f1', box: { x: 10, y: 10, w: 200, h: 30 } });
  await until(() => other.isVisible());
  await w.page.waitForTimeout(600);
  await other.getByRole('menuitem', { name: 'Manage passwords…' }).click(); // does: passwordsManage
  check('Manage passwords… opens Settings at Passwords', await until(() => w.settings.getByRole('heading', { name: 'Passwords', exact: true }).isVisible(), 10000) && (await other.count()) === 0);

  check('no errors in the console', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.stop();
  await lib.done();
})().catch(async (e) => {
  console.log(`FAIL the run ended early -> ${e.stack}`);
  process.exit(1);
});
