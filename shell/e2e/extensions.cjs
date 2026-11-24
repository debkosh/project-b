// Extensions on the mock engine, in a real browser: the settings page installs from the
// test extension's package, loads an unpacked one, turns them off and on and removes them; the toolbar's
// puzzle menu pins a button, which shows a badge and opens its popup where it sits. Last,
// a build with WITHOUT=extensions has none of this in it.
// Run: node shell/e2e/extensions.cjs   (from the repo root, after `npm run build` in shell/, and tools/fetch-extensions.sh and tools/pack-test-extension.py for the store)

const { spawn, execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { chromium } = require('playwright');

const root = path.resolve(__dirname, '../..');
const out = path.join(root, 'build/e2e');
// The mock engine's address, on a port the system gives it, which it prints once it listens:
// two runs at once each have their own. A restart takes a new one.
let base = '';
const storeDir = path.join(root, '.tools/extensions');
const unpacked = path.join(root, 'tools/test-extension');
// The same extension as a package, as tools/pack-test-extension.py packs it.
const packaged = path.join(storeDir, 'crx/oepjjojhhaclfhmopocgmdfbeiffpheh_1.0.crx');

// The test extension, as tools/pack-test-extension.py packs it into the store.
const id = 'oepjjojhhaclfhmopocgmdfbeiffpheh';
const name = 'Project B test extension';

function cannotRun(why) {
  console.error(why);
  process.exit(2);
}
if (!fs.existsSync(path.join(storeDir, 'catalogue.json'))) {
  cannotRun('No mock store: run tools/fetch-extensions.sh and tools/pack-test-extension.py first.');
}
if (!fs.readFileSync(path.join(root, 'shell/dist/shell/pbshell/pbshell.js'), 'utf8').includes('pbx-')) {
  cannotRun('shell/dist was built without extensions: run `npm run build` in shell/ without WITHOUT=extensions first.');
}

fs.mkdirSync(out, { recursive: true });
const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-extensions-'));
// The Extensions flag, as the settings page saves it.
const setFlag = (on) => fs.writeFileSync(path.join(profile, 'flags.json'), JSON.stringify(on ? { extensions: 'on' } : {}));
setFlag(true);
const bin = path.join(out, 'dev-extensions');
execFileSync('go', ['build', '-tags', 'sqlite_fts5,extensions', '-o', bin, './cmd/dev'], { cwd: path.join(root, 'core'), stdio: 'inherit' });

let server;
async function startServers() {
  server = spawn(bin, ['-addr=127.0.0.1:0', `-shell=${path.join(root, 'shell/dist')}`, `-profile=${profile}`], { stdio: ['ignore', 'ignore', 'pipe'] });
  base = await new Promise((resolve, reject) => {
    let log = '';
    server.stderr.setEncoding('utf8');
    server.stderr.on('data', (text) => {
      log += text;
      const m = /mock engine on (http:\/\/\S+)/.exec(log);
      if (m) resolve(m[1]);
    });
    server.once('exit', (code) => reject(new Error(`the mock engine ended, ${code}:\n${log}`)));
  });
}
async function stopServers() {
  for (const p of [server]) {
    const gone = new Promise((r) => p.once('exit', r));
    p.kill('SIGTERM');
    await gone;
  }
}

let failed = 0;
function check(name, ok, detail = '') {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${ok || !detail ? '' : ' -> ' + detail}`);
  if (!ok) failed++;
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
async function until(fn, ms = 5000) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    if (await fn()) return true;
    await sleep(30);
  }
  return false;
}
const regions = async (q = '') => (await fetch(`${base}/mock/regions${q}`)).json();
// The window's settings tab, 0 for none, and its closing, as a shell asks for it.
const settingsTab = async () => (await regions()).settings;
const closeSettings = async () => {
  await fetch(`${base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'closeTab', id: await settingsTab() }) });
  await until(async () => (await settingsTab()) === 0);
};
const runtime = async () => (await fetch(`${base}/mock/extensions`)).json();
const fire = (kind, extension, value) => fetch(`${base}/mock/extensions/fire`, { method: 'POST', body: new URLSearchParams({ kind, extension, value }) });
const call = async (extension, fn, args) =>
  (await fetch(`${base}/mock/extensions/call?extension=${extension}&name=${fn}&tab=0`, { method: 'POST', body: JSON.stringify(args) })).json();
const rectOf = (locator) => locator.evaluate((el) => {
  const r = el.getBoundingClientRect();
  return { x: r.x, y: r.y, w: r.width, h: r.height };
});
const same = (a, b) => !!a && !!b && ['x', 'y', 'w', 'h'].every((k) => Math.abs(a[k] - b[k]) < 0.5);

