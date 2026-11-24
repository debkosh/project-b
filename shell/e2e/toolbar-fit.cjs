// Every shell's own controls keep apart at every window width: no button, field or tab of
// the shell's bars lies over another, however narrow or wide the window is. Each shell runs
// with three tabs open, on a page, in desktop windows from narrow to wide, and the phone's
// shell at a phone's widths. What floats over the page (notices, dialogs) is not the bars',
// so only controls outside the page are checked.
// Run: node shell/e2e/toolbar-fit.cjs   (from the repo root, after `npm run build` in shell/)

const { chromium } = require('playwright');
const lib = require('./lib.cjs');
const { check, regions, settle, until } = lib;

const desktop = [500, 640, 760, 841, 960, 1100, 1440];
const phone = [320, 360, 412];
const shells = [
  ['standard', desktop],
  ['fox', desktop],
  ['minimal', desktop],
  ['glide', desktop],
  ['terminal', desktop],
  ['mobile', phone, true],
];

// overlaps lists each pair of the shell's visible controls, outside the page, whose boxes
// cross by more than a pixel each way. A control inside another (a tab's close button in
// its tab) is one control.
async function overlaps(w, pageRect) {
  const frame = w.shell();
  const at = await w.page.locator('iframe.shell').boundingBox();
  return frame.evaluate(({ pageRect, at }) => {
    const sel = 'button, input, select, textarea, a[href], [role=button], [role=tab], [role=textbox], [role=combobox]';
    const shown = [...document.querySelectorAll(sel)].filter((el) => {
      if (el.closest('[role=menu], [role=dialog], [role=listbox], [role=tooltip]')) return false;
      const s = getComputedStyle(el);
      if (s.visibility === 'hidden' || s.display === 'none' || Number(s.opacity) === 0) return false;
      const r = el.getBoundingClientRect();
      return r.width > 1 && r.height > 1;
    });
    const boxes = shown.map((el) => {
      // What is drawn of it: a button's icon and words, not the reach a touch target is
      // given past them, which may tuck under a neighbour's empty edge.
      let r = el.getBoundingClientRect();
      if (el.matches('button, [role=button], a[href]') && el.firstChild) {
        const range = document.createRange();
        range.selectNodeContents(el);
        const ink = range.getBoundingClientRect();
        if (ink.width > 1 && ink.height > 1) r = ink;
      }
      // What its scrolling or clipping parents show of it.
      let { left, top, right, bottom } = r;
      // A text field is where its text goes: its padding may lie under its own buttons.
      if (el.matches('input, textarea')) {
        const s = getComputedStyle(el);
        left += parseFloat(s.paddingLeft);
        right -= parseFloat(s.paddingRight);
      }
      for (let p = el.parentElement; p; p = p.parentElement) {
        const ps = getComputedStyle(p);
        if (ps.overflowX !== 'visible' || ps.overflowY !== 'visible') {
          const pr = p.getBoundingClientRect();
          left = Math.max(left, pr.left);
          top = Math.max(top, pr.top);
          right = Math.min(right, pr.right);
          bottom = Math.min(bottom, pr.bottom);
        }
      }
      return { el, left: left + at.x, top: top + at.y, right: right + at.x, bottom: bottom + at.y };
    }).filter((b) => b.right - b.left > 1 && b.bottom - b.top > 1)
      // The bars' controls: those outside the page.
      .filter((b) => !pageRect || b.right <= pageRect.x + 1 || b.left >= pageRect.x + pageRect.w - 1 || b.bottom <= pageRect.y + 1 || b.top >= pageRect.y + pageRect.h - 1);
    // A field's own button, as the address field's site button, lies inside the field, and
    // both are in a box of the field's size: the field is drawn around it. A field over a
    // bar's button has only the bar around both.
    const inside = (a, o) => a.left >= o.left - 1 && a.right <= o.right + 1 && a.top >= o.top - 1 && a.bottom <= o.bottom + 1;
    const adornment = (a, o) => {
      if (!inside(a, o)) return false;
      let c = o.el.parentElement;
      while (c && !c.contains(a.el)) c = c.parentElement;
      if (!c) return false;
      const r = c.getBoundingClientRect();
      const area = (o.right - o.left) * (o.bottom - o.top);
      return r.width * r.height <= area * 1.5;
    };
    const name = (el) => el.getAttribute('aria-label') || el.getAttribute('title') || el.textContent.trim().slice(0, 30) || el.tagName.toLowerCase();
    const found = [];
    for (let i = 0; i < boxes.length; i++) {
      for (let j = i + 1; j < boxes.length; j++) {
        const a = boxes[i];
        const b = boxes[j];
        if (a.el.contains(b.el) || b.el.contains(a.el)) continue;
        const x = Math.min(a.right, b.right) - Math.max(a.left, b.left);
        const y = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
        if (x <= 1 || y <= 1) continue;
        if (adornment(a, b) || adornment(b, a)) continue;
        found.push(`${name(a.el)} / ${name(b.el)}`);
      }
    }
    return found;
  }, { pageRect, at });
}

(async () => {
  const browser = await chromium.launch();
  for (const [id, widths, touch] of shells) {
    await lib.start([`-use=${id}`]);
    const height = touch ? 800 : 700;
    const w = await lib.openWindow(browser, { viewport: { width: widths[widths.length - 1], height }, hasTouch: !!touch });
    await until(async () => (await regions()).shell === id);
    for (const u of ['https://example.com/news', 'https://example.org/maps', 'https://docs.example/spec']) {
      await fetch(`${lib.base}/shell/send?window=1`, { method: 'POST', body: JSON.stringify({ t: 'newTab', text: u }) });
    }
    await until(async () => (await regions()).visible.length >= 1, 10000);
    for (const width of [...widths].reverse()) {
      await w.page.setViewportSize({ width, height });
      await settle(w.page);
      let found = [];
      await until(async () => {
        const r = await regions();
        found = await overlaps(w, r.visible[0]?.rect);
        return found.length === 0;
      }, 3000);
      check(`${id} at ${width} wide: no control of its bars lies over another`, found.length === 0, found.join(', '));
    }
    check(`${id}: no errors in the page`, w.errors.length === 0, w.errors.join(' | '));
    await w.page.close();
    await lib.stop();
  }
  await browser.close();
  await lib.done();
})();
