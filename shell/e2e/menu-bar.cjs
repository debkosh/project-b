// The menu bar off the Mac, on the mock engine as Linux (-platform=linux; docs/research/linux.md,
// "The window"): Alt alone shows it for a moment over the page's top with its first title
// focused, the arrows move along it and Enter opens a menu, Esc hides it; F10 opens the first
// menu at once, whose items run Go's actions with Linux's keys beside them; Settings' Show menu
// bar keeps it, taking its own 30px above the shell's top. On the Mac there is none of it.
// Run: node shell/e2e/menu-bar.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, regions, settle, until } = lib;

const shot = (name) => path.join(lib.out, `${name}.png`);
const prefs = () => {
  try {
    return JSON.parse(fs.readFileSync(path.join(lib.profile, 'prefs.json'), 'utf8'));
  } catch {
    return {};
  }
};
// Alt pressed and let go alone: its own key, then the modifiers as they change.
const altTap = async () => {
  await lib.fire('key', 'alt');
  await lib.fire('mods', '2');
  await lib.fire('mods', '0');
};

(async () => {
  const browser = await chromium.launch();
  await lib.start(['-use=standard', '-platform=linux']);
  let w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  await until(async () => (await regions()).shell === 'standard');
  const bar = () => w.ui.getByRole('menubar', { name: 'Menu bar' });
  // Where the shell's top row starts: a bar shown for a moment lies over it, so it stays.
  const rowTop = () => w.shell().evaluate(() => document.querySelector('header.top')?.getBoundingClientRect().top);
  const top = await rowTop();
  check('Linux starts with no menu bar showing', (await bar().count()) === 0);

  await altTap();
  check('Alt alone shows it, over the page\'s top', await until(() => bar().isVisible()));
  const titles = await bar().getByRole('menuitem').allTextContents();
  check('its titles are every menu of Go\'s bar, with no app menu', ['File', 'Edit', 'View', 'History', 'Bookmarks', 'Tab', 'Profiles', 'Window', 'Help'].every((t) => titles.includes(t)) && !titles.includes('Project B'), titles.join(', '));
  const cover = await w.shell().evaluate(() => ({ bar: document.querySelector('.menu-bar').getBoundingClientRect().bottom, row: document.querySelector('header.top').getBoundingClientRect().bottom }));
  check('for a moment it covers the shell\'s first row whole', cover.bar >= cover.row, JSON.stringify(cover));
  check('with the first title focused', await until(async () => (await w.shell().evaluate(() => document.activeElement?.textContent?.trim())) === titles[0]));
  check('and the shell\'s top row doesn\'t move', typeof top === 'number' && (await rowTop()) === top, `${top} -> ${await rowTop()}`);
  await w.page.keyboard.press('ArrowRight');
  check('→ moves to the next title', await until(async () => (await w.shell().evaluate(() => document.activeElement?.textContent?.trim())) === titles[1]));
  await w.page.keyboard.press('Enter');
  const open = w.ui.getByRole('menu', { name: titles[1] });
  check('Enter opens its menu', await until(() => open.isVisible()));
  await w.page.keyboard.press('ArrowRight');
  check('→ in a menu moves to the next one', await until(() => w.ui.getByRole('menu', { name: titles[2] }).isVisible()));
  await w.page.keyboard.press('Escape');
  check('Esc closes it and hides the bar', await until(async () => (await bar().count()) === 0));

  // F10: the first menu at once, its items with Linux's keys; choosing one runs it and hides
  // the bar: View's Tabs on Top, which Go keeps in the prefs.
  await lib.fire('key', 'f10'); // does: app.menuBar
  const first = w.ui.getByRole('menu', { name: titles[0] });
  check('F10 opens the first menu at once', await until(() => first.isVisible()));
  const newTab = first.getByRole('menuitem', { name: /New Tab/ });
  check('New Tab shows Linux\'s key, Ctrl+T', (await newTab.textContent()).includes('Ctrl+T'), await newTab.textContent());
  for (const scheme of ['light', 'dark']) {
    await w.page.emulateMedia({ colorScheme: scheme });
    await settle(w.page);
    await w.page.screenshot({ path: shot(`menu-bar-linux-open-${scheme}`) });
  }
  await w.page.emulateMedia({ colorScheme: 'light' });
  // The Mac's app menu's items, where Linux apps keep them: Quit ends File, Settings ends
  // Edit, About ends Help.
  const lastItem = async (name) => (await w.ui.getByRole('menu', { name }).getByRole('menuitem').last().textContent()).replace(/\s+/g, ' ').trim();
  check('File ends with Quit, on Ctrl+Shift+Q', /^Quit Project B ?Ctrl\+Shift\+Q$/.test(await lastItem(titles[0])), await lastItem(titles[0]));
  await w.page.keyboard.press('ArrowRight');
  check('Edit ends with Settings, on Ctrl+,', await until(async () => /^Settings… ?Ctrl\+,$/.test(await lastItem('Edit').catch(() => ''))), await lastItem('Edit').catch((e) => String(e)));
  // → a menu at a time, each open before the next press, as a person's eyes would.
  for (let i = titles.indexOf('Edit') + 1; i <= titles.indexOf('Help'); i++) {
    await w.page.keyboard.press('ArrowRight');
    await until(() => w.ui.getByRole('menu', { name: titles[i] }).isVisible());
  }
  check('Help ends with About Project B', await until(async () => (await lastItem('Help').catch(() => '')) === 'About Project B'), await lastItem('Help').catch((e) => String(e)));
  await w.page.keyboard.press('Escape');
  await lib.fire('key', 'f10');
  await until(() => first.isVisible());
  for (let i = 1; i <= titles.indexOf('View'); i++) {
    await w.page.keyboard.press('ArrowRight');
    await until(() => w.ui.getByRole('menu', { name: titles[i] }).isVisible());
  }
  const viewMenu = w.ui.getByRole('menu', { name: 'View' });
  check('→ from the first menu reaches View', await until(() => viewMenu.isVisible()));
  await viewMenu.getByRole('menuitem', { name: /Tabs on Top/ }).click();
  check('choosing Tabs on Top runs it and hides the bar', await until(async () => prefs().tabLayout === 'horizontal' && (await bar().count()) === 0),
    `${prefs().tabLayout}, bar ${await bar().count()}`);

  // Show menu bar, in Settings' Appearance, off the Mac: kept, it takes its own 30px.
  await lib.fire('menu', 'app.settings');
  await w.settings.locator('.nav [data-section="appearance"]').click();
  const keep = w.settings.getByRole('switch', { name: 'Show menu bar' });
  check('Settings\' Appearance has Show menu bar, off', await until(() => keep.isVisible()) && !(await keep.isChecked()));
  check('with Alt shows it for a moment', (await w.settings.getByText('Alt shows it for a moment').count()) === 1);
  await keep.click();
  check('on, it is saved in the profile', await until(() => prefs().showMenuBar === true), JSON.stringify(prefs()));
  // With the tabs on top the strip is the title bar: Linux's own window buttons end it.
  const ends = await w.shell().evaluate(() => {
    const b = document.querySelector('.strip .window-buttons')?.getBoundingClientRect();
    return b ? { right: Math.round(b.right), width: innerWidth } : null;
  });
  check('the tab strip ends with the window\'s own buttons', ends && ends.width - ends.right <= 16, JSON.stringify(ends));
  // Kept, the bar is in the layout: the shell's next row, the tab strip here, starts under it.
  const under = () => w.shell().evaluate(() => {
    const b = document.querySelector('.menu-bar');
    const next = b?.nextElementSibling?.getBoundingClientRect();
    return b ? { bar: b.getBoundingClientRect().height, moment: b.classList.contains('moment'), next: next?.top, band: next && getComputedStyle(b.nextElementSibling).paddingTop } : null;
  });
  check('the bar stays, in the layout, inside the frame\'s band, and the shell sits under it with no band of its own', await until(async () => {
    const u = await under();
    return u && !u.moment && u.bar === 34 && u.next >= 34 && u.band === '0px';
  }), JSON.stringify(await under()));
  await lib.fire('key', 'closeTab');
  await until(async () => (await regions()).settings === 0);
  for (const scheme of ['light', 'dark']) {
    await w.page.emulateMedia({ colorScheme: scheme });
    await settle(w.page);
    await w.page.screenshot({ path: shot(`menu-bar-linux-kept-${scheme}`) });
  }
  await w.page.emulateMedia({ colorScheme: 'light' });
  check('no errors on Linux', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();

  // The Mac has its own menu bar: no row in Settings, and Option alone does nothing.
  await lib.start(['-use=standard']);
  w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  await until(async () => (await regions()).shell === 'standard');
  await altTap();
  await settle(w.page);
  check('on the Mac, Option alone shows no menu bar, even with the pref on', (await w.ui.getByRole('menubar').count()) === 0);
  await lib.fire('menu', 'app.settings');
  await w.settings.locator('.nav [data-section="appearance"]').click();
  await w.settings.getByRole('radiogroup', { name: 'Theme' }).waitFor();
  check('and Settings has no Show menu bar', (await w.settings.getByRole('switch', { name: 'Show menu bar' }).count()) === 0);
  check('no errors on the Mac', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})();
