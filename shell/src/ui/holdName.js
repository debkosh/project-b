// A phone's icon button says its name when held, as Android's own tooltips do: a long press,
// half a second without moving, shows the name in a small dark label over the button, the
// lift doesn't press it, and the label goes a moment after. The name is the button's
// aria-label too, for TalkBack. For the shells' pages in a WebView, which have no Android
// tooltip (docs/design-system.md, "On a phone"). A Svelte action as it is, use:holdName={name};
// Vue's v-hold-name is the directive below.
const HOLD = 500;
const STAYS = 1500;
const SLOP = 10;

export function holdName(el, name) {
  let tip = null;
  let timer = 0;
  let gone = 0;
  let start = null;

  const hide = () => {
    clearTimeout(gone);
    tip?.remove();
    tip = null;
  };

  const show = () => {
    hide();
    tip = document.createElement('div');
    tip.className = 'hold-name';
    tip.setAttribute('role', 'tooltip');
    tip.textContent = name;
    document.body.append(tip);
    // Over the button, centred on it, kept 8px inside the window's edges.
    const b = el.getBoundingClientRect();
    const w = tip.offsetWidth;
    const x = Math.max(8, Math.min(window.innerWidth - 8 - w, b.left + b.width / 2 - w / 2));
    tip.style.left = `${x}px`;
    tip.style.top = `${Math.max(8, b.top - tip.offsetHeight - 8)}px`;
  };

  // The lift after a name is shown isn't a press, nor is the click it brings.
  const swallow = (e) => {
    e.preventDefault();
    e.stopPropagation();
  };

  const stop = () => {
    clearTimeout(timer);
    window.removeEventListener('pointermove', move, true);
    window.removeEventListener('pointerup', up, true);
    window.removeEventListener('pointercancel', up, true);
  };

  // A captured pointer's moves go to whatever captured it, the download bar for one, so they
  // are heard on the window.
  function move(e) {
    if (start && e.pointerId === start.id && Math.hypot(e.clientX - start.x, e.clientY - start.y) > SLOP) {
      start = null;
      stop();
      hide();
    }
  }

  function up(e) {
    if (!start || e.pointerId !== start.id) return;
    start = null;
    stop();
    if (!tip) return;
    window.addEventListener('click', swallow, { capture: true, once: true });
    setTimeout(() => window.removeEventListener('click', swallow, true), 300);
    gone = setTimeout(hide, STAYS);
  }

  function down(e) {
    if (!e.isPrimary || e.button > 0) return;
    hide();
    start = { id: e.pointerId, x: e.clientX, y: e.clientY };
    timer = setTimeout(show, HOLD);
    window.addEventListener('pointermove', move, true);
    window.addEventListener('pointerup', up, true);
    window.addEventListener('pointercancel', up, true);
  }

  // A WebView's own long press brings a context menu, and a vibration: not on a button.
  const menu = (e) => e.preventDefault();

  el.addEventListener('pointerdown', down);
  el.addEventListener('contextmenu', menu);
  return {
    update(n) {
      name = n;
      if (tip) tip.textContent = n;
    },
    destroy() {
      stop();
      hide();
      el.removeEventListener('pointerdown', down);
      el.removeEventListener('contextmenu', menu);
    },
  };
}

// v-hold-name="name", for the Vue pages.
export const vHoldName = {
  mounted: (el, b) => (el._holdName = holdName(el, b.value)),
  updated: (el, b) => el._holdName?.update(b.value),
  unmounted: (el) => el._holdName?.destroy(),
};
