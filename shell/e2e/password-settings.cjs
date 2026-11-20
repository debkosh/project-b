// Settings' Passwords on the mock engine (docs/passwords.md), in each of the page's two
// layouts, and on a phone that it isn't there: the row under Privacy with how many there are, its list page with
// a row per saved login and no password, the search, a row opening in place, Show and Copy
// each after the device's check (passed, refused, and with no screen lock), a username
// copied with no check, Delete asking first, and the page with none. The logins are brought
// as an import brings them, through the mock's own request.
// It leaves screenshots in build/e2e/password-settings-<layout>-*.png.
// Run: node shell/e2e/password-settings.cjs [page|sections|phone]   (from the repo root,
// after `npm run build` in shell/); with none, each in turn.

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');

const layout = process.argv[2];
if (!layout) {
  let code = 0;
  for (const l of ['page', 'sections', 'phone']) {
    console.log(`=== ${l}`);
    code = code || require('child_process').spawnSync(process.execPath, [__filename, l], { stdio: 'inherit' }).status;
  }
  process.exit(code);
}

const lib = require('./lib.cjs');
const { regions, settle, until } = lib;
const check = (name, ok, detail) => lib.check(`${layout}: ${name}`, ok, detail);
const phone = layout === 'phone';
if (layout === 'sections') fs.writeFileSync(path.join(lib.profile, 'prefs.json'), JSON.stringify({ settingsLayout: 'sections' }));

