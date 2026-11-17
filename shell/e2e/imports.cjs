// Bringing things over from another browser on the mock engine (docs/onboarding.md), from
// browserdata's test home: a Chrome of two profiles and a Firefox of one. The welcome's
// Bring your things over lists them, a row turned on shows what it brings, and Bring over
// brings Chrome · Personal's bookmarks into a folder of their own, its history and its
// search engine, then says what came. Settings' Import lists the same, with what was
// brought, and its Import… dialog brings Firefox's bookmarks alone. A start whose home has
// no browser has no such step.
// Run: node shell/e2e/imports.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, regions, settle, until } = lib;

const flags = (value) => fs.writeFileSync(path.join(lib.profile, 'flags.json'), JSON.stringify(value));
const read = (name) => {
  try {
    return JSON.parse(fs.readFileSync(path.join(lib.profile, name), 'utf8'));
  } catch {
    return null;
  }
};
const folders = () => (read('bookmarks.json')?.entries || []).filter((e) => e.kind === 'folder' && e.parent === '').map((e) => e.title);

(async () => {
  const browser = await chromium.launch();

  flags({ showWelcome: 'on' });
  await lib.start(['-use=standard', `-browsers=${lib.browsers}`]);
  let w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  const welcome = w.page.frameLocator('iframe.our-page[data-page="welcome"]');
  await welcome.getByRole('button', { name: 'Get started' }).click();
  check('Get started goes to Bring your things over', await until(() => welcome.getByRole('heading', { name: 'Bring your things over' }).isVisible()));
  const rows = welcome.getByRole('switch', { name: /^Bring over from / });
  check('a row for each profile found', await until(async () => (await rows.count()) === 3));
  check('each says what it holds', await welcome.getByText('4 bookmarks · 3 pages · Bing for search').isVisible());
  // Chrome's and Firefox's logos ship with us: the test home has neither app.
  check('each row shows its browser\'s logo', await until(async () => (await welcome.locator('img.ui-browser-icon').evaluateAll((imgs) => imgs.filter((i) => i.complete && i.naturalWidth > 0).length)) === 3));
  check('with no row on, the way on is Continue', await welcome.getByRole('button', { name: 'Continue' }).isVisible());
  await welcome.getByRole('switch', { name: 'Bring over from Chrome · Personal' }).check();
  const boxes = welcome.getByRole('group', { name: 'What to bring over from Chrome · Personal' }).getByRole('checkbox');
  check('a row turned on shows what it brings, all ticked', (await boxes.count()) === 3 && (await boxes.evaluateAll((b) => b.every((x) => x.checked))));
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-bring.png') });

  await welcome.getByRole('button', { name: 'Bring over' }).click();
  check('Bring over says what came', await until(() => welcome.getByRole('heading', { name: 'Brought over' }).isVisible()));
  const lines = welcome.getByRole('list', { name: 'Brought over from Chrome · Personal' });
  check('its bookmarks in their folder, its history and its search engine', await until(async () => {
    const t = await lines.innerText().catch(() => '');
    return t.includes('4 bookmarks, in 5 folders, under From Chrome · Personal') && t.includes('3 pages of history') && t.includes('Bing as the search engine');
  }), await lines.innerText().catch(() => ''));
  check('the bookmarks are in From Chrome · Personal', folders().includes('From Chrome · Personal'), JSON.stringify(folders()));
  check('Bing is the search engine', read('prefs.json')?.searchEngine === 'bing', JSON.stringify(read('prefs.json')));
  check('imports.json keeps what came', read('imports.json')?.['chrome/Default']?.pages === 3);
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-brought.png') });
  await welcome.getByRole('button', { name: 'Continue' }).click();
  check('Continue goes on to the style', await until(() => welcome.getByRole('heading', { name: 'Pick your style' }).isVisible()));
  await welcome.getByRole('button', { name: 'Skip setup' }).click();
  check('it logged no errors', w.errors.length === 0, w.errors.join(' | '));

  // Settings' Import: the same profiles, with what was brought over.
  await lib.fire('menu', 'app.settings');
  await w.settings.getByRole('button', { name: 'Import', exact: true }).click();
  check('Import lists the profiles, each with Import…', await until(async () => (await w.settings.getByRole('button', { name: /^Import from / }).count()) === 3));
  check('and says what Chrome · Personal brought', await w.settings.getByText('Brought over today: 4 bookmarks, 3 pages, Bing').isVisible());
  await w.settings.getByRole('button', { name: 'Import from Firefox · default-release' }).click();
  const dialog = w.settings.getByRole('dialog', { name: 'Import from Firefox · default-release' });
  check('Import… asks what to bring, all ticked', await until(() => dialog.isVisible()) && (await dialog.getByRole('checkbox').count()) === 3);
  await dialog.getByRole('checkbox', { name: /History/ }).uncheck();
  await dialog.getByRole('checkbox', { name: /Search engine/ }).uncheck();
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'settings-import.png') });
  await dialog.getByRole('button', { name: 'Import', exact: true }).click();
  const fx = w.settings.getByRole('list', { name: 'Brought over from Firefox · default-release' });
  check('it brings the bookmarks alone', await until(async () => (await fx.innerText().catch(() => '')).includes('2 bookmarks, in 4 folders, under From Firefox')), await fx.innerText().catch(() => ''));
  check('into From Firefox, after Chrome\'s', JSON.stringify(folders()) === JSON.stringify(['From Chrome · Personal', 'From Firefox']), JSON.stringify(folders()));
  check('and no history', read('imports.json')?.['firefox/test.default-release']?.pages === 0);
  check('Settings logged no errors', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();

  // No browser in the home folder: no such step.
  await lib.start(['-use=standard']);
  w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  const again = w.page.frameLocator('iframe.our-page[data-page="welcome"]');
  await again.getByRole('button', { name: 'Get started' }).click();
  check('with no other browser, Get started goes to the style', await until(() => again.getByRole('heading', { name: 'Pick your style' }).isVisible()));
  check('and the steps are four', (await again.getByRole('list', { name: 'Steps' }).getByRole('listitem').count()) === 4);
  await w.page.close();
  await browser.close();
  await lib.done();
})();
