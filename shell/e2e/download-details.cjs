// A download's details, opened out of its row: on the desktop in Standard's Downloads panel,
// on a phone on the Downloads page. Each says where the download came from, its whole
// address with Copy link, and the folder it is saved in; a crash report kept by Save report
// says it is a crash report instead, with no address. Screenshots, light and dark, go to
// build/e2e/download-details-*.png.
// Run: node shell/e2e/download-details.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, fire, regions, settle, until } = lib;

const url = 'https://example.com/files/2026/quarterly-report.pdf';

async function shots(page, what, clip) {
  for (const scheme of ['light', 'dark']) {
    await page.emulateMedia({ colorScheme: scheme });
    await settle(page);
    await page.screenshot({ path: path.join(lib.out, `download-details-${what}-${scheme}.png`), clip: clip ? await clip() : undefined });
  }
  await page.emulateMedia({ colorScheme: 'light' });
}

// The checks on one list, the panel or the page: the download's details, Copy link, and
// the report's.
async function details(list, { savedIn }) {
  const row = list.getByRole('listitem', { name: 'quarterly-report.pdf' });
  const more = row.getByRole('button', { name: 'Details of quarterly-report.pdf' });
  check('a row has Details, closed', (await more.getAttribute('aria-expanded')) === 'false');
  await more.click();
  check('which opens it out', (await more.getAttribute('aria-expanded')) === 'true');
  const opened = row.locator('dl');
  const text = await opened.innerText();
  check('to say where it came from, its whole address', text.includes('From') && text.includes(url), text);
  check(`and the folder it is saved in, ${savedIn}`, await until(async () => (await opened.innerText()).includes(savedIn)), await opened.innerText());
  await opened.getByRole('button', { name: 'Copy link' }).click();
  check('Copy link copies the address', await until(async () => (await regions()).clipboard === url), (await regions()).clipboard);
  check('and says it did', await until(() => opened.getByRole('button', { name: 'Copied' }).isVisible()));

  const report = list.getByRole('listitem').filter({ hasText: 'project-b-report-' }).first();
  await report.getByRole('button', { name: /^Details of project-b-report-/ }).click();
  const said = await report.locator('dl').innerText();
  check('the crash report says it is one, with no address to copy', said.includes('Crash report') &&
    (await report.locator('dl').getByRole('button', { name: 'Copy link' }).count()) === 0, said);
}

(async () => {
  const browser = await chromium.launch();

  // The desktop: Standard's panel.
  fs.writeFileSync(path.join(lib.profile, 'crash.log'), 'pid 1, started 2026-11-14 09:00:00\npanic: the last run\n');
  await lib.start(['-use=standard']);
  let w = await lib.openWindow(browser);
  await fire('download', url);
  await until(async () => (await regions()).tabs !== undefined);
  await w.ui.getByRole('button', { name: 'Save report' }).click();
  await until(async () => (await regions()).published);
  const saved = (await regions()).published;
  const panel = w.ui.getByRole('dialog', { name: 'Downloads' });
  check('the panel opens by itself for the report', await until(() => panel.isVisible()));
  // A press on the control makes the panel that peeked the person's.
  await w.ui.getByRole('button', { name: 'Downloads' }).click();
  await details(panel, { savedIn: path.dirname(saved) });
  await shots(w.page, 'panel', async () => {
    const b = await panel.boundingBox();
    return { x: b.x - 16, y: b.y - 56, width: b.width + 32, height: b.height + 72 };
  });
  check('no errors in the page', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();

  // A phone: the Downloads page, from the menu.
  fs.writeFileSync(path.join(lib.profile, 'crash.log'), 'pid 1, started 2026-11-14 09:00:00\npanic: the last run\n');
  await lib.start(['-use=mobile']);
  w = await lib.openWindow(browser, { viewport: { width: 412, height: 915 }, hasTouch: true });
  await fire('download', url);
  await w.ui.getByRole('button', { name: 'Save', exact: true }).click();
  await until(async () => (await regions()).published);
  await w.ui.getByRole('button', { name: 'Menu' }).click();
  await w.ui.getByRole('menuitem', { name: 'Downloads' }).click();
  const page = w.page.frameLocator('iframe.our-page[data-page="downloads"]');
  await until(() => page.getByRole('listitem', { name: 'quarterly-report.pdf' }).isVisible());
  await details(page, { savedIn: 'Downloads' });
  await shots(w.page, 'page');
  check('no errors in the page', w.errors.length === 0, w.errors.join(' | '));

  await browser.close();
  await lib.done();
})();
