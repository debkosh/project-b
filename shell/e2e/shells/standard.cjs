// The standard shell's own tests, past what first-app.cjs drives: the right-click menu over
// the page (docs/daily-driver.md), on a link, an image, a selection, a field and the page
// itself, chosen by the mouse and by the keys, its workspace submenu, and closing it. It
// leaves light and dark screenshots in build/e2e/standard-menu-*.png.
// Run: node shell/e2e/shells/standard.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('../lib.cjs');
const { check, fire, regions, settle, until } = lib;

(async () => {
  fs.writeFileSync(path.join(lib.profile, 'workspaces.json'), JSON.stringify({ version: 2, workspaces: [{ id: '', name: 'Main', color: '#2e9d5b' }, { id: '0a1b2c3d', name: 'Work', color: '#3470d0' }] }));
  await lib.start(['-use=standard']);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser);
  const { page, ui } = w;
  page.setDefaultTimeout(5000);

  const menu = ui.getByRole('menu').first();
  const item = (name) => ui.getByRole('menuitem', { name: new RegExp(`^${name}`) });
  const rightClick = async (on) => {
    await fire('contextMenu', JSON.stringify(on));
    await menu.waitFor();
    await settle(page);
  };
  const gone = () => until(async () => (await menu.count()) === 0);
  const shoot = async (name) => {
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: scheme });
      await settle(page);
      await page.screenshot({ path: path.join(lib.out, `standard-menu-${name}-${scheme}.png`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
  };

  // Standard starts with no tab; the checks want one.
  await until(async () => (await regions()).shell === 'standard', 10000);
  await fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'newTab', text: 'https://start.test/' }) });
  check('comes up', await until(async () => (await regions()).shell === 'standard' && (await regions()).visible.length === 1, 10000));
  // start.test loads before the tab goes on, or a.test takes its place and Back has nowhere to go.
  await until(async () => (await regions()).tabs[0]?.title.includes('start.test'));
  await fire('navigate', 'https://a.test/');
  await until(async () => (await regions()).tabs[0]?.title.includes('a.test'));
  const first = (await regions()).active;

  // A link: Go's actions, in a menu at the press, over the page and the shell's.
  await rightClick({ x: 120, y: 80, link: 'https://b.test/', linkText: 'b' });
  const names = await ui.getByRole('menuitem').allInnerTexts();
  check('a link\'s menu has Go\'s actions', ['Open Link in New Tab', 'Open Link in New Window', 'Open Link in Workspace', 'Copy Link'].every((n) => names.some((x) => x.startsWith(n))), JSON.stringify(names));
  check('it is named for what was pressed', (await menu.getAttribute('aria-label')) === 'Link');
  const box = await menu.boundingBox();
  const at = (await regions()).visible[0].rect;
  check('it opens at the press, in the page', Math.abs(box.x - (at.x + 120)) < 2 && Math.abs(box.y - (at.y + 80)) < 2, JSON.stringify({ box, at }));
  const cover = await regions({ x: at.x + at.w - 10, y: at.y + at.h - 10 });
  check('while open, a press anywhere is the shell\'s', cover.hit === 'shell', JSON.stringify(cover.hit));
  check('the link\'s copy actions come after a line', JSON.stringify(await menu.evaluate((m) => [...m.children].map((c) => c.getAttribute('role') === 'separator' ? '-' : c.textContent.trim()))) === JSON.stringify(['Open Link in New Tab', 'Open Link in Workspace', 'Open Link in New Window', '-', 'Copy Link', 'Save Link As…']));
  const row = menu.getByRole('menuitem', { name: /^Open Link in Workspace/ });
  const chevron = await row.locator('.ui-menu-chevron').boundingBox();
  const rowBox = await row.boundingBox();
  const menuBox = await menu.boundingBox();
  check('the submenu\'s chevron sits inside its row, 12px from the end, and the row inside the menu', chevron && Math.abs(rowBox.x + rowBox.width - 12 - (chevron.x + chevron.width)) < 1.5 && rowBox.x + rowBox.width <= menuBox.x + menuBox.width, JSON.stringify({ chevron, rowBox, menuBox }));
  await shoot('link');
  await item('Copy Link').click();
  check('Copy Link copies it, and the menu goes', (await gone()) && (await until(async () => (await regions()).clipboard === 'https://b.test/')), JSON.stringify((await regions()).clipboard));
  check('and the page has the keyboard again', await until(async () => (await regions()).focused === first));

  // The workspace submenu, by the keys.
  await rightClick({ link: 'https://work.test/' });
  await menu.focus();
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowRight');
  const sub = ui.getByRole('menu', { name: 'Open Link in Workspace' });
  check('→ opens the workspace submenu, with the other workspace', await until(async () => (await sub.isVisible()) && (await sub.getByRole('menuitem').allInnerTexts()).join() === 'Work'));
  await shoot('workspace');
  await page.keyboard.press('Enter');
  check('Enter opens the link in that workspace', await until(async () => (await regions()).tabs.some((t) => t.url === 'https://work.test/' && t.workspace === '0a1b2c3d')));

  // Escape closes, and nothing runs.
  await rightClick({ link: 'https://c.test/' });
  await menu.focus();
  await page.keyboard.press('Escape');
  check('Escape closes it', await gone());
  await rightClick({ link: 'https://c.test/' });
  const r = (await regions()).visible[0].rect;
  await page.mouse.click(r.x + r.w - 20, r.y + r.h - 20);
  check('a press outside closes it', await gone());
  check('and nothing ran', !(await regions()).tabs.some((t) => t.url === 'https://c.test/') && (await regions()).clipboard === 'https://b.test/');

  // A link opened in a new tab opens behind.
  await rightClick({ link: 'https://d.test/' });
  await item('Open Link in New Tab').click();
  check('Open Link in New Tab opens it behind', await until(async () => {
    const g = await regions();
    return g.tabs.some((t) => t.url === 'https://d.test/') && g.tabs.find((t) => t.id === g.active)?.url === 'https://a.test/';
  }));

  // A selection, a field and the page.
  await rightClick({ selection: 'set up your workspace and open the dashboard' });
  check('a selection offers Copy and a search for it', (await item('Copy').count()) === 1 && (await item('Search for “set up your workspace an…”').count()) === 1);
  await shoot('selection');
  await item('Copy').click();
  check('Copy copies the selection', await until(async () => (await regions()).clipboard === 'set up your workspace and open the dashboard'));

  await rightClick({ editable: true });
  check('a field offers Cut disabled with nothing selected', (await item('Cut').getAttribute('aria-disabled')) === 'true');
  await item('Paste').click();
  check('Paste pastes into the page', await until(async () => JSON.stringify((await regions()).edits) === '[5]'), JSON.stringify((await regions()).edits));

  await rightClick({});
  // The tab went from start.test to a.test, so Back has somewhere to go and Forward hasn't.
  check('the page offers Back, Forward disabled with nowhere to go, and Reload', (await item('Back').getAttribute('aria-disabled')) === null && (await item('Forward').getAttribute('aria-disabled')) === 'true' && (await item('Reload').count()) === 1);
  await shoot('page');
  await page.keyboard.press('Escape');
  await gone();

  // Near the window's corner it flips to stay inside.
  const v = (await regions()).visible[0].rect;
  await rightClick({ x: v.w - 10, y: v.h - 10, link: 'https://e.test/' });
  const b = await menu.boundingBox();
  const vp = page.viewportSize();
  check('near the corner it stays inside the window', b.x + b.width <= vp.width - 4 && b.y + b.height <= vp.height - 4, JSON.stringify({ b, vp }));
  await page.keyboard.press('Escape');

  // Downloads: the control shows once there is one, and its list acts on each.
  const control = ui.getByRole('button', { name: 'Downloads' });
  check('no Downloads control before any download', (await control.count()) === 0);
  const list = ui.getByRole('dialog', { name: 'Downloads' });
  const pageHasKeys = async () => (await regions()).focused === (await regions()).active;
  await fire('focusTab');
  await until(pageHasKeys);
  await fire('download', 'https://a.test/files/report.pdf');
  check('a download brings the Downloads control', await until(() => control.isVisible()));
  // It opens the list under the control by itself, and closes it after about five seconds.
  const opened = Date.now();
  check('the list opens by itself under the control as the download begins', await until(() => list.isVisible(), 2000));
  const [cb, lb] = [await control.boundingBox(), await list.boundingBox()];
  check('right under it, their right edges together', lb.y > cb.y + cb.height && lb.y - (cb.y + cb.height) < 12 && Math.abs(lb.x + lb.width - (cb.x + cb.width)) < 1.5, JSON.stringify({ cb, lb }));
  check('nothing in it has the keyboard', await list.evaluate((el) => !el.contains(document.activeElement) || document.activeElement === el));
  await shoot('downloads-peek');
  check('it closes by itself after about five seconds', (await until(async () => (await list.count()) === 0, 8000)) && Date.now() - opened > 4500, String(Date.now() - opened));
  check('and gives the page the keyboard back', await until(pageHasKeys));
  await control.click();
  const entry = list.getByRole('listitem', { name: 'report.pdf' });
  check('its list shows the download by its file name', await until(() => entry.isVisible()));
  check('done, it offers Open and Show in Finder', await until(async () => (await entry.getByRole('button', { name: 'Open' }).count()) === 1 && (await entry.getByRole('button', { name: 'Show in Finder' }).count()) === 1));
  await shoot('downloads');
  await entry.getByRole('button', { name: 'Show in Finder' }).click();
  check('Show in Finder asks Go to show the file', await until(async () => (await regions()).shown?.length === 1));
  await entry.getByRole('button', { name: 'Open' }).click();
  check('Open asks Go to open it', await until(async () => (await regions()).openedFiles?.length === 1));
  await page.keyboard.press('Escape');
  check('Escape closes the list', await until(async () => (await list.count()) === 0));
  // While it peeks, the pointer over it keeps it open; Escape, or a press outside, closes it.
  await fire('download', 'https://a.test/files/second.pdf');
  await until(() => list.isVisible(), 2000);
  await list.hover();
  await new Promise((r) => setTimeout(r, 6000));
  check('the pointer over it keeps it open', await list.isVisible());
  await page.mouse.move(20, 600);
  check('and once it leaves, it closes after a while', await until(async () => (await list.count()) === 0, 8000));
  await fire('download', 'https://a.test/files/third.pdf');
  await until(() => list.isVisible(), 2000);
  await page.keyboard.press('Escape');
  check('Escape closes it at once', await until(async () => (await list.count()) === 0, 1000));
  await fire('download', 'https://a.test/files/fourth.pdf');
  await until(() => list.isVisible(), 2000);
  // Peeking, it has no cover: the page under it is still the page's, and a press there
  // closes it.
  await settle(page);
  const pageAt = (await regions()).visible[0].rect;
  const outside = { x: pageAt.x + pageAt.w - 40, y: pageAt.y + pageAt.h - 40 };
  check('while it peeks, the page around it is still the page\'s', (await regions(outside)).hit === 'page');
  await page.mouse.click(outside.x, outside.y);
  check('and a press on the page closes it', await until(async () => (await list.count()) === 0, 1000));
  await fire('download', 'https://a.test/files/fifth.pdf');
  await until(() => list.isVisible(), 2000);
  await ui.getByRole('combobox', { name: 'Address' }).click();
  check('and so does a press in the shell outside it', await until(async () => (await list.count()) === 0, 1000));
  await page.keyboard.press('Escape');
  await fire('download', 'https://a.test/files/sixth.pdf');
  await until(() => list.isVisible(), 2000);
  await control.click();
  check('a press on the control makes it the person\'s list: it stays, with the keyboard in it', await until(async () => (await list.isVisible()) && (await list.evaluate((el) => el.contains(document.activeElement) && el !== document.activeElement))));
  await new Promise((r) => setTimeout(r, 5500));
  check('past five seconds', await list.isVisible());
  await page.keyboard.press('Escape');
  await until(async () => (await list.count()) === 0);
  await control.click();
  await list.getByRole('button', { name: 'Clear' }).click();
  check('Clear empties the list, and the control goes', await until(async () => (await control.count()) === 0));

  // A page's dialogs: drawn over the window while its tab shows, answered back to Go.
  const ask = ui.getByRole('alertdialog');
  await fire('prompt', 'Your name?');
  check('a prompt shows as an alertdialog with its words and a field', await until(async () => (await ask.isVisible()) && (await ask.innerText()).includes('Your name?') && (await ask.getByRole('textbox').count()) === 1));
  check('its field starts with the page\'s text', (await ask.getByRole('textbox').inputValue()) === 'mock');
  await shoot('prompt');
  const t1 = (await regions()).active;
  await fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'newTab', text: 'https://b.test/' }) });
  check('another tab shown hides it', await until(async () => (await ask.count()) === 0 && (await regions()).active !== t1));
  check('and its tab\'s row says it waits', await until(async () => (await ui.locator(`[role=tab][data-tab="${t1}"] [aria-label="Waiting for an answer"]`).count()) === 1));
  await ui.locator(`[role=tab][data-tab="${t1}"]`).click();
  check('and showing its tab brings it back', await until(() => ask.isVisible()));
  await ask.getByRole('textbox').focus();
  await page.keyboard.press('Tab');
  await page.keyboard.press('Tab');
  await page.keyboard.press('Tab');
  check('Tab goes round the card\'s controls', await ask.getByRole('textbox').evaluate((el) => el === document.activeElement));
  await ask.getByRole('textbox').fill('Ada');
  await ask.getByRole('button', { name: 'OK' }).click();
  check('OK answers the page with the text', await until(async () => JSON.stringify((await regions()).dialogAnswers) === JSON.stringify([{ id: 1, ok: true, text: 'Ada' }])), JSON.stringify((await regions()).dialogAnswers));
  check('and the dialog goes', await until(async () => (await ask.count()) === 0));
  await fire('alert', 'Saved.');
  await until(() => ask.isVisible());
  check('an alert has only OK', (await ask.getByRole('button').count()) === 1);
  await page.keyboard.press('Escape');
  check('Escape dismisses it', await until(async () => (await ask.count()) === 0 && (await regions()).dialogAnswers?.length === 2));
  await fire('beforeunload', '');
  await until(() => ask.isVisible());
  check('leaving asks with Leave and Cancel', (await ask.getByRole('button', { name: 'Leave' }).count()) === 1 && (await ask.getByRole('button', { name: 'Cancel' }).count()) === 1);
  await ask.getByRole('button', { name: 'Cancel' }).click();
  check('Cancel answers no', await until(async () => (await regions()).dialogAnswers?.[2]?.ok === false));

  // Find in page: the find key opens the bar with the keyboard in it.
  const finder = ui.getByRole('textbox', { name: 'Find in page' });
  await fire('menu', 'find.open');
  check('the find key opens a field named Find in page, with the keyboard in it', await until(async () => (await finder.isVisible()) && (await finder.evaluate((el) => el === document.activeElement))));
  await finder.pressSequentially('mock');
  const bar = ui.getByRole('search', { name: 'Find in page' });
  check('typing searches, and the count shows', await until(async () => (await bar.innerText()).includes('1 of 3')));
  await shoot('find');
  await finder.press('Enter');
  check('Enter shows the next match', await until(async () => (await bar.innerText()).includes('2 of 3')));
  await finder.press('Shift+Enter');
  await finder.press('Shift+Enter');
  check('Shift+Enter the one before', await until(async () => (await bar.innerText()).includes('3 of 3')));
  await finder.fill('nothing');
  await finder.dispatchEvent('input');
  check('a search with no match says so', await until(async () => (await bar.innerText()).includes('No results')));
  await finder.press('Escape');
  check('Escape closes it, and Go ends the search', await until(async () => (await finder.count()) === 0 && (await regions()).findsStopped?.length > 0));

  // A site's sign-in: the dialog with a username and a password.
  await fire('signin', 'Staff only');
  check('a sign-in shows as an alertdialog with Username and Password', await until(async () => (await ask.isVisible()) && (await ask.getByRole('textbox', { name: 'Username' }).count()) === 1 && (await ask.getByLabel('Password').count()) === 1));
  check('with the site\'s words quoted as the site\'s', (await ask.innerText()).includes('The site says: “Staff only”'));
  check('and the host in bold', (await ask.locator('b').innerText()) === 'secure.mock.test');
  await ask.getByRole('textbox', { name: 'Username' }).fill('ada');
  await ask.getByLabel('Password').fill('secret');
  await shoot('sign-in');
  await ask.getByLabel('Password').press('Enter');
  check('Enter signs in with both', await until(async () => (await regions()).dialogAnswers?.at(-1)?.text === 'ada:secret'), JSON.stringify((await regions()).dialogAnswers));
  await fire('signin', 'http:Staff only');
  check('over http, it warns the password goes as plain text', await until(async () => (await ask.isVisible()) && (await ask.innerText()).includes('plain text')));
  await page.keyboard.press('Escape');
  await until(async () => (await ask.count()) === 0);

  // A page filling the screen has the whole window, and Escape gives it back.
  const before = (await regions()).visible[0].rect;
  await fire('pageFullscreen', '');
  const screen = page.viewportSize();
  const covers = async () => {
    const r = (await regions()).visible;
    return r.length === 1 && r[0].rect.x === 0 && r[0].rect.y === 0 && Math.round(r[0].rect.w) === screen.width && Math.round(r[0].rect.h) === screen.height;
  };
  check('a page in full screen covers the whole window', await until(covers), JSON.stringify((await regions()).visible));
  check('with no control of the shell on it', (await ui.getByRole('button', { name: 'New Tab' }).count()) === 0);
  await shoot('page-full-screen');
  await fire('key', 'escape');
  check('Escape gives the window back as it was', await until(async () => JSON.stringify((await regions()).visible[0].rect) === JSON.stringify(before)), JSON.stringify({ before, now: (await regions()).visible[0].rect }));
  check('and the address bar still shows the page\'s address', await until(async () => (await ui.getByRole('combobox', { name: 'Address' }).inputValue()).includes('a.test')), await ui.getByRole('combobox', { name: 'Address' }).inputValue().catch((e) => e.message));

  // The ☰ menu: the menu bar's menus for this window, as Go gives them as it opens
  // (docs/menus-shortcuts.md, "The shell's menu"), Settings… first, nothing of the whole
  // program's; a submenu at the pointer; choosing an item runs its action.
  await ui.getByRole('button', { name: 'Menu', exact: true }).click();
  const top = ui.getByRole('menu', { name: 'Menu' });
  check('the ☰ menu opens with Settings… first', await until(async () => (await top.getByRole('menuitem').first().innerText()).startsWith('Settings…')));
  check('with the menu bar\'s menus, and not Quit', (await Promise.all(['File', 'View', 'History', 'Bookmarks', 'Tab'].map((n) => top.getByRole('menuitem', { name: n, exact: true }).count()))).every((n) => n === 1) &&
    (await ui.getByRole('menuitem', { name: /^Quit/ }).count()) === 0);
  await top.getByRole('menuitem', { name: 'View', exact: true }).hover();
  const zoomIn = ui.getByRole('menuitem', { name: /^Zoom In/ });
  check('View opens its submenu, with its keys', await until(async () => (await zoomIn.isVisible()) && (await zoomIn.innerText()).includes('⌘')));
  await top.getByRole('menuitem', { name: /^Downloads/ }).click();
  check('choosing Downloads runs it: the Downloads page opens, and the menu closes', await until(async () => (await regions()).tabs.some((t) => t.page === 'downloads') && (await top.count()) === 0));

  // The address field completes a visited address in place, the rest selected after the
  // caret, from Go's first row (docs/daily-driver.md, Inline completion).
  const send = (msg) => fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify(msg) });
  const field = ui.getByRole('combobox', { name: 'Address' });
  const shown = () => field.evaluate((el) => [el.value, el.selectionStart, el.selectionEnd]);
  const shows = (value, start, end) => until(async () => JSON.stringify(await shown()) === JSON.stringify([value, start, end]));
  // The active tab, as Go has it.
  const current = async () => {
    const g = await regions();
    return g.tabs.find((t) => t.id === g.active);
  };
  await send({ t: 'newTab', text: 'https://www.wikipedia.test/wiki/Web_browser' });
  await until(async () => (await current())?.title.includes('wikipedia.test'));
  await send({ t: 'bookmark', url: 'https://recipes.test/bread', title: 'Bread recipes' });
  await send({ t: 'newTab', text: 'https://b.test/' });
  await until(async () => (await current())?.url === 'https://b.test/');
  await field.click();
  await page.keyboard.type('wikip');
  check('typing the start of a visited host completes it in the field, the rest selected', await shows('wikipedia.test', 5, 14), JSON.stringify(await shown()));
  const rowsNow = ui.locator('#suggestions [role=option]');
  check('and the first row is that address, with no heading over it', (await rowsNow.first().innerText()).includes('wikipedia.test') && (await ui.locator('#suggestions > li').first().getAttribute('role')) === 'option');
  await shoot('inline-completion');
  await page.keyboard.type('e');
  check('a key that matches carries on with the completion', await shows('wikipedia.test', 6, 14), JSON.stringify(await shown()));
  await page.keyboard.press('Backspace');
  check('Backspace takes away the completion first', await shows('wikipe', 6, 6), JSON.stringify(await shown()));
  await settle(page);
  check('and it stays away while what was typed is kept', (await shows('wikipe', 6, 6)) && (await rowsNow.first().getAttribute('data-kind')) === 'search');
  await page.keyboard.type('d');
  check('typing again completes again', await shows('wikipedia.test', 7, 14), JSON.stringify(await shown()));
  await page.keyboard.press('ArrowDown');
  check('the arrows replace the completion with the row chosen', (await field.inputValue()) === 'https://www.wikipedia.test/', await field.inputValue());
  await page.keyboard.press('ArrowUp');
  check('and coming back up shows what was typed', await shows('wikiped', 7, 7), JSON.stringify(await shown()));
  await field.fill('');
  await page.keyboard.type('wikip');
  await shows('wikipedia.test', 5, 14);
  await page.keyboard.press('Enter');
  check('Enter goes where the field shows', await until(async () => (await current())?.url === 'https://www.wikipedia.test/'), JSON.stringify(await current()));

  // Enter takes a row only while it belongs to the text in the field. Keys and Enter sent
  // in one go, before Go can answer the new text, as a fast typist's do: the list on screen
  // is still the earlier text's.
  const typeAndEnter = (more) => field.evaluate((el, more) => {
    document.execCommand('insertText', false, more);
    el.form.requestSubmit();
  }, more);
  await field.click();
  await page.keyboard.type('wikip');
  await shows('wikipedia.test', 5, 14);
  await typeAndEnter('edia.example/typed');
  check('typed past the completion with Enter before Go answers, Enter goes to what was typed, not the earlier first row', await until(async () => (await current())?.url === 'https://wikipedia.example/typed'), JSON.stringify(await current()));
  // Both hosts are visited now: the field completes the one Go puts first.
  await field.click();
  await page.keyboard.type('wikip');
  const completed = async () => {
    const [value, start, end] = await shown();
    return start === 5 && end === value.length && value.length > 5 ? value : '';
  };
  await until(completed);
  const host = await completed();
  // The key as the browser first offers it, which the field takes to carry the completion
  // on (execCommand offers none).
  await field.evaluate((el) => {
    el.dispatchEvent(new InputEvent('beforeinput', { inputType: 'insertText', data: 'e', cancelable: true, bubbles: true }));
    el.form.requestSubmit();
  });
  check(`a key that carries the completion on, with Enter before Go answers, goes where the field shows, ${host}`, !!host && await until(async () => new URL((await current())?.url || 'about:blank').host.replace(/^www\./, '') === host), JSON.stringify(await current()));
  await field.click();
  await page.keyboard.type('bread');
  await until(async () => (await ui.locator('#suggestions [data-kind=bookmark]').count()) === 1);
  await field.evaluate((el) => {
    el.select();
    document.execCommand('insertText', false, 'b.test/fast');
    el.form.requestSubmit();
  });
  check('with no completion, a first row for an earlier text never takes Enter either', await until(async () => (await current())?.url === 'https://b.test/fast'), JSON.stringify(await current()));

  // Bookmarks whose title or address holds the text are rows, with the star.
  await field.click();
  await page.keyboard.type('bread');
  const mark = ui.locator('#suggestions [data-kind=bookmark]');
  check('a bookmark whose title holds the text is a row, under Bookmarks', await until(async () => (await mark.count()) === 1 && (await mark.innerText()).includes('Bread recipes') && (await ui.locator('#suggestions .heading').allInnerTexts()).includes('Bookmarks')));
  await shoot('bookmark-rows');
  await page.keyboard.press('Escape');
  await page.keyboard.press('Escape');

  // Escape: the first drops the list and the completion, the second puts the page's address
  // back, even with the find bar open, which once took the second one.
  const page1 = (await current()).url;
  for (const findOpen of [false, true]) {
    if (findOpen) {
      await fire('menu', 'find.open');
      await until(() => finder.isVisible());
    }
    const how = findOpen ? ', with the find bar open' : '';
    await field.click();
    await page.keyboard.type('wikip');
    await shows('wikipedia.test', 5, 14);
    await page.keyboard.press('Escape');
    check(`the first Escape closes the list and drops the completion${how}`, (await shows('wikip', 5, 5)) && (await ui.locator('#suggestions').count()) === 0, JSON.stringify(await shown()));
    await page.keyboard.press('Escape');
    check(`the second puts the page's address back${how}`, await until(async () => (await field.inputValue()) === page1), await field.inputValue());
    check(`and gives the page the keyboard${how}`, await until(async () => (await regions()).focused === (await regions()).active));
  }
  check('the find bar stays open through it', await finder.isVisible());
  await finder.press('Escape');

  // An http page says Not secure in words at the field's start, in --danger, and the words
  // are still the button to the site's panel.
  await send({ t: 'navigate', text: 'http://plain.test/' });
  await until(async () => (await current())?.connection === 'insecure');
  const warn = ui.getByRole('button', { name: 'Not secure · Site settings' });
  check('an http page shows Not secure in words at the field\'s start', await until(async () => (await warn.isVisible()) && (await warn.innerText()).trim() === 'Not secure'));
  const look = await warn.evaluate((el) => {
    const probe = document.createElement('span');
    probe.style.color = 'var(--danger)';
    document.body.append(probe);
    const danger = getComputedStyle(probe).color;
    probe.remove();
    const field = el.closest('form').getBoundingClientRect();
    const r = el.getBoundingClientRect();
    return { danger, color: getComputedStyle(el).color, h: r.height, w: r.width, left: r.left - field.left, size: getComputedStyle(el.querySelector('.words')).fontSize };
  });
  check('in --danger, 12px words, no taller than the field and at its start', look.color === look.danger && look.size === '12px' && look.h <= 32 && look.left <= 1 && look.w < 120, JSON.stringify(look));
  // The default is None, which the window paints in its light grey.
  for (const [name, colour, painted] of [['default', '', '#dcdee2'], ['dark-frame', '#384562', '#384562'], ['light-frame', '#faa804', '#faa804']]) {
    await send({ t: 'setPref', name: 'frameColor', value: colour });
    await until(async () => (await regions()).windowColor === painted);
    await shoot(`not-secure-${name}`);
  }
  await send({ t: 'setPref', name: 'frameColor', value: '' });
  await warn.click();
  check('a press on it opens the site\'s panel', await until(() => ui.getByRole('dialog', { name: 'Site settings for plain.test' }).isVisible()));
  await page.keyboard.press('Escape');

  check('no errors', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})().catch(async (e) => {
  check('the checks ran to their end', false, e.message.split('\n').slice(0, 12).join(' / '));
  await lib.done();
});
