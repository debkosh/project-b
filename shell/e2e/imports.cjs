// Bringing things over from another browser on the mock engine (docs/onboarding.md), from
// a copy of browserdata's test home: a Chrome of two profiles, a Firefox of one and a
// Safari, whose folder has no permissions, as macOS's Full Disk Access refuses it. The
// welcome's Bring your things over lists them, a row turned on shows what it brings, and
// Bring over brings Chrome · Personal's bookmarks into a folder of their own, its history
// then says what came. Safari's row, turned on, shows how to give
// the access, and Not now goes back. Settings' Import lists the same, with what was
// brought; Safari's Allow access… dialog turns into its choices once its folder can be
// read, and brings its bookmarks; its Import… dialog brings Firefox's bookmarks alone. A
// welcome waiting on the access comes back with Safari's row on once it is given. A start
// whose home has no browser has no such step.
// Run: node shell/e2e/imports.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, regions, settle, until } = lib;

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

(async () => {
  const browser = await chromium.launch();

  flags({ showWelcome: 'on' });
  lock();
  await lib.start(['-use=standard', `-browsers=${home}`]);
  let w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  const welcome = w.page.frameLocator('iframe.our-page[data-page="welcome"]');
  await welcome.getByRole('button', { name: 'Get started' }).click();
  check('Get started goes to Bring your things over', await until(() => welcome.getByRole('heading', { name: 'Bring your things over' }).isVisible()));
  const rows = welcome.getByRole('switch', { name: /^Bring over from / });
  check('a row for each profile found', await until(async () => (await rows.count()) === 4));
  check('each says what it holds', await welcome.getByText('4 bookmarks · 3 pages').isVisible());
  check('Safari\'s says it needs Full Disk Access', await welcome.getByText('Needs Full Disk Access, in System Settings').isVisible());
  // Chrome's and Firefox's logos ship with us: the test home has neither app. Safari's is
  // the app's own, which every Mac has.
  check('each row shows its browser\'s logo', await until(async () => (await welcome.locator('img.ui-browser-icon').evaluateAll((imgs) => imgs.filter((i) => i.complete && i.naturalWidth > 0).length)) === 4));
  await welcome.getByRole('switch', { name: 'Bring over from Safari' }).click();
  check('Safari\'s row turned on shows how to give the access', await until(() => welcome.getByRole('heading', { name: "Allow Safari's things" }).isVisible()));
  check('and opens System Settings at Full Disk Access', await until(() => lib.log.includes('imports: opening System Settings at Full Disk Access')));
  check('it waits for the access', await welcome.getByRole('status').getByText('Waiting for access…').isVisible());
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-allow.png') });
  await welcome.getByRole('button', { name: 'Not now' }).click();
  check('Not now goes back, with Safari\'s row off', await until(() => welcome.getByRole('heading', { name: 'Bring your things over' }).isVisible()) && !(await welcome.getByRole('switch', { name: 'Bring over from Safari' }).isChecked()));
  check('with no row on, the way on is Continue', await welcome.getByRole('button', { name: 'Continue' }).isVisible());
  await welcome.getByRole('switch', { name: 'Bring over from Chrome · Personal' }).check();
  const boxes = welcome.getByRole('group', { name: 'What to bring over from Chrome · Personal' }).getByRole('checkbox');
  check('a row turned on shows what it brings, all ticked', (await boxes.count()) === 2 && (await boxes.evaluateAll((b) => b.every((x) => x.checked))));
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-bring.png') });

  await welcome.getByRole('button', { name: 'Bring over' }).click();
  check('Bring over says what came', await until(() => welcome.getByRole('heading', { name: 'Brought over' }).isVisible()));
  const lines = welcome.getByRole('list', { name: 'Brought over from Chrome · Personal' });
  check('its bookmarks in their folder, and its history', await until(async () => {
    const t = await lines.innerText().catch(() => '');
    return t.includes('4 bookmarks, in 5 folders, under From Chrome · Personal') && t.includes('3 pages of history');
  }), await lines.innerText().catch(() => ''));
  check('the bookmarks are in From Chrome · Personal', folders().includes('From Chrome · Personal'), JSON.stringify(folders()));
  check('imports.json keeps what came', read('imports.json')?.['chrome/Default']?.pages === 3);
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'welcome-brought.png') });
  await welcome.getByRole('button', { name: 'Continue' }).click();
  check('Continue goes on to the style', await until(() => welcome.getByRole('heading', { name: 'Pick your style' }).isVisible()));
  await welcome.getByRole('button', { name: 'Skip setup' }).click();
  check('it logged no errors', w.errors.length === 0, w.errors.join(' | '));

  // Settings' Import: the same profiles, with what was brought over.
  await lib.fire('menu', 'app.settings');
  await w.settings.locator('[data-section="data"]').click();
  check('Import lists the profiles, each with Import…', await until(async () => (await w.settings.getByRole('button', { name: /^Import from / }).count()) === 3));
  check('and says what Chrome · Personal brought', await w.settings.getByText('Brought over today: 4 bookmarks, 3 pages').isVisible());

  // Safari, locked: Allow access… shows how, and the choices once its folder can be read.
  await w.settings.getByRole('button', { name: 'Allow access to Safari' }).click();
  const allow = w.settings.getByRole('dialog', { name: "Allow Safari's things" });
  check('Allow access… shows how to give it', await until(() => allow.isVisible()));
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'settings-allow.png') });
  unlock();
  const fromSafari = w.settings.getByRole('dialog', { name: 'Import from Safari' });
  check('once given, the dialog asks what to bring, all ticked', await until(async () => (await fromSafari.isVisible()) && (await fromSafari.getByRole('checkbox').count()) === 2));
  await fromSafari.getByRole('checkbox', { name: /History/ }).uncheck();
  await fromSafari.getByRole('button', { name: 'Import', exact: true }).click();
  const sf = w.settings.getByRole('list', { name: 'Brought over from Safari' });
  check('Safari\'s bookmarks come over, its Reading List too', await until(async () => (await sf.innerText().catch(() => '')).includes('4 bookmarks, in 4 folders, under From Safari')), await sf.innerText().catch(() => ''));
  await w.settings.getByRole('button', { name: 'Import from Firefox · default-release' }).click();
  const dialog = w.settings.getByRole('dialog', { name: 'Import from Firefox · default-release' });
  check('Import… asks what to bring, all ticked', await until(() => dialog.isVisible()) && (await dialog.getByRole('checkbox').count()) === 2);
  await dialog.getByRole('checkbox', { name: /History/ }).uncheck();
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'settings-import.png') });
  await dialog.getByRole('button', { name: 'Import', exact: true }).click();
  const fx = w.settings.getByRole('list', { name: 'Brought over from Firefox · default-release' });
  check('it brings the bookmarks alone', await until(async () => (await fx.innerText().catch(() => '')).includes('2 bookmarks, in 4 folders, under From Firefox')), await fx.innerText().catch(() => ''));
  check('into From Firefox, after the others', JSON.stringify(folders()) === JSON.stringify(['From Chrome · Personal', 'From Safari', 'From Firefox']), JSON.stringify(folders()));
  check('and no history', read('imports.json')?.['firefox/test.default-release']?.pages === 0);
  check('Settings logged no errors', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();

  // A welcome waiting on the access: once it is given, back to the list, Safari's row on.
  lock();
  await lib.start(['-use=standard', `-browsers=${home}`]);
  w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  const waiting = w.page.frameLocator('iframe.our-page[data-page="welcome"]');
  await waiting.getByRole('button', { name: 'Get started' }).click();
  await until(() => waiting.getByRole('switch', { name: 'Bring over from Safari' }).isVisible());
  await waiting.getByRole('switch', { name: 'Bring over from Safari' }).click();
  await until(() => waiting.getByRole('heading', { name: "Allow Safari's things" }).isVisible());
  unlock();
  check('given the access, the welcome comes back to the list', await until(() => waiting.getByRole('heading', { name: 'Bring your things over' }).isVisible()));
  check('with Safari\'s row on and what it brings', (await waiting.getByRole('switch', { name: 'Bring over from Safari' }).isChecked()) && (await waiting.getByText('4 bookmarks · 2 pages').isVisible()) && (await waiting.getByRole('group', { name: 'What to bring over from Safari' }).getByRole('checkbox').count()) === 2);
  check('the waiting welcome logged no errors', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();

  // The Extensions flag on: Settings' Import offers Chrome · Personal's extensions,
  // unticked, and installs them by id, with no prompt, from a store of this run's own that
  // has the test extension and not uBlock Origin Lite, which fails; those no store can have
  // are left out, each with why.
  flags({ extensions: 'on' });
  await lib.start(['-use=standard', '-skip-welcome', `-browsers=${home}`, `-store=${packStore()}`]);
  w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  await lib.fire('menu', 'app.settings');
  await w.settings.locator('[data-section="data"]').click();
  await w.settings.getByRole('button', { name: 'Import from Chrome · Personal' }).click();
  const fromChrome = w.settings.getByRole('dialog', { name: 'Import from Chrome · Personal' });
  const extBox = fromChrome.getByRole('checkbox', { name: /Extensions/ });
  check('Import… offers its 2 extensions, unticked', await until(() => extBox.isVisible()) && (await fromChrome.getByText('Extensions').locator('..').innerText()).includes('2') && !(await extBox.isChecked()));
  for (const name of [/Bookmarks/, /History/]) await fromChrome.getByRole('checkbox', { name }).uncheck();
  await extBox.check();
  await fromChrome.getByRole('button', { name: 'Import', exact: true }).click();
  const ext = w.settings.getByRole('list', { name: 'Brought over from Chrome · Personal' });
  const extText = () => ext.innerText().catch(() => '');
  check('the store\'s installs, one the store hasn\'t fails, and those left out say why', await until(async () => {
    const t = await extText();
    return t.includes('1 extension installed') && t.includes('Project B test extension') && t.includes("uBlock Origin Lite didn't install") &&
      t.includes('Old Notes left out') && t.includes('Manifest V2') && t.includes('my-extension left out') && t.includes('Other Store Helper left out');
  }, 10000), await extText());
  check('no install prompt was left over the page', (await w.settings.getByRole('dialog').count()) === 0);
  check('imports.json keeps them', read('imports.json')?.['chrome/Default']?.extensions?.installed?.length === 1, JSON.stringify(read('imports.json')?.['chrome/Default']?.extensions));
  await settle(w.page);
  await w.page.screenshot({ path: path.join(lib.out, 'settings-extensions.png') });
  check('the extensions import logged no errors', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();
  flags({ showWelcome: 'on' });

  // No browser in the home folder: no such step.
  await lib.start(['-use=standard']);
  w = await lib.openWindow(browser, { viewport: { width: 1280, height: 800 } });
  const again = w.page.frameLocator('iframe.our-page[data-page="welcome"]');
  await again.getByRole('button', { name: 'Get started' }).click();
  check('with no other browser, Get started goes to the style', await until(() => again.getByRole('heading', { name: 'Pick your style' }).isVisible()));
  check('and the steps are five', (await again.getByRole('list', { name: 'Steps' }).getByRole('listitem').count()) === 5);
  await w.page.close();
  await browser.close();
  await lib.done();
})();
