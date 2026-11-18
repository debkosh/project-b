// The standard shell's bookmark places, on the mock engine (docs/bookmark-folders.md, "Where
// bookmarks show"), each shown only while its pref is on: the new tab's tiles, under the
// command box, with a folder that opens in place and opens all its bookmarks as a group; the
// tree under the sidebar's tabs, whose bookmarks show their tabs or open new ones; the
// command box narrowed to bookmarks by a *, and its keys. Shot for the eye.
// Run: node shell/e2e/shells/standard-bookmarks.cjs   (from the repo root, after `npm run
// build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, settle, until } = lib;

const send = (m) => fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify(m) });
const setPref = (name, on) => send({ t: 'setPref', name, value: String(on) });
const prefsFile = () => {
  try {
    return JSON.parse(fs.readFileSync(path.join(lib.profile, 'prefs.json'), 'utf8'));
  } catch {
    return {};
  }
};

// The profile's bookmarks, as bookmarks.json keeps them: three folders, one with a folder in
// it, and two bookmarks at the top.
const id = (n) => n.toString(16).padStart(16, '0');
const folder = (n, parent, title, place) => ({ id: id(n), parent, kind: 'folder', title, added: 1, modified: 1, place });
const mark = (n, parent, title, url, place) => ({ id: id(n), parent, kind: 'bookmark', title, url, added: 1, modified: 1, place });
const reading = id(1);
const recipes = id(2);
const travel = id(3);
const weeknight = id(4);
const entries = [
  folder(1, '', 'Reading', 'a'),
  folder(2, '', 'Recipes', 'b'),
  folder(3, '', 'Travel', 'c'),
  mark(10, '', 'Bank', 'https://bank.example.com/', 'd'),
  mark(11, '', 'Shop', 'https://shop.example.net/', 'e'),
  mark(20, reading, 'News', 'https://news.example.org/', 'a'),
  mark(21, reading, 'Essays', 'https://essays.example.org/', 'b'),
  mark(30, recipes, 'Pasta with tomatoes', 'https://recipes.example.com/pasta', 'b'),
  mark(31, recipes, 'Lentil soup', 'https://soup.example.com/lentil', 'c'),
  mark(32, recipes, 'Curry', 'https://recipes.example.com/curry', 'd'),
  folder(4, recipes, 'Weeknight', 'a'),
  mark(33, weeknight, 'Flatbread', 'https://bread.example.com/', 'a'),
  mark(40, travel, 'Recommended trains', 'https://trains.example.com/', 'a'),
];
fs.writeFileSync(path.join(lib.profile, 'bookmarks.json'), JSON.stringify({ version: 2, last: '', entries }));

