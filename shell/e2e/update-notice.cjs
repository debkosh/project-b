// A release ready to relaunch into, on the mock engine: its one-time notice in the Standard
// shell (a card over the page's corner) and the Mobile shell (a bar over the toolbar), each
// gone for good once Later or Relaunch is pressed, and Settings' About with the version, the
// ready release, Automatic updates and this release's notes, light and dark, on a desktop and
// a phone (docs/updates.md, docs/settings.md).
// Run: node shell/e2e/update-notice.cjs   (from the repo root, after `npm run build` in shell/)

const { execFileSync } = require('child_process');
const crypto = require('crypto');
const fs = require('fs');
const http = require('http');
const os = require('os');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, settle, until } = lib;

const shot = (name) => path.join(lib.out, `${name}.png`);
// The profile's prefs as Go saved them; none before the first change.
const prefs = () => {
  try {
    return JSON.parse(fs.readFileSync(path.join(lib.profile, 'prefs.json'), 'utf8'));
  } catch {
    return {};
  }
};

// A release server whose stable channel is 0.3.7, a zip of a bundle that says it is 0.3.7,
// as tools/release.sh publishes and the mock's check (the version alone) takes.
const releases = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-notice-'));
const app = path.join(releases, 'new/Project B.app');
fs.mkdirSync(path.join(app, 'Contents/MacOS'), { recursive: true });
fs.writeFileSync(path.join(app, 'Contents/Info.plist'), '<?xml version="1.0" encoding="UTF-8"?>\n<plist version="1.0"><dict>\n  <key>CFBundleExecutable</key><string>project_b</string>\n  <key>CFBundleShortVersionString</key><string>0.3.7</string>\n</dict></plist>\n');
fs.writeFileSync(path.join(app, 'Contents/MacOS/project_b'), '#!/bin/sh\n', { mode: 0o755 });
const zip = path.join(releases, 'release.zip');
execFileSync('ditto', ['-c', '-k', '--keepParent', app, zip]);
const asset = fs.readFileSync(zip);
const installed = path.join(releases, 'installed/Project B.app');
fs.mkdirSync(path.join(installed, 'Contents/MacOS'), { recursive: true });
const server = http.createServer((req, res) => {
  if (req.url === '/asset.zip') return res.end(asset);
  // 0.3.7's notes beside the manifest, which About shows as Coming in 0.3.7.
  if (req.url === '/notes-0.3.7.md') return res.end('# 0.3.7\n\n<!-- commit: 0 -->\n\n- Saved passwords come to the phone\n- The Dock shows the download\n');
  const a = { url: `http://127.0.0.1:${server.address().port}/asset.zip`, sha256: crypto.createHash('sha256').update(asset).digest('hex'), size: asset.length };
  res.end(JSON.stringify({ spec: 1, project: 'project-b', published: '2026-11-20T00:00:00Z', channels: { stable: '0.3.7' }, releases: { '0.3.7': { assets: { [`${process.platform}/${process.arch}`]: a } } } }));
});

