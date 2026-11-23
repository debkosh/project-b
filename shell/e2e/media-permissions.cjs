// A page's camera and microphone when the OS refuses what the site is allowed
// (docs/media-permissions.md), on the mock: the site's question, then the OS's no as the
// prompt's state in the OS's words, Open settings and Not now, and the site panel's line, in
// Standard and Glide (the mock playing macOS) and Mobile (playing Android), with pictures of
// each for ux.

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');

const { check, fire, regions, settle, until } = lib;

async function shots(page, what) {
  for (const scheme of ['light', 'dark']) {
    await page.emulateMedia({ colorScheme: scheme });
    await settle(page);
    await page.screenshot({ path: path.join(lib.out, `media-permissions-${what}-${scheme}.png`) });
  }
  await page.emulateMedia({ colorScheme: 'light' });
}

// The checks in one shell: os is the OS's name in its words.
async function inShell(browser, id, os, viewport, hasTouch) {
  // Each shell meets the site as a new profile does: what the one before allowed would
  // answer its question with no prompt.
  fs.rmSync(path.join(lib.profile, 'sites.json'), { force: true });
  await lib.start([`-use=${id}`, ...(os === 'Android' ? ['-platform=android'] : [])]);
  const w = await lib.openWindow(browser, { viewport, hasTouch });
  await until(async () => (await regions()).active);
  const tab = (await regions()).active;
  await fire('navigate', 'https://meet.example/call', { tab });
  await until(async () => {
    const t = (await regions()).tabs.find((x) => x.id === tab);
    return t?.url === 'https://meet.example/call' && !t.loading;
  });
  await fire('permission', 'camera,microphone', { tab });
  const site = w.ui.getByRole('dialog', { name: 'meet.example wants to use your camera and microphone' });
  check(`${id}: the site's question`, await until(() => site.isVisible()));
  await shots(w.page, `${id}-question`);
  await site.getByRole('button', { name: 'Always allow', exact: true }).click();
  await until(async () => (await regions()).permissions.length === 1);

  await fire('mediaOS', 'refuse camera microphone');
  await fire('permission', 'camera,microphone', { tab });
  const title = `${os} hasn't let Project B use your camera and microphone`;
  const card = w.ui.getByRole('dialog', { name: title });
  check(`${id}: the OS's no, in its words, with Open settings and Not now`,
    (await until(() => card.isVisible())) &&
      (await card.getByRole('button', { name: 'Open settings' }).count()) === 1 &&
      (await card.getByRole('button', { name: 'Not now' }).count()) === 1);
  const line = os === 'Android'
    ? "So meet.example can't use them, though you've allowed them. Turn them on in Project B's app settings."
    : "So meet.example can't use them, though you've allowed them. Turn on Project B in System Settings › Privacy & Security › Camera and Microphone.";
  check(`${id}: and the line saying why`, (await card.innerText()).includes(line), await card.innerText());
  await shots(w.page, `${id}-os`);
  await card.getByRole('button', { name: 'Not now' }).click();
  check(`${id}: Not now denies the page`, await until(async () => JSON.stringify((await regions()).permissions[1]?.statuses) === '[2,2]'));
  check('no errors in the page', w.errors.length === 0, w.errors.join(' | '));
  await w.page.close();
  await lib.stop();
}

(async () => {
  const browser = await chromium.launch();
  await inShell(browser, 'standard', 'macOS', { width: 1100, height: 700 }, false);
  await inShell(browser, 'glide', 'macOS', { width: 1100, height: 700 }, false);
  await inShell(browser, 'mobile', 'Android', { width: 412, height: 915 }, true);
  await browser.close();
  await lib.done();
})();
