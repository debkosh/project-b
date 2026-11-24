// Dragging a tab in the standard shell, on the mock engine: onto another tab's row it lands
// before or after it, by the half it is let go over, in the sidebar and in the strip on
// top; among a group's tabs it joins the group; the active tab stays.
// Run: node shell/e2e/shells/standard-drag.cjs   (from the repo root, after `npm run build`
// in shell/)

const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, until } = lib;

const send = (m) => fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify(m) });

(async () => {
  await lib.start(['-use=pbshell']);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser, { viewport: { width: 1000, height: 600 } });
  const { page, ui } = w;
  page.setDefaultTimeout(5000);
  await until(async () => (await ui.locator('.sidebar [role=tab]').count()) === 1, 10000);
  await send({ t: 'navigate', text: 'https://a.example.com/' });
  for (const s of ['b', 'c', 'd']) await send({ t: 'newTab', text: `https://${s}.example.com/` });
  await until(async () => (await ui.locator('.sidebar [role=tab]').count()) === 4);
  const order = (list) => w.shell().evaluate((q) => [...document.querySelectorAll(q)].map((e) => +e.dataset.tab), `${list} [role=tab]`);
  const [a, b, c, d] = await order('.sidebar');
  const row = (list, id) => ui.locator(`${list} [role=tab][data-tab="${id}"]`);
  // dragTo lets go at a point in the target: its top or left quarter lands before it, its
  // bottom or right quarter after.
  const dragTo = async (list, id, onto, after) => {
    const box = await row(list, onto).boundingBox();
    const side = list === '.sidebar';
    const targetPosition = side ? { x: box.width / 2, y: box.height * (after ? 0.8 : 0.2) } : { x: box.width * (after ? 0.8 : 0.2), y: box.height / 2 };
    await row(list, id).dragTo(row(list, onto), { targetPosition });
  };
  const is = async (list, want) => until(async () => JSON.stringify(await order(list)) === JSON.stringify(want));

  await dragTo('.sidebar', d, a, false);
  check('in the sidebar, a tab let go on a row\'s top half lands before it', await is('.sidebar', [d, a, b, c]), JSON.stringify(await order('.sidebar')));
  await dragTo('.sidebar', d, b, true);
  check('and on its bottom half, after it', await is('.sidebar', [a, b, d, c]), JSON.stringify(await order('.sidebar')));
  check('the active tab stays as it was, the last one opened', (await ui.locator('.sidebar [role=tab][aria-selected=true]').getAttribute('data-tab')) === String(d));
  check('no row keeps its landing mark', (await ui.locator('.sidebar .drop-before, .sidebar .drop-after').count()) === 0);

  // Among a group's tabs, it joins the group.
  await send({ t: 'createGroup', ids: [a, b] });
  await until(async () => (await ui.locator('.sidebar [role=tab].grouped').count()) === 2);
  await dragTo('.sidebar', c, b, false);
  check('let go between two tabs of a group, it joins the group', await until(async () => (await row('.sidebar', c).evaluate((e) => e.classList.contains('grouped')))) && (await is('.sidebar', [a, c, b, d])));
  await dragTo('.sidebar', c, d, true);
  check('and dragged out past the group, it leaves it', await until(async () => !(await row('.sidebar', c).evaluate((e) => e.classList.contains('grouped')))) && (await is('.sidebar', [a, b, d, c])));

  // The same in the strip on top.
  await fire('menu', 'view.tabsTop');
  await until(async () => (await ui.locator('.strip [role=tab]').count()) === 4);
  await dragTo('.strip', c, d, false);
  check('on top, a tab let go on a tab\'s left half lands before it', await is('.strip', [a, b, c, d]), JSON.stringify(await order('.strip')));
  await dragTo('.strip', d, c, false);
  check('and the other way round', await is('.strip', [a, b, d, c]), JSON.stringify(await order('.strip')));

  check('no errors in the pages', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})();
