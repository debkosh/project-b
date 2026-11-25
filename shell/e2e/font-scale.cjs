// Android's font size on our pages, on the mock engine (docs/font-size.md): at 1.3 and 2.0,
// as Android's larger sizes give, our text grows by that much, and nothing of it is cut off
// by the box it is in, in the phone's shell and in Standard on a tablet: the shell's bars,
// Settings, Welcome, History, Bookmarks, Downloads and the page dialogs. The size comes as
// Chrome's DevTools give it (Emulation.setEmulatedOSTextScale), which Blink takes as it takes
// the engine's font_scale_factor. It leaves screenshots in build/e2e/font-*.png.
// Run: node shell/e2e/font-scale.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, fire, regions, settle, until } = lib;

const scales = [1, 1.3, 2];
// address: what shows the page's address in the shell's bar, whose text is held at 1.3.
const setups = [
  { name: 'phone', shell: 'mwshell', viewport: { width: 412, height: 915 }, touch: true, address: 'span.url' },
  { name: 'tablet', shell: 'pbshell', viewport: { width: 1024, height: 768 }, touch: true, address: 'input[aria-label=Address]' },
];
// The bars' text grows with the font size up to this much, as Chrome's toolbar barely
// follows it; the pages and their sheets and dialogs grow all the way.
const barCap = 1.3;

// cut lists the text in a document that a box it is in cuts off: lines whose ink lies past
// the edge of an ancestor that hides what overflows it. A box that scrolls shows the rest by
// scrolling, an ellipsis or a line clamp cuts on purpose, and text kept for screen readers
// only (a box of a pixel) is not drawn at all.
function cut() {
  const out = [];
  const walk = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  const range = document.createRange();
  for (let node = walk.nextNode(); node; node = walk.nextNode()) {
    const text = node.textContent.trim();
    if (!text) continue;
    const el = node.parentElement;
    const s = getComputedStyle(el);
    if (s.visibility === 'hidden' || el.closest('[hidden], [aria-hidden=true]')) continue;
    range.selectNodeContents(node);
    const ink = [...range.getClientRects()].filter((r) => r.width > 0 && r.height > 0);
    if (!ink.length) continue;
    for (let p = el; p && p !== document.documentElement; p = p.parentElement) {
      const ps = getComputedStyle(p);
      if (ps.display === 'none' || Number(ps.opacity) === 0) break;
      // A line clamp drops the lines past it on purpose, with an ellipsis, but not what a
      // line pushes out sideways.
      const clamped = ps.webkitLineClamp && ps.webkitLineClamp !== 'none';
      const hidesY = !clamped && (ps.overflowY === 'hidden' || ps.overflowY === 'clip');
      const hidesX = ps.overflowX === 'hidden' || ps.overflowX === 'clip';
      if (!hidesY && !hidesX) continue;
      const box = p.getBoundingClientRect();
      if (box.width <= 1 || box.height <= 1) break;
      const over = ink.some((r) => (hidesY && (r.top < box.top - 1 || r.bottom > box.bottom + 1))
        || (hidesX && ps.textOverflow !== 'ellipsis' && s.textOverflow !== 'ellipsis' && (r.left < box.left - 1 || r.right > box.right + 1)));
      if (over) {
        const at = ink[0];
        out.push(`"${text.slice(0, 30)}" in ${p.tagName.toLowerCase()}.${[...p.classList].join('.')} (${Math.round(box.height)}px high, text ${Math.round(at.height)}px)`);
        break;
      }
    }
  }
  return [...new Set(out)];
}

