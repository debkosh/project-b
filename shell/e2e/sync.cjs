// Settings' Sync on the mock engine (docs/sync.md), against a real sync server: synckosh's,
// which the dev server runs with -sync-server and sync reaches as memd.example.com. The card
// under Your data in each of the page's two layouts and on a phone, search finding it, the
// set-up Dialog through a new bucket and one in use, every refusal (a short passphrase, two
// that differ, a wrong one, an http address, a server out of reach, an address that isn't a
// bucket, a token refused), Suggest one, the connected rows (Sync now, syncing with Cancel
// and Retry, out of reach, signed out and Reconnect), Change sync passphrase and Turn off;
// a profile whose keys don't open here, and Set up again; and the project-b://sync link,
// from another app and pressed in a page, which sends nothing to the server before Connect,
// and from a page's own script, which does nothing.
// Every state is saved in light and dark under build/sync-screens/<layout>/, with an
// overview of each, for the ux review.
// Run: node shell/e2e/sync.cjs [page|sections|phone]   (from the repo root, after `npm run
// build` in shell/); with none, each in turn.

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
const { check, regions, settle, until } = lib;
const phone = layout === 'phone';
const sectionsLayout = layout === 'sections';

// A listed profile, Personal, as the mocks show it; on the mock engine its keys are wrapped by
// a folder.key of its own, as a folder named outright's are.
const root = path.dirname(lib.profile);
fs.writeFileSync(path.join(root, 'profiles.json'), JSON.stringify({ profiles: [{ id: 'profile', name: 'Personal', color: '#4f9e9e', template: 'Personal' }] }));
// The frame colour the mocks are drawn in.
fs.writeFileSync(path.join(lib.profile, 'prefs.json'), JSON.stringify({ frameColor: '#4f9e9e', ...(sectionsLayout ? { settingsLayout: 'sections' } : {}) }));
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

  // Sync's card, as a person goes to it: Your data's group, or the section that holds it.
  const showSync = async () => {
    if (sectionsLayout) await page.locator('.nav [data-section="profile"]').click();
    else if (phone) await page.locator('.chips [data-section="data"]').click();
    else await page.locator('.nav [data-section="data"]').click();
    await settle(page);
  };

  // A state's pictures, in each theme, the page where Your data's group starts, as the
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
  check('off, it has one row, Set up sync, and its line', (await card().locator('.row').count()) === 1 && (await card().innerText()).includes('Bookmarks, history and typed addresses, through your own sync server'));
  if (!(phone || sectionsLayout)) check('the card is under the heading Sync', (await card().locator('h2').innerText()) === 'Sync');
  await shot(1, 'off');

  // Search finds it by sync and by passphrase.
  for (const word of ['sync', 'passphrase']) {
    const search = page.getByLabel('Search settings');
    await search.fill(word);
    check(`searching for ${word} finds Set up sync`, await shown(button('Set up…', card())));
    await search.fill('');
  }
  await showSync();

  // Setting up: the address, as typed.
  await button('Set up…', card()).click();
  check('Set up… opens the Dialog, Set up sync, the address field focused', await shown(dialog()) && (await dialog().getAttribute('aria-label')) === 'Set up sync' && (await focused('Address')));
  check('with the token as an option', (await field('Token').getAttribute('placeholder')) === 'Only if your server gave you one');
  await field('Address').fill(`${address}/sync/7kq2mx9pab4c`);
  await shot(3, 'address');
  await field('Address').fill(`http://${HOST}/sync/7kq2mx9pab4c`);
  check('an http address is refused as it is typed, under the field, the text kept', await until(async () => (await errorUnder('Address').innerText()) === 'Sync needs an https address') &&
    (await field('Address').inputValue()) === `http://${HOST}/sync/7kq2mx9pab4c`);
  await shot(4, 'address-error');
  const before = await requests();
  await button('Connect').click();
  check('and Connect sends nothing for it', (await requests()) === before && (await dialog().getAttribute('aria-label')) === 'Set up sync');

  // Connecting: the button says so and waits; Cancel still works.
  await server({ slow: '1' });
  await field('Address').fill(`${address}/sync/7kq2mx9pab4c`);
  await button('Connect').click();
  check('Connect reads Connecting… and waits', await shown(button('Connecting…')) && (await button('Connecting…').isDisabled()));
  await shot(15, 'connecting');
  await button('Cancel').click();
  check('Cancel still works', await until(async () => (await dialog().count()) === 0));
  await server({ slow: '0' });

  // What the server says, under the field it is about, the text kept.
  const refusal = async (addr, token, under, words, n, id) => {
    await button('Set up…', card()).click();
    await field('Address').fill(addr);
    if (token) await field('Token').fill(token);
    await button('Connect').click();
    const ok = await until(async () => (await errorUnder(under).count()) === 1 && (await errorUnder(under).innerText()) === words);
    check(`${words}, under ${under}, the text kept`, ok && (await field('Address').inputValue()) === addr && (await field('Token').inputValue()) === token, ok ? '' : await dialog().innerText());
    if (n) await shot(n, id);
    await button('Cancel').click();
  };
  if (!phone) await refusal('https://127.0.0.1:1/sync/7kq2mx9pab4c', '', 'Address', "Couldn't reach 127.0.0.1:1");
  await server({ down: '1' });
  await refusal(`${address}/sync/7kq2mx9pab4c`, '', 'Address', `Couldn't reach ${HOST}`, 16, 'address-unreachable');
  await server({ down: '0' });
  await refusal(`${address}/notes/sync/7kq2mx9pab4c`, '', 'Address', "That address isn't a sync bucket", 17, 'not-bucket');
  await refusal(`${address}/sync`, '', 'Token', 'This address needs a token');
  await server({ refuse: 'TOKEN-k3v9x2m7q8rd' });
  await refusal(`${address}/sync`, 'TOKEN-k3v9x2m7q8rd', 'Token', "The server didn't accept that token", 18, 'token-refused');

  // A new bucket: a passphrase, chosen twice.
  const bucket = `${address}/sync`;
  const token = `tok-${layout}`;
  await button('Set up…', card()).click();
  await field('Address').fill(bucket);
  await field('Token').fill(token);
  await button('Connect').click();
  check('an empty bucket asks for a new passphrase, twice', await until(async () => (await dialog().getAttribute('aria-label')) === 'Choose a sync passphrase') &&
    (await dialog().innerText()).includes(`Nothing is synced on ${HOST} yet. Every device you add asks for this passphrase.`) && (await field('Type it again').count()) === 1);
  check('the passphrase field has the focus', await focused('Passphrase'));
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
  check('Turn on sync connects and closes the Dialog', await until(async () => (await dialog().count()) === 0, 10000));
  check('the card shows the server and when it last synced, with Sync now', await until(async () => (await serverRow().innerText()).includes(HOST) && (await serverRow().innerText()).includes('Last synced just now'), 10000) &&
    (await button('Sync now', card()).isVisible()), await card().innerText());
  check('and the passphrase\'s and Turn off\'s rows, naming nothing synced', (await card().locator('.row').count()) === 3 && !(await serverRow().innerText()).includes('Bookmarks'));
  await shot(9, 'on');

  // Sync now asks the server again.
  let n = await requests();
  let told = sends({ t: 'syncNow' });
  await button('Sync now', card()).click();
  check('Sync now sends syncNow', await told);
  check('Sync now syncs', await until(async () => (await requests()) > n));

  // A run that takes a while: after a second it says so, with Cancel and Retry.
  await server({ slow: '1' });
  await button('Sync now', card()).click();
  check('a run says Syncing… only after about a second', (await serverRow().innerText()).includes('Last synced') && await until(async () => (await serverRow().innerText()).includes('Syncing…'), 3000));
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
  await field('Address').fill(bucket);
  await field('Token').fill(token);
  await button('Connect').click();
  check('a bucket in use asks for its passphrase once', await until(async () => (await dialog().getAttribute('aria-label')) === 'Enter your sync passphrase') &&
    (await dialog().innerText()).includes(`${HOST} already has synced data. Type the passphrase you chose for it.`) && (await field('Type it again').count()) === 0);
  await field('Passphrase').fill(changed.slice(0, 20));
  await shot(7, 'later');
  await field('Passphrase').fill(passphrase);
  await button('Turn on sync').click();
  check('a wrong one is refused under the field, with where to set a new one', await until(async () => (await errorUnder('Passphrase').innerText()) === "That's not the sync passphrase", 10000) &&
    (await dialog().innerText()).includes("Forgot it? Set a new one on a device that's still syncing."));
  await shot(8, 'later-wrong');
  await field('Passphrase').fill(changed);
  await button('Turn on sync').click();
  check('the right one connects', await until(async () => (await dialog().count()) === 0 && (await button('Sync now', card()).isVisible()), 10000));

  // Signed out: the server no longer takes the token; Reconnect… fills in the address.
  await server({ refuse: token });
  await button('Sync now', card()).click();
  check('a token the server refuses signs the device out, with Reconnect…', await until(async () => (await serverRow().innerText()).includes('The server signed this device out')) && (await button('Reconnect…', card()).isVisible()));
  await shot(12, 'signed-out');
  await button('Reconnect…', card()).click();
  check('Reconnect… opens the set-up with the address filled in and the focus on Token', await shown(dialog()) && (await field('Address').inputValue()) === bucket && await until(() => focused('Token')));
  await shot(23, 'reconnect');
  const token2 = `${token}-again`;
  await field('Token').fill(token2);
  await button('Connect').click();
  await until(async () => (await dialog().getAttribute('aria-label')) === 'Choose a sync passphrase');
  await field('Passphrase').fill(changed);
  await field('Type it again').fill(changed);
  await button('Turn on sync').click();
  check('and connecting again with a new token syncs again', await until(async () => (await dialog().count()) === 0 && (await button('Sync now', card()).isVisible()), 10000));
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
  check('with the address filled in', (await field('Address').inputValue()) === linked && (await dialog().innerText()).includes("A link asked to set up sync with this server. Connect only if it's yours."));
  check('and Settings at Sync', await card().isVisible());
  await page.waitForTimeout(500);
  check('nothing went to the server', (await requests()) === n, `${(await requests()) - n} requests`);
  await shot(5, 'link');
  await button('Connect').click();
  check('until Connect', await until(async () => (await requests()) > n) && await until(async () => (await dialog().getAttribute('aria-label')) === 'Choose a sync passphrase'));
  await button('Cancel').click();
  await until(async () => (await dialog().count()) === 0);

  // An http address in a link: the plain set-up, with the address and its error.
  await fetch(`${lib.base}/mock/open`, { method: 'POST', body: new URLSearchParams({ url: link(`http://${HOST}/sync/7kq2mx9pab4c`) }) });
  check('a link with an http address opens the set-up with the address error', await until(async () => (await dialog().count()) === 1 && (await dialog().getAttribute('aria-label')) === 'Set up sync' &&
    (await errorUnder('Address').count()) === 1 && (await errorUnder('Address').innerText()) === 'Sync needs an https address') && (await field('Address').inputValue()) === `http://${HOST}/sync/7kq2mx9pab4c`);
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
  await field('Address').fill(bucket);
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
  check('Set up again… opens the Dialog empty', await shown(dialog()) && (await field('Address').inputValue()) === '' && (await field('Token').inputValue()) === '');
  await field('Address').fill(bucket);
  await field('Token').fill(token2);
  await button('Connect').click();
  await until(async () => (await dialog().getAttribute('aria-label')) === 'Enter your sync passphrase');
  await field('Passphrase').fill(changed);
  told = sends({ t: 'syncStartFresh' });
  await button('Turn on sync').click();
  check('Turn on sync sends syncStartFresh first, as the keys here don\'t open', await told);
  check('and syncs again with the passphrase, starting the profile\'s keys afresh', await until(async () => (await dialog().count()) === 0 && (await button('Sync now', card()).isVisible()), 10000));

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
  const flow = [1, 2, 3, 4, 15, 16, 17, 18, 5, 6, 20, 21, 19, 7, 8, 9, 10, 11, 12, 23, 13, 22, 14];
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
