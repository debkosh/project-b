// The standard shell's pinned tabs, on the mock engine: Go's pins first in the sidebar and
// in the strip on top, with a rule after them; Pin Tab and Unpin Tab in a tab's menu, off
// on a tab with nothing in it; a pin whose tab closed stays, muted, and opens its tab
// again; its × unpins it. Light and dark, both layouts, shot for the eye.
// Run: node shell/e2e/shells/standard-pins.cjs   (from the repo root, after `npm run build`
// in shell/)

const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, settle, until } = lib;

const send = (m) => fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify(m) });
const sites = ['docs.example.com', 'mail.example.com', 'news.example.org'];

(async () => {
  await lib.start(['-use=standard']);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser, { viewport: { width: 1000, height: 600 } });
  const { page, ui } = w;
  page.setDefaultTimeout(5000);
  for (const s of sites) await send({ t: 'newTab', text: `https://${s}/` });
  const tabs = ui.locator('.sidebar [role=tab]');
  await until(async () => (await tabs.count()) === sites.length, 10000);
  const ids = await w.shell().evaluate(() => [...document.querySelectorAll('.sidebar [role=tab]')].map((e) => +e.dataset.tab));
  const tabRow = (id) => ui.locator(`.sidebar [role=tab][data-tab="${id}"]`);
  const tabMenu = ui.locator('.ui-context-menu').first();
  const item = (name) => tabMenu.getByRole('menuitem', { name, exact: true });
  const order = () => w.shell().evaluate(() => [...document.querySelectorAll('.sidebar .tabs > *')].map((e) => (e.dataset.pin ? 'pin' : e.dataset.tab ? +e.dataset.tab : e.getAttribute('role'))));

  // Pin Tab from the tab's menu: the last tab moves to the top, with a rule under it.
  await tabRow(ids[2]).click({ button: 'right' });
  check('a web tab\'s menu offers Pin Tab', await item('Pin Tab').isEnabled());
  await item('Pin Tab').click();
  check('the pinned tab goes first, with a rule after the pins', await until(async () => JSON.stringify(await order()) === JSON.stringify([ids[2], 'separator', ids[0], ids[1]])), JSON.stringify(await order()));
  check('and is drawn as pinned', await tabRow(ids[2]).evaluate((e) => e.classList.contains('pinned')));
  await tabRow(ids[2]).click({ button: 'right' });
  check('a pinned tab\'s menu offers Unpin Tab, and no group', (await item('Unpin Tab').count()) === 1 && (await item('Add Tab to Group').count()) === 0);
  await page.keyboard.press('Escape');

  // Shots, on the side and on top, light and dark.
  await tabRow(ids[0]).click();
  for (const scheme of ['light', 'dark']) {
    await page.emulateMedia({ colorScheme: scheme });
    await settle(page);
    await page.screenshot({ path: path.join(lib.out, `standard-pins-side-${scheme}.png`) });
  }
  await fire('menu', 'view.tabsTop');
  const strip = ui.locator('.strip');
  check('on top, the strip starts with the pin and a rule', await until(async () => (await strip.locator('[role=tab]').first().getAttribute('data-tab')) === String(ids[2]) && (await strip.locator('.pins-end').count()) === 1));
  for (const scheme of ['light', 'dark']) {
    await page.emulateMedia({ colorScheme: scheme });
    await settle(page);
    await page.screenshot({ path: path.join(lib.out, `standard-pins-top-${scheme}.png`) });
  }
  await page.emulateMedia({ colorScheme: 'light' });
  await fire('menu', 'view.tabsSide');
  await until(async () => (await tabs.count()) === sites.length);

  // Closing a pinned tab keeps its pin, muted; a press opens its tab again.
  await tabRow(ids[2]).hover();
  await tabRow(ids[2]).locator('.close').click();
  const closed = ui.locator('.sidebar .pin-closed');
  check('closing a pinned tab keeps its pin, muted, with no tab', await until(async () => (await closed.count()) === 1 && (await tabs.count()) === 2));
  check('the pin keeps the tab\'s title', (await closed.innerText()).includes('news.example.org'), await closed.innerText());
  await settle(page);
  await page.screenshot({ path: path.join(lib.out, 'standard-pins-closed.png') });
  await closed.click();
  check('a press on the pin opens its tab, first and active', await until(async () => (await closed.count()) === 0 && (await tabs.count()) === 3 &&
    (await ui.locator('.sidebar [role=tab].pinned[aria-selected=true]').count()) === 1));

  // Unpin from the menu: the tab stays, as an ordinary tab, and the rule goes.
  const pinned = +(await ui.locator('.sidebar [role=tab].pinned').getAttribute('data-tab'));
  await tabRow(pinned).click({ button: 'right' });
  await item('Unpin Tab').click();
  check('Unpin Tab keeps the tab, ordinary, and the rule goes', await until(async () => (await tabs.count()) === 3 && (await ui.locator('.sidebar .pinned, .sidebar .pins-end').count()) === 0));

  // The × of a pin with no tab unpins it.
  await tabRow(ids[0]).click({ button: 'right' });
  await item('Pin Tab').click();
  await until(async () => (await ui.locator('.sidebar .pinned').count()) === 1);
  // With the row focused, its × shows, as on hover.
  await tabRow(ids[0]).focus();
  await tabRow(ids[0]).locator('.close').click();
  await until(async () => (await closed.count()) === 1);
  await closed.hover();
  await closed.getByRole('button', { name: /^Unpin / }).click();
  check('a pin\'s × unpins it', await until(async () => (await closed.count()) === 0 && (await ui.locator('.sidebar .pins-end').count()) === 0));

  // A tab with nothing in it has nothing to pin.
  await send({ t: 'newTab' });
  await page.keyboard.press('Escape');
  await until(async () => (await tabs.count()) === 3);
  const blank = +(await ui.locator('.sidebar [role=tab][aria-selected=true]').getAttribute('data-tab'));
  await tabRow(blank).click({ button: 'right' });
  check('a blank tab\'s Pin Tab is off', await item('Pin Tab').isDisabled());
  await page.keyboard.press('Escape');

  check('no errors in the pages', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})();
