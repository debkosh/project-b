// End-to-end test of the first app on the mock engine, in a real browser.
// Run: node shell/e2e/first-app.cjs   (from the repo root; Playwright is preinstalled)

const crypto = require('crypto');
const fs = require('fs');
const http = require('http');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, fire, out, profile, regions, settle, until } = lib;

// Each start has a port of its own, so the address follows the server.
let base = '';
const startServer = async (extra = []) => {
  await lib.start(['-block=blocked.test', ...extra]);
  base = lib.base;
};
const stopServer = lib.stop;

(async () => {
  await startServer();
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1100, height: 700 }, screen: { width: 1920, height: 1080 } });
  const errors = [];
  lib.watch(page, errors);

  // The page is the OS window: the shell is in a frame of its own, over the mock page's,
  // both at the window's coordinates. Locators reach the shell through its frame, and
  // survive it loading again; keyboard, mouse and screenshots are the window's.
  await page.goto(base + '/');
  const ui = page.frameLocator('iframe.shell');
  const shellFrame = () => page.frame({ url: /\/shell\/standard\/\?window=1$/ });
  // A person's pointer arrives in many moves. The window gives the shell the pointer on
  // the first move into its region, so the shell's hover begins with the next; Playwright's
  // hover is one move, so this makes two.
  const hover = async (locator) => {
    const b = await locator.boundingBox();
    await page.mouse.move(b.x + b.width / 2 - 1, b.y + b.height / 2);
    await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2);
  };
  const bar =ui.getByRole('combobox', { name: 'Address' });
  // The toolbar row moves the window in its gaps and nowhere else: the gap between the
  // address field and the buttons after it drags, the field and a button are shell, and
  // the page is the page's.
  const toolbarDrags = async (layout) => {
    const { gap, field, slot, back: b } = await shellFrame().evaluate(() => {
      const middle = (r) => ({ x: r.x + r.width / 2, y: r.y + r.height / 2 });
      const f = document.querySelector('form.field').getBoundingClientRect();
      const after = document.querySelector('form.field').nextElementSibling.getBoundingClientRect();
      return { gap: { x: (f.right + after.left) / 2, y: f.y + f.height / 2 }, field: middle(f), slot: { x: f.x + 10, y: f.y + f.height / 2 }, back: middle(document.querySelector('[aria-label=Back]').getBoundingClientRect()) };
    });
    const hit = async (at) => (await regions(at)).hit;
    check(`${layout}: the gap between the address field and the buttons drags`, (await hit(gap)) === 'drag', JSON.stringify({ gap, hit: await hit(gap) }));
    check(`${layout}: the address field and the back button are shell`, (await hit(field)) === 'shell' && (await hit(b)) === 'shell', JSON.stringify([await hit(field), await hit(b)]));
    check(`${layout}: the field's slot is shell too, so a drag from it selects rather than moves the window`, (await hit(slot)) === 'shell', await hit(slot));
    check(`${layout}: the page is the page's`, (await hit({ x: 600, y: 400 })) === 'page');
  };
  const list = ui.locator('#suggestions');
  const options = ui.getByRole('option');
  const mock = page.frameLocator('iframe.mock-page');
  // A new tab with nothing in it shows our box in the page area, not a blank page.
  const newTabBox = ui.getByRole('combobox', { name: 'Search tabs, history, actions or the web' });
  await newTabBox.waitFor();
  // Standard starts on its command box with no tab: a page is opened from it.
  const openFromBox = async (text, expected) => {
    await newTabBox.fill(text);
    await newTabBox.press('Enter');
    await page.frameLocator('iframe.mock-page').getByText(expected, { exact: true }).waitFor();
  };
  await settle(page);
  await page.screenshot({ path: path.join(out, '1-empty.png') });

  const back = ui.getByRole('button', { name: 'Back' });
  const forward = ui.getByRole('button', { name: 'Forward' });
  const reload = ui.getByRole('button', { name: 'Reload' });
  check('opens with nothing to go back to or reload', (await back.isDisabled()) && (await forward.isDisabled()) && (await reload.isDisabled()));

  let r = await regions();
  check('opens empty with the top bar and the sidebar as shell', r.regions.some((g) => g.x === 0 && g.y === 0 && g.w === 1100) && (await regions({ x: 100, y: 600 })).hit === 'shell' && (await bar.inputValue()) === '', JSON.stringify(r));
  check('the window loads the standard shell', r.shell === 'standard', r.shell);
  check('opens with the new tab\'s box focused, and the page area the shell\'s', await newTabBox.evaluate((el) => el === document.activeElement) && (await regions({ x: 600, y: 400 })).hit === 'shell');
  check('no update bubble when nothing is happening', (await ui.locator('.update').count()) === 0);

  // Waits for the commit. The URL shows as soon as loading starts, and a new navigation
  // before the commit would correctly replace this one.
  const loaded = async (expected) => {
    await mock.getByText(expected, { exact: true }).waitFor();
    await mock.getByText('Loaded', { exact: true }).waitFor();
    await shellFrame().waitForFunction((v) => document.querySelector('input').value === v, expected);
  };
  const visit = async (text, expected) => {
    await bar.fill(text);
    await bar.press('Enter');
    await loaded(expected);
  };
  // New Tab with no tab keeps the box, and a page opened from the address field instead
  // closes it and shows: the box never sits over a tab it didn't open on.
  await fire('menu', 'tab.new');
  await settle(page);
  await visit('example.com/docs', 'https://example.com/docs');
  check('a page opened from the address field closes the box and shows', await until(async () => (await newTabBox.count()) === 0 && (await regions()).visible.length === 1 && (await regions({ x: 600, y: 400 })).hit === 'page'), JSON.stringify((await regions()).visible));
  r = await regions();
  check('page sits right of the sidebar, below the bar, inside the frame', r.bounds.x === 224 && r.bounds.y === 48 && r.bounds.w === 1100 - 224 - 4 && r.bounds.y + r.bounds.h === 700 - 4, JSON.stringify(r));
  // The dev window places the mock page where the layout puts it, as the real window does.
  const placed = async () => {
    const want = (await regions()).bounds;
    const frame = await page.locator('iframe.mock-page').boundingBox();
    return !!frame && frame.x === want.x && frame.y === want.y && frame.width === want.w && frame.height === want.h;
  };
  check('the dev window puts the page where the layout says', await until(placed), JSON.stringify(await page.locator('iframe.mock-page').boundingBox()));
  const pageBounds = JSON.stringify(r.bounds);
  await visit('https://go.dev/learn', 'https://go.dev/learn');
  await visit('https://example.org/', 'https://example.org/');
  check('address bar shows the final URL', (await bar.inputValue()) === 'https://example.org/');
  check('the progress bar shows the page\'s real load progress', (await ui.locator('.progress').evaluate((el) => el.style.width)) === '100%');
  // Reloaded, the bars are made again once Go's data is in, with that tab active: the bar
  // shows its address, and its page gets the keyboard back.
  await fetch(`${base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'focusTab', id: 0 }) });
  await until(async () => (await regions()).focused === 0);
  await shellFrame().evaluate(() => location.reload());
  check('reloaded on a page, the bar shows its address and the page gets the keyboard',
    await until(async () => (await regions()).focused === (await regions()).active && (await bar.inputValue()) === 'https://example.org/'), String((await regions()).focused));

  // Back, forward and reload.
  check('back is on and forward is off after new pages', (await back.isEnabled()) && (await forward.isDisabled()) && (await reload.isEnabled()));
  await back.click();
  await loaded('https://go.dev/learn');
  check('back returns to the previous page', await forward.isEnabled());
  await forward.click();
  await loaded('https://example.org/');
  check('forward returns to the next page', await forward.isDisabled());
  // Loading lasts a moment on the mock, so the page's states are recorded from before the
  // press, as the contract does, rather than looked for after it.
  await mock.locator('#state').evaluate((shown) => {
    window.states = [];
    new MutationObserver(() => window.states.push(shown.textContent)).observe(shown, { childList: true, characterData: true, subtree: true });
  });
  await reload.click();
  await until(() => mock.locator('#state').evaluate(() => window.states.includes('Loading')));
  await loaded('https://example.org/');
  check('reload loads the page again', (await bar.inputValue()) === 'https://example.org/');
  await back.click();
  await loaded('https://go.dev/learn');
  await visit('https://example.org/', 'https://example.org/');
  await shellFrame().waitForFunction(() => document.querySelector('[aria-label=Forward]').disabled);
  check('a new page drops what was ahead', await forward.isDisabled());

  // Suggestions come from real history, over the page.
  await bar.click();
  await page.keyboard.type('exam');
  await options.first().waitFor();
  await settle(page);
  await page.screenshot({ path: path.join(out, '2-suggestions.png') });
  const history = ui.locator('#suggestions [data-kind=history]');
  const visited = ui.locator('#suggestions [data-kind=history], #suggestions [data-kind=tab]');
  // example.org is open, so the page the text completes is its tab's row, first.
  check('Go\'s list: what Enter does first, the open page the text completes, then the visited pages', (await options.first().getAttribute('data-kind')) === 'tab' && (await visited.count()) === 2, String(await visited.count()));
  check('titles arrive after the visit', /Mock example/.test(await history.first().innerText()));

  r = await regions();
  const box = await list.boundingBox();
  const boxRegion = (r) => r.regions.some((g) => g.x === box.x && g.y === box.y && g.w === box.width && g.h === box.height);
  check('the box becomes a shell rectangle', boxRegion(r), JSON.stringify(r.regions));
  const over = await regions({ x: box.x + 20, y: box.y + 10 });
  check('the box extends over the page', box.y + box.height > r.bounds.y + 40, JSON.stringify({ box, bounds: r.bounds }));
  check('a click on the box goes to the shell', over.hit === 'shell');
  const below = await regions({ x: box.x + 20, y: box.y + box.height + 40 });
  check('a click below the box goes to the page', below.hit === 'page');

  // Bar and list behave as one UI.
  await page.keyboard.press('ArrowDown');
  check('down moves into the list, and the bar shows the row', (await bar.inputValue()) !== 'exam' && (await ui.locator('li.active').count()) === 1, await bar.inputValue());
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowUp');
  await page.keyboard.press('ArrowUp');
  check('up from the first item returns to what was typed', (await bar.inputValue()) === 'exam' && (await ui.locator('li.active').count()) === 0, await bar.inputValue());
  check('focus never left the address bar', await bar.evaluate((el) => el === document.activeElement));
  await page.keyboard.press('Escape');
  await settle(page);
  r = await regions();
  check('escape closes the box and its rectangle goes', (await list.count()) === 0 && !boxRegion(r), JSON.stringify(r.regions));

  // Clicking a suggestion.
  await bar.fill('');
  await page.keyboard.type('go.d');
  // The go.dev row itself: under load the click could land on the list still showing the
  // last query's rows, before go.d's came (2 of 16 runs on 0.2.98).
  await history.filter({ hasText: 'go.dev' }).first().click();
  const went = await shellFrame().waitForFunction(() => document.querySelector('input').value === 'https://go.dev/learn').then(() => true, () => false);
  if (!went) await page.screenshot({ path: path.join(out, 'first-app-suggestion-click.png') });
  check('clicking a suggestion navigates', went && (await list.count()) === 0, went ? '' : `the address is ${await bar.inputValue()}; build/e2e/first-app-suggestion-click.png`);

  // Clicking the page closes the box, and keeps what was typed until Escape.
  await bar.click();
  await page.keyboard.type('exam');
  await options.first().waitFor();
  await page.mouse.click(1050, 650);
  await settle(page);
  check('clicking the page closes the box and keeps what was typed', (await list.count()) === 0 && (await bar.inputValue()) === 'exam', await bar.inputValue());
  await bar.click();
  await page.keyboard.press('Escape');
  await page.keyboard.press('Escape');
  check('Escape then puts the page\'s address back', (await bar.inputValue()) === 'https://go.dev/learn', await bar.inputValue());

  // The window sends each press where its regions say, with the mouse moved there first,
  // as a person's is: beside the suggestions the page takes it, on them the shell.
  const titleButton = await page.locator('iframe.mock-page').contentFrame().getByRole('button', { name: 'Title' }).boundingBox();
  await page.mouse.click(titleButton.x + titleButton.width / 2, titleButton.y + titleButton.height / 2);
  check('a press in the page area reaches the page', await until(async () => (await ui.locator('.row.tab.active').innerText()).includes('A new title')), await ui.locator('.row.tab.active').innerText());
  await bar.click();
  await page.keyboard.type('exam');
  await options.first().waitFor();
  await settle(page);
  const optionBox = await options.last().boundingBox();
  check('the suggestions lie over the page', optionBox.y > (await regions()).bounds.y, JSON.stringify(optionBox));
  await page.mouse.click(optionBox.x + 40, optionBox.y + optionBox.height / 2);
  check('a press on them, over the page, reaches the shell', await until(async () => (await list.count()) === 0 && (await bar.inputValue()).startsWith('https://example')), await bar.inputValue());

  // Events from the page side. Shortcuts arrive from the engine, whichever page had focus.
  const shortcut = async (name) => {
    await mock.getByLabel('Key name').fill(name);
    await mock.getByRole('button', { name: 'Key', exact: true }).click();
  };
  await shortcut('focusAddressBar');
  await shellFrame().waitForFunction(() => document.activeElement === document.querySelector('input'));
  check('the shortcut focuses the address bar from the page', true);
  await page.keyboard.press('Escape');

  await mock.getByRole('button', { name: 'Fail' }).click();
  await ui.getByText('Failed', { exact: true }).waitFor();
  check('a failed navigation shows', true);
  check('and a notice covers its page, with Try again', await until(() => ui.getByRole('alert').getByRole('button', { name: 'Try again' }).isVisible()));
  // The notice covers the page's own buttons, so the page crashes as the engine says it.
  await fetch(`${base}/mock/fire`, { method: 'POST', body: new URLSearchParams({ kind: 'crash' }) });
  await ui.getByText('Crashed', { exact: true }).waitFor();
  check('a crashed page shows', true);
  check('and its notice offers Reload', await until(() => ui.getByRole('alert').getByRole('button', { name: 'Reload' }).isVisible()));

  // Policy.
  await bar.fill('https://www.blocked.test/secret');
  await bar.press('Enter');
  await ui.getByText('Blocked', { exact: true }).waitFor();
  await bar.click();
  await bar.fill('');
  await page.keyboard.type('secret');
  await settle(page);
  check('a blocked URL stays out of history', (await history.count()) === 0);
  await bar.fill('');
  await page.keyboard.type('blank');
  await settle(page);
  check('the blank page stays out of history', (await history.count()) === 0);
  await page.keyboard.press('Escape');
  await page.keyboard.press('Escape');

  // What was typed stays when the bar loses focus, as when the window does, until Escape.
  const address = await bar.inputValue();
  await bar.click();
  await bar.fill('');
  await page.keyboard.type('half typed');
  await bar.evaluate((e) => e.blur());
  await settle(page);
  check('typed text stays when the address bar loses focus', (await bar.inputValue()) === 'half typed', await bar.inputValue());
  await bar.click();
  await page.keyboard.press('Escape');
  await page.keyboard.press('Escape');
  check('and Escape puts the page\'s address back', (await bar.inputValue()) === address, await bar.inputValue());

  // The shell's own page never loads in a tab: a copy there would talk to Go as the
  // shell. Content's own internal pages still load when typed.
  const mockShows = async (state, url) => {
    for (let i = 0; i < 100; i++) {
      if ((await mock.locator('#url').textContent()) === url && (await mock.locator('#state').textContent()) === state) return true;
      await new Promise((r) => setTimeout(r, 30));
    }
    return false;
  };
  await bar.fill('chrome://gpu/');
  await bar.press('Enter');
  check('typing an internal page content serves, such as chrome://gpu/, is let through', await mockShows('Loaded', 'chrome://gpu/'));
  await bar.fill('chrome://shell/');
  await bar.press('Enter');
  check('typing chrome://shell/ is refused by the gate', await mockShows('Blocked', 'chrome://shell/'));
  await bar.fill('https://links.test/');
  await bar.press('Enter');
  await mockShows('Loaded', 'https://links.test/');
  await mock.locator('form[data-kind=navigate] input').fill('chrome://shell/');
  await mock.getByRole('button', { name: 'Navigate' }).click();
  check('a page linking to chrome://shell/ is refused', await mockShows('Blocked', 'chrome://shell/'));
  // A blocked page is covered by its notice; what follows presses the page's own buttons.
  await bar.fill('https://links.test/');
  await bar.press('Enter');
  await mockShows('Loaded', 'https://links.test/');
  await ui.getByRole('button', { name: 'Tabs on top' }).click();
  await ui.locator('.strip').waitFor();
  await ui.getByRole('button', { name: 'Tabs on the side' }).click();
  await ui.locator('nav').waitFor();
  await settle(page);
  check('after those attempts the shell still takes clicks: the layout toggle works', (await ui.locator('.strip').count()) === 0);

  // The engine says where the window controls are; the shell leaves exactly that space.
  // In full screen there are none, so the first control moves into their place.
  const toggle = ui.getByRole('button', { name: /sidebar/ });
  const windowLeft = (await toggle.boundingBox()).x;
  check('the first control starts after the window controls the engine reported', windowLeft === 19 + 60 + 8, String(windowLeft));
  r = await regions({ x: 74, y: 16 });
  check('the empty strip between the window buttons and the controls drags the window', r.drag.length === 1 && r.hit === 'drag', JSON.stringify(r.drag));
  check('the empty part of the sidebar is shell and never drags', (await regions({ x: 100, y: 600 })).hit === 'shell');
  check('the frame\'s left, right and bottom edges never drag', (await regions({ x: 2, y: 300 })).hit !== 'drag' && (await regions({ x: 1098, y: 300 })).hit !== 'drag' && (await regions({ x: 600, y: 698 })).hit !== 'drag');
  check('the control itself is shell, not drag', (await regions({ x: windowLeft + 12, y: 16 })).hit === 'shell');
  await toolbarDrags('tabs on the side');
  // The frame colour has one owner: Go gave it to the window and to the shell. None is
  // picked at first: None's light grey (core/tones).
  const frame = await shellFrame().evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--frame').trim());
  check('the window and the shell paint the frame colour Go owns', frame === '#dcdee2' && (await regions()).windowColor === frame, `${frame} ${(await regions()).windowColor}`);
  check('the window already had the frame colour when it went on screen', (await regions()).shownColor === frame, (await regions()).shownColor);

  // The shell is not a page: nothing scrolls, and a trackpad swipe must not rubber-band it.
  const rootStyle = await shellFrame().evaluate(() => { const s = getComputedStyle(document.documentElement); return [s.overflow, s.overscrollBehaviorY, document.scrollingElement.scrollHeight - window.innerHeight]; });
  check('the shell document neither scrolls nor bounces', rootStyle[0] === 'hidden' && rootStyle[1] === 'none' && rootStyle[2] === 0, JSON.stringify(rootStyle));

  // The engine also says how round the window's corners are; the frame's inner edge
  // follows, one frame width inside.
  const windowRadius = () => shellFrame().evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--window-radius').trim());
  check('the shell hears the window\'s corner radius from the engine', (await windowRadius()) === '26px', await windowRadius());
  await mock.getByRole('button', { name: 'Full screen' }).click();
  await shellFrame().waitForFunction(() => document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x < 20);
  check('in full screen the controls move into the buttons\' place', (await toggle.boundingBox()).x === 4 + 8, String((await toggle.boundingBox()).x));
  check('in full screen the corners are square', (await windowRadius()) === '0px', await windowRadius());
  check('in full screen nothing drags: the empty top row is shell', (await regions({ x: 600, y: 2 })).hit === 'shell' && (await regions()).drag.length === 0, JSON.stringify((await regions()).drag));
  await mock.getByRole('button', { name: 'Full screen' }).click();
  await shellFrame().waitForFunction((x) => document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x === x, windowLeft);
  check('leaving full screen gives the buttons their space back', true);
  check('and the corners their curve', (await windowRadius()) === '26px', await windowRadius());
  check('and the empty top row drags again', (await regions({ x: 600, y: 2 })).hit === 'drag');

  // The window draws its frame band over the shell and the page, and its controls where
  // the engine reports them; full screen has none.
  const band = await page.locator('.band').evaluate((el) => { const s = getComputedStyle(el); return [s.borderTopColor, s.borderLeftWidth, s.borderRadius]; });
  check('the window draws the frame band in the frame colour, the frame width wide', JSON.stringify(band) === JSON.stringify(['rgb(220, 222, 226)', '4px', '26px']), JSON.stringify(band));
  const controls = page.locator('.controls');
  const controlsBox = await controls.boundingBox();
  check('and its controls where the engine reports them', JSON.stringify(controlsBox) === JSON.stringify({ x: 19, y: 19, width: 60, height: 14 }), JSON.stringify(controlsBox));
  await mock.getByRole('button', { name: 'Full screen' }).click();
  check('in full screen the window has no controls', await until(async () => !(await controls.isVisible())));
  await mock.getByRole('button', { name: 'Full screen' }).click();
  check('and out of it they are back', await until(() => controls.isVisible()));

  // The window's size reaches Go as the real window's does, and the shell's layout follows.
  const sized = async (w, h) => {
    const r = await regions();
    return r.window.W === w && r.window.H === h && r.bounds.w === w - 224 - 4 && r.bounds.y + r.bounds.h === h - 4 && (await placed());
  };
  await page.setViewportSize({ width: 900, height: 600 });
  check('resizing the window reaches Go, and the page follows the shell\'s layout', await until(() => sized(900, 600)), JSON.stringify(await regions()));
  await page.setViewportSize({ width: 1100, height: 700 });
  check('and back', await until(() => sized(1100, 700)), JSON.stringify(await regions()));

  // Tabs. Standard's New Tab goes to a new tab; a tab is also made by Go's newTab, as a
  // shell asks for one.
  const makeTab = () => fetch(`${base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'newTab' }) });
  const rows = ui.getByRole('tab');
  const activeRow = ui.locator('.row.tab.active');
  const rowTitles = () => rows.evaluateAll((els) => els.map((el) => el.querySelector('.title').textContent));
  check('one tab to start with, active', (await rows.count()) === 1 && (await activeRow.count()) === 1);
  await visit('https://example.org/', 'https://example.org/');
  const commandBox = ui.locator('.command-box');
  // Standard's New Tab goes to the window's new tab, one with nothing loaded, opening one
  // when there is none, and its command box shows on it (docs/windows.md).
  await ui.getByRole('button', { name: 'New Tab', exact: true }).click();
  check('Standard\'s New Tab opens a new tab with its command box on it, focused', await until(async () => (await rows.count()) === 2 && (await commandBox.count()) === 1) &&
    (await newTabBox.evaluate((el) => el === document.activeElement)), JSON.stringify(await rowTitles()));
  await ui.getByRole('button', { name: 'New Tab', exact: true }).click();
  await settle(page);
  check('New Tab again stays on that new tab, and makes no other', (await rows.count()) === 2, JSON.stringify(await rowTitles()));
  check('a new tab is made and made active', (await activeRow.innerText()).startsWith('New Tab'), await activeRow.innerText());
  check('a new tab shows the command box, focused, over an empty address bar', (await bar.inputValue()) === '' && (await newTabBox.evaluate((el) => el === document.activeElement)));
  // A new tab with nothing in it shows Standard's command box in the page area, and no
  // page: the window shows no tab's page while the box has the area.
  r = await regions();
  check('and reads as a new tab: no address, titled New Tab', (await bar.inputValue()) === '' && (await activeRow.innerText()).startsWith('New Tab'), await activeRow.innerText());
  check('no page shows under the box: the window shows neither tab\'s page', await until(async () => (await regions()).visible.length === 0), JSON.stringify((await regions()).visible));
  check('and the page area is the shell\'s', await until(async () => (await regions({ x: 600, y: 400 })).hit === 'shell'));
  check('back and reload follow the new, empty tab', (await back.isDisabled()) && (await reload.isDisabled()));
  await visit('https://example.net/', 'https://example.net/');
  check('each tab keeps its own title', JSON.stringify(await rowTitles()) === JSON.stringify(['Mock example.org', 'Mock example.net']), JSON.stringify(await rowTitles()));

  // A layout that lists two tabs, as a shell with panes sends, gets a mock page for each,
  // where it puts them, in its order. Each page's buttons act on its own tab, and a press
  // in one gives its tab the keyboard, which makes it active; the shell then lists that
  // tab alone again. The shell's own layout settles first, the command box gone.
  await until(async () => (await ui.locator('.command-box').count()) === 0);
  await settle(page);
  r = await regions();
  const half = { ...r.bounds, w: r.bounds.w / 2 };
  await fetch(`${base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'layout', shell: r.regions, drag: r.drag, viewW: 1100, viewH: 700,
    tabs: [{ tab: 2, rect: half }, { tab: 1, rect: { ...half, x: half.x + half.w } }] }) });
  const frames = page.locator('iframe.mock-page');
  const framesShow = async (want) => JSON.stringify(await frames.evaluateAll((els) => els.filter((el) => el.style.display !== 'none').map((el) => {
    const b = el.getBoundingClientRect();
    return [Number(el.dataset.tab), b.x, b.width];
  }))) === JSON.stringify(want);
  check('a layout with two tabs gets a mock page for each, where it puts them, in its order', await until(() => framesShow([[2, half.x, half.w], [1, half.x + half.w, half.w]])), JSON.stringify(await frames.evaluateAll((els) => els.map((el) => el.dataset.tab))));
  const second = page.frameLocator('iframe.mock-page[data-tab="1"]');
  await second.getByText('tab 1', { exact: true }).waitFor();
  await second.locator('form[data-kind=title] input').fill('Its own title');
  const secondTitle = await second.getByRole('button', { name: 'Title' }).boundingBox();
  await page.mouse.click(secondTitle.x + secondTitle.width / 2, secondTitle.y + secondTitle.height / 2);
  check("a press in the second page reaches it, and its button acts on its own tab", await until(async () => (await rowTitles())[0] === 'Its own title'), JSON.stringify(await rowTitles()));
  check('and gives its tab the keyboard, as the real engine says', await until(async () => (await regions()).focused === 1), String((await regions()).focused));
  check('which makes it active, and the shell lists it alone again', await until(async () => (await rows.first().getAttribute('aria-selected')) === 'true' && (await framesShow([[1, r.bounds.x, r.bounds.w]]))));
  await rows.nth(1).click();
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]')[1].getAttribute('aria-selected') === 'true');
  await rows.first().click();
  await shellFrame().waitForFunction((v) => document.querySelector('input').value === v, 'https://example.org/');
  check('clicking a row switches tabs and the address bar follows', (await rows.first().getAttribute('aria-selected')) === 'true');
  check('back follows the tab that has history', await back.isEnabled());
  await mock.getByText('tab 1', { exact: true }).waitFor();
  check('the page view shows the active tab', true);

  // Favicons come from the page; a tab without one shows none.
  check('a tab with no favicon shows no icon', (await ui.locator('[role=tab] img').count()) === 0);
  await mock.getByRole('button', { name: 'Favicon' }).click();
  await ui.locator('[role=tab][aria-selected=true] img').waitFor();
  check('the favicon the page gives shows on its row only', (await ui.locator('[role=tab] img').count()) === 1);
  // Go's tab list carries no favicons: another tab made active, a list without them, leaves
  // the icon where it was.
  const iconRow = await ui.locator('[role=tab][aria-selected=true]').getAttribute('data-tab');
  await ui.locator('[role=tab][aria-selected=false]').first().click();
  await shellFrame().waitForFunction((id) => document.querySelector(`[role=tab][data-tab="${id}"]`).getAttribute('aria-selected') === 'false', iconRow);
  await settle(page);
  check('a favicon stays when the tab list changes', (await ui.locator(`[role=tab][data-tab="${iconRow}"] img`).count()) === 1);
  await ui.locator(`[role=tab][data-tab="${iconRow}"]`).click();
  await shellFrame().waitForFunction((id) => document.querySelector(`[role=tab][data-tab="${id}"]`).getAttribute('aria-selected') === 'true', iconRow);
  await visit('https://example.org/', 'https://example.org/');
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab] img').length === 0);
  check('a new page drops the old favicon', true);

  // The page opens a tab.
  await mock.getByRole('button', { name: 'Open in new tab' }).click();
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]').length === 3);
  await loaded('https://opened.test/');
  check('a tab the page opens appears and is active', (await activeRow.innerText()).includes('opened.test'), await activeRow.innerText());

  await shortcut('tab1');
  await shellFrame().waitForFunction(() => document.querySelector('[role=tab]').getAttribute('aria-selected') === 'true');
  check('Cmd+1 picks the first tab', true);
  await shortcut('nextTab');
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]')[1].getAttribute('aria-selected') === 'true');
  check('Shift+Cmd+] picks the next tab', true);
  await shortcut('closeTab');
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]').length === 2);
  check('Cmd+W closes the active tab and its right neighbour takes over', (await activeRow.innerText()).includes('opened.test'), await activeRow.innerText());
  await shortcut('newTab');
  check('Cmd+T opens a new tab with the command box on it', await until(async () => (await ui.locator('.command-box').count()) === 1 && (await rows.count()) === 3));
  check('a new tab has an empty address field', (await bar.inputValue()) === '');

  // Close with the button on the row.
  await hover(rows.nth(1));
  await rows.nth(1).getByRole('button', { name: /^Close/ }).click();
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]').length === 2);
  check('the close button on a row closes that tab', !(await rowTitles()).some((t) => t.includes('opened.test')), JSON.stringify(await rowTitles()));

  // The sidebar collapses and the page follows. A page shows, not the command box.
  if ((await bar.inputValue()) === '') {
    await visit('https://example.com/side', 'https://example.com/side');
    await until(async () => (await regions({ x: 600, y: 400 })).hit === 'page');
  }
  await toggle.click();
  await shellFrame().waitForFunction(() => document.querySelector('nav').getBoundingClientRect().width === 0);
  await settle(page);
  r = await regions();
  check('collapsing the sidebar gives its width to the page', r.bounds.x === 4 && r.bounds.w === 1100 - 8, JSON.stringify(r.bounds));
  await page.screenshot({ path: path.join(out, '4-collapsed.png') });
  await toggle.click();
  await shellFrame().waitForFunction(() => document.querySelector('nav').getBoundingClientRect().width === 220);
  await settle(page);
  r = await regions();
  check('expanding it takes the width back', r.bounds.x === 224, JSON.stringify(r.bounds));
  await page.screenshot({ path: path.join(out, '5-tabs.png') });

  // The same tabs on top. One switch, kept by the managers, drives both layouts.
  const strip = ui.locator('.strip');
  await ui.getByRole('button', { name: 'Tabs on top' }).click();
  await strip.waitFor();
  await settle(page);
  check('switching puts the same tabs in a strip on top', (await rows.count()) === 2 && (await activeRow.count()) === 0 && (await ui.locator('.strip [aria-selected=true]').count()) === 1);
  check('the strip leaves the window controls their space', (await rows.first().boundingBox()).x === 19 + 60 + 8, String((await rows.first().boundingBox()).x));
  const look = await shellFrame().evaluate(() => { const s = (q) => getComputedStyle(document.querySelector(q)); return [s('.strip .tab.active').backgroundColor, s('.strip').color, s('.strip .tab.active').borderRadius, getComputedStyle(document.documentElement).getPropertyValue('--accent-soft').trim()]; });
  check('the active tab is a flat row in the selected colour, as Settings marks its section', look[2] === '6px' && look[0] !== 'rgba(0, 0, 0, 0)', JSON.stringify(look));
  r = await regions();
  check('without a sidebar the page spans the width below two rows', r.bounds.x === 4 && r.bounds.y === 92 && r.bounds.w === 1100 - 8, JSON.stringify(r.bounds));
  check('and the dev window moves the page there', await until(placed), JSON.stringify(await page.locator('iframe.mock-page').boundingBox()));
  check('the empty strip drags the window', (await regions({ x: 900, y: 16 })).hit === 'drag');
  check('the bands beside the toolbar under the strip drag too, as the row they are in', (await regions({ x: 2, y: 60 })).hit === 'drag');
  // The active tab has a page, so the page area is the page's, not the command box's.
  if ((await bar.inputValue()) === '') {
    await visit('https://example.com/top', 'https://example.com/top');
    await until(async () => (await regions({ x: 600, y: 400 })).hit === 'page');
  }
  await toolbarDrags('tabs on top');
  const firstTab = await rows.first().boundingBox();
  check('a tab in the strip is shell', (await regions({ x: firstTab.x + firstTab.width / 2, y: firstTab.y + firstTab.height / 2 })).hit === 'shell');
  // Where the next tab goes is the strip's empty end until it comes; the kit sees it come
  // and measures it, with nothing from the shell.
  const newTab = await ui.locator('.strip .new-tab').boundingBox();
  const next = { x: newTab.x + 100, y: newTab.y + newTab.height / 2 };
  check('the strip\'s empty end drags', (await regions(next)).hit === 'drag');
  // With nothing else changing at all, a button put into the strip's empty end by
  // hand is shell, and the end drags again once it is gone.
  const end = await shellFrame().evaluate(() => {
    const strip = document.querySelector('.strip');
    const b = document.createElement('button');
    b.id = 'by-hand';
    b.style.cssText = 'position: absolute; top: 10px; right: 120px; width: 40px; height: 28px';
    strip.append(b);
    const r = b.getBoundingClientRect();
    return { x: r.x + 20, y: r.y + 14 };
  });
  check('a control that comes into a drag bar is shell, measured by the kit alone', await until(async () => (await regions(end)).hit === 'shell'), (await regions(end)).hit);
  await shellFrame().evaluate(() => document.getElementById('by-hand').remove());
  check('and where it went drags again', await until(async () => (await regions(end)).hit === 'drag'), (await regions(end)).hit);
  await makeTab();
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]').length === 3);
  check('New Tab works from the strip', (await bar.inputValue()) === '');
  const added = await rows.nth(2).boundingBox();
  check('the tab that came is shell where the empty end was', added.x < next.x && next.x < added.x + added.width && await until(async () => (await regions(next)).hit === 'shell'), JSON.stringify({ added, next, hit: (await regions(next)).hit }));
  await rows.first().click();
  await shellFrame().waitForFunction(() => document.querySelector('[role=tab]').getAttribute('aria-selected') === 'true');
  check('clicking a strip tab activates it', true);
  await hover(rows.nth(2));
  await rows.nth(2).getByRole('button', { name: /^Close/ }).click();
  await shellFrame().waitForFunction(() => document.querySelectorAll('[role=tab]').length === 2);
  check('the close button on a strip tab closes it', true);
  await page.screenshot({ path: path.join(out, '6-horizontal.png') });
  await ui.getByRole('button', { name: 'Tabs on the side' }).click();
  await ui.locator('nav').waitFor();
  await settle(page);
  check('switching back brings the sidebar and the page follows', (await rows.count()) === 2 && (await regions()).bounds.x === 224);
  await ui.getByRole('button', { name: 'Tabs on top' }).click();
  await strip.waitFor();

  // A new tab shows Standard's command box where the page goes, as its own, with the
  // keyboard. Light and dark, from the system's setting, which the shell follows by
  // default, with the tabs on top and on the side.
  await shortcut('newTab');
  const tabsThen = await until(async () => (await rows.count()) === 3) && 3;
  const empty = ui.locator('.command-box');
  const emptyShots = async (name) => {
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await settle(page);
      await page.screenshot({ path: path.join(out, `standard-empty-${name}-${scheme}.png`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
  };
  // area is where the page goes, in each layout; name is the screenshots'.
  const emptyState = async (layout, name, area) => {
    await empty.waitFor();
    check(`${layout}: the page area shows the command box`, await newTabBox.isVisible());
    const b = await empty.boundingBox();
    check(`${layout}: it fills the page area, as the shell's: a press there reaches the shell`,
      b.x === area.x && b.y === area.y && b.width === area.w && b.height === area.h && await until(async () => (await regions({ x: b.x + b.width / 2, y: b.y + 40 })).hit === 'shell'),
      JSON.stringify({ empty: b, area }));
    check(`${layout}: no page shows`, (await regions()).visible.length === 0, JSON.stringify((await regions()).visible));
    check(`${layout}: the address is empty, and back, forward and reload are off`,
      (await bar.inputValue()) === '' && (await back.isDisabled()) && (await forward.isDisabled()) && (await reload.isDisabled()));
    await emptyShots(name);
  };
  check('the command box has the keyboard on the new tab', !!tabsThen && await until(() => newTabBox.evaluate((el) => el === document.activeElement)));
  await emptyState('tabs on top', 'top', { x: 4, y: 92, w: 1100 - 8, h: 700 - 92 - 4 });
  await ui.getByRole('button', { name: 'Tabs on the side' }).click();
  await ui.locator('nav').waitFor();
  await settle(page);
  await emptyState('tabs on the side', 'side', { x: 224, y: 48, w: 1100 - 224 - 4, h: 700 - 48 - 4 });
  await ui.getByRole('button', { name: 'Tabs on top' }).click();
  await strip.waitFor();

  // An address and Enter in the command box load it in the new tab.
  await newTabBox.fill('empty.test/box');
  await newTabBox.press('Enter');
  await loaded('https://empty.test/box');
  check('an address and Enter in the command box load it in the new tab, and the box goes',
    (await rows.count()) === 3 && (await regions()).visible.length === 1 && (await empty.count()) === 0);

  // A window always has a tab: Cmd+W on its last tab closes it.
  for (let n = 3; n > 1; n--) {
    await shortcut('closeTab');
    await shellFrame().waitForFunction((n) => document.querySelectorAll('[role=tab]').length === n - 1, n);
  }
  check('closing tabs keeps the window while it has one', !(await regions()).closed);
  await shortcut('closeTab');
  check('Cmd+W on the last tab closes the window', await until(async () => (await regions()).closed === true));

  // History survives a restart.
  await stopServer();
  await startServer();
  await page.goto(base + '/');
  errors.length = 0; // Stopping the server breaks the event stream, as it should.
  // Standard starts on a new tab, with its command box.
  await newTabBox.waitFor();
  await bar.click();
  await page.keyboard.type('exam');
  // The address field's own list: the command box under it lists the new tab's rows at once.
  check('history survives a restart', await until(async () => (await history.count()) >= 3), String(await history.count()));
  check('titles survive a restart', /Mock example/.test(await history.first().innerText()));
  await page.keyboard.press('Escape');
  check('the tab layout survives a restart', (await ui.locator('.strip').count()) === 1);
  await ui.getByRole('button', { name: 'Tabs on the side' }).click();
  await ui.locator('nav').waitFor();
  check('and can be switched back', (await ui.locator('.strip').count()) === 0);

  // Phone width.
  const phone = await browser.newPage({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2 });
  await phone.goto(base + '/');
  const phoneUi = phone.frameLocator('iframe.shell');
  const phoneBar = phoneUi.getByRole('combobox', { name: 'Address' });
  await phoneUi.getByRole('combobox', { name: 'Search tabs, history, actions or the web' }).waitFor();
  await phoneBar.click();
  await phone.keyboard.type('exam');
  await phoneUi.getByRole('option').first().waitFor();
  await settle(phone);
  await phone.screenshot({ path: path.join(out, '3-phone.png') });
  const scroll = await phone.frame({ url: /\/shell\/standard\/\?window=1$/ }).evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);
  check('no horizontal scroll on a phone', scroll);
  // The mock engine keeps one layout, the last any page sent. Left open, this page sends its
  // own again when the server restarts below, and now and then it lands after the other's.
  await phone.close();

  // Updates. The real manager runs against a release server here: a manifest and a zip of a
  // bundle, as tools/release.sh publishes, served slowly enough to watch the download.
  const releases = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-releases-'));
  const fakeApp = (dir, version) => {
    const app = path.join(dir, 'Project B.app');
    fs.mkdirSync(path.join(app, 'Contents/MacOS'), { recursive: true });
    fs.writeFileSync(path.join(app, 'Contents/Info.plist'), `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
  <key>CFBundleExecutable</key><string>project_b</string>
  <key>CFBundleShortVersionString</key><string>${version}</string>
</dict></plist>
`);
    fs.writeFileSync(path.join(app, 'Contents/MacOS/project_b'), `#!/bin/sh\necho ${version} > "$(dirname "$0")/ran"\n`, { mode: 0o755 });
    return app;
  };
  const installed = path.join(releases, 'installed');
  const version = (app) => /CFBundleShortVersionString<\/key><string>([^<]+)/.exec(fs.readFileSync(path.join(app, 'Contents/Info.plist'), 'utf8'))[1];
  const zip = path.join(releases, 'Project-B-0.2.0-darwin-arm64.zip');
  execFileSync('ditto', ['-c', '-k', '--keepParent', fakeApp(path.join(releases, 'new'), '0.2.0'), zip]);
  const good = fs.readFileSync(zip);
  let served = good;
  // On a port of its own, read once it listens. The first download stops half-way and stays
  // so, as one does across the Mac's sleep, until the test ends.
  let releasePort = 0;
  let assets = 0;
  const held = [];
  const releaseServer = http.createServer((req, res) => {
    if (req.url === '/releases.json') {
      res.end(JSON.stringify({ spec: 1, project: 'project-b', published: '2026-11-09T00:00:00Z', channels: { stable: '0.2.0' },
        releases: { '0.2.0': { assets: { 'darwin/arm64': { url: `http://127.0.0.1:${releasePort}/asset.zip`, sha256: crypto.createHash('sha256').update(good).digest('hex'), size: good.length } } } } }));
    } else {
      // Half at once and the rest later, so the download is seen part-way.
      const body = served;
      res.writeHead(200, { 'Content-Length': body.length });
      res.write(body.subarray(0, body.length >> 1));
      if (assets++ === 0) held.push(res);
      else setTimeout(() => res.end(body.subarray(body.length >> 1)), 1500);
    }
  });
  await new Promise((r) => releaseServer.listen(0, '127.0.0.1', r));
  releasePort = releaseServer.address().port;
  const updateArgs = () => ['-version=0.1.0', `-releases=http://127.0.0.1:${releasePort}/releases.json`, `-app=${path.join(installed, 'Project B.app')}`];
  const bubble = ui.locator('.update');

  fs.rmSync(installed, { recursive: true, force: true });
  fakeApp(installed, '0.1.0');
  await stopServer();
  await startServer(updateArgs());
  await page.goto(base + '/');
  errors.length = 0;
  await openFromBox('example.com/update', 'https://example.com/update');
  // Stuck half-way: Retry and Cancel are there while it runs, and Retry fetches it again.
  await ui.getByText(/Downloading Project B 0\.2\.0… [1-9]\d?%/).waitFor({ timeout: 10000 });
  check('a download shows Retry and Cancel while it runs',
    (await bubble.getByRole('button', { name: 'Retry' }).isVisible()) && (await bubble.getByRole('button', { name: 'Cancel' }).isVisible()));
  await settle(page);
  await page.screenshot({ path: path.join(out, '7-update-stuck.png') });
  await bubble.getByRole('button', { name: 'Retry' }).click();
  check('Retry fetches the release again', await until(() => assets === 2), String(assets));
  await ui.getByText(/Downloading Project B 0\.2\.0… [1-9]\d?%/).waitFor({ timeout: 10000 });
  await settle(page);
  const progress = Number(await ui.getByRole('progressbar', { name: 'Update download' }).getAttribute('aria-valuenow'));
  check('the bubble shows how much of the release has arrived', progress > 0 && progress < 100, String(progress));
  await page.screenshot({ path: path.join(out, '7-update-downloading.png') });
  r = await regions();
  let bb = await bubble.boundingBox();
  check('the download shows as a bubble in the page\'s bottom-left corner', bb.x === r.bounds.x && bb.y + bb.height === r.bounds.y + r.bounds.h, JSON.stringify({ bb, bounds: r.bounds }));
  check('the bubble takes no layout space', JSON.stringify(r.bounds) === pageBounds, JSON.stringify(r.bounds));
  check('the bubble is shell, so its click does not reach the page', r.regions.some((g) => g.x === bb.x && g.y === bb.y) && (await regions({ x: bb.x + 10, y: bb.y + 10 })).hit === 'shell', JSON.stringify(r.regions));
  check('just above the bubble is still the page', (await regions({ x: bb.x + 10, y: bb.y - 10 })).hit === 'page');
  const relaunch = ui.getByRole('button', { name: /Relaunch to update/ });
  await relaunch.waitFor({ timeout: 10000 });
  await settle(page);
  await page.screenshot({ path: path.join(out, '8-update-ready.png') });
  check('the bubble offers relaunch once the release is checked and staged', /0\.2\.0 is ready/.test(await bubble.innerText()), await bubble.innerText());
  check('the installed app is untouched until relaunch', version(path.join(installed, 'Project B.app')) === '0.1.0');
  await ui.getByRole('button', { name: 'Tabs on top' }).click();
  await strip.waitFor();
  await settle(page);
  r = await regions();
  bb = await bubble.boundingBox();
  check('with the tabs on top the bubble follows the page\'s corner', bb.x === r.bounds.x && bb.x === 4 && bb.y + bb.height === r.bounds.y + r.bounds.h, JSON.stringify({ bb, bounds: r.bounds }));
  await page.screenshot({ path: path.join(out, '9-update-ready-horizontal.png') });
  await ui.getByRole('button', { name: 'Tabs on the side' }).click();
  await ui.locator('nav').waitFor();
  await relaunch.click();
  await until(async () => (await regions()).closed === true);
  check('relaunch puts the new release in place and closes the window', version(path.join(installed, 'Project B.app')) === '0.2.0');
  check('the running release is kept as the previous one', version(path.join(profile, 'updates/previous/Project B.app')) === '0.1.0');
  await stopServer();
  const ran = path.join(installed, 'Project B.app/Contents/MacOS/ran');
  // The fake release's shell makes the file before it writes the version into it, so wait for
  // what it says, not for the file; and the helper polls the old process every 0.2 s, which a
  // loaded machine can stretch.
  const said = () => (fs.existsSync(ran) ? fs.readFileSync(ran, 'utf8').trim() : '');
  for (let i = 0; i < 200 && said() !== '0.2.0'; i++) await new Promise((r) => setTimeout(r, 50));
  check('the new release starts once the old process has exited', said() === '0.2.0', said());

  // A corrupted download is refused and the installed release keeps working.
  served = Buffer.from(good);
  served[Math.floor(served.length / 2)] ^= 0xff;
  fs.rmSync(installed, { recursive: true, force: true });
  fakeApp(installed, '0.1.0');
  await startServer(updateArgs());
  await page.goto(base + '/');
  errors.length = 0;
  await ui.getByText(/Update to 0\.2\.0 failed/).waitFor({ timeout: 10000 });
  await settle(page);
  await page.screenshot({ path: path.join(out, '10-update-failed.png') });
  check('a corrupted download is refused and the bubble says so', /sha256/.test(await bubble.innerText()), await bubble.innerText());
  check('the installed app is left alone', version(path.join(installed, 'Project B.app')) === '0.1.0' && !fs.existsSync(path.join(profile, 'updates/stage')));
  await bar.fill('https://example.org/');
  await bar.press('Enter');
  await loaded('https://example.org/');
  check('the running release keeps working', (await bar.inputValue()) === 'https://example.org/');
  held.forEach((res) => res.destroy());
  releaseServer.close();

  // With one workspace, Main, every tab is in it, and none shows a dot.
  check('with one workspace no tab shows a dot', (await ui.locator('[role=tab]').count()) > 0 && (await ui.locator('[role=tab] .workspace').count()) === 0);

  // With two, a tab shows a dot in its workspace's colour, in both layouts: one in Work,
  // and the first, in Main, which Go puts above a list from before it. The tab is opened as
  // Tab, New Tab in Workspace does, and New Tab from it opens in it too.
  await stopServer();
  fs.writeFileSync(path.join(profile, 'workspaces.json'), JSON.stringify({ workspaces: [{ id: '0a1b2c3d', name: 'Work', color: '#5b7fd6' }] }));
  await startServer();
  await page.goto(base + '/');
  errors.length = 0;
  await openFromBox('example.com/work', 'https://example.com/work');
  await ui.locator('nav [role=tab]').first().waitFor();
  await fetch(`${base}/mock/fire`, { method: 'POST', body: new URLSearchParams({ kind: 'menu', value: 'workspaces:0a1b2c3d' }) });
  const sideRows = ui.locator('nav [role=tab]');
  await until(async () => (await sideRows.count()) === 2);
  const dotColor = (row) => row.locator('.workspace').evaluate((e) => getComputedStyle(e).backgroundColor);
  check('a tab in a workspace shows a dot in its colour, with the tabs on the side',
    (await sideRows.nth(1).locator('.workspace').count()) === 1 && (await dotColor(sideRows.nth(1))) === 'rgb(91, 127, 214)' &&
      (await sideRows.nth(1).locator('.workspace').getAttribute('aria-label')) === 'In Work');
  check('and a tab in Main shows Main\'s', (await dotColor(sideRows.first())) === 'rgb(52, 112, 208)' && (await sideRows.first().locator('.workspace').getAttribute('aria-label')) === 'In Main');
  await settle(page);
  await page.screenshot({ path: path.join(out, '11-workspace-dot-side.png') });
  await ui.getByRole('button', { name: 'Tabs on top' }).click();
  await ui.locator('.strip').waitFor();
  const topRows = ui.locator('.strip [role=tab]');
  check('and with the tabs on top', (await topRows.nth(1).locator('.workspace').count()) === 1 && (await dotColor(topRows.nth(1))) === 'rgb(91, 127, 214)');
  check('where Main\'s tab shows Main\'s', (await dotColor(topRows.first())) === 'rgb(52, 112, 208)');
  await settle(page);
  await page.screenshot({ path: path.join(out, '12-workspace-dot-top.png') });
  await ui.getByRole('button', { name: 'Tabs on the side' }).click();
  await ui.locator('nav').waitFor();
  await makeTab();
  check('New Tab from a tab in a workspace opens in it',
    await until(async () => (await sideRows.count()) === 3 && (await sideRows.nth(2).locator('.workspace').getAttribute('aria-label')) === 'In Work'));

  check('no page errors', errors.length === 0, errors.join(' | '));
  await browser.close();
  fs.rmSync(releases, { recursive: true, force: true });
  await lib.done();
})();
