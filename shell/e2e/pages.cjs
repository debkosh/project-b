// The History and Bookmarks pages, tabs of ours, on the mock engine (docs/daily-driver.md):
// Show All History lists the pages visited and searches and forgets them, the address bar's
// star bookmarks a page, and Bookmark Manager lists, edits and removes bookmarks. It leaves
// light screenshots in build/e2e/pages-*.png.
// Run: node shell/e2e/pages.cjs   (from the repo root, after `npm run build` in shell/)

const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, fire, regions, settle, until } = lib;

(async () => {
  await lib.start(['-use=standard']);
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
  await send({ t: 'newTab', text: 'https://a.test/' });
  await at('https://a.test/');
  await fire('navigate', 'https://b.test/news');
  await at('https://b.test/news');
  const star = ui.getByRole('button', { name: 'Bookmark this page' });
  check('the address bar has a star', await until(() => star.isVisible()));
  await star.click();
  check('the star bookmarks the page and fills', await until(async () => (await ui.getByRole('button', { name: 'Remove bookmark' }).getAttribute('aria-pressed')) === 'true'));

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
  await history.getByRole('link').first().click();
  check('a row opens its page in a new tab', await until(async () => (await regions()).tabs.length === 3));

  // Bookmarks.
  await fire('menu', 'bookmarks.show');
  const bookmarks = page.frameLocator('iframe.our-page[data-page="bookmarks"]');
  const marks = bookmarks.getByRole('listitem');
  check('Bookmark Manager lists the bookmark', await until(async () => (await marks.count()) === 1));
  await marks.first().hover();
  await bookmarks.getByRole('button', { name: /^Edit / }).click();
  await bookmarks.getByRole('textbox', { name: 'Name' }).fill('The news');
  await bookmarks.getByRole('button', { name: 'Save' }).click();
  check('a bookmark can be renamed', await until(async () => (await marks.first().getAttribute('aria-label')) === 'The news'));
  await shoot('bookmarks');
  await bookmarks.getByRole('button', { name: 'Remove bookmark The news' }).click();
  check('and removed', await until(async () => (await marks.count()) === 0));

  check('no errors', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})().catch(async (e) => {
  check('the checks ran to their end', false, e.message.split('\n').slice(0, 6).join(' / '));
  await lib.done();
});
