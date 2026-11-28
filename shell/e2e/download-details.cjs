// A download's details: on the desktop opened out of its row in Standard's Downloads panel,
// saying where it came from, its whole address with Copy link, and the folder it is saved
// in; on a phone in the sheet of the Downloads page's ⋮. A crash report kept by Save report
// says it is a crash report instead, with no address. Screenshots, light and dark, go to
// build/e2e/download-details-*.png.
// Run: node shell/e2e/download-details.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, fire, regions, settle, until } = lib;

// A zip, which Open hands to the OS: a page, PDF, image or text would open in a tab here.
const url = 'https://example.com/files/2026/quarterly-report.zip';

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
  const row = list.getByRole('listitem', { name: 'quarterly-report.zip' });
  const more = row.getByRole('button', { name: 'Details of quarterly-report.zip' });
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

// A sheet risen to its place: tapped while it still rises, a button can sit below the page,
// and the tap lands outside it. Playwright's own "stable" can't tell, since a rise slowed on
// a loaded Mac moves nothing between its two frames (2026-11-24, at a load of 28).
const risen = (sheet) => until(async () => (await sheet.isVisible()) && sheet.evaluate((el) => el.getAnimations({ subtree: true }).length === 0), 10000);

// A phone's Downloads page (docs/design-system.md, "On a phone"): one row each under its
// day, the name and one line, a tap to open it, everything else in its ⋮, a sheet whose head
// says where it came from; and a search.
async function phonePage(page, w) {
  const row = page.getByRole('listitem', { name: 'quarterly-report.zip' });
  check('a row says its size and its host on one line, under Today', (await row.innerText()).includes('example.com') &&
    (await page.getByRole('region', { name: 'Today' }).isVisible()), await row.innerText());
  await row.getByRole('button', { name: 'Actions for quarterly-report.zip' }).click();
  const sheet = page.getByRole('dialog', { name: 'quarterly-report.zip' });
  await risen(sheet);
  check('its ⋮ opens a sheet whose head has its whole address, without its scheme', await until(async () => (await sheet.isVisible()) &&
    (await sheet.innerText()).includes(url.replace('https://', '')) && !(await sheet.innerText()).includes('https://')), await sheet.innerText());
  const said = await sheet.getByRole('button').allInnerTexts();
  check('with Share, Show in Downloads and Copy link, then Remove from list and Delete', JSON.stringify(said) === JSON.stringify(['Share', 'Show in Downloads', 'Copy link', 'Remove from list', 'Delete']), JSON.stringify(said));
  await shots(w.page, 'page-sheet');
  await sheet.getByRole('button', { name: 'Copy link' }).click();
  check('Copy link copies the address, and the sheet goes', await until(async () => (await regions()).clipboard === url && !(await sheet.isVisible())), (await regions()).clipboard);
  await row.getByRole('button', { name: 'Actions for quarterly-report.zip' }).click();
  await risen(sheet);
  await sheet.getByRole('button', { name: 'Share' }).click();
  check('Share hands the file to the system\'s share sheet', await until(async () => ((await regions()).shared || '').endsWith('quarterly-report.zip')), (await regions()).shared);
  await row.getByRole('button', { name: /^quarterly-report\.zip/ }).click();
  check('a tap on the row opens the file', await until(async () => ((await regions()).openedFiles || []).some((f) => f.endsWith('quarterly-report.zip'))), JSON.stringify((await regions()).openedFiles));

  const report = page.getByRole('listitem').filter({ hasText: 'project-b-report-' }).first();
  await report.getByRole('button', { name: /^Actions for project-b-report-/ }).click();
  const reportSheet = page.getByRole('dialog', { name: /^project-b-report-/ });
  await risen(reportSheet);
  check('the crash report\'s sheet says it is one, with no address to copy', await until(async () => (await reportSheet.innerText()).includes('Crash report')) &&
    (await reportSheet.getByRole('button', { name: 'Copy link' }).count()) === 0, await reportSheet.innerText());
  await page.locator('.ui-sheet-backdrop').click({ position: { x: 20, y: 20 } });
  check('a tap on the page behind closes it', await until(async () => !(await reportSheet.isVisible())));

  await page.getByRole('button', { name: 'Search' }).click();
  await page.getByRole('searchbox', { name: 'Search downloads' }).fill('quarterly');
  const names = async () => (await page.getByRole('listitem').evaluateAll((items) => items.map((i) => i.getAttribute('aria-label'))));
  check('Search finds by name, and leaves the rest out', await until(async () => (await names()).length > 0 && (await names()).every((n) => n.includes('quarterly'))), JSON.stringify(await names()));
  check('with our own × to clear it', await page.getByRole('button', { name: 'Clear search' }).isVisible());
  await shots(w.page, 'page-search');
  await page.getByRole('button', { name: 'Back' }).click();
  check('and Back closes the search, the list whole again', await until(async () => (await report.isVisible()) && (await page.getByRole('heading', { name: 'Downloads' }).isVisible())), JSON.stringify(await names()));

  // Delete, asked first.
  await row.getByRole('button', { name: 'Actions for quarterly-report.zip' }).click();
  await risen(sheet);
  await sheet.getByRole('button', { name: 'Delete' }).click();
  const confirm = page.getByRole('dialog', { name: 'Delete quarterly-report.zip?' });
  check('Delete asks first', await until(() => confirm.isVisible()));
  await shots(w.page, 'page-delete');
  await confirm.getByRole('button', { name: 'Delete' }).click();
  check('and deletes it, gone from the list', await until(async () => (await row.count()) === 0 && ((await regions()).deleted || []).some((f) => f.endsWith('quarterly-report.zip'))), JSON.stringify((await regions()).deleted));
}

