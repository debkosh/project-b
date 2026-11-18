// Bringing things over from another browser on the mock engine (docs/onboarding.md): the
// import wizard, in the welcome and in Settings, from a copy of browserdata's test home: a
// Chrome of two profiles, an Opera and a Zen, forks, a Firefox and a Safari, whose folder
// has no permissions, as macOS's Full Disk Access refuses it; and a Thorium elsewhere, a
// fork no one named, whose folder is chosen. The welcome lists the browsers, each with its
// logo; Chrome shows its profiles and what to bring; Import ticks each kind as it lands,
// then says what came. Safari asks for the access, and once it is given shows its own.
// Settings' Import is the same wizard: a folder chosen is a browser, one of no browser
// says so, and with the Extensions flag on a profile's extensions install from a store
// of this run's own, those no store can have left out with why.
// Run: node shell/e2e/imports.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, settle, until } = lib;

const flags = (value) => fs.writeFileSync(path.join(lib.profile, 'flags.json'), JSON.stringify(value));
const read = (name) => {
  try {
    return JSON.parse(fs.readFileSync(path.join(lib.profile, name), 'utf8'));
  } catch {
    return null;
  }
};
const folders = () => (read('bookmarks.json')?.entries || []).filter((e) => e.kind === 'folder' && e.parent === '').map((e) => e.title);

// A store of this run's own for the mock engine (-store): the test extension packed as the
// mock reads a package, a CRX3 header with its id and its folder zipped, unsigned, as only
// the real engine checks a signature. So the test needs nothing fetched.
const packStore = () => {
  const store = path.join(lib.profile, '..', 'store');
  require('child_process').execFileSync('python3', ['-I', '-c', `
import io, os, struct, sys, zipfile
src, out, ext = sys.argv[1:4]
buf = io.BytesIO()
with zipfile.ZipFile(buf, 'w') as z:
    for n in sorted(os.listdir(src)):
        z.write(os.path.join(src, n), n)
key = bytes((ord(ext[2*i]) - 97) << 4 | (ord(ext[2*i+1]) - 97) for i in range(16))
header = bytes([0x82, 0xf1, 0x04, 18, 0x0a, 16]) + key
os.makedirs(os.path.join(out, 'crx'), exist_ok=True)
open(os.path.join(out, 'crx', ext + '_1.0.crx'), 'wb').write(b'Cr24' + struct.pack('<II', 3, len(header)) + header + buf.getvalue())
`, path.join(lib.root, 'tools/test-extension'), store, 'oepjjojhhaclfhmopocgmdfbeiffpheh']);
  return store;
};

// The test home, copied, with Safari locked until unlock.
const home = path.join(lib.profile, '..', 'browsers');
fs.cpSync(lib.browsers, home, { recursive: true });
const safari = path.join(home, 'Library/Safari');
const lock = () => fs.chmodSync(safari, 0);
const unlock = () => fs.existsSync(safari) && fs.chmodSync(safari, 0o755);
process.on('exit', unlock);

// What the importing list says now.
const said = (frame, from) => frame.getByRole('list', { name: `From ${from}` }).innerText().catch(() => '');

