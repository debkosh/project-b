// Settings' Sync on the mock engine (docs/sync.md), against a real sync server: synckosh's,
// which the dev server runs with -sync-server and sync reaches as memd.example.com. The card
// in Profile, in a desktop's sections and on a phone, search finding it, the
// set-up Dialog through a new bucket and one in use, every refusal (a short passphrase, two
// that differ, a wrong one, an http address, a server out of reach, an address that isn't a
// bucket, a token refused), Suggest one, the connected rows (Sync now, syncing with Cancel
// and Retry, out of reach, the server's own failure, a failure of another kind, stalled,
// signed out and Reconnect), Change sync passphrase and Turn off;
// a profile whose keys don't open here, and Set up again; and the project-b://sync link,
// from another app and pressed in a page, which sends nothing to the server before Connect,
// and from a page's own script, which does nothing.
// Every state is saved in light and dark under build/sync-screens/<layout>/, with an
// overview of each, for the ux review.
// Run: node shell/e2e/sync.cjs [sections|phone]   (from the repo root, after `npm run
// build` in shell/); with none, each in turn.

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
const { check, regions, settle, until } = lib;
const phone = layout === 'phone';

// A listed profile, Personal, as the mocks show it; on the mock engine its keys are wrapped by
// a folder.key of its own, as a folder named outright's are.
const root = path.dirname(lib.profile);
fs.writeFileSync(path.join(root, 'profiles.json'), JSON.stringify({ profiles: [{ id: 'profile', name: 'Personal', color: '#4f9e9e', template: 'Personal' }] }));
// The frame colour the mocks are drawn in.
fs.writeFileSync(path.join(lib.profile, 'prefs.json'), JSON.stringify({ frameColor: '#4f9e9e' }));
const args = ['-sync-server', `-root=${root}`, '-id=profile'];

// The sync server: its address, how many requests it has had, and how it answers.
const server = async (form) => (await fetch(`${lib.base}/mock/sync`, form ? { method: 'POST', body: new URLSearchParams(form) } : {})).json();
const requests = async () => (await server()).requests;
const HOST = 'memd.example.com';
const link = (address) => `project-b://sync?address=${encodeURIComponent(address)}`;

// Each state's pictures, light and dark, by the mocks' numbers.
const screens = path.join(lib.root, 'build/sync-screens', layout);
fs.rmSync(screens, { recursive: true, force: true });
fs.mkdirSync(screens, { recursive: true });
const taken = [];

