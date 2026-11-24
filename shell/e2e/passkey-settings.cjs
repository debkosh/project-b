// Settings' Passkeys on the mock engine (docs/passkeys.md, Seeing and deleting them), in
// a desktop's sections and on a phone: the row under Passwords with how many
// there are, its list page with a row per passkey, Delete asking first and sending Go the
// passkey, the count following, and the page with none. The passkeys are made as a page
// makes them, through the mock's own request.
// It leaves screenshots in build/e2e/passkey-settings-<layout>-*.png.
// Run: node shell/e2e/passkey-settings.cjs [sections|phone]   (from the repo root,
// after `npm run build` in shell/); with none, each in turn.

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');

const layout = process.argv[2];
if (!layout) {
  let code = 0;
  for (const l of ['sections', 'phone']) {
    console.log(`=== ${l}`);
    code = code || require('child_process').spawnSync(process.execPath, [__filename, l], { stdio: 'inherit' }).status;
  }
  process.exit(code);
}

const lib = require('./lib.cjs');
const { regions, settle, until } = lib;
const check = (name, ok, detail) => lib.check(`${layout}: ${name}`, ok, detail);
const phone = layout === 'phone';

(async () => {
  const browser = await chromium.launch();
  const errors = [];
  const viewport = phone ? { width: 412, height: 860 } : { width: 1000, height: 680 };
  const context = await browser.newContext({ viewport, hasTouch: phone, deviceScaleFactor: 2 });
  await lib.start([]);

  // Two passkeys for example.com, saved as a person saves them: the mock's request, and
  // Save on its card. The mock's requests are numbered from 1.
  const made = async () => ((await regions()).passkeyAnswers || []).filter((a) => a.action === 'made').length;
  const save = async (id, user) => {
    await lib.fire('passkey', `create own user=${user}`);
    await until(async () => {
      await fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'answerPasskey', id, action: 'own', text: '' }) });
      return (await made()) === id;
    });
  };
  await save(1, 'asha');
  await save(2, 'work');

  await lib.fire('menu', 'app.settings');
  await until(async () => (await regions()).settings !== 0);
  const page = await context.newPage();
  lib.watch(page, errors);
  await page.goto(`${lib.base}/settings/?tab=${(await regions()).settings}`);
  await page.getByRole('heading', { name: 'Settings' }).waitFor();
  const shoot = async (name) => {
    await page.mouse.move(1, viewport.height - 1);
    await settle(page);
    await page.screenshot({ path: path.join(lib.out, `passkey-settings-${layout}-${name}.png`) });
  };
  const sends = (msg) => page.waitForRequest((r) => r.url().includes('/settings/send') && JSON.parse(r.postData() || '{}').t === msg.t, { timeout: 5000 }).then((r) => JSON.parse(r.postData()), () => null);

  // The row, in the section that holds it.
  if (phone) await page.locator('.section-row[data-parts~="passkeys"]').click();
  else await page.locator('.nav [data-parts~="passkeys"]').click();
  await settle(page);
  const row = page.locator('.row', { hasText: 'Passkeys' }).first();
  check('Passwords has a Passkeys row, with how many there are', await until(async () => (await row.innerText().catch(() => '')).replace(/\s+/g, ' ').includes('2 passkeys')), await row.innerText().catch(() => ''));
  await shoot('row');
  const asked = sends({ t: 'passkeysList' });
  await row.getByRole('button').first().click();
  check('the row opens the Passkeys page, which asks Go for the list', (await asked) !== null && (await until(() => page.getByRole('heading', { name: 'Passkeys', exact: true }).isVisible())));
  const list = page.getByRole('region', { name: 'Passkeys', exact: true });
  const rows = list.locator('.row.passkey');
  const whos = async () => (await rows.locator('.who').allInnerTexts()).map((t) => t.replace(/\s+/g, ' ')).sort();
  check('with a row per passkey: the site, and the account', (await until(async () => (await rows.count()) === 2)) && JSON.stringify(await whos()) === '["example.com Asha Rao · asha@example.com","example.com Work Rao · work@example.com"]', JSON.stringify(await whos()));
  check('each with its site\'s mark, 32px', JSON.stringify(await rows.locator('.mark').evaluateAll((all) => all.map((m) => [m.textContent, m.offsetWidth, m.offsetHeight]))) === '[["E",32,32],["E",32,32]]');
  const del = rows.first().getByRole('button');
  if (phone) {
    const box = await del.boundingBox();
    check('on a phone Delete is a 40px button named for its passkey, and the date is left out', box.width === 40 && box.height === 40 && /^Delete passkey for example\.com, (Asha|Work) Rao$/.test(await del.getAttribute('aria-label')) && (await rows.locator('.when').count()) === 0, JSON.stringify(box));
  } else {
    check('each says when it was made, with Delete… after it', (await rows.first().locator('.when').innerText()) === 'Made today' && (await del.innerText()) === 'Delete…' && /^Delete passkey for example\.com, (Asha|Work) Rao$/.test(await del.getAttribute('aria-label')));
  }
  check('and under them, that they sync', (await list.locator('p.after').innerText()) === 'Passkeys sync with your other devices when sync is on.');
  await shoot('list');

  // Delete asks first.
  const who = (await rows.first().innerText()).includes('Asha') ? 'Asha Rao' : 'Work Rao';
  await del.click();
  const dialog = page.getByRole('dialog');
  check('Delete asks first, saying what goes and what the site keeps', (await until(() => dialog.isVisible())) && (await dialog.getAttribute('aria-label')) === 'Delete this passkey?' && (await dialog.locator('p').innerText()).replace(/\s+/g, ' ') === `Your passkey for example.com (${who}) is deleted here and on your synced devices. example.com still lists it until you remove it in its settings.`, await dialog.locator('p').innerText().catch(() => ''));
  check('its buttons are Cancel and Delete, the danger', JSON.stringify(await dialog.locator('.ui-dialog-buttons button').allInnerTexts()) === '["Cancel","Delete"]' && (await dialog.locator('.text-button.danger').innerText()) === 'Delete');
  await shoot('delete');
  await dialog.getByRole('button', { name: 'Cancel' }).click();
  check('Cancel deletes nothing', (await until(async () => !(await dialog.isVisible()))) && (await rows.count()) === 2);
  await del.click();
  const sent = sends({ t: 'passkeyDelete' });
  await dialog.getByRole('button', { name: 'Delete', exact: true }).click();
  const msg = await sent;
  check('Delete sends Go the passkey', !!msg && /^[\w-]{22}$/.test(msg.id), JSON.stringify(msg));
  check('and its row goes', await until(async () => (await rows.count()) === 1 && !(await rows.first().innerText()).includes(who)), JSON.stringify(await rows.allInnerTexts()));

  // The last one gone: the page says there are none, and the row has no count.
  await rows.first().getByRole('button').click();
  await dialog.getByRole('button', { name: 'Delete', exact: true }).click();
  check('with none left the page says so', await until(async () => (await rows.count()) === 0 && (await list.locator('.row.empty').innerText().catch(() => '')) === 'No passkeys yet. When a site offers to create one, Project B can save it here.'), await list.innerText());
  await shoot('empty');
  await page.getByRole('button', { name: /^Back to/ }).click();
  check('and the row under Passwords has no count', await until(async () => { const t = (await row.innerText().catch(() => '')).replace(/\s+/g, ' '); return t.includes('Passkeys') && !/\d passkey/.test(t); }), await row.innerText().catch(() => ''));
  check('no errors in the console', errors.length === 0, errors.join(' | '));
  await page.close();
  await lib.stop();

  // GNOME's keyring gives no key: the page says so in place of the list, with Learn more,
  // which says why and what to do, and Try again, which asks again.
  await lib.start(['-platform=linux', '-keyring-locked']);
  await lib.fire('menu', 'app.settings');
  await until(async () => (await regions()).settings !== 0);
  const locked = await context.newPage();
  lib.watch(locked, errors);
  await locked.goto(`${lib.base}/settings/?tab=${(await regions()).settings}`);
  await locked.getByRole('heading', { name: 'Settings' }).waitFor();
  if (phone) await locked.locator('.section-row[data-parts~="passkeys"]').click();
  else await locked.locator('.nav [data-parts~="passkeys"]').click();
  await locked.locator('.row', { hasText: 'Passkeys' }).first().getByRole('button').first().click();
  const shut = locked.getByRole('region', { name: 'Passkeys', exact: true }).locator('.row.keyring');
  check('a keyring that gives no key: the line, Learn more and Try again', await until(async () => (await shut.innerText().catch(() => '')).includes("Your keyring is locked. Unlock it when asked; if it doesn't ask, log out and back in.")) &&
    (await shut.getByRole('button', { name: 'Learn more' }).isVisible()) && (await shut.getByRole('button', { name: 'Try again' }).isVisible()), await shut.innerText().catch(() => ''));
  await locked.mouse.move(1, viewport.height - 1);
  await settle(locked);
  await locked.screenshot({ path: path.join(lib.out, `passkey-settings-${layout}-keyring.png`) });
  await shut.getByRole('button', { name: 'Learn more' }).click();
  const why = locked.getByRole('dialog', { name: 'Why is my keyring locked?' });
  check('Learn more says why, and on GNOME that logging out and in mends a keyring that never asks', await until(() => why.isVisible()) &&
    (await why.innerText()).includes('pressed Cancel, try again and unlock it') && (await why.innerText()).includes("GNOME's keyring has stopped answering") && (await why.innerText()).includes('Log out and back in'), await why.innerText().catch(() => ''));
  check('Close has the focus, so Enter only dismisses it', await until(() => why.getByRole('button', { name: 'Close' }).evaluate((b) => b === document.activeElement)));
  await settle(locked);
  await locked.screenshot({ path: path.join(lib.out, `passkey-settings-${layout}-keyring-why.png`) });
  await locked.keyboard.press('Enter');
  check('Enter shuts it', await until(async () => (await why.count()) === 0));
  await shut.getByRole('button', { name: 'Learn more' }).click();
  const help = locked.waitForRequest((r) => r.url().includes('/settings/send') && JSON.parse(r.postData() || '{}').t === 'openHelp', { timeout: 5000 }).then((r) => JSON.parse(r.postData()), () => null);
  await why.getByRole('button', { name: 'Read more' }).click();
  check('Read more asks Go for the keyring\'s help page, and shuts the dialog', (await help)?.page === 'keyring' && (await until(async () => (await why.count()) === 0)), JSON.stringify(await help));
  const askedAgain = locked.waitForRequest((r) => r.url().includes('/settings/send') && JSON.parse(r.postData() || '{}').t === 'passkeysList', { timeout: 5000 }).then(() => true, () => false);
  await shut.getByRole('button', { name: 'Try again' }).click();
  check('Try again asks Go for the list again, and the line stays while the keyring gives nothing', (await askedAgain) && (await until(() => shut.isVisible())));
  check('no errors in the console with the keyring locked', errors.length === 0, errors.join(' | '));
  await browser.close();
  await lib.stop();
  await lib.done();
})().catch(async (e) => {
  console.log(`FAIL the run ended early -> ${e.stack}`);
  process.exit(1);
});
