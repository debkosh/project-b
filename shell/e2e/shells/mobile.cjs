// The mobile shell's own tests, on the mock engine in a phone-sized window with a touch
// screen: the one bar at the foot, the address pill and its editing with the suggestions
// over the field, a new tab in edit mode, the switcher (open, switch, close a card, the
// last tab), the menu sheet and Escape, a swipe along the bar, the workspaces, light and
// dark, and where bookmarks show. The check covers what every shell does.
// Run: node shell/e2e/shells/mobile.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const centred = (inner, outer) => Promise.all([inner.boundingBox(), outer.boundingBox()]).then(([i, o]) => Math.abs(i.x + i.width / 2 - (o.x + o.width / 2)) < 1);
const { check, fire, regions, settle, until } = lib;

const id = 'mobile';
const W = 412;
const H = 915;

(async () => {
  fs.writeFileSync(path.join(lib.profile, 'workspaces.json'), JSON.stringify({ workspaces: [{ id: '0a1b2c3d', name: 'Work', color: '#3470d0' }] }));
  await lib.start([`-use=${id}`]);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser, { viewport: { width: W, height: H }, hasTouch: true });
  const { page, ui } = w;
  // Console messages for a page picture that failed to load, as lib.watch words them.
  const thumbMisses = [];
  page.on('console', (m) => m.type() === 'error' && /\/mock\/thumb\//.test(m.location().url) && thumbMisses.push(m.text()));
  page.setDefaultTimeout(5000);

  const field = ui.getByRole('textbox', { name: 'Address' });
  const tabsButton = ui.getByRole('button', { name: 'Tabs', exact: true });
  const menuButton = ui.getByRole('button', { name: 'Menu' });
  const switcher = ui.getByRole('dialog', { name: 'Tabs' });
  const menu = ui.getByRole('dialog', { name: 'Menu' });
  const cards = switcher.getByRole('tab');
  const suggestions = ui.getByRole('list', { name: 'Suggestions' }).getByRole('listitem');
  // Whether the list on screen is Go's answer for text, rather than an earlier text's.
  const listAnswers = (text) => until(async () => (await ui.getByRole('list', { name: 'Suggestions' }).getAttribute('data-answers').catch(() => null)) === text);
  const box = (selector) => w.shell().evaluate((s) => document.querySelector(s)?.getBoundingClientRect().toJSON(), selector);
  const active = async () => {
    const r = await regions();
    return r.tabs.find((t) => t.id === r.active);
  };
  const tabCount = (n) => until(async () => (await regions()).tabs.length === n);
  const inField = () => w.shell().evaluate(() => document.hasFocus() && document.activeElement?.getAttribute('aria-label') === 'Address');
  // A tab with nothing in it shows the new tab page, with the keyboard down
  // (docs/daily-driver.md, A new tab).
  const newTabPage = async () => (await ui.getByRole('region', { name: 'New tab' }).isVisible()) && !(await inField());
  const visit = async (text, url) => {
    await field.tap();
    await page.keyboard.press('ControlOrMeta+A');
    await page.keyboard.type(text);
    await page.keyboard.press('Enter');
    return until(async () => {
      const t = await active();
      return t.url === url && !t.loading;
    });
  };
  // Every control a finger presses is 48px or more each way, Android's least (48dp; a CSS
  // pixel is a dp on a phone). Lists those that aren't.
  const { controls: controlSelector } = await import('../../src/sdk/controls.js');
  const smallTargets = () => w.shell().evaluate((sel) => [...document.querySelectorAll(sel)]
    .filter((el) => el.checkVisibility({ visibilityProperty: true }) && el.getBoundingClientRect().width > 0)
    .map((el) => ({ what: el.getAttribute('aria-label') || el.textContent.trim().slice(0, 20) || el.tagName, r: el.getBoundingClientRect() }))
    .filter(({ r }) => r.width < 47.5 || r.height < 47.5)
    .map(({ what, r }) => `${what} ${Math.round(r.width)}x${Math.round(r.height)}`), controlSelector);
  // Every button and switch has a name of its own: a button its words or aria-label, a switch
  // or checkbox its aria-label. Android reads none from a <label> around a switch, nor from
  // aria-labelledby, and then names it nothing.
  const nameless = () => w.shell().evaluate(() => [...document.querySelectorAll('button, [role=switch], input[type=checkbox]')]
    .filter((el) => el.checkVisibility({ visibilityProperty: true }) && el.getBoundingClientRect().width > 0)
    .filter((el) => {
      const own = el.tagName === 'BUTTON' ? el.textContent : '';
      return !`${el.getAttribute('aria-label') || ''}${own}`.trim();
    })
    .map((el) => el.outerHTML.slice(0, 80)));
  const shots = async (what) => {
    const small = await smallTargets();
    check(`every control is 48px or more each way: ${what}`, small.length === 0, small.join(', '));
    const unnamed = await nameless();
    check(`every button and switch has a name of its own: ${what}`, unnamed.length === 0, unnamed.join(' | '));
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await settle(page);
      await page.screenshot({ path: path.join(lib.out, `${id}-${what}-${scheme}.png`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
  };

  check('the window loads the mobile shell with its one tab', await until(async () => (await regions()).shell === id && (await regions()).visible.length === 1, 10000));
  let r = await regions();
  check('it asks for no band', r.frameWidth === 0, String(r.frameWidth));

  // The one bar, at the foot: 48px, the page above it.
  const bar = await box('.toolbar');
  check('the toolbar is the window\'s foot, 48px', bar.bottom === H && bar.height === 48, JSON.stringify(bar));
  r = await regions();
  const shown = r.visible[0].rect;
  check('the page fills the window down to the toolbar', shown.x === 0 && shown.w === W && shown.y + shown.h === H - 48, JSON.stringify(shown));
  const pill = await field.boundingBox();
  check('the address pill fills the bar beside the count and ⋯', pill.width > W - 140, JSON.stringify(pill));

  // A first tab with nothing in it shows the new tab page, the keyboard down.
  check('the first, empty tab shows the new tab page, the keyboard down', await until(newTabPage));
  await visit('example.com/docs/release', 'https://example.com/docs/release');
  check('an address and Enter take the tab there', true);
  await settle(page);
  check('at rest the pill shows the host alone', await until(async () => (await field.inputValue()) === 'example.com'), await field.inputValue());
  check('with a lock, the page being https', (await ui.locator('.address .mark').count()) === 1);
  // A page whose connection isn't secure says so in words at the pill's start, in --danger,
  // and the words are the site's mark, which opens the site's sheet.
  const notSecure = ui.getByRole('button', { name: 'Not secure · Site settings' });
  check('a secure page has no Not secure', (await notSecure.count()) === 0);
  await visit('http://example.com/plain', 'http://example.com/plain');
  check('an http page says Not secure at the pill\'s start', await until(() => notSecure.isVisible()) && (await notSecure.innerText()).trim() === 'Not secure');
  const danger = await w.shell().evaluate(() => {
    const probe = document.createElement('i');
    probe.style.color = 'var(--danger)';
    document.body.append(probe);
    const c = getComputedStyle(probe).color;
    probe.remove();
    return c;
  });
  check('in --danger', (await notSecure.evaluate((el) => getComputedStyle(el).color)) === danger);
  const notSecureBox = await notSecure.boundingBox();
  check('before the host', notSecureBox.x + notSecureBox.width <= (await field.boundingBox()).x + 1);
  await shots('not-secure');
  await notSecure.tap();
  const siteSheet = ui.getByRole('dialog', { name: /^Site settings for / });
  check('a tap on it opens the site\'s sheet, which says why', await until(async () => (await siteSheet.isVisible()) && /Not secure/.test(await siteSheet.innerText())), await siteSheet.innerText().catch(() => ''));
  await page.keyboard.press('Escape');
  check('Escape closes it', await until(async () => (await siteSheet.count()) === 0));
  await fire('certificate', 'https://old.test/');
  const certificate = ui.getByRole('alert');
  check('a bad certificate brings Go\'s notice over the page', await until(async () => (await active()).notice && (await certificate.getByRole('button', { name: 'Open anyway' }).isVisible())), JSON.stringify((await active()).notice));
  await certificate.getByRole('button', { name: 'Open anyway' }).tap();
  check('an https page whose certificate was passed with Open anyway says Not secure too', await until(async () => (await active()).url === 'https://old.test/' && (await notSecure.isVisible())));
  await visit('example.com/docs/rollout', 'https://example.com/docs/rollout');
  await visit('example.org/release-notes', 'https://example.org/release-notes');
  await shots('page');

  // Editing: a tap selects the whole address; typing lists suggestions over the field,
  // the closest nearest it.
  await field.tap();
  check('a tap on the pill puts the whole address in the field, selected', await until(async () => (await field.inputValue()) === 'https://example.org/release-notes' &&
    await w.shell().evaluate(() => { const f = document.activeElement; return f.selectionStart === 0 && f.selectionEnd === f.value.length; })));
  await page.keyboard.type('example.com/docs');
  check('typing lists suggestions', await listAnswers('example.com/docs') && await until(async () => (await suggestions.count()) >= 2), String(await suggestions.count()));
  const list = await box('.suggestions');
  const fieldBox = await field.boundingBox();
  check('they lie over the field, the toolbar under them', list.bottom <= fieldBox.y, JSON.stringify({ list, fieldBox }));
  check('and span the screen, edge to edge', Math.round(list.left) === 0 && Math.round(list.right) === W, JSON.stringify(list));
  const rows = await suggestions.evaluateAll((els) => els.map((e) => e.getBoundingClientRect().height));
  check('each row is 48px', rows.every((h) => Math.round(h) === 48 || Math.round(h) === 49), JSON.stringify(rows));
  check('a press on them reaches the shell, over the page', await until(async () => (await regions({ x: 100, y: list.bottom - 10 })).hit === 'shell'));
  await shots('suggestions');
  const nearest = suggestions.last();
  const target = await nearest.locator('.url').innerText();
  await nearest.getByRole('button', { name: /^Edit / }).tap();
  check('↖ puts its address in the field and keeps editing', await until(async () => (await field.inputValue()).endsWith(target) && await inField()), await field.inputValue());
  await page.keyboard.press('Escape');
  check('Escape puts the address back and gives the page the keyboard', await until(async () => (await field.inputValue()) === 'example.org' && (await regions()).focused === (await regions()).active));
  await field.tap();
  await page.keyboard.type('example.com/docs/re');
  await listAnswers('example.com/docs/re');
  await until(async () => (await suggestions.count()) >= 1);
  const go = await suggestions.last().locator('.url').innerText();
  await suggestions.last().getByRole('button').first().tap();
  check('a tap on a suggestion goes there', await until(async () => (await active()).url.endsWith(go) && (await suggestions.count()) === 0), JSON.stringify({ go, now: (await active()).url }));

  // A new tab from the menu starts in edit mode. The page loaded first, so the menu offers
  // Reload rather than Stop.
  await until(async () => !(await active()).loading);
  await menuButton.tap();
  check('⋯ opens the menu sheet', await until(() => menu.isVisible()));
  check('with Back, as the tab has somewhere to go back to', await menu.getByRole('button', { name: 'Back' }).isVisible());
  // Shaped for a phone: the page's actions in a row, a flat list, the places as tiles,
  // Settings, and More last (docs/shells.md, "The menu sheet").
  const menuItem = (name) => menu.getByRole('menuitem', { name, exact: true });
  const actionRow = () => menu.locator('.actions button').evaluateAll((els) => els.map((e) => e.getAttribute('aria-label')));
  check('its row of the page\'s actions: Back, Forward, Bookmark, Reload and New tab', await until(async () => JSON.stringify(await actionRow()) === JSON.stringify(['Back', 'Forward', 'Bookmark', 'Reload', 'New Tab'])), JSON.stringify(await actionRow()));
  check('then the flat list: Print…, which Go\'s menu lifts to its top, Find in page, and Workspaces, as there are two', await until(async () => JSON.stringify(await menu.getByRole('menu', { name: 'Page' }).getByRole('menuitem').evaluateAll((els) => els.map((e) => e.getAttribute('aria-label')))) === JSON.stringify(['Print…', 'Find in page', 'Workspaces'])),
    JSON.stringify(await menu.getByRole('menu', { name: 'Page' }).getByRole('menuitem').evaluateAll((els) => els.map((e) => e.getAttribute('aria-label')))));
  const tileRow = menu.getByRole('menu', { name: 'Places' }).getByRole('menuitem');
  check('then History, Bookmarks and Downloads as tiles in one row', JSON.stringify(await tileRow.evaluateAll((els) => els.map((e) => e.getAttribute('aria-label')))) === JSON.stringify(['History', 'Bookmarks', 'Downloads']) &&
    new Set(await tileRow.evaluateAll((els) => els.map((e) => Math.round(e.getBoundingClientRect().top)))).size === 1);
  check('then Settings, and More last', JSON.stringify(await menu.getByRole('menu', { name: 'App' }).getByRole('menuitem').evaluateAll((els) => els.map((e) => e.getAttribute('aria-label')))) === JSON.stringify(['Settings', 'More']));
  check('and none of the menu bar\'s menus at its top level', (await menuItem('File').count()) === 0 && (await menuItem('View').count()) === 0);
  await shots('menu');
  // The star bookmarks the page, and its sheet takes the menu's place, to change the name or
  // the folder (docs/bookmark-folders.md); the star is filled while it is one, and a second
  // tap shows the sheet to edit, whose Remove takes it away.
  const star = menu.getByRole('button', { name: 'Bookmark' });
  const bookmarked = lib.bookmarks;
  const marked = (url) => JSON.stringify(bookmarked()).includes(url);
  const starredUrl = (await active()).url;
  await star.tap();
  const added = ui.getByRole('dialog', { name: 'Bookmarked' });
  check('Bookmark bookmarks the page, and its sheet takes the menu\'s place', await until(async () => (await added.isVisible()) && !(await menu.isVisible()) && marked(starredUrl)), JSON.stringify(bookmarked()));
  await shots('bookmark-sheet');
  await added.getByRole('textbox', { name: 'Name' }).fill('Docs page');
  await added.getByRole('button', { name: 'Done' }).tap();
  check('Done saves the name and closes it', await until(async () => !(await added.isVisible()) && JSON.stringify(bookmarked()).includes('Docs page')));
  await menuButton.tap();
  await until(() => menu.isVisible());
  const starOn = menu.getByRole('button', { name: 'Edit bookmark' });
  // Its name says it is on: aria-pressed would make it a ToggleButton on Android, which
  // carries no name there (docs/fragility.md, "The phone's star and folders with no names").
  check('the menu\'s star is filled, and named Edit bookmark', await until(async () => (await starOn.isVisible()) && (await starOn.getAttribute('aria-pressed')) === null));
  await shots('menu-bookmarked');
  await starOn.tap();
  const editing = ui.getByRole('dialog', { name: 'Edit bookmark' });
  await until(() => editing.isVisible());
  await editing.getByRole('button', { name: 'Remove' }).tap();
  check('a second tap shows it to edit, and Remove takes the bookmark away', await until(async () => !(await editing.isVisible()) && !marked(starredUrl)));
  // The tiles run Go's actions: History and Bookmarks open those pages of ours, in tabs.
  for (const [name, kind] of [['History', 'history'], ['Bookmarks', 'bookmarks']]) {
    if (!(await menu.isVisible())) {
      await menuButton.tap();
      await until(() => menu.isVisible());
    }
    await menuItem(name).tap();
    const ours = async () => (await regions()).tabs.find((t) => t.page === kind);
    check(`the ${name} tile opens the ${name} page, a tab of ours, shown`, await until(async () => (await ours())?.id === (await regions()).active), JSON.stringify((await regions()).tabs.map((t) => [t.id, t.page, t.url])));
    await ui.getByRole('button', { name: `Close ${name}` }).tap();
    await until(async () => !(await ours()));
  }
  await until(async () => (await active()).url === starredUrl);
  await menuButton.tap();
  await until(() => menu.isVisible());
  await page.keyboard.press('Escape');
  check('Escape closes it', await until(async () => !(await menu.isVisible())));
  await menuButton.tap();
  await menu.getByRole('button', { name: 'Back' }).tap();
  check('Back in the menu goes back, and closes it', await until(async () => (await active()).url === 'https://example.org/release-notes' && !(await menu.isVisible())));
  await menuButton.tap();
  await menu.getByRole('button', { name: 'New Tab' }).tap();
  await tabCount(2);
  check('New tab opens an empty tab with the new tab page, the keyboard down', await until(async () => (await active()).url === '' && await newTabPage()));
  check('the count says 2', (await tabsButton.innerText()) === '2');
  await visit('example.net/mail', 'https://example.net/mail');

  // The bar tapped with nothing typed lists what one more tap reaches, Go's list for nothing
  // typed, the closest nearest the thumb. What the shell sends and Go's answers are
  // watched in the shell's frame, each sent message with whether the switcher showed, and
  // a whole answer is handed to it as Go would send one.
  await w.shell().evaluate(() => {
    const send = window.chrome.send;
    window.sent = [];
    window.chrome.send = (channel, args) => {
      const m = JSON.parse(args[0]);
      window.sent.push({ t: m.t, switcher: !document.querySelector('.switcher')?.hidden });
      if (m.t === 'suggest') window.lastSuggest = m;
      return send.call(window.chrome, channel, args);
    };
    const receive = window.__shellReceive;
    window.__shellReceive = (json) => {
      const m = JSON.parse(json);
      if (m.t === 'suggestions') window.lastAnswer = m;
      receive(json);
    };
  });
  const lastSuggest = () => w.shell().evaluate(() => window.lastSuggest);
  const headingsShown = () => ui.locator('.suggestions li.heading').allInnerTexts();
  await field.tap();
  check('a tap on the bar asks Go for its list for nothing typed: open and closed tabs, top sites and bookmarks', await until(async () => {
    const m = await lastSuggest();
    return m && m.text === '' && JSON.stringify(m.kinds) === JSON.stringify(['tab', 'closed', 'history', 'bookmark']);
  }), JSON.stringify(await lastSuggest()));
  check('and lists it before anything is typed, the sites visited under Frequently visited', await until(async () => (await suggestions.count()) >= 1 && (await headingsShown()).includes('Frequently visited')), JSON.stringify(await headingsShown()));
  const answer = await w.shell().evaluate(() => window.lastAnswer);
  check('Go\'s first row nearest the thumb', (await suggestions.last().locator('.title').innerText()) === answer.items[0].title, JSON.stringify(answer.items[0]));
  // Go's whole list, as the suggestions' contract has it: the open tabs (the active one left
  // out), the tabs closed lately, the sites visited most and the newest bookmarks. Closed
  // rows come from Go once the suggestions' branch lands; until then this one is handed in.
  const before4 = await regions();
  const other = before4.tabs.find((t) => t.id !== before4.active);
  await w.shell().evaluate(({ id, other }) => window.__shellReceive(JSON.stringify({ t: 'suggestions', id, items: [
    { kind: 'tab', title: other.title || other.url, detail: '', url: other.url, tab: other.id },
    { kind: 'closed', title: 'A page closed lately', detail: 'closed.example', url: 'https://closed.example/page' },
    { kind: 'history', title: 'Example Domain', detail: 'example.com', url: 'https://example.com/' },
    { kind: 'history', title: 'Example Org', detail: 'example.org', url: 'https://example.org/' },
    { kind: 'bookmark', title: 'Release notes', detail: 'example.org', url: 'https://example.org/release-notes' },
  ] })), { id: (await lastSuggest()).id, other });
  check('each kind under its heading, Open tabs nearest the thumb and Bookmarks furthest', await until(async () => JSON.stringify(await headingsShown()) === JSON.stringify(['Bookmarks', 'Frequently visited', 'Recently closed', 'Open tabs'])), JSON.stringify(await headingsShown()));
  check('the open tab the row nearest the field', (await suggestions.last().locator('.url').innerText()) === other.url.replace(/^https?:\/\/(www\.)?/, ''));
  check('only a page or a bookmark has ↖, not a tab', (await suggestions.last().getByRole('button', { name: /^Edit / }).count()) === 0 && (await suggestions.first().getByRole('button', { name: /^Edit / }).count()) === 1);
  const listBox = await box('.suggestions');
  check('the list lies over the field', listBox.bottom <= (await field.boundingBox()).y, JSON.stringify(listBox));
  await shots('empty-list');
  await suggestions.last().getByRole('button').first().tap();
  check('a tap on an open tab\'s row shows that tab, and the list goes', await until(async () => (await regions()).active === other.id && (await suggestions.count()) === 0));
  // Enter goes to what is typed, never to a row: keys and Enter sent in one go, before Go
  // can answer the new text, as a fast typist's do, with the list for nothing typed on screen.
  await field.tap();
  await until(async () => (await headingsShown()).length > 0);
  await field.evaluate((el) => {
    el.select();
    document.execCommand('insertText', false, 'example.com/typed-fast');
    el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
  });
  check('typing and Enter before Go answers goes to what was typed, not a row of the list for nothing typed', await until(async () => (await active()).url === 'https://example.com/typed-fast' && !(await active()).loading), JSON.stringify(await active()));
  // Over a new tab, whose page shows the sites and bookmarks, the bar lists the tabs alone.
  await fire('menu', 'tab.new');
  await until(newTabPage);
  await field.tap();
  check('on a new tab it asks for the open and closed tabs alone, which its page doesn\'t show', await until(async () => JSON.stringify((await lastSuggest())?.kinds) === JSON.stringify(['tab', 'closed'])), JSON.stringify(await lastSuggest()));
  await page.keyboard.type('example');
  check('typing replaces the list with the usual one, without headings', await until(async () => (await suggestions.count()) >= 1 && (await headingsShown()).length === 0));
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.press('Backspace');
  check('and emptying the field asks for the list for nothing typed again', await until(async () => (await lastSuggest()).text === ''));
  await page.keyboard.press('Escape');
  await lib.closeTab((await regions()).active);
  await tabCount(2);
  await fire('menu', 'tab.select2');
  await until(async () => (await active()).url === 'https://example.net/mail');

  // The switcher.
  await tabsButton.tap();
  // Once it has slid in.
  check('the count opens the switcher over the page, the toolbar still under it', await until(async () => await switcher.isVisible() && (await box('.switcher')).bottom === H - 48), JSON.stringify(await box('.switcher')));
  const captures = (await w.shell().evaluate(() => window.sent)).filter((m) => m.t === 'captureTab');
  check('opening, it asks Go for a picture of the page first, before it covers it', captures.length === 1 && captures[0].switcher === false, JSON.stringify(captures));
  check('a card per tab, the active one selected', (await cards.count()) === 2 && (await cards.nth(1).getAttribute('aria-selected')) === 'true');
  const card = await cards.first().boundingBox();
  check('two columns', card.width > 150 && card.width < W / 2, JSON.stringify(card));
  await shots('switcher');
  // A search over the tabs: with three or fewer, behind a search button.
  const searchField = switcher.getByRole('textbox', { name: 'Search tabs' });
  check('with two tabs the search waits behind a button', (await searchField.count()) === 0);
  await switcher.getByRole('button', { name: 'Search tabs' }).tap();
  check('which shows the field, the keyboard in it', await until(async () => (await searchField.isVisible()) && await searchField.evaluate((el) => el === document.activeElement)));
  await page.keyboard.type('MAIL');
  check('typing keeps the cards whose title or address holds it, any case', await until(async () => (await cards.filter({ visible: true }).count()) === 1 && /example\.net/.test(await cards.filter({ visible: true }).first().innerText())), await switcher.innerText());
  await shots('switcher-search');
  await page.keyboard.type('zzz');
  check('none found says so', await until(() => switcher.getByText('No tabs found').isVisible()));
  await switcher.getByRole('button', { name: 'Clear search' }).tap();
  check('its × clears it, every card back, and on a short list puts the field away', await until(async () => (await cards.filter({ visible: true }).count()) === 2 && (await searchField.count()) === 0));
  // Page pictures: a card shows tab.thumb, Go's picture of its page, cropped from the top
  // of a phone-shaped place; one without shows its favicon or letter. The engine's pictures
  // come on their own branch; here one is handed to the shell as Go sends a tab.
  const pictured = (await regions()).tabs.find((t) => t.url === 'https://example.net/mail');
  const thumb = 'data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="412" height="1200"><rect width="412" height="1200" fill="#fff"/><rect width="412" height="96" fill="#24292f"/><text x="24" y="60" font-family="sans-serif" font-size="32" fill="#fff">example.net</text><text x="24" y="190" font-family="sans-serif" font-size="40" font-weight="700" fill="#111">Mail</text><rect x="24" y="230" width="364" height="18" fill="#ddd"/><rect x="24" y="266" width="260" height="18" fill="#ddd"/><rect x="24" y="320" width="160" height="56" rx="8" fill="#1f883d"/><rect y="1100" width="412" height="100" fill="#d33"/></svg>');
  await w.shell().evaluate(({ tab, thumb }) => window.__shellReceive(JSON.stringify({ t: 'tab', tab: { ...tab, thumb } })), { tab: pictured, thumb });
  const picture = switcher.locator(`[data-tab="${pictured.id}"] img.thumb`);
  check('a card with a picture shows it', await until(() => picture.isVisible()));
  const pic = await picture.evaluate((img) => ({ fit: getComputedStyle(img).objectFit, at: getComputedStyle(img).objectPosition, box: img.getBoundingClientRect().toJSON(), place: img.parentElement.getBoundingClientRect().toJSON() }));
  check('filling an upright, phone-shaped place, cropped from the top', pic.fit === 'cover' && pic.at.startsWith('50% 0') && Math.abs(pic.place.width / pic.place.height - 3 / 4) < 0.02 && Math.abs(pic.box.height - pic.place.height) < 1, JSON.stringify(pic));
  // The other tab as Go sends one with no picture yet.
  const { thumb: _, ...unpictured } = (await regions()).tabs.find((t) => t.id !== pictured.id);
  await w.shell().evaluate((tab) => window.__shellReceive(JSON.stringify({ t: 'tab', tab })), unpictured);
  const plain = switcher.locator(`.card[data-tab="${unpictured.id}"]`);
  check('one without a picture keeps its favicon or letter', await until(async () => (await plain.locator('img.thumb').count()) === 0 && (await plain.locator('.body .letter, .body img').count()) === 1));
  await shots('switcher-thumb');
  const first = (await regions()).tabs[0].id;
  await cards.first().tap();
  check('a tap on a card shows its tab and closes the switcher', await until(async () => (await regions()).active === first && !(await switcher.isVisible())));
  await tabsButton.tap();
  await cards.nth(1).getByRole('button', { name: /^Close / }).tap();
  check('its × closes that tab and keeps the switcher open', await tabCount(1) && await switcher.isVisible());
  // A window always has a tab: New Tab opens an empty one, with the new tab page, and
  // closing the last tab closes the window (the contract's rule 18).
  const old = (await regions()).tabs[0].id;
  await switcher.getByRole('button', { name: 'New Tab' }).tap();
  check('the switcher\'s New Tab opens an empty tab with the new tab page', await until(async () => (await regions()).tabs.length === 2 && (await active()).url === '' && await newTabPage()));
  check('and shuts the switcher', await until(async () => !(await switcher.isVisible())));
  await fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'closeTab', id: old }) });
  await tabCount(1);

  // A swipe along the bar goes to the tab after or before.
  await visit('example.com/one', 'https://example.com/one');
  await fire('menu', 'tab.new');
  await tabCount(2);
  await visit('example.com/two', 'https://example.com/two');
  const [one, two] = (await regions()).tabs.map((t) => t.id);
  const swipe = async (from, to) => {
    const y = H - 24;
    await page.mouse.move(from, y);
    await page.mouse.down();
    await page.mouse.move((from + to) / 2, y);
    await page.mouse.move(to, y);
    await page.mouse.up();
  };
  await swipe(150, 330);
  check('a swipe right along the bar goes to the tab before', await until(async () => (await regions()).active === one));
  await swipe(330, 150);
  check('a swipe left goes to the tab after', await until(async () => (await regions()).active === two));
  check('and leaves the field alone', !(await inField()));

  // Workspaces: chips in the switcher, and where new tabs open.
  await tabsButton.tap();
  const chips = switcher.getByRole('radio');
  check('the switcher has a chip per workspace, Main first', await until(async () => (await chips.allInnerTexts()).join() === 'Main,Work'), await switcher.locator('.head').innerHTML());
  await chips.nth(1).tap();
  check('Work shows none of Main\'s cards', await until(async () => (await cards.filter({ visible: true }).count()) === 0));
  await switcher.getByRole('button', { name: 'New Tab' }).tap();
  check('+ there opens a tab in Work', await until(async () => (await regions()).tabs.length === 3 && (await active()).workspace === '0a1b2c3d'), JSON.stringify(await active()));
  await visit('example.com/work', 'https://example.com/work');
  await tabsButton.tap();
  check('the switcher opens on the active tab\'s workspace', await until(async () => (await chips.nth(1).getAttribute('aria-checked')) === 'true' && (await cards.filter({ visible: true }).count()) === 1));
  await page.keyboard.press('Escape');

  // Groups. A long press on a card opens its menu; New group makes a group of its tab and a
  // new one, which starts in edit mode.
  const strip = ui.getByRole('group', { name: /^Group / });
  const stripTabs = strip.locator('button[data-tab]');
  const groupSheet = ui.getByRole('dialog', { name: 'Group' });
  const cardMenu = ui.getByRole('dialog', { name: 'Tab' });
  await tabsButton.tap();
  await chips.first().tap();
  await until(async () => (await cards.filter({ visible: true }).count()) === 2);
  const [g1] = (await regions()).tabs.map((t) => t.id);
  await cards.filter({ visible: true }).first().click({ button: 'right' });
  check('a long press on a card opens its menu', await until(() => cardMenu.isVisible()));
  await cardMenu.getByRole('menuitem', { name: 'Add to group' }).tap();
  await cardMenu.getByRole('menuitem', { name: 'New group' }).tap();
  check('New group opens an empty tab with the new tab page', await until(async () => (await regions()).tabs.length === 4 && (await active()).url === '' && await newTabPage()));
  check('and the strip shows the group, the tab and the new one', await until(async () => (await strip.isVisible()) && (await stripTabs.count()) === 2));
  const stripBox = await box('[role=group][aria-label^="Group "]');
  check('the strip is 49px with its hairline, just over the toolbar', Math.round(stripBox.height) === 49 && Math.round(stripBox.bottom) === H - 48, JSON.stringify(stripBox));
  check('the page ends over the strip', await until(async () => {
    const v = (await regions()).visible[0].rect;
    return v.y + v.h === H - 48 - 49;
  }), JSON.stringify((await regions()).visible));
  await visit('example.com/group/two', 'https://example.com/group/two');
  const g2 = (await regions()).active;
  await strip.getByRole('button', { name: 'New tab in group' }).tap();
  check('+ in the strip opens a third tab in the group', await until(async () => (await stripTabs.count()) === 3 && (await active()).url === ''));
  await visit('example.com/group/three', 'https://example.com/group/three');
  const g3 = (await regions()).active;
  check('the strip lists the group\'s tabs in order', JSON.stringify(await stripTabs.evaluateAll((els) => els.map((e) => Number(e.dataset.tab)))) === JSON.stringify([g1, g2, g3]));
  await shots('strip');
  await stripTabs.first().tap();
  check('a tap on a favicon in the strip goes to its tab', await until(async () => (await regions()).active === g1));
  await swipe(330, 150);
  check('a swipe along the bar stays in the group', await until(async () => (await regions()).active === g2));

  // The group sheet: rename, colour.
  await strip.getByRole('button', { name: /^Group / }).tap();
  check('the group\'s dot opens its sheet', await until(() => groupSheet.isVisible()));
  const nameField = groupSheet.getByRole('textbox', { name: 'Group name' });
  await nameField.tap();
  await page.keyboard.type('Release');
  await page.keyboard.press('Enter');
  check('a name and Enter rename the group', await until(async () => (await strip.getAttribute('aria-label')) === 'Group Release'));
  check('the sheet offers gulal\'s seven hues, then a +', (await groupSheet.getByRole('radio').count()) === 7 && await groupSheet.getByRole('button', { name: 'Custom colour' }).isVisible());
  await groupSheet.getByRole('radio', { name: 'Indigo' }).tap();
  check('a swatch gives it that colour', await until(async () => (await groupSheet.getByRole('radio', { name: 'Indigo' }).getAttribute('aria-checked')) === 'true'));
  await shots('group');

  // + opens our Custom colour sheet, not the system's picker (docs/design-system.md, On a
  // phone). Each change shows at once; Done, the back key or a tap outside keep it.
  const custom = ui.getByRole('dialog', { name: 'Custom colour' });
  const hueSlider = custom.getByRole('slider', { name: 'Hue' });
  const hexField = custom.getByRole('textbox', { name: 'Hex' });
  const groupDot = () => groupSheet.locator('.head .dot').evaluate((e) => getComputedStyle(e).backgroundColor);
  const rgbOf = (hex) => `rgb(${[1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16)).join(', ')})`;
  const plus = groupSheet.getByRole('button', { name: 'Custom colour' });
  await plus.tap();
  check('+ opens the Custom colour sheet over the group\'s', await until(() => custom.isVisible()) && await groupSheet.isVisible());
  check('it starts on the group\'s hue at its middle shade, of five', (await hueSlider.inputValue()) === '270' &&
    (await custom.getByRole('radio').count()) === 5 && (await custom.getByRole('radio', { name: 'Middle' }).getAttribute('aria-checked')) === 'true');
  check('its preview is the group, its dot and its chip', (await custom.locator('.ui-colour-name').innerText()) === 'Release' && (await custom.locator('.ui-colour-chip').innerText()) === 'Release');
  const hueBox = await hueSlider.boundingBox();
  check('Hue is a full-width slider in a 48px target', hueBox.height === 48 && hueBox.width >= W - 32 - 1, JSON.stringify(hueBox));
  check('Shade\'s swatches are 40px in 48px targets', (await custom.getByRole('radio', { name: 'Deep', exact: true }).evaluate((b) => [b.getBoundingClientRect().width, getComputedStyle(b, '::after').width].join())) === '48,40px');
  await shots('custom-colour');
  await custom.getByRole('radio', { name: 'Deep', exact: true }).tap();
  const deep = await hexField.inputValue();
  check('a shade shows at once: the group takes it as it is', /^#[0-9a-f]{6}$/.test(deep) && await until(async () => (await groupDot()) === rgbOf(deep)), deep);
  await hueSlider.fill('120');
  check('moving the hue shows at once too', await until(async () => (await hexField.inputValue()) !== deep && (await groupDot()) === rgbOf(await hexField.inputValue())));
  await hexField.fill('#c2410c');
  check('a hex typed is taken as it is, and no shade is chosen', await until(async () => (await groupDot()) === 'rgb(194, 65, 12)') &&
    (await custom.locator('[aria-checked=true]').count()) === 0);
  await settle(page);
  await fire('backKey');
  check('the back key closes it, keeping the colour, and the group\'s sheet stays', await until(async () => !(await custom.isVisible())) && await groupSheet.isVisible() &&
    (await groupDot()) === 'rgb(194, 65, 12)' && (await groupSheet.getByRole('radio', { checked: true }).count()) === 0);
  check('the + stays on the colour picked', (await plus.locator('i').evaluate((e) => getComputedStyle(e).color)) !== 'rgba(0, 0, 0, 0)' && (await plus.innerText()) === '+');
  await shots('group-custom');
  await plus.tap();
  check('opened again, it starts on the colour picked', await until(() => custom.isVisible()) && (await hexField.inputValue()) === '#c2410c');
  await page.touchscreen.tap(W / 2, 40);
  check('a tap outside closes it', await until(async () => !(await custom.isVisible())) && await groupSheet.isVisible());
  await plus.tap();
  await until(() => custom.isVisible());
  await custom.getByRole('button', { name: 'Done' }).tap();
  check('and so does Done', await until(async () => !(await custom.isVisible())) && await groupSheet.isVisible());
  await groupSheet.getByRole('radio', { name: 'Indigo' }).tap();
  check('a swatch again takes the hue back', await until(async () => (await groupSheet.getByRole('radio', { name: 'Indigo' }).getAttribute('aria-checked')) === 'true'));
  await page.keyboard.press('Escape');

  // The switcher shows the group as one card.
  await tabsButton.tap();
  const groupCard = switcher.locator('[data-group]');
  check('the switcher shows the group as one card, its tabs\' favicons in it', await until(async () => (await groupCard.count()) === 1 && (await groupCard.getByRole('tab').count()) === 3));
  check('named, and the ungrouped tab beside it', (await groupCard.locator('.title').innerText()) === 'Release' && (await switcher.locator('.card:not([data-group]):visible').count()) === 1);
  await shots('switcher-group');
  await groupCard.locator('.title').tap();
  check('a tap on the group card opens its sheet', await until(() => groupSheet.isVisible()));
  await page.keyboard.press('Escape');

  // Reloaded, the shell draws the groups Go keeps.
  await w.shell().evaluate(() => location.reload());
  check('reloaded, the group comes back from Go', await until(async () => (await strip.getAttribute('aria-label').catch(() => '')) === 'Group Release' && (await stripTabs.count()) === 3, 10000));

  // The strip's × closes the active tab and goes to its neighbour in the group.
  await until(async () => (await regions()).active === g2);
  await strip.getByRole('button', { name: /^Close / }).tap();
  check('a tap on the active favicon in the strip closes it, and the strip goes to the next in the group', await until(async () => (await regions()).active === g3 && (await stripTabs.count()) === 2));
  await stripTabs.last().click({ button: 'right' });
  check('a long press on a favicon closes its tab, and a group of one ends', await until(async () => (await regions()).tabs.length === 3 && !(await strip.isVisible())));

  // Add to group, Remove from group, and Close group with its confirm.
  await tabsButton.tap();
  await until(async () => (await cards.filter({ visible: true }).count()) === 2);
  await cards.filter({ visible: true }).nth(1).click({ button: 'right' });
  await cardMenu.getByRole('menuitem', { name: 'Add to group' }).tap();
  await cardMenu.getByRole('menuitem', { name: 'New group' }).tap();
  await until(async () => (await regions()).tabs.length === 3);
  await visit('example.com/again', 'https://example.com/again');
  await tabsButton.tap();
  const loose = switcher.locator('.card:not([data-group])').filter({ visible: true });
  await until(async () => (await loose.count()) === 1);
  await loose.first().click({ button: 'right' });
  await cardMenu.getByRole('menuitem', { name: 'Add to group' }).tap();
  await cardMenu.getByRole('menuitem', { name: '2 tabs' }).tap();
  check('Add to group puts a card\'s tab in that group', await until(async () => (await groupCard.getByRole('tab').count()) === 3 && (await loose.count()) === 0));
  await groupCard.getByRole('tab').first().click({ button: 'right' });
  await cardMenu.getByRole('menuitem', { name: 'Remove from group' }).tap();
  check('Remove from group takes it out', await until(async () => (await groupCard.getByRole('tab').count()) === 2 && (await loose.count()) === 1));
  await groupCard.getByRole('button', { name: /^Close group/ }).tap();
  const confirm = ui.getByRole('dialog', { name: 'Close group' });
  check('Close group asks first: Close 2 tabs?', await until(async () => (await confirm.isVisible()) && (await confirm.locator('h2').innerText()) === 'Close 2 tabs?'));
  const before = (await regions()).tabs.length;
  await confirm.getByRole('button', { name: 'Close', exact: true }).tap();
  check('and Close closes them', await until(async () => (await regions()).tabs.length === before - 2), String((await regions()).tabs.length));

  // Slice 3: a swipe sideways closes a card; a card held and dragged onto another makes a
  // group of the two.
  const swipeCard = async (card, by) => {
    const b = await card.boundingBox();
    const y = b.y + b.height / 2;
    await page.mouse.move(b.x + b.width / 2, y);
    await page.mouse.down();
    for (let i = 1; i <= 6; i++) await page.mouse.move(b.x + b.width / 2 + (by * i) / 6, y);
    await page.mouse.up();
  };
  const dragCard = async (from, to) => {
    const a = await from.boundingBox();
    const b = await to.boundingBox();
    await page.mouse.move(a.x + a.width / 2, a.y + a.height / 2);
    await page.mouse.down();
    await page.waitForTimeout(500);
    for (let i = 1; i <= 8; i++) await page.mouse.move(a.x + a.width / 2 + ((b.x - a.x) * i) / 8, a.y + a.height / 2 + ((b.y - a.y) * i) / 8);
    await page.mouse.up();
  };
  for (let i = (await regions()).tabs.length; i < 4; i++) await fire('menu', 'tab.new');
  await tabCount(4);
  for (const t of (await regions()).tabs) if (!t.url) await fire('navigate', `https://example.com/card/${t.id}`, { tab: t.id });
  await until(async () => (await regions()).tabs.every((t) => t.url && !t.loading));
  // With four tabs or more in the workspace, the search field is there at once.
  const extra = (await regions()).tabs.length;
  await fire('menu', 'tab.new');
  await tabCount(extra + 1);
  const extraTab = (await regions()).active;
  await fire('navigate', 'https://example.com/card/extra', { tab: extraTab });
  await until(async () => !(await active()).loading && (await active()).url !== '');
  await tabsButton.tap();
  await until(() => switcher.isVisible());
  // The shown workspace's tabs: each card's, and each in a group's card.
  const shownTabs = await w.shell().evaluate(() => [...document.querySelectorAll('.switcher [data-tab]')].filter((el) => !el.closest('.card').hidden).length);
  check(`with ${shownTabs} tabs, more than three, the search field is there at once`, shownTabs > 3 && (await switcher.getByRole('textbox', { name: 'Search tabs' }).isVisible()) && (await switcher.getByRole('button', { name: 'Search tabs' }).count()) === 0, String(shownTabs));
  await page.keyboard.press('Escape');
  await lib.closeTab(extraTab);
  await tabCount(extra);
  await tabsButton.tap();
  const visibleCards = switcher.locator('.card:not([data-group])').filter({ visible: true });
  await until(async () => (await visibleCards.count()) >= 3 && (await box('.switcher')).bottom === H - 48);
  // A card other than the active tab's, whose picture Go may be making as the switcher
  // opens, once its own picture, if it has one, has loaded: a tab closed while its picture
  // is on its way has none to load.
  const doomedCard = switcher.locator('.card:not([data-group]):not(.active)').filter({ visible: true }).last();
  const doomed = Number(await doomedCard.getAttribute('data-tab'));
  await until(() => doomedCard.locator('img.thumb').evaluateAll((els) => els.every((e) => e.complete)));
  await swipeCard(doomedCard, -170);
  check('a swipe sideways closes a card', await until(async () => !(await regions()).tabs.some((t) => t.id === doomed)) && await switcher.isVisible(), JSON.stringify({ doomed, tabs: (await regions()).tabs.map((t) => t.id), open: await switcher.isVisible() }));
  const small = Number(await visibleCards.nth(0).getAttribute('data-tab'));
  await swipeCard(visibleCards.nth(0), 40);
  check('a short one puts it back, and does not open it', await until(async () => (await regions()).tabs.some((t) => t.id === small)) && await switcher.isVisible());
  const [x, y2] = [Number(await visibleCards.nth(0).getAttribute('data-tab')), Number(await visibleCards.nth(1).getAttribute('data-tab'))];
  await dragCard(visibleCards.nth(0), visibleCards.nth(1));
  check('a card held and dropped on another makes a group of the two', await until(async () => (await switcher.locator('[data-group]').filter({ visible: true }).count()) === 1 &&
    (await switcher.locator('[data-group] [role=tab]').evaluateAll((els) => els.map((e) => Number(e.dataset.tab)).sort().join())) === [x, y2].sort().join()));
  await page.keyboard.press('Escape');

  // Slice 6: the bars slide away while the page scrolls down, and give the page their room;
  // they come back when it scrolls up, or for another tab.
  const pageRect = async () => (await regions()).visible[0].rect;
  const act = (await regions()).active;
  // With the bars: down to the toolbar, or to the strip while the tab is in a group.
  const foot = async () => H - 48 - ((await strip.isVisible()) ? 49 : 0);
  await until(async () => (await pageRect()).y + (await pageRect()).h === await foot());
  const withBars = await pageRect();
  // The shell lets go of a direction while the bars' own resize settles (data-settling on
  // the window); a person's scroll comes after it.
  const settled = () => w.shell().evaluate(() => !document.querySelector('.window[data-settling]'));
  const scroll = async (direction) => {
    await until(settled);
    await fire('scroll', direction, { tab: act });
  };
  // The page's room changes only while the bars cover it. The shell's frame notes each
  // change of the toolbar's classes as it happens, with where the page's part ends then and
  // whether the toolbar is still moving.
  await w.shell().evaluate(() => {
    const foot = document.querySelector('.bars.foot');
    const state = () => ({ over: foot.classList.contains('over'), away: foot.classList.contains('away') });
    let was = state();
    window.barsSeen = [];
    new MutationObserver(() => {
      const now = state();
      if (now.over === was.over && now.away === was.away) return;
      was = now;
      window.barsSeen.push({ ...now, pageFoot: Math.round(document.querySelector('main').getBoundingClientRect().bottom), moving: foot.getAnimations().length > 0, transform: getComputedStyle(foot).transform });
    }).observe(foot, { attributes: true, attributeFilter: ['class'] });
    // Waits for the bars to settle where the shell last asked for them (data-bars), the
    // resize's pause over and nothing moving, and says how they are; in 5 s at most, saying
    // what holds them.
    window.barsSettled = () => new Promise((resolve) => {
      const until = Date.now() + 5000;
      const look = () => {
        const win = document.querySelector('.window');
        const settling = win.hasAttribute('data-settling');
        const moving = foot.getAnimations().length > 0;
        const hidden = win.dataset.bars === 'hidden';
        const there = state().over === hidden && state().away === hidden;
        if (!settling && !moving && there) resolve(state());
        else if (Date.now() > until) resolve({ ...state(), stuck: { settling, moving, asked: win.dataset.bars } });
        else setTimeout(look, 16);
      };
      setTimeout(look, 50);
    });
  });
  // The changes seen, once there are two: the room and the slide, one way.
  const seen = async () => {
    await until(async () => (await w.shell().evaluate(() => window.barsSeen.length)) >= 2);
    return w.shell().evaluate(() => window.barsSeen.splice(0));
  };
  await scroll('down');
  check('scrolling down, the bars go and the page takes the window to its foot', await until(async () => {
    const v = await pageRect();
    return v.y === withBars.y && v.y + v.h === H;
  }), JSON.stringify(await pageRect()));
  const going = await seen();
  check('going, the page takes their room under them first, and only then do they slide off it',
    going.length === 2 && going[0].over && !going[0].away && going[0].pageFoot === H && going[1].over && going[1].away, JSON.stringify(going));
  await shots('scrolled');
  await scroll('up');
  check('scrolling up, they come back', await until(async () => JSON.stringify(await pageRect()) === JSON.stringify(withBars) && (await box('.toolbar')).bottom === H), JSON.stringify({ now: await pageRect(), withBars, bar: await box('.toolbar') }));
  const coming = await seen();
  check('coming back, they slide in over the page, and it gives their room back only once they are in',
    coming.length === 2 && coming[0].over && !coming[0].away && !coming[1].over && !coming[1].away && !coming[1].moving && coming[1].transform === 'none', JSON.stringify(coming));
  // In the shell's frame: wait for the bars to settle, the resize's pause over and nothing
  // moving, and say how they are.
  // Content's report of the resize itself comes at once: the shell's frame hands it in, as
  // Go would send it, the moment the page takes the bars' room or gives it back.
  const echo = (over, direction) => w.shell().evaluate(({ over, direction, tab }) => {
    window.barsAnswer = new Promise((resolve) => {
      const foot = document.querySelector('.bars.foot');
      const seen = new MutationObserver(() => {
        if (foot.classList.contains('over') !== over) return;
        seen.disconnect();
        window.__shellReceive(JSON.stringify({ t: 'scroll', tab, direction }));
        resolve(window.barsSettled());
      });
      seen.observe(foot, { attributes: true, attributeFilter: ['class'] });
      setTimeout(() => resolve({ never: over ? 'took the room' : 'gave it back' }), 5000);
    });
  }, { over, direction, tab: act });
  const barsAnswer = () => w.shell().evaluate(() => window.barsAnswer);
  await until(settled);
  await echo(true, 'up');
  await fire('scroll', 'down', { tab: act });
  const afterUp = await barsAnswer();
  check('scrolling up just as the page took their room is the resize\'s own, and they stay away', afterUp.over && afterUp.away, JSON.stringify(afterUp));
  await until(settled);
  await echo(false, 'down');
  await fire('scroll', 'up', { tab: act });
  const afterDown = await barsAnswer();
  check('and scrolling down just as it gave the room back leaves them there', !afterDown.over && !afterDown.away, JSON.stringify(afterDown));
  // A change of mind mid-slide turns the bars round where they are. The shell's frame asks
  // for the other way as the slide starts, and notes every change on the way.
  const turn = (first, then) => w.shell().evaluate(({ first, then }) => new Promise((resolve) => {
    const foot = document.querySelector('.bars.foot');
    const changes = [];
    const away = first.t === 'scroll' && first.direction === 'down';
    const watch = new MutationObserver(() => {
      changes.push({ over: foot.classList.contains('over'), away: foot.classList.contains('away') });
    });
    watch.observe(foot, { attributes: true, attributeFilter: ['class'] });
    const started = new MutationObserver(() => {
      if (foot.classList.contains('away') !== away) return;
      started.disconnect();
      window.__shellReceive(JSON.stringify(then));
      window.barsSettled().then((end) => {
        watch.disconnect();
        resolve({ changes, end });
      });
    });
    started.observe(foot, { attributes: true, attributeFilter: ['class'] });
    setTimeout(() => resolve({ never: 'started', changes }), 5000);
    window.__shellReceive(JSON.stringify(first));
  }), { first, then });
  const barsSettled = () => w.shell().evaluate(() => window.barsSettled());
  await barsSettled();
  const outThenIn = await turn({ t: 'scroll', tab: act, direction: 'down' }, { t: 'focusAddressBar' });
  check('called back as they slide off, they come back from where they are, and the page gives their room back once they are in',
    outThenIn.end && !outThenIn.end.over && !outThenIn.end.away && outThenIn.changes.at(-1).over === false && outThenIn.changes.at(-2)?.away === false, JSON.stringify(outThenIn));
  await page.keyboard.press('Escape');
  await until(async () => !(await inField()) && JSON.stringify(await pageRect()) === JSON.stringify(withBars));
  await scroll('down');
  check('with the address left, scrolling down sends them away once more', await until(async () => (await pageRect()).h > withBars.h), JSON.stringify(await pageRect()));
  await barsSettled();
  const inThenOut = await turn({ t: 'scroll', tab: act, direction: 'up' }, { t: 'scroll', tab: act, direction: 'down' });
  check('scrolled down as they slide in, they go again from where they are, and the page keeps their room all along',
    inThenOut.end?.over && inThenOut.end.away && inThenOut.changes.every((c) => c.over), JSON.stringify(inThenOut));
  await w.shell().evaluate((tab) => window.__shellReceive(JSON.stringify({ t: 'scroll', tab, direction: 'up' })), act);
  await until(async () => JSON.stringify(await pageRect()) === JSON.stringify(withBars));
  await scroll('down');
  check('scrolling down after that sends them away again', await until(async () => (await pageRect()).h > withBars.h));
  await fire('navigate', 'https://example.com/scrolled/next', { tab: act });
  check('a page that starts loading brings them back', await until(async () => (await box('.toolbar')).bottom === H && JSON.stringify(await pageRect()) === JSON.stringify(withBars)));
  await until(async () => !(await active()).loading);
  await scroll('down');
  check('and down again, they go', await until(async () => (await pageRect()).h > withBars.h));
  await fire('key', 'focusAddressBar', { tab: act });
  check('the address bar called for brings them back', await until(async () => (await box('.toolbar')).bottom === H && await inField()));
  await page.keyboard.press('Escape');

  // The phone's back key closes what the shell has open, then goes back in the page.
  // A person's press comes a moment after the sheet opens, once the shell has told Go.
  const backKey = async () => {
    await settle(page);
    await fire('backKey');
  };
  await tabsButton.tap();
  await until(() => switcher.isVisible());
  await backKey();
  check('the back key closes the switcher', await until(async () => !(await switcher.isVisible())));
  await menuButton.tap();
  await until(() => menu.isVisible());
  await backKey();
  check('and the menu', await until(async () => !(await menu.isVisible())));
  await field.tap();
  await until(inField);
  await backKey();
  check('and ends editing the address', await until(async () => !(await inField())));
  const here = await active();
  await fire('navigate', 'https://example.com/back/one', { tab: here.id });
  await until(async () => (await active()).url === 'https://example.com/back/one' && !(await active()).loading);
  await fire('navigate', 'https://example.com/back/two', { tab: here.id });
  await until(async () => (await active()).url === 'https://example.com/back/two' && !(await active()).loading);
  await backKey();
  check('with nothing open, it goes back in the page', await until(async () => (await active()).url === 'https://example.com/back/one'));

  // Slice 5: a long press on a link in the page. The shell draws the menu, a dense sheet: a
  // row of tiles, New tab first, then In group, which opens the link behind, in the tab's
  // group, starting one when it has none.
  const linkSheet = ui.getByRole('dialog', { name: 'Link' });
  await fire('menu', 'tab.new');
  await until(inField);
  await visit('example.com/article', 'https://example.com/article');
  const reader = (await regions()).active;
  const before5 = (await regions()).tabs.length;
  await fire('contextMenu', 'https://example.com/linked/one', { tab: reader });
  check('a long press on a link opens the shell\'s menu for it', await until(() => linkSheet.isVisible()));
  const tileNames = async (group) => (await linkSheet.getByRole('group', { name: group }).getByRole('menuitem').allInnerTexts()).map((t) => t.trim());
  check('its tiles are New tab, In group, Copy link, Download and Share', JSON.stringify(await tileNames('Link')) === JSON.stringify(['New tab', 'In group', 'Copy link', 'Download', 'Share']), JSON.stringify(await tileNames('Link')));
  check('and Copy link text is a row under them', await linkSheet.getByRole('menuitem', { name: 'Copy link text' }).isVisible());
  check('a plain link\'s one row of tiles has no heading', (await linkSheet.locator('.heading').count()) === 0);
  await shots('link');
  await linkSheet.getByRole('menuitem', { name: 'In group' }).tap();
  check('it opens the link behind, in a group with this tab', await until(async () => {
    const r = await regions();
    const opened = r.tabs.find((t) => t.url === 'https://example.com/linked/one');
    return r.tabs.length === before5 + 1 && r.active === reader && opened && (await stripTabs.count()) === 2;
  }), JSON.stringify((await regions()).tabs.map((t) => [t.id, t.url])));
  await fire('contextMenu', 'https://example.com/linked/two', { tab: reader });
  await linkSheet.getByRole('menuitem', { name: 'In group' }).tap();
  check('a second joins the same group, still behind', await until(async () => (await stripTabs.count()) === 3 && (await regions()).active === reader));
  await shots('link-group');
  await stripTabs.nth(2).tap();
  check('and the strip goes to it', await until(async () => (await active()).url === 'https://example.com/linked/two'));
  await fire('contextMenu', 'https://example.com/linked/three', { tab: (await regions()).active });
  await linkSheet.getByRole('menuitem', { name: 'New tab', exact: true }).tap();
  check('New tab opens it behind too', await until(async () => (await regions()).tabs.some((t) => t.url === 'https://example.com/linked/three')) &&
    (await active()).url === 'https://example.com/linked/two');
  await fire('contextMenu', 'https://example.com/linked/four', { tab: (await regions()).active });
  await until(() => linkSheet.isVisible());
  await backKey();
  check('the back key closes the link menu', await until(async () => !(await linkSheet.isVisible())));

  // A page that fails, crashes or is blocked gets a plain notice over it, not a blank.
  const notice = ui.getByRole('alert');
  const nowTab = (await regions()).active;
  await fire('fail', 'not-found ERR_NAME_NOT_RESOLVED', { tab: nowTab });
  check('a page that fails to load says so over it, with the error and Try again', await until(async () => (await notice.isVisible()) &&
    /Can't find/.test(await notice.innerText()) && /ERR_NAME_NOT_RESOLVED/.test(await notice.innerText())), await notice.innerText().catch(() => ''));
  check('a press on the notice reaches the shell', await until(async () => {
    const b = await notice.boundingBox();
    return (await regions({ x: b.x + b.width / 2, y: b.y + 20 })).hit === 'shell';
  }));
  await shots('failed');
  await notice.getByRole('button', { name: 'Try again' }).tap();
  check('Try again loads the page again, and the notice goes', await until(async () => !(await notice.isVisible())));
  await fire('crash', '', { tab: nowTab });
  check('a page that crashed says so, with Reload', await until(async () => (await notice.isVisible()) && /crashed/.test(await notice.innerText())));
  await notice.getByRole('button', { name: 'Share report' }).tap();
  const shared = await until(async () => (await regions()).shared);
  const report = shared ? fs.readFileSync((await regions()).shared, 'utf8') : '';
  check('Share report hands the system one text report, written in the profile', /reports\/project-b-report-.*Z\.txt$/.test((await regions()).shared) &&
    report.includes('Project B on the mock engine') && report.includes("This run's timeline"), (await regions()).shared);
  await notice.getByRole('button', { name: 'Save report' }).tap();
  await until(async () => (await regions()).published);
  const saved = (await regions()).published;
  check('Save report keeps the same kind of report as a finished download, for the engine to put in the phone\'s Downloads',
    /Downloads\/project-b-report-.*Z\.txt$/.test(saved || '') && fs.readFileSync(saved, 'utf8').includes("This run's timeline"), saved);
  const savedBar = ui.getByRole('group', { name: 'Download' });
  check('and the download bar says the crash report is saved to Downloads, with Open', await until(async () =>
    (await savedBar.innerText().catch(() => '')).includes('Crash report saved to Downloads') && (await savedBar.getByRole('button', { name: 'Open' }).isVisible())),
    await savedBar.innerText().catch(() => ''));
  await notice.getByRole('button', { name: 'Reload' }).tap();
  check('Reload brings it back', await until(async () => !(await notice.isVisible()) && !(await active()).crashed));

  // The back key with nothing open: back in the page, then on a tab with no history the
  // tab closes and the one used before it shows, down to the last tab, whose closing closes
  // the window, and on a phone the app (tools/drive_android.py checks that).
  await settle(page);
  const hadTabs = (await regions()).tabs.length;
  const used = (await regions()).active;
  await fire('menu', 'tab.new');
  await until(async () => (await regions()).tabs.length === hadTabs + 1);
  await visit('example.com/fresh', 'https://example.com/fresh');
  const fresh = (await regions()).active;
  // Back to the new tab's blank page first, then the tab has no history left.
  for (let i = 0; i < 3 && (await regions()).tabs.some((t) => t.id === fresh); i++) {
    await backKey();
    await until(async () => !(await regions()).tabs.some((t) => t.id === fresh), 1500);
  }
  check('the back key on a tab with no history closes it, and the tab used before it shows', await until(async () =>
    !(await regions()).tabs.some((t) => t.id === fresh) && (await regions()).active === used), JSON.stringify({ used, now: (await regions()).active }));
  for (let i = 0; i < 40 && (await regions()).tabs.length > 1; i++) {
    const n = (await regions()).tabs.length;
    await backKey();
    await until(async () => (await regions()).tabs.length < n || (await active())?.url !== undefined, 3000);
  }
  check('back after back closes every tab but the last', await until(async () => (await regions()).tabs.length === 1 && !(await regions()).closed));
  // A page's dialog: in the middle over the dimmed window, its buttons a finger's size,
  // and the back key answers it no, as Android's dialogs go.
  const ask = ui.getByRole('alertdialog');
  const answers = async () => (await regions()).dialogAnswers || [];
  await fire('confirm', 'Leave this page? What you typed will be lost.');
  check('a page\'s confirm shows as an alertdialog', await until(() => ask.isVisible()));
  const asked = await ask.boundingBox();
  check('in the middle of the screen', Math.abs(asked.x + asked.width / 2 - W / 2) < 2 && Math.abs(asked.y + asked.height / 2 - H / 2) < 2, JSON.stringify(asked));
  await shots('dialog');
  const had = (await answers()).length;
  await backKey();
  check('the back key answers it Cancel', await until(async () => (await answers()).length === had + 1 && (await answers()).at(-1).ok === false));
  check('and it goes', await until(async () => (await ask.count()) === 0));
  await fire('signin', 'Staff only');
  await until(() => ask.isVisible());
  check('a sign-in starts with the keyboard in Username', await until(async () => (await w.shell().evaluate(() => document.activeElement?.id)) === 'm-sign-in-user'));
  await shots('sign-in');
  // The room the keyboard leaves: the card's title and buttons stay in sight, and only
  // what is between them scrolls.
  await w.page.setViewportSize({ width: W, height: 300 });
  const room = await ask.evaluate((el) => {
    const inSight = (n) => { const r = n.getBoundingClientRect(); return r.top >= 0 && r.bottom <= innerHeight; };
    const body = el.querySelector('.body');
    return { title: inSight(el.querySelector('h2')), buttons: inSight(el.querySelector('.buttons')), scrolls: body.scrollHeight > body.clientHeight, whole: el.scrollHeight <= el.clientHeight };
  });
  check('with little room, as over the keyboard, its title and buttons stay in sight and only its body scrolls', room.title && room.buttons && room.scrolls && room.whole, JSON.stringify(room));
  await w.page.setViewportSize({ width: W, height: H });
  await backKey();
  check('and the back key answers it no too', await until(async () => (await answers()).at(-1).ok === false && (await ask.count()) === 0));

  // A download seen as it happens: a bar over the foot of the page, above the toolbar,
  // "Downloading big.zip", then "big.zip downloaded" with Open; a failed one says so with
  // Retry. What it says is kept as it changes, as a mock download takes a moment. The
  // first, while notifications aren't allowed, offers them, once, with Allow only.
  const dlBar = ui.getByRole('group', { name: 'Download' });
  await w.shell().evaluate(() => {
    window.barSaid = [];
    new MutationObserver(() => {
      const t = document.querySelector('[role=group][aria-label=Download] .words')?.textContent.trim();
      if (t && window.barSaid.at(-1) !== t) window.barSaid.push(t);
    }).observe(document.body, { subtree: true, childList: true, characterData: true });
  });
  const barSaid = () => w.shell().evaluate(() => window.barSaid);
  // A press on the bar reaches it once the bar has risen into place and the window routes
  // that point to the shell: its region goes to Go, and back to the window, a moment after
  // it draws. Pressed before, it goes to the page, and the bar never feels it.
  const barPressable = async (dx, dy) => {
    await until(async () => (await dlBar.evaluate((el) => el.getAnimations().length === 0).catch(() => false)));
    const b = await dlBar.boundingBox();
    await until(async () => (await regions({ x: Math.round(b.x + dx), y: Math.round(b.y + dy) })).hit === 'shell');
    return b;
  };
  // Its bytes held, so the offer stays up through its pictures; failed after, to be done.
  await fire('download', 'https://example.com/files/offered.zip');
  await fire('downloadsStall');
  check('the first download offers notifications on its bar, on a line of its own, with Allow only', await until(async () => (await barSaid()).at(-1) === 'Downloading offered.zip' &&
    (await dlBar.locator('.ask').textContent()) === 'Show progress in notifications?' &&
    (await dlBar.getByRole('button').count()) === 1 && (await dlBar.getByRole('button', { name: 'Allow' }).isVisible())), JSON.stringify(await barSaid()));
  await shots('download-offer');
  await dlBar.getByRole('button', { name: 'Allow' }).tap();
  check('Allow asks the system, and the bar goes', await until(async () => (await regions()).notificationsAllowed === true && !(await dlBar.isVisible())), JSON.stringify((await regions()).notificationsAllowed));
  await fire('downloadsFail');
  await until(async () => (await barSaid()).at(-1) === "Couldn't download offered.zip: the connection stopped");
  await fire('download', 'https://example.com/files/big.zip');
  check('a download starting says Downloading big.zip in a bar, with no offer again', await until(async () => (await barSaid()).includes('Downloading big.zip')), JSON.stringify(await barSaid()));
  check('and once done, big.zip downloaded, with Open', await until(async () => (await barSaid()).at(-1) === 'big.zip downloaded' && (await dlBar.getByRole('button', { name: 'Open' }).isVisible())), JSON.stringify(await barSaid()));
  // Where it ends once it has risen.
  const barBox = await dlBar.evaluate(async (el) => {
    await Promise.all(el.getAnimations().map((a) => a.finished));
    return el.getBoundingClientRect().toJSON();
  });
  const toolTop = (await box('.toolbar')).top;
  check('the bar lies over the page, just above the toolbar', barBox.bottom <= toolTop && barBox.bottom > toolTop - 24, JSON.stringify({ barBox, toolTop }));
  check('a press on it reaches the shell', (await regions({ x: barBox.x + 40, y: barBox.y + barBox.height / 2 })).hit === 'shell');
  await dlBar.getByRole('button', { name: 'Open' }).tap();
  check('Open opens the file, as the Downloads page\'s Open does, and the bar goes', await until(async () => ((await regions()).openedFiles || []).some((f) => f.endsWith('big.zip')) && !(await dlBar.isVisible())), JSON.stringify((await regions()).openedFiles));
  // A download whose bytes stop: Cancel and Retry on the bar while it runs; Retry starts it
  // over.
  await fire('download', 'https://example.com/files/stalled.zip');
  await fire('downloadsStall');
  check('a running download has Cancel and Retry on its bar', await until(async () => (await barSaid()).at(-1) === 'Downloading stalled.zip' &&
    (await dlBar.getByRole('button', { name: 'Cancel' }).isVisible()) && (await dlBar.getByRole('button', { name: 'Retry' }).isVisible())), JSON.stringify(await barSaid()));
  await dlBar.getByRole('button', { name: 'Retry' }).tap();
  check('Retry gets it', await until(async () => (await barSaid()).at(-1) === 'stalled.zip downloaded'), JSON.stringify(await barSaid()));
  await fire('download', 'https://example.com/files/broken.zip');
  await fire('downloadsFail');
  check('a download that fails says so, with Retry', await until(async () => (await barSaid()).at(-1) === "Couldn't download broken.zip: the connection stopped" && (await dlBar.getByRole('button', { name: 'Retry' }).isVisible())), JSON.stringify(await barSaid()));
  await dlBar.getByRole('button', { name: 'Retry' }).tap();
  check('Retry runs it again, and the bar says so as it ends', await until(async () => (await barSaid()).at(-1) === 'broken.zip downloaded' && (await dlBar.isVisible())), JSON.stringify(await barSaid()));
  const b = await barPressable(40, 10);
  await page.mouse.move(b.x + 40, b.y + 10);
  await page.mouse.down();
  await page.mouse.move(b.x + 40, b.y + 30);
  await page.mouse.move(b.x + 40, b.y + 50);
  await page.mouse.up();
  check('a swipe down puts the bar away', await until(async () => !(await dlBar.isVisible())));
  await fire('download', 'https://example.com/files/sideways.zip');
  await until(() => dlBar.isVisible());
  const sb = await barPressable(40, 10);
  await page.mouse.move(sb.x + 40, sb.y + 10);
  await page.mouse.down();
  await page.mouse.move(sb.x + 70, sb.y + 10);
  await page.mouse.move(sb.x + 100, sb.y + 10);
  await page.mouse.up();
  check('and so does a swipe sideways', await until(async () => !(await dlBar.isVisible())));
  // News goes after 2 seconds, but not while a finger holds it.
  // Pressed only once the bar is held.zip's: sideways.zip's end, put away while it ran, can
  // still come after, on a bar of its own that goes after its time.
  await fire('download', 'https://example.com/files/held.zip');
  await until(async () => (await barSaid()).at(-1)?.includes('held.zip') && (await dlBar.isVisible()));
  const hb = await barPressable(40, 10);
  await page.mouse.move(hb.x + 40, hb.y + 10);
  await page.mouse.down();
  await page.waitForTimeout(3500);
  check('a press keeps the bar while held, past its 2 seconds', await dlBar.isVisible());
  await page.mouse.up();
  check('and once let go it goes after its time', await until(async () => !(await dlBar.isVisible()), 5000));
  await fire('download', 'https://example.com/files/third.zip');
  check('another download brings it back', await until(() => dlBar.isVisible()));
  await page.mouse.click(4, H - 24);
  check('and a tap elsewhere puts it away', await until(async () => !(await dlBar.isVisible())));
  // A sheet that opens on its own, as a long press in the page opens one, puts it away
  // too, and it doesn't come back over what the person went on to.
  await fire('download', 'https://example.com/files/fourth.zip');
  await until(() => dlBar.isVisible());
  const pressSheet = ui.getByRole('dialog', { name: 'Link' });
  await fire('contextMenu', 'https://example.com/linked/sheet', { tab: (await regions()).active });
  check('a sheet opening puts it away, never under the bar', await until(async () => (await pressSheet.isVisible()) && !(await dlBar.isVisible())));
  await page.keyboard.press('Escape');
  check('and once the sheet closes it stays away', await until(async () => (await pressSheet.count()) === 0) && !(await dlBar.isVisible()));
  // Pictures of it, after the checks, as it lasts only a few seconds.
  await fire('download', 'https://example.com/files/picture.zip');
  await until(async () => (await barSaid()).at(-1) === 'picture.zip downloaded');
  await shots('download-bar');
  await fire('download', 'https://example.com/files/failing.zip');
  await fire('downloadsFail');
  await until(async () => (await barSaid()).at(-1) === "Couldn't download failing.zip: the connection stopped");
  await shots('download-failed');
  await page.mouse.click(4, H - 24);
  await until(async () => !(await dlBar.isVisible()));

  // The long-press sheet's other actions are Go's: Copy link, Share and Download for a link,
  // and for an image Copy image, Download, Share and Copy image address, its picture in the
  // head. A linked image has both rows of tiles.
  const press = async (what, item, group) => {
    await fire('contextMenu', JSON.stringify({ x: 40, y: 60, ...what }), { tab: (await regions()).active });
    await until(() => linkSheet.isVisible());
    const scope = group ? linkSheet.getByRole('group', { name: group }) : linkSheet;
    await scope.getByRole('menuitem', { name: item, exact: true }).tap();
    return until(async () => (await linkSheet.count()) === 0);
  };
  const aLink = { link: 'https://example.com/files/press.zip', linkText: 'The press file' };
  check('Copy link copies it, through Go, and the sheet goes', (await press(aLink, 'Copy link')) &&
    await until(async () => (await regions()).clipboard === aLink.link), (await regions()).clipboard);
  await press(aLink, 'Copy link text');
  check('Copy link text copies its words', await until(async () => (await regions()).clipboard === 'The press file'), (await regions()).clipboard);
  await press(aLink, 'Share');
  check('Share hands the link to the share sheet', await until(async () => (await regions()).sharedText === aLink.link), (await regions()).sharedText);
  await press(aLink, 'Download');
  check('Download starts it at once, with no Save As', await until(async () => (await barSaid()).includes('Downloading press.zip')), JSON.stringify(await barSaid()));
  await page.mouse.click(4, H - 24);
  await until(async () => !(await dlBar.isVisible()));

  const linked = { ...aLink, src: 'https://example.com/photo.png', media: 'image' };
  await fire('contextMenu', JSON.stringify({ x: 40, y: 60, ...linked }), { tab: (await regions()).active });
  const imageSheet = ui.getByRole('dialog', { name: 'Link' });
  await until(() => imageSheet.isVisible());
  check('a linked image has the link\'s tiles and the image\'s: New tab, Copy image, Download, Share', JSON.stringify(await tileNames('Image')) === JSON.stringify(['New tab', 'Copy image', 'Download', 'Share']) &&
    (await tileNames('Link')).length === 5, JSON.stringify([await tileNames('Link'), await tileNames('Image')]));
  check('and each row says whose it is, Link then Image', JSON.stringify(await imageSheet.locator('.heading').allInnerTexts()) === '["Link","Image"]',
    JSON.stringify(await imageSheet.locator('.heading').allInnerTexts()));
  check('and its picture shows in the head once Go has it', await until(async () => ((await imageSheet.locator('.thumb img').getAttribute('src').catch(() => '')) || '').startsWith('data:image/png;base64,')));
  check('Copy image address is a row', await imageSheet.getByRole('menuitem', { name: 'Copy image address' }).isVisible());
  await shots('linked-image');
  await imageSheet.getByRole('menuitem', { name: 'Copy image', exact: true }).tap();
  check('Copy image puts the image itself on the clipboard', await until(async () => /shared\/image-\d+\.png$/.test((await regions()).copiedImage || '')), (await regions()).copiedImage);
  await press(linked, 'Share', 'Image');
  check('the image\'s Share hands the image file to the share sheet', await until(async () => /shared\/image-\d+\.png$/.test((await regions()).shared || '')), (await regions()).shared);
  await press(linked, 'Copy image address');
  check('Copy image address copies its address', await until(async () => (await regions()).clipboard === linked.src), (await regions()).clipboard);

  // Downloads: always in the menu, with how many are running, and it opens the Downloads
  // page, a tab of ours, its title where the address goes.
  await fire('download', 'https://example.com/files/report.pdf');
  await menuButton.tap();
  await until(() => menu.isVisible());
  const downloadsItem = ui.getByRole('menuitem', { name: 'Downloads' });
  check('the menu has Downloads', await downloadsItem.isVisible());
  await downloadsItem.tap();
  const downloadsTab = async () => (await regions()).tabs.find((t) => t.page === 'downloads');
  check('which opens the Downloads page in a tab of ours, shown', await until(async () => (await downloadsTab())?.id === (await regions()).active));
  const dlFrame = page.frameLocator('iframe.our-page[data-page="downloads"]');
  check('the page lists the download by its name, with its actions', await until(async () => (await dlFrame.getByRole('listitem', { name: 'report.pdf' }).getByRole('button', { name: 'Actions for report.pdf' }).count()) > 0));
  check('and the bar says nothing over it, as the page says it all', !(await dlBar.isVisible()));
  check('and the pill says Downloads, with a close', await ui.getByRole('button', { name: 'Close Downloads' }).isVisible());
  await shots('downloads');
  await ui.getByRole('button', { name: 'Close Downloads' }).tap();
  check('its close closes it', await until(async () => !(await downloadsTab())));

  // Find in page, from the menu: a bar in the toolbar's place, the keyboard in it, closed
  // by its × as by the back key. An empty tab has nothing to find in.
  await visit('example.com/find', 'https://example.com/find');
  await menuButton.tap();
  await until(() => menu.isVisible());
  await ui.getByRole('menuitem', { name: 'Find in page' }).tap();
  const findField = ui.getByRole('textbox', { name: 'Find in page' });
  check('Find in page in the menu opens the find bar, with the keyboard in it', await until(async () => (await findField.isVisible()) && (await findField.evaluate((el) => el === document.activeElement))));
  const findBox = await ui.getByRole('search', { name: 'Find in page' }).boundingBox();
  const toolBox = await ui.getByRole('group', { name: 'Toolbar' }).boundingBox();
  check('in the toolbar\'s place', Math.abs(findBox.y - toolBox.y) < 1 && Math.abs(findBox.height - toolBox.height) < 1, JSON.stringify({ findBox, toolBox }));
  await findField.pressSequentially('mock');
  check('typing searches, and the count shows', await until(async () => /\d+\/\d+/.test(await ui.getByRole('status').filter({ visible: true }).first().innerText())));
  await shots('find');
  await ui.getByRole('button', { name: 'Close find' }).tap();
  check('its × closes it', await until(async () => (await findField.count()) === 0));

  // More, last in the sheet: the menu bar's menus for this window, as Go gives them
  // (docs/menus-shortcuts.md, "The shell's menu"), so nothing in them is out of reach: a
  // submenu opens in the sheet, with a row back; nothing of the whole program's, nor the
  // desktop's.
  await menuButton.tap();
  await until(() => menu.isVisible());
  const item = (name) => menu.getByRole('menuitem', { name, exact: true });
  await item('More').tap();
  check('More lists the menu bar\'s menus: Settings…, File, View, History, Bookmarks, Tab and Downloads',
    await until(async () => (await Promise.all(['Settings…', 'File', 'View', 'History', 'Bookmarks', 'Tab', 'Downloads'].map((n) => item(n).count()))).every((n) => n === 1)));
  check('and nothing of the whole program\'s, such as Quit', (await item('Quit Project B').count()) === 0);
  await item('View').tap();
  check('View opens in the sheet, with Reload This Page and a row back', await until(async () => (await item('Reload This Page').isVisible()) && (await menu.getByRole('menuitem', { name: 'Back to the menu' }).isVisible())));
  check('and without the desktop\'s, such as Full Screen', (await item('Full Screen').count()) === 0);
  await menu.getByRole('menuitem', { name: 'Back to the menu' }).tap();
  check('the row back shows the menus again', await until(() => item('File').isVisible()));
  await menu.getByRole('menuitem', { name: 'Back to the menu' }).tap();
  check('and from there, the sheet\'s own rows', await until(() => item('Settings').isVisible()) && (await item('File').count()) === 0);

  // Settings, from the menu.
  await item('Settings').tap();
  check('Settings in the menu opens the settings tab', await until(async () => (await regions()).settings !== 0 && (await regions()).active === (await regions()).settings));

  // The address bar on top, from settings on a phone: the toolbar moves under the status
  // bar, the strip stays at the foot, the suggestions grow down.
  await w.settings.getByRole('radiogroup', { name: 'Address bar' }).getByRole('radio', { name: 'Top' }).tap();
  // The settings page's message goes on its tab's channel, which a closed tab no longer has,
  // so the tab closes only once Go has kept the choice.
  const pref = () => JSON.parse(fs.readFileSync(path.join(lib.profile, 'prefs.json'), 'utf8')).addressBarPosition;
  check('Top in settings reaches Go', await until(async () => fs.existsSync(path.join(lib.profile, 'prefs.json')) && pref() === 'top'));
  await lib.closeTab((await regions()).settings);
  await until(async () => (await regions()).settings === 0);
  const clear = (await regions()).window.Controls;
  const topY = clear.h > 0 ? clear.y + clear.h : 0;
  check('Top in settings moves the toolbar to the top', await until(async () => {
    const b = await box('.toolbar');
    return b && Math.round(b.y) === topY && b.height === 48;
  }), JSON.stringify(await box('.toolbar')));
  check('the page lies under it, down to the foot or the strip', await until(async () => {
    const v = (await regions()).visible[0].rect;
    return v.y === topY + 48 && v.y + v.h === H - ((await strip.isVisible()) ? 49 : 0);
  }), JSON.stringify((await regions()).visible));
  await fire('menu', 'tab.new');
  await until(newTabPage);
  await field.tap();
  await until(inField);
  await page.keyboard.type('example.com/docs');
  await listAnswers('example.com/docs');
  await until(async () => (await suggestions.count()) >= 2);
  const below = await box('.suggestions');
  check('the suggestions grow down from the field', below.y >= topY + 48 - 4 && below.y < topY + 60, JSON.stringify(below));
  await shots('top-suggestions');
  await page.keyboard.press('Enter');
  await until(async () => !(await inField()));
  await tabsButton.tap();
  check('the switcher opens under the toolbar', await until(async () => {
    const b = await box('.switcher');
    return (await switcher.isVisible()) && Math.round(b.y) === topY + 48 && b.bottom === H;
  }), JSON.stringify(await box('.switcher')));
  await page.keyboard.press('Escape');
  await fire('menu', 'tab.select1');
  const firstTab = (await regions()).tabs[0].id;
  await until(async () => (await regions()).active === firstTab);
  await menuButton.tap();
  await menu.getByRole('button', { name: 'New Tab' }).tap();
  await until(inField);
  await page.keyboard.press('Escape');
  await tabsButton.tap();
  const lastCard = cards.filter({ visible: true }).first();
  await lastCard.click({ button: 'right' });
  await cardMenu.getByRole('menuitem', { name: 'Add to group' }).tap();
  await cardMenu.getByRole('menuitem', { name: 'New group' }).tap();
  check('with the toolbar on top, the strip is at the foot', await until(async () => {
    const b = await box('[role=group][aria-label^="Group "]');
    return b && Math.round(b.bottom) === H;
  }), JSON.stringify(await box('[role=group][aria-label^="Group "]')));
  await page.keyboard.press('Escape');
  await shots('top-strip');

  // Where bookmarks show (docs/bookmark-folders.md), each while its pref is on. A tree to
  // show: folders made as the Bookmarks page makes them, and bookmarks as the star makes
  // them, then put in their folders as its panel does; each new one goes first, so they are
  // made last first.
  const shellSend = (m) => fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify(m) });
  // A pref set as a shell sets one, once Go has kept it.
  const setPref = async (name, value) => {
    await shellSend({ t: 'setPref', name, value: String(value) });
    await until(() => String(JSON.parse(fs.readFileSync(path.join(lib.profile, 'prefs.json'), 'utf8'))[name]) === String(value));
  };
  const entries = lib.bookmarks;
  await fire('menu', 'bookmarks.show');
  await until(async () => (await regions()).tabs.some((t) => t.page === 'bookmarks'));
  const manager = (await regions()).tabs.find((t) => t.page === 'bookmarks').id;
  const newFolder = async (title, parent = '') => {
    await fetch(`${lib.base}/bookmarks/send?tab=${manager}`, { method: 'POST', body: JSON.stringify({ t: 'newFolder', parent, title }) });
    await until(() => entries().some((e) => e.kind === 'folder' && e.title === title));
    return entries().find((e) => e.kind === 'folder' && e.title === title).id;
  };
  const newBookmark = async (title, url, parent = '') => {
    await shellSend({ t: 'bookmark', url, title });
    await until(() => entries().some((e) => e.url === url));
    const id = entries().find((e) => e.url === url).id;
    await shellSend({ t: 'saveBookmark', bookmark: id, title, parent });
    await until(() => entries().find((e) => e.id === id).parent === parent);
  };
  const travel = await newFolder('Travel');
  const recipes = await newFolder('Recipes');
  const reading = await newFolder('Reading');
  const baking = await newFolder('Baking', recipes);
  await newBookmark('Shop', 'https://shop.example.com/');
  await newBookmark('Bank', 'https://bank.example.com/');
  await newBookmark('Hotels', 'https://hotels.example.com/', travel);
  await newBookmark('Maps', 'https://maps.example.com/', travel);
  await newBookmark('Science', 'https://science.example.com/', reading);
  await newBookmark('News', 'https://news.example.com/', reading);
  await newBookmark('Bread', 'https://bread.example.com/', baking);
  const recipeMarks = [['Salads', 'salads'], ['Curry', 'curry'], ['Flatbread', 'flatbread'], ['Lentil soup', 'soup'], ['Pasta', 'pasta']];
  for (const [title, host] of recipeMarks) await newBookmark(title, `https://${host}.example.com/`, recipes);
  await page.keyboard.press('Escape');
  await lib.closeTab(manager);
  await until(async () => !(await regions()).tabs.some((t) => t.page === 'bookmarks'));

  // The new tab page (bookmarksNewTab, on by default): the top folder as tiles, folders
  // first; a folder's tile shows its first bookmarks two by two, and opens in place.
  await fire('menu', 'tab.new');
  await until(newTabPage);
  const newTabMarks = ui.getByRole('region', { name: 'New tab' }).getByRole('list', { name: 'Bookmarks' });
  const markTiles = newTabMarks.locator(':scope > li > .tile');
  check('the new tab shows the top folder as tiles, its folders first', await until(async () => JSON.stringify(await markTiles.locator('.name').allInnerTexts()) === JSON.stringify(['Reading', 'Recipes', 'Travel', 'Bank', 'Shop'])), JSON.stringify(await markTiles.locator('.name').allInnerTexts()));
  const recipesTile = markTiles.filter({ hasText: 'Recipes' });
  check('a folder\'s tile shows its first four bookmarks, two by two', (await recipesTile.locator('.folder > .mark').allInnerTexts()).join('') === 'PLFC');
  await recipesTile.tap();
  const folderCard = newTabMarks.getByRole('group', { name: 'Recipes' });
  check('a tap on a folder opens it in place, under its row, with how many it holds', await until(() => folderCard.isVisible()) && (await folderCard.locator('.count').innerText()) === '· 5' &&
    (await folderCard.boundingBox()).y > (await recipesTile.boundingBox()).y + (await recipesTile.boundingBox()).height && (await recipesTile.getAttribute('aria-expanded')) === 'true');
  check('its folders first, then its bookmarks', JSON.stringify(await folderCard.locator('.tile .name').allInnerTexts()) === JSON.stringify(['Baking', 'Pasta', 'Lentil soup', 'Flatbread', 'Curry', 'Salads']), JSON.stringify(await folderCard.locator('.tile .name').allInnerTexts()));
  const bakingFolder = folderCard.locator('.tile').filter({ hasText: 'Baking' }).locator('.folder');
  check('a folder with one bookmark shows it in the middle of its tile', await centred(bakingFolder.locator('.mark'), bakingFolder));
  await shots('bookmarks-new-tab');
  await folderCard.locator('.tile').filter({ hasText: 'Baking' }).tap();
  const bakingCard = newTabMarks.getByRole('group', { name: 'Baking' });
  check('a folder in it opens in the card, with a way back', await until(() => bakingCard.isVisible()) && JSON.stringify(await bakingCard.locator('.tile .name').allInnerTexts()) === '["Bread"]');
  await bakingCard.getByRole('button', { name: 'Back' }).tap();
  await until(() => folderCard.isVisible());
  await recipesTile.tap();
  check('a second tap on the folder closes it', await until(async () => (await folderCard.count()) === 0));
  const newTabId = (await regions()).active;
  await markTiles.filter({ hasText: 'Bank' }).tap();
  check('a tap on a bookmark loads it in this tab', await until(async () => (await regions()).active === newTabId && (await active()).url === 'https://bank.example.com/' && !(await ui.getByRole('region', { name: 'New tab' }).isVisible())));
  // Open all: the first in this tab, the rest behind it, in a group named after the folder.
  await fire('menu', 'tab.new');
  await until(newTabPage);
  const openAllTab = (await regions()).active;
  const tabsBefore = (await regions()).tabs.length;
  await recipesTile.tap();
  await until(() => folderCard.isVisible());
  await folderCard.getByRole('button', { name: 'Open all' }).tap();
  const grouped = async () => {
    const r = await regions();
    const ids = await stripTabs.evaluateAll((els) => els.map((e) => Number(e.dataset.tab)));
    return ids.map((id) => r.tabs.find((t) => t.id === id)?.url).join();
  };
  const recipeUrls = recipeMarks.map(([, host]) => `https://${host}.example.com/`).reverse().join();
  check('Open all loads the first in this tab and the rest behind it, in a group with it named after the folder', await until(async () => {
    const r = await regions();
    return r.active === openAllTab && r.tabs.length === tabsBefore + 4 && (await strip.getAttribute('aria-label').catch(() => '')) === 'Group Recipes' && (await grouped()) === recipeUrls;
  }), JSON.stringify({ urls: await grouped(), label: await strip.getAttribute('aria-label').catch(() => '') }));
  // Off, the new tab has no bookmarks.
  await setPref('bookmarksNewTab', false);
  await fire('menu', 'tab.new');
  await until(newTabPage);
  check('with bookmarksNewTab off, the new tab has no bookmarks', await until(async () => (await newTabMarks.count()) === 0) && (await ui.getByRole('region', { name: 'New tab' }).getByRole('heading', { name: 'Bookmarks' }).count()) === 0);
  await setPref('bookmarksNewTab', true);
  check('and on again, they are back', await until(() => newTabMarks.isVisible()));

  // The switcher (bookmarksSidebar, off by default): Tabs and Bookmarks at its top, the
  // folders from the top, a bookmark opening in a new tab.
  const views = switcher.getByRole('radiogroup', { name: 'Show' });
  const marksView = switcher.getByRole('region', { name: 'Bookmarks' });
  const marksTrail = () => marksView.getByRole('navigation', { name: 'Path' }).getByRole('button').allInnerTexts();
  const marksRows = () => marksView.getByRole('listitem').locator('.title').allInnerTexts();
  await tabsButton.tap();
  await until(() => switcher.isVisible());
  check('with bookmarksSidebar off, the switcher has its tabs alone', (await views.count()) === 0);
  await page.keyboard.press('Escape');
  await setPref('bookmarksSidebar', true);
  await tabsButton.tap();
  await until(() => switcher.isVisible());
  check('on, Tabs and Bookmarks at its top, Tabs chosen', await until(() => views.isVisible()) && (await views.getByRole('radio', { name: /^Tabs · \d+$/ }).getAttribute('aria-checked')) === 'true');
  await views.getByRole('radio', { name: 'Bookmarks' }).tap();
  check('Bookmarks shows the top folder in place of the cards, its folders first', await until(async () => JSON.stringify(await marksRows()) === JSON.stringify(['Reading', 'Recipes', 'Travel', 'Bank', 'Shop'])) && (await cards.filter({ visible: true }).count()) === 0, JSON.stringify(await marksRows()));
  await marksView.getByRole('button').filter({ hasText: 'Recipes' }).tap();
  check('a folder opens in the list, under its path', await until(async () => JSON.stringify(await marksTrail()) === JSON.stringify(['Bookmarks', 'Recipes']) && (await marksRows()).length === 6), JSON.stringify(await marksTrail()));
  await shots('bookmarks-switcher');
  await marksView.getByRole('navigation', { name: 'Path' }).getByRole('button', { name: 'Bookmarks' }).tap();
  check('its path leads back up', await until(async () => JSON.stringify(await marksTrail()) === JSON.stringify(['Bookmarks']) && (await marksRows()).length === 5));
  await marksView.getByRole('button').filter({ hasText: 'Recipes' }).tap();
  await until(async () => (await marksRows()).length === 6);
  const tabsBeforeOpen = (await regions()).tabs.length;
  await marksView.getByRole('button').filter({ hasText: 'Flatbread' }).tap();
  check('a tap on a bookmark opens it in a new tab and closes the switcher', await until(async () => (await regions()).tabs.length === tabsBeforeOpen + 1 && (await active()).url === 'https://flatbread.example.com/' && !(await switcher.isVisible())), JSON.stringify(await active()));
  await tabsButton.tap();
  check('it opens on Tabs again', await until(async () => (await views.getByRole('radio', { name: /^Tabs/ }).getAttribute('aria-checked')) === 'true' && (await cards.filter({ visible: true }).count()) > 0));
  await page.keyboard.press('Escape');

  // The address (bookmarksCommandBox, on by default): with nothing typed, the top folder's
  // folders as chips, All first, and the chosen one's bookmarks as tiles, nearest the field:
  // under it with the bar on top, over it with the bar at the foot. Typing replaces them.
  const addressMarks = ui.getByRole('region', { name: 'Bookmarks' }).filter({ has: ui.getByRole('radiogroup', { name: 'Folder' }) });
  const folderChips = addressMarks.getByRole('radio');
  const addressTiles = () => addressMarks.locator('.tile .name').allInnerTexts();
  const recipeTitles = recipeMarks.map(([title]) => title).reverse();
  await fire('menu', 'tab.new');
  await until(newTabPage);
  await field.tap();
  check('the address tapped shows the folders as chips, All first and chosen', await until(() => addressMarks.isVisible()) &&
    JSON.stringify(await folderChips.allInnerTexts()) === JSON.stringify(['All', 'Reading', 'Recipes', 'Travel']) && (await folderChips.first().getAttribute('aria-checked')) === 'true', JSON.stringify(await folderChips.allInnerTexts()));
  check('All: every bookmark, as the folders hold them', JSON.stringify(await addressTiles()) === JSON.stringify(['News', 'Science', 'Bread', ...recipeTitles, 'Maps', 'Hotels', 'Bank', 'Shop']), JSON.stringify(await addressTiles()));
  const topField = await field.boundingBox();
  check('with the bar on top, they lie under the field', (await addressMarks.boundingBox()).y >= topField.y + topField.height, JSON.stringify(await addressMarks.boundingBox()));
  await folderChips.filter({ hasText: 'Recipes' }).tap();
  check('a folder\'s chip shows its bookmarks, in it and in its folders', await until(async () => JSON.stringify(await addressTiles()) === JSON.stringify(['Bread', ...recipeTitles])) && await inField(), JSON.stringify(await addressTiles()));
  await shots('bookmarks-address-top');
  await page.keyboard.type('example');
  check('typing replaces them with the suggestions', await until(async () => (await addressMarks.count()) === 0 && (await suggestions.count()) >= 1));
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.press('Backspace');
  check('emptying the field brings them back', await until(() => addressMarks.isVisible()));
  const fromAddress = (await regions()).active;
  await addressMarks.locator('.tile').filter({ hasText: 'Curry' }).tap();
  check('a tap on one loads it in this tab, and the field lets go', await until(async () => (await regions()).active === fromAddress && (await active()).url === 'https://curry.example.com/' && !(await inField())));
  await until(async () => !(await active()).loading);
  // The shell was loaded again since the suggests were first watched.
  await w.shell().evaluate(() => {
    const send = window.chrome.send;
    window.lastSuggest = null;
    window.chrome.send = (channel, args) => {
      const m = JSON.parse(args[0]);
      if (m.t === 'suggest') window.lastSuggest = m;
      return send.call(window.chrome, channel, args);
    };
  });
  await setPref('addressBarPosition', 'bottom');
  await until(async () => (await box('.toolbar')).bottom === H);
  await field.tap();
  check('on a page, the list for nothing typed asks for no bookmark rows, the bookmarks showing', await until(async () => JSON.stringify((await lastSuggest())?.kinds) === JSON.stringify(['tab', 'closed', 'history'])) && await until(() => addressMarks.isVisible()), JSON.stringify(await lastSuggest()));
  const footField = await field.boundingBox();
  const marksBox = await addressMarks.boundingBox();
  check('with the bar at the foot, they lie over the field, the list over them', marksBox.y + marksBox.height <= footField.y && (await box('.suggestions').then((b) => !b || b.bottom <= marksBox.y + 1)), JSON.stringify({ marksBox, footField, list: await box('.suggestions') }));
  await shots('bookmarks-address');
  await page.keyboard.press('Escape');
  await setPref('bookmarksCommandBox', false);
  await field.tap();
  check('with bookmarksCommandBox off, the address has none, and asks for bookmark rows again', await until(async () => JSON.stringify((await lastSuggest())?.kinds) === JSON.stringify(['tab', 'closed', 'history', 'bookmark'])) && (await addressMarks.count()) === 0);
  await page.keyboard.press('Escape');
  await setPref('bookmarksCommandBox', true);

  // The bookmarks bar (bookmarksBar, off by default): the chosen folder's entries as chips,
  // the top's when it is gone, laid with the bars: over the foot's, the group strip too, or
  // under the address bar on top. It hides and comes back with them, and the page's room
  // counts it.
  const marksBar = ui.getByRole('toolbar', { name: 'Bookmarks bar' });
  const barChips = () => marksBar.locator('.name').allInnerTexts();
  const pageNow = async () => {
    const r = await regions();
    return r.visible.find((v) => v.tab === r.active)?.rect;
  };
  const scrollNow = async (direction) => {
    await until(settled);
    await fire('scroll', direction, { tab: (await regions()).active });
  };
  check('with bookmarksBar off, there is no bookmarks bar', (await marksBar.count()) === 0);
  await setPref('bookmarksBar', true);
  check('on, the bar shows the top folder\'s entries, its folders first', await until(async () => JSON.stringify(await barChips()) === JSON.stringify(['Reading', 'Recipes', 'Travel', 'Bank', 'Shop'])), JSON.stringify(await barChips()));
  const barAt = await box('.marks-bar');
  check('with the address at the foot, just over the toolbar, 49px with its hairline', Math.round(barAt.bottom) === H - 48 && Math.round(barAt.height) === 49, JSON.stringify(barAt));
  check('the page ends over it', await until(async () => {
    const v = await pageNow();
    return v.y + v.h === H - 48 - 49;
  }), JSON.stringify(await pageNow()));
  await shots('bookmarks-bar');
  await marksBar.getByRole('button', { name: 'Bank' }).tap();
  check('a tap on a bookmark loads it in the tab', await until(async () => (await active()).url === 'https://bank.example.com/'));
  await marksBar.getByRole('button', { name: 'Recipes' }).tap();
  const barFolder = ui.getByRole('dialog', { name: 'Recipes' });
  check('a tap on a folder opens a sheet of what it holds', await until(async () => (await barFolder.isVisible()) && (await barFolder.getByRole('listitem').count()) === 6));
  await shots('bookmarks-bar-folder');
  await barFolder.getByRole('button').filter({ hasText: 'Salads' }).tap();
  check('and a bookmark in it loads in the tab, the sheet closing', await until(async () => (await active()).url === 'https://salads.example.com/' && (await barFolder.count()) === 0));
  await setPref('bookmarksBarFolder', recipes);
  check('bookmarksBarFolder chooses the folder it shows', await until(async () => JSON.stringify(await barChips()) === JSON.stringify(['Baking', ...recipeTitles])), JSON.stringify(await barChips()));
  await setPref('bookmarksBarFolder', '0123456789abcdef');
  check('and a folder that is gone shows the top', await until(async () => (await barChips())[0] === 'Reading'), JSON.stringify(await barChips()));
  // In a group, the group strip under it.
  const groupTab = Number(await stripTabs.first().getAttribute('data-tab').catch(() => 0)) || (await regions()).tabs.find((t) => t.url === 'https://pasta.example.com/').id;
  await shellSend({ t: 'activateTab', id: groupTab });
  await until(async () => (await regions()).active === groupTab && (await strip.isVisible()));
  await until(async () => !(await active()).loading);
  check('with the active tab in a group, it lies over the group strip', await until(async () => {
    const b = await box('.marks-bar');
    const g = await box('[role=group][aria-label^="Group "]');
    return Math.round(b.bottom) === Math.round(g.top) && Math.round(g.bottom) === H - 48;
  }), JSON.stringify({ bar: await box('.marks-bar'), strip: await box('[role=group][aria-label^="Group "]') }));
  const withMarks = await pageNow();
  check('and the page ends over both', withMarks.y + withMarks.h === H - 48 - 49 - 49, JSON.stringify(withMarks));
  await shots('bookmarks-bar-group');
  await scrollNow('down');
  check('scrolling down, it goes with the bars, and the page takes the window to its foot', await until(async () => {
    const v = await pageNow();
    return v.y + v.h === H && (await box('.marks-bar')).top >= H;
  }), JSON.stringify({ page: await pageNow(), bar: await box('.marks-bar') }));
  await w.shell().evaluate(() => window.barsSettled());
  await scrollNow('up');
  check('scrolling up, it comes back with them, and the page gives their room back', await until(async () => JSON.stringify(await pageNow()) === JSON.stringify(withMarks) && Math.round((await box('.marks-bar')).bottom) === H - 48 - 49),
    JSON.stringify({ page: await pageNow(), bar: await box('.marks-bar') }));
  // On top: under the address bar.
  await setPref('addressBarPosition', 'top');
  check('with the address on top, it lies under the toolbar, and the page under it', await until(async () => {
    const t = await box('.toolbar');
    const b = await box('.marks-bar');
    const v = await pageNow();
    return t && b && Math.round(b.top) === Math.round(t.bottom) && v.y === Math.round(b.bottom) && v.y + v.h === H - 49;
  }), JSON.stringify({ toolbar: await box('.toolbar'), bar: await box('.marks-bar'), page: await pageNow() }));
  const topWithMarks = await pageNow();
  await shots('bookmarks-bar-top');
  await w.shell().evaluate(() => window.barsSettled());
  await scrollNow('down');
  check('scrolling down, it goes up with the toolbar, and the page takes their room to the top', await until(async () => {
    const v = await pageNow();
    return v.y === topWithMarks.y - 48 - 49 && (await box('.marks-bar')).bottom <= v.y;
  }), JSON.stringify({ page: await pageNow(), bar: await box('.marks-bar') }));
  await w.shell().evaluate(() => window.barsSettled());
  await scrollNow('up');
  check('and back with it', await until(async () => JSON.stringify(await pageNow()) === JSON.stringify(topWithMarks) && Math.round((await box('.marks-bar')).top) === Math.round((await box('.toolbar')).bottom)),
    JSON.stringify({ page: await pageNow(), bar: await box('.marks-bar') }));
  await setPref('bookmarksBar', false);
  check('off, it goes and the page has its room', await until(async () => (await marksBar.count()) === 0 && (await pageNow()).y === topWithMarks.y - 49));

  // A tab's picture is let go as the tab closes, so one on its way then fails to load, and
  // its card, gone or going, shows the favicon: not an error.
  const errors = [...w.errors];
  for (const miss of thumbMisses) errors.splice(errors.indexOf(miss) >>> 0, errors.includes(miss) ? 1 : 0);
  check('it logged no errors', errors.length === 0, errors.join(' | '));
  await browser.close();
  await lib.done();
})();
