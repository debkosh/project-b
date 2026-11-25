// A page's passkey request on the mock engine (docs/passkeys.md), on every desktop shell:
// each step's card with Go's words and buttons, each answer reaching the engine, the PIN
// field's own errors, the phone's code, the keyboard's place at each step, and Escape; and
// our own passkeys' steps: Save, the passkeys here, the OS's check and our errors. The mock
// plays the steps a key, a phone or our own would bring (core/mockengine/passkeys.go).
// It leaves light and dark screenshots in build/e2e/passkey-<shell>-<step>-*.png, and in
// PASSKEY_SHOTS too when that names a folder.
// Run: node shell/e2e/passkeys.cjs [shell ...]   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { fire, regions, settle, until } = lib;

const registry = JSON.parse(fs.readFileSync(path.join(lib.root, 'core/shells/registry.json'), 'utf8'));
const shells = process.argv.length > 2 ? process.argv.slice(2) : registry.filter((s) => !s.phone).map((s) => s.id);
const shots = process.env.PASSKEY_SHOTS;
if (shots) fs.mkdirSync(shots, { recursive: true });

(async () => {
  const browser = await chromium.launch();
  for (const id of shells) {
    console.log(`--- part ${id}`);
    const check = (name, ok, detail) => lib.check(`${id}: ${name}`, ok, detail);
    // Each shell starts with no passkeys: the profile keeps every one saved, and the checks
    // below count them.
    for (const file of ['store.db', 'store.db-wal', 'store.db-shm']) fs.rmSync(path.join(lib.profile, file), { force: true });
    await lib.start([`-use=${id}`]);
    const { page, ui, errors } = await lib.openWindow(browser);
    page.setDefaultTimeout(5000);
    await until(async () => (await regions()).shell === id, 10000);
    await fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'newTab', text: 'https://start.test/' }) });
    // The new tab is the one shown and loaded before the page in it asks anything.
    const up = async () => {
      const r = await regions();
      const tab = r.tabs.find((t) => t.id === r.active);
      return r.visible.length === 1 && r.visible[0].tab === r.active && !!tab?.title.includes('start.test');
    };
    check('comes up with a tab', await until(up, 10000), JSON.stringify(await regions()).slice(0, 600));

    const card = ui.getByRole('alertdialog');
    const button = (name) => card.getByRole('button', { name, exact: true });
    const buttons = () => card.locator('.buttons button').allInnerTexts();
    const answers = async () => (await regions()).passkeyAnswers || [];
    const last = async () => (await answers()).at(-1) || {};
    const step = (name, carries) => fire('passkeyStep', carries ? `${name} ${JSON.stringify(carries)}` : name);
    const named = (title) => until(async () => (await card.count()) === 1 && (await card.getAttribute('aria-label')) === title && (await card.isVisible()));
    const focused = () => card.evaluate((el) => {
      const a = el.getRootNode().activeElement;
      return a && el.contains(a) ? (a.tagName === 'INPUT' ? `field ${a.labels?.[0]?.textContent}` : a.textContent.trim()) : '';
    });
    const shoot = async (name) => {
      for (const scheme of ['light', 'dark']) {
        await page.emulateMedia({ colorScheme: scheme });
        await settle(page);
        await page.screenshot({ path: path.join(lib.out, `passkey-${id}-${name}-${scheme}.png`) });
        if (shots && id === 'pbshell') await page.screenshot({ path: path.join(shots, `${name}-${scheme}.png`) });
      }
      await page.emulateMedia({ colorScheme: 'light' });
    };

    // The key step: no primary, Use a phone apart on the left, the keyboard on Cancel.
    await fire('passkey', 'create');
    check('a request shows as an alertdialog named by its step', await named('Create a passkey'));
    check('with Go\'s words, the site in bold', (await card.locator('p').first().innerText()) === 'example.com wants to save a passkey on your security key. Plug it in and touch it.' && (await card.locator('p b').innerText()) === 'example.com');
    check('its buttons are Use a phone and Cancel, with no primary', JSON.stringify(await buttons()) === '["Use a phone","Cancel"]' && (await card.locator('.text-button.primary').count()) === 0, JSON.stringify(await buttons()));
    const other = await button('Use a phone').boundingBox();
    const cancel = await button('Cancel').boundingBox();
    const box = await card.boundingBox();
    check('Use a phone is at the left and Cancel at the right', other.x - box.x < 40 && box.x + box.width - (cancel.x + cancel.width) < 40, JSON.stringify({ other, cancel, box }));
    check('the keyboard is on Cancel', await until(async () => (await focused()) === 'Cancel'), await focused());
    await shoot('key');

    // The PIN: the field has the keyboard; a short one is ours to refuse, under the field.
    await step('pin', { reason: 'enter', error: '', attempts: 8, minLength: 4 });
    check('the PIN step has a password field named PIN, with the keyboard', (await named('Enter your security key\'s PIN')) && (await until(async () => (await focused()) === 'field PIN')) && (await card.getByLabel('PIN', { exact: true }).getAttribute('type')) === 'password', await focused());
    check('its primary is Next, beside Cancel', JSON.stringify(await buttons()) === '["Cancel","Next"]' && (await card.locator('.text-button.primary').innerText()) === 'Next');
    const pin = card.getByLabel('PIN', { exact: true });
    await pin.fill('12');
    const before = (await answers()).length;
    await button('Next').click();
    check('Next with a short PIN says so under the field, and sends nothing', (await until(async () => (await card.locator('.text-field-error').count()) === 1)) && (await card.locator('.text-field-error').innerText()) === 'At least 4 characters' && (await answers()).length === before);
    check('and the keyboard stays in the field', (await focused()) === 'field PIN');
    await pin.fill('1234');
    check('typing takes the words away', (await card.locator('.text-field-error').count()) === 0);
    await page.keyboard.press('Enter');
    check('Enter sends the PIN', await until(async () => JSON.stringify(await last()).includes('"action":"pin","text":"1234"')), JSON.stringify(await last()));

    // The key refused it: the same step again, empty, with the key's words under the field.
    await step('pin', { reason: 'enter', error: 'wrong', attempts: 3, minLength: 4 });
    check('a wrong PIN says so under the field, which is empty again with the keyboard', (await until(async () => (await card.locator('.text-field-error').count()) === 1 && (await card.getByLabel('PIN', { exact: true }).inputValue()) === '')) && (await card.locator('.text-field-error').innerText()) === 'Wrong PIN. 3 tries left.' && (await until(async () => (await focused()) === 'field PIN')), await focused());
    await shoot('pin-wrong');

    // Setting a PIN: two fields, and the two must agree.
    await step('pin', { reason: 'set', error: '', minLength: 4 });
    check('setting a PIN has PIN and Confirm PIN, and Set PIN', (await named('Set a PIN for your security key')) && (await card.getByLabel('PIN', { exact: true }).count()) === 1 && (await card.getByLabel('Confirm PIN', { exact: true }).count()) === 1 && (await button('Set PIN').count()) === 1);
    await card.getByLabel('PIN', { exact: true }).fill('123456');
    await card.getByLabel('Confirm PIN', { exact: true }).fill('123457');
    await button('Set PIN').click();
    check('two that differ say so under Confirm PIN, with the keyboard there', (await until(async () => (await card.locator('.text-field-error').count()) === 1)) && (await card.locator('.text-field-error').innerText()) === 'The PINs don\'t match' && (await focused()) === 'field Confirm PIN', await focused());
    await card.getByLabel('Confirm PIN', { exact: true }).fill('123456');
    await page.keyboard.press('Enter');
    check('two that agree are sent', await until(async () => (await last()).text === '123456'));
    await step('pin', { reason: 'change', error: '', minLength: 4 });
    check('changing a PIN has New PIN and Confirm new PIN, and Change PIN', (await named('Change your security key\'s PIN')) && (await card.getByLabel('New PIN', { exact: true }).count()) === 1 && (await card.getByLabel('Confirm new PIN', { exact: true }).count()) === 1 && (await button('Change PIN').count()) === 1);

    await step('touch');
    check('the touch step has only Cancel', (await named('Touch your security key')) && JSON.stringify(await buttons()) === '["Cancel"]');

    // The accounts: a row each, a button, the display name over the name.
    await step('account', { accounts: [{ name: 'ada@example.com', displayName: 'Ada Lovelace' }, { name: 'grace@example.com', displayName: '' }, { name: 'a-very-long-account-name-that-cannot-fit-on-one-line-of-the-card@example.com', displayName: 'Katherine' }] });
    check('the account step counts the accounts', (await named('Choose an account')) && (await card.locator('p').first().innerText()) === 'Your security key has 3 accounts for example.com.');
    const rows = card.locator('button.account');
    check('each account is a button, the display name over the name', (await rows.count()) === 3 && (await rows.nth(0).innerText()).replace(/\s+/g, ' ') === 'Ada Lovelace ada@example.com' && (await rows.nth(1).innerText()).trim() === 'grace@example.com');
    const sizes = await rows.evaluateAll((all) => all.map((r) => [r.offsetHeight, r.offsetWidth, r.parentElement.clientWidth, r.scrollWidth <= r.clientWidth]));
    check('a row is 48px high, the list\'s full width, on one line', sizes.every(([h, w, list, fits]) => h === 48 && w === list && fits), JSON.stringify(sizes));
    const look = await card.locator('.accounts').evaluate((el) => {
      const s = getComputedStyle(el);
      return { border: s.borderTopWidth, radius: s.borderTopLeftRadius, gap: el.getBoundingClientRect().top - el.previousElementSibling.getBoundingClientRect().bottom, between: getComputedStyle(el.children[1]).borderTopWidth, first: getComputedStyle(el.children[0]).borderTopWidth, pad: getComputedStyle(el.children[0]).padding };
    });
    check('the rows sit in a card 16px under the words, a hairline round it and between them', look.border === '1px' && (look.radius === '6px' || id === 'mnshell') && Math.abs(look.gap - 16) < 1 && look.between === '1px' && look.first === '0px' && look.pad === '8px 12px', JSON.stringify(look));
    check('a long name is cut with its whole in the tooltip', (await rows.nth(2).getAttribute('title')).includes('a-very-long-account-name-that-cannot-fit-on-one-line-of-the-card@example.com'));
    await shoot('account');
    await rows.nth(1).click();
    check('a row sends its index', await until(async () => JSON.stringify(await last()).includes('"action":"account","text":"1"')), JSON.stringify(await last()));
    await step('account', { accounts: [{ name: '', displayName: '' }, { name: 'ada@example.com', displayName: '' }] });
    await until(async () => (await rows.count()) === 2);
    const muted = await rows.evaluateAll((all) => all.map((r) => [r.textContent.trim(), getComputedStyle(r.querySelector('.name')).color]));
    check('an account with no name says No name, in the muted colour', muted[0][0] === 'No name' && muted[0][1] !== muted[1][1], JSON.stringify(muted));
    await step('account', { accounts: Array.from({ length: 9 }, (_, i) => ({ name: `user${i}@example.com`, displayName: `User ${i}` })) });
    await until(async () => (await rows.count()) === 9);
    const view = page.viewportSize();
    const far = await button('Cancel').boundingBox();
    check('past five or so the list scrolls inside the card, Cancel still in view', (await card.locator('.accounts').evaluate((el) => el.scrollHeight > el.clientHeight)) && far.y + far.height < view.height, JSON.stringify(far));

    // The phone: the code, and the Bluetooth line when it isn't on.
    await step('key', { request: 'get', site: 'example.com', phone: true });
    check('signing in has its own words', (await named('Sign in with a passkey')) && (await card.locator('p').first().innerText()) === 'example.com wants you to sign in with your security key. Plug it in and touch it.');
    await button('Use a phone').click();
    check('Use a phone goes to the phone step', (await named('Use your phone')) && (await last()).action === 'phone');
    const code = card.getByRole('img', { name: 'QR code for your phone' });
    const tile = await card.locator('.code').evaluate((el) => {
      const s = getComputedStyle(el);
      const r = el.getBoundingClientRect();
      const c = el.parentElement.getBoundingClientRect();
      return { w: r.width, h: r.height, bg: s.backgroundColor, radius: s.borderTopLeftRadius, off: Math.abs(r.left + r.width / 2 - (c.left + c.width / 2)) };
    });
    check('its code is an image named for the phone, 200px on a white tile, centred', (await code.count()) === 1 && tile.w === 200 && tile.h === 200 && tile.bg === 'rgb(255, 255, 255)' && tile.radius === '4px' && tile.off < 1, JSON.stringify(tile));
    const drawn = await code.evaluate((svg) => ({ box: svg.getAttribute('viewBox'), dark: svg.querySelector('path').getAttribute('d').split('M').length - 1, fill: svg.querySelector('path').getAttribute('fill') }));
    check('the code has its quiet zone and its dark modules', Number(drawn.box.split(' ')[2]) >= 21 + 8 && drawn.dark > 100 && drawn.fill === '#000', JSON.stringify(drawn));
    check('with Bluetooth on there is no line about it, and no primary', (await card.locator('.bluetooth').count()) === 0 && JSON.stringify(await buttons()) === '["Use a security key","Cancel"]');
    await page.emulateMedia({ colorScheme: 'dark' });
    await settle(page);
    check('the tile is white in the dark theme too', (await card.locator('.code').evaluate((el) => getComputedStyle(el).backgroundColor)) === 'rgb(255, 255, 255)');
    await page.emulateMedia({ colorScheme: 'light' });
    await step('phone', { link: 'FIDO:/0123456789012345678901234567890123456789', bluetooth: 'off', canTurnOn: true });
    const line = card.locator('.bluetooth');
    check('Bluetooth off says so under the code, with Turn on Bluetooth on the same row', (await until(async () => (await line.count()) === 1)) && (await line.locator('span').innerText()) === 'Bluetooth is off.' && (await line.getByRole('button', { name: 'Turn on Bluetooth' }).count()) === 1);
    const gap = await card.evaluate((el) => el.querySelector('.bluetooth').getBoundingClientRect().top - el.querySelector('.code').getBoundingClientRect().bottom);
    check('12px under the code', Math.abs(gap - 12) < 1, String(gap));
    const turnOn = await line.evaluate((el) => {
      const b = el.querySelector('button');
      const w = el.querySelector('span').getBoundingClientRect();
      const r = b.getBoundingClientRect();
      return { quiet: b.classList.contains('quiet'), size: getComputedStyle(b).fontSize, border: getComputedStyle(b).borderTopWidth, off: Math.abs(w.top + w.height / 2 - (r.top + r.height / 2)) };
    });
    check('Turn on Bluetooth is a normal button at 13px, centred on the row', !turnOn.quiet && turnOn.size === '13px' && turnOn.border === '1px' && turnOn.off < 1, JSON.stringify(turnOn));
    await shoot('phone-bluetooth-off');
    await line.getByRole('button', { name: 'Turn on Bluetooth' }).click();
    check('Turn on Bluetooth asks the engine', await until(async () => (await last()).action === 'bluetoothOn'));
    await step('phone', { link: 'FIDO:/0123456789012345678901234567890123456789', bluetooth: 'denied', canTurnOn: false });
    check('Bluetooth refused says where to allow it, with Open System Settings', (await until(async () => (await line.innerText()).includes('Project B can\'t use Bluetooth. Allow it in System Settings › Privacy & Security › Bluetooth.'))) && (await line.getByRole('button', { name: 'Open System Settings' }).count()) === 1);
    await line.getByRole('button', { name: 'Open System Settings' }).click();
    check('which asks the engine to open them', await until(async () => (await last()).action === 'bluetoothSettings'));
    await step('phoneBusy');
    check('a phone with the request shows Check your phone, with only Cancel', (await named('Check your phone')) && JSON.stringify(await buttons()) === '["Cancel"]');
    await step('phone', { link: 'FIDO:/0123456789012345678901234567890123456789', bluetooth: 'on', canTurnOn: false });
    await named('Use your phone');
    await button('Use a security key').click();
    check('Use a security key goes back to the key step', (await named('Create a passkey')) && (await last()).action === 'key');

    // Errors: one primary, Try again where it can help.
    await step('error', { reason: 'already-registered' });
    check('an error that trying again can\'t help has Close alone, the primary', (await named('Already saved')) && JSON.stringify(await buttons()) === '["Close"]' && (await card.locator('.text-button.primary').innerText()) === 'Close' && (await until(async () => (await focused()) === 'Close')));
    await step('error', { reason: 'timeout' });
    check('one it can help has Close and Try again, the primary, with the keyboard', (await named('That took too long')) && JSON.stringify(await buttons()) === '["Close","Try again"]' && (await card.locator('.text-button.primary').innerText()) === 'Try again' && (await until(async () => (await focused()) === 'Try again')), JSON.stringify(await buttons()));
    check('its words start with the site, in bold', (await card.locator('p').first().innerText()) === 'example.com stopped waiting. Try again when your key or phone is ready.' && (await card.locator('p b').innerText()) === 'example.com');
    await shoot('error-timeout');
    await button('Try again').click();
    check('Try again starts over at the key step', (await named('Create a passkey')) && (await last()).action === 'retry');

    // Tab stays in the card; another tab hides it; Escape cancels.
    await button('Cancel').focus();
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    check('Tab goes round the card\'s controls', (await focused()) === 'Cancel', await focused());
    if (id === 'pbshell') {
      const t1 = (await regions()).active;
      await fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'newTab', text: 'https://b.test/' }) });
      check('another tab shown hides it', await until(async () => (await card.count()) === 0 && (await regions()).active !== t1));
      await ui.locator(`[role=tab][data-tab="${t1}"]`).click();
      check('and showing its tab brings it back, at its step', await named('Create a passkey'));
    }
    await page.keyboard.press('Escape');
    check('Escape cancels the request, and the card goes', await until(async () => (await card.count()) === 0 && (await last()).action === 'cancel'), JSON.stringify(await last()));

    // The engine ending it, as when the key answered.
    await fire('passkey', 'get');
    await named('Sign in with a passkey');
    const n = (await answers()).length;
    await step('end');
    check('the request ending takes the card away, unanswered', (await until(async () => (await card.count()) === 0)) && (await answers()).length === n);

    // Our own passkeys (docs/passkeys.md, Our own passkeys): Save first, with the way to a
    // security key or a phone; the card away while the OS checks the person; the site's
    // passkeys as rows, the keyboard on the first; and our own errors.
    const checks = async () => (await regions()).personChecks || [];
    const heard = async (action) => (await answers()).filter((a) => a.action === action);
    await fire('passkey', 'create own');
    check('a make offers Project B first', (await named('Create a passkey')) && (await card.locator('p').first().innerText()) === 'Save a passkey for example.com in Project B?' && (await card.locator('p b').innerText()) === 'example.com');
    check('with Use a security key or phone, Cancel and Save, the primary', JSON.stringify(await buttons()) === '["Use a security key or phone","Cancel","Save"]' && (await card.locator('.text-button.primary').innerText()) === 'Save', JSON.stringify(await buttons()));
    check('the keyboard is on Save', await until(async () => (await focused()) === 'Save'), await focused());
    const row = await card.locator('.buttons button').evaluateAll((all) => all.map((b) => { const r = b.getBoundingClientRect(); return [Math.round(r.left), Math.round(r.right), Math.round(r.top)]; }));
    const wide = await card.boundingBox();
    check('the card is 440px wide, its three buttons on one row inside it', wide.width === 440 && row.every((b) => b[2] === row[0][2] && b[0] >= wide.x && b[1] <= wide.x + wide.width) && row[0][1] <= row[1][0], JSON.stringify({ wide, row }));
    await shoot('save');
    await button('Use a security key or phone').click();
    check('Use a security key or phone goes to the key step, and the engine hears nothing of it', (await until(async () => (await card.locator('p').first().innerText().catch(() => '')) === 'example.com wants to save a passkey on your security key. Plug it in and touch it.')) && (await heard('other')).length === 0 && (await heard('own')).length === 0);
    await button('Cancel').click();
    await until(async () => (await card.count()) === 0);

    // Save: the OS checks the person, and the card is away meanwhile.
    await fire('personCheck', 'hold');
    await fire('passkey', 'create own');
    await named('Create a passkey');
    const asked = (await checks()).length;
    await page.keyboard.press('Enter');
    check('Enter saves: the card goes while the OS checks the person', (await until(async () => (await card.count()) === 0 && (await checks()).length === asked + 1)) && (await heard('own')).length === 1, JSON.stringify(await checks()));
    check('the OS is asked for this site, by name', JSON.stringify((await checks()).at(-1)) === '{"Reason":"save a passkey for example.com","Title":"Save your passkey","Subtitle":"example.com"}', JSON.stringify((await checks()).at(-1)));
    check('nothing is made until the person is checked', (await heard('made')).length === 0);
    await fire('personCheck', 'answer verified');
    check('checked, the passkey is made and the engine has it', await until(async () => (await heard('made')).length === 1 && /"credentialId":"[\w-]{22}"/.test((await heard('made'))[0].text)), JSON.stringify(await last()));
    await fire('passkey', 'create own user=work');
    await named('Create a passkey');
    await button('Save').click();
    await until(async () => (await heard('made')).length === 2);

    // A sign-in: the site's passkeys here, the keyboard on the first.
    await fire('passkey', 'get own');
    check('a sign-in shows the passkeys here, under the site', (await named('Sign in to example.com')) && (await card.locator('button.account').count()) === 2, await card.innerText().catch(() => ''));
    const mine = card.locator('button.account');
    check('each a row, the display name over the name', (await mine.allInnerTexts()).map((t) => t.replace(/\s+/g, ' ')).sort().join('|') === 'Asha Rao asha@example.com|Work Rao work@example.com', JSON.stringify(await mine.allInnerTexts()));
    check('its buttons are Use a security key or phone and Cancel, with no primary', JSON.stringify(await buttons()) === '["Use a security key or phone","Cancel"]' && (await card.locator('.text-button.primary').count()) === 0, JSON.stringify(await buttons()));
    const first = (await mine.nth(0).innerText()).replace(/\s+/g, ' ');
    check('the keyboard is on the first passkey', await until(async () => (await focused()).replace(/\s+/g, '') === first.replace(/\s+/g, '')), await focused());
    // The pointer aside, so no row shows as hovered.
    await page.mouse.move(1, 1);
    await shoot('own');
    const uses = (await checks()).length;
    await page.keyboard.press('Enter');
    check('Enter chooses it: the OS checks, and the engine has the signature', await until(async () => (await heard('signed')).length === 1 && (await checks()).length === uses + 1 && (await card.count()) === 0), JSON.stringify(await last()));
    check('the OS is asked to use it', (await checks()).at(-1).Reason === 'use a passkey for example.com');
    const made = JSON.parse((await heard('made'))[0].text).credentialId;
    check('it signed with the first row\'s passkey', (JSON.parse((await heard('signed'))[0].text).credentialId === made) === first.startsWith('Asha'), JSON.stringify(await last()));

    // The person saying no to the check ends the request, with no card.
    await fire('personCheck', 'denied');
    await fire('passkey', 'get own');
    await named('Sign in to example.com');
    await mine.nth(1).click();
    check('no to the check ends the request as Cancel does', await until(async () => (await card.count()) === 0 && (await last()).action === 'ownFailed' && (await last()).text === 'denied'), JSON.stringify(await last()));
    await fire('personCheck', 'verified');

    // The site already has the passkey.
    await fire('passkey', `create own exclude=${made}`);
    await named('Create a passkey');
    await button('Save').click();
    check('a passkey the site already has says Already saved, with Close alone, the primary', (await named('Already saved')) && (await card.locator('p').first().innerText()) === 'Project B already has a passkey for example.com for this account.' && JSON.stringify(await buttons()) === '["Close"]' && (await until(async () => (await focused()) === 'Close')), JSON.stringify(await buttons()));
    check('that card is the usual width', (await card.boundingBox()).width === 360);
    await shoot('already-saved');
    const failures = (await heard('ownFailed')).length;
    await button('Close').click();
    check('Close ends it, and the engine hears why', await until(async () => (await card.count()) === 0 && (await heard('ownFailed')).length === failures + 1 && (await last()).text === 'excluded'), JSON.stringify(await last()));

    // A second passkey for an account that has one here, from a site that names none to
    // leave out: both stay, and a sign-in lists every one, the two for the one account
    // each saying when it was made.
    await fire('passkey', 'create own');
    await named('Create a passkey');
    await button('Save').click();
    await until(async () => (await heard('made')).length === 3);
    await fire('passkey', 'get own');
    await named('Sign in to example.com');
    const every = async () => (await mine.allInnerTexts()).map((t) => t.replace(/\s+/g, ' '));
    // Both were made just now: today, so with the time, unless midnight fell between them.
    const twice = /^Asha Rao asha@example\.com · Made (today, \d\d:\d\d|today|\d{1,2} [A-Z][a-z]{2})$/;
    check(
      'a second passkey for one account stays beside the first: a sign-in lists both together, each with when it was made, then the other account\'s',
      (await until(async () => (await mine.count()) === 3)) && twice.test((await every())[0]) && twice.test((await every())[1]) && (await every())[2] === 'Work Rao work@example.com',
      JSON.stringify(await every()),
    );
    await page.mouse.move(1, 1);
    await shoot('own-two-for-one');
    await button('Cancel').click();
    await until(async () => (await card.count()) === 0);

    check('no errors in the console', errors.length === 0, errors.join(' | '));
    await page.close();
    await lib.stop();
  }
  // As Linux, which has no check of the person (docs/passkeys.md, Our own passkeys): Save
  // saves and a passkey here signs in with nothing asked, the keyring being what stands
  // before a passkey there; and with a keyring that gave no key, Save says the keyring's line.
  for (const locked of [false, true]) {
    console.log(`--- part linux${locked ? ', its keyring locked' : ''}`);
    const check = (name, ok, detail) => lib.check(`linux${locked ? ', its keyring locked' : ''}: ${name}`, ok, detail);
    for (const file of ['store.db', 'store.db-wal', 'store.db-shm']) fs.rmSync(path.join(lib.profile, file), { force: true });
    await lib.start(['-use=pbshell', '-platform=linux', ...(locked ? ['-keyring-locked'] : [])]);
    const { page, ui } = await lib.openWindow(browser);
    page.setDefaultTimeout(5000);
    await until(async () => (await regions()).shell === 'pbshell', 10000);
    await fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'newTab', text: 'https://start.test/' }) });
    await until(async () => {
      const r = await regions();
      return r.visible.length === 1 && r.visible[0].tab === r.active && !!r.tabs.find((t) => t.id === r.active)?.title.includes('start.test');
    }, 10000);
    const card = ui.getByRole('alertdialog');
    const named = (title) => until(async () => (await card.count()) === 1 && (await card.getAttribute('aria-label')) === title && (await card.isVisible()));
    const heard = async (action) => ((await regions()).passkeyAnswers || []).filter((a) => a.action === action);
    const checks = async () => ((await regions()).personChecks || []).length;
    await fire('passkey', 'create own');
    await named('Create a passkey');
    await card.getByRole('button', { name: 'Save', exact: true }).click();
    if (locked) {
      check('Save says the keyring\'s line, with Close alone', (await named('Your keyring is locked')) && (await card.locator('p').first().innerText()) === 'Unlock it when asked; if it doesn\'t ask, log out and back in.' && JSON.stringify(await card.locator('.buttons button').allInnerTexts()) === '["Close"]',
        await card.innerText().catch(() => ''));
      await card.getByRole('button', { name: 'Close', exact: true }).click();
      check('and Close ends the request, nothing made', (await until(async () => (await card.count()) === 0)) && (await heard('made')).length === 0);
    } else {
      check('Save saves the passkey, with nothing asked of the person', await until(async () => (await heard('made')).length === 1 && (await checks()) === 0 && (await card.count()) === 0), await card.innerText().catch(() => ''));
      await fire('passkey', 'get own');
      await named('Sign in to example.com');
      await page.keyboard.press('Enter');
      check('and a passkey here signs in, with nothing asked', await until(async () => (await heard('signed')).length === 1 && (await checks()) === 0 && (await card.count()) === 0), JSON.stringify((await regions()).passkeyAnswers));
    }
    await page.close();
    await lib.stop();
  }
  await browser.close();
  await lib.done();
})().catch(async (e) => {
  console.log(`FAIL the run ended early -> ${e.stack}`);
  process.exit(1);
});
