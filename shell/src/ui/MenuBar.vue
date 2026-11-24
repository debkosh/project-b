<script setup>
import { computed, nextTick, onUnmounted, ref, watch, watchEffect } from 'vue';
import ContextMenu from './ContextMenu.vue';
import { phone } from '@/phone.js';
import { keyLabel, requestBarMenu, runMenuItem, send, state } from '@/sdk/vue.js';

// The menu bar off the Mac (docs/design-system.md, "The menu bar"; docs/research/linux.md):
// a strip of Go's top-level menus across the window, above the shell's own top, Firefox's
// model on Linux. With Show menu bar on it takes its 30px over everything else, inside the
// frame's band at the window's top, which the shell's next strip then leaves out
// (--top-band); off, Alt alone shows it for a moment with its first title focused, and F10
// with its first menu open, as a layer over the shell's first row, whole, which doesn't
// move. Esc, a press elsewhere or a choice hides it again. Its menus are Go's bar
// (requestBarMenu): every menu, the Mac's app menu's items where these desktops keep them.
// Each opens as the shell's floating menu, with Linux's keys. hidden is the shell hiding
// its bars, for a page in full screen or the welcome: Alt and F10 still show it, over it.
const props = defineProps({ hidden: Boolean });

const ours = computed(() => state.platform !== 'mac' && !phone);
const kept = computed(() => ours.value && !!state.prefs.showMenuBar && !props.hidden);
const moment = ref(false);
const shown = computed(() => kept.value || (ours.value && moment.value));
const menus = computed(() => state.barMenu.filter((it) => it.items));

// Kept, the bar is the window's top: the shell's next strip draws no band of its own.
watchEffect(() => {
  if (kept.value) document.documentElement.style.setProperty('--top-band', '0px');
  else document.documentElement.style.removeProperty('--top-band');
});
onUnmounted(() => document.documentElement.style.removeProperty('--top-band'));

// For a moment, the bar covers the shell's first row whole, so nothing of it half-shows.
const cover = ref(0);
function measure() {
  let el = bar.value?.nextElementSibling;
  while (el && (!el.offsetHeight || el.classList.contains('ui-context-cover'))) el = el.nextElementSibling;
  const top = bar.value?.parentElement?.getBoundingClientRect().top || 0;
  cover.value = el ? Math.round(el.getBoundingClientRect().bottom - top) : 0;
}

const bar = ref(null);
const open = ref(-1);
const at = ref({ x: 0, y: 0 });
const keyboard = ref(false);

const check = '<path d="M3.5 8.5 6.5 11.5 12.5 4.5"/>';
function row(it) {
  return { label: it.title, key: it.action ? keyLabel(it.action) : '', disabled: !it.enabled, icon: it.checked ? check : '', run: () => runMenuItem(it) };
}
// A submenu's own submenu, such as View › Shell's, shows as its items between lines.
function flat(items) {
  const out = [];
  for (const it of items) {
    if (it.separator) out.push({ separator: true });
    else if (it.items) out.push({ separator: true }, ...flat(it.items), { separator: true });
    else out.push(row(it));
  }
  return out.filter((it, i, all) => !(it.separator && (i === 0 || i === all.length - 1 || all[i - 1].separator)));
}
const items = computed(() =>
  (menus.value[open.value]?.items || []).map((it) => (it.separator ? { separator: true } : it.items ? { label: it.title, items: flat(it.items) } : row(it))),
);

const titles = () => [...(bar.value?.querySelectorAll('.menu-bar-title') || [])];

async function openMenu(i, byKeys) {
  if (i < 0 || i >= menus.value.length) return;
  keyboard.value = byKeys;
  open.value = -1;
  await nextTick();
  const r = titles()[i]?.getBoundingClientRect();
  if (!r) return;
  at.value = { x: r.left, y: r.bottom + 2 };
  open.value = i;
}

function hide() {
  open.value = -1;
  moment.value = false;
  if (document.activeElement && bar.value?.contains(document.activeElement)) document.activeElement.blur();
}

// A menu closed: by a choice, Esc or a press elsewhere, which hides a bar shown for a
// moment. A press on another title opens that one instead.
function closed(e) {
  const i = e ? titles().findIndex((el) => { const r = el.getBoundingClientRect(); return e.clientX >= r.left && e.clientX < r.right && e.clientY >= r.top && e.clientY < r.bottom; }) : -1;
  if (i >= 0 && i !== open.value) return openMenu(i, false);
  hide();
}