(async () => {
  await startServers();
  const browser = await chromium.launch();
  const errors = [];
  const watch = (p) => {
    p.on('pageerror', (e) => errors.push(e.message));
    p.on('console', (m) => m.type() === 'error' && errors.push(m.text()));
  };
  const shell = await browser.newPage({ viewport: { width: 1100, height: 700 } });
  watch(shell);
  // The window page, which holds the shell in a frame: see first-app.cjs.
  const ui = shell.frameLocator('iframe.shell');
  // Settings is in the standard shell's ☰ menu, the menu bar's menus (docs/menus-shortcuts.md).
  const openSettings = async () => {
    await ui.getByRole('button', { name: 'Menu', exact: true }).click();
    await ui.getByRole('menuitem', { name: /^Settings…/ }).click();
  };
  await shell.goto(base + '/');
  // The window starts on a new tab: the checks want a page, so it loads one.
  await fetch(`${base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'navigate', text: 'https://example.com/' }) });
  await shell.frameLocator('iframe.mock-page').getByRole('button', { name: 'Navigate' }).waitFor();

  // The dev window shows the settings page in the settings tab's frame once Go opens it, as
  // the real window does (core/cmd/dev/window.html). The test drives a page of its own,
  // the same tab's at /settings/?tab=<id>, which talks to Go the same way, and stays
  // usable once Go has closed the tab.
  const open = async () => {
    const p = await browser.newPage({ viewport: { width: 1100, height: 800 } });
    watch(p);
    await p.goto(`${base}/settings/?tab=${await settingsTab()}`);
    await p.getByRole('heading', { name: 'Settings' }).waitFor();
    return p;
  };
  await openSettings();
  await until(async () => (await settingsTab()) !== 0);
  let page = await open();

  await page.getByRole('button', { name: 'Extensions' }).click();
  let installed = page.getByRole('region', { name: 'Installed' });
  const storeRow = page.getByRole('region', { name: 'Store' });
  check('settings shows the Extensions section', await installed.getByText('No extensions installed').isVisible() && await storeRow.getByText('Chrome Web Store').isVisible());

  // The store is a website, which opens in a tab beside the settings tab.
  await storeRow.getByRole('button', { name: 'Open' }).click();
  check('the store opens in a tab beside the settings tab, which stays', await until(async () => {
    const r = await regions();
    return r.tabs.length === 3 && r.tabs[1].id === r.settings && r.tabs[2].url === 'https://chromewebstore.google.com/' && r.active === r.tabs[2].id;
  }), JSON.stringify((await regions()).tabs));
  await openSettings();
  await until(async () => (await regions()).active === (await settingsTab()));

  // A package file, which the engine asks about before it installs.
  const pick = async () => {
    await fire('choose', '', packaged);
    await page.getByRole('button', { name: 'Install package…' }).click();
  };
  const dialog = page.getByRole('dialog');
  await pick();
  check('Install package asks first, with what it can do', await until(async () => (await dialog.count()) === 1 && (await dialog.textContent()).includes(`Add "${name}"?`)));
  const asked = await dialog.locator('li').allTextContents();
  check('the prompt lists the permissions', asked.includes('storage') && asked.includes('tabs'), asked.join(', '));
  check('the shell has the prompt too, for an install a store\'s page begins', await until(async () => (await ui.getByRole('dialog').count()) === 1));
  await page.keyboard.press('Escape');
  check('Escape answers Cancel', await until(async () => (await dialog.count()) === 0));
  await sleep(200);
  check('and leaves the settings tab open', (await settingsTab()) !== 0);
  await pick();
  await dialog.waitFor();
  await dialog.getByRole('button', { name: 'Cancel' }).click();
  check('Cancel installs nothing', await until(async () => (await dialog.count()) === 0) && await installed.getByText('No extensions installed').isVisible());

  // The shell's copy answers as the settings page's does.
  await pick();
  await ui.getByRole('dialog').getByRole('button', { name: 'Add extension' }).click();
  const rows = installed.getByRole('group', { name });
  // A row's actions are in its menu: Details, Pin and Remove….
  const act = async (row, item) => {
    await row.getByRole('button', { name: `Actions for ${name}` }).click();
    await page.getByRole('menuitem', { name: item, exact: true }).click();
  };
  check('Add extension, in the shell, installs it', await until(async () => (await rows.count()) === 1 && (await ui.getByRole('dialog').count()) === 0));
  await act(rows.first(), 'Details');
  const details = page.getByRole('definition');
  check('Details shows its ID, source and toolbar button', (await details.allTextContents()).join('|').startsWith(`${id}|1.0|Package|`) && (await page.locator('.pbx-details').textContent()).includes('Yes'), (await details.allTextContents()).join('|'));
  await act(rows.first(), 'Hide details');

  await fire('choose', '', unpacked);
  await page.getByRole('button', { name: 'Load unpacked…' }).click();
  check('Load unpacked adds a second one', await until(async () => (await rows.count()) === 2));
  await act(rows.nth(1), 'Details');
  check('which says where it was loaded from', (await page.locator('.pbx-details').textContent()).includes(`Unpacked, from ${unpacked}`), await page.locator('.pbx-details').textContent());
  await page.screenshot({ path: path.join(out, 'extensions-settings.png'), fullPage: true });
  await act(rows.nth(1), 'Remove…');
  const removeDialog = page.getByRole('dialog', { name: `Remove ${name}?` });
  check('Remove asks first', await until(async () => (await removeDialog.count()) === 1) && (await rows.count()) === 2);
  await removeDialog.getByRole('button', { name: 'Remove', exact: true }).click();
  check('and then takes the unpacked one away', await until(async () => (await rows.count()) === 1));

  const enabled = rows.first().getByRole('switch');
  await enabled.click();
  check('the switch turns it off', await until(async () => !(await enabled.isChecked())));
  const puzzle = ui.getByRole('button', { name: 'Extensions', exact: true });
  const menu = ui.getByRole('group', { name: 'Extensions menu' });
  await puzzle.click();
  check('an extension that is off is not in the menu', await until(async () => (await menu.textContent()).includes('No extensions installed')));
  await puzzle.click();
  check('the puzzle piece closes its menu', await until(async () => (await menu.count()) === 0));
  await enabled.click();
  check('and on again', await until(async () => await enabled.isChecked()));

  // The toolbar.
  check('the shell shows the puzzle piece', await puzzle.isVisible());
  await puzzle.click();
  check('its menu lists the extension', await until(async () => (await menu.locator('.pbx-row').count()) === 1 && (await menu.textContent()).includes(name)));
  const m = await rectOf(menu);
  check('the menu is shell, over the page', await until(async () => (await regions(`?x=${m.x + m.w / 2}&y=${m.y + m.h / 2}`)).hit === 'shell'));
  await shell.screenshot({ path: path.join(out, 'extensions-menu.png') });
  await menu.getByRole('button', { name: `Pin ${name}` }).click();
  const button = ui.locator('.pbx-action');
  check('pin puts its button on the toolbar', await until(async () => (await button.count()) === 1 && (await button.getAttribute('aria-label')) === name));
  check('and the menu shows it pinned', await until(async () => (await menu.getByRole('button', { name: `Unpin ${name}` }).count()) === 1));
  await shell.keyboard.press('Escape');
  check('Escape closes the menu', await until(async () => (await menu.count()) === 0));

  const set = await call(id, 'action.setBadgeText', [{ text: '7' }]);
  check('a badge the extension sets shows on its button', await until(async () => (await button.locator('.pbx-badge').count()) === 1 && (await button.locator('.pbx-badge').textContent()) === '7'), JSON.stringify(set));

  await button.click();
  const anchor = await rectOf(button);
  check('clicking the button opens its popup under it', await until(async () => {
    const p = (await runtime()).popup;
    return p && p.extension === id && same(p.anchor, anchor);
  }), JSON.stringify((await runtime()).popup) + ' vs ' + JSON.stringify(anchor));
  check('and it looks pressed while the popup is open', await until(async () => (await button.getAttribute('class')).includes('pbx-open')));
  const card = shell.locator('.mock-popup');
  check('the dev window draws the popup under the button', await until(async () => (await card.isVisible()) && (await rectOf(card)).y >= anchor.y + anchor.h), JSON.stringify(await card.count() && await rectOf(card)));
  await shell.screenshot({ path: path.join(out, 'extensions-toolbar.png') });
  // A click outside the popup closes it, as in the real window.
  await shell.mouse.click(20, 650);
  check('a click outside the popup closes it and lets the button go', await until(async () => !(await button.getAttribute('class')).includes('pbx-open') && (await runtime()).popup === null));

  // An unpinned extension's popup opens under the puzzle piece.
  await puzzle.click();
  await menu.getByRole('button', { name: `Unpin ${name}` }).click();
  check('unpin takes the button off the toolbar', await until(async () => (await button.count()) === 0));
  await menu.locator('.pbx-item', { hasText: name }).click();
  const puzzleRect = await rectOf(puzzle);
  check('a click in the menu opens the popup under the puzzle piece', await until(async () => {
    const p = (await runtime()).popup;
    return p && p.extension === id && same(p.anchor, puzzleRect);
  }), JSON.stringify((await runtime()).popup));
  check('and closes the menu', (await menu.count()) === 0);
  await fire('closePopup', '', '');
  await until(async () => (await runtime()).popup === null);
  await puzzle.click();
  // The address field's own area: a text field on a web tab, a title on a tab of ours.
  await ui.locator('form.field').click();
  check('a click elsewhere in the shell closes the menu', await until(async () => (await menu.count()) === 0));

  // Manage extensions opens the settings tab at its Extensions section, from closed and from
  // elsewhere in settings (docs/settings.md).
  await page.close();
  await closeSettings();
  await puzzle.click();
  await menu.getByRole('button', { name: 'Manage extensions' }).click();
  check('Manage extensions opens the settings tab', await until(async () => (await settingsTab()) !== 0 && (await regions()).active === (await settingsTab())));
  page = await open();
  installed = page.getByRole('region', { name: 'Installed' });
  check('at its Extensions section', await until(() => installed.isVisible()) && (await page.title()) === 'Settings › Extensions', await page.title());
  await page.locator('.nav [data-section="appearance"]').click();
  await until(async () => (await installed.count()) === 0);
  await puzzle.click();
  await menu.getByRole('button', { name: 'Manage extensions' }).click();
  check('and goes back there when it is already open', await until(() => installed.isVisible()));

  await page.getByRole('region', { name: 'Installed' }).getByRole('button', { name: `Actions for ${name}` }).click();
  await page.getByRole('menuitem', { name: 'Remove…' }).click();
  await page.getByRole('dialog', { name: `Remove ${name}?` }).getByRole('button', { name: 'Remove', exact: true }).click();
  check('Remove takes it away', await until(async () => await page.getByRole('region', { name: 'Installed' }).getByText('No extensions installed').isVisible()));
  await puzzle.click();
  check('and from the menu', await until(async () => (await menu.textContent()).includes('No extensions installed')));

  // With the flag off, the build's extensions don't start. The server's restart cuts the
  // pages' event streams, which is no error of theirs.
  const before = errors.length;
  await page.close();
  await stopServers();
  setFlag(false);
  await startServers();
  await shell.goto(base + '/');
  await fetch(`${base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'navigate', text: 'https://example.com/' }) });
  await shell.frameLocator('iframe.mock-page').getByRole('button', { name: 'Navigate' }).waitFor();
  errors.splice(before);
  check('with the flag off the shell has no extensions toolbar', (await ui.locator('.pbx-toolbar').count()) === 0);
  await openSettings();
  await until(async () => (await settingsTab()) !== 0);
  page = await open();
  // The section is Sparks alone; the flag is in Flags.
  check('and settings no Extensions section, Sparks in its place', await until(async () => (await page.locator('.nav [data-section="extensions"]').textContent()).trim() === 'Sparks'));
  await page.locator('.nav [data-section="flags"]').click();
  await until(() => page.getByRole('switch', { name: /Extensions/ }).isVisible());
  check('but the flag is there to turn on', await page.getByRole('switch', { name: /Extensions/ }).isEnabled());

  check('no errors in the pages', errors.length === 0, errors.join(' | '));
  await browser.close();
  await stopServers();
  fs.rmSync(profile, { recursive: true, force: true });

  // A build with WITHOUT=extensions has none of the extensions UI in it.
  const off = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-no-extensions-'));
  const env = { ...process.env, WITHOUT: 'extensions' };
  const shellDir = path.join(root, 'shell');
  execFileSync('node', ['build.js', '--logLevel', 'error', '--outDir', off], { cwd: shellDir, env, stdio: 'inherit' });
  const leaked = fs.readdirSync(off, { recursive: true })
    .filter((f) => fs.statSync(path.join(off, f)).isFile() && fs.readFileSync(path.join(off, f), 'utf8').includes('pbx-'));
  check('a build without extensions has no extensions UI', leaked.length === 0, leaked.join(', '));
  fs.rmSync(off, { recursive: true, force: true });

  console.log(failed ? `${failed} FAILED` : 'all passed');
  process.exit(failed ? 1 : 0);
})().catch(async (err) => {
  console.error(err);
  server?.kill('SIGTERM');
  process.exit(1);
});
