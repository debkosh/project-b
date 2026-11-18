// What a test of a shell on the mock engine needs: the mock engine itself, on a free port
// and a profile of its own, so tests run side by side; a window in a real browser; and the
// checks. Every test prints ok or FAIL per check and ends with done().
//
//   const lib = require('./lib.cjs');
//   await lib.start(['-use=minimal']);          // core/cmd/dev, on the built shell/dist
//   const w = await lib.openWindow(browser);    // the OS window, with the shell in it
//   lib.check('a tab shows', (await lib.regions()).visible.length === 1);
//   lib.done();                                 // stops the server, exits 1 if any failed

const { spawn, execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '../..');
// Where tests leave their screenshots.
const out = path.join(root, 'build/e2e');
fs.mkdirSync(out, { recursive: true });
// This run's own folder: the server binary, and the profile, which outlives a restart so a
// test can see what a profile keeps. The server is built with extensions, which run only
// while their flag is on in the profile's flags.json.
const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'project-b-e2e-'));
const profile = path.join(temp, 'profile');
fs.mkdirSync(profile);
const bin = path.join(temp, 'dev');
execFileSync('go', ['build', '-tags', 'sqlite_fts5,extensions', '-o', bin, './cmd/dev'], { cwd: path.join(root, 'core'), stdio: 'inherit' });

// A home folder with no browsers in it, which Import looks in unless a test names its own,
// such as lib.browsers, browserdata's: a Chrome of two profiles and a Firefox of one.
const noBrowsers = path.join(temp, 'no-browsers');
fs.mkdirSync(noBrowsers);

let server = null;
// The running server's address, such as http://127.0.0.1:53124, and what it has printed:
// the timeline, line by line.
const lib = { root, out, profile, base: '', log: '', browsers: path.join(root, 'browserdata/testdata/home') };

// start runs the mock engine with the built pages and the profile, and args, on a port
// the system gives it, and returns once it answers. The profile keeps what it keeps, but
// its windows and tabs start afresh, as the app would otherwise bring back the last run's:
// with keepTabs, they come back as the app brings them back.
lib.start = async (args = [], { keepTabs = false } = {}) => {
  if (!keepTabs) {
    fs.rmSync(path.join(profile, 'session.json'), { force: true });
  }
  server = spawn(bin, ['-addr=127.0.0.1:0', `-shell=${path.join(root, 'shell/dist')}`, `-profile=${profile}`, `-browsers=${noBrowsers}`, ...args], { stdio: ['ignore', 'ignore', 'pipe'] });
  lib.log = '';
  lib.base = await new Promise((resolve, reject) => {
    server.stderr.setEncoding('utf8');
    server.stderr.on('data', (text) => {
      lib.log += text;
      const m = /mock engine on (http:\/\/\S+)/.exec(lib.log);
      if (m) resolve(m[1]);
    });
    server.once('exit', (code) => reject(new Error(`the mock engine ended, ${code}:\n${lib.log}`)));
  });
};

// stop ends the server and returns once it has gone.
lib.stop = async () => {
  const gone = new Promise((r) => server.once('exit', r));
  server.kill('SIGTERM');
  await gone;
  server = null;
};

