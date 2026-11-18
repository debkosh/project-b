<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { send, state } from '@/sdk/vue.js';
import { bookmarksUnder, inFolder, path, topFolder } from '@/ui/bookmarks.js';
import { host, openGroup, openHere } from './bookmarks.js';
import SiteMark from './SiteMark.vue';
import { folderTile } from '@/ui/tint.js';
import { useTints } from '@/ui/useTints.js';

// The new tab's bookmarks, under the command box (docs/bookmark-folders.md, "Where bookmarks
// show"): the top folder as tiles, its folders first, each folder showing up to four of its
// sites. A folder opens in place, in a panel under its row, with its own folders and
// bookmarks and Open all as a group; a bookmark loads where the box's rows do.
defineProps({ overPage: Boolean });

const top = computed(() => inFolder(state.bookmarks, ''));
// The folder whose tile is open, and the one its panel shows: it or a folder in it.
const open = ref('');
const at = ref('');
const shown = computed(() => inFolder(state.bookmarks, at.value));
const trail = computed(() => path(state.bookmarks, at.value));
const marks = computed(() => shown.value.filter((e) => e.kind === 'bookmark'));

// A folder that goes closes its panel; one in it that goes shows the panel's own again.
watch(
  () => state.bookmarks,
  (list) => {
    const has = (id) => list.some((e) => e.id === id);
    if (open.value && !has(open.value)) open.value = at.value = '';
    else if (!has(at.value)) at.value = open.value;
  },
);

function toggle(f) {
  open.value = at.value = open.value === f.id ? '' : f.id;
}

// sitesIn is the addresses a folder's tile shows: its own bookmarks first, then its folders'.
function sitesIn(id, n = 4) {
  const out = [];
  const walk = (f) => {
    const here = inFolder(state.bookmarks, f);
    for (const e of here) if (e.kind === 'bookmark' && out.length < n) out.push(e.url);
    for (const e of here) if (e.kind === 'folder' && out.length < n) walk(e.id);
  };
  walk(id);
  return out;
}

// The panel goes after the row its folder's tile is in, across the grid: how many tiles a
// row holds comes from the grid as it is laid out.
const grid = ref(null);
const columns = ref(1);
const measure = () => {
  if (grid.value) columns.value = Math.max(1, getComputedStyle(grid.value).gridTemplateColumns.split(' ').length);
};
const resize = new ResizeObserver(measure);
onMounted(() => grid.value && resize.observe(grid.value));
onBeforeUnmount(() => resize.disconnect());
const panelAfter = computed(() => {
  const i = top.value.findIndex((e) => e.id === open.value);
  return i < 0 ? -1 : Math.min(top.value.length - 1, Math.ceil((i + 1) / columns.value) * columns.value - 1);
});
watch(top, () => nextTick(measure));

// A folder's tile in its own colour (tint.js), made again on a new palette. The top
// folder's tiles sit on the command box, much the bar; an open folder's on its surface.
const tints = useTints();
const tileOf = (id, on) => {
  const t = (tints.value, folderTile(id, on));
  return { background: t.background, borderColor: t.border };
};
const label = (e) => e.title || (e.kind === 'folder' ? 'Folder' : host(e.url));
</script>