(async () => {
  const browser = await chromium.launch();
  const errors = [];
  const viewport = phone ? { width: 412, height: 860 } : { width: 1000, height: 680 };
  const context = await browser.newContext({ viewport, hasTouch: phone, deviceScaleFactor: 2 });
  await lib.start([]);

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
  // A phone has no import, so nothing brings a password to it yet: it has no Passwords row.
  if (phone) {
    check('a phone has Passkeys under Privacy, and no Passwords row', (await until(() => page.locator('.row', { hasText: 'Passkeys' }).first().isVisible())) && (await row.count()) === 0, await page.locator('main').innerText().catch(() => ''));
    check('no errors in the console', errors.length === 0, errors.join(' | '));
    await browser.close();
    await lib.stop();
    await lib.done();
    return;
  }
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
  check('over them, what keeps them and what showing one asks', /^Sealed on this device for this profile\. Showing or copying a password asks for .+ first\.$/.test((await list.locator('.intro').innerText()).trim()), await list.locator('.intro').innerText());
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

  // A row opens in place: the username, the password hidden, when it was saved.
  const first = rows.first();
  await first.click();
  const panel = list.locator('.panel');
  const text = async () => (await panel.innerText()).replace(/\s+/g, ' ');
  check('a row opens under itself: the username, the password hidden, and when it was saved', (await first.getAttribute('aria-expanded')) === 'true' && (await until(() => panel.isVisible())) &&
    /^Username asha@example\.org Copy Password •+ Show Copy Saved today Delete…$/.test(await text()), await text());
  await shoot('open');

  // Show: the device checks the person, then the password is here until it is hidden.
  let told = sends({ t: 'passwordShow' });
  await panel.getByRole('button', { name: 'Show the password for mail.example.org' }).click();
  check('Show sends Go passwordShow', (await told) !== null);
  check('and after the device\'s check the password shows, with Hide', await until(async () => (await panel.locator('.secret').innerText()) === 'correct horse battery' && (await panel.getByRole('button', { name: 'Hide the password for mail.example.org' }).isVisible())), await text());
  await shoot('shown');
  await panel.getByRole('button', { name: 'Hide the password for mail.example.org' }).click();
  check('Hide hides it again', await until(async () => /^•+$/.test(await panel.locator('.secret').innerText())));

  // Copy: Go puts it on the clipboard after the check, and the page never has it.
  told = sends({ t: 'passwordCopy' });
  await panel.getByRole('button', { name: 'Copy the password for mail.example.org' }).click();
  const copyMsg = await told;
  check('Copy sends Go passwordCopy for the password', !!copyMsg && copyMsg.what === 'password', JSON.stringify(copyMsg));
  check('the password is on the clipboard as a secret, the row says so, and the page still hides it', await until(async () => (await secret()) === 'correct horse battery' && (await clipboard()) !== 'correct horse battery' && (await text()).includes('Password copied')) && /^•+$/.test(await panel.locator('.secret').innerText()), await text());
  await panel.getByRole('button', { name: 'Copy the username for mail.example.org' }).click();
  check('the username copies too, plainly', await until(async () => (await clipboard()) === 'asha@example.org' && (await text()).includes('Username copied')), await text());
  check('and the line goes back to when it was saved', await until(async () => (await text()).includes('Saved today'), 4000), await text());

  // The device's check refused: nothing shows and nothing is said. With no screen lock, why.
  await lib.fire('personCheck', 'denied');
  await panel.getByRole('button', { name: 'Show the password for mail.example.org' }).click();
  await page.waitForTimeout(600);
  check('a check the person refuses shows nothing', /^•+$/.test(await panel.locator('.secret').innerText()) && (await text()).includes('Saved today'), await text());
  await lib.fire('personCheck', 'noScreenLock');
  await panel.getByRole('button', { name: 'Show the password for mail.example.org' }).click();
  check('with no screen lock the row says to set one, in --danger', await until(async () => (await text()).includes('Set a screen lock on this device to show passwords')) &&
    (await panel.locator('.foot .err').evaluate((e) => getComputedStyle(e).color !== getComputedStyle(e.closest('.panel').querySelector('.k')).color)), await text());
  await shoot('no-screen-lock');
  await lib.fire('personCheck', 'verified');

  // Another row opened closes this one, and forgets what it showed.
  await panel.getByRole('button', { name: 'Show the password for mail.example.org' }).click();
  await until(async () => (await panel.locator('.secret').innerText()) === 'correct horse battery');
  await rows.nth(1).click();
  check('opening another row closes this one', await until(async () => (await first.getAttribute('aria-expanded')) === 'false' && (await rows.nth(1).getAttribute('aria-expanded')) === 'true' && (await list.locator('.panel').count()) === 1));
  await first.click();
  check('and what it showed is hidden when it opens again', await until(async () => /^•+$/.test(await panel.locator('.secret').innerText())));

  // Delete asks first.
  await panel.getByRole('button', { name: 'Delete the password for mail.example.org, asha@example.org' }).click();
  const dialog = page.getByRole('dialog');
  check('Delete asks first, saying what goes and what stays', (await until(() => dialog.isVisible())) && (await dialog.getAttribute('aria-label')) === 'Delete this password?' &&
    (await dialog.locator('p').innerText()).replace(/\s+/g, ' ') === "Your saved password for mail.example.org (asha@example.org) is deleted from this profile. Your account on mail.example.org isn't changed.", await dialog.innerText().catch(() => ''));
  check('its buttons are Cancel and Delete, the danger', JSON.stringify(await dialog.locator('.ui-dialog-buttons button').allInnerTexts()) === '["Cancel","Delete"]' && (await dialog.locator('.text-button.danger').innerText()) === 'Delete');
  await shoot('delete');
  await dialog.getByRole('button', { name: 'Cancel' }).click();
  check('Cancel deletes nothing', (await until(async () => !(await dialog.isVisible()))) && (await rows.count()) === 3);
  await panel.getByRole('button', { name: 'Delete the password for mail.example.org, asha@example.org' }).click();
  told = sends({ t: 'passwordDelete' });
  await dialog.getByRole('button', { name: 'Delete', exact: true }).click();
  check('Delete sends Go passwordDelete', (await told) !== null);
  check('its row goes, and the count follows', await until(async () => (await rows.count()) === 2 && (await list.locator('p.after').innerText()) === '2 saved' && (await list.locator('.panel').count()) === 0), JSON.stringify(await whos()));

  // The last ones gone: the page says there are none, and the row has no count.
  for (let n = 2; n > 0; n--) {
    await rows.first().click();
    await list.locator('.panel').getByRole('button', { name: /^Delete the password/ }).click();
    await dialog.getByRole('button', { name: 'Delete', exact: true }).click();
    await until(async () => (await rows.count()) === n - 1);
  }
  check('with none left the page says so, with no search over it', await until(async () => (await list.locator('.row.empty').innerText().catch(() => '')) === 'No saved passwords yet. Bring them from another browser under Bring your things over.') && (await search.count()) === 0, await list.innerText());
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
