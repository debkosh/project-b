// The check every shell passes: "What every shell does" in docs/shells.md, rule by rule, on
// the mock engine in a real browser, with the Extensions flag on. It knows nothing of a
// shell but what the rules say: the tabs are role=tab with data-tab, the basic controls
// are found by their role and name, pages show where the layout puts them, and Go's state
// comes through the mock (/mock/regions, /mock/fire).
// Run: node shell/e2e/shell-contract.cjs [id]   (from the repo root, after `npm run build`
// in shell/; with no id, every shell in core/shells/registry.json)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { regions, fire, settle, until } = lib;

const registry = JSON.parse(fs.readFileSync(path.join(lib.root, 'core/shells/registry.json'), 'utf8'));
// A native shell is no page, so the mock can't load it: tools/drive_android.py checks it.
const ids = process.argv[2] ? [process.argv[2]] : registry.filter((s) => !s.native).map((s) => s.id);

const rules = {
  1: 'It says ready, draws the snapshot, and says shown; the window shows its pages only after the frame with the layout that places them',
  2: "The active tab's page shows, after every change, and no page is given the keyboard before it shows",
  3: "The pages it shows lie inside the band it asked for, don't overlap, and are all tabs of this window",
  4: 'Each tab is an element with role=tab and data-tab, the active one aria-selected; a click makes it active, on the shell or in a sheet one of its controls opens',
  5: 'focusAddressBar puts the keyboard in the field named Address, where an address and Enter take the active tab there, while a web tab is active',
  6: 'Every control it draws is the shell\'s: a press on it reaches the shell',
  7: 'A press in the middle of a page it shows reaches that page',
  8: 'Some space is marked drag, and no page lies under the native window buttons',
  9: 'Reloaded, it comes back showing the same pages',
  10: 'Switched to and away from, it starts from Go\'s data alone',
  11: 'Settings is one press away, or two: a control named Settings, on the shell or in a menu one of its controls opens, opens the settings tab or shows it',
  12: 'The basic controls are somewhere in the shell, found by role and name, and each does its job',
  13: 'It logs no errors',
  14: 'Before Go\'s data comes it draws a splash of its own, marked aria-busy="true", with none of its controls; when the data comes the splash goes and the UI shows',
  15: 'A page\'s alert, confirm and prompt show as an alertdialog with Go\'s buttons by name, a prompt with a field; the answer reaches the page; another tab shown hides it, and back shows it',
  16: 'On a desktop, a right click in a page opens a menu of Go\'s actions there; choosing one makes Go do it; Escape, or a press outside, closes it',
  17: 'A control named Downloads shows once there is a download, and opens a list of them by file name, in the shell or in the Downloads page, a finished one with Show in Finder (on a phone, Show in Downloads)',
  18: 'A new tab has nothing loaded: New Tab shows one, active, and the shell draws its new tab where the page goes, as its own; closing the last tab closes the window',
  19: 'For one of our tabs, it shows the page\'s title where the address goes, with a way to close the tab, and no field to type in',
  20: 'The find key (on a phone, the menu) opens a text box named Find in page with the keyboard in it; typing searches and shows the count; Enter shows the next match; Escape (a phone\'s back key) closes it',
  21: 'A page in full screen covers the whole window with nothing of the shell on it, and when it ends (Escape, or a phone\'s back key) the window is as it was',
  22: 'A site\'s sign-in shows as an alertdialog with text boxes named Username and Password, and Go\'s Sign in answers with both',
  30: 'A site asking for its camera, microphone or notifications in the active tab brings a dialog named by its question, with Block and Allow, or Block, Always allow and Allow this time; an Allow grants it, Block denies it, Escape dismisses it',
  31: 'On a web page, a control named Site settings opens settings on that site, at once or from its panel\'s Details',
  33: 'The Site settings control opens a panel over the page, a dialog named Site settings for the site\'s host, with a switch named Ad blocker, a switch named Sparks with the names of the sparks that run on the page, and a choice of Links to other sites, Allow or Block, each Go\'s and each changing Go\'s setting; a change that applies when the page loads again says so, with a Reload button; Escape closes it; a popup Links to other sites closed shows in the address field as a button named by its count, which opens the panel',
  32: 'A page that failed to load, or has a bad certificate, is covered by an alert with Go\'s sentence and its actions as buttons; Try again loads it again, and Open anyway opens it',
};

// The desktop shells that don't draw the right-click menu yet, and who brings it to each
// (docs/daily-driver.md). Rule 16 holds every other desktop shell to it.
const noMenuYet = {
};

// The shells that don't draw a page's dialogs yet, and who brings each. Rule 15 holds
// every other shell to them.
const noDialogsYet = {
};

// The desktop shells that don't lay a page in full screen across the window yet, and who
// brings each. Rule 21 holds every other desktop shell to it.
const noPageFullYet = {
};

// The desktop shells that don't draw a find bar yet, and who brings each. Rule 20 holds
// every other desktop shell to it.
const noFindYet = {
};

// The desktop shells that don't draw the downloads list yet, and who brings it to each.
// Rule 17 holds every other desktop shell to it.
const noDownloadsYet = {
};

// The shells that don't draw a splash yet, and who brings it to each. Rule 14 holds every
// other shell to it.
const noSplashYet = {
  mobile: 'r3 brings it',
};

// The shells that don't draw our tabs' title yet, and who brings it to each: their
// owners. Rule 19 holds every other shell to it.
const noTitleYet = {
  glide: 'r6 brings it',
  'mobile-native': 'r8, the native mobile shell\'s session, brings it',
};

// The shells that don't draw the permission prompt and the Site settings control yet, and
// who brings them to each (docs/daily-driver.md). Rules 30 and 31 hold every other shell to
// them.
const noPromptYet = {
  glide: 'the daily driver session brings it',
  mobile: 'the android session brings it, as a sheet',
  'mobile-native': 'the native mobile shell session brings it',
};

// The shells whose Site settings control goes straight to settings, without the site panel,
// and why (docs/site-features.md). Rule 33 holds every other shell to the panel.
const noPanelYet = {
  minimal: 'goes straight to settings, as a minimal shell',
  terminal: 'the terminal session brings it',
  glide: 'the daily driver session brings it',
  'mobile-native': 'the native mobile shell is on hold',
};

// The shells that don't draw Go's notice over a page yet, and who brings it to each
// (docs/daily-driver.md). Rule 32 holds every other shell to it.
const noNoticeYet = {
  glide: 'the daily driver session brings it',
  'mobile-native': 'it draws a notice of its own; the native mobile shell session moves it to Go\'s',
};

