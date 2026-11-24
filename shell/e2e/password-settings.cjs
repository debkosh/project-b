// Settings' Passwords on the mock engine (docs/passwords.md), in each of the page's two
// layouts and on a phone: the row under Privacy with how many there are, its list page with
// one row per saved login and no password, the search, Show and Copy at a row's end, the
// device's check asked once and held for the page (passed, refused, and with no screen
// lock) until the page is left, the username copied from the row's menu with no check,
// Delete asking first, and the page with none. Then as Linux, where the OS has no check of
// the person: the page names the keyring in its place, Show and Copy ask none, and with a
// keyring that gives no key (`-keyring-locked`) Show says the keyring's line. The logins are
// brought as an import brings them, through the mock's own request.
// It leaves screenshots in build/e2e/password-settings-<layout>-*.png.
// Run: node shell/e2e/password-settings.cjs [page|sections|phone|linux]   (from the repo root,
// after `npm run build` in shell/); with none, each in turn.

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');

const layout = process.argv[2];
if (!layout) {
  let code = 0;
  for (const l of ['page', 'sections', 'phone', 'linux']) {
    console.log(`=== ${l}`);
    code = code || require('child_process').spawnSync(process.execPath, [__filename, l], { stdio: 'inherit' }).status;
  }
  process.exit(code);
}

const lib = require('./lib.cjs');
const { regions, settle, until } = lib;
const check = (name, ok, detail) => lib.check(`${layout}: ${name}`, ok, detail);
const phone = layout === 'phone';
const linux = layout === 'linux';
if (layout === 'sections') fs.writeFileSync(path.join(lib.profile, 'prefs.json'), JSON.stringify({ settingsLayout: 'sections' }));

