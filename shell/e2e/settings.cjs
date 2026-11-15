// The settings page on the mock engine, in a real browser: the gear opens it in a tab,
// what it shows, that its changes go through Go, and the keyboard section. The dev window
// shows the page in the settings tab's frame once Go opens the tab, as the real window
// does (core/cmd/dev/window.html); the test drives a page of its own, the same tab's at
// /settings/?tab=<id>, which talks to Go the same way.
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
// A listed profile, as a copy started on one has: its folder in a root beside the list.
const profilesRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-settings-'));
const profileID = 'a1b2c3d4';
const profile = path.join(profilesRoot, profileID);
fs.mkdirSync(profile);
fs.writeFileSync(path.join(profilesRoot, 'profiles.json'), JSON.stringify({ profiles: [{ id: profileID, name: 'Personal', color: '#4f9e9e', template: 'Personal' }] }));
const listed = () => JSON.parse(fs.readFileSync(path.join(profilesRoot, 'profiles.json'), 'utf8')).profiles[0];
const bin = path.join(out, 'dev');
execFileSync('go', ['build', '-o', bin, './cmd/dev'], { cwd: path.join(root, 'core'), stdio: 'inherit' });

// A release server whose stable channel is the version the app runs, so a check finds it
// up to date, on a port of its own. It counts manifest requests: each is one check.
let manifests = 0;
const releaseServer = http.createServer((req, res) => {
  manifests++;
  res.end(JSON.stringify({ spec: 1, project: 'project-b', published: '2026-11-09T00:00:00Z', channels: { stable: '0.2.0' }, releases: { '0.2.0': { assets: {} } } }));
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
// WCAG 2's contrast ratio of two computed colours, rgb(r, g, b).
function contrast(a, b) {
  const luminance = (c) => {
    const [r, g, bl] = c.match(/\d+/g).slice(0, 3).map((v) => {
      const s = v / 255;
      return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * r + 0.7152 * g + 0.0722 * bl;
  };
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
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
  const shellFrame = () => shell.frame({ url: /\/shell\/standard\/\?window=1$/ });
  await shell.goto(base + '/');
  // Standard starts with no tab: the checks want a page, so one is opened.
  await fetch(`${base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'newTab', text: 'https://example.com/' }) });
  await shell.frameLocator('iframe.mock-page').getByRole('button', { name: 'Navigate' }).waitFor();
  await settle(shell);
  const root = (name) => shellFrame().evaluate((n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim(), name);

  // The gear asks Go, and Go opens the settings tab, beside the active tab, and shows it.
  const settingsShown = async () => {
    const r = await regions();
    return r.settings !== 0 && r.active === r.settings && r.tabs.find((t) => t.id === r.settings).page === 'settings';
  };
  check('settings starts closed', (await regions()).settings === 0);
  await ui.getByRole('button', { name: 'Settings', exact: true }).click();
  check('the gear opens the settings tab and shows it', await until(settingsShown));
  check('and the dev window shows the page in the tab\'s frame', await until(() => shell.locator('iframe.our-page[data-page="settings"]').isVisible()));
  check('its title is the page\'s own', await until(async () => (await regions()).tabs.find((t) => t.page === 'settings').title === 'Settings'));
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
  await shell.screenshot({ path: path.join(out, 'window-settings.png') });

  // The window, saved as name: the middle of the page area, the settings tab's, looks the
  // same with the shell's frame over it as without.
  const seesThrough = async (name) => {
    const b = (await regions()).bounds;
    const clip = { x: b.x + b.w / 4, y: b.y + b.h / 2, width: b.w / 2, height: b.h / 4 };
    const show = (selector, on) => shell.locator(selector).evaluate((el, v) => (el.style.visibility = v), on ? '' : 'hidden');
    await settle(shell);
    await shell.screenshot({ path: path.join(out, name) });
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
  const radio = (label) => page.getByRole('radio', { name: label });
  const autoUpdate = page.getByRole('switch', { name: 'Automatic updates' });

  check('it shows the current layout', (await radio('Vertical').isChecked()) && !(await radio('Horizontal').isChecked()));
  check('it shows the running version', (await page.locator('.version').textContent()) === '0.2.0');
  check('it shows automatic updates on, the default', await autoUpdate.isChecked());
  await page.screenshot({ path: path.join(out, 'settings-general.png') });

  // The search engine: chosen here, saved, and named in the shell's address field.
  const engine = page.getByRole('combobox', { name: 'Search engine' });
  // The prefs file is first written by the first change.
  const savedEngine = () => (fs.existsSync(path.join(profile, 'prefs.json')) ? prefsFile().searchEngine : '');
  check('the search engine shows DuckDuckGo, the default', (await engine.inputValue()) === 'duckduckgo');
  await engine.selectOption('brave');
  check('another chosen is saved, and the address field names it', await until(async () => savedEngine() === 'brave' && (await ui.getByRole('combobox', { name: 'Address' }).getAttribute('placeholder')) === 'Search Brave Search or type a URL'), savedEngine());
  await engine.selectOption('duckduckgo');
  await until(() => savedEngine() === 'duckduckgo');

  await page.getByRole('radio', { name: 'Dark' }).check();
  check('Dark turns the shell dark and is saved', await until(async () => (await root('--text')) === '#e8eaed' && prefsFile().theme === 'dark'), await root('--text'));
  check('and the settings page too', await until(async () => (await page.evaluate(() => getComputedStyle(document.documentElement).colorScheme)) === 'dark'));
  await shell.screenshot({ path: path.join(out, 'window-settings-dark.png') });
  check('in dark the page shows through the shell, which Chrome paints opaque if their schemes differ', await seesThrough('window-dark.png'));
  await page.getByRole('radio', { name: 'Light' }).check();
  check('Light turns it back', await until(async () => (await root('--text')) === '#1f1f1f' && prefsFile().theme === 'light'));
  check('and in light too', await seesThrough('window-light.png'));
  await page.getByRole('radio', { name: 'System' }).check();
  await until(() => prefsFile().theme === 'system');

  await radio('Horizontal').check();
  check('choosing Horizontal switches the shell and is saved', await until(async () => (await ui.locator('.strip').count()) > 0 && prefsFile().tabLayout === 'horizontal'), JSON.stringify(prefsFile()));
  await ui.getByRole('button', { name: 'Tabs on the side' }).click();
  check('a change from the toolbar reaches the settings page', await until(async () => await radio('Vertical').isChecked()));

  // The update manager checks by itself two seconds after it starts. Let that one land
  // first, so the count below is only the button's.
  await until(() => manifests > 0);
  const counted = manifests;
  await page.getByRole('radio', { name: 'Blue' }).click();
  check('a frame colour recolours the window and the shell, and is saved', await until(async () => (await root('--frame')) === '#5b7fd6' && (await regions()).windowColor === '#5b7fd6' && prefsFile().frameColor === '#5b7fd6'));
  const swatch = page.getByRole('radio', { name: 'Blue' });
  await swatch.press('ArrowRight');
  check('an arrow key moves the colour, and focus with it', await until(async () => prefsFile().frameColor === '#faa804' && (await page.getByRole('radio', { name: 'Amber' }).evaluate((e) => e === document.activeElement))));
  await page.getByRole('radio', { name: 'Blue' }).click();
  await page.getByRole('button', { name: 'Check for updates' }).click();
  const updateStatus = page.getByRole('region', { name: 'Updates' }).getByRole('status');
  check('Check asks the update manager, which says it is up to date', await until(async () => (await updateStatus.textContent()).trim() === 'Up to date'));
  check('one check reached the release server', manifests === counted + 1, `${counted} -> ${manifests}`);
  await autoUpdate.click();
  check('the switch turns automatic updates off and saves it in the root\'s update folder, for every copy', await until(() => updateSettings()?.autoUpdate === false));

  // A dialog's answer reads on its fill in every preset colour, light and dark: its words
  // reach WCAG's 4.5 for text, by the colours the page computes. New workspace's dialog
  // has one.
  const presets = await page.getByRole('radiogroup', { name: 'Frame colour' }).getByRole('radio')
    .evaluateAll((els) => els.map((e) => ({ name: e.getAttribute('aria-label'), rgb: getComputedStyle(e).backgroundColor })));
  const toHex = (rgb) => '#' + rgb.match(/\d+/g).map((v) => Number(v).toString(16).padStart(2, '0')).join('');
  const rootOf = (p, name) => p.evaluate((n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim(), name);
  for (const theme of ['Light', 'Dark']) {
    for (const p of presets) {
      await page.getByRole('button', { name: 'General', exact: true }).click();
      await page.getByRole('radio', { name: theme }).check();
      await page.getByRole('radio', { name: p.name, exact: true }).click();
      const shown = await until(async () => (await rootOf(page, '--frame')) === toHex(p.rgb) && (await rootOf(page, '--text')) === (theme === 'Dark' ? '#e8eaed' : '#1f1f1f'));
      await page.getByRole('button', { name: 'Workspaces', exact: true }).click();
      await page.getByRole('button', { name: 'New workspace…' }).click();
      const answer = page.getByRole('dialog').getByRole('button', { name: 'Create', exact: true });
      const [fg, bg] = await answer.evaluate((b) => [getComputedStyle(b).color, getComputedStyle(b).backgroundColor]);
      const ratio = contrast(fg, bg);
      check(`a primary button reads in ${p.name}, ${theme.toLowerCase()}: ${ratio.toFixed(2)}`, shown && ratio >= 4.5, `${fg} on ${bg}`);
      await page.keyboard.press('Escape');
    }
  }
  await page.getByRole('button', { name: 'General', exact: true }).click();
  await page.getByRole('radio', { name: 'System' }).check();
  await page.getByRole('radio', { name: 'Slate' }).click();
  await until(() => prefsFile().theme === 'system' && prefsFile().frameColor === '#384562');

  // Profile: the name in the list, and reset to a template, which takes its values and
  // nothing else.
  await page.getByRole('button', { name: 'Profile', exact: true }).click();
  const profileSection = page.locator('main');
  const nameField = page.getByLabel('Name', { exact: true });
  const templateNow = () => profileSection.locator('.row', { hasText: 'Template' }).locator('.secondary').textContent();
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
  const shellChoice = page.getByLabel('Shell', { exact: true });
  check('Profile shows the shell in use, and offers only the listed shells', (await shellChoice.inputValue()) === 'standard' &&
    JSON.stringify(await shellChoice.locator('option').allTextContents()) === JSON.stringify(listedShells));
  await fetch(`${base}/settings/send?tab=${(await regions()).settings}`, { method: 'POST', body: JSON.stringify({ t: 'setShell', text: 'minimal' }) });
  check('choosing a shell saves it and the window loads it', await until(async () => prefsFile().shell === 'minimal' && (await regions()).shell === 'minimal'));
  await shellChoice.selectOption('standard');
  check('and choosing the standard one loads it again', await until(async () => prefsFile().shell === 'standard' && (await regions()).shell === 'standard' && (await ui.getByRole('button', { name: 'Settings', exact: true }).count()) === 1));
  const resetButton = page.getByRole('button', { name: 'Reset to template…' });
  await resetButton.click();
  const confirm = page.getByRole('dialog', { name: "Reset this profile's settings?" });
  check('reset asks first, with the template to reset to in the question', await confirm.isVisible());
  await confirm.getByLabel('Template').selectOption('Work');
  await page.screenshot({ path: path.join(out, 'settings-profile-reset.png') });
  await page.keyboard.press('Escape');
  check('Escape answers Cancel, gives focus back and leaves the tab open', await until(async () => (await confirm.count()) === 0) && (await regions()).settings !== 0 && prefsFile().frameColor === '#384562' &&
    (await resetButton.evaluate((e) => e === document.activeElement)));
  await resetButton.click();
  await confirm.getByLabel('Template').selectOption('Work');
  await confirm.getByRole('button', { name: 'Reset', exact: true }).click();
  check('reset writes the template\'s values over the prefs', await until(() => prefsFile().frameColor === '#5b7fd6' && prefsFile().tabLayout === 'vertical'), JSON.stringify(prefsFile()));
  check('the shell follows at once', await until(async () => (await root('--frame')) === '#5b7fd6' && (await ui.locator('.sidebar').count()) > 0));
  check('and the Dock and the list', await until(async () => (await regions()).dock === '#5b7fd6' && listed().color === '#5b7fd6'), JSON.stringify(listed()));
  check('the list and the page name the template reset to', await until(async () => listed().template === 'Work' && (await templateNow()).trim() === 'Work'));
  await settle(page);
  await page.screenshot({ path: path.join(out, 'settings-profile.png') });

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
  await page.getByRole('button', { name: 'Workspaces', exact: true }).click();
  const workspacesSection = page.getByRole('region', { name: 'Workspaces' });
  const mainRow = workspacesSection.locator('.row', { hasText: 'Main' });
  check('Workspaces lists Main at first, with the tab open in it',
    (await workspacesSection.locator('.row').count()) === 1 && (await mainRow.textContent()).includes('1 tab') && workspacesFile().length === 0);
  await page.getByRole('button', { name: 'New workspace…' }).click();
  const wsDialog = page.getByRole('dialog', { name: 'New workspace' });
  const wsName = wsDialog.getByLabel('Name', { exact: true });
  check('New workspace… opens a dialog with the name ready to type', await until(async () => await wsName.evaluate((e) => e === document.activeElement)));
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
  await ui.getByRole('button', { name: 'Settings', exact: true }).click();
  check('and the gear shows the settings tab again, the one the window has', await until(async () => (await settingsShown()) && (await regions()).tabs.filter((t) => t.page === 'settings').length === 1));
  await page.close();
  page = await open();
  await page.getByRole('button', { name: 'Workspaces', exact: true }).click();
  await settle(page);
  await page.screenshot({ path: path.join(out, 'settings-workspaces.png') });

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
  await page.screenshot({ path: path.join(out, 'settings-workspace-delete.png') });
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
  await page.getByRole('button', { name: 'Flags' }).click();
  const relaunch = page.getByRole('button', { name: 'Relaunch' });
  check('Flags lists the flags, none waiting', await page.getByRole('region', { name: 'Flags' }).getByText('Chromium features', { exact: true }).isVisible() && (await relaunch.count()) === 0);
  check('Extensions is off and can\'t be turned on in a build without them', await page.getByRole('switch', { name: /Extensions/ }).isDisabled());
  check('and says so', (await page.locator('.row', { hasText: 'Extensions' }).textContent()).includes('Not in this build'));
  const port = page.getByLabel('Remote debugging port');
  await port.fill('80');
  await port.press('Enter');
  check('a port it can\'t use is refused under the field, and what was typed stays', await until(async () => (await port.inputValue()) === '80' && (await page.locator('.row', { has: port }).getByRole('alert').count()) === 1) && flagsFile() === null);
  await port.fill('9333');
  await port.press('Enter');
  check('a port is saved, and Relaunch shows', await until(async () => flagsFile()?.remoteDebugging === '9333' && await relaunch.isVisible()), JSON.stringify(flagsFile()));
  check('once, at the top, with the flag tagged', (await relaunch.count()) === 1 && (await page.getByText('Relaunch to apply 1 change').isVisible()) && (await page.locator('.row', { has: port }).getByText('Relaunch to apply').count()) === 1);
  check('each flag says what it does', await page.getByText('Let DevTools clients connect on this port').isVisible());
  await page.screenshot({ path: path.join(out, 'settings-flags.png') });
  await port.fill('');
  await port.press('Enter');
  check('cleared, nothing waits', await until(async () => (await relaunch.count()) === 0 && JSON.stringify(flagsFile()) === '{}'));

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
  await page.screenshot({ path: path.join(out, 'settings-flags-types.png') });
  await sizes.fill('');
  await sizes.press('Enter');
  await until(() => JSON.stringify(flagsFile()) === '{}');

  // Web features: the registry, as Go sends it for this platform, only shown, in groups by
  // status, worst first.
  await page.getByRole('button', { name: 'Web features' }).click();
  const featureSection = page.locator('.content');
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
  await page.screenshot({ path: path.join(out, 'settings-web-features.png') });

  // Keyboard.
  await page.getByRole('button', { name: 'Keyboard' }).click();
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
  await page.screenshot({ path: path.join(out, 'settings-keyboard-shells.png'), fullPage: true });
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

  await page.getByRole('combobox', { name: 'Template' }).selectOption('chrome');
  const discard = page.getByRole('dialog', { name: /^Discard 1 changed shortcut and use/ });
  check('picking a template with a changed shortcut asks first, saying how many', await until(async () => (await discard.count()) === 1) && keymapFile()?.follows === undefined);
  await page.keyboard.press('Escape');
  check('Escape keeps the shortcuts and the tab', await until(async () => (await discard.count()) === 0) && keymapFile()?.follows === undefined && (await regions()).settings !== 0);
  await page.getByRole('combobox', { name: 'Template' }).selectOption('chrome');
  await discard.getByRole('button', { name: 'Discard' }).click();
  check('Discard resets to following the template', await until(() => keymapFile()?.follows === 'chrome'), JSON.stringify(keymapFile()));
  check('and the added key is gone', await until(async () => !(await page.getByRole('region', { name: 'File', exact: true }).textContent()).includes('⇧⌘K')));
  await page.screenshot({ path: path.join(out, 'settings-keyboard.png'), fullPage: true });

  await page.locator('body').click({ position: { x: 5, y: 300 } });
  await page.keyboard.press('Escape');
  check('Escape closes the open editor, and leaves the tab', await until(async () => (await page.locator('.editor').count()) === 0) && (await regions()).settings !== 0);
  await page.keyboard.press('Escape');
  await sleep(200);
  check('and Escape again leaves it too: the page closes as a tab does, not by itself', (await regions()).settings !== 0);

  // It closes as any tab does: Cmd+W while it is active.
  await ui.getByRole('button', { name: 'Settings', exact: true }).click();
  await until(settingsShown);
  await fetch(`${base}/mock/fire`, { method: 'POST', body: new URLSearchParams({ kind: 'key', value: 'closeTab' }) });
  check('Cmd+W closes the settings tab', await until(async () => (await regions()).settings === 0));
  check('and the dev window lets its page go', await until(async () => (await shell.locator('iframe.our-page').count()) === 0));

  // Site settings: a row per site saying how each of its settings is, and its menu to reset
  // one, or Remove, last, for all of them.
  await ui.getByRole('button', { name: 'Settings', exact: true }).click();
  await until(settingsShown);
  page = await open();
  await page.getByRole('button', { name: 'Site settings', exact: true }).click();
  const sites = page.getByRole('region', { name: 'Site settings' });
  check('Site settings lists each site with how each of its settings is',
    await until(async () => (await sites.textContent()).includes('meet.test') && (await sites.textContent()).includes('Camera allowed, Microphone blocked') && (await sites.textContent()).includes('Notifications blocked')),
    await sites.textContent());
  await settle(page);
  await page.screenshot({ path: path.join(out, 'settings-sites.png') });
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
  await page.screenshot({ path: path.join(out, 'settings-site.png') });
  await siteRadio('Camera', 'Allow').click();
  check('Allow there sets it for the site', await until(() => sitesFile()['https://meet.test']?.camera === 'allow'), JSON.stringify(sitesFile()));
  await siteRadio('Camera', 'Ask').click();
  check('and Ask resets it', await until(() => !('camera' in (sitesFile()['https://meet.test'] || {}))), JSON.stringify(sitesFile()));
  await site.getByRole('button', { name: 'All sites' }).click();
  check('All sites goes back to every site', await until(() => sites.isVisible()));
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
  check('with none left, the list says so', await until(async () => (await sites.textContent()).includes('No site has asked for a permission yet')));
  await page.close();
  await fetch(`${base}/mock/fire`, { method: 'POST', body: new URLSearchParams({ kind: 'key', value: 'closeTab' }) });
  await until(async () => (await regions()).settings === 0);

  check('no errors in the pages', errors.length === 0, errors.join(' | '));
  await browser.close();
  await stopServer();
  releaseServer.close();
  console.log(failed ? `${failed} FAILED` : 'all passed');
  process.exit(failed ? 1 : 0);
})();