(async () => {
  // What a control is, as the SDK has it: the same selector it finds them in a drag bar by.
  const { controls: control } = await import('../src/sdk/controls.js');
  const browser = await chromium.launch();
  const summary = [];
  // The extensions' buttons show only while their flag is on, as the settings page saves it.
  fs.writeFileSync(path.join(lib.profile, 'flags.json'), JSON.stringify({ extensions: 'on' }));
  // A spark that runs on the contract's pages, for the panel's Sparks switch (rule 33).
  fs.mkdirSync(path.join(lib.profile, 'sparks'), { recursive: true });
  fs.writeFileSync(path.join(lib.profile, 'sparks', '0000c0de.user.js'), '// ==UserScript==\n// @name Contract spark\n// @match https://contract.test/*\n// ==/UserScript==\n');
  fs.writeFileSync(path.join(lib.profile, 'sparks', 'sparks.json'), JSON.stringify([{ id: '0000c0de', on: true }]));
  for (const id of ids) {
    if (!registry.some((s) => s.id === id)) throw new Error(`no shell ${id} in core/shells/registry.json`);
    console.log(`\n=== the ${id} shell`);
    // Each shell meets the sites as a new profile does: what the shell before it allowed
    // or blocked (rules 30 and 31) would answer its questions with no prompt.
    fs.rmSync(path.join(lib.profile, 'sites.json'), { force: true });
    const broken = new Set();
    // rule records one check of rule n.
    const rule = (n, name, ok, detail) => {
      if (!lib.check(`${n}. ${name}`, ok, detail)) broken.add(n);
      return ok;
    };
    await lib.start([`-use=${id}`]);
    // A phone's shell runs on a touch screen, which our pages read too (shell/src/phone.js).
    const w = await lib.openWindow(browser, { held: true, hasTouch: !!registry.find((s) => s.id === id).phone });
    const { page, ui } = w;
    page.setDefaultTimeout(5000);
    const tabEls = ui.locator('[role=tab]');
    // A tab made as a shell asks for one, by its newTab message. New Tab the action may be
    // the shell's own to run, which by itself makes no tab.
    // Each tab the check opens holds a page of its own, by newTab with an address: a shell
    // may draw its own view over a tab with no address, as a shell that takes New Tab
    // does, and rule 2 is about pages.
    let opened = 0;
    const openTab = () => fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'newTab', text: `https://contract.test/tab-${++opened}` }) });
    // Whether the keyboard is in a field of the shell's to type in: its focused element is
    // a text field, a text area, editable, or has a text box's role.
    const inTextField = () => w.shell().evaluate(() => {
      const el = document.activeElement;
      if (!el || el === document.body) return false;
      const role = el.getAttribute('role');
      return (el.tagName === 'INPUT' && ['text', 'search', 'url', ''].includes(el.getAttribute('type') || '')) ||
        el.tagName === 'TEXTAREA' || el.isContentEditable || ['textbox', 'combobox', 'searchbox'].includes(role);
    });

    // The window as Go and the mock have it now, and the tabs as the shell draws them.
    const drawn = () => tabEls.evaluateAll((els) => els.map((el) => ({ tab: Number(el.dataset.tab), selected: el.getAttribute('aria-selected') })));
    const shows = (r) => r.visible.some((v) => v.tab === r.active);
    // A new tab, the active one with nothing loaded, has no page: the shell draws it.
    const fresh = (r) => {
      const t = r.tabs.find((t) => t.id === r.active);
      return !!t && !t.url && !t.loading && !t.page;
    };
    // The shell is up when it loads this shell, draws a tab for each of Go's, and shows the
    // active one: its page, or for a new tab the shell's own view.
    const up = async () => {
      const r = await regions();
      return r.shell === id && (await tabEls.count()) === r.tabs.length && (shows(r) || fresh(r));
    };

    // 14. Before Go's data, a splash with none of the shell's controls; then the UI. The
    // window holds the shell's ready, so Go's snapshot doesn't come until it lets it go.
    if (!(await until(w.waiting, 10000))) {
      rule(14, 'not checked: the shell never said ready', false);
    } else if (noSplashYet[id]) {
      rule(14, `not checked yet: ${noSplashYet[id]}`, true);
      await w.release();
    } else {
      // Rule 12's controls, by role and name, wherever they show on the shell.
      const name = /^(Back|Forward|Reload|Stop|New Tab|Close|Settings|Extensions)\b/;
      const basics = ui.getByRole('button', { name }).or(ui.getByRole('menuitem', { name }))
        .or(ui.getByRole('textbox', { name: 'Address', exact: true })).or(ui.getByRole('combobox', { name: 'Address', exact: true }));
      const busy = ui.locator('[aria-busy="true"]');
      const html = (locator) => locator.evaluateAll((els) => els.map((el) => el.outerHTML.slice(0, 80)).join(' | '));
      rule(14, 'before Go\'s data comes it draws a splash, marked aria-busy="true"', await until(async () => (await busy.filter({ visible: true }).count()) > 0));
      rule(14, 'with none of its controls', (await basics.count()) === 0, await html(basics));
      await w.release();
      // The splash and the UI cross-fade; once the transitions are over the splash is gone.
      const over = () => w.shell().evaluate(() => !document.getAnimations().some((a) => a instanceof CSSTransition));
      await until(async () => (await up()) && (await over()), 10000);
      rule(14, 'when the data comes, once the transition is over, no aria-busy="true" is left', (await busy.count()) === 0, await html(busy));
      rule(14, 'and its controls are there', (await basics.count()) > 0);
    }

    // 1. ready, the snapshot drawn, shown.
    let ready = await until(up, 10000);
    rule(1, 'it draws the snapshot: a tab for each of Go\'s', ready, JSON.stringify({ drawn: await tabEls.count(), go: (await regions()).tabs.length }));
    rule(1, 'and says shown, so Go marks the app ready', await until(() => /\bapp\.ready\b/.test(lib.log)));
    // The window shows its pages after the shell's next frame once the layout that places
    // them comes, so the shell sends that layout only once it has drawn what it measured.
    // The window starts on a new tab, which has no page to show; the rules after this one
    // start from a page, so the check opens one, as a shell does, by its newTab, and closes
    // the new tab, and they hold the shell to the rest as ever.
    if (fresh(await regions())) {
      rule(1, 'the window shows its pages only after the shell has drawn the layout that places them: it starts on a new tab, with no page to show', true);
      const first = (await regions()).active;
      await openTab();
      await until(async () => (await regions()).tabs.length === 2 && (await up()));
      await fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'closeTab', id: first }) });
      ready = ready && (await until(async () => (await regions()).tabs.length === 1 && (await up())));
    } else {
      await until(() => page.evaluate(() => window.pagesShownWith !== undefined));
      const drawnThen = await page.evaluate(() => window.pagesShownWith);
      const go = (await regions()).tabs.length;
      rule(1, 'the window shows its pages only after the shell has drawn the layout that places them: a tab for each of Go\'s',
        drawnThen === go, JSON.stringify({ drawnThen, go }));
    }

    // 3, for the layout as it is now.
    const insideAndApart = async (when) => {
      const r = await regions();
      const f = r.frameWidth;
      const W = r.window.W;
      const H = r.window.H;
      const outside = r.visible.filter(({ rect: p }) => p.x < f || p.y < f || p.x + p.w > W - f || p.y + p.h > H - f);
      const overlap = r.visible.some((a, i) => r.visible.some((b, j) => j > i &&
        a.rect.x < b.rect.x + b.rect.w && b.rect.x < a.rect.x + a.rect.w && a.rect.y < b.rect.y + b.rect.h && b.rect.y < a.rect.y + a.rect.h));
      const strangers = r.visible.filter((v) => !r.tabs.some((t) => t.id === v.tab));
      rule(3, `${when}: the pages lie inside the band, apart, and are this window's tabs`, outside.length === 0 && !overlap && strangers.length === 0,
        JSON.stringify({ visible: r.visible, frame: f, window: [W, H], tabs: r.tabs.map((t) => t.id) }));
    };
    // 2, after a change: Go's active tab is one the layout shows, and the dev window shows
    // a page for it.
    const activeShows = async (when) => {
      const ok = await until(async () => {
        const r = await regions();
        return shows(r) && (await page.locator(`iframe.mock-page[data-tab="${r.active}"]`).isVisible());
      });
      rule(2, `${when}: the active tab's page shows`, ok, JSON.stringify(await regions().then((r) => ({ active: r.active, visible: r.visible }))));
      // A page the window doesn't show can't take the keyboard, so the layout that shows it
      // goes first.
      const unshown = (await regions()).unshownFocus;
      rule(2, `${when}: the shell gave no page the keyboard before its layout showed it`, unshown.length === 0, JSON.stringify(unshown));
      await insideAndApart(when);
    };
    const tabCount = async (n) => until(async () => (await regions()).tabs.length === n && (await drawn()).length === n);
    // The settings page opens in a tab of the window, which becomes active.
    const settingsShown = () => until(async () => {
      const r = await regions();
      return r.settings !== 0 && r.active === r.settings;
    });
    // And closes as any tab does, the window then showing the tab made active.
    const closeSettings = async () => {
      await lib.closeTab((await regions()).settings);
      await until(async () => {
        const r = await regions();
        return r.settings === 0 && shows(r);
      });
    };

    // The field named Address, as a text box or a combo box.
    const address = ui.getByRole('textbox', { name: 'Address', exact: true }).or(ui.getByRole('combobox', { name: 'Address', exact: true }));
    // A control by its name: a button or a menu item, whose name may end with its key.
    const byName = (name) => {
      const pattern = new RegExp(`^(${name})\\b`);
      return ui.getByRole('button', { name: pattern }).or(ui.getByRole('menuitem', { name: pattern }));
    };
    // reach is the first of what locator finds that shows: on the shell, or once one of the
    // shell's controls that open something is pressed. One that shows nothing is closed again
    // with Escape. null when none shows it.
    // The controls that open something, to look in what each opens: those that say they open
    // a popup first, then other expanders, as a sidebar's toggle is one too and Escape
    // doesn't undo it.
    const openersOf = async () => [...await ui.locator('[aria-haspopup]').filter({ visible: true }).all(),
      ...await ui.locator('[aria-expanded]:not([aria-haspopup])').filter({ visible: true }).all()];
    const reach = async (locator) => {
      const shown = locator.filter({ visible: true });
      if (await shown.count()) return shown.first();
      for (const opener of await openersOf()) {
        await opener.click();
        if (await until(async () => (await shown.count()) > 0, 500)) return shown.first();
        await page.keyboard.press('Escape');
      }
      return null;
    };
    // The controls the shell shows, and those a press on misses: the layout follows what the
    // shell draws a frame or more later, so this looks until every control is the shell's.
    const shellsControls = async () => {
      let controls = [];
      let outsideShell = [];
      await until(async () => {
        controls = await w.shell().evaluate((control) => [...document.querySelectorAll(control)]
          .filter((el) => el.checkVisibility({ visibilityProperty: true, opacityProperty: true }))
          .map((el) => ({ what: el.outerHTML.slice(0, 60), rect: el.getBoundingClientRect().toJSON() }))
          .filter(({ rect }) => rect.width > 0 && rect.height > 0 && rect.right > 0 && rect.bottom > 0 && rect.left < innerWidth && rect.top < innerHeight), control);
        outsideShell = [];
        for (const c of controls) {
          const x = Math.min(Math.max(c.rect.x + c.rect.width / 2, 0), 1099);
          const y = Math.min(Math.max(c.rect.y + c.rect.height / 2, 0), 699);
          if ((await regions({ x, y })).hit !== 'shell') outsideShell.push(c.what);
        }
        return controls.length > 0 && outsideShell.length === 0;
      });
      return { controls, outsideShell };
    };

    // A check that throws fails the rule it was checking, and leaves the rest unchecked.
    let current = 2;
    if (!ready) {
      for (let n = 2; n <= 12; n++) rule(n, 'not checked: the shell never came up', false);
    } else {
      try {
        await activeShows('at the start');
        await openTab();
        await tabCount(2);
        await activeShows('after a new tab');
        await openTab();
        await tabCount(3);
        await activeShows('after another');
        await fire('menu', 'tab.select1');
        await until(async () => (await regions()).active === (await regions()).tabs[0].id);
        await activeShows('after another tab was made active');
        let r = await regions();
        await fire('open', 'https://opened.test/', { tab: r.active });
        await tabCount(4);
        await activeShows('after a page opened a tab');
        await fire('menu', 'tab.close');
        await tabCount(3);
        await activeShows('after the active tab closed');
        await page.setViewportSize({ width: 900, height: 600 });
        await until(async () => (await regions()).window.W === 900);
        await activeShows('after a resize');
        await page.setViewportSize({ width: 1100, height: 700 });
        await until(async () => (await regions()).window.W === 1100);
        await activeShows('after a resize back');

        // 4. The tabs as elements.
        current = 4;
        r = await regions();
        let d = await drawn();
        rule(4, 'each of Go\'s tabs is an element with role=tab and its data-tab, in order', JSON.stringify(d.map((t) => t.tab)) === JSON.stringify(r.tabs.map((t) => t.id)), JSON.stringify({ drawn: d, go: r.tabs.map((t) => t.id) }));
        rule(4, 'the active one, and only it, is aria-selected', d.every((t) => (t.selected === 'true') === (t.tab === r.active)), JSON.stringify({ drawn: d, active: r.active }));
        const other = r.tabs.find((t) => t.id !== r.active).id;
        // A tab may be in a sheet one of the shell's controls opens, as a phone's switcher
        // is: each is pressed in turn until the tab shows, and closed again with Escape.
        const otherEl = tabEls.nth(r.tabs.findIndex((t) => t.id === other));
        for (const opener of await openersOf()) {
          if (await otherEl.isVisible()) break;
          await opener.click();
          if (!(await until(() => otherEl.isVisible(), 500))) await page.keyboard.press('Escape');
        }
        await otherEl.click();
        rule(4, 'a click on a tab makes it active', await until(async () => (await regions()).active === other && (await drawn()).find((t) => t.tab === other)?.selected === 'true'), JSON.stringify(await regions().then((r) => r.active)));
        // Tab groups are Go's, whatever the shell makes of them (docs/groups.md): with two
        // tabs in one, every tab is still an element, and a click still makes it active.
        r = await regions();
        await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'createGroup', ids: r.tabs.slice(0, 2).map((t) => t.id) }) });
        await settle(page);
        d = await drawn();
        rule(4, 'with two tabs in a group, each tab is still an element', r.tabs.every((t) => d.some((x) => x.tab === t.id)), JSON.stringify(d));
        const grouped = r.tabs[1].id === r.active ? r.tabs[0].id : r.tabs[1].id;
        const groupedEl = tabEls.nth((await drawn()).findIndex((t) => t.tab === grouped));
        for (const opener of await openersOf()) {
          if (await groupedEl.isVisible()) break;
          await opener.click();
          if (!(await until(() => groupedEl.isVisible(), 500))) await page.keyboard.press('Escape');
        }
        await groupedEl.click();
        rule(4, 'and a click on one in the group makes it active', await until(async () => (await regions()).active === grouped), JSON.stringify(await regions().then((r) => r.active)));
        await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'activateTab', id: other }) });
        await until(async () => (await regions()).active === other);
        await settle(page);

        // 5. The address bar.
        current = 5;
        await fire('key', 'focusAddressBar', { tab: other });
        const inField = () => address.evaluateAll((els) => document.hasFocus() && els.includes(document.activeElement));
        rule(5, 'focusAddressBar puts the keyboard in the text field named Address', await until(inField), await w.shell().evaluate(() => document.activeElement?.outerHTML.slice(0, 80)));
        await page.keyboard.press('ControlOrMeta+A');
        await page.keyboard.type('contract.test/typed');
        await page.keyboard.press('Enter');
        rule(5, 'an address and Enter take the active tab there', await until(async () => (await regions()).tabs.find((t) => t.id === other)?.url === 'https://contract.test/typed'), JSON.stringify((await regions()).tabs.find((t) => t.id === other)));
        await activeShows('after a navigation');

        // 6. What the shell draws to be pressed is the shell's.
        current = 6;
        const { controls, outsideShell } = await shellsControls();
        rule(6, `every one of the ${controls.length} controls is the shell's`, controls.length > 0 && outsideShell.length === 0, outsideShell.join(' | '));

        // 7. A press in the middle of each page it shows.
        current = 7;
        r = await regions();
        for (const v of r.visible) {
          const x = v.rect.x + v.rect.w / 2;
          const y = v.rect.y + v.rect.h / 2;
          // The keyboard goes elsewhere first, as a shell can send it.
          // Its end is the shell's frame holding the keyboard too, which the window gives it
          // a moment after Go has it; a press before that would lose the keyboard to it.
          await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'focusTab', id: 0 }) });
          await until(async () => (await regions()).focused === 0 && (await w.shell().evaluate(() => document.hasFocus())));
          // With the middle button: the middle of a mock page may hold one of its buttons,
          // which a left press would set off, and a right press opens the right-click menu,
          // which a shell draws and gives the keyboard (rule 16).
          // The pointer over the page first: the window lets presses through the shell's frame
          // to the page under it once it has seen the pointer there (core/cmd/dev).
          await page.mouse.move(x - 1, y);
          await until(() => page.evaluate(() => document.querySelector('iframe.shell').style.pointerEvents === 'none'));
          await page.mouse.click(x, y, { button: 'middle' });
          rule(7, `a press in the middle of tab ${v.tab}'s page reaches it`, (await regions({ x, y })).hit === 'page' && await until(async () => (await regions()).focused === v.tab), JSON.stringify(await regions({ x, y }).then((h) => ({ hit: h.hit, focused: h.focused }))));
        }
        await activeShows('after a press in a page');

        // 8. Drag space, and the native buttons over no page.
        current = 8;
        r = await regions();
        rule(8, 'some space is marked drag', r.drag.length > 0, JSON.stringify(r.drag));
        const c = r.window.Controls;
        const under = r.visible.filter(({ rect: p }) => c.w > 0 && p.x < c.x + c.w && c.x < p.x + p.w && p.y < c.y + c.h && c.y < p.y + p.h);
        rule(8, 'no page lies under the native window buttons', under.length === 0, JSON.stringify({ controls: c, under }));

        // 9. Reloaded, the same pages.
        current = 9;
        const shownSet = (r) => JSON.stringify(r.visible.map((v) => v.tab).sort((a, b) => a - b));
        const before = shownSet(await regions());
        await w.shell().evaluate(() => { window.beforeReload = true; location.reload(); });
        const reloaded = await until(async () => (await w.shell()?.evaluate(() => !window.beforeReload).catch(() => false)) && up(), 10000);
        await settle(page);
        const after = shownSet(await regions());
        rule(9, 'reloaded, it comes back showing the same pages', reloaded && before === after, JSON.stringify({ before, after }));

        // 10. Away to another shell and back, through the pref, as View, Shell does.
        current = 10;
        const away = registry.find((s) => s.id !== id).id;
        await fire('menu', `shells:${away}`);
        rule(10, `switched away, the window loads ${away}`, await until(async () => (await regions()).shell === away && shows(await regions()), 10000), JSON.stringify((await regions()).shell));
        await fire('menu', `shells:${id}`);
        ready = await until(up, 10000);
        rule(10, 'switched back, it comes up from Go\'s data: its tabs, and the active one shown', ready, JSON.stringify(await drawn().catch((e) => e.message)));
        await insideAndApart('after switching back');

        // 11. Settings: a control named so, on the shell or in a menu one of its controls
        // opens. A menu item may show its key after the name.
        current = 11;
        const named = /^Settings\b/;
        const settings = ui.getByRole('button', { name: named }).or(ui.getByRole('menuitem', { name: named })).filter({ visible: true });
        // None on the shell: each control that opens something is pressed in turn, until
        // one shows it.
        const openers = await openersOf();
        // One that shows nothing is closed again with Escape, as rule 12 does.
        while ((await settings.count()) === 0 && openers.length > 0) {
          await openers.shift().click();
          if (!(await until(async () => (await settings.count()) > 0, 500))) await page.keyboard.press('Escape');
        }
        const found = (await settings.count()) > 0;
        rule(11, 'a control named Settings is on the shell, or in a menu one of its controls opens', found);
        if (found) {
          await settings.first().click();
          rule(11, 'pressed, it opens the settings tab', await settingsShown());
          await closeSettings();
        }

        // 12. The basic controls, each found only by its role and its name, as a screen
        // reader finds it, on the shell or in a menu one of its controls opens, and pressed
        // or typed into to see it work.
        current = 12;
        const escaped = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        // basic checks one control: that it is there, and that pressing it does what done
        // waits for.
        const basic = async (what, locator, press, done) => {
          const found = await reach(locator);
          if (!rule(12, `${what}: found by its role and name`, !!found)) return;
          const which = await found.evaluate((el) => el.outerHTML.slice(0, 80));
          await press(found);
          rule(12, `${what}: does its job`, await done(), which);
        };
        const click = (el) => el.click();
        const active = async () => {
          const r = await regions();
          return r.tabs.find((t) => t.id === r.active);
        };
        // Whether the active tab has loaded url.
        const at = (url) => until(async () => {
          const t = await active();
          return t.url === url && !t.loading;
        });

        // History to go back and forward in, made as a page makes it.
        const tab = (await regions()).active;
        await fire('navigate', 'https://contract.test/one', { tab });
        await at('https://contract.test/one');
        await fire('navigate', 'https://contract.test/two', { tab });
        await at('https://contract.test/two');
        await basic('Back', byName('Back'), click, () => at('https://contract.test/one'));
        await basic('Forward', byName('Forward'), click, () => at('https://contract.test/two'));
        // Reload loads the page again, which its mock page shows.
        const mockPage = await (await page.locator(`iframe.mock-page[data-tab="${tab}"]`).elementHandle()).contentFrame();
        await mockPage.evaluate(() => {
          const shown = document.getElementById('state');
          window.states = [];
          new MutationObserver(() => window.states.push(shown.textContent)).observe(shown, { childList: true, characterData: true, subtree: true });
        });
        await basic('Reload', byName('Reload'), click, async () =>
          await until(() => mockPage.evaluate(() => window.states.includes('Loading'))) && at('https://contract.test/two'));

        // New Tab is a way to a new tab: the press makes one, or puts the keyboard in a field
        // where an address and Enter make one.
        const had = (await regions()).tabs.map((t) => t.id);
        let opened = 0;
        const fresh = async (url) => {
          const r = await regions();
          const t = r.tabs.find((t) => !had.includes(t.id));
          opened = t?.id || 0;
          return opened !== 0 && r.active === opened && (!url || (t.url === url && !t.loading));
        };
        await basic('New Tab', byName('New Tab'), click, async () => {
          if (!(await until(async () => (await fresh()) || (await inTextField())))) return false;
          if (await fresh()) return true;
          await page.keyboard.type('contract.test/new');
          await page.keyboard.press('Enter');
          return until(() => fresh('https://contract.test/new'));
        });
        await basic('the text field named Address', address, async (field) => {
          await field.click();
          await page.keyboard.press('ControlOrMeta+A');
          await page.keyboard.type('close.test/tab');
          await page.keyboard.press('Enter');
        }, () => at('https://close.test/tab'));
        // The tab just opened has a title of its own, which no other tab has.
        await until(async () => (await active()).title !== '');
        const title = (await active()).title;
        await basic(`a way to close a tab, Close Tab or Close ${title}`, byName(`Close Tab|Close ${escaped(title)}`), click,
          () => until(async () => !(await regions()).tabs.some((t) => t.id === opened)));

        await basic('Settings', byName('Settings'), click, settingsShown);
        await closeSettings();
        // Extensions, whose menu's Manage extensions opens the settings page.
        await basic('Extensions', byName('Extensions'), async (el) => {
          await el.click();
          await byName('Manage extensions').filter({ visible: true }).first().click();
        }, settingsShown);
        await closeSettings();
      } catch (e) {
        rule(current, 'its checks ran to their end', false, e.message.split('\n')[0]);
        for (let n = current + 1; n <= 12; n++) rule(n, 'not checked: an earlier check ended the run', false);
      }
    }

    // 15. A page's dialogs, on every shell.
    if (noDialogsYet[id]) {
      rule(15, `not checked yet: ${noDialogsYet[id]}`, true);
    } else {
      try {
        const ask = ui.getByRole('alertdialog');
        const answers = async () => (await regions()).dialogAnswers || [];
        const before = (await answers()).length;
        await fire('confirm', 'Go on?');
        rule(15, 'a confirm shows as an alertdialog with its words', await until(async () => (await ask.isVisible()) && (await ask.innerText()).includes('Go on?')));
        rule(15, "with Go's buttons, OK and Cancel, by name", (await ask.getByRole('button', { name: 'OK' }).count()) === 1 && (await ask.getByRole('button', { name: 'Cancel' }).count()) === 1);
        const home = (await regions()).active;
        await openTab();
        rule(15, 'another tab shown hides it', await until(async () => (await regions()).active !== home && (await ask.count()) === 0));
        // By its tab where one shows; a phone's tabs sit in a switcher that's shut, so there
        // Go shows it, as the switcher would.
        const homeTab = ui.locator(`[role=tab][data-tab="${home}"]`).first();
        if (await homeTab.isVisible()) await homeTab.click({ force: true });
        else await fetch(`${lib.base}/shell/send`, { method: 'POST', body: JSON.stringify({ t: 'activateTab', id: home }) });
        rule(15, 'and showing its tab again brings it back', await until(() => ask.isVisible()));
        await ask.getByRole('button', { name: 'OK' }).click();
        rule(15, 'OK answers the page yes', await until(async () => (await answers()).length === before + 1 && (await answers()).at(-1).ok === true));
        await fire('prompt', 'Name?');
        await until(() => ask.isVisible());
        const field = ask.getByRole('textbox');
        rule(15, 'a prompt has a field', (await field.count()) === 1);
        await field.fill('Ada');
        await ask.getByRole('button', { name: 'OK' }).click();
        rule(15, "and OK answers with the field's text", await until(async () => (await answers()).at(-1)?.text === 'Ada'));
      } catch (e) {
        rule(15, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 22. A site's sign-in, on every shell that draws dialogs.
    if (noDialogsYet[id]) {
      rule(22, `not checked yet: ${noDialogsYet[id]}`, true);
    } else {
      try {
        const ask = ui.getByRole('alertdialog');
        await fire('signin', 'Staff only');
        rule(22, 'a sign-in shows as an alertdialog with Username and Password', await until(async () => (await ask.isVisible()) && (await ask.getByRole('textbox', { name: 'Username' }).count()) === 1 && (await ask.getByLabel('Password').count()) === 1));
        await ask.getByRole('textbox', { name: 'Username' }).fill('ada');
        await ask.getByLabel('Password').fill('secret');
        await ask.getByRole('button', { name: 'Sign in' }).click();
        rule(22, "Go's Sign in answers with both", await until(async () => ((await regions()).dialogAnswers || []).at(-1)?.text === 'ada:secret'));
      } catch (e) {
        rule(22, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 16. The right-click menu, on a desktop shell.
    const shell = registry.find((s) => s.id === id);
    if (shell.phone) {
      rule(16, 'not for a phone: a long press keeps its own sheet', true);
    } else if (noMenuYet[id]) {
      rule(16, `not checked yet: ${noMenuYet[id]}`, true);
    } else {
      try {
        const menu = ui.getByRole('menu').first();
        const open = async (link) => {
          await fire('contextMenu', JSON.stringify({ x: 60, y: 60, link }));
          return until(() => menu.isVisible());
        };
        const closed = () => until(async () => (await ui.getByRole('menu').count()) === 0);
        rule(16, 'a right click on a link opens a role=menu', await open('https://contract.test/menu'));
        const copy = ui.getByRole('menuitem', { name: /^Copy Link/ });
        rule(16, "Go's Copy Link is a menuitem by its title", (await copy.count()) === 1, JSON.stringify(await ui.getByRole('menuitem').allInnerTexts()));
        await copy.click();
        rule(16, 'choosing it makes Go do it, and the menu goes', (await until(async () => (await regions()).clipboard === 'https://contract.test/menu')) && (await closed()));
        await open('https://contract.test/escape');
        await menu.press('Escape');
        rule(16, 'Escape closes it', await closed());
        await open('https://contract.test/outside');
        const v = (await regions()).visible[0].rect;
        await page.mouse.click(v.x + v.w - 20, v.y + v.h - 20);
        rule(16, 'a press outside closes it, and nothing ran', (await closed()) && (await regions()).clipboard === 'https://contract.test/menu');
      } catch (e) {
        rule(16, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 30. The permission prompt.
    if (noPromptYet[id]) {
      rule(30, `not checked yet: ${noPromptYet[id]}`, true);
    } else {
      try {
        const asked = (host, kinds) => ui.getByRole('dialog', { name: `${host} wants to ${kinds}` });
        // Go's statuses, as the mock reports them: 0 ask, 1 granted, 2 denied.
        const answered = async (n) => {
          let a = [];
          await until(async () => (a = (await regions()).permissions).length === n);
          return a[n - 1]?.statuses || [];
        };
        // Whether tab has committed url: Go has the address from the load's start.
        const loaded = (tab, url) => until(async () => {
          const t = (await regions()).tabs.find((x) => x.id === tab);
          return t?.url === url && !t.loading;
        });
        const tab = (await regions()).active;
        await fire('navigate', 'https://contract.test/call', { tab });
        await loaded(tab, 'https://contract.test/call');
        await fire('permission', 'camera,microphone', { tab });
        const prompt = asked('contract.test', 'use your camera and microphone');
        rule(30, 'a request for the camera and microphone brings a dialog named by its question, with the site', await until(() => prompt.isVisible()));
        await prompt.getByRole('button', { name: 'Allow this time', exact: true }).click();
        rule(30, 'Allow this time grants both, and the dialog goes', JSON.stringify(await answered(1)) === '[1,1]' && (await until(async () => (await prompt.count()) === 0)));
        await fire('permission', 'notifications', { tab });
        const notify = asked('contract.test', 'show notifications');
        await until(() => notify.isVisible());
        rule(30, 'a notifications question has Block and Allow, and no Allow this time', (await notify.getByRole('button', { name: 'Allow', exact: true }).count()) === 1 && (await notify.getByRole('button', { name: 'Allow this time' }).count()) === 0);
        await notify.getByRole('button', { name: 'Block' }).click();
        rule(30, 'Block denies it', JSON.stringify(await answered(2)) === '[2]');
        await fire('navigate', 'https://other.test/call', { tab });
        await loaded(tab, 'https://other.test/call');
        await fire('permission', 'camera', { tab });
        const again = asked('other.test', 'use your camera');
        await until(() => again.isVisible());
        await page.keyboard.press('Escape');
        rule(30, 'Escape dismisses it, answering neither', JSON.stringify(await answered(3)) === '[0]' && (await until(async () => (await again.count()) === 0)));
      } catch (e) {
        rule(30, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 31. Site settings, from the page's site.
    if (noPromptYet[id]) {
      rule(31, `not checked yet: ${noPromptYet[id]}`, true);
    } else {
      try {
        const tab = (await regions()).active;
        await fire('navigate', 'https://contract.test/site', { tab });
        await until(async () => {
          const t = (await regions()).tabs.find((x) => x.id === tab);
          return t?.url === 'https://contract.test/site' && !t.loading;
        });
        await fire('focusTab', '', { tab });
        const control = ui.getByRole('button', { name: 'Site settings' });
        rule(31, 'on a web page, a control named Site settings is there', await until(() => control.first().isVisible()));
        await control.first().click();
        if (!noPanelYet[id]) {
          await ui.getByRole('dialog', { name: 'Site settings for contract.test' }).getByRole('button', { name: 'Details' }).click();
        }
        const site = page.frameLocator('iframe.our-page[data-page="settings"]').getByRole('region', { name: 'contract.test' });
        rule(31, 'pressed, it opens settings on that site', (await settingsShown()) && (await until(async () => (await site.count()) === 1)));
        await closeSettings();
      } catch (e) {
        rule(31, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 33. The site panel.
    if (noPanelYet[id]) {
      rule(33, `not checked yet: ${noPanelYet[id]}`, true);
    } else {
      try {
        const tab = (await regions()).active;
        await fire('navigate', 'https://contract.test/panel', { tab });
        await until(async () => {
          const t = (await regions()).tabs.find((x) => x.id === tab);
          return t?.url === 'https://contract.test/panel' && !t.loading;
        });
        await fire('focusTab', '', { tab });
        const control = ui.getByRole('button', { name: 'Site settings' }).first();
        const panel = ui.getByRole('dialog', { name: 'Site settings for contract.test' });
        const reopen = async () => {
          await page.keyboard.press('Escape');
          await until(async () => (await panel.count()) === 0);
          await control.click();
          return until(() => panel.isVisible());
        };
        const tabsBefore = (await regions()).tabs.length;
        await control.click();
        rule(33, 'pressed, it opens a dialog named for the site', await until(() => panel.isVisible()));
        await page.waitForTimeout(200);
        fs.mkdirSync(path.join(__dirname, '../../build/e2e'), { recursive: true });
        await page.screenshot({ path: path.join(__dirname, `../../build/e2e/site-panel-${id}.png`) });
        const ads = panel.getByRole('checkbox', { name: /Ad blocker/ });
        rule(33, 'the ad blocker is on, as Go has it', await until(() => ads.isChecked()));
        await ads.click();
        await reopen();
        rule(33, 'switched off, Go keeps it off', await until(async () => !(await ads.isChecked())));
        const sparks = panel.getByRole('checkbox', { name: /^Sparks/ });
        rule(33, 'Sparks is on, naming the sparks that run here', (await until(() => sparks.isChecked())) && (await panel.getByText('Contract spark').isVisible()));
        await sparks.click();
        const reloadNow = panel.getByRole('button', { name: 'Reload', exact: true });
        rule(33, 'a change the page needs again says so, with Reload', await until(() => reloadNow.isVisible()));
        await reloadNow.click();
        rule(33, 'Reload reloads the page, and the line goes', await until(async () => (await reloadNow.count()) === 0));
        await reopen();
        rule(33, 'switched off, Go keeps sparks off on the site', await until(async () => !(await sparks.isChecked())));
        await sparks.click();
        const block = panel.getByRole('radiogroup', { name: 'Links to other sites' }).getByRole('radio', { name: 'Block' });
        await block.click();
        await reopen();
        rule(33, 'Links to other sites set to Block, Go keeps it', await until(async () => (await block.getAttribute('aria-checked')) === 'true'));
        // Under Block, a popup the page opens to another site closes at once, and the
        // address field counts it: a press opens the panel.
        await page.keyboard.press('Escape');
        await until(async () => (await panel.count()) === 0);
        await fire('open', 'https://elsewhere.test/', { tab });
        const popups = ui.getByRole('button', { name: '1 popup blocked' });
        rule(33, 'a popup to another site under Block closes, and the field counts it', (await until(() => popups.isVisible())) && (await regions()).tabs.length === tabsBefore);
        await popups.click();
        rule(33, 'the count opens the panel', await until(() => panel.isVisible()));
        await ads.click();
        await panel.getByRole('radiogroup', { name: 'Links to other sites' }).getByRole('radio', { name: 'Allow' }).click();
        await page.keyboard.press('Escape');
        rule(33, 'Escape closes it', await until(async () => (await panel.count()) === 0));
      } catch (e) {
        rule(33, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 32. The notice over a page with nothing to show.
    if (noNoticeYet[id]) {
      rule(32, `not checked yet: ${noNoticeYet[id]}`, true);
    } else {
      try {
        const tab = (await regions()).active;
        const tabNow = async () => (await regions()).tabs.find((x) => x.id === tab);
        const loaded = (url) => until(async () => {
          const t = await tabNow();
          return t?.url === url && !t.loading && !t.notice;
        });
        const alert = (text) => ui.getByRole('alert').filter({ hasText: text });
        await fire('navigate', 'https://contract.test/down', { tab });
        await loaded('https://contract.test/down');
        await fire('fail', 'refused ERR_CONNECTION_REFUSED', { tab });
        const failed = alert("Can't reach contract.test");
        rule(32, 'a load that fails brings an alert with Go\'s sentence', await until(() => failed.isVisible()));
        await failed.getByRole('button', { name: 'Try again' }).click();
        rule(32, 'Try again loads the page again, and the alert goes', (await loaded('https://contract.test/down')) && (await until(async () => (await ui.getByRole('alert').count()) === 0)));
        await fire('certificate', 'https://old.test/', { tab });
        const warning = alert("old.test's certificate has expired");
        rule(32, 'a bad certificate brings an alert that says what is wrong with it', await until(() => warning.isVisible()));
        await warning.getByRole('button', { name: 'Open anyway' }).click();
        rule(32, 'Open anyway opens the page, which is then not secure', (await loaded('https://old.test/')) && (await tabNow()).connection === 'insecure' && (await until(async () => (await ui.getByRole('alert').count()) === 0)));
      } catch (e) {
        rule(32, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 17. Downloads. On a phone the control may be an item of the menu one of the shell's
    // controls opens, and the file shows in the phone's Downloads rather than Finder.
    if (noDownloadsYet[id]) {
      rule(17, `not checked yet: ${noDownloadsYet[id]}`, true);
    } else {
      try {
        const reveal = shell.phone ? 'Show in Downloads' : 'Show in Finder';
        const control = ui.getByRole('button', { name: 'Downloads' }).or(ui.getByRole('menuitem', { name: 'Downloads' }));
        await fire('download', 'https://contract.test/files/contract.pdf');
        const menu = ui.getByRole('button', { name: 'Menu', exact: true });
        const shows = async () => {
          if (await control.first().isVisible()) return true;
          if (shell.phone && await menu.isVisible()) await menu.click();
          return control.first().isVisible();
        };
        rule(17, 'a download brings a control named Downloads', await until(shows));
        await control.first().click();
        // The list is the shell's own, or the Downloads page, one of our tabs, which the
        // control opens.
        const inShell = ui.getByRole('listitem', { name: 'contract.pdf' });
        const inPage = page.frameLocator('iframe.our-page[data-page="downloads"]').getByRole('listitem', { name: 'contract.pdf' });
        let entry = null;
        rule(17, 'which opens a list with the download by its file name', await until(async () => {
          entry = (await inShell.isVisible()) ? inShell : (await inPage.isVisible()) ? inPage : null;
          return entry !== null;
        }));
        rule(17, `finished, it has ${reveal}`, await until(async () => (await entry.getByRole('button', { name: reveal }).count()) === 1));
        await entry.getByRole('button', { name: reveal }).click();
        rule(17, `${reveal} makes Go show the file`, await until(async () => (await regions()).shown?.length > 0));
        // The page closes as any tab; the shell's list with Escape.
        const downloadsTab = (await regions()).tabs.find((t) => t.page === 'downloads');
        if (downloadsTab) {
          await lib.closeTab(downloadsTab.id);
          await until(async () => !(await regions()).tabs.some((t) => t.page === 'downloads'));
        } else {
          await page.keyboard.press('Escape');
        }
      } catch (e) {
        rule(17, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 20. Find in page. A phone has no find key, so its menu runs the action, and its back
    // key closes the bar rather than Escape.
    if (noFindYet[id]) {
      rule(20, `not checked yet: ${noFindYet[id]}`, true);
    } else {
      try {
        const field = ui.getByRole('textbox', { name: 'Find in page' });
        await fire('menu', 'find.open');
        rule(20, 'the find key opens a text box named Find in page, with the keyboard in it', await until(async () => (await field.isVisible()) && (await field.evaluate((el) => el === document.activeElement))));
        await field.pressSequentially('mock');
        const shows = (s) => until(async () => (await ui.locator('body').innerText()).includes(s));
        rule(20, 'typing searches, and the count shows', await shows('3'));
        await field.press('Enter');
        rule(20, 'Enter shows the next match', await shows('2'));
        if (shell.phone) await fire('backKey');
        else await field.press('Escape');
        rule(20, `${shell.phone ? 'The back key' : 'Escape'} closes it, and Go ends the search`, await until(async () => (await field.count()) === 0 && (await regions()).findsStopped?.length > 0));
      } catch (e) {
        rule(20, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 21. A page in full screen. A phone ends it with its back key rather than Escape.
    if (noPageFullYet[id]) {
      rule(21, `not checked yet: ${noPageFullYet[id]}`, true);
    } else {
      try {
        const before = (await regions()).visible.map((v) => v.rect);
        const screen = page.viewportSize();
        await fire('pageFullscreen', '');
        rule(21, 'a page in full screen covers the whole window', await until(async () => {
          const r = await regions();
          return r.visible.length === 1 && r.visible[0].rect.x === 0 && r.visible[0].rect.y === 0 && Math.round(r.visible[0].rect.w) === screen.width && Math.round(r.visible[0].rect.h) === screen.height;
        }), JSON.stringify((await regions()).visible));
        const controls = await ui.locator(control).evaluateAll((els) => els.filter((el) => el.getClientRects().length > 0)
          .map((el) => el.getAttribute('aria-label') || el.title || el.textContent.trim().slice(0, 30) || el.tagName));
        rule(21, 'with nothing of the shell on it', controls.length === 0, `${controls.length} controls show: ${controls.join(' | ')}`);
        await fire(...(shell.phone ? ['backKey'] : ['key', 'escape']));
        rule(21, `${shell.phone ? 'The back key' : 'Escape'} ends it, and the window is as it was`, await until(async () => JSON.stringify((await regions()).visible.map((v) => v.rect)) === JSON.stringify(before)));
      } catch (e) {
        rule(21, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 19. One of our tabs: the settings tab, with its title where the address goes, no
    // field to type in, and a way to close it.
    if (noTitleYet[id]) {
      rule(19, `not checked yet: ${noTitleYet[id]}`, true);
    } else if (ready) {
      try {
        await fire('menu', 'app.settings');
        const shown = await until(async () => {
          const r = await regions();
          return r.settings !== 0 && r.active === r.settings && r.tabs.find((t) => t.id === r.settings).title !== '';
        });
        const r = await regions();
        const title = r.tabs.find((t) => t.id === r.settings)?.title;
        rule(19, 'the settings tab shows, with its title', shown, JSON.stringify(r.tabs));
        await settle(page);
        const field = ui.getByRole('textbox', { name: 'Address', exact: true }).or(ui.getByRole('combobox', { name: 'Address', exact: true }));
        rule(19, 'no field named Address shows for it', (await field.filter({ visible: true }).count()) === 0);
        const titled = await ui.getByText(title, { exact: true }).filter({ visible: true })
          .evaluateAll((els) => els.filter((el) => !el.closest('[role=tab]')).length);
        rule(19, `its title, ${title}, shows outside the tabs`, titled > 0);
        const close = ui.getByRole('button', { name: new RegExp(`^Close (Tab|${title})\\b`) }).filter({ visible: true });
        if (rule(19, 'a way to close it shows', (await close.count()) > 0)) {
          await close.first().click();
          rule(19, 'and closes it', await until(async () => (await regions()).settings === 0));
        }
      } catch (e) {
        rule(19, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 18. New Tab shows a new tab, which has nothing loaded: the shell draws its new tab
    // where the page goes, as its own. Then its tabs closed one at a time, as Close Tab
    // does: the last closes the window, so it runs last.
    if (!ready) {
      rule(18, 'not checked: the shell never came up', false);
    } else {
      try {
        // Where the page shows, which the shell draws itself on a new tab.
        const where = (await regions()).visible[0].rect;
        const middle = { x: where.x + where.w / 2, y: where.y + where.h / 2 };
        const newTab = await reach(byName('New Tab'));
        if (rule(18, 'New Tab: found by its role and name', !!newTab)) {
          await newTab.click();
          rule(18, 'New Tab shows a new tab, with nothing loaded, the active one', await until(async () => fresh(await regions())), JSON.stringify(await regions().then((r) => ({ tabs: r.tabs.map((t) => [t.id, t.url]), active: r.active }))));
          rule(18, 'the shell draws its new tab where the page goes: a press there reaches the shell', await until(async () => (await regions(middle)).hit === 'shell'), JSON.stringify({ where, hit: (await regions(middle)).hit }));
        }
        for (let n = (await regions()).tabs.length; n > 1; n--) {
          await fire('menu', 'tab.close');
          await tabCount(n - 1);
        }
        rule(18, 'closing tabs keeps the window while it has one', !(await regions()).closed);
        await fire('menu', 'tab.close');
        rule(18, 'closing the last tab closes the window', await until(async () => (await regions()).closed === true));
      } catch (e) {
        rule(18, 'its checks ran to their end', false, e.message.split('\n')[0]);
      }
    }

    // 13.
    rule(13, 'it logged no errors', w.errors.length === 0, w.errors.join(' | '));

    await w.page.close();
    await lib.stop();
    for (const [n, name] of Object.entries(rules)) summary.push(`${broken.has(+n) ? 'FAIL' : 'ok  '} ${id} ${n}. ${name}`);
  }
  await browser.close();
  console.log('\n' + summary.join('\n'));
  await lib.done();
})();