// Holds el for a long press, as a finger does, and says what the name shown then says.
async function hold(w, el) {
  const b = await el.boundingBox();
  await w.page.mouse.move(b.x + b.width / 2, b.y + b.height / 2);
  await w.page.mouse.down();
  await w.page.waitForTimeout(700);
  const tip = w.page.frameLocator('iframe.our-page[data-page="downloads"]').getByRole('tooltip');
  const said = (await tip.count()) ? (await tip.textContent()).trim() : '';
  await w.page.mouse.up();
  return said;
}

// A phone's rows while one runs and once it fails (docs/ux-guide.md, "Anything that takes
// time"): a running row keeps Pause and Cancel in its ⋮, Cancel's a square, and has no
// Retry; a failed one carries Retry in its row as an icon, named for TalkBack and when held,
// and a hold shows the name without trying again.
async function phoneRows(page, w) {
  await fire('download', 'https://example.com/files/a-long-name-for-a-stalled-download-2026.zip');
  await fire('downloadsStall');
  const row = page.getByRole('listitem', { name: 'a-long-name-for-a-stalled-download-2026.zip' });
  check('a running row shows', await until(() => row.isVisible()));
  check('with no Retry in it, its actions in its ⋮', (await row.getByRole('button', { name: 'Retry' }).count()) === 0 &&
    JSON.stringify(await row.getByRole('button').evaluateAll((l) => l.map((b) => b.getAttribute('aria-label') || b.textContent.trim()))) ===
    JSON.stringify(['a-long-name-for-a-stalled-download-2026.zip' + (await row.locator('.line').textContent()), 'Actions for a-long-name-for-a-stalled-download-2026.zip']),
    JSON.stringify(await row.getByRole('button').evaluateAll((l) => l.map((b) => b.getAttribute('aria-label') || b.textContent.trim()))));
  check('and no word Downloading', !(await row.innerText()).includes('Downloading'), await row.innerText());
  await shots(w.page, 'page-running');
  await row.getByRole('button', { name: 'Actions for a-long-name-for-a-stalled-download-2026.zip' }).click();
  const sheet = page.getByRole('dialog', { name: 'a-long-name-for-a-stalled-download-2026.zip' });
  await risen(sheet);
  const tiles = await sheet.getByRole('button').allInnerTexts();
  check('its sheet has Pause, Cancel and Copy link only', JSON.stringify(tiles) === JSON.stringify(['Pause', 'Cancel', 'Copy link']), JSON.stringify(tiles));
  check("and Cancel's icon is a square, not a cross", (await sheet.getByRole('button', { name: 'Cancel' }).locator('path').getAttribute('d')) === 'M7 7h10v10H7z');
  await page.locator('.ui-sheet-backdrop').click({ position: { x: 20, y: 20 } });
  await until(async () => !(await sheet.isVisible()));
  await fire('downloadsFail');
  const retry = row.getByRole('button', { name: 'Retry' });
  check('a failed row carries Retry, an icon named Retry', await until(() => retry.isVisible()) && (await retry.textContent()).trim() === '' &&
    (await retry.locator('path').getAttribute('d')) === 'M19 12a7 7 0 1 1-2.05-4.95M19 4.5V8h-3.5');
  const r = await retry.boundingBox();
  check('a 48px target', Math.round(r.width) === 48 && Math.round(r.height) === 48, JSON.stringify(r));
  await shots(w.page, 'page-failed');
  check('held, it shows its name', (await hold(w, retry)) === 'Retry');
  await shots(w.page, 'page-hold');
  check('and the hold does not try again', (await row.innerText()).includes('Failed'), await row.innerText());
  await until(async () => (await page.getByRole('tooltip').count()) === 0, 4000);
  await retry.click();
  check('a tap on Retry tries again', await until(async () => !(await row.innerText()).includes('Failed')), await row.innerText());
}