function move(by) {
  const n = menus.value.length;
  if (n) openMenu((open.value + by + n) % n, true);
}

function click(i) {
  if (open.value === i) hide();
  else openMenu(i, false);
}

// The keys on a focused title: ← and → move along the bar, Enter, Space or ↓ open its menu,
// Esc hides a bar shown for a moment.
function onKey(e, i) {
  const n = menus.value.length;
  if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') titles()[(i + (e.key === 'ArrowRight' ? 1 : -1) + n) % n]?.focus();
  else if (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown') openMenu(i, true);
  else if (e.key === 'Escape') hide();
  else return;
  e.preventDefault();
}

function onFocusOut(e) {
  if (open.value < 0 && !bar.value?.contains(e.relatedTarget)) moment.value = false;
}

// Go's ask, Alt alone or F10: the bar comes, the shell takes the keys, and its first title
// is focused, or its first menu open, once Go's menu is in. Alt again hides a bar it showed.
let pending = '';
async function apply() {
  if (!pending || !menus.value.length) return;
  const what = pending;
  pending = '';
  await nextTick();
  if (what === 'open') openMenu(0, true);
  else titles()[0]?.focus();
}
watch(() => menus.value.length, apply);
watch(
  () => state.menuBar.n,
  () => {
    if (!ours.value) return;
    if (moment.value && !state.menuBar.open && open.value < 0) return hide();
    requestBarMenu();
    send({ t: 'focusShell' });
    moment.value = true;
    pending = state.menuBar.open ? 'open' : 'focus';
    nextTick(measure);
    apply();
  },
);

// The menu as Go has it now, each time the bar shows.
watch(shown, (on) => on && requestBarMenu(), { immediate: true });
</script>

<template>
  <div
    v-if="shown"
    v-shell
    ref="bar"
    class="menu-bar"
    :class="{ moment: !kept }"
    :style="!kept && cover ? { height: cover + 'px' } : null"
    role="menubar"
    aria-label="Menu bar"
    @focusout="onFocusOut"
  >
    <button
      v-for="(m, i) in menus"
      :key="m.title"
      class="menu-bar-title"
      :class="{ open: open === i }"
      type="button"
      role="menuitem"
      aria-haspopup="menu"
      :aria-expanded="open === i"
      @click="click(i)"
      @keydown="onKey($event, i)"
      @pointerenter="open >= 0 && open !== i && openMenu(i, false)"
    >{{ m.title }}</button>
    <div v-drag class="menu-bar-rest"></div>
  </div>
  <ContextMenu v-if="open >= 0 && items.length" v-shell :items="items" :x="at.x" :y="at.y" :keyboard="keyboard" :label="menus[open]?.title" @close="closed" @move="move" />
</template>

<style scoped>
/* ux's spec: 30px of the bar's colour inside the frame's band, a line under it; titles in
   the text's colour, tinted on hover and softer while open. Kept, it is in the layout; for
   a moment, it lies over the shell's first row as a layer, with a soft shadow. */
.menu-bar {
  display: flex;
  align-items: center;
  flex: none;
  gap: 2px;
  height: calc(30px + var(--frame-width));
  padding: var(--frame-width) var(--frame-width) 0;
  /* The bar's colour in the middle, the frame's in the band, and the bar's again under the
     line, which is translucent: over the frame's colour it read as a hard teal line. */
  background: linear-gradient(var(--bar), var(--bar)) content-box, linear-gradient(var(--frame), var(--frame)) padding-box, var(--bar);
  border-bottom: 1px solid var(--line);
  box-sizing: border-box;
  /* Above what a shell lays over its page, as Curve's command bar's dimming. */
  position: relative;
  z-index: 2;
}

.menu-bar.moment {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 50;
  box-shadow: 0 2px 8px rgb(0 0 0 / 14%);
}

/* The bar's own colour, not the band's, between the band and the first title. */
.menu-bar-title:first-child {
  margin-left: 6px;
}

.menu-bar-title {
  height: 24px;
  padding: 0 10px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  white-space: nowrap;
}

.menu-bar-title:hover {
  background: var(--accent-tint);
}

.menu-bar-title.open {
  background: var(--accent-soft);
}

.menu-bar-title:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.menu-bar-rest {
  flex: 1;
  align-self: stretch;
}
</style>