// The shells Android never shows: Android's font size never reaches them, so their text may
// stay in px. Everything else of ours sizes its text in rem (calc(13rem / 16) for 13px), as
// px text stays the size it was at any font size.
const desktopOnly = ['arshell', 'fxshell', 'mnshell', 'tmshell'].map((s) => `apps/shell/${s}/`);
function pxText() {
  const src = path.join(lib.root, 'shell/src');
  const found = [];
  const walk = (dir) => {
    for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
      const at = path.join(dir, e.name);
      const rel = path.relative(src, at) + (e.isDirectory() ? '/' : '');
      if (desktopOnly.some((d) => rel.startsWith(d))) continue;
      if (e.isDirectory()) walk(at);
      else if (/\.(css|vue|svelte)$/.test(e.name)) {
        fs.readFileSync(at, 'utf8').split('\n').forEach((line, i) => {
          if (/(font-size|line-height):\s*[\d.]+px|\bfont:[^;]*?[\d.]+px/.test(line)) found.push(`${rel}:${i + 1}`);
        });
      }
    }
  };
  walk(src);
  return found;
}

(async () => {
  const px = pxText();
  check('our pages size their text in rem, not px, so it grows with Android\'s font size', px.length === 0, px.join(', '));
  const browser = await chromium.launch();
  fs.writeFileSync(path.join(lib.profile, 'flags.json'), JSON.stringify({ showWelcome: 'on' }));
  for (const setup of setups.filter((s) => !process.env.ONLY || s.name === process.env.ONLY)) {
    await lib.start([`-use=${setup.shell}`]);
    const w = await lib.openWindow(browser, { viewport: setup.viewport, hasTouch: setup.touch });
    const { page, ui } = w;
    page.setDefaultTimeout(5000);
    const cdp = await page.context().newCDPSession(page);
    await until(async () => (await regions()).shell === setup.shell, 10000);
    const frames = (name) => [w.shell(), ...(name ? [page.frames().find((f) => f.name() === name || f.url().includes(`/${name}/`))] : [])].filter(Boolean);
    const scaleAll = async (name, what, ready) => {
      let base = null;
      let bar = null;
      for (const scale of scales) {
        await cdp.send('Emulation.setEmulatedOSTextScale', { scale });
        await settle(page);
        if (ready) await ready();
        // Our text, read where the page draws it. The root keeps the browser's medium, so rem
        // is 16px at 1, and the body's text, 14px at 1, grows all the way.
        const ours = frames(name).at(-1);
        const [root, body] = await ours.evaluate(() => [document.documentElement, document.body].map((el) => parseFloat(getComputedStyle(el).fontSize)));
        if (scale === 1) {
          base = body;
          check(`${setup.name}, ${what} at 1: the root is the browser's 16px, and our text 14px`, root === 16 && body === 14, `root ${root}px, body ${body}px`);
        } else check(`${setup.name}, ${what} at ${scale}: our text grows with it`, Math.abs(body - base * scale) <= 1, `${body}px, ${base}px at 1`);
        const size = await w.shell().evaluate((sel) => {
          const el = [...document.querySelectorAll(sel)].find((e) => e.getClientRects().length);
          return el ? parseFloat(getComputedStyle(el).fontSize) : 0;
        }, setup.address);
        if (scale === 1) bar = size;
        else if (size) check(`${setup.name}, ${what} at ${scale}: the address in the bar grows to ${barCap} times at most`, Math.abs(size - bar * Math.min(scale, barCap)) <= 1, `${size}px, ${bar}px at 1`);
        let found = [];
        await until(async () => {
          found = (await Promise.all(frames(name).map((f) => f.evaluate(cut)))).flat();
          return found.length === 0;
        }, 2000);
        check(`${setup.name}, ${what} at ${scale}: no text is cut off`, found.length === 0, found.join('; '));
        await page.screenshot({ path: path.join(lib.out, `font-${setup.name}-${what.replace(/\W+/g, '-').toLowerCase()}-${scale}.png`) });
      }
      await cdp.send('Emulation.setEmulatedOSTextScale', {});
    };
    const showing = (name) => until(async () => {
      const r = await regions();
      return r.tabs.find((t) => t.id === r.active)?.page === name && page.frames().some((f) => f.url().includes(`/${name}/`));
    }, 10000);

    // Welcome first: the flag opens it at the start.
    // Each of ours is closed once checked: the window holds only so many of our pages.
    const closeOurs = async () => {
      const r = await regions();
      await lib.closeTab(r.active);
      await until(async () => (await regions()).tabs.every((t) => t.id !== r.active));
    };
    // The welcome goes once a page of the web is open: it is the window's one tab till then.
    const welcome = (await showing('welcome')) ? (await regions()).active : 0;
    // Its first step, past the mark it opens with: the first screen a person sees.
    const welcomeFrame = () => page.frameLocator('iframe.our-page[data-page="welcome"]');
    const firstStep = async () => {
      await welcomeFrame().getByRole('heading').first().waitFor({ timeout: 10000 });
      await welcomeFrame().locator('body').evaluate(() => Promise.all(document.getAnimations().filter((a) => a.effect?.getComputedTiming().iterations !== Infinity).map((a) => a.finished.catch(() => {}))));
      await settle(page);
    };
    if (welcome) await scaleAll('welcome', 'Welcome', firstStep);
    else check(`${setup.name}: the welcome opens at the start`, false);

    // A page of the web with the shell's bars, and its dialogs over it: a site of its own
    // for each, as the profile keeps the answer the one before gave its camera.
    const site = `https://${setup.name}.example.com`;
    await fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'newTab', text: `${site}/a-rather-long-path/to/see` }) });
    await until(async () => {
      const r = await regions();
      return r.tabs.find((t) => t.id === r.active)?.url.startsWith(site) && r.visible.length > 0;
    }, 10000);
    if (welcome) await lib.closeTab(welcome);
    await until(async () => (await regions()).tabs.every((t) => t.id !== welcome));
    await scaleAll(null, 'the bars');
    const tab = (await regions()).active;
    for (const [kind, value, what] of [['confirm', 'Go on with what was asked, or not?', 'a confirm'], ['prompt', 'Name?', 'a prompt'], ['signin', 'Staff only', 'a sign-in'], ['permission', 'camera', 'a permission prompt']]) {
      // After the one before, the tab may be loading again, as a sign-in cancelled shows
      // its page: a question then waits till it has.
      await until(async () => !(await regions()).tabs.find((t) => t.id === tab)?.loading);
      await settle(page);
      await fire(kind, value, { tab });
      const shown = await until(() => ui.locator('[role=dialog], [role=alertdialog]').filter({ visible: true }).count().then((n) => n > 0), 5000);
      if (!check(`${setup.name}: ${what} shows`, shown, JSON.stringify([(await regions()).tabs.find((t) => t.id === tab)?.loading, await w.shell().evaluate(() => [...document.querySelectorAll('[role=dialog], [role=alertdialog]')].map((e) => [e.className.slice(0, 30), e.getClientRects().length, e.textContent.trim().slice(0, 50)]))]))) continue;
      await scaleAll(null, what);
      const no = ui.getByRole('button', { name: /^(Cancel|Not now|Block|Don.t allow)$/ }).filter({ visible: true }).first();
      if (await no.count()) await no.click();
      else await page.keyboard.press('Escape');
      await until(async () => !(await ui.locator('[role=dialog], [role=alertdialog]').filter({ visible: true }).count().then((n) => n > 0)), 5000);
    }

    for (const [action, name, what] of [['app.settings', 'settings', 'Settings'], ['history.show', 'history', 'History'], ['bookmarks.show', 'bookmarks', 'Bookmarks'], ['downloads.show', 'downloads', 'Downloads']]) {
      await fire('menu', action);
      if (!check(`${setup.name}: ${what} opens`, await showing(name), JSON.stringify([(await regions()).active, (await regions()).tabs.map((t) => [t.id, t.page, t.url]), page.frames().map((f) => f.url().replace(lib.base, ''))]))) continue;
      await scaleAll(name, what);
      await closeOurs();
    }
    check(`${setup.name}: no errors in the page`, w.errors.length === 0, w.errors.join(' | '));
    await w.page.close();
    await lib.stop();
  }
  await browser.close();
  await lib.done();
})();