(async () => {
  const browser = await chromium.launch();

  // The desktop: Standard's panel.
  fs.writeFileSync(path.join(lib.profile, 'crash.log'), 'pid 1, started 2026-11-14 09:00:00\npanic: the last run\n');
  await lib.start(['-use=pbshell']);
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
  // A streamed video's playlist is saved as one MP4 in its row (docs/video-streams.md). The
  // mock engine's file is no playlist, so its row, under the MP4's name, says why not.
  await fire('download', 'https://example.com/watch/film.m3u8');
  const film = panel.getByRole('listitem', { name: 'film.mp4' });
  check("a playlist's row goes by its MP4's name, and one that can't be saved says why, as a failed download does",
    await until(async () => (await film.innerText().catch(() => '')).includes("Failed: this video's format can't be saved yet")), await film.innerText().catch(() => 'no row'));
  check("with no Retry, as trying again can't help", (await film.getByRole('button', { name: /^Retry/ }).count()) === 0);
  // A download whose name says nothing is named after its page, and its details keep the
  // name its server gave it (docs/daily-driver.md, "Naming downloads").
  const tab = (await regions()).active;
  await fire('navigate', 'https://films.example/watch', { tab });
  await until(async () => {
    const t = (await regions()).tabs.find((x) => x.id === tab);
    return t?.url === 'https://films.example/watch' && !t.loading && t.title === 'Mock films.example';
  });
  await fire('download', 'https://cdn.example/files/9f86d081884c7d65.zip', { tab });
  const named = panel.getByRole('listitem', { name: 'Mock films.example.zip' });
  check('a download with a random name is named after its page, its ending its own', await until(() => named.isVisible()));
  await named.getByRole('button', { name: 'Details of Mock films.example.zip' }).click();
  check('and its details say the name it came by', await until(async () => (await named.locator('dl').innerText()).replace(/\s+/g, ' ').includes('Named from 9f86d081884c7d65.zip')), await named.locator('dl').innerText().catch(() => ''));
  await shots(w.page, 'panel', async () => {
    const b = await panel.boundingBox();
    return { x: b.x - 16, y: b.y - 56, width: b.width + 32, height: b.height + 72 };
  });
  check('no errors in the page', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();

  // A phone: the Downloads page, from the menu.
  fs.writeFileSync(path.join(lib.profile, 'crash.log'), 'pid 1, started 2026-11-14 09:00:00\npanic: the last run\n');
  await lib.start(['-use=mwshell']);
  w = await lib.openWindow(browser, { viewport: { width: 412, height: 915 }, hasTouch: true });
  await fire('download', url);
  await w.ui.getByRole('button', { name: 'Save', exact: true }).click();
  await until(async () => (await regions()).published);
  await w.ui.getByRole('button', { name: 'Menu' }).click();
  await w.ui.getByRole('menuitem', { name: 'Downloads' }).click();
  const page = w.page.frameLocator('iframe.our-page[data-page="downloads"]');
  await until(() => page.getByRole('listitem', { name: 'quarterly-report.zip' }).isVisible());
  await shots(w.page, 'page');
  await phonePage(page, w);
  await phoneRows(page, w);
  check('no errors in the page', w.errors.length === 0, w.errors.join(' | '));

  await browser.close();
  await lib.done();
})();
