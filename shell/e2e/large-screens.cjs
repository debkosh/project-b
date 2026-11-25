// On Android the window's size picks the shell (docs/large-screens.md), on the mock playing
// Android: a wide window loads Standard, a narrow one Mobile, a phone on its side stays
// Mobile, and an opened foldable is Standard again, with the same tab and page throughout.

const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');

const { check, fire, regions, until } = lib;

(async () => {
  await lib.start(['-platform=android']);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 }, hasTouch: true });
  const loads = (id) => until(() => new RegExp(`/shell/${id}/`).test(w.shell()?.url() || ''), 5000);

  check('a tablet on its side loads Standard', await loads('pbshell'));
  check('with its Menu button', await until(() => w.ui.getByRole('button', { name: 'Menu' }).isVisible()));
  check('and no window buttons of its own, as Android has its own', (await w.ui.getByRole('button', { name: 'Minimise' }).count()) === 0);
  // Touch: a finger and no mouse grows Standard's targets (docs/large-screens.md).
  const touch = await w.shell().evaluate(() => matchMedia('(pointer: coarse) and (not (any-pointer: fine))').matches);
  check('the tablet is a touch screen with no mouse', touch);
  const row = await w.ui.locator('.sidebar .row.active').boundingBox();
  const button = await w.ui.getByRole('button', { name: 'Menu' }).boundingBox();
  check('a touch screen gets rows and buttons a finger hits', row?.height === 44 && button?.height === 44, `row ${row?.height}, button ${button?.height}`);
  check('and no shortcut hints', (await w.ui.locator('.sidebar .hint').count()) === 0 || !(await w.ui.locator('.sidebar .hint').first().isVisible()));
  // Shots for ux: five tabs with pages, the ☰ menu open, and tabs on top.
  const sites = ['https://news.example/', 'https://mail.example/inbox', 'https://docs.example/notes', 'https://shop.example/cart'];
  for (const u of sites) await fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'newTab', text: u }) });
  await until(async () => (await regions()).tabs.length === 5);
  await until(async () => (await regions()).tabs.every((t) => !t.loading));
  const closes = w.ui.locator('.sidebar .row.tab .close');
  const shown = await closes.evaluateAll((els) => els.map((e) => getComputedStyle(e).visibility));
  check('every tab shows its close to a finger', shown.length === 5 && shown.every((v) => v === 'visible'), shown.join(','));
  await lib.settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'large-screens-tablet-touch.png') });
  await w.ui.getByRole('button', { name: 'Menu' }).click();
  const item = w.ui.getByRole('menu', { name: 'Menu' }).getByRole('menuitem').first();
  check('the ☰ menu has rows a finger hits', (await item.boundingBox())?.height === 44, `${(await item.boundingBox())?.height}`);
  await lib.settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'large-screens-tablet-menu.png') });
  await w.page.keyboard.press('Escape');
  await fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'setTabLayout', text: 'horizontal' }) });
  check('tabs on top have tabs a finger hits', await until(async () => (await w.ui.locator('.strip .tab').first().boundingBox())?.height === 40));
  await lib.settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'large-screens-tablet-top.png') });
  await fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'setTabLayout', text: 'vertical' }) });
  await w.ui.locator('.sidebar').waitFor();
  await w.ui.locator(`.sidebar .row.tab`).first().click();
  const menu = w.ui.getByRole('button', { name: 'Menu' });
  await menu.click();
  check('its Menu opens Go\'s menus', await until(() => w.ui.getByRole('menu', { name: 'Menu' }).isVisible()));
  await w.page.keyboard.press('Escape');
  await until(async () => (await regions()).active);
  const tab = (await regions()).active;
  await fire('navigate', 'https://news.example/story', { tab });
  await until(async () => (await regions()).tabs.find((t) => t.id === tab)?.url === 'https://news.example/story');

  await w.page.setViewportSize({ width: 412, height: 915 });
  check('a phone-sized window loads Mobile', await loads('mwshell'));
  const kept = async () => {
    const r = await regions();
    return r.active === tab && r.tabs.find((t) => t.id === tab)?.url === 'https://news.example/story';
  };
  check('and keeps the tab and its page', await until(kept));

  // A narrow window with a mouse, as on a tablet: a right click opens Mobile's sheet, whose
  // rows light under the pointer (docs/large-screens.md).
  const cdp = await w.page.context().newCDPSession(w.page);
  await cdp.send('Emulation.setTouchEmulationEnabled', { enabled: false });
  check('the narrow window has a mouse', await w.shell().evaluate(() => matchMedia('(any-hover: hover)').matches));
  await fire('contextMenu', JSON.stringify({ x: 60, y: 60, link: 'https://news.example/other' }), { tab });
  const sheet = w.ui.getByRole('dialog', { name: 'Link' });
  check('a right click in Mobile opens its sheet', await until(() => sheet.isVisible()));
  const lit = sheet.getByRole('menuitem').last();
  const unlit = await lit.evaluate((e) => getComputedStyle(e).backgroundColor);
  // Once the sheet has risen, the pointer moves onto the row as a mouse does: one jump into
  // the shell's frame, which is how hover() moves it, didn't always light it in headless
  // Chromium, and a row still rising moves out from under the pointer.
  await sheet.evaluate((e) => Promise.all(e.getAnimations({ subtree: true }).map((a) => a.finished)));
  const at = await lit.boundingBox();
  await w.page.mouse.move(at.x + 4, at.y + 4);
  await w.page.mouse.move(at.x + at.width / 2, at.y + at.height / 2, { steps: 4 });
  check('its rows light under the pointer', await until(async () => (await lit.evaluate((e) => getComputedStyle(e).backgroundColor)) !== unlit), unlit);
  await w.page.keyboard.press('Escape');
  check('and Escape closes it', await until(async () => (await sheet.count()) === 0));
  // A field in a sheet keeps its own Escape: in the group's name, it undoes the rename and
  // the sheet stays; the next Escape closes the sheet.
  await fire('contextMenu', JSON.stringify({ x: 60, y: 60, link: 'https://news.example/grouped' }), { tab });
  await sheet.getByRole('menuitem', { name: 'In group' }).click();
  const dot = w.ui.locator('.strip .dot');
  await until(() => dot.isVisible());
  await dot.click();
  const groupSheet = w.ui.getByRole('dialog', { name: 'Group' });
  const field = groupSheet.getByRole('textbox', { name: 'Group name' });
  await field.click();
  const was = await field.inputValue();
  await field.pressSequentially('Renamed');
  await w.page.keyboard.press('Escape');
  check('Escape in a sheet\'s field undoes what was typed', await until(async () => (await field.inputValue()) === was), await field.inputValue());
  check('and the sheet stays', await groupSheet.isVisible());
  await w.page.keyboard.press('Escape');
  check('the next Escape closes the sheet', await until(async () => (await groupSheet.count()) === 0));

  await w.page.setViewportSize({ width: 891, height: 412 });
  await new Promise((r) => setTimeout(r, 1200));
  check('a phone on its side stays on Mobile', /\/shell\/mwshell\//.test(w.shell()?.url() || ''));

  await w.page.setViewportSize({ width: 841, height: 673 });
  check('an opened foldable loads Standard', await loads('pbshell'));
  check('and keeps the tab and its page', await until(kept));
  await lib.settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'large-screens-fold-open.png') });

  check('no page logged an error', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})();