(async () => {
  await new Promise((r) => server.listen(0, '127.0.0.1', r));
  const args = (shell) => [`-use=${shell}`, '-version=0.3.6', `-releases=http://127.0.0.1:${server.address().port}/releases.json`, `-app=${installed}`];
  const browser = await chromium.launch();

  // The Standard shell: the card shows once the release is ready, in light and dark.
  await lib.start(args('standard'));
  let w = await lib.openWindow(browser);
  let notice = w.ui.getByRole('dialog', { name: 'Update ready' });
  check('a ready release shows its notice over the page', await until(() => notice.isVisible(), 15000));
  check('it names the release, with Later and Relaunch', (await notice.textContent()).includes('Project B 0.3.7 is ready') &&
    (await notice.getByRole('button', { name: 'Later' }).isVisible()) && (await notice.getByRole('button', { name: 'Relaunch' }).isVisible()));
  for (const scheme of ['light', 'dark']) {
    await w.page.emulateMedia({ colorScheme: scheme });
    await settle(w.page);
    await w.page.screenshot({ path: shot(`update-notice-standard-${scheme}`) });
  }
  await w.page.emulateMedia({ colorScheme: 'light' });

  // Settings' About, light and dark: the version, the release ready with Relaunch,
  // Automatic updates, and this release's notes. The status line has the default browser
  // alone. Standard shows nothing else of the update: no bubble in the page's corner.
  check('Standard has no bubble in the page\'s corner', (await w.ui.locator('.update').count()) === 0);
  const settingsTab = async () => {
    await w.ui.getByRole('button', { name: 'Menu', exact: true }).click();
    await w.ui.getByRole('menuitem', { name: /^Settings…/ }).click();
    let id = 0;
    await until(async () => (id = (await lib.regions()).settings));
    return id;
  };
  const about = async (tab) => {
    const page = await browser.newPage({ viewport: { width: 1100, height: 760 } });
    await page.goto(`${lib.base}/settings/?tab=${tab}`);
    await page.getByRole('region', { name: 'Status' }).waitFor();
    await page.locator('.nav [data-section="about"]').click();
    return page;
  };
  let page = await about(await settingsTab());
  check('the status line no longer has the version', !(await page.getByRole('region', { name: 'Status' }).textContent()).includes('0.3.6'));
  const version = page.getByRole('region', { name: 'Version' });
  check('About has the version and the release ready, with Relaunch', await until(async () => (await version.textContent()).includes('Version 0.3.6') &&
    (await version.textContent()).includes('0.3.7 is ready') && (await version.getByRole('button', { name: 'Relaunch' }).isVisible())), await version.textContent());
  const notes = page.getByRole('region', { name: 'Release notes' });
  // The notes are the release's own, so their heading moves with each bump.
  const notesVersion = fs.readFileSync(path.join(__dirname, '../src/apps/settings/release-notes.md'), 'utf8').match(/^# (.+)$/m)[1].trim();
  check('and Automatic updates, and this release\'s notes under its version', (await page.getByRole('switch', { name: 'Automatic updates' }).isVisible()) &&
    (await notes.getByRole('heading').textContent()) === `What's new in ${notesVersion}` && (await notes.getByRole('listitem').count()) > 0);
  const coming = page.getByRole('region', { name: 'Coming next' });
  check('and above them, the release ready\'s own notes, as Coming in it', await until(async () => (await coming.isVisible()) &&
    (await coming.getByRole('heading').textContent()) === 'Coming in 0.3.7' &&
    JSON.stringify(await coming.getByRole('listitem').allTextContents()) === JSON.stringify(['Saved passwords come to the phone', 'The Dock shows the download'])),
    await coming.textContent().catch(() => ''));
  for (const scheme of ['light', 'dark']) {
    await page.emulateMedia({ colorScheme: scheme });
    await settle(page);
    await page.screenshot({ path: shot(`settings-about-${scheme}`) });
  }
  await page.close();

  // Later puts it away and records the version; after a restart, with the release still
  // ready, it doesn't come back.
  await notice.getByRole('button', { name: 'Later' }).click();
  check('Later puts the notice away and records the release as seen', await until(async () => !(await notice.isVisible()) && prefs().updateNoticed === '0.3.7'));
  await w.page.close();
  await lib.stop();
  await lib.start(args('standard'));
  w = await lib.openWindow(browser);
  notice = w.ui.getByRole('dialog', { name: 'Update ready' });
  page = await about(await settingsTab());
  check('after a restart About offers the release still', await until(() => page.getByRole('region', { name: 'Version' }).getByRole('button', { name: 'Relaunch' }).isVisible(), 15000));
  check('but its notice shows no more', !(await notice.isVisible()));
  await page.close();
  await w.page.close();
  await lib.stop();

  // The Mobile shell, on a fresh notice: a bar over the toolbar, with Later and Install,
  // light and dark; and a phone's Settings' About.
  fs.writeFileSync(path.join(lib.profile, 'prefs.json'), JSON.stringify({ ...prefs(), updateNoticed: '' }));
  await lib.start(args('mobile'));
  w = await lib.openWindow(browser, { viewport: { width: 412, height: 915 }, hasTouch: true });
  const bar = w.ui.getByRole('dialog', { name: 'Update ready' });
  check('on a phone, a ready release shows its bar over the toolbar', await until(() => bar.isVisible(), 15000));
  check('it names the release, with Later and Install', (await bar.textContent()).includes('Project B 0.3.7 is ready') && (await bar.getByRole('button', { name: 'Install' }).isVisible()));
  const small = await bar.getByRole('button').evaluateAll((bs) => bs.map((b) => b.getBoundingClientRect()).filter((r) => r.height < 48).length);
  check('its buttons are 48px high, a finger\'s', small === 0, String(small));
  for (const scheme of ['light', 'dark']) {
    await w.page.emulateMedia({ colorScheme: scheme });
    await settle(w.page);
    await w.page.screenshot({ path: shot(`update-notice-mobile-${scheme}`) });
  }
  await w.page.emulateMedia({ colorScheme: 'light' });
  const phoneTab = await (async () => {
    await w.ui.getByRole('button', { name: 'Menu' }).tap();
    await w.ui.getByRole('dialog', { name: 'Menu' }).getByRole('menuitem', { name: 'Settings' }).tap();
    let id = 0;
    await until(async () => (id = (await lib.regions()).settings));
    return id;
  })();
  const context = await browser.newContext({ viewport: { width: 412, height: 915 }, hasTouch: true });
  await context.addInitScript(() => Object.defineProperty(Navigator.prototype, 'maxTouchPoints', { get: () => 5, configurable: true }));
  const phonePage = await context.newPage();
  await phonePage.goto(`${lib.base}/settings/?tab=${phoneTab}`);
  await phonePage.getByRole('heading', { name: 'Settings' }).waitFor();
  check('on a phone, About\'s line says an update is ready', await until(async () => (await phonePage.locator('.section-row[data-section="about"]').textContent()).includes('0.3.6 · Update ready')));
  await phonePage.locator('.section-row[data-section="about"]').tap();
  check('on a phone, About offers Install update', await until(() => phonePage.getByRole('region', { name: 'Version' }).getByRole('button', { name: 'Install update' }).isVisible()));
  for (const scheme of ['light', 'dark']) {
    await phonePage.emulateMedia({ colorScheme: scheme });
    await settle(phonePage);
    await phonePage.screenshot({ path: shot(`settings-about-phone-${scheme}`) });
  }
  await context.close();
  await bar.getByRole('button', { name: 'Later' }).tap();
  check('on a phone, Later puts the bar away and records the release as seen', await until(async () => !(await bar.isVisible()) && prefs().updateNoticed === '0.3.7'));

  check('no page logged an error', w.errors.length === 0, w.errors.join('\n'));
  await browser.close();
  await lib.stop();
  server.close();
  console.log(lib.failed() ? `${lib.failed()} failed` : 'update notice: all passed');
  process.exit(lib.failed() ? 1 : 0);
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
