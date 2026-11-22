// Saving a password on a phone, in the Mobile shell on the mock engine
// (docs/password-autofill.md). The mock runs no page, so this test plays the form script of
// a page's frame. Once a sign-in has gone through, the shell's sheet asks: the site, the
// username, the password as dots, which Show shows; Save keeps the login, which the chips
// over the keyboard then offer; a tap outside the sheet is Not now; another password for a
// saved username asks to update; Never for this site ends the offers.
// Run: node shell/e2e/password-save-mobile.cjs   (from the repo root, after `npm run build` in shell/)

const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { regions, until } = lib;
const check = (name, ok, detail) => lib.check(name, ok, detail);

(async () => {
  await lib.start(['-use=mobile']);
  await lib.fire('logins', JSON.stringify([{ Origin: 'https://shop.example', Username: 'asha@example.org', Password: 'correct horse battery' }]));
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser, { viewport: { width: 412, height: 860 }, hasTouch: true });
  const shot = (name) => w.page.screenshot({ path: path.join(lib.out, `password-save-mobile-${name}.png`) });
  const go = async (url) => {
    await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'navigate', text: url }) });
    await until(async () => (await regions()).tabs.some((t) => t.url === url));
  };
  const tell = (frame, kind, payload) => lib.fire('form', JSON.stringify({ frame, kind, payload }));
  const fields = { fields: [
    { ref: 'f1', form: 'F1', tag: 'input', type: 'email', name: 'email', visible: true, focusable: true, empty: true },
    { ref: 'f2', form: 'F1', tag: 'input', type: 'password', name: 'password', visible: true, focusable: true, empty: true },
  ] };
  let docs = 0;
  const page = () => ({ frame: 'main', document: `doc-${docs}`, origin: 'https://shop.example', top: true, secure: true, x: 0, y: 0 });
  const signIn = async (username, password) => {
    docs++;
    await tell(page(), 'fields', fields);
    await tell(page(), 'submitted', { form: 'F1', values: { f1: username, f2: password } });
  };
  const save = w.ui.getByRole('dialog', { name: 'Save password?' });
  const update = w.ui.getByRole('dialog', { name: 'Update password?' });

  await go('https://shop.example/login');
  await signIn('work', 'second-one');
  await w.page.waitForTimeout(2600);
  check('a form sent, with the tab staying where it is, brings no sheet', (await save.count()) === 0);
  await go('https://shop.example/account');
  check('once the tab has gone on, a sheet asks: Save password?', await until(() => save.isVisible(), 6000));
  await w.page.waitForTimeout(400);
  const text = (await save.innerText()).replace(/\s+/g, ' ').trim();
  check('it names the site and the username, the password as dots, and Never for this site, Not now, Save', text === 'Save password? For shop.example work •••••••• Show Never for this site Not now Save', text);
  check('no password is in the shell', !(await w.ui.locator('body').innerHTML()).includes('second-one'));
  const sizes = await save.locator('button').evaluateAll((bs) => bs.map((b) => Math.round(b.getBoundingClientRect().height)));
  check('each of its buttons is at least 48px tall', sizes.every((h) => h >= 48), JSON.stringify(sizes));
  const fits = await save.evaluate((s) => s.scrollWidth <= s.clientWidth && [...s.querySelectorAll('button')].every((b) => b.getBoundingClientRect().right <= innerWidth));
  check('and all of it is inside the window', fits);
  await shot('sheet');
  await save.getByRole('button', { name: 'Show the password' }).tap(); // does: passwordSaveShow
  check('Show shows the password as it was typed', await until(async () => (await save.locator('.secret').count()) === 1 && (await save.locator('.secret').innerText()) === 'second-one'));
  await shot('shown');

  await w.ui.locator('.backdrop').tap({ position: { x: 200, y: 80 } });
  check('a tap outside the sheet is Not now: the sheet goes', await until(async () => (await save.count()) === 0));
  await signIn('work', 'second-one');
  await go('https://shop.example/account?again');
  check('and the next sign-in is asked about again', await until(() => save.isVisible(), 6000));
  await w.page.waitForTimeout(400);
  await save.getByRole('button', { name: 'Save', exact: true }).tap(); // does: passwordSaveAnswer
  check('Save takes the sheet away', await until(async () => (await save.count()) === 0));

  // What was saved is what the chips now offer on the site.
  const bar = w.ui.getByRole('group', { name: 'Saved passwords for shop.example' });
  const fills = async () => ((await regions()).formsSent || []).filter((m) => m.kind === 'fill');
  docs++;
  await tell(page(), 'fields', fields);
  await tell(page(), 'pressed', { ref: 'f2', box: { x: 40, y: 300, w: 300, h: 44 } });
  check('the saved login is a chip at the site\'s fields', await until(() => bar.isVisible()) && (await bar.locator('.chip').allInnerTexts()).some((t) => t.trim().startsWith('work')), JSON.stringify(await bar.locator('.chip').allInnerTexts()));
  await w.page.waitForTimeout(600);
  await bar.locator('.chip', { hasText: 'work' }).tap();
  check('and fills with the password that was saved', await until(async () => (await fills()).length === 1) && (await fills())[0].payload.values.f2 === 'second-one', JSON.stringify(await fills()));

  await signIn('asha@example.org', 'changed-one');
  await go('https://shop.example/account?changed');
  check('a saved username with another password asks: Update password?', await until(() => update.isVisible(), 6000));
  await w.page.waitForTimeout(400);
  const asks = (await update.innerText()).replace(/\s+/g, ' ').trim();
  check('with Update as its button', asks === 'Update password? For shop.example asha@example.org •••••••• Show Never for this site Not now Update', asks);
  await shot('update');
  await update.getByRole('button', { name: 'Never for this site' }).tap();
  await until(async () => (await update.count()) === 0);
  await signIn('third', 'third-one');
  await go('https://shop.example/account?never');
  await w.page.waitForTimeout(2600);
  check('after Never for this site, a new sign-in there brings no sheet', (await save.count()) === 0);

  check('no errors in the console', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.stop();
  await lib.done();
})().catch(async (e) => {
  console.log(`FAIL the run ended early -> ${e.stack}`);
  process.exit(1);
});