(async () => {
  const browser = await chromium.launch();
  const errors = [];
  const viewport = phone ? { width: 412, height: 860 } : { width: 1000, height: 680 };
  const context = await browser.newContext({ viewport, hasTouch: phone, deviceScaleFactor: 2 });
  await lib.start(linux ? ['-platform=linux'] : []);

  await lib.fire('logins', JSON.stringify([
    { Origin: 'https://mail.example.org', Username: 'asha@example.org', Password: 'correct horse battery' },
    { Origin: 'https://shop.example.com', Username: 'asha', Password: 'tr0ub4dor&3' },
    { Origin: 'https://shop.example.com', Username: 'work@example.com', Password: 'second-one' },
  ]));

  await lib.fire('menu', 'app.settings');
  await until(async () => (await regions()).settings !== 0);
  const page = await context.newPage();
  lib.watch(page, errors);
  await page.goto(`${lib.base}/settings/?tab=${(await regions()).settings}`);
  await page.getByRole('heading', { name: 'Settings' }).waitFor();
  const shoot = async (name) => {
    await page.mouse.move(1, viewport.height - 1);
    await settle(page);
    await page.screenshot({ path: path.join(lib.out, `password-settings-${layout}-${name}.png`) });
  };
  const sends = (msg) => page.waitForRequest((r) => r.url().includes('/settings/send') && JSON.parse(r.postData() || '{}').t === msg.t, { timeout: 5000 }).then((r) => JSON.parse(r.postData()), () => null);
  const clipboard = async () => (await regions()).clipboard;
  const secret = async () => (await regions()).secretCopied;

  // The row, where Privacy is.
  if (phone) await page.locator('.section-row[data-section="privacy"]').click();
  else await page.locator('.nav [data-section="privacy"]').click();
  await settle(page);
  const row = page.locator('.row', { hasText: 'Passwords' }).first();
  check('Privacy has a Passwords row, with how many there are', await until(async () => (await row.innerText().catch(() => '')).replace(/\s+/g, ' ').includes('3 passwords')), await row.innerText().catch(() => ''));
  await shoot('row');
  const asked = sends({ t: 'passwordsList' });
  await row.getByRole('button').first().click();
  check('the row opens the Passwords page, which asks Go for the list', (await asked) !== null && (await until(() => page.getByRole('heading', { name: 'Passwords', exact: true }).isVisible())));
  const list = page.getByRole('region', { name: 'Passwords', exact: true });
  const rows = list.locator('.row.login');
  const whos = async () => (await rows.locator('.who').allInnerTexts()).map((t) => t.replace(/\s+/g, ' '));
  check('with a row per login, by site and then username, and no password', (await until(async () => (await rows.count()) === 3)) &&
    JSON.stringify(await whos()) === '["mail.example.org asha@example.org","shop.example.com asha","shop.example.com work@example.com"]' && !(await list.innerText()).includes('correct horse'), JSON.stringify(await whos()));
  check('each with its site\'s mark, 32px', JSON.stringify(await rows.locator('.mark').evaluateAll((all) => all.map((m) => [m.textContent, m.offsetWidth, m.offsetHeight]))) === '[["M",32,32],["S",32,32],["S",32,32]]');
  const intro = (await list.locator('.intro').innerText()).replace(/\s+/g, ' ').trim();
  if (linux) check('over them, what keeps them, and the keyring in place of a check, as Linux has none', intro === 'Sealed on this device for this profile. Your keyring unlocks them.', intro);
  else check('over them, what keeps them, and that showing one asks once for the page', /^Sealed on this device for this profile\. Showing or copying a password asks for .+, then not again for 5 minutes while this page stays open\.$/.test(intro), intro);
  check('and under them, how many', (await list.locator('p.after').innerText()) === '3 saved');
  check('listing had the device check nobody', (await sendsNone()) === true);
  await shoot('list');

  // The search keeps the logins whose site or username has what was typed.
  const search = list.getByRole('textbox', { name: 'Search passwords' });
  await search.fill('SHOP');
  check('the search keeps the sites that match', await until(async () => (await rows.count()) === 2 && (await whos()).every((w) => w.startsWith('shop.example.com'))), JSON.stringify(await whos()));
  await search.fill('work@');
  check('and the usernames', await until(async () => JSON.stringify(await whos()) === '["shop.example.com work@example.com"]'), JSON.stringify(await whos()));
  await search.fill('zzz');
  check('and says when none does', await until(async () => (await rows.count()) === 0 && (await list.locator('.row.empty').innerText()) === 'No saved password matches “zzz”'));
  await search.press('Escape');
  check('Escape empties it', await until(async () => (await rows.count()) === 3 && (await search.inputValue()) === ''));

  // One row a login: Show, Copy and a menu at its end, and no password until Show.
  const first = rows.first();
  const text = async () => (await first.innerText()).replace(/\s+/g, ' ');
  const eye = (name = 'Show') => first.getByRole('button', { name: `${name} the password for mail.example.org` });
  const copyButton = first.getByRole('button', { name: 'Copy the password for mail.example.org' });
  const more = first.getByRole('button', { name: 'More for mail.example.org, asha@example.org' });
  const item = (name) => first.getByRole('menuitem', { name, exact: true });
  const checks = async () => ((await regions()).personChecks || []).length;
  check('a row has Show, Copy and a menu at its end, as icons with their names, and no password', (await eye().isVisible()) && (await copyButton.isVisible()) && (await more.isVisible()) &&
    (await first.locator('.secret').count()) === 0 && (await text()) === 'M mail.example.org asha@example.org', await text());
  const sizes = await first.locator('.icon-button').evaluateAll((all) => all.map((b) => { const r = b.getBoundingClientRect(); return Math.round(Math.min(r.width, r.height)); }));
  check(`its three buttons are the design system's icon buttons, ${phone ? 40 : 28}px`, sizes.length === 3 && sizes.every((n) => n === (phone ? 40 : 28)), JSON.stringify(sizes));

  // As Linux the OS has no check of the person: Show and Copy open the password asking none,
  // and Go tells the page of no hold to hide it at.
  if (linux) {
    await eye().click();
    check('Show shows the password on its row, and the device checked nobody', await until(async () => (await first.locator('.secret').innerText().catch(() => '')) === 'correct horse battery' && (await eye('Hide').isVisible())) && (await checks()) === 0, `${await text()} ${await checks()}`);
    await shoot('shown');
    await eye('Hide').click();
    await until(async () => (await first.locator('.secret').count()) === 0);
    await copyButton.click();
    check('Copy puts it on the clipboard, the row says so, and the device checked nobody', await until(async () => (await secret()) === 'correct horse battery' && (await text()).includes('Password copied')) && (await checks()) === 0, `${await text()} ${await checks()}`);
    check('no errors in the console', errors.length === 0, errors.join(' | '));

    // A start whose keyring gives no key, as one the person left locked: Show says the
    // keyring's line on its row, and shows nothing.
    await page.close();
    await lib.stop();
    await lib.start(['-platform=linux', '-keyring-locked']);
    await lib.fire('menu', 'app.settings');
    await until(async () => (await regions()).settings !== 0);
    const locked = await context.newPage();
    await locked.goto(`${lib.base}/settings/?tab=${(await regions()).settings}`);
    await locked.locator('.nav [data-section="privacy"]').click();
    await locked.locator('.row', { hasText: 'Passwords' }).first().getByRole('button').first().click();
    const lockedRow = locked.getByRole('region', { name: 'Passwords', exact: true }).locator('.row.login').first();
    await lockedRow.getByRole('button', { name: 'Show the password for mail.example.org' }).click();
    const lockedText = async () => (await lockedRow.innerText()).replace(/\s+/g, ' ');
    check('with a keyring that gives no key, Show says the keyring\'s line on its row and shows nothing', await until(async () => (await lockedText()).includes("Your keyring is locked. Unlock it when asked; if it doesn't ask, log out and back in.")) &&
      (await lockedRow.locator('.secret').count()) === 0 && (await lockedRow.locator('.said.err').count()) === 1, await lockedText());
    await locked.mouse.move(1, viewport.height - 1);
    await settle(locked);
    await locked.screenshot({ path: path.join(lib.out, `password-settings-${layout}-keyring.png`) });
    await browser.close();
    await lib.stop();
    await lib.done();
    return;
  }

  // Show: the device checks the person, then the password is on the row until it is hidden.
  let told = sends({ t: 'passwordShow' });
  await eye().click();
  check('Show sends Go passwordShow', (await told) !== null);
  check('and after the device\'s check the password shows on its row, under the username, with Hide', await until(async () => (await first.locator('.secret').innerText().catch(() => '')) === 'correct horse battery' && (await eye('Hide').isVisible())) && (await checks()) === 1, `${await text()} ${await checks()}`);
  await shoot('shown');
  await eye('Hide').click();
  check('Hide hides it again', await until(async () => (await first.locator('.secret').count()) === 0));

  // The check holds for the page: Copy, and another login's Show, ask nobody again.
  told = sends({ t: 'passwordCopy' });
  await copyButton.click();
  const copyMsg = await told;
  check('Copy sends Go passwordCopy for the password', !!copyMsg && copyMsg.what === 'password', JSON.stringify(copyMsg));
  check('the password is on the clipboard as a secret, the row says so, the page still hides it, and the device checked nobody again', await until(async () => (await secret()) === 'correct horse battery' && (await clipboard()) !== 'correct horse battery' && (await text()).includes('Password copied')) &&
    (await first.locator('.secret').count()) === 0 && (await checks()) === 1, `${await text()} ${await checks()}`);
  await shoot('copied');
  check('and the line goes after a moment', await until(async () => !(await text()).includes('Password copied'), 4000), await text());
  await rows.nth(1).getByRole('button', { name: 'Show the password for shop.example.com' }).click();
  check('another login shows with no second check, while the first check holds', await until(async () => (await rows.nth(1).locator('.secret').innerText().catch(() => '')) === 'tr0ub4dor&3') && (await checks()) === 1, String(await checks()));
  await rows.nth(1).getByRole('button', { name: 'Hide the password for shop.example.com' }).click();

  // The menu has the rest: the username, copied plainly, and Delete.
  await more.click();
  check('the menu has Copy username and Delete…', (await until(() => item('Copy username').isVisible())) && JSON.stringify(await first.getByRole('menuitem').allInnerTexts()) === '["Copy username","Delete…"]', JSON.stringify(await first.getByRole('menuitem').allInnerTexts()));
  await shoot('menu');
  await item('Copy username').click();
  check('the username copies plainly, and the row says so', await until(async () => (await clipboard()) === 'asha@example.org' && (await text()).includes('Username copied')), await text());

  // The page left and opened again: the check holds no more, and what was shown is hidden.
  await eye().click();
  await until(async () => (await first.locator('.secret').count()) === 1);
  told = sends({ t: 'passwordsClosed' });
  await page.getByRole('button', { name: /^Back to/ }).click();
  check('leaving the page tells Go, which ends the check', (await told) !== null);
  await row.getByRole('button').first().click();
  await until(async () => (await rows.count()) === 3);
  check('back on it, nothing is shown', (await first.locator('.secret').count()) === 0);
  // The device's check refused: nothing shows and nothing is said. With no screen lock, why.
  await lib.fire('personCheck', 'denied');
  await eye().click();
  check('and Show asks the device again: a check the person refuses shows nothing', await until(async () => (await checks()) === 2) && (await page.waitForTimeout(400), true) && (await first.locator('.secret').count()) === 0 && (await text()) === 'M mail.example.org asha@example.org', `${await text()} ${await checks()}`);
  await lib.fire('personCheck', 'noScreenLock');
  await eye().click();
  check('with no screen lock the row says to set one, in --danger', await until(async () => (await text()).includes('Set a screen lock on this device to show passwords')) &&
    (await first.locator('.said.err').evaluate((e) => getComputedStyle(e).color !== getComputedStyle(e.closest('.who').querySelector('.secondary.name')).color)), await text());
  await shoot('no-screen-lock');
  await lib.fire('personCheck', 'verified');

  // Delete, from the menu, asks first.
  await more.click();
  await item('Delete…').click();
  const dialog = page.getByRole('dialog');
  check('Delete asks first, saying what goes and what stays', (await until(() => dialog.isVisible())) && (await dialog.getAttribute('aria-label')) === 'Delete this password?' &&
    (await dialog.locator('p').innerText()).replace(/\s+/g, ' ') === "Your saved password for mail.example.org (asha@example.org) is deleted from this profile. Your account on mail.example.org isn't changed.", await dialog.innerText().catch(() => ''));
  check('its buttons are Cancel and Delete, the danger', JSON.stringify(await dialog.locator('.ui-dialog-buttons button').allInnerTexts()) === '["Cancel","Delete"]' && (await dialog.locator('.text-button.danger').innerText()) === 'Delete');
  await shoot('delete');
  await dialog.getByRole('button', { name: 'Cancel' }).click();
  check('Cancel deletes nothing', (await until(async () => !(await dialog.isVisible()))) && (await rows.count()) === 3);
  await more.click();
  await item('Delete…').click();
  told = sends({ t: 'passwordDelete' });
  await dialog.getByRole('button', { name: 'Delete', exact: true }).click();
  check('Delete sends Go passwordDelete', (await told) !== null);
  check('its row goes, and the count follows', await until(async () => (await rows.count()) === 2 && (await list.locator('p.after').innerText()) === '2 saved'), JSON.stringify(await whos()));

  // The last ones gone: the page says there are none, and the row has no count.
  for (let n = 2; n > 0; n--) {
    await rows.first().getByRole('button', { name: /^More for / }).click();
    await rows.first().getByRole('menuitem', { name: 'Delete…' }).click();
    await dialog.getByRole('button', { name: 'Delete', exact: true }).click();
    await until(async () => (await rows.count()) === n - 1);
  }
  check('with none left the page says so, with no search over it', await until(async () => (await list.locator('.row.empty').innerText().catch(() => '')) === (phone ? 'No saved passwords yet. With sync on, the ones on your other devices come here.' : 'No saved passwords yet. Bring them from another browser under Bring your things over.')) && (await search.count()) === 0, await list.innerText());
  await shoot('empty');
  await page.getByRole('button', { name: /^Back to/ }).click();
  check('and the row under Privacy has no count', await until(async () => { const t = (await row.innerText().catch(() => '')).replace(/\s+/g, ' '); return t.includes('Passwords') && !/\d password/.test(t); }), await row.innerText().catch(() => ''));
  check('no errors in the console', errors.length === 0, errors.join(' | '));
  await browser.close();
  await lib.stop();
  await lib.done();

  // No message of the page's own asks the device to check anyone for the list.
  async function sendsNone() {
    return ((await regions()).personChecks || []).length === 0;
  }
})().catch(async (e) => {
  console.log(`FAIL the run ended early -> ${e.stack}`);
  process.exit(1);
});
