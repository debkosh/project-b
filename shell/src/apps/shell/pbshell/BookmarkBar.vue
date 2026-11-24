<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import ContextMenu from '@/ui/ContextMenu.vue';
import { send, state } from '@/sdk/vue.js';
import { host, openGroup, under } from './bookmarks.js';
import SiteMark from './SiteMark.vue';
import { folderMark } from '@/ui/tint.js';
import { useTints } from '@/ui/useTints.js';

// The bookmarks bar, under the top bar in either layout, while bookmarksBar is on
// (docs/bookmark-folders.md, "Where bookmarks show"): bookmarksBarFolder's entries in their
// places, the top's when that folder is gone. A bookmark loads in the active tab, or in a
// new one behind it on a middle press; a folder drops down its bookmarks, its folders as
// submenus, and Open all as a group. What doesn't fit goes in a » menu at the end. It sits
// above the page, which is laid out under it, so the page's region starts below it.

const folder = computed(() => {
  const id = state.prefs.bookmarksBarFolder;
  return id && state.bookmarks.some((e) => e.id === id && e.kind === 'folder') ? id : '';
});
const entries = computed(() => state.bookmarks.filter((e) => e.parent === folder.value));

const label = (e) => e.title || (e.kind === 'folder' ? 'Folder' : host(e.url));

function load(url) {
  send({ t: 'navigate', text: url });
}

// A folder's icon in its own colour (tint.js), made again on a new palette.
const tints = useTints();
const folderColour = (id) => (tints.value, folderMark(id, 'bar'));
const folderIcon = '<path d="M2.5 4.5a1 1 0 0 1 1-1h3l1.5 1.5h4.5a1 1 0 0 1 1 1v5.5a1 1 0 0 1-1 1h-9a1 1 0 0 1-1-1z"/>';

// A bookmark as a menu item, and a folder as a submenu of every bookmark it holds: a menu
// goes one submenu deep.
const item = (e) =>
  e.kind === 'folder'
    ? { label: label(e), icon: folderIcon, items: under(state.bookmarks, e.id).map((b) => ({ label: label(b), run: () => load(b.url) })) }
    : { label: label(e), run: () => load(e.url) };

// A folder's menu: what it holds, then Open all as a group for its own bookmarks.
function folderItems(f) {
  const inside = state.bookmarks.filter((e) => e.parent === f.id);
  const marks = inside.filter((e) => e.kind === 'bookmark');
  const out = inside.map(item);
  if (marks.length) out.push({ separator: true }, { label: 'Open all as a group', run: () => openGroup(marks.map((e) => e.url), f.title) });
  if (!out.length) out.push({ label: 'This folder is empty', disabled: true });
  return out;
}

const menu = ref(null);
function openMenu(items, label, e) {
  const r = e.currentTarget.getBoundingClientRect();
  menu.value = { items, label, x: r.left, y: r.bottom + 4, keyboard: !e.clientX && !e.clientY };
}

// How many chips fit, the rest going in the » menu, which then takes room of its own.
const row = ref(null);
const fits = ref(Infinity);
const more = 32;
function measure() {
  const el = row.value;
  if (!el) return;
  const chips = [...el.querySelectorAll('.chip')];
  const right = el.getBoundingClientRect().right;
  const within = (room) => chips.findIndex((c) => c.getBoundingClientRect().right > right - room);
  const all = within(0);
  fits.value = all < 0 ? Infinity : Math.max(0, within(more));
}
const resize = new ResizeObserver(measure);
onMounted(() => {
  resize.observe(row.value);
  measure();
});
onBeforeUnmount(() => resize.disconnect());
watch(entries, () => nextTick(measure));
const hidden = computed(() => entries.value.slice(fits.value));
</script>

<template>
  <div v-shell class="bookmark-bar" role="toolbar" aria-label="Bookmarks bar">
    <div ref="row" class="chips">
      <button
        v-for="(e, i) in entries"
        :key="e.id"
        type="button"
        class="chip"
        :class="{ over: i >= fits }"
        :tabindex="i >= fits ? -1 : undefined"
        :aria-hidden="i >= fits ? 'true' : undefined"
        :aria-haspopup="e.kind === 'folder' ? 'menu' : undefined"
        :title="e.kind === 'folder' ? e.title : `${label(e)}\n${e.url}`"
        @click="e.kind === 'folder' ? openMenu(folderItems(e), `Folder ${e.title}`, $event) : load(e.url)"
        @auxclick.middle="e.kind === 'bookmark' && send({ t: 'newTab', text: e.url, background: true })"
      >
        <svg v-if="e.kind === 'folder'" class="icon" width="16" height="16" viewBox="0 0 16 16" aria-hidden="true" :style="{ stroke: folderColour(e.id) }" v-html="folderIcon"></svg>
        <SiteMark v-else :url="e.url" on="bar" />
        <span class="title">{{ label(e) }}</span>
        <svg v-if="e.kind === 'folder'" class="drop" width="8" height="8" viewBox="0 0 8 8" aria-hidden="true"><path d="m1.5 3 2.5 2.5L6.5 3" /></svg>
      </button>
    </div>
    <button v-if="hidden.length" type="button" class="icon-button more" aria-label="More bookmarks" aria-haspopup="menu" @click="openMenu(hidden.map(item), 'More bookmarks', $event)">»</button>
    <ContextMenu v-if="menu" v-shell :items="menu.items" :x="menu.x" :y="menu.y" :label="menu.label" :keyboard="menu.keyboard" @close="menu = null" />
  </div>
</template>

<style scoped>
/* A slim strip in the bar's colour, the page under it, parted by a hairline. */
.bookmark-bar {
  display: flex;
  align-items: center;
  flex: none;
  height: 32px;
  padding: 0 4px;
  background: var(--bar);
  box-shadow: inset 0 -1px 0 var(--line);
}

.chips {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
  gap: 2px;
  overflow: hidden;
}

.chip {
  display: flex;
  align-items: center;
  flex: none;
  gap: 6px;
  max-width: 180px;
  height: 26px;
  padding: 0 8px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}

.chip:hover {
  background: var(--accent-tint);
}

.chip:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

/* Past the room there is: in the » menu instead. */
.chip.over {
  visibility: hidden;
}

.title {
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.icon,
.drop {
  flex: none;
  fill: none;
  stroke: var(--icon);
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.more {
  flex: none;
  width: 28px;
  height: 26px;
  font-size: 14px;
}
</style>