(async () => {
  const browser = await chromium.launch();

  // The welcome.
  flags({ showWelcome: 'on' });
  lock();
  await lib.start(['-use=standard', `-browsers=${home}`]);
  let w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  const welcome = w.page.frameLocator('iframe.our-page[data-page="welcome"]');
  await welcome.getByRole('button', { name: 'Get started' }).click();
  check('Get started goes to Bring your things over', await until(() => welcome.getByRole('heading', { name: 'Bring your things over' }).isVisible()));
  const rows = welcome.getByRole('button', { name: /^From / });
  const names = () => rows.evaluateAll((b) => b.map((x) => x.getAttribute('aria-label')).join(', '));
  check('a row for each browser found, forks among them, and one for another', await until(async () =>
    (await names()) === 'From Chrome, From Opera, From Firefox, From Zen, From Safari, From another browser'), await names());
  check('each says what it has', (await welcome.getByText('2 profiles').isVisible()) && (await welcome.getByText('Needs Full Disk Access, once').isVisible()));
  // Every browser's logo ships with us: the test home has none of the apps.
  check('each browser shows its logo', await until(async () => (await welcome.locator('img.ui-browser-icon').evaluateAll((imgs) => imgs.filter((i) => i.complete && i.naturalWidth > 0).length)) === 5));

  await welcome.getByRole('button', { name: 'From Chrome' }).click();
  check('a browser shows its profiles, the first picked', await until(() => welcome.getByRole('heading', { name: 'Chrome', exact: true }).isVisible()) &&
    (await welcome.getByRole('radio', { name: /Personal/ }).isChecked()) && (await welcome.getByText('4 bookmarks · 3 pages').isVisible()));
  const boxes = welcome.getByRole('group', { name: 'What to bring over from Chrome · Personal' }).getByRole('checkbox');
  check('and what to bring, all ticked', (await boxes.count()) === 2 && (await boxes.evaluateAll((b) => b.every((x) => x.checked))));
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-bring.png') });
  await welcome.getByRole('button', { name: 'Import', exact: true }).click();
  check('Import ticks each kind as it lands, then says what came', await until(async () => {
    const t = await said(welcome, 'Chrome · Personal');
    return t.includes('4 bookmarks, in 5 folders') && t.includes('under From Chrome · Personal') && t.includes('3 pages of history') && (await welcome.getByRole('heading', { name: 'Imported from Chrome · Personal' }).isVisible());
  }), await said(welcome, 'Chrome · Personal'));
  check('the bookmarks are in From Chrome · Personal', folders().includes('From Chrome · Personal'), JSON.stringify(folders()));
  check('nothing about the import is kept', read('imports.json') === null);
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-brought.png') });

  await welcome.getByRole('button', { name: 'Import another' }).click();
  await welcome.getByRole('button', { name: 'From Safari' }).click();
  check('Safari shows how to give the access', await until(() => welcome.getByRole('heading', { name: "Allow Safari's things" }).isVisible()));
  check('and opens System Settings at Full Disk Access', await until(() => lib.log.includes('imports: opening System Settings at Full Disk Access')));
  check('it waits for the access', await welcome.getByRole('status').getByText('Waiting for access…').isVisible());
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-allow.png') });
  unlock();
  check('given the access, Safari shows what it holds', await until(() => welcome.getByRole('heading', { name: 'Safari', exact: true }).isVisible(), 10000) && (await welcome.getByText('4 bookmarks · 2 pages').isVisible()),
    lib.log.split('\n').filter((l) => l.includes('imports:')).slice(-4).join(' | '));
  await welcome.getByRole('button', { name: 'Back' }).first().click();
  await welcome.getByRole('button', { name: 'Continue' }).click();
  check('Continue goes on to the style', await until(() => welcome.getByRole('heading', { name: 'Pick your style' }).isVisible()));
  await welcome.getByRole('button', { name: 'Skip setup' }).click();
  check('the welcome logged no errors', w.errors.length === 0, w.errors.join(' | '));

  // Settings' Import: the same wizard. A folder chosen is a browser; one of no browser
  // says so.
  await lib.fire('menu', 'app.settings');
  await w.settings.locator('[data-section="data"]').click();
  const imp = w.settings.getByRole('region', { name: 'Import' });
  check('Import lists the browsers', await until(() => imp.getByRole('button', { name: 'From Chrome' }).isVisible()));
  await lib.fire('choose', path.join(home, 'Library'));
  await imp.getByRole('button', { name: 'From another browser' }).click();
  check('a folder of no browser says so', await until(() => imp.getByRole('alert').getByText(/holds no Chrome-based or Firefox-based browser/).isVisible()));
  await lib.fire('choose', path.join(home, 'Elsewhere/Thorium/User Data'));
  await imp.getByRole('button', { name: 'From another browser' }).click();
  check('a folder chosen is the browser it holds, named after it', await until(() => imp.getByRole('heading', { name: 'Thorium', exact: true }).isVisible()));
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'settings-import.png') });
  await imp.getByRole('button', { name: 'Import', exact: true }).click();
  check('and its things come over', await until(async () => (await said(imp, 'Thorium · Me')).includes('1 bookmark')), await said(imp, 'Thorium · Me'));
  await imp.getByRole('button', { name: 'Import another' }).click();
  await imp.getByRole('button', { name: 'From Firefox' }).click();
  await imp.getByRole('checkbox', { name: /History/ }).uncheck();
  await imp.getByRole('button', { name: 'Import', exact: true }).click();
  check('Firefox\'s bookmarks alone come over', await until(async () => {
    const t = await said(imp, 'Firefox · default-release');
    return t.includes('2 bookmarks') && !t.includes('history');
  }), await said(imp, 'Firefox · default-release'));
  check('each into a folder of its own, after the others', JSON.stringify(folders()) === JSON.stringify(['From Chrome · Personal', 'From Thorium · Me', 'From Firefox · default-release']), JSON.stringify(folders()));
  check('Settings logged no errors', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();

  // The Extensions flag on: a profile's extensions can come too, unticked, installed by id
  // with no prompt from a store of this run's own that has the test extension and not
  // uBlock Origin Lite, which fails; those no store can have are left out, each with why.
  flags({ extensions: 'on' });
  await lib.start(['-use=standard', '-skip-welcome', `-browsers=${home}`, `-store=${packStore()}`]);
  w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  await lib.fire('menu', 'app.settings');
  await w.settings.locator('[data-section="data"]').click();
  const ex = w.settings.getByRole('region', { name: 'Import' });
  await ex.getByRole('button', { name: 'From Chrome' }).click();
  const ext = ex.getByRole('checkbox', { name: /Extensions/ });
  check('a profile offers its 2 extensions, unticked', await until(() => ext.isVisible()) && (await ex.getByText('4 bookmarks · 3 pages · 2 extensions').isVisible()) && !(await ext.isChecked()));
  for (const name of [/Bookmarks/, /History/]) await ex.getByRole('checkbox', { name }).uncheck();
  await ext.check();
  await ex.getByRole('button', { name: 'Import', exact: true }).click();
  check('the store\'s installs, one the store hasn\'t fails, and those left out say why', await until(async () => {
    const t = await said(ex, 'Chrome · Personal');
    return t.includes('1 extension installed') && t.includes('Project B test extension') && t.includes("uBlock Origin Lite didn't install") &&
      t.includes('Old Notes left out') && t.includes('Manifest V2') && t.includes('my-extension left out') && t.includes('Other Store Helper left out');
  }, 10000), await said(ex, 'Chrome · Personal'));
  check('no install prompt was left over the page', (await w.settings.getByRole('dialog').count()) === 0);
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'settings-extensions.png') });
  check('the extensions import logged no errors', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();

  // With a passwords store, passwords come over too: Chrome's through the keychain (the
  // mock's gives the test home's secret), Zen's with its primary password, typed. Files
  // a person exported come over in Settings too: bookmarks and passwords.
  await lib.start(['-use=standard', '-skip-welcome', `-browsers=${home}`, '-password-store', '-keychain=test-safe-storage']);
  w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  await lib.fire('menu', 'app.settings');
  await w.settings.locator('[data-section="data"]').click();
  const pw = w.settings.getByRole('region', { name: 'Import' });
  await pw.getByRole('button', { name: 'From Chrome' }).click();
  check('a profile offers its passwords, ticked', await until(() => pw.getByRole('checkbox', { name: /Passwords/ }).isChecked()) && (await pw.getByText('4 bookmarks · 3 pages · 3 passwords').isVisible()));
  for (const name of [/Bookmarks/, /History/]) await pw.getByRole('checkbox', { name }).uncheck();
  await pw.getByRole('button', { name: 'Import', exact: true }).click();
  check('Chrome\'s passwords come over', await until(async () => (await said(pw, 'Chrome · Personal')).includes('3 passwords')), await said(pw, 'Chrome · Personal'));
  await pw.getByRole('button', { name: 'Import another' }).click();
  await pw.getByRole('button', { name: 'From Zen' }).click();
  const primary = pw.getByRole('textbox', { name: "Zen's primary password" }).or(pw.getByLabel("Zen's primary password"));
  check('Zen asks for its primary password', await until(() => primary.isVisible()));
  await primary.fill('letmein');
  await pw.getByRole('checkbox', { name: /Bookmarks/ }).uncheck();
  await pw.getByRole('checkbox', { name: /History/ }).uncheck();
  await pw.getByRole('button', { name: 'Import', exact: true }).click();
  check('and with it, its password comes over', await until(async () => (await said(pw, 'Zen · Default (release)')).includes('1 password')), await said(pw, 'Zen · Default (release)'));
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'settings-passwords.png') });
  await pw.getByRole('button', { name: 'Import another' }).click();
  await pw.getByLabel('Passwords file').setInputFiles({ name: 'Passwords.csv', mimeType: 'text/csv', buffer: Buffer.from('Title,URL,Username,Password,Notes,OTPAuth\nnew,https://new.example.com/,nina,n3w,,\nagain,https://example.org/,bob,hunter2,,\n') });
  check('a passwords file comes over, and says to delete it', await until(async () => (await said(pw, 'Passwords.csv')).includes('1 password') && (await pw.getByText('Delete Passwords.csv now').isVisible())), await said(pw, 'Passwords.csv'));
  await pw.getByRole('button', { name: 'Import another' }).click();
  await pw.getByLabel('Bookmarks file').setInputFiles({ name: 'bookmarks.html', mimeType: 'text/html', buffer: Buffer.from('<!DOCTYPE NETSCAPE-Bookmark-file-1>\n<DL><p>\n<DT><H3>Reading</H3>\n<DL><p>\n<DT><A HREF="https://example.com/file">From a file</A>\n</DL><p>\n</DL><p>\n') });
  check('a bookmarks file comes over into its own folder', await until(async () => (await said(pw, 'bookmarks.html')).includes('1 bookmark')) && folders().includes('From bookmarks'), JSON.stringify(folders()));
  check('passwords and files logged no errors', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();

  // No browser in the home folder: the step still offers another browser's folder.
  flags({ showWelcome: 'on' });
  await lib.start(['-use=standard']);
  w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  const again = w.page.frameLocator('iframe.our-page[data-page="welcome"]');
  await again.getByRole('button', { name: 'Get started' }).click();
  check('with no other browser, the step offers another browser\'s folder', await until(async () => (await again.getByRole('button', { name: /^From / }).count()) === 1 && (await again.getByRole('button', { name: 'From another browser' }).isVisible())));
  check('and the steps are six', (await again.getByRole('list', { name: 'Steps' }).getByRole('listitem').count()) === 6);
  await w.page.close();
  await browser.close();
  await lib.done();
})();
