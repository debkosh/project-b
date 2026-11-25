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
