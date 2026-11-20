// The settings page on the mock engine, in a real browser: the gear opens it in a tab,
// what it shows, that its changes go through Go, and the keyboard section. The dev window
// shows the page in the settings tab's frame once Go opens the tab, as the real window
// does (core/cmd/dev/window.html); the test drives a page of its own, the same tab's at
// /settings/?tab=<id>, which talks to Go the same way.
// The page is a list of sections, one showing at a time (docs/settings.md).
// Run: node shell/e2e/settings.cjs   (from the repo root, after `npm run build` in shell/)

const { spawn, execFileSync } = require('child_process');
const fs = require('fs');
const http = require('http');
const os = require('os');
const path = require('path');
const { chromium } = require('playwright');


const root = path.resolve(__dirname, '../..');
const out = path.join(root, 'build/e2e');
// The names of the shells the user can choose, as the registry lists them.
const listedShells = JSON.parse(fs.readFileSync(path.join(root, 'core/shells/registry.json'), 'utf8')).filter((s) => !s.phone).map((s) => s.name);
// The web features, as their registry lists them.
const webFeatures = JSON.parse(fs.readFileSync(path.join(root, 'core/webfeatures/registry.json'), 'utf8'));
// The mock engine's address, on a port the system gives it, which it prints once it listens:
// two runs at once each have their own.
let base = '';

fs.mkdirSync(out, { recursive: true });
const shot = (name) => path.join(out, `${name}.png`);
// zipOf is a zip of [name, Buffer] pairs, stored as they are.
const zipOf = (files) => {
  const { crc32 } = require('zlib');
  const parts = [];
  const central = [];
  let at = 0;
  for (const [name, data] of files) {
    const n = Buffer.from(name);
    const head = Buffer.alloc(30);
    head.writeUInt32LE(0x04034b50, 0);
    head.writeUInt16LE(20, 4);
    head.writeUInt32LE(crc32(data), 14);
    head.writeUInt32LE(data.length, 18);
    head.writeUInt32LE(data.length, 22);
    head.writeUInt16LE(n.length, 26);
    const dir = Buffer.alloc(46);
    dir.writeUInt32LE(0x02014b50, 0);
    dir.writeUInt16LE(20, 4);
    dir.writeUInt16LE(20, 6);
    dir.writeUInt32LE(crc32(data), 16);
    dir.writeUInt32LE(data.length, 20);
    dir.writeUInt32LE(data.length, 24);
    dir.writeUInt16LE(n.length, 28);
    dir.writeUInt32LE(at, 42);
    parts.push(head, n, data);
    central.push(dir, n);
    at += 30 + n.length + data.length;
  }
  const size = central.reduce((s, b) => s + b.length, 0);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(files.length, 8);
  end.writeUInt16LE(files.length, 10);
  end.writeUInt32LE(size, 12);
  end.writeUInt32LE(at, 16);
  return Buffer.concat([...parts, ...central, end]);
};
// A listed profile, as a copy started on one has: its folder in a root beside the list.
const profilesRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-settings-'));
const profileID = 'a1b2c3d4';
const profile = path.join(profilesRoot, profileID);
fs.mkdirSync(profile);
fs.writeFileSync(path.join(profilesRoot, 'profiles.json'), JSON.stringify({ profiles: [{ id: profileID, name: 'Personal', color: '#4f9e9e', template: 'Personal' }] }));
const listed = () => JSON.parse(fs.readFileSync(path.join(profilesRoot, 'profiles.json'), 'utf8')).profiles[0];
const bin = path.join(out, 'dev');
execFileSync('go', ['build', '-tags', 'sqlite_fts5', '-o', bin, './cmd/dev'], { cwd: path.join(root, 'core'), stdio: 'inherit' });

// A release server whose stable channel is the version the app runs, so a check finds it
// up to date, on a port of its own. It counts manifest requests: each is one check. With
// stuck, it offers 0.3.0, whose download stops half-way and stays so, as one does across
// the Mac's sleep, until the test ends.
let manifests = 0;
let stuck = false;
let assets = 0;
const held = [];
const stuckAsset = Buffer.alloc(100000);
const releaseServer = http.createServer((req, res) => {
  if (req.url === '/asset.zip') {
    assets++;
    res.writeHead(200, { 'Content-Length': stuckAsset.length });
    res.write(stuckAsset.subarray(0, stuckAsset.length >> 1));
    held.push(res);
    return;
  }
  manifests++;
  const asset = { url: `http://127.0.0.1:${releaseServer.address().port}/asset.zip`, sha256: require('crypto').createHash('sha256').update(stuckAsset).digest('hex'), size: stuckAsset.length };
  res.end(JSON.stringify(stuck
    ? { spec: 1, project: 'project-b', published: '2026-11-09T00:00:00Z', channels: { stable: '0.3.0' }, releases: { '0.3.0': { assets: { [`${process.platform}/${process.arch}`]: asset } } } }
    : { spec: 1, project: 'project-b', published: '2026-11-09T00:00:00Z', channels: { stable: '0.2.0' }, releases: { '0.2.0': { assets: {} } } }));
});
const installed = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-installed-'));
fs.mkdirSync(path.join(installed, 'Project B.app/Contents/MacOS'), { recursive: true });

// Flags of the types registry.json has no row of yet, so the page draws every type.
const moreFlags = path.join(profile, 'more-flags.json');
fs.writeFileSync(moreFlags, JSON.stringify([
  { id: 'mode', name: 'Mode', description: 'An enum', switch: 'pb-mode', type: 'enum', choices: ['fast', 'slow'] },
  { id: 'label', name: 'Label', description: 'A string', switch: 'pb-label', type: 'string', pattern: '^[a-z]+$' },
  { id: 'sizes', name: 'Sizes', description: 'Numbers', switch: 'pb-sizes', type: 'numbers', min: 1 },
]));

let server;
async function startServer() {
  server = spawn(bin, ['-addr=127.0.0.1:0', `-shell=${path.join(root, 'shell/dist')}`, `-profile=${profile}`, `-root=${profilesRoot}`, `-id=${profileID}`, `-flags=${moreFlags}`,
    '-version=0.2.0', `-releases=http://127.0.0.1:${releaseServer.address().port}/releases.json`, `-app=${path.join(installed, 'Project B.app')}`], { stdio: ['ignore', 'ignore', 'pipe'] });
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
async function stopServer() {
  const gone = new Promise((r) => server.once('exit', r));
  server.kill('SIGTERM');
  await gone;
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
const updateSettings = () => {
  try {
    return JSON.parse(fs.readFileSync(path.join(profilesRoot, 'update/settings.json'), 'utf8'));
  } catch {
    return null;
  }
};
const prefsFile = () => JSON.parse(fs.readFileSync(path.join(profile, 'prefs.json'), 'utf8'));

const regions = async (q = '') => (await fetch(`${base}/mock/regions${q}`)).json();
const settle = (page) => page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(r, 60)))));

const keymapFile = () => {
  try {
    return JSON.parse(fs.readFileSync(path.join(profile, 'keymap.json'), 'utf8'));
  } catch {
    return null;
  }
};
// APCA's lightness contrast, Lc, of text on bg, two computed colours rgb(r, g, b), as a
// magnitude (APCA 0.0.98G), as core/tones picks a label by it.
function lightness(text, bg) {
  const y = (c) => {
    const [r, g, b] = c.match(/\d+/g).slice(0, 3).map((v) => (v / 255) ** 2.4);
    const v = 0.2126729 * r + 0.7151522 * g + 0.072175 * b;
    return v < 0.022 ? v + (0.022 - v) ** 1.414 : v;
  };
  const [t, b] = [y(text), y(bg)];
  if (b > t) {
    const s = (b ** 0.56 - t ** 0.57) * 1.14;
    return s < 0.1 ? 0 : (s - 0.027) * 100;
  }
  const s = (b ** 0.65 - t ** 0.62) * 1.14;
  return s > -0.1 ? 0 : -(s + 0.027) * 100;
}
const menuHas = (menu, title) => JSON.stringify(menu).includes(`"title":"${title}"`);

// Two sites' settings, as Go keeps them in the profile.
const sitesFile = () => JSON.parse(fs.readFileSync(path.join(profile, 'sites.json'), 'utf8'));
fs.writeFileSync(path.join(profile, 'sites.json'), JSON.stringify({
  'https://meet.test': { camera: 'allow', microphone: 'block' },
  'https://news.test': { notifications: 'block' },
}));

