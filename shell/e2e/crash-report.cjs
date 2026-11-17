// The crash report on the desktop's shell, Standard: the last run's crash notice offers
// Share report, which writes one text report in the profile, holding the last run's crash
// log, and hands it to the system's share sheet (on the mock, it says what it was given);
// and Save report, which hands the same report to the engine to copy into ~/Downloads.
// Run: node shell/e2e/crash-report.cjs   (from the repo root, after `npm run build` in shell/)

const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, regions, until } = lib;

(async () => {
  fs.writeFileSync(path.join(lib.profile, 'crash.log'), 'pid 1, started 2026-11-14 09:00:00\npanic: the last run\n');
  await lib.start(['-use=standard']);
  const browser = await chromium.launch();
  const w = await lib.openWindow(browser);
  const share = w.ui.getByRole('button', { name: 'Share report' });
  check('the last run\'s crash notice offers Share report', await until(() => share.isVisible(), 10000));
  await share.click();
  const shared = await until(async () => (await regions()).shared);
  const report = shared ? fs.readFileSync((await regions()).shared, 'utf8') : '';
  check('it hands the system one text report in the profile, with the last run\'s crash log in it',
    /reports\/project-b-report-.*Z\.txt$/.test((await regions()).shared) && report.includes('panic: the last run'), (await regions()).shared);
  await w.ui.getByRole('button', { name: 'Save report' }).click();
  await until(async () => (await regions()).saved);
  const saved = (await regions()).saved;
  check('Save report hands the engine a report to copy into Downloads, with the crash log in it',
    /reports\/project-b-report-.*Z\.txt$/.test(saved || '') && fs.readFileSync(saved, 'utf8').includes('panic: the last run'), saved);
  check('no errors in the page', w.errors.length === 0, w.errors.join(' | '));
  await browser.close();
  await lib.done();
})();