(async () => {
  await lib.start(['-use=standard']);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser, { viewport: { width: 1100, height: 760 } });
  const { page, ui } = w;
  page.setDefaultTimeout(5000);
  await until(() => ui.locator('.command-box').isVisible(), 10000);
  const goTabs = async () => (await lib.regions()).tabs;
  const activeURL = async () => {
    const r = await lib.regions();
    return r.tabs.find((t) => t.id === r.active)?.url || '';
  };

  // 1. The new tab: the top folder as tiles, folders first.
  const tiles = ui.locator('.new-tab-bookmarks > .tiles > .tile');
  const names = (l) => l.locator('.name').allInnerTexts();
  const had = (await goTabs()).length;
  check('the new tab shows the top folder as tiles, folders first', await until(async () => JSON.stringify(await names(tiles)) === JSON.stringify(['Reading', 'Recipes', 'Travel', 'Bank', 'Shop'])), JSON.stringify(await names(tiles)));
  check('a folder\'s tile shows its sites, four at most', (await tiles.nth(1).locator('.site-mark').count()) === 4 && (await tiles.nth(0).locator('.site-mark').count()) === 2);
  await tiles.nth(1).click();
  const panel = ui.getByRole('group', { name: 'Folder Recipes' });
  check('a folder opens in place, its folders first, then its bookmarks', await until(async () => JSON.stringify(await names(panel)) === JSON.stringify(['Weeknight', 'Pasta with tomatoes', 'Lentil soup', 'Curry'])), JSON.stringify(await names(panel)));
  check('under the row of its tile, across the grid', await ui.locator('.new-tab-bookmarks > .tiles > .panel').evaluate((p) => p.previousElementSibling.textContent.includes('Shop')));
  await settle(page);
  await page.screenshot({ path: path.join(lib.out, 'standard-bookmarks-newtab.png') });
  await panel.locator('.tile', { hasText: 'Weeknight' }).click();
  check('a folder in it opens in the panel, with a path back', await until(async () => (await names(ui.getByRole('group', { name: 'Folder Weeknight' }))).join() === 'Flatbread'));
  await ui.getByRole('navigation', { name: 'Path' }).getByRole('button', { name: 'Recipes' }).click();
  check('and the path goes back up', await until(() => panel.isVisible()));

  // Open all as a group: a tab for each of its bookmarks, in a group named for the folder.
  await panel.getByRole('button', { name: 'Open all as a group' }).click();
  check('Open all as a group opens a tab for each bookmark, the first in front', await until(async () => (await goTabs()).length === had + 3 && (await activeURL()) === 'https://recipes.example.com/pasta'), JSON.stringify(await goTabs()));
  check('in a group named for the folder', await until(async () => (await ui.locator('.sidebar .group').innerText().catch(() => '')).includes('Recipes') && (await ui.locator('.sidebar [role=tab].grouped').count()) === 3));

  // A bookmark tile loads in a tab with nothing in it.
  await send({ t: 'newTab' });
  await until(async () => (await goTabs()).length === had + 4 && (await tiles.count()) === 5);
  await tiles.filter({ hasText: 'Bank' }).click();
  check('a bookmark\'s tile loads it in the tab with nothing in it', await until(async () => (await activeURL()) === 'https://bank.example.com/' && (await goTabs()).length === had + 4), await activeURL());

  // Over a page, New Tab shows them too, and a tile opens a tab of its own.
  await ui.locator('.sidebar .new-tab').click();
  await until(async () => (await tiles.count()) === 5);
  await tiles.filter({ hasText: 'Shop' }).click();
  check('over a page, a tile opens in a new tab', await until(async () => (await activeURL()) === 'https://shop.example.net/' && (await goTabs()).length === had + 5));

  await setPref('bookmarksNewTab', false);
  await ui.locator('.sidebar .new-tab').click();
  await until(() => ui.locator('.command-box').isVisible());
  await settle(page);
  check('with its pref off the new tab has no bookmarks', (await ui.locator('.new-tab-bookmarks').count()) === 0);
  await page.keyboard.press('Escape');
  await setPref('bookmarksNewTab', true);

  // 2. The sidebar: a tree under the tabs, its folders folded.
  const tree = ui.locator('.sidebar .bookmarks');
  const treeRows = tree.locator('[role=treeitem] .title');
  check('the sidebar has no bookmarks by default', (await tree.count()) === 0);
  await setPref('bookmarksSidebar', true);
  check('with its pref on, the top folder is a tree under the tabs, its folders folded', await until(async () => JSON.stringify(await treeRows.allInnerTexts()) === JSON.stringify(['Reading', 'Recipes', 'Travel', 'Bank', 'Shop'])), JSON.stringify(await treeRows.allInnerTexts()));
  const row = (name) => tree.getByRole('treeitem', { name, exact: false });
  check('a bookmark open in a tab has a dot', (await row('Bank').locator('.open').count()) === 1);
  await row('Recipes').click();
  check('a press on a folder unfolds it', await until(async () => (await treeRows.allInnerTexts()).join() === 'Reading,Recipes,Weeknight,Pasta with tomatoes,Lentil soup,Curry,Travel,Bank,Shop'), (await treeRows.allInnerTexts()).join());
  const before = (await goTabs()).length;
  await row('Pasta with tomatoes').click();
  check('a bookmark open in a tab shows that tab', await until(async () => (await activeURL()) === 'https://recipes.example.com/pasta' && (await goTabs()).length === before));
  check('and its row is the active one', await until(async () => (await row('Pasta with tomatoes').getAttribute('class')).includes('active')));
  await row('Reading').click();
  await row('News').click();
  check('one not open opens in a new tab', await until(async () => (await activeURL()) === 'https://news.example.org/' && (await goTabs()).length === before + 1));
  check('which gets its dot', await until(async () => (await row('News').locator('.open').count()) === 1));
  await settle(page);
  await page.screenshot({ path: path.join(lib.out, 'standard-bookmarks-sidebar.png') });
  await row('Reading').click();
  check('a second press folds it again', await until(async () => (await row('News').count()) === 0));
  await tree.getByRole('button', { name: 'Bookmarks' }).click();
  check('the heading folds the whole tree', await until(async () => (await treeRows.count()) === 0));
  await tree.getByRole('button', { name: 'Bookmarks' }).click();
  await fire('menu', 'view.tabsTop');
  check('with the tabs on top there is no sidebar, nor its bookmarks', await until(async () => (await ui.locator('.strip').count()) === 1 && (await tree.count()) === 0));
  await fire('menu', 'view.tabsSide');
  check('back on the side, the tree is as it was', await until(async () => (await treeRows.allInnerTexts()).includes('Weeknight')));
  await setPref('bookmarksSidebar', false);
  check('with its pref off the sidebar has none', await until(async () => (await tree.count()) === 0));

  // 3. The command box: * narrows it to bookmarks.
  const box = ui.locator('.command-box input');
  const chip = ui.locator('.command-box .chip');
  const finder = ui.locator('.finder');
  const finderFolders = finder.getByRole('listbox', { name: 'Folders' }).getByRole('option');
  const results = finder.getByRole('listbox', { name: 'Bookmarks' }).getByRole('option');
  const resultTitles = () => results.locator('.title').allInnerTexts();
  const openBox = async () => {
    await ui.locator('.sidebar .new-tab').click();
    await until(async () => (await box.evaluate((el) => el === document.activeElement).catch(() => false)));
  };
  await openBox();
  await page.keyboard.type('*');
  check('a * typed first narrows the box to bookmarks, shown as a chip', await until(async () => (await chip.isVisible()) && (await box.inputValue()) === '' && (await finder.isVisible())));
  check('the folders on the left, All bookmarks first and chosen', JSON.stringify(await finderFolders.locator('.title').allInnerTexts()) === JSON.stringify(['All bookmarks', 'Reading', 'Recipes', 'Weeknight', 'Travel']) && (await finderFolders.first().getAttribute('aria-selected')) === 'true', JSON.stringify(await finderFolders.locator('.title').allInnerTexts()));
  check('All bookmarks lists every bookmark', (await results.count()) === 9);
  await page.keyboard.type('rec');
  check('typing searches every folder, each saying its folder', await until(async () => JSON.stringify(await resultTitles()) === JSON.stringify(['Pasta with tomatoes', 'Curry', 'Recommended trains'])) && JSON.stringify(await results.locator('.where').allInnerTexts()) === JSON.stringify(['in Recipes', 'in Recipes', 'in Travel']), JSON.stringify(await resultTitles()));
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  check('→ chooses the next folder: its bookmarks first, the rest after a rule, saying where', await until(async () => (await finderFolders.nth(2).getAttribute('aria-selected')) === 'true' && JSON.stringify(await resultTitles()) === JSON.stringify(['Pasta with tomatoes', 'Curry', 'Recommended trains'])) &&
    JSON.stringify(await results.locator('.where').allInnerTexts()) === JSON.stringify(['in Travel']) && (await finder.locator('.rule').count()) === 1);
  await page.keyboard.press('ArrowLeft');
  check('← the one before', await until(async () => (await finderFolders.nth(1).getAttribute('aria-selected')) === 'true' && (await results.locator('.title').first().innerText()) === 'Pasta with tomatoes'));
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowDown');
  check('↓ chooses the next bookmark', await until(async () => (await results.nth(1).getAttribute('aria-selected')) === 'true'));
  await settle(page);
  await page.screenshot({ path: path.join(lib.out, 'standard-bookmarks-commandbox.png') });
  let had3 = (await goTabs()).length;
  await page.keyboard.press('Enter');
  check('Enter opens it, over a page in a new tab', await until(async () => (await activeURL()) === 'https://recipes.example.com/curry' && (await goTabs()).length === had3 + 1 && !(await ui.locator('.command-box').isVisible())));

  await openBox();
  await page.keyboard.type('*');
  await page.keyboard.press('ArrowRight');
  had3 = (await goTabs()).length;
  const groups = (await ui.locator('.sidebar .group').count());
  await page.keyboard.press('Shift+Enter');
  check('Shift+Enter opens the folder\'s bookmarks as a group', await until(async () => (await goTabs()).length === had3 + 2 && (await ui.locator('.sidebar .group').count()) === groups + 1 &&
    (await ui.locator('.sidebar .group').allInnerTexts()).some((t) => t.includes('Reading')) && (await activeURL()) === 'https://news.example.org/'));

  await openBox();
  await page.keyboard.type('*');
  await until(() => chip.isVisible());
  await page.keyboard.press('Backspace');
  check('Backspace with nothing typed leaves bookmarks', await until(async () => !(await chip.isVisible()) && !(await finder.isVisible()) && (await box.getAttribute('placeholder')).startsWith('Search tabs')));
  await page.keyboard.type('*news');
  await until(async () => (await resultTitles()).join() === 'News');
  await page.keyboard.press('Escape');
  check('Escape leaves them too, and what was typed, the box still open', await until(async () => !(await chip.isVisible()) && (await box.inputValue()) === '' && (await ui.locator('.command-box').isVisible())));
  await setPref('bookmarksCommandBox', false);
  await until(() => prefsFile().bookmarksCommandBox === false);
  await settle(page);
  await page.keyboard.type('*');
  await settle(page);
  check('with its pref off, a * is only typed', (await box.inputValue()) === '*' && !(await chip.isVisible()));
  await page.keyboard.press('Escape');
  await page.keyboard.press('Escape');
  await setPref('bookmarksCommandBox', true);

  check('no errors in the pages', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})();
