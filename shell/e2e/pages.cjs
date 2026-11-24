// The History and Bookmarks pages, tabs of ours, on the mock engine (docs/daily-driver.md,
// docs/bookmark-folders.md): Show All History lists the pages visited and searches and
// forgets them; the address bar's star and Cmd+D bookmark a page into the folder last saved
// in and show the panel to change its name and folder; Bookmark Manager makes folders, moves
// bookmarks into them by a drag or Move to…, edits them, and deletes a folder with what it
// holds. It leaves light screenshots in build/e2e/pages-*.png.
// Run: node shell/e2e/pages.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, fire, regions, settle, until } = lib;

(async () => {
  await lib.start(['-use=pbshell']);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser);
  const { page, ui } = w;
  page.setDefaultTimeout(5000);
  const shoot = async (name) => {
    await settle(page);
    await page.screenshot({ path: path.join(lib.out, `pages-${name}.png`) });
  };
  const send = (msg) => fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify(msg) });
  const at = (url) => until(async () => {
    const r = await regions();
    const t = r.tabs.find((t) => t.id === r.active);
    return t?.url === url && !t.loading && t.title !== '';
  });

  // Two pages visited, the second bookmarked with the star.
  // The second goes in the tab the first opened, by its id: tab 0 can be the window's own
  // blank tab, as every window has a tab, and b.test then never reached history.
  await send({ t: 'newTab', text: 'https://a.test/' });
  check('a.test opens in a new tab', await at('https://a.test/'));
  await fire('navigate', 'https://b.test/news', { tab: (await regions()).active });
  check('then b.test in the same tab', await at('https://b.test/news'));
  const star = ui.getByRole('button', { name: 'Bookmark this page' });
  check('the address bar has a star', await until(() => star.isVisible()));
  const saved = lib.bookmarks;
  const name = ui.getByRole('textbox', { name: 'Name' });
  await star.click();
  const added = ui.getByRole('dialog', { name: 'Bookmarked' });
  check('the star bookmarks the page and shows its panel, the name in focus', await until(async () => (await added.isVisible()) && saved().length === 1 && (await name.evaluate((el) => el === document.activeElement))));
  check('with no folders yet, it shows no folder list', (await ui.getByRole('group', { name: 'Folder' }).count()) === 0);
  await shoot('star-panel');
  await page.keyboard.press('Escape');
  check('Escape closes it with the bookmark kept, the star filled', await until(async () => !(await added.isVisible()) && (await ui.getByRole('button', { name: 'Edit bookmark' }).getAttribute('aria-pressed')) === 'true'));
  await fire('menu', 'bookmarks.toggle');
  const edit = ui.getByRole('dialog', { name: 'Edit bookmark' });
  check('Cmd+D on a bookmarked page shows its panel to edit', await until(() => edit.isVisible()));
  await name.fill('The news');
  await page.keyboard.press('Enter');
  check('Enter saves the name and closes it', await until(async () => !(await edit.isVisible()) && saved()[0]?.title === 'The news'), JSON.stringify(saved()));

  // History.
  await fire('menu', 'history.show');
  const history = page.frameLocator('iframe.our-page[data-page="history"]');
  const rows = history.getByRole('listitem');
  check('Show All History opens the History tab, with both pages', await until(async () => (await rows.count()) === 2), String(await rows.count()));
  check('our tab has no star', (await ui.getByRole('button', { name: /bookmark/i }).count()) === 0);
  await shoot('history');
  await history.getByRole('searchbox', { name: 'Search history' }).fill('news');
  check('a search narrows the list', await until(async () => (await rows.count()) === 1 && (await rows.first().getAttribute('aria-label')).includes('b.test')));
  await rows.first().hover();
  await history.getByRole('button', { name: /^Remove .* from history$/ }).click();
  check('a row can be removed', await until(async () => (await rows.count()) === 0));
  await history.getByRole('searchbox', { name: 'Search history' }).fill('');
  await until(async () => (await rows.count()) === 1);
  const tabsBefore = (await regions()).tabs.length;
  await history.getByRole('link').first().click();
  // The new tab shown, by its page: counting tabs went wrong once every window had a tab of
  // its own, and Bookmarks then opened under the row's tab still coming.
  const shown = async (want) => until(async () => {
    const r = await regions();
    const t = r.tabs.find((x) => x.id === r.active);
    return r.tabs.length === tabsBefore + 1 && t && !t.page && t.url !== '' && want(t);
  });
  check('a row opens its page in a new tab', await shown(() => true));

  // Bookmarks.
  await fire('menu', 'bookmarks.show');
  await until(async () => {
    const r = await regions();
    return r.tabs.find((x) => x.id === r.active)?.page === 'bookmarks';
  });
  const bookmarks = page.frameLocator('iframe.our-page[data-page="bookmarks"]');
  // The folder's entries, not the path's steps, which are a list too.
  const marks = bookmarks.locator('.card').getByRole('listitem');
  const labels = () => marks.evaluateAll((els) => els.map((e) => e.getAttribute('aria-label')));
  const lists = async (want) => until(async () => JSON.stringify(await labels()) === JSON.stringify(want));
  check('Bookmark Manager lists the bookmark', await lists(['The news']), JSON.stringify(await labels()));
  const tree = bookmarks.getByRole('navigation', { name: 'Folders' });
  const folder = (n) => tree.getByRole('button', { name: new RegExp(`^${n} \\d+$`) });
  const newFolder = async (n) => {
    await tree.getByRole('button', { name: 'New folder' }).click();
    await bookmarks.getByRole('textbox', { name: 'New folder name' }).fill(n);
    await bookmarks.getByRole('button', { name: 'Make' }).click();
  };
  await newFolder('Reading');
  check('New folder makes one, on the left and first in the list', await until(async () => (await folder('Reading').count()) === 1) && (await lists(['Reading', 'The news'])), JSON.stringify(await labels()));
  await marks.filter({ hasText: 'The news' }).dragTo(folder('Reading'));
  check('a bookmark dragged onto a folder on the left moves into it, which counts it', await lists(['Reading']) && (await until(async () => (await folder('Reading').innerText()).includes('1'))), JSON.stringify(await labels()));
  await folder('Reading').click();
  check('a folder shows what it holds, under a path back to the top', await lists(['The news']) && (await bookmarks.getByRole('list', { name: 'Path' }).getByRole('button', { name: 'Bookmarks' }).isVisible()));
  // Back, the phone's key or the toolbar's, goes to the folder before, not out of the page
  // (docs/shells.md, "Back on our pages"); Forward comes back.
  await bookmarks.locator('body').evaluate(() => history.back());
  check('Back goes to the folder before, the top, and the page stays', await until(() => lists(['Reading'])), JSON.stringify(await labels()));
  await bookmarks.locator('body').evaluate(() => history.forward());
  check('and Forward to the folder again', await until(() => lists(['The news'])), JSON.stringify(await labels()));
  await shoot('bookmarks');
  const act = async (entry, item) => {
    await marks.filter({ hasText: entry }).first().hover();
    await bookmarks.getByRole('button', { name: `Actions for ${entry}` }).click();
    await bookmarks.getByRole('menuitem', { name: item }).click();
  };
  await act('The news', 'Move to…');
  await bookmarks.getByRole('dialog').getByText('Bookmarks', { exact: true }).click();
  await bookmarks.getByRole('button', { name: 'Move', exact: true }).click();
  check('Move to… moves it back to the top, leaving the folder empty', await lists([]) && (await bookmarks.getByText('This folder is empty.').isVisible()));
  await bookmarks.getByRole('list', { name: 'Path' }).getByRole('button', { name: 'Bookmarks' }).click();
  await act('The news', 'Edit…');
  await bookmarks.getByRole('textbox', { name: 'Name' }).fill('News');
  await bookmarks.getByRole('button', { name: 'Save' }).click();
  check('a bookmark can be renamed', await lists(['Reading', 'News']), JSON.stringify(await labels()));
  await bookmarks.getByRole('searchbox', { name: 'Search bookmarks' }).fill('b.test');
  check('a search looks in every folder', await lists(['News']));
  await bookmarks.getByRole('searchbox', { name: 'Search bookmarks' }).fill('');
  await marks.filter({ hasText: 'News' }).dragTo(marks.filter({ hasText: 'Reading' }));
  check('a bookmark dragged onto a folder in the list moves into it', await lists(['Reading']));
  await act('Reading', 'Delete');
  const ask = bookmarks.getByRole('dialog', { name: 'Delete “Reading”?' });
  check('deleting a folder that holds anything asks, saying how much', await until(async () => (await ask.isVisible()) && (await ask.innerText()).includes('1 bookmark')));
  await ask.getByRole('button', { name: 'Delete' }).click();
  check('and Delete takes it with what it holds', await lists([]) && saved().length === 0, JSON.stringify(saved()));

  // The panel's folder: the star's next bookmark goes where the last was saved.
  await newFolder('Later');
  await until(async () => (await folder('Later').count()) === 1);
  await send({ t: 'newTab', text: 'https://c.test/' });
  await at('https://c.test/');
  await star.click();
  await until(() => added.isVisible());
  await added.getByRole('radio', { name: 'Later' }).check();
  await shoot('star-panel-folders');
  await page.keyboard.press('Enter');
  const later = () => saved().find((e) => e.title === 'Later')?.id;
  check('the panel saves the folder chosen', await until(() => saved().some((e) => e.url === 'https://c.test/' && e.parent === later())), JSON.stringify(saved()));
  await send({ t: 'newTab', text: 'https://d.test/' });
  await at('https://d.test/');
  await star.click();
  check('and the next bookmark goes there, the panel showing it chosen', await until(async () => (await added.isVisible()) && (await added.getByRole('radio', { name: 'Later' }).isChecked()) && saved().some((e) => e.url === 'https://d.test/' && e.parent === later())));
  await page.keyboard.press('Escape');

  check('no errors', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})().catch(async (e) => {
  check('the checks ran to their end', false, e.message.split('\n').slice(0, 6).join(' / '));
  await lib.done();
});