let failed = 0;
// check prints one check's result, and what went wrong when it did.
lib.check = (name, ok, detail = '') => {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${ok || !detail ? '' : ' -> ' + detail}`);
  if (!ok) failed++;
  return ok;
};
lib.failed = () => failed;

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
// until is whether test came true within ms.
lib.until = async (test, ms = 5000) => {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    if (await test()) return true;
    await sleep(50);
  }
  return false;
};

// settle returns once page has drawn what it holds now.
lib.settle = (page) => page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(r, 60)))));

// regions is what the mock says of a window (core/cmd/dev, /mock/regions): its last layout,
// Go's tabs and active tab, the shell it loads, the tab last given the keyboard, and, with
// x and y, what a press there does.
lib.regions = async ({ window = 1, x, y } = {}) => {
  const q = new URLSearchParams({ window });
  if (x !== undefined) q.set('x', x), q.set('y', y);
  return (await fetch(`${lib.base}/mock/regions?${q}`)).json();
};

// closeTab closes a tab as a shell asks for it.
lib.closeTab = (id, { window = 1 } = {}) =>
  fetch(`${lib.base}/shell/send?window=${window}`, { method: 'POST', body: JSON.stringify({ t: 'closeTab', id }) });

// fire does what a page or the user can (/mock/fire): kind 'menu' with an action's id runs
// it, 'focusTab' is a press in a tab's page, and every button of the mock page is a kind.
lib.fire = (kind, value = '', { window = 1, tab = 0 } = {}) =>
  fetch(`${lib.base}/mock/fire?window=${window}`, { method: 'POST', body: new URLSearchParams({ kind, value, tab }) });

// watch collects a page's errors, its frames' too, into errors.
lib.watch = (page, errors) => {
  page.on('pageerror', (e) => errors.push(e.message));
  page.on('console', (m) => m.type() === 'error' && errors.push(m.text()));
};

// openWindow opens one of Go's windows in a page of browser, the OS window: the shell in a
// frame, and a mock page for each tab the layout shows (core/cmd/dev/window.html); with
// hasTouch, a touch screen's, as a phone's is. With held, the shell's first ready waits
// until release(): Go answers ready with the snapshot and sends a shell nothing before it
// (docs/shells.md, The messages), so until then the shell has none of Go's data. It has
//
//   page          the window, whose mouse, keyboard and screenshots are the window's
//   ui            a locator into the shell's frame, which survives it loading again
//   shell()       the shell's frame itself, to evaluate in
//   mockPage(t)   a locator into the mock page of tab t
//   settings      a locator into the settings page, which the window's settings tab shows
//   errors        every error the window and its frames logged
//   waiting()     with held, whether the shell has said ready and waits for the snapshot
//   release()     with held, lets its ready through, and the snapshot comes
lib.openWindow = async (browser, { window = 1, viewport = { width: 1100, height: 700 }, hasTouch = false, held = false } = {}) => {
  const page = await browser.newPage({ viewport, hasTouch });
  const errors = [];
  lib.watch(page, errors);
  const sends = (url) => url.pathname === '/shell/send';
  let ready = null;
  const hold = (route) => (JSON.parse(route.request().postData()).t === 'ready' ? (ready = route) : route.continue());
  if (held) await page.route(sends, hold);
  // A touch screen in every frame from its first script: our pages read navigator.maxTouchPoints
  // once as they load (src/phone.js), and under a full run's load a page in an iframe now and
  // then loaded before hasTouch's emulation reached it, and drew itself as a desktop's.
  if (hasTouch) await page.addInitScript(() => Object.defineProperty(Navigator.prototype, 'maxTouchPoints', { get: () => 5, configurable: true }));
  // What the shell had drawn when the window first showed its pages: the tabs, for the
  // check that the pages wait for the layout's frame.
  await page.addInitScript(() => {
    if (window !== window.top) return;
    new MutationObserver((records, observer) => {
      if (!document.querySelector('#window.pages-shown')) return;
      observer.disconnect();
      window.pagesShownWith = document.querySelector('iframe.shell').contentDocument.querySelectorAll('[role=tab]').length;
    }).observe(document, { subtree: true, attributes: true, attributeFilter: ['class'] });
  });
  await page.goto(`${lib.base}/?window=${window}`);
  const shellURL = new RegExp(`/shell/[a-z0-9-]+/\\?window=${window}$`);
  return {
    page,
    errors,
    ui: page.frameLocator('iframe.shell'),
    shell: () => page.frame({ url: shellURL }),
    mockPage: (tab) => page.frameLocator(`iframe.mock-page[data-tab="${tab}"]`),
    waiting: () => ready !== null,
    release: async () => {
      await ready.continue();
      await page.unroute(sends, hold);
    },
    settings: page.frameLocator('iframe.our-page[data-page="settings"]'),
  };
};

// done stops the server, says how it went, and ends the test.
lib.done = async () => {
  if (server) await lib.stop();
  fs.rmSync(temp, { recursive: true, force: true });
  console.log(failed ? `\n${failed} FAILED` : '\nall passed');
  process.exit(failed ? 1 : 0);
};

// A test that throws ends here, with its server.
process.on('unhandledRejection', (e) => {
  console.error(e);
  server?.kill('SIGKILL');
  process.exit(1);
});

module.exports = lib;
