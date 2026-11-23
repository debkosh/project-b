// Saving a password, in the Standard shell on the mock engine (docs/password-autofill.md).
// The mock runs no page, so this test plays the form script of a page's frame: its fields,
// then its form sent with what it held. A form sent offers nothing by itself; a sign-in that
// comes back to a sign-in offers nothing; once the tab has gone on and stayed there, a key
// shows in the address field with its panel open: the site, the username, the password as
// dots, which Show shows. Escape closes the panel and the key brings it back. Save keeps the
// login, which the fill list then offers; a saved username with another password asks to
// update it; Never for this site has the site not offered on again, until Settings'
// Passwords takes it off its Never saved list.
// Run: node shell/e2e/password-save.cjs   (from the repo root, after `npm run build` in shell/)

const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { regions, settle, until } = lib;
const check = (name, ok, detail) => lib.check(name, ok, detail);

(async () => {
  await lib.start(['-use=standard']);
  await lib.fire('logins', JSON.stringify([{ Origin: 'https://shop.example', Username: 'asha@example.org', Password: 'correct horse battery' }]));
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser);
  const shot = (name) => w.page.screenshot({ path: path.join(lib.out, `password-save-${name}.png`) });
  const go = async (url) => {
    await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'navigate', text: url }) });
    await until(async () => (await regions()).tabs.some((t) => t.url === url));
  };
  const tell = (frame, kind, payload) => lib.fire('form', JSON.stringify({ frame, kind, payload }));
  const fields = { fields: [
    { ref: 'f1', form: 'F1', tag: 'input', type: 'email', name: 'email', guess: 'username', visible: true, focusable: true, empty: true },
    { ref: 'f2', form: 'F1', tag: 'input', type: 'password', name: 'password', guess: 'password', visible: true, focusable: true, empty: true },
  ] };
  // A sign-in played on the page shown: its fields, then its form sent.
  let docs = 0;
  const page = () => ({ frame: 'main', document: `doc-${docs}`, origin: 'https://shop.example', top: true, secure: true, x: 0, y: 0 });
  const signIn = async (username, password) => {
    docs++;
    await tell(page(), 'fields', fields);
    await tell(page(), 'submitted', { form: 'F1', values: { f1: username, f2: password } });
  };
  const save = w.ui.getByRole('dialog', { name: 'Save password?' });
  const update = w.ui.getByRole('dialog', { name: 'Update password?' });
  const key = w.ui.locator('[data-password-key]');
  const html = () => w.ui.locator('body').innerHTML();

  await go('https://shop.example/login');
  await signIn('work', 'second-one');
  await w.page.waitForTimeout(2600);
  check('a form sent, with the tab staying where it is, offers nothing', (await save.count()) === 0 && (await key.count()) === 0);

  // A sign-in that fails: the page the tab comes to shows the sign-in again.
  await go('https://shop.example/login?failed');
  docs++;
  await tell(page(), 'fields', fields);
  await w.page.waitForTimeout(2600);
  check('a sign-in that comes back to a sign-in offers nothing', (await save.count()) === 0 && (await key.count()) === 0);

  await signIn('work', 'second-one');
  await go('https://shop.example/account');
  check('once the tab has gone on, the address field shows a key with its panel open: Save password?', await until(() => save.isVisible(), 6000) && (await key.getAttribute('aria-expanded')) === 'true' && (await key.getAttribute('aria-label')) === 'Save password');
  const text = (await save.innerText()).replace(/\s+/g, ' ').trim();
  const colours = await key.evaluate((k) => { const probe = document.createElement('i'); probe.style.color = 'var(--accent)'; document.body.append(probe); const accent = getComputedStyle(probe).color; probe.remove(); return [getComputedStyle(k).color, accent]; });
  check('the key is in the accent colour while its offer waits', colours[0] === colours[1], JSON.stringify(colours));
  check('the panel names the site and the username, the password as dots, and Never for this site, Not now, Save', text === 'Save password? For shop.example work •••••••• Show Never for this site Not now Save', text);
  check('no password is in the shell', !(await html()).includes('second-one'));
  check('Save has the focus, so Enter saves', await save.getByRole('button', { name: 'Save', exact: true }).evaluate((b) => b === document.activeElement));
  const at = await save.boundingBox();
  const frame = await w.page.locator('iframe.shell').boundingBox();
  const keyAt = await key.boundingBox();
  check('it hangs from the field\'s end, under the key', at.y >= keyAt.y + keyAt.height && at.x + at.width >= keyAt.x + keyAt.width - 40 && at.x < keyAt.x, JSON.stringify({ at, keyAt }));
  await settle(w.page);
  check('and is the shell\'s own area over the page', await until(async () => (await regions({ x: at.x - frame.x + 20, y: at.y - frame.y + at.height - 20 })).hit === 'shell'));
  await shot('panel');

  await save.getByRole('button', { name: 'Show the password' }).click(); // does: passwordSaveShow
  check('Show shows the password as it was typed', await until(async () => (await save.locator('.secret').count()) === 1 && (await save.locator('.secret').innerText()) === 'second-one'));
  await shot('shown');
  await save.getByRole('button', { name: 'Hide the password' }).click();
  check('and Hide puts the dots back', (await save.locator('.secret').count()) === 0 && (await save.innerText()).includes('••••••••'));

  await w.ui.locator('body').press('Escape');
  check('Escape closes the panel and the key stays', await until(async () => (await save.count()) === 0) && (await key.isVisible()) && (await key.getAttribute('aria-expanded')) === 'false');
  await key.click();
  check('a press on the key brings the panel back', await until(() => save.isVisible()));
  await save.getByRole('button', { name: 'Save', exact: true }).click(); // does: passwordSaveAnswer
  check('Save takes the panel and the key away', await until(async () => (await save.count()) === 0 && (await key.count()) === 0));

  // What was saved is what the fill list now offers on the site.
  const list = w.ui.getByRole('menu', { name: 'Saved passwords for shop.example' });
  const names = async () => (await list.getByRole('menuitem').allInnerTexts()).map((t) => t.replace(/\s+/g, ' ').trim());
  const fills = async () => ((await regions()).formsSent || []).filter((m) => m.kind === 'fill');
  const box = { x: 120, y: 160, w: 280, h: 36 };
  docs++;
  await tell(page(), 'fields', fields);
  await tell(page(), 'pressed', { ref: 'f2', box });
  check('the saved login is offered at the site\'s fields', await until(() => list.isVisible()) && (await names()).includes('work ••••••••'), JSON.stringify(await names()));
  await w.page.waitForTimeout(600);
  await list.getByRole('menuitem', { name: /^work/ }).click();
  check('and fills with the password that was saved', await until(async () => (await fills()).length === 1) && (await fills())[0].payload.values.f2 === 'second-one', JSON.stringify(await fills()));

  // The same sign-in again offers nothing; another password for a saved username asks to update.
  await signIn('work', 'second-one');
  await go('https://shop.example/account?again');
  await w.page.waitForTimeout(2600);
  check('a sign-in with a login saved as it is offers nothing', (await save.count()) === 0 && (await update.count()) === 0 && (await key.count()) === 0);
  await signIn('asha@example.org', 'changed-one');
  await go('https://shop.example/account?changed');
  check('a saved username with another password asks: Update password?', await until(() => update.isVisible(), 6000) && (await key.getAttribute('aria-label')) === 'Update password');
  const asks = (await update.innerText()).replace(/\s+/g, ' ').trim();
  check('with Update as its button', asks === 'Update password? For shop.example asha@example.org •••••••• Show Never for this site Not now Update', asks);
  await shot('update');
  await update.getByRole('button', { name: 'Update', exact: true }).click();
  await until(async () => (await update.count()) === 0);
  docs++;
  await tell(page(), 'fields', fields);
  await tell(page(), 'pressed', { ref: 'f2', box });
  await until(() => list.isVisible());
  await w.page.waitForTimeout(600);
  await list.getByRole('menuitem', { name: /^asha/ }).click();
  check('Update gives the saved login the new password, and adds no second one', await until(async () => (await fills()).length === 2) && (await fills())[1].payload.values.f2 === 'changed-one' && (await names().catch(() => [])).filter((n) => n.startsWith('asha')).length <= 1, JSON.stringify(await fills()));

  // Not now keeps nothing and the next sign-in is offered again; Never ends the offers.
  await signIn('third', 'third-one');
  await go('https://shop.example/account?third');
  await until(() => save.isVisible(), 6000);
  await save.getByRole('button', { name: 'Not now' }).click();
  check('Not now takes the panel and the key away', await until(async () => (await save.count()) === 0 && (await key.count()) === 0));
  await signIn('third', 'third-one');
  await go('https://shop.example/account?third-again');
  check('and the next sign-in is offered again', await until(() => save.isVisible(), 6000));
  await save.getByRole('button', { name: 'Never for this site' }).click();
  check('Never for this site takes the panel and the key away', await until(async () => (await save.count()) === 0 && (await key.count()) === 0));
  await signIn('third', 'third-one');
  await go('https://shop.example/account?never');
  await w.page.waitForTimeout(2600);
  check('after Never for this site, a sign-in there offers nothing', (await save.count()) === 0 && (await key.count()) === 0);

  // Never can be undone: Settings' Passwords lists the site, with Remove.
  await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'passwordsManage' }) });
  const never = w.settings.getByLabel('Never saved', { exact: true });
  check('Settings\' Passwords lists the site under Never saved', await until(() => never.isVisible(), 10000) && (await never.locator('.row').allInnerTexts()).map((t) => t.replace(/\s+/g, ' ').trim()).join('|') === 'shop.example Remove', JSON.stringify(await never.locator('.row').allInnerTexts().catch(() => [])));
  await w.settings.locator('.never-title').scrollIntoViewIfNeeded();
  await w.page.screenshot({ path: path.join(lib.out, 'password-save-never.png') });
  await never.getByRole('button', { name: 'Remove shop.example from never saved' }).click(); // does: passwordNeverRemove
  check('Remove takes the site off, and the list goes with its last site', await until(async () => (await w.settings.getByText('Never saved').count()) === 0));

  check('no errors in the console', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.stop();
  await lib.done();
})().catch(async (e) => {
  console.log(`FAIL the run ended early -> ${e.stack}`);
  process.exit(1);
});
