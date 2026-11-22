// Release notes per platform (shell/src/apps/settings/platform-notes.js): a line that starts
// with its platform shows only there, and every other line everywhere. No page and no Go: the
// rules alone, on 0.3.10's own kinds of line.
// Run: node shell/e2e/platform-notes.cjs   (from the repo root)

const path = require('path');
const { pathToFileURL } = require('url');

let failed = 0;
const check = (name, ok, detail) => {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${ok ? '' : ` -> ${JSON.stringify(detail)}`}`);
  if (!ok) failed++;
};

(async () => {
  const { platformOf, forPlatform, here } = await import(
    pathToFileURL(path.join(__dirname, '../src/apps/settings/platform-notes.js')).href);

  const lines = [
    'On Dev, you get each Stable release too',
    'On Linux, tabs, history and settings are kept between starts',
    'On a Mac the update shows a progress bar on the Dock icon',
    'On a phone, an update shows as a notification while it downloads',
    'On Android 10 the first letter you type is no longer doubled',
    'The crash notice names the log\'s file',
    'Online, the page loads',
  ];
  check('a line is for the platform it starts with',
    lines.map(platformOf).join(',') === ',linux,mac,android,android,,', lines.map(platformOf));
  const mac = forPlatform(lines, 'mac');
  check('a Mac shows its own line and the general ones, in order',
    JSON.stringify(mac) === JSON.stringify([lines[0], lines[2], lines[5], lines[6]]), mac);
  const linux = forPlatform(lines, 'linux');
  check('Linux shows its own line and the general ones',
    JSON.stringify(linux) === JSON.stringify([lines[0], lines[1], lines[5], lines[6]]), linux);
  const android = forPlatform(lines, 'android');
  check('Android shows the phone and Android lines and the general ones',
    JSON.stringify(android) === JSON.stringify([lines[0], lines[3], lines[4], lines[5], lines[6]]), android);
  check('Android is told by the user agent, as Go calls its keymap linux',
    here('linux', 'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36') === 'android', null);
  check('a Mac and Linux are the keymap\'s platform',
    here('mac', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)') === 'mac' &&
    here('linux', 'Mozilla/5.0 (X11; Linux aarch64)') === 'linux', null);

  console.log(failed ? `\n${failed} FAILED` : '\nall passed');
  process.exit(failed ? 1 : 0);
})();