<template>
  <section v-if="top.length" class="new-tab-bookmarks" aria-label="Bookmarks" @mousedown.prevent>
    <div class="heading">
      <h2>{{ topFolder }}</h2>
      <button type="button" class="link" @click="send({ t: 'run', text: 'bookmarks.show' })">All bookmarks ›</button>
    </div>
    <div ref="grid" class="tiles">
      <template v-for="(e, i) in top" :key="e.id">
        <button
          type="button"
          class="tile"
          :class="{ folder: e.kind === 'folder', open: open === e.id }"
          :title="e.kind === 'folder' ? e.title : `${label(e)}\n${e.url}`"
          :aria-expanded="e.kind === 'folder' ? open === e.id : undefined"
          @click="e.kind === 'folder' ? toggle(e) : openHere(e.url, overPage)"
        >
          <span v-if="e.kind === 'folder'" class="mark sites" :style="tileOf(e.id, 'bar')" aria-hidden="true">
            <SiteMark v-for="url in sitesIn(e.id)" :key="url" :url="url" :size="16" on="bar" />
          </span>
          <SiteMark v-else class="mark" :url="e.url" :size="56" on="bar" />
          <span class="name">{{ label(e) }}</span>
        </button>
        <div v-if="i === panelAfter" class="panel" role="group" :aria-label="`Folder ${trail[trail.length - 1].title}`" @keydown.esc="open = at = ''">
          <div class="panel-head">
            <nav class="trail" aria-label="Path">
              <template v-for="(f, j) in trail" :key="f.id">
                <span v-if="j === trail.length - 1" class="here">{{ f.title }}</span>
                <button v-else type="button" class="crumb" @click="f.id ? (at = f.id) : (open = at = '')">{{ f.title }}</button>
                <span v-if="j < trail.length - 1" class="sep" aria-hidden="true">›</span>
              </template>
              <span class="count">{{ bookmarksUnder(state.bookmarks, at) }}</span>
            </nav>
            <button type="button" class="text-button" :disabled="!marks.length" @click="openGroup(marks.map((m) => m.url), trail[trail.length - 1].title)">Open all as a group</button>
          </div>
          <div v-if="shown.length" class="tiles inner">
            <button
              v-for="f in shown"
              :key="f.id"
              type="button"
              class="tile"
              :class="{ folder: f.kind === 'folder' }"
              :title="f.kind === 'folder' ? f.title : `${label(f)}\n${f.url}`"
              @click="f.kind === 'folder' ? (at = f.id) : openHere(f.url, overPage)"
            >
              <span v-if="f.kind === 'folder'" class="mark sites" :style="tileOf(f.id, 'surface')" aria-hidden="true">
                <SiteMark v-for="url in sitesIn(f.id)" :key="url" :url="url" :size="16" />
              </span>
              <SiteMark v-else class="mark" :url="f.url" :size="56" />
              <span class="name">{{ label(f) }}</span>
            </button>
          </div>
          <p v-else class="empty">This folder is empty.</p>
        </div>
      </template>
    </div>
  </section>
</template>

<style scoped>
.new-tab-bookmarks {
  width: min(704px, 92%);
  margin-top: 32px;
  padding-bottom: 32px;
}

.heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 30px;
  margin-bottom: 8px;
}

h2 {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  font-weight: 500;
}

.link {
  padding: 0;
  border: 0;
  background: none;
  color: var(--muted);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}

.link:hover {
  color: var(--text);
}

.tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, 88px);
  justify-content: space-between;
  gap: 16px 0;
}

.tiles.inner {
  justify-content: start;
}

.tile {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  width: 88px;
  padding: 4px 0;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}

.tile:hover .mark {
  box-shadow: 0 0 0 4px var(--accent-tint);
}

.tile:focus-visible {
  outline: none;
}

.tile:focus-visible .mark,
.tile.open .mark {
  box-shadow: var(--focus-ring);
}

.mark {
  border-radius: 8px;
}

/* A folder: a card of up to four of its sites' marks. */
.sites {
  display: grid;
  grid-template-columns: 16px 16px;
  grid-auto-rows: 16px;
  align-content: center;
  justify-content: center;
  gap: 6px;
  width: 56px;
  height: 56px;
  border: 1px solid var(--line);
  background: var(--surface);
  box-sizing: border-box;
}

.name {
  max-width: 84px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

/* The open folder, across the grid under its row: a card. */
.panel {
  grid-column: 1 / -1;
  padding: 12px 16px 16px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.trail {
  display: flex;
  align-items: baseline;
  gap: 4px;
  min-width: 0;
  font-size: 13px;
}

.crumb {
  padding: 0;
  border: 0;
  background: none;
  color: var(--muted);
  font: inherit;
  cursor: pointer;
}

.crumb:hover {
  color: var(--text);
}

.sep {
  color: var(--muted);
}

.here {
  font-size: 15px;
  font-weight: 600;
}

.count {
  margin-left: 4px;
  color: var(--muted);
}

.empty {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}
</style>
