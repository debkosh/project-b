// The welcome on the mock engine (docs/onboarding.md): with the flag Show the welcome at each
// start on, a start opens it over the whole window, Standard's bars hidden but for a strip at
// the top to drag the window by. A colour picked there is the profile's at once: the window,
// the page and the shell take it. Start browsing gives the window back to the shell and keeps
// that it was finished; Settings' Run setup again opens it in a tab.
// Run: node shell/e2e/welcome.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, regions, settle, until } = lib;

const flags = (value) => fs.writeFileSync(path.join(lib.profile, 'flags.json'), JSON.stringify(value));
const welcomeTab = (r) => (r.tabs || []).find((t) => t.page === 'welcome');
const done = () => {
  try {
    return JSON.parse(fs.readFileSync(path.join(lib.profile, 'welcome.json'), 'utf8')).done;
  } catch {
    return undefined;
  }
};

// What a window's shell shows of itself: a tab, or a field to type in, as Standard's empty
// window has.
const bars = async (w) => (await w.ui.getByRole('tab').first().isVisible()) || (await w.ui.getByRole('combobox').first().isVisible());

(async () => {
  const browser = await chromium.launch();

  flags({ showWelcome: 'on' });
  await lib.start(['-use=standard']);
  let w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  const welcome = w.page.frameLocator('iframe.our-page[data-page="welcome"]');
  check('a start with the flag on opens the welcome, its tab the active one', await until(async () => {
    const r = await regions();
    return welcomeTab(r) && r.active === welcomeTab(r).id && r.visible.some((v) => v.tab === r.active);
  }));
  check('the shell draws no bars around it', await until(async () => !(await bars(w))));
  let r = await regions();
  check('a strip across the top drags the window', r.drag.length === 1 && r.drag[0].y === 0 && r.drag[0].h === 40 && (await regions({ x: 600, y: 20 })).hit === 'drag', JSON.stringify(r.drag));
  check('and the page has the rest of it', (await regions({ x: 600, y: 400 })).hit === 'page');
  check('welcome.json says it is not finished', done() === false, String(done()));
  await welcome.getByRole('button', { name: 'Get started' }).waitFor();
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-hello.png') });

  await welcome.getByRole('button', { name: 'Get started' }).click();
  await welcome.getByRole('radio', { name: 'Blue' }).click();
  const rootOf = (name) => welcome.locator(':root').evaluate((el, n) => getComputedStyle(el).getPropertyValue(n).trim(), name);
  check('a colour picked is the window\'s and the page\'s at once', await until(async () => (await regions()).windowColor === '#5b7fd6' && (await rootOf('--frame')) === '#5b7fd6'));
  await welcome.getByRole('radio', { name: 'Tabs on top' }).click();
  check('Tabs on top is the profile\'s tab layout', await until(() => JSON.parse(fs.readFileSync(path.join(lib.profile, 'prefs.json'), 'utf8')).tabLayout === 'horizontal'));
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-style.png') });
  await welcome.getByRole('button', { name: 'Continue' }).click();
  await welcome.getByRole('button', { name: 'Continue' }).click();
  // Make default browser asks the system, which the mock answers yes for.
  await welcome.getByRole('button', { name: 'Make default browser' }).click();
  check('Make default browser asks, and the page says it is the default', await until(() => welcome.getByText('This is your default browser').isVisible()));
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-done.png') });
  await welcome.getByRole('button', { name: 'Start browsing' }).click();
  check('Start browsing closes it and the shell comes back', await until(async () => !welcomeTab(await regions()) && (await bars(w))));
  check('and welcome.json says it was finished', done() === true, String(done()));
  check('it logged no errors', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();

  // Finished, and the flag off: no welcome; Settings' Run setup again opens it in a tab.
  flags({});
  await lib.start(['-use=standard']);
  w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  await until(async () => (await regions()).shell === 'standard');
  check('a start after it was finished opens none', !welcomeTab(await regions()));
  await lib.fire('menu', 'app.settings');
  await w.settings.getByRole('button', { name: 'Run setup again' }).click();
  check('Run setup again opens it in a tab, with the shell around it', await until(async () => {
    const r = await regions();
    return welcomeTab(r) && r.active === welcomeTab(r).id && (await bars(w));
  }));
  await w.page.close();
  await browser.close();
  await lib.done();
})();