(async () => {
  await new Promise((r) => releaseServer.listen(0, '127.0.0.1', r));
  await startServer();
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
  const shellFrame = () => shell.frame({ url: /\/shell\/standard\/\?window=1$/ });
  await shell.goto(base + '/');
  // The window starts on a new tab: the checks want a page, so it loads one.
  await fetch(`${base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'navigate', text: 'https://example.com/' }) });
  await shell.frameLocator('iframe.mock-page').getByRole('button', { name: 'Navigate' }).waitFor();
  await settle(shell);
  const root = (name) => shellFrame().evaluate((n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim(), name);

  // The gear asks Go, and Go opens the settings tab, beside the active tab, and shows it.
  const settingsShown = async () => {
    const r = await regions();
    return r.settings !== 0 && r.active === r.settings && r.tabs.find((t) => t.id === r.settings).page === 'settings';
  };
  check('settings starts closed', (await regions()).settings === 0);
  await openSettings();
  check('the gear opens the settings tab and shows it', await until(settingsShown));
  check('and the dev window shows the page in the tab\'s frame', await until(() => shell.locator('iframe.our-page[data-page="settings"]').isVisible()));
  const firstTitle = 'Settings › Profile';
  check('its title is the page\'s own, with the section showing', await until(async () => (await regions()).tabs.find((t) => t.page === 'settings').title === firstTitle), JSON.stringify((await regions()).tabs.find((t) => t.page === 'settings')?.title));
  const titleBox = await ui.locator('form.field').boundingBox();
  check('the title in the address field is the shell\'s, not a drag area', await until(async () => (await regions(`?x=${Math.round(titleBox.x + titleBox.width / 2)}&y=${Math.round(titleBox.y + titleBox.height / 2)}`)).hit === 'shell'));
  // Its title and icon follow the page, as a web page's do.
  const ownFrame = () => shell.frame({ url: /\/settings\/\?tab=\d+$/ });
  const icon = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3E%3Ccircle cx='8' cy='8' r='7'/%3E%3C/svg%3E";
  await ownFrame().evaluate((href) => {
    document.title = 'Settings › Test';
    const link = Object.assign(document.createElement('link'), { id: 'test-icon', rel: 'icon', href });
    document.head.append(link);
  }, icon);
  check('a title and an icon the page sets are its tab\'s, and the shell shows the title', await until(async () => {
    const t = (await regions()).tabs.find((t) => t.page === 'settings');
    return t.title === 'Settings › Test' && t.favicon?.startsWith('data:image/svg+xml') && (await ui.locator(`[role=tab][data-tab="${t.id}"]`).textContent()).includes('Settings › Test');
  }), JSON.stringify((await regions()).tabs));
  await ownFrame().evaluate(() => {
    document.title = 'Settings';
    document.getElementById('test-icon').remove();
  });
  await until(async () => (await regions()).tabs.find((t) => t.page === 'settings').title === 'Settings');
  await settle(shell);
  await shell.screenshot({ path: shot('window-settings') });

  // The window, saved as name: the middle of the page area, the settings tab's, looks the
  // same with the shell's frame over it as without.
  const seesThrough = async (name) => {
    const b = (await regions()).bounds;
    const clip = { x: b.x + b.w / 4, y: b.y + b.h / 2, width: b.w / 2, height: b.h / 4 };
    const show = (selector, on) => shell.locator(selector).evaluate((el, v) => (el.style.visibility = v), on ? '' : 'hidden');
    await settle(shell);
    await shell.screenshot({ path: name });
    const over = await shell.screenshot({ clip });
    await show('iframe.shell', false);
    await settle(shell);
    const under = await shell.screenshot({ clip });
    await show('iframe.shell', true);
    return over.equals(under);
  };

  const open = async () => {
    const p = await browser.newPage({ viewport: { width: 1100, height: 700 } });
    watch(p);
    await p.goto(`${base}/settings/?tab=${(await regions()).settings}`);
    await p.getByRole('heading', { name: 'Settings' }).waitFor();
    return p;
  };
  let page = await open();
  // show goes to where a part of the page is, as a person does: the section it is in, from
  // the list of sections, and Sites, a row of Privacy that opens it (docs/settings.md).
  const sectionOf = {
    theme: 'appearance', frameColour: 'appearance', tabLayout: 'appearance', shell: 'appearance', start: 'start', searchEngine: 'start',
    workspaces: 'workspaces', bookmarks: 'bookmarks', sites: 'privacy', keys: 'keyboard', keyboard: 'keyboard', profile: 'profile',
    sparks: 'extensions', blockAds: 'privacy', clearData: 'privacy', clearOnQuit: 'privacy', askWhereToSave: 'downloads', downloadFolder: 'downloads', version: 'about', autoUpdate: 'about', flags: 'flags', webFeatures: 'webFeatures',
  };
  const show = async (part) => {
    await page.locator(`.nav [data-section="${sectionOf[part]}"]`).click();
    if (part === 'sites') await page.locator('[data-section="sites"]').click();
    await settle(page);
  };
  const radio = (label) => page.getByRole('radio', { name: label });
  const autoUpdate = page.getByRole('switch', { name: 'Automatic updates' });

  await show('tabLayout');
  check('it shows the current layout', (await radio('Vertical').isChecked()) && !(await radio('Horizontal').isChecked()));
  await show('version');
  check('it shows the running version', (await page.locator('.version').textContent()) === '0.2.0');
  check('it shows automatic updates on, the default', await autoUpdate.isChecked());
  // The status line, over every layout's page: the default browser alone; the version is
  // About's.
  const status = page.getByRole('region', { name: 'Status' });
  check('the status line says the app isn\'t the default browser, with a way to make it so, and not the version', (await status.textContent()).includes('Not your default browser') &&
    !(await status.textContent()).includes('0.2.0'));
  const about = page.getByRole('region', { name: 'Version' });
  await status.getByRole('button', { name: 'Make default browser' }).click();
  check('Make default browser asks the system, and the line says it is now', await until(async () => (await status.textContent()).includes('Your default browser') &&
    (await status.getByRole('button', { name: 'Make default browser' }).count()) === 0));
  await page.screenshot({ path: shot('settings-general') });

  // The search engine: chosen here, saved, and named in the shell's address field.
  await show('searchEngine');
  const engine = page.getByRole('combobox', { name: 'Search engine' });
  // The prefs file is first written by the first change.
  const savedEngine = () => (fs.existsSync(path.join(profile, 'prefs.json')) ? prefsFile().searchEngine : '');
  check('the search engine shows DuckDuckGo, the default', (await engine.inputValue()) === 'duckduckgo');
  await engine.selectOption('brave');
  // The address field shows on a web tab, not on the settings tab: the window shows the web
  // tab while the field is read, then the settings tab again.
  const ours = (await regions()).tabs.find((t) => t.page === 'settings');
  const web = (await regions()).tabs.find((t) => !t.page);
  const showTab = (id) => fetch(`${base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'activateTab', id }) });
  await showTab(web.id);
  check('another chosen is saved, and the address field names it', await until(async () => savedEngine() === 'brave' && (await ui.getByRole('combobox', { name: 'Address' }).getAttribute('placeholder')) === 'Search Brave Search or type a URL'), savedEngine());
  await showTab(ours.id);
  await engine.selectOption('duckduckgo');
  await until(() => savedEngine() === 'duckduckgo');

  await show('theme');
  await page.getByRole('radio', { name: 'Dark' }).check();
  check('Dark turns the shell dark and is saved', await until(async () => (await root('--text')) === '#e8eaed' && prefsFile().theme === 'dark'), await root('--text'));
  check('and the settings page too', await until(async () => (await page.evaluate(() => getComputedStyle(document.documentElement).colorScheme)) === 'dark'));
  await shell.screenshot({ path: shot('window-settings-dark') });
  check('in dark the page shows through the shell, which Chrome paints opaque if their schemes differ', await seesThrough(shot('window-dark')));
  await page.getByRole('radio', { name: 'Light' }).check();
  check('Light turns it back', await until(async () => (await root('--text')) === '#1f1f1f' && prefsFile().theme === 'light'));
  check('and in light too', await seesThrough(shot('window-light')));
  await page.getByRole('radio', { name: 'System' }).check();
  await until(() => prefsFile().theme === 'system');

  // On start: where you left off, the default, or a new tab.
  await show('start');
  const start = page.getByRole('radiogroup', { name: 'On start, open' });
  check('On start opens where you left off by default', (await start.getByRole('radio', { name: 'Where you left off' }).getAttribute('aria-checked')) === 'true');
  await start.getByRole('radio', { name: 'Start fresh' }).click();
  check('Start fresh is saved', await until(() => prefsFile().start === 'fresh'));
  await start.getByRole('radio', { name: 'Where you left off' }).click();
  await until(() => prefsFile().start === 'continue');

  await show('tabLayout');
  await radio('Horizontal').check();
  check('choosing Horizontal switches the shell and is saved', await until(async () => (await ui.locator('.strip').count()) > 0 && prefsFile().tabLayout === 'horizontal'), JSON.stringify(prefsFile()));
  await ui.getByRole('button', { name: 'Tabs on the side' }).click();
  check('a change from the toolbar reaches the settings page', await until(async () => await radio('Vertical').isChecked()));

  // The update manager checks by itself two seconds after it starts. Let that one land
  // first, so the count below is only the button's.
  await until(() => manifests > 0);
  const counted = manifests;
  await show('frameColour');
  await page.getByRole('radio', { name: 'Blue' }).click();
  check('a frame colour recolours the window and the shell, and is saved', await until(async () => (await root('--frame')) === '#5b7fd6' && (await regions()).windowColor === '#5b7fd6' && prefsFile().frameColor === '#5b7fd6'));
  const swatch = page.getByRole('radio', { name: 'Blue' });
  await swatch.press('ArrowRight');
  check('an arrow key moves the colour, and focus with it', await until(async () => prefsFile().frameColor === '#faa804' && (await page.getByRole('radio', { name: 'Amber' }).evaluate((e) => e === document.activeElement))));
  await page.getByRole('radio', { name: 'Blue' }).click();
  await show('version');
  await page.getByRole('button', { name: 'Check for updates' }).click();
  const updateStatus = about.getByRole('status');
  check('Check asks the update manager, which says it is up to date', await until(async () => (await updateStatus.textContent()).trim() === 'Up to date'));
  check('one check reached the release server', manifests === counted + 1, `${counted} -> ${manifests}`);
  await autoUpdate.click();
  check('the switch turns automatic updates off and saves it in the root\'s update folder, for every copy', await until(() => updateSettings()?.autoUpdate === false));
  // A release stuck half-way: Retry and Cancel are there while it downloads, in place of
  // Check for updates; Retry fetches it again, and Cancel stops it and leaves the version.
  stuck = true;
  await about.getByRole('button', { name: 'Check for updates' }).click();
  check('a download shows Cancel and Retry while it runs, in place of Check for updates',
    await until(async () => /Downloading 0\.3\.0… \d+%/.test(await updateStatus.textContent()) && (await about.getByRole('button', { name: 'Retry' }).isVisible()) &&
      (await about.getByRole('button', { name: 'Cancel' }).isVisible()) && (await about.getByRole('button', { name: 'Check for updates' }).count()) === 0),
    await updateStatus.textContent());
  await settle(page);
  await page.screenshot({ path: shot('settings-update-stuck') });
  // At a phone's width the row keeps Cancel then Retry beside what they act on, Retry last,
  // and the progress line stays under the download's words.
  // The row as wide as a 360-point phone's page leaves it.
  await about.evaluate((s) => { s.style.width = '328px'; });
  const narrow = await about.evaluate((s) => {
    const box = (e) => e.getBoundingClientRect();
    const row = box(s);
    const words = box(s.querySelector('[role="status"]'));
    const bar = box(s.querySelector('[role="progressbar"]'));
    const buttons = [...s.querySelectorAll('button')].filter((b) => /^(Retry|Cancel)$/.test(b.textContent.trim()));
    return { order: buttons.map((b) => b.textContent.trim()).join(' '), within: buttons.every((b) => box(b).right <= row.right + 1 && box(b).left >= row.left - 1),
      inside: bar.left >= words.left - 1 && bar.right <= words.right + 1 && bar.top >= words.bottom - 1 && bar.top <= words.bottom + 6 };
  });
  check('at a phone\'s width, Cancel then Retry stay in the row, and its progress line under its words',
    narrow.within && narrow.inside && narrow.order === 'Cancel Retry', JSON.stringify(narrow));
  await settle(page);
  await about.screenshot({ path: shot('settings-update-narrow') });
  await about.evaluate((s) => { s.style.width = ''; });
  await about.getByRole('button', { name: 'Retry' }).click();
  check('Retry fetches it again', await until(() => assets === 2), String(assets));
  await until(async () => /Downloading 0\.3\.0/.test(await updateStatus.textContent()));
  await about.getByRole('button', { name: 'Cancel' }).click();
  check('Cancel stops it, and Check for updates is back', await until(async () => !/Downloading/.test(await about.textContent()) &&
    (await about.getByRole('button', { name: 'Check for updates' }).isEnabled())), await about.textContent());
  stuck = false;

  // A dialog's answer reads on its fill in every preset colour, light and dark: its words
  // reach APCA's Lc 60, by the colours the page computes. New workspace's dialog
  // has one.
  await show('frameColour');
  const presets = await page.getByRole('radiogroup', { name: 'Frame colour' }).getByRole('radio')
    .evaluateAll((els) => els.map((e) => ({ name: e.getAttribute('aria-label'), rgb: getComputedStyle(e.querySelector('.ui-swatch-dot')).backgroundColor })));
  const toHex = (rgb) => '#' + rgb.match(/\d+/g).map((v) => Number(v).toString(16).padStart(2, '0')).join('');
  const rootOf = (p, name) => p.evaluate((n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim(), name);
  for (const theme of ['Light', 'Dark']) {
    for (const p of presets) {
      await show('theme');
      await page.getByRole('radio', { name: theme }).check();
      await page.getByRole('radio', { name: p.name, exact: true }).click();
      // None paints its grey for the theme (core/tones); its swatch is struck through.
      const frame = p.name === 'None' ? (theme === 'Dark' ? '#3a3f4a' : '#dcdee2') : toHex(p.rgb);
      const shown = await until(async () => (await rootOf(page, '--frame')) === frame && (await rootOf(page, '--text')) === (theme === 'Dark' ? '#e8eaed' : '#1f1f1f'));
      await show('workspaces');
      await page.getByRole('button', { name: 'New workspace…' }).click();
      const answer = page.getByRole('dialog').getByRole('button', { name: 'Create', exact: true });
      const [fg, bg] = await answer.evaluate((b) => [getComputedStyle(b).color, getComputedStyle(b).backgroundColor]);
      const lc = lightness(fg, bg);
      check(`a primary button reads in ${p.name}, ${theme.toLowerCase()}: Lc ${lc.toFixed(0)}`, shown && lc >= 60, `${fg} on ${bg}`);
      await page.keyboard.press('Escape');
    }
  }
  await show('theme');
  await page.getByRole('radio', { name: 'System' }).check();
  await page.getByRole('radio', { name: 'Slate' }).click();
  await until(() => prefsFile().theme === 'system' && prefsFile().frameColor === '#384562');

  // Profile: the name in the list, and reset to a template, which takes its values and
  // nothing else.
  await show('profile');
  const nameField = page.getByLabel('Profile name', { exact: true });
  const templateNow = () => page.locator('.template-row .secondary').textContent();
  check('Profile shows the name and the template', (await nameField.inputValue()) === 'Personal' && (await templateNow()).trim() === 'Personal');
  await nameField.fill('');
  await nameField.press('Enter');
  check('a blank name is refused under the field, and nothing is saved', await until(async () => (await page.getByRole('alert').textContent()) === 'A profile needs a name') && listed().name === 'Personal');
  await nameField.press('Escape');
  check('Escape puts the name back, and the tab stays open', await until(async () => (await nameField.inputValue()) === 'Personal') && (await regions()).settings !== 0);
  await nameField.fill('Home');
  await nameField.press('Enter');
  check('a new name is the list\'s', await until(() => listed().name === 'Home'), JSON.stringify(listed()));
  // The shell it loads: the ones the user can choose, a phone's not among them. A choice
  // is the profile's pref, and every window loads that shell at once.
  await show('shell');
  const shellChoice = page.getByLabel('Shell', { exact: true });
  check('Profile shows the shell in use, and offers only the listed shells', (await shellChoice.inputValue()) === 'standard' &&
    JSON.stringify(await shellChoice.locator('option').allTextContents()) === JSON.stringify(listedShells));
  await fetch(`${base}/settings/send?tab=${(await regions()).settings}`, { method: 'POST', body: JSON.stringify({ t: 'setShell', text: 'minimal' }) });
  check('choosing a shell saves it and the window loads it', await until(async () => prefsFile().shell === 'minimal' && (await regions()).shell === 'minimal'));
  await shellChoice.selectOption('standard');
  check('and choosing the standard one loads it again', await until(async () => prefsFile().shell === 'standard' && (await regions()).shell === 'standard' && (await ui.getByRole('button', { name: 'Menu', exact: true }).count()) === 1));
  await show('profile');
  const resetButton = page.getByRole('button', { name: 'Reset to template…' });
  await resetButton.click();
  const confirm = page.getByRole('dialog', { name: "Reset this profile's settings?" });
  check('reset asks first, with the template to reset to in the question', await confirm.isVisible());
  await confirm.getByLabel('Template').selectOption('Work');
  await page.screenshot({ path: shot('settings-profile-reset') });
  await page.keyboard.press('Escape');
  check('Escape answers Cancel, gives focus back and leaves the tab open', await until(async () => (await confirm.count()) === 0) && (await regions()).settings !== 0 && prefsFile().frameColor === '#384562' &&
    (await resetButton.evaluate((e) => e === document.activeElement)));
  await resetButton.click();
  await confirm.getByLabel('Template').selectOption('Work');
  await confirm.getByRole('button', { name: 'Reset', exact: true }).click();
  check('reset writes the template\'s values over the prefs', await until(() => prefsFile().frameColor === '#5b7fd6' && prefsFile().tabLayout === 'vertical'), JSON.stringify(prefsFile()));
  await show('profile');
  check('the shell follows at once', await until(async () => (await root('--frame')) === '#5b7fd6' && (await ui.locator('.sidebar').count()) > 0));
  check('and the Dock and the list', await until(async () => (await regions()).dock === '#5b7fd6' && listed().color === '#5b7fd6'), JSON.stringify(listed()));
  check('the list and the page name the template reset to', await until(async () => listed().template === 'Work' && (await templateNow()).trim() === 'Work'));
  await settle(page);
  await page.screenshot({ path: shot('settings-profile') });

  // Workspaces: the list, Main in a new profile, New Workspace with a name and a colour, and
  // rename. Go keeps them in the profile's folder, and the shell has them at once: a tab
  // opened in one, as Tab, New Tab in Workspace does, shows its dot, and a rename reaches it.
  const workspacesFile = () => {
    try {
      return JSON.parse(fs.readFileSync(path.join(profile, 'workspaces.json'), 'utf8')).workspaces;
    } catch {
      return [];
    }
  };
  // Bookmarks: where they show, each a switch, and the bar with the folder it shows.
  await show('bookmarks');
  const bookmarksSection = page.getByRole('region', { name: 'Bookmarks' });
  const place = (name) => bookmarksSection.getByRole('switch', { name });
  check('Bookmarks shows them on the new tab page and in the command box, not beside the tabs nor in a bar, by default',
    (await place(/On the new tab page/).isChecked()) && (await place(/In the command box/).isChecked()) && !(await place(/Beside the tabs/).isChecked()) && !(await place(/Bookmarks bar/).isChecked()));
  await place(/Beside the tabs/).click();
  await place(/On the new tab page/).click();
  check('turning them on and off is saved', await until(() => prefsFile().bookmarksSidebar === true && prefsFile().bookmarksNewTab === false), JSON.stringify(prefsFile()));
  await place(/Bookmarks bar/).click();
  const barFolder = bookmarksSection.getByRole('combobox', { name: 'Folder it shows' });
  check('the bar, turned on, is saved and asks which folder, the top until one is chosen', await until(async () => prefsFile().bookmarksBar === true && (await barFolder.isVisible()) && (await barFolder.inputValue()) === ''));
  await page.screenshot({ path: shot('settings-bookmarks') });
  for (const name of [/Beside the tabs/, /On the new tab page/, /Bookmarks bar/]) await place(name).click();
  await until(() => prefsFile().bookmarksSidebar === false && prefsFile().bookmarksNewTab === true && prefsFile().bookmarksBar === false);

  // Ask where to save: every download through the Save panel; not on a phone, which has none.
  await show('askWhereToSave');
  const askWhere = page.getByRole('switch', { name: /Ask where to save each file/ });
  check('Ask where to save is off by default', !(await askWhere.isChecked()));
  await askWhere.click();
  check('on, it is saved', await until(() => prefsFile().askWhereToSave === true));
  await askWhere.click();
  await until(() => prefsFile().askWhereToSave === false);

  // Save to: the Downloads folder to start; Change… takes the folder the system's panel
  // chose, here the mock's, and Use Downloads goes back.
  await show('downloadFolder');
  const saveTo = page.locator('.row', { has: page.getByText('Save to', { exact: true }) });
  check('Save to is the Downloads folder to start', await until(async () => (await saveTo.textContent()).includes('Downloads') && (await saveTo.getByRole('button', { name: 'Use Downloads' }).count()) === 0));
  const chosenFolder = fs.mkdtempSync(path.join(os.tmpdir(), 'pb-saved-'));
  await fetch(`${base}/mock/fire`, { method: 'POST', body: new URLSearchParams({ kind: 'choose', value: chosenFolder }) });
  await saveTo.getByRole('button', { name: /Change…/ }).click();
  check('Change… takes the folder chosen, and says so', await until(async () => prefsFile().downloadFolder === chosenFolder && (await saveTo.textContent()).includes(path.basename(chosenFolder))),
    JSON.stringify(prefsFile()));
  await settle(page);
  await page.screenshot({ path: shot('settings-downloads') });
  await saveTo.getByRole('button', { name: 'Use Downloads' }).click();
  check('Use Downloads goes back to the Downloads folder', await until(() => prefsFile().downloadFolder === ''));

  // Search: the rows whose label, second line or section's name has the words, each in
  // its section, working in place, and the sections counted; Escape puts the page back.
  const search = page.getByLabel('Search settings');
  await show('theme');
  await search.fill('bar');
  const bookmarksBar = page.getByRole('switch', { name: /Bookmarks bar/ });
  check('search shows the rows that match, and says how many', await until(async () => (await bookmarksBar.isVisible()) &&
    (await page.getByRole('radiogroup', { name: 'Theme' }).count()) === 0 && /^\d+ settings? match “bar”$/.test(await page.locator('.results').textContent())), await page.locator('.results').textContent());
  check('marking what it found', (await page.locator('mark.hit', { hasText: /^bar$/i }).count()) >= 2);
  check('and the list of sections counts them in each, and dims those with none', (await page.locator('.nav [data-section="bookmarks"] .count').textContent()).trim() === String(await bookmarksSection.locator('.row').count()) &&
    (await page.locator('.nav [data-section="appearance"]').evaluate((b) => b.classList.contains('dim'))));
  await settle(page);
  await page.screenshot({ path: shot('settings-search') });
  await bookmarksBar.click();
  check('a row found works in place', await until(() => prefsFile().bookmarksBar === true) && (await search.inputValue()) === 'bar');
  await bookmarksBar.click();
  await until(() => prefsFile().bookmarksBar === false);
  // A section's name finds its rows.
  await search.fill('about');
  check('a section\'s name finds every row in it', await until(() => page.getByRole('switch', { name: /Automatic updates/ }).isVisible()));
  await search.fill('zzzz');
  check('words nothing has say so', await until(async () => (await page.locator('.results').textContent()) === 'No settings match “zzzz”' && (await page.locator('.content .row').count()) === 0));
  await search.press('Escape');
  check('Escape empties it, and the page is back as it was', await until(async () => (await search.inputValue()) === '' && (await page.getByRole('radiogroup', { name: 'Theme' }).isVisible())) && (await regions()).settings !== 0);
  await search.fill('workspace');
  await page.locator('.content [data-section="workspaces"]').click();
  check('a long list found opens it, and the search ends', await until(async () => (await search.inputValue()) === '' && (await page.getByRole('button', { name: 'New workspace…' }).isVisible())));

  await show('workspaces');
  const workspacesSection = page.getByRole('region', { name: 'Workspaces' });
  const mainRow = workspacesSection.locator('.row', { hasText: 'Main' });
  check('Workspaces lists Main at first, with the tab open in it',
    (await workspacesSection.locator('.row').count()) === 1 && (await mainRow.textContent()).includes('1 tab') && workspacesFile().length === 0);
  await page.getByRole('button', { name: 'New workspace…' }).click();
  const wsDialog = page.getByRole('dialog', { name: 'New workspace' });
  const wsName = wsDialog.getByLabel('Name', { exact: true });
  check('New workspace… opens a dialog with the name ready to type', await until(async () => await wsName.evaluate((e) => e === document.activeElement)));
  check('its colours are gulal\'s seven hues and a +, and it starts on the one farthest from Main\'s', (await wsDialog.getByRole('radio').count()) === 7 &&
    (await wsDialog.getByRole('radio', { name: 'Teal' }).getAttribute('aria-checked')) === 'true' && (await wsDialog.getByLabel('Custom colour').getAttribute('type')) === 'color');
  await wsDialog.getByLabel('Custom colour').fill('#c2410c');
  await wsName.fill('Work');
  await wsName.press('Enter');
  check('Create makes one with its name and colour, kept in the profile after Main', await until(() => workspacesFile().length === 2 && workspacesFile()[0].name === 'Main' &&
    workspacesFile()[1].name === 'Work' && workspacesFile()[1].color === '#c2410c'), JSON.stringify(workspacesFile()));
  check('and lists it as a row with its menu', await until(async () => (await page.getByRole('button', { name: 'Actions for Work' }).count()) === 1 && (await wsDialog.count()) === 0));
  await page.getByRole('button', { name: 'New workspace…' }).click();
  await wsName.fill('work');
  await wsName.press('Enter');
  check('a name the list has is refused under the field', await until(async () => (await wsDialog.getByRole('alert').textContent()).includes('already') && workspacesFile().length === 2));
  await page.keyboard.press('Escape');
  check('Escape cancels the dialog and leaves the tab open', await until(async () => (await wsDialog.count()) === 0) && (await regions()).settings !== 0);
  const made = workspacesFile()[1].id;
  await fetch(`${base}/mock/fire`, { method: 'POST', body: new URLSearchParams({ kind: 'menu', value: 'workspaces:' + made }) });
  const dots = ui.locator('[role=tab] .workspace');
  const dot = dots.nth(1);
  check('a tab opened in it shows its dot in the shell at once', await until(async () => (await dots.count()) === 2 && (await dot.getAttribute('aria-label')) === 'In Work' &&
    (await dot.evaluate((e) => getComputedStyle(e).backgroundColor)) === 'rgb(194, 65, 12)'));
  check('and the settings tab, which is in no workspace, has none', (await ui.locator(`[role=tab][data-tab="${(await regions()).settings}"] .workspace`).count()) === 0);
  check('and the row counts it', await until(async () => (await workspacesSection.locator('.row', { hasText: 'Work' }).textContent()).includes('1 tab')));
  await workspacesSection.locator('.row', { hasText: 'Work' }).click({ button: 'right' });
  check('a right-click opens the row\'s menu', await until(async () => (await page.getByRole('menu', { name: 'Actions for Work' }).count()) === 1));
  await page.getByRole('menuitem', { name: 'Edit…' }).click();
  const editDialog = page.getByRole('dialog', { name: 'Edit workspace' });
  await editDialog.getByLabel('Name', { exact: true }).fill('Office');
  await editDialog.getByRole('button', { name: 'Save' }).click();
  check('Edit renames it, under the same id', await until(() => workspacesFile()[1].name === 'Office' && workspacesFile()[1].id === made), JSON.stringify(workspacesFile()));
  check('and the dot in the shell says it at once', await until(async () => (await dot.getAttribute('aria-label')) === 'In Office'));
  await page.getByRole('button', { name: 'Actions for Office' }).click();
  await page.getByRole('menuitem', { name: 'New tab in workspace' }).click();
  check('New tab in workspace opens a tab in it and shows it, the settings tab staying', await until(async () => {
    const r = await regions();
    return (await dots.count()) === 3 && r.settings !== 0 && r.active !== r.settings && r.tabs.find((t) => t.id === r.active).workspace === made;
  }));
  await openSettings();
  check('and the gear shows the settings tab again, the one the window has', await until(async () => (await settingsShown()) && (await regions()).tabs.filter((t) => t.page === 'settings').length === 1));
  await page.close();
  page = await open();
  await show('workspaces');
  await page.screenshot({ path: shot('settings-workspaces') });

  // Delete…, last in a row's menu in the danger colour, asks first: the dialog says how many
  // of its tabs close and that its cookies and site data go, and Delete is the danger
  // button. Delete closes its tabs and takes it out of the list; with one workspace left,
  // Delete… is disabled and the shell shows no dot.
  await page.getByRole('button', { name: 'Actions for Office' }).click();
  const deleteItem = page.getByRole('menuitem', { name: 'Delete…' });
  check('a row\'s menu ends with Delete…, in the danger colour', await deleteItem.isEnabled() && await deleteItem.evaluate((e) => e.classList.contains('danger') && !e.nextElementSibling));
  await deleteItem.click();
  const deleteDialog = page.getByRole('dialog', { name: 'Delete Office?' });
  check('Delete… asks first, saying its open tabs close and its site data go',
    (await deleteDialog.textContent()).includes('Its 2 open tabs close, and its cookies and site data are erased.'), await deleteDialog.textContent());
  const deleteButton = deleteDialog.getByRole('button', { name: 'Delete', exact: true });
  check('and Delete is the danger button', await deleteButton.evaluate((b) => b.matches('.text-button.danger')));
  await settle(page);
  await page.screenshot({ path: shot('settings-workspace-delete') });
  await page.keyboard.press('Escape');
  check('Escape cancels it, and nothing is deleted', await until(async () => (await deleteDialog.count()) === 0) && workspacesFile().length === 2 && (await dots.count()) === 3);
  await page.getByRole('button', { name: 'Actions for Office' }).click();
  await page.getByRole('menuitem', { name: 'Delete…' }).click();
  await deleteButton.click();
  check('Delete closes its tabs and takes it out of the list, the settings tab staying', await until(async () => {
    const r = await regions();
    const web = r.tabs.filter((t) => !t.page);
    return workspacesFile().length === 1 && workspacesFile()[0].name === 'Main' && web.length === 1 && web[0].workspace === '' && r.settings !== 0;
  }), JSON.stringify(workspacesFile()));
  check('and focus goes to the row left', await until(async () => page.getByRole('button', { name: 'Actions for Main' }).evaluate((e) => e === document.activeElement)));
  check('with one workspace left the shell shows no dot', await until(async () => (await dots.count()) === 0));
  await page.getByRole('button', { name: 'Actions for Main' }).click();
  check('and Delete… is disabled', await page.getByRole('menuitem', { name: 'Delete…' }).isDisabled());
  await page.keyboard.press('Escape');
  // A delete never closes the window, nor the settings tab, which is in no workspace:
  // deleting Main, which the window's only web tab is in, leaves the window its settings
  // tab. Another workspace first, as the last can't go.
  await page.getByRole('button', { name: 'New workspace…' }).click();
  const spareName = page.getByRole('dialog', { name: 'New workspace' }).getByLabel('Name', { exact: true });
  await spareName.fill('Spare');
  await spareName.press('Enter');
  await until(() => workspacesFile().length === 2);
  const spare = workspacesFile()[1].id;
  await page.getByRole('button', { name: 'Actions for Main' }).click();
  await page.getByRole('menuitem', { name: 'Delete…' }).click();
  const deleteMain = page.getByRole('dialog', { name: 'Delete Main?' });
  check('Main\'s delete says its one tab closes', (await deleteMain.textContent()).includes('Its 1 open tab closes'), await deleteMain.textContent());
  await deleteMain.getByRole('button', { name: 'Delete', exact: true }).click();
  check('deleting the workspace every web tab of the window is in keeps the window, with its settings tab', await until(async () => {
    const r = await regions();
    return workspacesFile().length === 1 && r.tabs.length === 1 && r.tabs[0].id === r.settings && !r.closed && !r.quit;
  }), JSON.stringify(await regions()));
  // Standard takes New Tab: its command box opens over the settings page, and an address
  // chosen there opens in a new tab.
  await ui.getByRole('button', { name: 'New Tab' }).first().click();
  const box = ui.getByRole('combobox', { name: 'Search tabs, history, actions or the web' });
  await box.fill('https://example.com/');
  await box.press('Enter');
  check('and a new tab from it opens in the first workspace left', await until(async () => {
    const r = await regions();
    return r.tabs.length === 2 && r.tabs[1].workspace === spare && r.active === r.tabs[1].id;
  }), JSON.stringify((await regions()).tabs));

  // Flags: saved for the next launch, with Relaunch offered until they are what runs.
  const flagsFile = () => {
    try {
      return JSON.parse(fs.readFileSync(path.join(profile, 'flags.json'), 'utf8'));
    } catch {
      return null;
    }
  };
  const flagsAsStarted = () => JSON.stringify(flagsFile() || {}) === '{}';
  await show('flags');
  const flagsRegion = page.getByRole('region', { name: 'Flags', exact: true });
  const relaunch = page.getByRole('button', { name: 'Relaunch' });
  check('Flags lists the flags, none waiting', await flagsRegion.getByText('Chromium features', { exact: true }).isVisible() && (await relaunch.count()) === 0);
  check('Extensions is off and can\'t be turned on in a build without them', await flagsRegion.getByRole('switch', { name: /Extensions/ }).isDisabled());
  check('and says so', (await flagsRegion.locator('.row', { hasText: 'Extensions' }).textContent()).includes('Not in this build'));
  const port = page.getByLabel('Remote debugging port');
  await port.fill('80');
  await port.press('Enter');
  check('a port it can\'t use is refused under the field, and what was typed stays', await until(async () => (await port.inputValue()) === '80' && (await page.locator('.row', { has: port }).getByRole('alert').count()) === 1) && flagsAsStarted());
  await port.fill('9333');
  await port.press('Enter');
  check('a port is saved, and Relaunch shows', await until(async () => flagsFile()?.remoteDebugging === '9333' && await relaunch.isVisible()), JSON.stringify(flagsFile()));
  check('once, at the top, with the flag tagged', (await relaunch.count()) === 1 && (await page.getByText('Relaunch to apply 1 change').isVisible()) && (await page.locator('.row', { has: port }).getByText('Relaunch to apply').count()) === 1);
  check('each flag says what it does', await page.getByText('Let DevTools clients connect on this port').isVisible());
  await page.screenshot({ path: shot('settings-flags') });
  await port.fill('');
  await port.press('Enter');
  check('cleared, nothing waits', await until(async () => (await relaunch.count()) === 0 && flagsAsStarted()));

  // The other types.
  const mode = page.getByLabel('Mode');
  check('an enum is a select, Off first', (await mode.evaluate((s) => [...s.options].map((o) => o.text).join())) === 'Off,fast,slow');
  await mode.selectOption('slow');
  check('picking a choice saves it', await until(async () => flagsFile()?.mode === 'slow' && (await page.locator('.row', { has: mode }).getByText('Relaunch to apply').count()) === 1));
  await mode.selectOption('');
  const label = page.getByLabel('Label');
  await label.fill('Not Lower');
  await label.press('Enter');
  check('a string the pattern refuses is not saved, and says why', await until(async () => (await page.locator('.row', { has: label }).getByRole('alert').count()) === 1) && !('label' in (flagsFile() || {})));
  await label.press('Escape');
  const sizes = page.getByLabel('Sizes');
  await sizes.fill('1, 2 ,3');
  await sizes.press('Enter');
  check('a list of numbers is saved without the spaces', await until(async () => flagsFile()?.sizes === '1,2,3' && (await sizes.inputValue()) === '1,2,3'), JSON.stringify(flagsFile()));
  await sizes.fill('1,0');
  await sizes.press('Enter');
  check('and refused when one is out of range', await until(async () => (await page.locator('.row', { has: sizes }).getByRole('alert').count()) === 1) && flagsFile()?.sizes === '1,2,3');
  await sizes.press('Escape');
  await page.screenshot({ path: shot('settings-flags-types') });
  await sizes.fill('');
  await sizes.press('Enter');
  await until(flagsAsStarted);

  // Web features: the registry, as Go sends it for this platform, only shown, in groups by
  // status, worst first.
  await show('webFeatures');
  const featureSection = page.locator('.web-features');
  const featureRows = featureSection.locator('.card .row');
  const featureHeadings = async () => featureSection.locator('h2').allTextContents();
  const macCount = (s) => webFeatures.filter((f) => f.status.mac === s).length;
  const statusOrder = [['quiet', 'Fails quietly'], ['partly', 'Partly'], ['off', 'Turned off'], ['absent', 'Absent'], ['works', 'Works'], ['kills', 'Crashes the page'], ['unknown', 'Not checked']];
  const wantHeadings = statusOrder.filter(([s]) => macCount(s)).map(([s, t]) => `${t} (${macCount(s)})`);
  check('Web features lists every row of the registry', await until(async () => (await featureRows.count()) === webFeatures.length), `${await featureRows.count()} of ${webFeatures.length}`);
  check('in groups by what a page gets on the Mac, worst first, each with its count', JSON.stringify(await featureHeadings()) === JSON.stringify(wantHeadings), JSON.stringify(await featureHeadings()));
  check('each group holds its rows', (await page.getByRole('region', { name: `Fails quietly (${macCount('quiet')})` }).locator('.row').count()) === macCount('quiet'));
  check('a line above the filter says what the section is', await page.getByText('How web features behave in Project B, and why. Read-only.').isVisible());
  const quietGroup = page.getByRole('region', { name: `Fails quietly (${macCount('quiet')})` });
  const offGroup = page.getByRole('region', { name: `Turned off (${macCount('off')})` });
  check('each row sits under what a page gets on the Mac, and says why', (await quietGroup.locator('.row', { hasText: 'App badge' }).first().textContent()).includes(webFeatures.find((f) => f.id === 'badge').why) && (await offGroup.locator('.row', { hasText: 'Installed related apps' }).count()) === 1);
  check('and carries no tag of its own: the heading says it', (await featureSection.locator('.tag').count()) === 0);
  const colourOf = (loc) => loc.evaluate((e) => getComputedStyle(e).color);
  const [danger, muted] = await page.evaluate(() => ['--danger', '--muted'].map((v) => {
    const probe = document.createElement('span');
    probe.style.color = `var(${v})`;
    document.body.append(probe);
    const c = getComputedStyle(probe).color;
    probe.remove();
    return c;
  }));
  check('the Fails quietly heading is in --danger', (await colourOf(quietGroup.locator('h2'))) === danger, `${await colourOf(quietGroup.locator('h2'))} want ${danger}`);
  check('and the others in --muted', (await colourOf(offGroup.locator('h2'))) === muted && muted !== danger, `${await colourOf(offGroup.locator('h2'))} want ${muted}`);
  const longWhy = featureSection.locator('.row', { hasText: 'Line wrap on view-source pages' }).locator('.why');
  check('a long reason wraps rather than being cut', await longWhy.evaluate((e) => e.scrollWidth <= e.clientWidth));
  check('and nothing on it can be changed', (await featureSection.locator('select, button, input:not(.filter)').count()) === 0);
  const featureFilter = page.getByLabel('Filter web features');
  await featureFilter.fill('turned off');
  check('the filter keeps the rows that match, by status too, with the group\'s count', await until(async () => (await featureRows.count()) === macCount('off') && JSON.stringify(await featureHeadings()) === JSON.stringify([`Turned off (${macCount('off')})`])), JSON.stringify(await featureHeadings()));
  await featureFilter.fill('absent');
  check('"absent" finds the Absent rows, under their heading', await until(async () => (await featureRows.count()) === macCount('absent') && JSON.stringify(await featureHeadings()) === JSON.stringify([`Absent (${macCount('absent')})`])), JSON.stringify(await featureHeadings()));
  await featureFilter.fill('play billing');
  check('and by the reason', await until(async () => (await featureRows.count()) === 1 && (await featureRows.first().textContent()).includes('Digital goods')));
  await featureFilter.fill('getInstalledRelatedApps');
  check('but not by the API', await until(async () => (await featureSection.getByText('No features match').isVisible()) && (await featureRows.count()) === 1));
  await featureFilter.press('Escape');
  check('Escape empties the filter, and leaves the page', await until(async () => (await featureRows.count()) === webFeatures.length) && (await regions()).settings !== 0);
  await page.screenshot({ path: shot('settings-web-features') });

  // Keyboard.
  await show('keyboard');
  const keyboardTitle = 'Settings › Keyboard';
  check('the page\'s title follows what shows, under its gear', await until(async () => (await page.title()) === keyboardTitle) &&
    (await page.locator('link[rel=icon]').getAttribute('href')).startsWith('data:image/svg+xml'), await page.title());
  // Back, the phone's key or the toolbar's, goes to where the page was, not out of it, and
  // Forward comes back (docs/shells.md, "Back on our pages").
  await page.goBack();
  check('Back goes to where the page was before the list, still in Settings', await until(async () => (await page.title()) !== keyboardTitle &&
    (await page.title()).startsWith('Settings') && page.url().includes('/settings/')), [await page.title(), page.url()]);
  await page.goForward();
  check('and Forward to the list again', await until(async () => (await page.title()) === keyboardTitle), await page.title());
  // A list's own arrow goes up to the place that holds it, as Android's Up does: never an
  // entry of its own, so Back after it never returns to the list.
  const arrow = page.locator('.list-page .list-head .back');
  if (await arrow.count()) {
    await arrow.click();
    await until(async () => (await page.title()) !== keyboardTitle);
    await page.goBack();
    check('the list\'s arrow then Back never returns to the list', !(await until(async () => (await page.title()) === keyboardTitle, 2000)), [await page.title(), page.url()]);
    if (!page.url().includes('/settings/')) await page.goForward();
    await until(async () => page.url().includes('/settings/') && (await page.title()).startsWith('Settings'));
    await show('keyboard');
    await until(async () => (await page.title()) === keyboardTitle);
  }
  // The list only shows; clicking an action opens its editor.
  const openRow = (title) => page.locator('button.action').filter({ has: page.locator('.name', { hasText: new RegExp('^\\s*' + title.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\s*$') }) }).click();
  check('a key the browser wins carries a tag, and a page-first key none', await until(async () => (await page.locator('button.action', { hasText: 'Close Tab' }).textContent()).includes('Browser first, not in full screen') && !(await page.locator('button.action', { hasText: 'Open Location' }).textContent()).includes('Browser first')));
  check('the list has no buttons of its own: no plus, remove or reset', (await page.getByRole('button', { name: /^(Add a key for|Remove |Reset (?!all))/ }).count()) === 0);
  check('the keymap follows its template until an edit', keymapFile() === null);
  check('with the standard shell in use, no shell\'s own actions show, and no Shells part', (await page.getByRole('region', { name: 'Shells' }).count()) === 0 &&
    (await page.locator('button.action', { hasText: 'Split right' }).count()) === 0 && (await page.locator('button.action', { hasText: 'Open Command Bar' }).count()) === 0);
  await fetch(`${base}/settings/send?tab=${(await regions()).settings}`, { method: 'POST', body: JSON.stringify({ t: 'setShell', text: 'glide' }) });
  check('with Glide in use, its own actions are listed last, under a Shells part, marked In use', await until(async () => {
    const glide = page.getByRole('region', { name: 'Glide', exact: true });
    if (!(await glide.count()) || !(await page.getByRole('region', { name: 'Shells' }).count())) return false;
    const add = await glide.locator('button.action', { hasText: 'Add to Split View' }).textContent();
    const last = await page.locator('section[aria-label]').last().getAttribute('aria-label');
    return (await glide.locator('h2').textContent()).includes('In use') && add.includes('Hidden') && last === 'Glide';
  }));
  check('and the shells that are off show none of theirs', (await page.locator('button.action', { hasText: 'Split right' }).count()) === 0);
  await page.screenshot({ path: shot('settings-keyboard-shells'), fullPage: true });
  await fetch(`${base}/settings/send?tab=${(await regions()).settings}`, { method: 'POST', body: JSON.stringify({ t: 'setShell', text: 'standard' }) });
  check('switching back takes the Shells part away', await until(async () => (await page.getByRole('region', { name: 'Shells' }).count()) === 0));
  await openRow('New Tab');
  await page.getByRole('button', { name: 'Add key' }).click();
  await page.keyboard.press('Meta+Shift+KeyK');
  check('a recorded key is added, and the keymap becomes the user\'s', await until(() => {
    const k = keymapFile();
    return k && !k.follows && k.template === 'project-b' && k.actions['tab.new'].mac.some((b) => b.keys === 'Shift+Cmd+K');
  }), JSON.stringify(keymapFile()?.actions?.['tab.new']));
  check('the page shows it', await until(async () => (await page.getByRole('region', { name: 'File', exact: true }).textContent()).includes('⇧⌘K')));
  await openRow('Quit Project B');
  check('Quit\'s key is locked: no remove button', (await page.getByRole('button', { name: 'Remove Cmd+Q' }).count()) === 0 && (await page.getByText('Locked', { exact: true }).count()) >= 1);
  check('Quit is always in the menu, said by a tag rather than a dead switch', (await page.getByRole('switch', { name: 'Quit Project B in the menu' }).count()) === 0 && (await page.getByText('Always in menu', { exact: true }).count()) === 1);

  await openRow('Reload This Page');
  await page.getByRole('switch', { name: 'Reload This Page in the menu', exact: true }).click();
  check('hiding an action takes it out of the menu', await until(async () => !menuHas((await regions()).menu, 'Reload This Page')));
  check('and the list marks it Hidden', await until(async () => (await page.locator('button.action', { hasText: 'Reload This Page' }).filter({ hasText: 'Hidden' }).count()) === 1));
  await page.getByRole('button', { name: 'Reset', exact: true }).click();
  check('reset puts it back', await until(async () => menuHas((await regions()).menu, 'Reload This Page')));
  await page.screenshot({ path: shot('settings-keyboard'), fullPage: true });

  await page.locator('body').click({ position: { x: 5, y: 300 } });
  await page.keyboard.press('Escape');
  check('Escape closes the open editor, and leaves the tab', await until(async () => (await page.locator('.editor').count()) === 0) && (await regions()).settings !== 0);
  await page.keyboard.press('Escape');
  await sleep(200);
  check('and Escape again leaves it too: the page closes as a tab does, not by itself', (await regions()).settings !== 0);

  // Keys, the keymap's template, is a row of the page's, out of the shortcuts' list.
  await show('keys');
  const keys = page.getByRole('radiogroup', { name: 'Keys' });
  check('Keys shows the template in use', (await keys.getByRole('radio', { name: 'Project B' }).getAttribute('aria-checked')) === 'true');
  await keys.getByRole('radio', { name: 'Chrome' }).click();
  const discard = page.getByRole('dialog', { name: /^Discard 1 changed shortcut and use/ });
  check('picking a template with a changed shortcut asks first, saying how many', await until(async () => (await discard.count()) === 1) && keymapFile()?.follows === undefined);
  await page.keyboard.press('Escape');
  check('Escape keeps the shortcuts and the tab, and the template in use', await until(async () => (await discard.count()) === 0) && keymapFile()?.follows === undefined && (await regions()).settings !== 0 &&
    (await keys.getByRole('radio', { name: 'Project B' }).getAttribute('aria-checked')) === 'true');
  await keys.getByRole('radio', { name: 'Chrome' }).click();
  await discard.getByRole('button', { name: 'Discard' }).click();
  check('Discard resets to following the template', await until(() => keymapFile()?.follows === 'chrome'), JSON.stringify(keymapFile()));
  await show('keyboard');
  check('and the added key is gone', await until(async () => !(await page.getByRole('region', { name: 'File', exact: true }).textContent()).includes('⇧⌘K')));

  // It closes as any tab does: Cmd+W while it is active.
  await openSettings();
  await until(settingsShown);
  await fetch(`${base}/mock/fire`, { method: 'POST', body: new URLSearchParams({ kind: 'key', value: 'closeTab' }) });
  check('Cmd+W closes the settings tab', await until(async () => (await regions()).settings === 0));
  check('and the dev window lets its page go', await until(async () => (await shell.locator('iframe.our-page').count()) === 0));

  // Site settings: a row per site saying how each of its settings is, and its menu to reset
  // one, or Remove, last, for all of them.
  await openSettings();
  await until(settingsShown);
  // What two sites keep, as pages would have stored it: a host of meet.test, and shop.test,
  // which has no setting. The page counts it as it opens.
  for (const value of ['mail.meet.test 2 3000000', 'shop.test 1 120000']) {
    await fetch(`${base}/mock/fire`, { method: 'POST', body: new URLSearchParams({ kind: 'siteData', value }) });
  }
  page = await open();
  await show('sites');
  const sites = page.getByRole('region', { name: 'Site settings' });
  const siteRow = (name) => sites.locator('.site-row', { has: page.getByRole('button', { name, exact: true }) });
  check('Sites lists each site with how each of its settings is, and what it keeps',
    await until(async () => (await siteRow('meet.test').textContent()).includes('Camera allowed, Microphone blocked · 3 MB') && (await siteRow('news.test').textContent()).includes('Notifications blocked') &&
      (await siteRow('shop.test').textContent()).includes('120 KB')),
    await sites.textContent());
  check('and under the list, how much every site keeps', (await sites.textContent()).includes('3 sites · 3.1 MB'));
  await settle(page);
  await page.screenshot({ path: shot('settings-sites') });
  // A site's Clear data asks first, and takes the site's row when it has no setting.
  await page.getByRole('button', { name: 'Actions for shop.test' }).click();
  await page.getByRole('menuitem', { name: 'Clear data…' }).click();
  const clearSite = page.getByRole('dialog', { name: 'Clear data for shop.test?' });
  check('Clear data asks first', await until(() => clearSite.isVisible()));
  await clearSite.getByRole('button', { name: 'Clear', exact: true }).click();
  check('and clears the site, which leaves the list', await until(async () => (await siteRow('shop.test').count()) === 0 && (await sites.textContent()).includes('2 sites · 3 MB')));
  // The site's own view says what it keeps on which hosts, and its Clear keeps the settings.
  await sites.getByRole('button', { name: 'meet.test', exact: true }).click();
  const meet = page.getByRole('region', { name: 'meet.test' });
  check('a site\'s view says what it keeps, and on which hosts',
    await until(async () => (await meet.textContent()).replace(/\s+/g, ' ').includes('Cookies and site data 2 cookies · 3 MB on mail.meet.test')), await meet.textContent());
  check('and its settings, as its row says them', (await meet.getByRole('radiogroup', { name: 'Camera' }).getByRole('radio', { name: 'Allow' }).getAttribute('aria-checked')) === 'true' &&
    (await meet.getByRole('radiogroup', { name: 'Microphone' }).getByRole('radio', { name: 'Block' }).getAttribute('aria-checked')) === 'true');
  await settle(page);
  await page.screenshot({ path: shot('settings-site-data') });
  // Clear cookies when the app quits, but the sites kept, each kept from its page, in the
  // card with what it keeps.
  await show('clearOnQuit');
  const quitSwitch = page.getByRole('switch', { name: /Clear cookies when the app quits/ });
  const quitRow = page.locator('.row', { has: quitSwitch });
  check('Clear cookies when the app quits is off to start', !(await quitSwitch.isChecked()));
  await quitSwitch.click();
  check('on, it is saved, and says every site\'s go', await until(async () => prefsFile().clearCookiesOnQuit === true && (await quitRow.textContent()).includes('Every site\'s, as the app quits')));
  await show('sites');
  await sites.getByRole('button', { name: 'meet.test', exact: true }).click();
  const keepSwitch = meet.getByRole('switch', { name: /Keep cookies when the app quits/ });
  check('a site\'s page asks whether to keep it, in the card with what it keeps',
    await until(async () => (await meet.locator('.card', { has: page.getByRole('switch', { name: /Keep cookies when the app quits/ }) }).textContent()).includes('Cookies and site data')));
  await keepSwitch.click();
  check('and keeps its cookies as the app quits', await until(() => JSON.stringify(prefsFile().keepCookies) === '["meet.test"]'), JSON.stringify(prefsFile()));
  await settle(page);
  await page.screenshot({ path: shot('settings-site-kept') });
  await page.getByRole('button', { name: 'Back to Sites' }).click();
  check('and its row says it is kept', await until(async () => (await siteRow('meet.test').textContent()).includes('Kept')));
  await show('clearOnQuit');
  check('and the switch says how many sites are kept', await until(async () => (await quitRow.textContent()).includes('Except the site you keep')));
  await quitRow.scrollIntoViewIfNeeded();
  await settle(page);
  await page.screenshot({ path: shot('settings-privacy-quit') });
  await quitSwitch.click();
  check('off, it is saved', await until(() => prefsFile().clearCookiesOnQuit === false));
  await show('sites');
  await sites.getByRole('button', { name: 'meet.test', exact: true }).click();
  await meet.getByRole('button', { name: 'Clear…' }).click();
  await page.getByRole('dialog', { name: 'Clear data for meet.test?' }).getByRole('button', { name: 'Clear', exact: true }).click();
  check('its Clear takes what it keeps and leaves its settings', await until(async () => !(await meet.textContent()).includes('Cookies and site data')) &&
    (await meet.getByRole('radiogroup', { name: 'Camera' }).getByRole('radio', { name: 'Allow' }).getAttribute('aria-checked')) === 'true');
  await page.getByRole('button', { name: 'Back to Sites' }).click();
  await until(() => sites.isVisible());
  await page.getByRole('button', { name: 'Actions for meet.test' }).click();
  await page.getByRole('menuitem', { name: 'Reset camera' }).click();
  check('Reset camera forgets only that', await until(() => JSON.stringify(sitesFile()['https://meet.test']) === '{"microphone":"block"}'), JSON.stringify(sitesFile()));
  check('and the row says so at once', await until(async () => !(await sites.textContent()).includes('Camera allowed')));
  // A page of the site open in a tab: its view says first how the page reached it.
  await fetch(`${base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'newTab', text: 'https://meet.test/room' }) });
  await until(async () => (await regions()).tabs.some((t) => t.url === 'https://meet.test/room' && !t.loading));
  // The row opens the site's own view: each setting settings can set, Ask, Allow or Block.
  await sites.getByRole('button', { name: 'meet.test', exact: true }).click();
  const site = page.getByRole('region', { name: 'meet.test' });
  const siteRadio = (group, name) => site.getByRole('radiogroup', { name: group }).getByRole('radio', { name });
  check('a site\'s row opens its own view, with each setting as it is',
    await until(async () => (await siteRadio('Microphone', 'Block').getAttribute('aria-checked')) === 'true' && (await siteRadio('Camera', 'Ask').getAttribute('aria-checked')) === 'true' && (await siteRadio('Notifications', 'Ask').getAttribute('aria-checked')) === 'true'));
  check('with a page of the site open, its view says first how the page reached it',
    await until(async () => (await site.locator('.row').first().textContent()).includes('Connection is secure')), await site.textContent());
  await settle(page);
  await page.screenshot({ path: shot('settings-site') });
  check('the site\'s view has the ad blocker, on, and links to other sites, allowed',
    (await siteRadio('Ad blocker', 'On').getAttribute('aria-checked')) === 'true' && (await siteRadio('Links to other sites', 'Allow').getAttribute('aria-checked')) === 'true');
  await siteRadio('Links to other sites', 'Block').click();
  check('Block there sets it for the site', await until(() => sitesFile()['https://meet.test']?.links === 'block'), JSON.stringify(sitesFile()));
  await siteRadio('Links to other sites', 'Allow').click();
  check('and Allow, the default, forgets it', await until(() => !('links' in (sitesFile()['https://meet.test'] || {}))), JSON.stringify(sitesFile()));
  // Block ads, off on every site: a site's view has no switch of its own for it.
  await show('blockAds');
  const blockAds = page.getByRole('switch', { name: /Block ads/ });
  check('Block ads is on every site by default', await blockAds.isChecked());
  await blockAds.click();
  check('off, it is saved, and says so', await until(async () => prefsFile().blockAds === false && (await page.locator('.row', { has: blockAds }).textContent()).includes('Off on every site')));
  await show('sites');
  await sites.getByRole('button', { name: 'meet.test', exact: true }).click();
  check('and a site\'s view has no ad blocker of its own', await until(() => siteRadio('Camera', 'Ask').isVisible()) && (await site.getByRole('radiogroup', { name: 'Ad blocker' }).count()) === 0);
  await show('blockAds');
  await blockAds.click();
  await until(() => prefsFile().blockAds === true);
  await show('sites');
  await sites.getByRole('button', { name: 'meet.test', exact: true }).click();
  await until(() => siteRadio('Camera', 'Ask').isVisible());
  await siteRadio('Camera', 'Allow').click();
  check('Allow there sets it for the site', await until(() => sitesFile()['https://meet.test']?.camera === 'allow'), JSON.stringify(sitesFile()));
  await siteRadio('Camera', 'Ask').click();
  check('and Ask resets it', await until(() => !('camera' in (sitesFile()['https://meet.test'] || {}))), JSON.stringify(sitesFile()));
  await page.getByRole('button', { name: 'Back to Sites' }).click();
  check('Back goes from a site\'s page to every site', await until(() => sites.isVisible()));
  await sites.getByRole('button', { name: 'meet.test', exact: true }).click();
  await site.getByRole('button', { name: 'Remove all for this site…' }).click();
  const removeAll = page.getByRole('dialog', { name: 'Remove all for meet.test?' });
  check('Remove all for this site asks first', await until(() => removeAll.isVisible()));
  await removeAll.getByRole('button', { name: 'Remove', exact: true }).click();
  check('and Remove forgets the site and goes back to every site', await until(async () => !('https://meet.test' in sitesFile()) && (await sites.isVisible()) && !(await sites.textContent()).includes('meet.test')), JSON.stringify(sitesFile()));
  await page.getByRole('button', { name: 'Actions for news.test' }).click();
  const removeItem = page.getByRole('menuitem', { name: 'Remove' });
  check('a site\'s menu ends with Remove, in the danger colour', await removeItem.evaluate((e) => e.classList.contains('danger') && !e.nextElementSibling));
  await removeItem.click();
  check('Remove forgets the site', await until(async () => !('https://news.test' in sitesFile()) && !(await sites.textContent()).includes('news.test')));
  check('with none left, the list says so', await until(async () => (await sites.textContent()).includes('No site keeps anything or has asked for a permission yet')));

  // Clear browsing data asks from when, all time to start, and what goes: history, cookies
  // and site data, and the cache ticked, History saying how many pages; Escape clears nothing.
  await show('clearData');
  const clearRow = page.locator('.row', { has: page.getByRole('button', { name: 'Clear…' }) });
  const clearDialog = page.getByRole('dialog', { name: 'Clear browsing data' });
  // Back closes a dialog, as Escape does, and the page stays.
  await page.getByRole('button', { name: 'Clear…' }).click();
  await until(() => clearDialog.isVisible());
  await page.goBack();
  check('Back closes a dialog, and the page stays where it was', await until(async () => !(await clearDialog.isVisible())) && page.url().includes('/settings/') &&
    (await page.getByRole('button', { name: 'Clear…' }).isVisible()));
  await page.getByRole('button', { name: 'Clear…' }).click();
  const from = clearDialog.getByLabel('From');
  check('Clear browsing data asks from when, all time to start, and what goes, the downloads list not ticked', await until(() => clearDialog.isVisible()) &&
    (await from.inputValue()) === 'all' &&
    (await clearDialog.getByRole('checkbox', { name: /History/ }).isChecked()) && (await clearDialog.getByRole('checkbox', { name: /Cookies and site data/ }).isChecked()) &&
    (await clearDialog.getByRole('checkbox', { name: /Cached images and files/ }).isChecked()) &&
    !(await clearDialog.getByRole('checkbox', { name: /downloads list/ }).isChecked()));
  check('History says how many pages the range takes', await until(async () => /History\s*\d[\d,]* pages?/.test(await clearDialog.textContent())));
  await settle(page);
  await page.screenshot({ path: shot('settings-clear-data') });
  await page.keyboard.press('Escape');
  check('Escape clears nothing', await until(async () => (await clearDialog.count()) === 0) && !(await clearRow.textContent()).includes('Cleared'));
  await page.getByRole('button', { name: 'Clear…' }).click();
  await from.selectOption('hour');
  await clearDialog.getByRole('checkbox', { name: /Cookies and site data/ }).uncheck();
  await clearDialog.getByRole('checkbox', { name: /Cached images and files/ }).uncheck();
  await clearDialog.getByRole('button', { name: 'Clear', exact: true }).click();
  check('Clear clears what is ticked from the time chosen, and the row says so', await until(async () => {
    const t = await clearRow.textContent();
    return t.includes('Cleared history') && t.includes('from the last hour') && !t.includes('cookies and site data');
  }));
  await page.close();
  await fetch(`${base}/mock/fire`, { method: 'POST', body: new URLSearchParams({ kind: 'key', value: 'closeTab' }) });
  await until(async () => (await regions()).settings === 0);

  // Sparks: New opens an editor; a save Go refuses says why; one it takes is listed, with
  // its switch and a menu, a tap on the row edits it, and the menu's Delete removes it.
  await openSettings();
  await until(settingsShown);
  page = await open();
  await show('sparks');
  const sparksRegion = page.getByRole('region', { name: 'Sparks' });
  const menuOf = async (name, item) => {
    await sparksRegion.getByRole('group', { name, exact: true }).getByRole('button', { name: `Actions for ${name}` }).click();
    await page.getByRole('menuitem', { name: item, exact: true }).click();
  };
  // A new profile has the three built-in sparks, off, as its own: Delete takes them away.
  const builtins = ['Video downloader', 'Screen time limit', 'Page cleanup'];
  check('a new profile lists the built-in sparks, each off', await until(async () => {
    for (const name of builtins) {
      const r = sparksRegion.getByRole('group', { name, exact: true });
      if (!(await r.isVisible()) || (await r.getByRole('switch').isChecked())) return false;
    }
    return (await sparksRegion.textContent()).includes('3 of 25.');
  }));
  for (const name of builtins) await menuOf(name, 'Delete');
  check('Sparks says there are none once they are deleted', await until(async () => (await sparksRegion.textContent()).includes('No sparks yet')));
  await page.getByRole('button', { name: 'New', exact: true }).click();
  const source = sparksRegion.getByRole('textbox', { name: 'Spark source' });
  await source.fill('// ==UserScript==\n// @name Nameless match\n// ==/UserScript==\n');
  await sparksRegion.getByRole('button', { name: 'Save' }).click();
  check('a spark without @match is refused, saying why', await until(async () => /@match/.test((await sparksRegion.getByRole('alert').textContent().catch(() => '')) || '')));
  await source.fill('// ==UserScript==\n// @name Wide pages\n// @match https://news.test/*\n// ==/UserScript==\ndocument.body.style.maxWidth = "none";\n');
  await sparksRegion.getByRole('button', { name: 'Save' }).click();
  const row = sparksRegion.getByRole('group', { name: 'Wide pages' });
  check('a good one is listed, on, with where it runs', await until(async () => (await row.isVisible()) && (await row.getByRole('switch').isChecked()) && (await row.textContent()).includes('https://news.test/*')));
  check('and the editor closes', await until(async () => (await source.count()) === 0));
  check('the list says how many of 25', (await sparksRegion.textContent()).includes('1 of 25.'));
  await settle(page);
  await page.screenshot({ path: shot('settings-sparks') });
  await row.getByRole('switch').click();
  check('its switch turns it off', await until(async () => !(await row.getByRole('switch').isChecked())));
  await row.locator('.name').click();
  check('a tap on the row edits it', await until(async () => (await source.inputValue().catch(() => '')).includes('@name Wide pages')));
  await sparksRegion.getByRole('button', { name: 'Cancel' }).click();
  await row.click({ button: 'right' });
  check('a right-click opens its menu: Edit…, Export, and Delete apart', await until(async () => (await page.getByRole('menuitem').allTextContents()).map((t) => t.trim()).join('|') === 'Edit…|Export|Delete'));
  await page.keyboard.press('Escape');
  await menuOf('Wide pages', 'Delete');
  check('Delete removes it', await until(async () => (await row.count()) === 0));

  // Import…: the owner's own sparks, in a zip a Mac made (its __MACOSX folder and a README
  // beside them) with a file that is no spark, are added and the rest not imported, saying
  // why; one he already has comes back in the same dialog as a clash, settled by Replace,
  // Rename or Ignore; and Export… makes a zip of the ones ticked that Import reads back.
  const sparksDir = path.resolve(__dirname, '../../core/sparks/testdata/site-sparks');
  const his = fs.readdirSync(sparksDir).filter((f) => f.endsWith('.user.js')).sort();
  const hisZip = zipOf([
    ...his.map((f) => [`SiteSparks/${f}`, fs.readFileSync(path.join(sparksDir, f))]),
    ...his.map((f) => [`__MACOSX/SiteSparks/._${f}`, Buffer.from('\x00\x05\x16\x07junk')]),
    ['SiteSparks/README.md', Buffer.from('# Sparks')],
  ]);
  const picker = sparksRegion.locator('input[type=file]');
  const dialog = page.getByRole('dialog');
  const said = async () => ((await dialog.innerText().catch(() => '')) || '').replace(/\s+/g, ' ');
  const more = page.getByRole('button', { name: 'More for sparks' });
  await more.click();
  check('New sits on the heading line, and Import… and Export… in the menu beside it', await until(async () => {
    const head = await ((await page.locator('.list-head').count()) ? page.locator('.list-head') : sparksRegion.locator('.heading')).boundingBox();
    const n = await sparksRegion.page().getByRole('button', { name: 'New', exact: true }).boundingBox();
    return (await page.getByRole('menuitem').allTextContents()).map((t) => t.trim()).join('|') === 'Import…|Export…' &&
      n && head && Math.abs(n.y + n.height / 2 - (head.y + head.height / 2)) < 4;
  }));
  await page.keyboard.press('Escape');
  await picker.setInputFiles([
    { name: 'SiteSparks.zip', mimeType: 'application/zip', buffer: hisZip },
    { name: 'notes.js', mimeType: 'text/javascript', buffer: Buffer.from('console.log(1);') },
  ]);
  check('a zip and a file import: his three added, the file that is no spark not imported, with why', await until(async () => {
    const t = await said();
    return (await dialog.getAttribute('aria-label').catch(() => '')) === 'Added 3 sparks · 1 not imported' &&
      t.includes('Limit Screen Time added') && t.includes('dhd added') && t.includes('notes.js') && t.includes('no ==UserScript== header');
  }));
  check('each is listed, on', await until(async () => {
    for (const name of ['Limit Screen Time', 'Page Cleanup', 'dhd']) {
      const r = sparksRegion.getByRole('group', { name, exact: true });
      if (!(await r.isVisible()) || !(await r.getByRole('switch').isChecked())) return false;
    }
    return true;
  }));
  check('a match of <all_urls> reads as every site, a real pattern as it is', (await sparksRegion.getByRole('group', { name: 'Page Cleanup' }).textContent()).includes('every site') &&
    (await sparksRegion.getByRole('group', { name: 'dhd', exact: true }).textContent()).includes('https://*/*'));
  await dialog.getByRole('button', { name: 'Done' }).click();
  check('Done closes it', await until(async () => (await dialog.count()) === 0));

  const dhd = fs.readFileSync(path.join(sparksDir, 'dhd.user.js'), 'utf8');
  const changed = (version) => ({ name: 'dhd.user.js', mimeType: 'text/javascript', buffer: Buffer.from(dhd.replace('// @name ', `// @version      ${version}\n// @name `)) });
  await picker.setInputFiles([changed('2'), { name: 'Again.zip', mimeType: 'application/zip', buffer: hisZip }]);
  const clashes = page.getByRole('dialog', { name: 'You already have this spark' });
  check('one he has, changed, asks in a dialog: Replace, Rename or Ignore, Replace chosen', await until(async () => {
    const t = (await clashes.textContent().catch(() => '')) || '';
    return t.includes('dhd') && (await clashes.getByRole('radio', { name: 'Replace' }).getAttribute('aria-checked')) === 'true' &&
      (await clashes.getByRole('radio', { name: 'Rename' }).count()) === 1 && (await clashes.getByRole('radio', { name: 'Ignore' }).count()) === 1;
  }));
  await page.keyboard.press('Escape');
  check('Escape leaves everything as it was', await until(async () => (await dialog.count()) === 0) &&
    (await sparksRegion.getByRole('group', { name: 'dhd 2', exact: true }).count()) === 0);
  await picker.setInputFiles([changed('2'), { name: 'Again.zip', mimeType: 'application/zip', buffer: hisZip }]);
  await until(() => clashes.isVisible());
  await clashes.getByRole('radio', { name: 'Rename' }).click();
  check('Rename says the name it gives', await until(async () => (await clashes.textContent()).includes('Adds it as “dhd 2”')));
  await settle(page);
  await page.screenshot({ path: shot('settings-sparks-clash') });
  await clashes.getByRole('button', { name: 'Import' }).click();
  check('it is added as dhd 2, said once, and the rest are already here', await until(async () => {
    const t = await said();
    return (await dialog.getAttribute('aria-label').catch(() => '')) === 'Added dhd 2 · 2 already here · 1 not imported' &&
      t.includes('dhd 2 added beside your dhd') && t.includes('Page Cleanup already here, unchanged') && t.includes('in the import twice, the first one used');
  }));
  await settle(page);
  await page.screenshot({ path: shot('settings-sparks-imported') });
  await dialog.getByRole('button', { name: 'Done' }).click();
  const renamedRow = sparksRegion.getByRole('group', { name: 'dhd 2', exact: true });
  check('and listed beside the one he had', await until(async () => (await renamedRow.isVisible()) && (await sparksRegion.getByRole('group', { name: 'dhd', exact: true }).isVisible())));

  await picker.setInputFiles([changed('3')]);
  check('a clash again', await until(async () => (await clashes.count()) === 1));
  await clashes.getByRole('radio', { name: 'Ignore' }).click();
  await clashes.getByRole('button', { name: 'Import' }).click();
  check('Ignore keeps his', await until(async () => (await dialog.getAttribute('aria-label').catch(() => '')) === 'Ignored dhd'));
  await dialog.getByRole('button', { name: 'Done' }).click();
  await picker.setInputFiles([changed('3')]);
  await until(() => clashes.isVisible());
  await clashes.getByRole('button', { name: 'Import' }).click();
  check('Replace puts it in place of his', await until(async () => (await dialog.getAttribute('aria-label').catch(() => '')) === 'Replaced dhd'));
  await dialog.getByRole('button', { name: 'Done' }).click();

  await more.click();
  await page.getByRole('menuitem', { name: 'Export…' }).click();
  const exporting = page.getByRole('dialog', { name: 'Export sparks' });
  const allBox = exporting.getByRole('checkbox', { name: 'All' });
  check('Export… lists every spark, ticked, under All', await until(async () => (await exporting.getByRole('checkbox').count()) === 5 && (await allBox.isChecked()) && (await exporting.getByRole('button', { name: 'Export 4 sparks' }).isVisible())));
  await exporting.getByRole('checkbox', { name: 'Page Cleanup' }).uncheck();
  check('All is mixed when some are ticked', await allBox.evaluate((b) => b.indeterminate && !b.checked));
  await settle(page);
  await page.screenshot({ path: shot('settings-sparks-export') });
  const [download] = await Promise.all([page.waitForEvent('download'), exporting.getByRole('button', { name: 'Export 3 sparks' }).click()]);
  const zipped = fs.readFileSync(await download.path());
  const zipNames = [...zipped.toString('latin1').matchAll(/PK\x01\x02[\s\S]{42}([^\x00]*?\.user\.js)/g)].map((m) => m[1]);
  check('it downloads Sparks.zip of the ticked ones', download.suggestedFilename() === 'Sparks.zip' && zipNames.join('|') === 'Limit Screen Time.user.js|dhd.user.js|dhd 2.user.js');
  for (const name of ['Limit Screen Time', 'Page Cleanup', 'dhd', 'dhd 2']) await menuOf(name, 'Delete');
  await picker.setInputFiles([{ name: 'Sparks.zip', mimeType: 'application/zip', buffer: zipped }]);
  check('and Import… reads that zip back', await until(async () => (await dialog.getAttribute('aria-label').catch(() => '')) === 'Added 3 sparks'));
  await dialog.getByRole('button', { name: 'Done' }).click();
  for (const name of ['Limit Screen Time', 'dhd', 'dhd 2']) await menuOf(name, 'Delete');
  check('and they delete', await until(async () => (await sparksRegion.textContent()).includes('No sparks yet')));
  await page.close();
  await fetch(`${base}/mock/fire`, { method: 'POST', body: new URLSearchParams({ kind: 'key', value: 'closeTab' }) });
  await until(async () => (await regions()).settings === 0);

  // Room between the page's parts: each heading clear of the card above it, and no two cards
  // touching. Sudipta found Bookmarks' heading right under the card above it on a phone.
  const crowded = (p) => p.evaluate(() => {
    const parts = [...document.querySelectorAll('main h2, main .card')].filter((e) => e.offsetParent && !e.closest('[role="dialog"]'))
      .map((e) => ({ e, r: e.getBoundingClientRect() })).filter(({ r }) => r.height > 0).sort((a, b) => a.r.top - b.r.top);
    const out = [];
    for (let i = 1; i < parts.length; i++) {
      const above = parts.slice(0, i).filter(({ e }) => !e.contains(parts[i].e) && !parts[i].e.contains(e));
      if (!above.length) continue;
      const bottom = Math.max(...above.map(({ r }) => r.bottom));
      const need = parts[i].e.tagName === 'H2' ? 16 : 8;
      if (parts[i].r.top - bottom < need) out.push(`${(parts[i].e.textContent || '').trim().slice(0, 30)}: ${Math.round(parts[i].r.top - bottom)}px`);
    }
    return out;
  });

  // On a phone a workspace's + opens our Custom colour sheet over the dialog, in place of the
  // system's picker; its changes show in the dialog at once, and Done keeps them there.
  async function phoneCustomColour() {
    await page.getByRole('button', { name: 'New workspace…' }).click();
    const dialog = page.getByRole('dialog', { name: 'New workspace' });
    const sheet = page.getByRole('dialog', { name: 'Custom colour' });
    await dialog.getByLabel('Name', { exact: true }).fill('Garden');
    const targets = await dialog.getByRole('radiogroup', { name: 'Colour' }).locator('.ui-swatch').evaluateAll((els) => els.map((e) => `${e.getBoundingClientRect().width}x${e.getBoundingClientRect().height}`));
    check('on a phone, each swatch is a 48px target', targets.length === 8 && targets.every((t) => t === '48x48'), targets.join());
    await dialog.getByRole('button', { name: 'Custom colour' }).tap();
    check('on a phone, a workspace\'s + opens the Custom colour sheet', await until(() => sheet.isVisible()) && (await sheet.locator('.ui-colour-chip').innerText()) === 'Garden');
    await sheet.getByRole('radio', { name: 'Light', exact: true }).tap();
    const hex = await sheet.getByRole('textbox', { name: 'Hex' }).inputValue();
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await settle(page);
      await page.screenshot({ path: shot(`settings-phone-custom-colour-${scheme}`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
    await sheet.getByRole('button', { name: 'Done' }).tap();
    const plus = dialog.getByRole('button', { name: 'Custom colour' });
    check('Done keeps it: the + wears the colour, and the dialog is still open', await until(async () => !(await sheet.isVisible())) && await dialog.isVisible() &&
      (await plus.locator('.ui-swatch-dot').evaluate((e) => getComputedStyle(e).backgroundColor)) === `rgb(${[1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16)).join(', ')})`, hex);
    await dialog.getByRole('button', { name: 'Cancel' }).click();
    await until(async () => !(await dialog.isVisible()));
  }

  // A phone reaches every group and list, a touch screen's page at a phone's size; what makes
  // no sense there (Tab layout, Keys, the shortcuts) isn't on it.
  await openSettings();
  await until(settingsShown);
  const phoneContext = await browser.newContext({ viewport: { width: 412, height: 860 }, hasTouch: true });
  page = await phoneContext.newPage();
  watch(page);
  await page.goto(`${base}/settings/?tab=${(await regions()).settings}`);
  await page.getByRole('heading', { name: 'Settings' }).waitFor();
  const phoneSearch = page.getByLabel('Search settings');
  await phoneSearch.fill('address bar');
  check('on a phone, search finds what is there', await until(() => page.getByRole('radiogroup', { name: 'Address bar' }).isVisible()) && (await page.getByRole('radiogroup', { name: 'Theme' }).count()) === 0);
  await settle(page);
  await page.screenshot({ path: shot('settings-phone-search') });
  await phoneSearch.fill('');
  // The phone's page is the list of sections; each opens, with a way back. A phone has no
  // Keyboard, nor Downloads, whose two rows are a desktop's.
  const sectionRows = page.locator('.section-row');
  const want = {
    profile: () => page.getByLabel('Profile name'),
    appearance: () => page.getByRole('radiogroup', { name: 'Address bar' }),
    start: () => page.getByRole('radiogroup', { name: 'On start, open' }),
    workspaces: () => page.getByRole('region', { name: 'Workspaces', exact: true }),
    bookmarks: () => page.getByRole('region', { name: 'Bookmarks', exact: true }),
    privacy: () => page.locator('[data-section="sites"]'),
    extensions: () => page.getByRole('region', { name: 'Sparks', exact: true }),
    about: () => page.getByRole('switch', { name: /Automatic updates/ }),
    flags: () => page.getByRole('region', { name: 'Flags', exact: true }),
    webFeatures: () => page.locator('.web-features'),
  };
  check('on a phone, the list has every section but Keyboard and Downloads, in order, each with what it is set to', JSON.stringify(await sectionRows.evaluateAll((rs) => rs.map((r) => r.dataset.section))) === JSON.stringify(Object.keys(want)) &&
    (await page.locator('.section-row[data-section="appearance"]').textContent()).includes('System'), JSON.stringify(await sectionRows.evaluateAll((rs) => rs.map((r) => r.dataset.section))));
  await page.screenshot({ path: shot('settings-phone') });
  // On the narrowest phone each section's name, and the line under it, stay on one line.
  await page.setViewportSize({ width: 360, height: 860 });
  await settle(page);
  const wrapped = await sectionRows.evaluateAll((rs) => rs.flatMap((r) => [...r.querySelectorAll('.label, .secondary')].map((e) => {
    const text = [...e.childNodes].find((n) => n.nodeType === 3 && n.textContent.trim());
    const range = document.createRange();
    range.selectNodeContents(text);
    return new Set([...range.getClientRects()].map((x) => Math.round(x.top))).size > 1 ? `${r.dataset.section}: ${text.textContent.trim()}` : '';
  })).filter((x) => x));
  check('at 360px, no section\'s name or line wraps', wrapped.length === 0, wrapped.join(' | '));
  check('and every section above the line has a line under its name', (await page.locator('.section-list:not(.below) .section-row').evaluateAll((rs) => rs.filter((r) => !r.querySelector('.secondary')).map((r) => r.dataset.section))).length === 0);
  await page.screenshot({ path: shot('settings-phone-360') });
  await page.setViewportSize({ width: 412, height: 860 });
  for (const [id, there] of Object.entries(want)) {
    await page.locator(`.section-row[data-section="${id}"]`).click();
    check(`on a phone, ${id} opens, under its own heading`, await until(() => there().isVisible()) && (await page.locator('.list-head .list-title').textContent()) !== '');
    const tight = await crowded(page);
    check(`and ${id}'s headings and cards have room above them`, tight.length === 0, tight.join(' | '));
    if (id === 'appearance') {
      await settle(page);
      await page.screenshot({ path: shot('settings-phone-section') });
      check('and there is no Tab layout on a phone', (await page.getByRole('radiogroup', { name: 'Tab layout' }).count()) === 0);
    }
    if (id === 'workspaces') await phoneCustomColour();
    if (id === 'privacy') {
      check('on a phone Privacy has no Clear cookies when the app quits', (await page.getByRole('switch', { name: /Clear cookies when the app quits/ }).count()) === 0);
      await page.locator('[data-section="sites"]').click();
      check('on a phone, Sites opens its page, and back goes to its section', await until(() => page.getByRole('region', { name: 'Site settings', exact: true }).isVisible()));
      await page.locator('.list-page .list-head .back').click();
      await until(() => there().isVisible());
    }
    if (id === 'extensions') {
      const there = page.getByRole('region', { name: 'Sparks', exact: true });
      // How the list, an import's clash, its outcome and Export… look on a phone, in dark.
      await page.emulateMedia({ colorScheme: 'dark' });
      const picker = there.locator('input[type=file]');
      const dialog = page.getByRole('dialog');
      await picker.setInputFiles([{ name: 'SiteSparks.zip', mimeType: 'application/zip', buffer: hisZip }]);
      await until(() => dialog.getByRole('button', { name: 'Done' }).isVisible());
      await dialog.getByRole('button', { name: 'Done' }).click();
      await picker.setInputFiles([changed('2')]);
      await until(() => dialog.getByRole('radio', { name: 'Rename' }).isVisible());
      await dialog.getByRole('radio', { name: 'Rename' }).click();
      await settle(page);
      await page.screenshot({ path: shot('settings-sparks-clash-phone-dark'), fullPage: true });
      await dialog.getByRole('button', { name: 'Import' }).click();
      await until(() => dialog.getByRole('button', { name: 'Done' }).isVisible());
      await settle(page);
      await page.screenshot({ path: shot('settings-sparks-imported-phone-dark'), fullPage: true });
      await dialog.getByRole('button', { name: 'Done' }).click();
      await settle(page);
      await page.screenshot({ path: shot('settings-sparks-phone-dark'), fullPage: true });
      await page.getByRole('button', { name: 'More for sparks' }).click();
      await page.getByRole('menuitem', { name: 'Export…' }).click();
      await settle(page);
      await page.screenshot({ path: shot('settings-sparks-export-phone-dark'), fullPage: true });
      await dialog.getByRole('button', { name: 'Cancel' }).click();
      for (const name of ['Limit Screen Time', 'Page Cleanup', 'dhd', 'dhd 2']) {
        await there.getByRole('group', { name, exact: true }).getByRole('button', { name: `Actions for ${name}` }).click();
        await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();
      }
      await until(async () => (await there.textContent()).includes('No sparks yet'));
      await page.emulateMedia({ colorScheme: 'light' });
    }
    await page.getByRole('button', { name: 'Back to Settings' }).click();
    await until(() => sectionRows.first().isVisible());
  }
  await phoneContext.close();
  await fetch(`${base}/mock/fire`, { method: 'POST', body: new URLSearchParams({ kind: 'key', value: 'closeTab' }) });
  await until(async () => (await regions()).settings === 0);

  check('no errors in the pages', errors.length === 0, errors.join(' | '));
  await browser.close();
  await stopServer();
  held.forEach((res) => res.destroy());
  releaseServer.close();
  console.log(failed ? `${failed} FAILED` : 'all passed');
  process.exit(failed ? 1 : 0);
})();