(async () => {
  const browser = await chromium.launch();
  const errors = [];
  const viewport = phone ? { width: 412, height: 860 } : { width: 1000, height: 680 };
  const context = await browser.newContext({ viewport, hasTouch: phone, deviceScaleFactor: 2 });
  let page;

  // The settings tab, opened as the menu opens it, and a page of its own on it.
  const openSettings = async () => {
    await lib.fire('menu', 'app.settings');
    await until(async () => (await regions()).settings !== 0);
    page = await context.newPage();
    lib.watch(page, errors);
    await page.goto(`${lib.base}/settings/?tab=${(await regions()).settings}`);
    await page.getByRole('heading', { name: 'Settings' }).waitFor();
  };
  const card = () => page.getByRole('region', { name: 'Sync', exact: true });
  const dialog = () => page.getByRole('dialog');
  const field = (label) => dialog().getByLabel(label, { exact: true });
  const errorUnder = (label) => field(label).locator('xpath=ancestor::div[contains(@class,"ui-dialog-part")][1]').locator('.text-field-error');
  const button = (name, within = dialog()) => within.getByRole('button', { name, exact: true });
  const serverRow = () => card().locator('.row').first();
  const focused = (label) => field(label).evaluate((el) => document.activeElement === el);
  const shown = (locator) => until(() => locator.isVisible());
  // Whether the page sends Go this message next, as a press should: the mock engine's
  // chrome.send posts each one to /settings/send.
  const sends = (msg) => page.waitForRequest((r) => r.url().includes('/settings/send') && JSON.parse(r.postData() || '{}').t === msg.t, { timeout: 5000 }).then(() => true, () => false);

  // Sync's card, as a person goes to it: Profile, the section that holds it.
  const showSync = async () => {
    if (phone) await page.locator('.section-row[data-section="profile"]').click();
    else await page.locator('.nav [data-section="profile"]').click();
    await settle(page);
  };

  // A state's pictures, in each theme, the page at Profile's section, as the
  // mocks have it.
  const shot = async (n, id) => {
    // The pointer goes aside, so nothing shows as hovered.
    await page.mouse.move(1, viewport.height - 1);
    const name = `${String(n).padStart(2, '0')}-${id}`;
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await settle(page);
      await page.screenshot({ path: path.join(screens, `${name}-${scheme}.png`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
    taken.push(name);
  };

  await lib.start(args);
  // A phone has a QR scanner, which a person leaves until the test says what it reads.
  if (phone) await lib.fire('scanner', '');
  const address = (await server()).address;
  await openSettings();
  await showSync();

  // Where it is: a card under Your data, after the profile's and before Import (none on a
  // phone), and its address, sync, goes to it.
  const order = await page.evaluate(() => {
    const sync = document.querySelector('[data-part="sync"]');
    const before = sync?.previousElementSibling;
    return { before: before?.textContent.includes('Profile name'), after: sync?.nextElementSibling?.getAttribute('aria-label') || sync?.nextElementSibling?.textContent.slice(0, 20) };
  });
  check('Sync is a card of its own after the profile\'s', order.before === true, JSON.stringify(order));
  check(phone ? 'and on a phone Run setup again follows it' : 'and Import follows it', phone ? order.after.includes('Run setup') : order.after === 'Import', JSON.stringify(order));
  check('off, it has one row, Set up sync, and its line', (await card().locator('.row').count()) === 1 && (await card().innerText()).includes('Bookmarks, history, passwords and passkeys, through your own sync server'));
  await shot(1, 'off');

  // Search finds it by sync and by passphrase. On a phone the field is over the list of
  // sections, which back goes to.
  if (phone) await page.getByRole('button', { name: 'Back to Settings' }).click();
  for (const word of ['sync', 'passphrase']) {
    const search = page.getByLabel('Search settings');
    await search.fill(word);
    check(`searching for ${word} finds Set up sync`, await shown(button('Set up…', card())));
    await search.fill('');
  }
  await showSync();

  // Setting up: the address, as typed.
  await button('Set up…', card()).click();
  check('Set up… opens the Dialog, Set up sync, the address field focused', await shown(dialog()) && (await dialog().getAttribute('aria-label')) === 'Set up sync' && (await focused('Server address')));
  // The server to start with: its address is there to change, and a link goes to the page
  // that gives a bucket on it, in a tab, the Dialog going with it.
  const START = 'https://memd.debkosh.com/sync';
  check('the address starts as the server to start with, for a person to change', (await field('Server address').inputValue()) === START);
  check('with what that server is, and Get a free sync bucket', (await dialog().innerText()).includes("Sync through your own server, or try ours.") &&
    (await dialog().innerText()).includes("memd.debkosh.com is free to try. It can't read your data, and you can host your own.") && (await shown(button('Get a free sync bucket'))));
  await shot(49, 'start');
  // Connect with that address and no token is no dead end: what the server said, and the way
  // to a token, which does what the button above does.
  await button('Connect').click();
  const way = errorUnder('Token').getByRole('button', { name: 'Get a free sync bucket' });
  check('Connect on the server to start with, with no token, says it needs one and how to get one', await until(async () => (await errorUnder('Token').count()) === 1 &&
    (await errorUnder('Token').innerText()).replace(/\s+/g, ' ') === 'This server needs a token. Get a free sync bucket to make one.') && (await way.count()) === 1, await dialog().innerText());
  await shot(50, 'start-needs-token');
  await way.click(); // does: syncGetBucket
  check('Get a free sync bucket, there as above, opens its page in a tab, and the Dialog goes', await until(async () => (await dialog().count()) === 0 && (await regions()).tabs.some((t) => t.url === 'https://memd.debkosh.com/synckosh')),
    JSON.stringify((await regions()).tabs.map((t) => t.url)));
  await button('Set up…', card()).click();
  check('with the token as an option', (await field('Token').getAttribute('placeholder')) === 'Only if your server gave you one');
  // The token is a secret: hidden as it is typed, with Show once there is one to show.
  check('an empty Token field has no Show', (await field('Token').getAttribute('type')) === 'password' && (await button('Show', dialog()).count()) === 0);
  await field('Token').fill('TOKEN-k3v9x2m7q8rd');
  check('a token is hidden as it is typed', (await field('Token').getAttribute('type')) === 'password' && (await shown(button('Show', dialog()))));
  await button('Show', dialog()).click();
  check('Show shows it, and reads Hide', (await field('Token').getAttribute('type')) === 'text' && (await field('Token').inputValue()) === 'TOKEN-k3v9x2m7q8rd' && (await shown(button('Hide', dialog()))));
  await button('Hide', dialog()).click();
  check('Hide hides it again', (await field('Token').getAttribute('type')) === 'password');
  await field('Token').fill('');
  await field('Server address').fill(`${address}/sync/7kq2mx9pab4c`);
  await shot(3, 'address');
  await field('Server address').fill(`http://${HOST}/sync/7kq2mx9pab4c`);
  check('an http address is refused as it is typed, under the field, the text kept', await until(async () => (await errorUnder('Server address').innerText()) === 'Sync needs an https address') &&
    (await field('Server address').inputValue()) === `http://${HOST}/sync/7kq2mx9pab4c`);
  await shot(4, 'address-error');
  const before = await requests();
  await button('Connect').click();
  check('and Connect sends nothing for it', (await requests()) === before && (await dialog().getAttribute('aria-label')) === 'Set up sync');

  // Connecting: the button says so and waits; Cancel still works.
  await server({ slow: '1' });
  await field('Server address').fill(`${address}/sync/7kq2mx9pab4c`);
  await button('Connect').click();
  check('Connect reads Connecting… and waits', await shown(button('Connecting…')) && (await button('Connecting…').isDisabled()));
  await shot(15, 'connecting');
  await button('Cancel').click();
  check('Cancel still works', await until(async () => (await dialog().count()) === 0));
  await server({ slow: '0' });

  // What the server says, under the field it is about, the text kept.
  const refusal = async (addr, token, under, words, n, id) => {
    await button('Set up…', card()).click();
    await field('Server address').fill(addr);
    if (token) await field('Token').fill(token);
    await button('Connect').click();
    const ok = await until(async () => (await errorUnder(under).count()) === 1 && (await errorUnder(under).innerText()) === words);
    check(`${words}, under ${under}, the text kept`, ok && (await field('Server address').inputValue()) === addr && (await field('Token').inputValue()) === token, ok ? '' : await dialog().innerText());
    if (n) await shot(n, id);
    await button('Cancel').click();
  };
  if (!phone) await refusal('https://127.0.0.1:1/sync/7kq2mx9pab4c', '', 'Server address', "Couldn't reach 127.0.0.1:1");
  await server({ down: '1' });
  await refusal(`${address}/sync/7kq2mx9pab4c`, '', 'Server address', `Couldn't reach ${HOST}`, 16, 'address-unreachable');
  await server({ down: '0' });
  await refusal(`${address}/notes/sync/7kq2mx9pab4c`, '', 'Server address', "That address isn't a sync bucket", 17, 'not-bucket');
  // An address is taken as it is given, whatever its path: the server says whether it wants a
  // token, and the page says so under Token.
  await refusal(`${address}/sync`, '', 'Token', 'This server needs a token');
  await server({ refuse: 'TOKEN-k3v9x2m7q8rd' });
  await refusal(`${address}/sync`, 'TOKEN-k3v9x2m7q8rd', 'Token', "The server didn't accept that token", 18, 'token-refused');

  // A new bucket: a passphrase, chosen twice.
  const bucket = `${address}/sync`;
  const token = `tok-${layout}`;
  await button('Set up…', card()).click();
  await field('Server address').fill(bucket);
  await field('Token').fill(token);
  await button('Connect').click();
  check('an empty bucket asks for a new passphrase, twice', await until(async () => (await dialog().getAttribute('aria-label')) === 'Choose a sync passphrase') &&
    (await dialog().innerText()).includes(`Nothing is synced on ${HOST} yet. Every device you add asks for this passphrase.`) && (await field('Type it again').count()) === 1);
  check('the passphrase field has the focus', await focused('Passphrase'));
  // The room a phone's keyboard leaves: the dialog's title and buttons stay in sight, and
  // only its body scrolls between them.
  await page.setViewportSize({ width: viewport.width, height: 380 });
  const room = await dialog().evaluate((el) => {
    const inSight = (n) => { const r = n.getBoundingClientRect(); return r.top >= 0 && r.bottom <= innerHeight; };
    const body = el.querySelector('.ui-dialog-body');
    return { title: inSight(el.querySelector('.ui-dialog-title')), buttons: inSight(el.querySelector('.ui-dialog-buttons')), scrolls: body.scrollHeight > body.clientHeight, whole: el.scrollHeight <= el.clientHeight };
  });
  check('with little room, as over a keyboard, the dialog keeps its title and buttons in sight and only its body scrolls', room.title && room.buttons && room.scrolls && room.whole, JSON.stringify(room));
  if (phone) await page.screenshot({ path: path.join(screens, 'keyboard-room.png') });
  await page.setViewportSize(viewport);
  await field('Passphrase').fill('short one');
  await field('Type it again').fill('short one');
  await button('Turn on sync').click();
  check('Turn on sync refuses one under 12 characters, the reason under the field, the text kept', await until(async () => (await errorUnder('Passphrase').innerText()) === 'Use at least 12 characters') &&
    (await field('Passphrase').inputValue()) === 'short one');
  await field('Type it again').fill('');
  await field('Passphrase').focus();
  await shot(20, 'first-short');
  const chosen = 'copper lantern orbit';
  await field('Passphrase').fill(chosen + 's');
  check('the strength shows in words, fair for three', (await dialog().innerText()).includes('Fair: add another word'));
  await field('Type it again').fill(chosen + 's');
  await shot(6, 'first');
  await field('Type it again').fill(chosen);
  await button('Turn on sync').click();
  check('two that differ are refused under Type it again', await until(async () => (await errorUnder('Type it again').innerText()) === "The passphrases don't match"));
  await shot(19, 'first-mismatch');
  await button('Suggest one', dialog()).click();
  const suggested = await until(async () => (await field('Passphrase').inputValue()).split(' ').length === 5);
  check('Suggest one fills in five words, shown, and Type it again waits for them', suggested && (await field('Passphrase').getAttribute('type')) === 'text' &&
    (await button('Hide').count()) === 1 && (await field('Type it again').inputValue()) === '' && (await focused('Type it again')));
  check('which are strong', (await dialog().innerText()).includes('Strong'));
  await shot(21, 'first-suggested');
  const passphrase = await field('Passphrase').inputValue();
  await field('Type it again').fill(passphrase);
  await button('Turn on sync').click();
  // The second step: the backup code (docs/sync.md, "Backup code").
  const titled = async (title, ms = 10000) => until(async () => (await dialog().getAttribute('aria-label').catch(() => '')) === title, ms);
  // The code as the card holds it: its text, which has the dash between its two lines too.
  const codeShown = async () => ((await dialog().getByLabel('Backup code', { exact: true }).textContent().catch(() => '')) || '').replace(/\s+/g, '');
  const aCode = /^([0-9A-Z]{4}-){5}[0-9A-Z]{4}$/;
  check('Turn on sync connects, and a bucket just made goes on to its backup code, step 2 of 2', (await titled('Save your backup code')) && (await dialog().innerText()).includes('Step 2 of 2') &&
    (await dialog().innerText()).includes('If you forget your passphrase, this code opens your sync. Keep it somewhere apart from this device.'), await dialog().innerText().catch(() => ''));
  check('the code shows, six groups of four on two lines', await until(async () => aCode.test(await codeShown()), 10000) && (await dialog().getByLabel('Backup code', { exact: true }).locator('span:not(.between)').count()) === 2, await codeShown());
  const firstCode = await codeShown();
  check('with Copy, Save as file, Show as QR, Skip for now, and I\'ve saved it, which has the keyboard', JSON.stringify(await dialog().getByRole('button').allInnerTexts()) === JSON.stringify(['Copy', 'Save as file', 'Show as QR', 'Skip for now', "I've saved it"]) &&
    (await button("I've saved it").evaluate((el) => document.activeElement === el)), JSON.stringify(await dialog().getByRole('button').allInnerTexts()));
  check('and no QR until it is asked for', (await dialog().getByRole('img').count()) === 0);
  await shot(40, 'backup-code');
  await button('Copy').click(); // does: syncBackupCopy
  check('Copy puts the code on the clipboard as a secret, and says so', await until(async () => (await regions()).secretCopied === firstCode && (await dialog().innerText()).includes('Copied')));
  const saving = page.waitForEvent('download', { timeout: 10000 }).catch(() => null);
  await button('Save as file').click();
  const saved = await saving;
  const file = saved && fs.readFileSync(await saved.path(), 'utf8');
  check('Save as file saves it as text, named for what it is, with the server\'s name', !!saved && saved.suggestedFilename() === 'Project B backup code.txt' && file.includes(firstCode) && file.includes(HOST), saved && saved.suggestedFilename());
  await button('Show as QR').click();
  check('Show as QR draws its code, the code in a line under it, and offers to hide it', (await dialog().getByRole('img', { name: 'QR code of your backup code' }).isVisible()) && (await codeShown()) === firstCode && (await button('Hide QR').count()) === 1);
  await shot(41, 'backup-code-qr');
  await button('Hide QR').click();
  await button("I've saved it").click(); // does: syncBackupKeep
  check('I\'ve saved it closes the Dialog', await until(async () => (await dialog().count()) === 0, 10000));
  check('the card shows the server and when it last synced, with Sync now', await until(async () => (await serverRow().innerText()).includes(HOST) && (await serverRow().innerText()).includes('Last synced just now'), 10000) &&
    (await button('Sync now', card()).isVisible()), await card().innerText());
  check('and the passphrase\'s, the backup code\'s and Turn off\'s rows, naming nothing synced', (await card().locator('.row').count()) === 5 && !(await serverRow().innerText()).includes('Bookmarks'));
  await shot(9, 'on');

  // The backup code's row: when it was made, Check… and New code….
  const backupRow = () => card().locator('.row', { hasText: 'Backup code' });
  const today = new Date().toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
  check('the Backup code row says when it was made, with Check… and New code…', (await backupRow().innerText()).replace(/\s+/g, ' ').trim() === `Backup code Made ${today} Check… New code…`, await backupRow().innerText());
  await button('Check…', card()).click();
  await titled('Check your backup code');
  await field('Backup code').fill('0000-0000-0000-0000-0000-0000');
  await button('Check').click(); // does: syncBackupCheck
  check('Check… says of another code that it isn\'t this sync\'s, as an answer and no error', await until(async () => (await dialog().innerText()).includes("That's not this sync's backup code.")) && (await dialog().locator('.text-field-error').count()) === 0);
  await field('Backup code').fill(firstCode.toLowerCase().replace(/-/g, ' '));
  await field('Backup code').press('Enter');
  check('and of the code itself, typed in small letters with spaces, that it is', await until(async () => (await dialog().innerText()).includes('This is your backup code.')), await dialog().innerText());
  if (phone) {
    // A saved code may be only its QR: a phone's scanner reads it into the field and checks it.
    await field('Backup code').fill('');
    await lib.fire('scanner', firstCode);
    await button('Scan QR code').click();
    check('on a phone Check… scans a code too, and checks what was read', await until(async () => (await field('Backup code').inputValue()) === firstCode && (await dialog().innerText()).includes('This is your backup code.'), 10000));
    await lib.fire('scanner', '');
  }
  await shot(42, 'backup-check');
  await button('Done').click();
  // New code…: a warning first, and the old code stays until the new one is said to be saved.
  await button('New code…', card()).click();
  check('New code… warns first that the current one stops working once the new one is saved', (await titled('Make a new backup code?')) && (await dialog().innerText()).includes("Your current code stops working once you've saved the new one.") &&
    JSON.stringify(await dialog().getByRole('button').allInnerTexts()) === JSON.stringify(['Cancel', 'Make new code']));
  await shot(43, 'backup-new');
  await button('Make new code').click(); // does: syncBackupMake
  await titled('Save your backup code');
  await until(async () => aCode.test(await codeShown()), 10000);
  const dropped = await codeShown();
  check('the new code\'s card has Cancel, and no step', dropped !== firstCode && (await button('Cancel').count()) === 1 && !(await dialog().innerText()).includes('Step 2 of 2'));
  await button('Cancel').click(); // does: syncBackupDrop
  await until(async () => (await dialog().count()) === 0);
  const is = async (code) => {
    await button('Check…', card()).click();
    await titled('Check your backup code');
    await field('Backup code').fill(code);
    await button('Check').click();
    await until(async () => /This is your backup code\.|That's not this sync's backup code\./.test(await dialog().innerText()), 10000);
    const said = await dialog().innerText();
    await button('Done').click();
    return said.includes('This is your backup code.');
  };
  const afterCancel = [await is(firstCode), await is(dropped)];
  check('Cancel takes the new code away and keeps the old one', afterCancel[0] && !afterCancel[1], JSON.stringify(afterCancel));
  await button('New code…', card()).click();
  await button('Make new code').click();
  await titled('Save your backup code');
  await until(async () => aCode.test(await codeShown()), 10000);
  const code = await codeShown();
  await button("I've saved it").click();
  await until(async () => (await dialog().count()) === 0, 10000);
  const afterKept = [await is(code), await is(firstCode)];
  check('I\'ve saved it makes the new code the one, and the old one stops working', afterKept[0] && !afterKept[1], JSON.stringify(afterKept));

  // Sync now asks the server again.
  let n = await requests();
  let told = sends({ t: 'syncNow' });
  await button('Sync now', card()).click();
  // A run too quick to see still answers the press: Syncing… at once and for a second, then
  // when it last synced.
  check('Sync now says Syncing… at once, and stays under the finger, disabled', (await until(async () => (await serverRow().innerText()).includes('Syncing…'), 500)) && (await button('Sync now', card()).isDisabled()) && (await button('Cancel', card()).count()) === 0);
  check('Sync now sends syncNow', await told);
  check('Sync now syncs', await until(async () => (await requests()) > n));
  check('and then says when it last synced, with Sync now again', await until(async () => (await serverRow().innerText()).includes('Last synced just now') && (await button('Sync now', card()).isVisible()), 3000));

  // A run that takes a while: after a second it says so, with Cancel and Retry.
  await server({ slow: '1' });
  await button('Sync now', card()).click();
  check('a run that takes a while goes on saying Syncing…', (await until(async () => (await serverRow().innerText()).includes('Syncing…'), 500)) && (await page.waitForTimeout(2000), (await serverRow().innerText()).includes('Syncing…')));
  check('with Cancel and Retry', (await button('Cancel', card()).isVisible()) && (await button('Retry', card()).isVisible()));
  await shot(10, 'syncing');
  told = sends({ t: 'syncCancel' });
  await button('Cancel', card()).click();
  check('Cancel sends syncCancel', await told);
  check('Cancel stops it, and the row is as it was', await until(async () => (await serverRow().innerText()).includes('Last synced') && !(await serverRow().innerText()).includes("Couldn't")));
  await server({ slow: '0' });

  // Out of reach: the row says so, with Try again.
  await server({ down: '1' });
  await button('Sync now', card()).click();
  check('a server out of reach says so on its row, with Try again', await until(async () => (await serverRow().innerText()).includes("Couldn't reach the server · Last synced")) && (await button('Try again', card()).isVisible()));
  await shot(11, 'unreachable');
  await server({ down: '0' });
  await button('Try again', card()).click();
  check('Try again syncs', await until(async () => (await button('Sync now', card()).isVisible()) && !(await serverRow().innerText()).includes("Couldn't")));

  // A failed run says what it was. A server that answers that it failed was reached, so the
  // row doesn't say it wasn't; a failure with no name of its own says no more than that sync
  // failed, with the error's words in the tooltip; and a run with nothing arriving ends as
  // stopped, which a run that is only long never does.
  const rowSays = (words, within = 15000) => until(async () => (await serverRow().innerText()).includes(words), within);
  await until(async () => (await serverRow().innerText()).includes('Last synced just now') && (await button('Sync now', card()).isEnabled()));
  await server({ fail: '500' });
  await button('Sync now', card()).click();
  check("a server that answers with its own failure says that, not that it couldn't be reached, with Try again", (await rowSays('The server answered with an error · Last synced')) && !(await serverRow().innerText()).includes("Couldn't reach") && (await button('Try again', card()).isVisible()), await serverRow().innerText());
  await shot(27, 'server-error');
  await server({ fail: '400' });
  await button('Try again', card()).click();
  check("a failure of another kind says no more than Couldn't sync, with its own words in the tooltip", (await rowSays("Couldn't sync · Last synced")) && ((await serverRow().getAttribute('title')) || '').includes('400') && !((await serverRow().getAttribute('title')) || '').includes(token), `${await serverRow().innerText()} | ${await serverRow().getAttribute('title')}`);
  await shot(28, 'failed');
  await server({ fail: '0', slow: '1', stalled: '4000' });
  await button('Try again', card()).click();
  check('a run with nothing arriving goes on saying Syncing…, with Cancel and Retry', (await rowSays('Syncing…', 500)) && (await page.waitForTimeout(2000), (await serverRow().innerText()).includes('Syncing…')) && (await button('Cancel', card()).isVisible()));
  check('and is ended as stopped, with Try again', (await rowSays('Stopped: nothing arrived for 30 seconds')) && (await button('Try again', card()).isVisible()), await serverRow().innerText());
  await shot(29, 'stalled');
  await server({ slow: '0', stalled: '30000' });
  await button('Try again', card()).click();
  check('Try again syncs after each', await until(async () => (await button('Sync now', card()).isVisible()) && (await serverRow().innerText()).includes('Last synced just now')), await serverRow().innerText());

  // Sync details (docs/sync-details.md): a page of its own from the card's second row, which
  // shows what sync holds now. Nothing is stored for it, so it asks Go as it opens.
  const details = () => page.getByRole('region', { name: 'Sync details', exact: true });
  const stateRow = () => details().locator('.row').first();
  const runRows = () => details().locator('.runs .run');
  check('the card\'s second row is Sync details, with its line', (await card().locator('.row').nth(1).innerText()).includes('Sync details') && (await card().locator('.row').nth(1).innerText()).includes('What synced, and when'));
  told = sends({ t: 'syncDetails' });
  await card().getByRole('button', { name: 'Sync details' }).click();
  check('it opens the page, which asks Go with syncDetails', (await told) && await shown(details()) && await shown(page.getByRole('heading', { name: 'Sync details' })));
  check('its first row is the state, with the card\'s Sync now', await until(async () => (await stateRow().innerText()).includes('Sync is on')) && (await stateRow().innerText()).includes('After a change here, and every 5 minutes') &&
    (await button('Sync now', details()).isVisible()));
  const text = async () => (await details().innerText()).replace(/\s+/g, ' ');
  check('then the server with no token, when it last synced and when it syncs next', await until(async () => (await text()).includes(`https://${HOST}/sync`)) && !(await text()).includes(token) &&
    /Last synced Today, \d\d:\d\d · /.test(await text()) && /Next sync About \d\d:\d\d, or sooner after a change here/.test(await text()) && (await text()).includes('Server changes read') &&
    (await text()).includes('What the server holds Sealed items only. The key stays on your devices'), await text());
  const kinds = await details().locator('[data-kind]').evaluateAll((rows) => rows.map((r) => r.getAttribute('data-kind')));
  check('What syncs has bookmarks, history, passwords, passkeys and typed addresses, each with its counts', kinds.join() === 'bookmarks,history,logins,passkeys,typed' && (await text()).includes('Bookmarks') && (await text()).includes('Typed addresses') && (await text()).includes('Passkeys') && (await text()).includes('Passwords') &&
    (phone ? (await text()).includes('Nothing waiting to send') : (await text()).includes('Items Deletions Waiting to send')), `${kinds} ${await text()}`);
  check('Waiting to send says nothing is waiting', (await text()).includes('Waiting to send Nothing is waiting') || (phone && (await text()).includes('Nothing is waiting')));
  check('the runs since the app started are listed, newest first, each saying why and what it did', (await text()).includes('Runs since the app started') && (await runRows().count()) >= 2 &&
    (await runRows().first().innerText()).includes('Sync now') && (await runRows().last().innerText()).includes('Turning sync on'), await details().locator('.runs').innerText());
  check('the server out of reach was one row, in --danger', await until(async () => (await details().locator('.runs .err').count()) >= 1) &&
    (await details().locator('.runs .err').first().evaluate((e) => getComputedStyle(e).color === getComputedStyle(document.documentElement).getPropertyValue('--danger').trim() || getComputedStyle(e).color !== getComputedStyle(e.closest('.run')).color)));
  await shot(24, 'details');
  // A run opens in place, under its row.
  const first = runRows().first();
  await first.click();
  const panel = () => details().locator('.panel');
  check('a run opens under its row, with its requests', (await first.getAttribute('aria-expanded')) === 'true' && await shown(panel()) && /\d+ requests? · [\d.]+ (B|KB) sent · [\d.]+ (B|KB) received/.test(await panel().innerText()) &&
    / (GET|POST) +(pull|push) +200 /.test(await panel().innerText()), await panel().innerText());
  check('its lines never hold the token', !(await panel().innerText()).includes(token));
  await shot(25, 'details-run');
  await first.click();
  check('and closes again', (await first.getAttribute('aria-expanded')) === 'false' && (await panel().count()) === 0);
  // The run Cancel cut short, earlier, reads Cancelled, muted as a run with nothing new is:
  // only an error is coloured.
  const cancelled = details().locator('.runs .run', { hasText: /cancelled/i }).first();
  check('a run a person cut short reads Cancelled, in --muted', (await cancelled.count()) === 1 &&
    (await cancelled.evaluate((r) => { const d = [...r.querySelectorAll('.did, .on-phone span')].find((e) => e.offsetParent && /cancelled/i.test(e.textContent)); const hint = r.querySelector('.secondary'); return !!d && getComputedStyle(d).color === getComputedStyle(hint).color; })));
  // Each failed run says what it was, and its error's own words are under its row with
  // nothing to open, the host alone in them.
  const said = async () => (await details().locator('.runs .said.words').allInnerTexts()).join(' | ');
  const runsText = async () => (await details().locator('.runs').innerText()).replace(/\s+/g, ' ');
  check('each failed run says what it was: out of reach, the server\'s own failure, stalled, or no more than that it failed',
    /couldn't reach the server/i.test(await runsText()) && /the server answered with an error/i.test(await runsText()) && /stopped: nothing arrived for 30 seconds/i.test(await runsText()) && /couldn't sync/i.test(await runsText()), await runsText());
  check('and its error\'s own words are under its row, with nothing opened and no token in them', (await panel().count()) === 0 && (await said()).includes('500 Internal Server Error') && (await said()).includes('400 Bad Request') &&
    (await said()).includes('nothing arrived for 30 seconds') && (await said()).includes('GET v1/pull?after=') && !(await said()).includes('sync:') && !(await said()).includes(token), await said());
  check('the words can be selected, to copy into a report', await details().locator('.runs .said.words').first().evaluate((e) => getComputedStyle(e).userSelect === 'text'));
  // The queue: a page visited while the server is out of reach waits to go out, named by its
  // address, and goes with the next run, which names it too.
  await server({ down: '1' });
  const visited = 'https://example.org/waiting';
  await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'newTab', text: visited }) });
  const queue = () => details().locator('.queue');
  check('a page visited while the server is out of reach waits in the queue, by its address', await until(async () => (await queue().innerText()).includes('example.org/waiting') && !(await queue().innerText()).includes('https://'), 30000) && (await queue().innerText()).includes('History'), await queue().innerText());
  check('and History counts it as waiting', await until(async () => /(^|\D)1( waiting to send)?$/.test((await details().locator('[data-kind="history"]').innerText()).trim())), await details().locator('[data-kind="history"]').innerText());
  check('the state says the server couldn\'t be reached, in --danger, with Try again and when it tries next', await until(async () => (await stateRow().innerText()).includes("Couldn't reach the server"), 15000) &&
    (await button('Try again', details()).isVisible()) && /Trying again in \d+ minutes?/.test(await stateRow().innerText()) && /Next sync About \d\d:\d\d/.test(await text()), await stateRow().innerText());
  check('and what went wrong under it, the request and then the cause, with no token', /What went wrong (GET|POST) v1\/\S+: .{3,} Server https/.test(await text()) && !(await text()).includes(token) &&
    (await details().locator('.wrong .said').evaluate((e) => getComputedStyle(e).userSelect === 'text')), await text());
  await stateRow().evaluate((e) => e.scrollIntoView({ block: 'center' }));
  await shot(31, 'details-wrong');
  await queue().evaluate((e) => e.scrollIntoView({ block: 'start' }));
  await shot(26, 'details-waiting');
  await server({ down: '0' });
  await button('Try again', details()).click();
  check('Try again sends it: nothing waits, and the run says Sent 1', await until(async () => (await queue().innerText()).includes('Nothing is waiting') && /sent 1/i.test(await runRows().first().innerText()), 15000), await details().locator('.runs').innerText());
  await runRows().first().click();
  check('and names what it sent', (await panel().innerText()).includes('example.org/waiting') && (await panel().innerText()).includes('Sent 1'), await panel().innerText());
  await runRows().first().click();

  // A run going on is the first row: how long it has gone on, and which request it waits on.
  await server({ slow: '1' });
  await button('Sync now', details()).click();
  const going = () => runRows().first().innerText();
  const goingFor = async () => Number((/Syncing… for (\d+) s/i.exec(await going()) || [])[1] ?? -1);
  const waitsFor = () => details().locator('.runs .said').first().innerText();
  check('a run going on is the first row, Now, saying how long it has gone on', await until(async () => (await going()).includes('Now') && (await goingFor()) >= 0, 5000), await going());
  check('and under it what it is doing: no request answered yet, waiting for its pull, and since when', await until(async () => /No request answered yet · waiting for GET pull, asked \d+ s ago/.test(await waitsFor()), 8000), await waitsFor());
  const goneOn = await goingFor();
  check('it counts on while the run goes on', await until(async () => (await goingFor()) >= goneOn + 2, 6000) && (await stateRow().innerText()).includes('Syncing…') && (await button('Cancel', details()).isVisible()), await going());
  await runRows().first().evaluate((e) => e.scrollIntoView({ block: 'center' }));
  await shot(30, 'details-syncing');
  await server({ slow: '0' });
  check('and when the run ends it is a row as any other', await until(async () => !(await going()).includes('Now') && (await button('Sync now', details()).isEnabled()), 15000), await going());

  // Sync now from here is the card's: at once, disabled, then a new run on the list.
  const runsBefore = await runRows().count();
  n = await requests();
  told = sends({ t: 'syncNow' });
  await button('Sync now', details()).click();
  check('Sync now here says Syncing… at once, the button disabled', (await until(async () => (await stateRow().innerText()).includes('Syncing…'), 500)) && (await button('Sync now', details()).isDisabled()));
  check('and sends syncNow, and syncs', (await told) && await until(async () => (await requests()) > n));
  check('and the page asks again, so the run shows', await until(async () => (await stateRow().innerText()).includes('After a change here') && (await runRows().count()) === runsBefore + 1 && (await runRows().first().innerText()).includes('Sync now'), 5000), await details().locator('.runs').innerText());
  // A row sync can't send is set aside, with why, and everything else syncs: the state
  // counts it, the queue names it, and the run that set it aside says so.
  await lib.fire('bigBookmark', 'add');
  check("a row that can't be sent is counted on the state, in --danger, while the rest is synced", await until(async () => (await stateRow().innerText()).includes("1 item couldn't be sent: the rest is synced"), 20000) &&
    (await stateRow().locator('.err').count()) === 1 && (await button('Sync now', details()).isVisible()), await stateRow().innerText());
  const stuckText = async () => (await queue().innerText()).replace(/\s+/g, ' ');
  check("the queue names it, says it couldn't be sent, and why under it", /Too big to send/.test(await stuckText()) && /couldn't be sent/i.test(await stuckText()) &&
    (await queue().locator('.said.words').innerText()).length > 10 && !(await stuckText()).includes(token), await stuckText());
  const setAside = details().locator('.runs .run', { hasText: /couldn't be sent/i }).first();
  check("the run that set it aside says 1 couldn't be sent, in --danger", (await setAside.count()) === 1 && (await setAside.locator('.err').count()) >= 1, await details().locator('.runs').innerText());
  await setAside.click();
  check('and opens to it by name, with why', /Couldn't be sent 1/.test((await panel().innerText()).replace(/\s+/g, ' ')) && (await panel().innerText()).includes('Too big to send') && (await panel().locator('.aside .said.words').innerText()).length > 10, await panel().innerText());
  await details().locator('.queue').evaluate((e) => e.scrollIntoView({ block: 'start' }));
  await shot(32, 'details-unsent');
  await setAside.click();
  await page.getByRole('button', { name: /^Back to / }).click();
  check("the card's row says so too, before when it last synced, with Sync now still there", await until(async () => /1 item couldn't be sent · Last synced/.test((await serverRow().innerText()).replace(/\s+/g, ' '))) &&
    (await serverRow().locator('.err').count()) === 1 && (await button('Sync now', card()).isVisible()), await serverRow().innerText());
  await shot(33, 'unsent');
  // The row gone, nothing is left to say.
  await lib.fire('bigBookmark', 'remove');
  check('once the row is gone the card says only when it last synced', await until(async () => !(await serverRow().innerText()).includes("couldn't be sent") && (await serverRow().innerText()).includes('Last synced'), 30000), await serverRow().innerText());
  await card().getByRole('button', { name: 'Sync details' }).click();
  await until(() => shown(details()));

  // Back goes to the section that holds Sync.
  await page.getByRole('button', { name: /^Back to / }).click();
  check('Back goes to Sync\'s card', await shown(card()) && (await details().count()) === 0);

  // Change sync passphrase: only the new one, twice.
  await button('Change…', card()).click();
  check('Change… asks for the new passphrase only, twice', await shown(dialog()) && (await dialog().getAttribute('aria-label')) === 'Change sync passphrase' &&
    (await field('New passphrase').count()) === 1 && (await focused('New passphrase')));
  const changed = 'river meadow lantern copper drift';
  await field('New passphrase').fill(changed);
  await field('Type it again').fill(changed);
  await shot(13, 'change');
  await field('Type it again').fill(changed + 's');
  await button('Change').click();
  check('two that differ are refused here too', await until(async () => (await errorUnder('Type it again').innerText()) === "The passphrases don't match"));
  await shot(22, 'change-mismatch');
  await field('Type it again').fill(changed);
  await button('Change').click();
  check('Change sets the new one and closes', await until(async () => (await dialog().count()) === 0, 10000));

  // Turn off: a plain primary, nothing destroyed.
  await button('Turn off…', card()).click();
  check('Turn off… asks, with a plain primary Turn off', await shown(dialog()) && (await dialog().getAttribute('aria-label')) === 'Turn off sync on this device?' &&
    (await button('Turn off').getAttribute('class')).includes('primary') && !(await button('Turn off').getAttribute('class')).includes('danger'));
  await shot(14, 'turn-off');
  told = sends({ t: 'syncDisconnect' });
  await button('Turn off').click();
  check('Turn off sends syncDisconnect', await told);
  check('Turn off turns it off', await shown(button('Set up…', card())));

  // A bucket in use: its passphrase, once; the old one is wrong now.
  await button('Set up…', card()).click();
  await field('Server address').fill(bucket);
  await field('Token').fill(token);
  await button('Connect').click();
  check('a bucket in use asks for its passphrase once', await until(async () => (await dialog().getAttribute('aria-label')) === 'Enter your sync passphrase') &&
    (await dialog().innerText()).includes(`${HOST} already has synced data. Type the passphrase you chose for it.`) && (await field('Type it again').count()) === 0);
  await field('Passphrase').fill(changed.slice(0, 20));
  await shot(7, 'later');
  await field('Passphrase').fill(passphrase);
  await button('Turn on sync').click();
  check('a wrong one is refused under the field, with where to set a new one', await until(async () => (await errorUnder('Passphrase').innerText()) === "That's not the sync passphrase", 10000) &&
    (await dialog().innerText()).includes("Forgot it? Use your backup code, or set a new one on a device that's still syncing."));
  await shot(8, 'later-wrong');
  await field('Passphrase').fill(changed);
  await button('Turn on sync').click();
  check('the right one connects', await until(async () => (await dialog().count()) === 0 && (await button('Sync now', card()).isVisible()), 10000));

  // The other way in: the backup code, typed, and then a new passphrase if the person likes.
  await button('Turn off…', card()).click();
  await button('Turn off').click();
  await shown(button('Set up…', card()));
  await button('Set up…', card()).click();
  await field('Server address').fill(bucket);
  await field('Token').fill(token);
  await button('Connect').click();
  await titled('Enter your sync passphrase');
  await field('Passphrase').fill('not the passphrase at all');
  await button('Turn on sync').click();
  check('a wrong passphrase now names the backup code too', await until(async () => (await dialog().innerText()).includes("Forgot it? Use your backup code, or set a new one on a device that's still syncing."), 10000));
  await button('Use backup code instead').click();
  check('Use backup code instead asks for the code, shown as it is typed', (await dialog().innerText()).includes(`${phone ? 'Type or scan' : 'Type'} the backup code you saved. Then you can choose a new passphrase.`) && (await field('Backup code').getAttribute('type')) === 'text' &&
    (await field('Backup code').inputValue()) === '' && (await button('Use the passphrase instead').count()) === 1 && (await focused('Backup code')));
  if (phone) {
    check('a phone offers to scan the code\'s QR, at the field\'s end, and says so', (await button('Scan QR code').count()) === 1 && (await dialog().innerText()).includes('Type or scan the backup code you saved.') &&
      (await button('Scan QR code').evaluate((b) => { const r = b.getBoundingClientRect(), f = document.getElementById('sync-pass').getBoundingClientRect(); return r.width === 40 && r.height === 40 && r.right <= f.right && r.left > f.left && r.top >= f.top && r.bottom <= f.bottom; })));
    await button('Scan QR code').click(); // does: syncBackupScan
    await page.waitForTimeout(500);
    check('a scanner the person leaves fills nothing in', (await field('Backup code').inputValue()) === '');
    await lib.fire('scanner', code);
    await button('Scan QR code').click();
    check('what the scanner reads goes into the field, and the person turns sync on', await until(async () => (await field('Backup code').inputValue()) === code) &&
      (await button('Turn on sync').evaluate((el) => document.activeElement === el)));
    await shot(46, 'backup-scan');
  } else {
    check('a computer has no scanner: the code is typed', (await button('Scan QR code').count()) === 0 && (await dialog().innerText()).includes('Type the backup code you saved.'));
  }
  await field('Backup code').fill('0000-0000-0000-0000-0000-0000');
  await shot(44, 'backup-join');
  await button('Turn on sync').click();
  check('a code that isn\'t this sync\'s is refused under the field', await until(async () => (await errorUnder('Backup code').innerText()) === "That's not this sync's backup code", 10000));
  await field('Backup code').fill(code.toLowerCase().replace(/-/g, ''));
  await button('Turn on sync').click();
  check('the code, typed in small letters with no dashes, gets in, and a new passphrase can be chosen', (await titled('Choose a new sync passphrase')) &&
    (await dialog().innerText()).includes("You're in with your backup code, which still works.") && (await button('Not now').count()) === 1 && (await field('New passphrase').inputValue()) === '');
  await shot(45, 'backup-renew');
  await field('New passphrase').fill(changed);
  await field('Type it again').fill(changed);
  await button('Change').click();
  check('Change sets it, and sync is on', await until(async () => (await dialog().count()) === 0 && (await button('Sync now', card()).isVisible()), 10000));

  // With sync on, the saved passwords are on the sync server too, sealed, and a delete goes
  // round: the Passwords page says both, where with sync off it speaks of this device alone.
  await lib.fire('logins', JSON.stringify([{ Origin: 'https://mail.example.org', Username: 'asha@example.org', Password: 'correct horse battery' }]));
  if (phone) await page.getByRole('button', { name: 'Back to Settings' }).click();
  if (phone) await page.locator('.section-row[data-section="privacy"]').click();
  else await page.locator('.nav [data-section="privacy"]').click();
  await settle(page);
  await page.locator('.row', { hasText: 'Passwords' }).first().getByRole('button').first().click();
  const passwords = page.getByRole('region', { name: 'Passwords', exact: true });
  check("with sync on, Passwords says the sync server can't read them", await until(async () => (await passwords.locator('.intro').innerText().catch(() => '')).replace(/\s+/g, ' ').trim().startsWith("Sealed for this profile; your sync server can't read them. Showing or copying a password asks for ")),
    await passwords.locator('.intro').innerText().catch(() => ''));
  await passwords.locator('.row.login').first().getByRole('button', { name: /^More for / }).click();
  await passwords.getByRole('menuitem', { name: 'Delete…' }).click();
  check('and deleting one says it goes from the synced devices too', await until(async () => (await dialog().innerText()).replace(/\s+/g, ' ').includes("Your saved password for mail.example.org (asha@example.org) is deleted here and on your synced devices. Your account on mail.example.org isn't changed.")),
    await dialog().innerText().catch(() => ''));
  await shot(36, 'password-delete-synced');
  await dialog().getByRole('button', { name: 'Cancel' }).click();
  await page.getByRole('button', { name: /^Back to / }).click();
  if (phone) await page.getByRole('button', { name: 'Back to Settings' }).click();
  await showSync();

  // The bucket reset on the server: its key is gone with everything else, so this device
  // sends nothing and says so. Set up again… opens the set-up with the address filled in,
  // and the bucket, empty again, asks for a new passphrase, as a first device's does.
  await server({ reset: '1' });
  // Sync now finds it, unless a run of sync's own found it first: the login brought above
  // is a change, and its run takes the button away as it starts.
  await button('Sync now', card()).click({ timeout: 3000 }).catch(() => {});
  check('a bucket that was reset says so on the row, with Set up again…', await until(async () => (await serverRow().innerText()).includes('The sync bucket was reset'), 15000) && (await button('Set up again…', card()).isVisible()) &&
    (await button('Sync now', card()).count()) === 0, await serverRow().innerText());
  check('with no passphrase to change until then, and Turn off still there, keeping what is here: the server holds nothing', !(await card().innerText()).includes('Change sync passphrase') && (await button('Turn off…', card()).isVisible()) &&
    (await card().innerText()).includes('Keeps everything here') && !(await card().innerText()).includes('and on the server'));
  await shot(34, 'bucket-reset');
  told = sends({ t: 'syncInspect' });
  await button('Set up again…', card()).click();
  check('Set up again… asks nothing of the address or the token: Go holds them, and looks at the bucket at once', (await told) && (await field('Server address').count()) === 0 || (await until(async () => (await dialog().getAttribute('aria-label')) === 'Choose a sync passphrase')));
  check('the bucket is empty again, so a new passphrase is chosen, as a first device chooses one', await until(async () => (await dialog().getAttribute('aria-label')) === 'Choose a sync passphrase'));
  await shot(35, 'reset-choose');
  await field('Passphrase').fill(changed);
  await field('Type it again').fill(changed);
  await button('Turn on sync').click();
  await titled('Save your backup code');
  await button('Skip for now').click();
  check('and sync is on again, with everything here sent up', await until(async () => (await dialog().count()) === 0 && (await button('Sync now', card()).isVisible()) && !(await serverRow().innerText()).includes('reset'), 15000), await serverRow().innerText());

  // Signed out: the server no longer takes the token; Reconnect… fills in the address. Sync
  // now is pressed for a run to meet the refusal. A run may be going already, which the row
  // doesn't show in its first second: the one that takes away the backup code just skipped,
  // which comes after its making however long that takes. Refused, it signs the device out
  // by itself, and then there is no Sync now to press, and no need of one.
  await server({ refuse: token });
  await button('Sync now', card()).click({ timeout: 5000 }).catch(() => {});
  check('a token the server refuses signs the device out, with Reconnect…', await until(async () => (await serverRow().innerText()).includes('The server signed this device out')) && (await button('Reconnect…', card()).isVisible()));
  await shot(12, 'signed-out');
  await button('Reconnect…', card()).click();
  check('Reconnect… opens the set-up with the address filled in and the focus on Token', await shown(dialog()) && (await field('Server address').inputValue()) === bucket && await until(() => focused('Token')));
  await shot(23, 'reconnect');
  const token2 = `${token}-again`;
  await field('Token').fill(token2);
  await button('Connect').click();
  await until(async () => (await dialog().getAttribute('aria-label')) === 'Choose a sync passphrase');
  await field('Passphrase').fill(changed);
  await field('Type it again').fill(changed);
  await button('Turn on sync').click();
  await titled('Save your backup code');
  await button('Skip for now').click();
  check('and connecting again with a new token syncs again, the backup code skipped: its row offers to make one', (await until(async () => (await card().innerText().catch(() => '')).includes('No backup code'), 10000)) && (await button('Make one…', card()).count()) === 1 && (await dialog().count()) === 0 && (await button('Sync now', card()).isVisible()), await card().innerText().catch(() => ''));
  // A code still being made when its card is left goes too, as one already shown does: with
  // the server slow, Cancel comes while the card still says it is making one. Sync does what
  // is asked of it in turn, so the card opened again shows its code only once the first was
  // made and taken away; that one left too, the bucket has no backup code.
  await server({ slow: '1' });
  await button('Make one…', card()).click();
  await titled('Save your backup code');
  check('with the server slow, the card says it is making the code', await until(async () => (await dialog().innerText()).includes('Making your backup code…')), await dialog().innerText());
  await button('Cancel').click();
  await until(async () => (await dialog().count()) === 0);
  await server({ slow: '0' });
  // A code that stayed leaves the row with no Make one… to press, which is the failure too.
  const again = await button('Make one…', card()).click({ timeout: 15000 }).then(() => true, () => false);
  if (again) {
    await titled('Save your backup code');
    await until(async () => (await codeShown()) !== '', 20000);
    await button('Cancel').click();
  }
  check('a code left while it was still being made is taken away, as one left after it showed is: the row still offers to make one',
    again && (await until(async () => (await card().innerText()).includes('No backup code') && (await button('Make one…', card()).count()) === 1, 15000)) && !(await card().innerText()).includes('New code…'), await card().innerText());
  await button('Turn off…', card()).click();
  await button('Turn off').click();
  await shown(button('Set up…', card()));

  // The link from another app: Settings at Sync, the Dialog in its link form, and nothing
  // sent until Connect.
  const linked = `${address}/sync/7kq2mx9pab4c`;
  n = await requests();
  await fetch(`${lib.base}/mock/open`, { method: 'POST', body: new URLSearchParams({ url: link(linked) }) });
  check('a link from another app opens the Dialog in its link form', await until(async () => (await dialog().count()) === 1 && (await dialog().getAttribute('aria-label')) === `Sync with ${HOST}?`));
  check('its title has the host in bold', (await dialog().locator('.ui-dialog-title').evaluate((h) => [...h.querySelectorAll('.w')].map((w) => getComputedStyle(w).fontWeight).join() + '|' + getComputedStyle(h).fontWeight)) === '400,400|600');
  check('with the address filled in', (await field('Server address').inputValue()) === linked && (await dialog().innerText()).includes("A link asked to set up sync with this server. Connect only if it's yours."));
  check('and Settings at Sync', await card().isVisible());
  await page.waitForTimeout(500);
  check('nothing went to the server', (await requests()) === n, `${(await requests()) - n} requests`);
  await shot(5, 'link');
  await button('Connect').click();
  check('until Connect', await until(async () => (await requests()) > n) && await until(async () => (await dialog().getAttribute('aria-label')) === 'Choose a sync passphrase'));
  await button('Cancel').click();
  await until(async () => (await dialog().count()) === 0);

  // A link that gives a token too: it is in its field, hidden, and goes with Connect.
  await fetch(`${lib.base}/mock/open`, { method: 'POST', body: new URLSearchParams({ url: `${link(`${address}/sync`)}&token=${encodeURIComponent(`tok-link-${layout}`)}` }) });
  check('a link with a token opens the set-up with the token in its field, hidden', await until(async () => (await dialog().count()) === 1 && (await field('Token').inputValue()) === `tok-link-${layout}`) &&
    (await field('Token').getAttribute('type')) === 'password' && (await field('Server address').inputValue()) === `${address}/sync`);
  await shot(48, 'link-token');
  await button('Connect').click();
  check('and Connect reaches the server with it', await until(async () => (await dialog().getAttribute('aria-label')) === 'Choose a sync passphrase'));
  await button('Cancel').click();
  await until(async () => (await dialog().count()) === 0);

  // An http address in a link: the plain set-up, with the address and its error.
  await fetch(`${lib.base}/mock/open`, { method: 'POST', body: new URLSearchParams({ url: link(`http://${HOST}/sync/7kq2mx9pab4c`) }) });
  check('a link with an http address opens the set-up with the address error', await until(async () => (await dialog().count()) === 1 && (await dialog().getAttribute('aria-label')) === 'Set up sync' &&
    (await errorUnder('Server address').count()) === 1 && (await errorUnder('Server address').innerText()) === 'Sync needs an https address') && (await field('Server address').inputValue()) === `http://${HOST}/sync/7kq2mx9pab4c`);
  await button('Cancel').click();
  await until(async () => (await dialog().count()) === 0);

  // A link as the app starts: the page is laid out narrow first, then takes the window's
  // width, and Sync stays near the top of the page.
  if (!phone) {
    await page.setViewportSize({ width: 360, height: viewport.height });
    await page.evaluate(() => (document.querySelector('main.content').scrollTop = 0));
    await fetch(`${lib.base}/mock/open`, { method: 'POST', body: new URLSearchParams({ url: link(linked) }) });
    await until(async () => (await dialog().count()) === 1);
    await settle(page);
    await page.setViewportSize(viewport);
    await settle(page);
    const top = await page.evaluate(() => document.querySelector('[data-part="sync"]').getBoundingClientRect().top - document.querySelector('main.content').getBoundingClientRect().top);
    // The one page puts Your data's group at its top, and Sections its section, Sync in it.
    check('opened at Sync while narrow, then widened, Sync stays in the top half of the page', top >= 0 && top < viewport.height / 2, top);
    await button('Cancel').click();
    await until(async () => (await dialog().count()) === 0);
  }

  // A page's link: pressed, it opens the Dialog; gone to by the page's own script, nothing.
  await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'newTab', text: 'https://example.com/' }) });
  let web;
  await until(async () => (web = (await regions()).tabs.find((t) => t.url === 'https://example.com/')));
  n = await requests();
  await lib.fire('navigate', link(linked), { tab: web.id });
  await page.waitForTimeout(800);
  check('a page\'s script going to the link opens nothing, and the page stays', (await dialog().count()) === 0 && (await regions()).tabs.find((t) => t.id === web.id).url === 'https://example.com/');
  await lib.fire('link', link(linked), { tab: web.id });
  check('a press on the link in a page opens the Dialog in its link form', await until(async () => (await page.getByRole('dialog', { name: `Sync with ${HOST}?` }).count()) === 1));
  check('and sends nothing either', (await requests()) === n);
  await button('Cancel').click();
  await until(async () => (await dialog().count()) === 0);

  // A profile whose keys don't open here: connected, then its folder.key replaced, as a
  // folder from another device has.
  await button('Set up…', card()).click();
  await field('Server address').fill(bucket);
  await field('Token').fill(token2);
  await button('Connect').click();
  await until(async () => (await dialog().getAttribute('aria-label')) === 'Enter your sync passphrase');
  await field('Passphrase').fill(changed);
  await button('Turn on sync').click();
  await until(async () => (await button('Sync now', card()).isVisible()), 10000);
  await page.close();
  await lib.stop();
  const key = path.join(lib.profile, 'folder.key');
  fs.rmSync(key, { force: true });
  fs.writeFileSync(key, 'k'.repeat(32), { mode: 0o600 });
  await lib.start(args);
  await openSettings();
  await showSync();
  check('keys that don\'t open here are a state: Sync can\'t run on this device, with Set up again…', await shown(button('Set up again…', card())) &&
    (await card().innerText()).includes("Sync can't run on this device") && (await card().innerText()).includes("This profile's saved sync details can't be opened here"));
  check('its reason in the tooltip', ((await card().locator('.row').getAttribute('title')) || '').includes('moved from another device'));
  await shot(2, 'no-key');
  await button('Set up again…', card()).click();
  check('Set up again… opens the Dialog as new', await shown(dialog()) && (await field('Server address').inputValue()) === 'https://memd.debkosh.com/sync' && (await field('Token').inputValue()) === '');
  await field('Server address').fill(bucket);
  await field('Token').fill(token2);
  await button('Connect').click();
  await until(async () => (await dialog().getAttribute('aria-label')) === 'Enter your sync passphrase');
  await field('Passphrase').fill(changed);
  told = sends({ t: 'syncStartFresh' });
  await button('Turn on sync').click();
  check('Turn on sync sends syncStartFresh first, as the keys here don\'t open', await told);
  check('and syncs again with the passphrase, starting the profile\'s keys afresh', await until(async () => (await dialog().count()) === 0 && (await button('Sync now', card()).isVisible()), 10000));

  // A Linux keyring that gives no device key, as a locked or broken one: sync stays on and
  // says it can't start, with the keyring's line, Learn more and Try again, where it showed
  // as off; Try again asks again, and a start whose keyring gives the key syncs as before.
  await page.close();
  await lib.stop();
  await lib.start([...args, '-platform=linux', '-keyring-locked']);
  await openSettings();
  await showSync();
  const KEYRING = "Your keyring is locked. Unlock it when asked; if it doesn't ask, log out and back in.";
  check('a keyring that gives no key leaves sync on: Sync can\'t start, with the keyring\'s line, Learn more and Try again',
    (await until(async () => (await card().innerText()).includes("Sync can't start"))) && (await card().innerText()).includes(KEYRING) &&
      (await button('Learn more', card()).isVisible()) && (await button('Try again', card()).isVisible()) && (await button('Set up…', card()).count()) === 0, await card().innerText());
  check('the rest of the card stays, as sync is on: Sync details and Turn off sync', (await card().innerText()).includes('Sync details') && (await button('Turn off…', card()).isVisible()));
  check('what is the bucket\'s isn\'t there, as nothing is known of it yet: no Change sync passphrase, and no backup code\'s row to say there is none',
    !(await card().innerText()).includes('Change sync passphrase') && !(await card().innerText()).includes('backup code') && (await button('Make one…', card()).count()) === 0, await card().innerText());
  await shot(27, 'keyring');
  told = sends({ t: 'syncNow' });
  await button('Try again', card()).click();
  check('Try again asks sync to start again, and the line is back while the keyring gives nothing', (await told) && (await until(async () => (await card().innerText()).includes(KEYRING))), await card().innerText());
  await page.close();
  await lib.stop();
  await lib.start(args);
  await openSettings();
  await showSync();
  check('a start whose keyring gives the key syncs as before', await until(async () => (await button('Sync now', card()).isVisible()), 10000), await card().innerText());

  // A phone's buttons: drawn at 40px, each in a 48px target; its fields 48px.
  if (phone) {
    const targets = async (within) => page.evaluate((sel) => [...document.querySelectorAll(sel)]
      .filter((b) => b.checkVisibility())
      .map((b) => {
        const r = b.getBoundingClientRect();
        const drawn = getComputedStyle(b, '::before');
        return { what: b.textContent.trim(), h: Math.round(r.height), w: Math.round(r.width), drawn: Math.round(r.height) - parseFloat(drawn.top) - parseFloat(drawn.bottom) };
      })
      .filter((t) => t.h < 48 || t.w < 48 || t.drawn !== 40)
      .map((t) => `${t.what} ${t.w}x${t.h} drawn ${t.drawn}`), within);
    let small = await targets('[data-part="sync"] .text-button');
    check('on a phone the card\'s buttons are 40px, each in a 48px target', small.length === 0, small.join(', '));
    await button('Change…', card()).click();
    await shown(dialog());
    small = await targets('dialog .text-button');
    const fields = await dialog().locator('.text-field').evaluateAll((fs) => fs.map((f) => Math.round(f.getBoundingClientRect().height)));
    check('and the Dialog\'s too, its fields 48px', small.length === 0 && fields.every((h) => h === 48), `${small.join(', ')} | ${fields}`);
    await button('Cancel').click();
  }

  check('no errors in the pages', errors.length === 0, errors.join(' | '));

  // An overview of every state, light and dark, in the mocks' order.
  const flow = [1, 2, 27, 3, 4, 15, 16, 17, 18, 5, 6, 20, 21, 19, 7, 8, 9, 10, 11, 24, 25, 26, 12, 23, 13, 22, 14];
  const names = flow.map((k) => taken.find((t) => t.startsWith(String(k).padStart(2, '0') + '-'))).filter((x) => x);
  for (const scheme of ['light', 'dark']) {
    const w = phone ? 412 : 1000;
    const cols = phone ? 8 : 4;
    const html = `<body style="margin:0;padding:24px;background:${scheme === 'dark' ? '#1d2224' : '#e9ecee'};font:15px -apple-system,sans-serif;color:${scheme === 'dark' ? '#eee' : '#1f1f1f'}">
      <h1 style="font-size:22px;margin:0 0 16px">Settings › Sync, ${layout}, ${scheme}</h1>
      <div style="display:grid;grid-template-columns:repeat(${cols},${w}px);gap:28px 20px">${names.map((nm) => `<div><div style="margin:0 0 8px;font-weight:600">${nm}</div>
      <img src="file://${path.join(screens, `${nm}-${scheme}.png`)}" style="width:${w}px;display:block;border-radius:6px;box-shadow:0 0 0 1px rgba(0,0,0,.15)"></div>`).join('')}</div></body>`;
    const sheet = path.join(screens, `overview-${scheme}.html`);
    fs.writeFileSync(sheet, html);
    const p = await browser.newPage({ viewport: { width: cols * (w + 20) + 48, height: 800 } });
    await p.goto(`file://${sheet}`);
    // Every picture decoded, and the window the sheet's height, or those below the first
    // screen can be drawn empty.
    await p.evaluate(() => Promise.all([...document.images].map((i) => i.decode())));
    await p.setViewportSize({ width: cols * (w + 20) + 48, height: await p.evaluate(() => document.documentElement.scrollHeight) });
    await settle(p);
    await p.screenshot({ path: path.join(screens, `overview-${scheme}.png`), fullPage: true });
    await p.close();
    fs.rmSync(sheet);
  }

  await browser.close();
  await lib.done();
})();
