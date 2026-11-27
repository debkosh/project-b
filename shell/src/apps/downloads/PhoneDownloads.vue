<script setup>
import { computed, nextTick, ref, watch } from 'vue';
import ActionSheet from '@/ui/ActionSheet.vue';
import Dialog from '@/ui/Dialog.vue';
import LineIcon from '@/ui/LineIcon.vue';
import { find } from '@/ui/filter.js';
import { pageHistory, pushEscape } from '@/ui/escape.js';
import { act, clear, send, state } from './state.js';
import { hostOf, status } from './words.js';

// The Downloads page on a phone (docs/design-system.md, "On a phone"; docs/daily-driver.md):
// a 52px bar with ←, the title, Search and ⋮; the downloads newest first, in groups by the
// day each started; one 56px row each, its kind's icon, the name and one --muted line. A tap
// opens a finished one; a failed one carries Retry in its row; everything else is in its ⋮,
// a sheet of actions. ← leaves the page, as the back key does; the back key closes the search
// and a sheet first (ui/escape.js).
pageHistory(() => '', () => {});

// What kind of file a name is, for its icon.
const kinds = {
  image: /\.(png|jpe?g|gif|webp|svg|heic|avif|bmp)$/i,
  video: /\.(mp4|mov|webm|mkv|avi|m4v)$/i,
  audio: /\.(mp3|m4a|wav|ogg|flac|aac|opus)$/i,
  archive: /\.(zip|rar|7z|tar|gz|tgz|bz2|xz)$/i,
};
const kindOf = (name) => Object.keys(kinds).find((k) => kinds[k].test(name)) || 'file';

// The search, while open, and what it looks for: a name or a host.
const searching = ref(false);
const text = ref('');
const field = ref(null);
let popSearch = null;
async function openSearch() {
  searching.value = true;
  popSearch = pushEscape(closeSearch);
  await nextTick();
  field.value?.focus();
}
function closeSearch() {
  if (!searching.value) return;
  searching.value = false;
  text.value = '';
  popSearch?.();
  popSearch = null;
}

const shown = computed(() => {
  const q = text.value.trim().toLowerCase();
  if (!q) return state.downloads;
  return find(state.downloads, (d) => [d.name, hostOf(d.url || '')], q);
});

// The day each started, as a group's heading: Today, Yesterday, then the date; one listed
// before the list kept the time is Earlier.
const day = (d) => {
  if (!d.started) return 'Earlier';
  const at = new Date(d.started * 1000);
  const today = new Date();
  const days = Math.round((new Date(today.toDateString()) - new Date(at.toDateString())) / 864e5);
  if (days === 0) return 'Today';
  if (days === 1) return 'Yesterday';
  return at.toLocaleDateString(undefined, { month: 'short', day: 'numeric', ...(at.getFullYear() === today.getFullYear() ? {} : { year: 'numeric' }) });
};
const groups = computed(() => {
  const out = [];
  for (const d of shown.value) {
    const name = day(d);
    if (out.at(-1)?.name !== name) out.push({ name, items: [] });
    out.at(-1).items.push(d);
  }
  return out;
});

const share = (d) => (d.total > 0 ? Math.min(1, d.received / d.total) : 0);

// An address in a sheet's head, as the long-press sheet shows one: without its scheme.
const withoutScheme = (u) => u.replace(/^[a-z][a-z0-9+.-]*:\/\//i, '');

// The sheet open: a download's actions, or the page's own.
const sheet = ref(null);
function actions(d) {
  const link = d.url ? [{ icon: 'link', label: 'Copy link', run: () => act(d.id, 'copyLink') }] : [];
  const remove = d.state !== 'progress' ? [{ icon: 'remove', label: 'Remove from list', run: () => act(d.id, 'remove') }] : [];
  // A finished one can be deleted, last, in --danger.
  const rows = d.state === 'done' ? [...remove, { icon: 'trash', label: 'Delete', danger: true, run: () => (deleting.value = d) }] : remove;
  let tiles;
  if (d.state === 'done') {
    tiles = [
      { icon: 'share', label: 'Share', run: () => act(d.id, 'share') },
      { icon: 'folder', label: 'Show in Downloads', run: () => act(d.id, 'reveal') },
      ...link,
    ];
  } else if (d.state === 'progress') {
    tiles = [
      d.paused ? { icon: 'play', label: 'Resume', run: () => act(d.id, 'resume') } : { icon: 'pause', label: 'Pause', run: () => act(d.id, 'pause') },
      { icon: 'stop', label: 'Cancel', run: () => act(d.id, 'cancel') },
      ...(d.url ? [{ icon: 'retry', label: 'Retry', run: () => act(d.id, 'retry') }] : []),
      ...link,
    ];
  } else {
    // No Retry where trying again can't help: a protected video, a format not read.
    tiles = [...(d.url && !d.noRetry ? [{ icon: 'retry', label: 'Retry', run: () => act(d.id, 'retry') }] : []), ...link];
  }
  sheet.value = { id: d.id, name: d.name, detail: d.url ? withoutScheme(d.url) : d.from || '', tiles, rows };
}
function pageActions() {
  sheet.value = {
    name: 'Downloads',
    detail: '',
    tiles: [],
    rows: state.downloads.some((d) => d.state !== 'progress') ? [{ icon: 'remove', label: 'Clear all', run: clear }] : [],
  };
}
// Delete: asked first, as it can't be undone.
const deleting = ref(null);
function deleteFile() {
  send({ t: 'deleteDownload', id: deleting.value.id });
  deleting.value = null;
}

// A download gone from the list takes its sheet with it.
watch(
  () => state.downloads,
  () => {
    if (sheet.value?.id && !state.downloads.some((d) => d.id === sheet.value.id)) sheet.value = null;
  },
);
</script>

<template>
  <div class="phone-page">
    <h1 v-if="searching" class="hidden-heading">Downloads</h1>
    <header class="top">
      <button type="button" class="icon" aria-label="Back" @click="searching ? closeSearch() : send({ t: 'close' })"><LineIcon name="back" /></button>
      <input v-if="searching" ref="field" v-model="text" class="search" type="search" aria-label="Search downloads" placeholder="Search downloads" enterkeyhint="search" />
      <button v-if="searching && text" type="button" class="icon" aria-label="Clear search" @click="(text = ''), field?.focus()"><LineIcon name="close" /></button>
      <h1 v-else-if="!searching" class="title">Downloads</h1>
      <button v-if="!searching" type="button" class="icon" aria-label="Search" @click="openSearch"><LineIcon name="search" /></button>
      <button v-if="!searching" type="button" class="icon" aria-label="More" @click="pageActions"><LineIcon name="more" /></button>
    </header>
    <main class="list">
      <p v-if="!state.downloads.length" class="empty">Files you download show here.</p>
      <p v-else-if="!shown.length" class="empty">Nothing found</p>
      <section v-for="g in groups" :key="g.name" :aria-label="g.name">
        <h2 class="day">{{ g.name }}</h2>
        <ul role="list">
          <li v-for="d in g.items" :key="d.id" class="item" role="listitem" :aria-label="d.name" :class="d.state">
            <button type="button" class="main" :disabled="d.state !== 'done'" @click="act(d.id, 'open')">
              <span class="kind"><LineIcon :name="kindOf(d.name)" /></span>
              <span class="lines">
                <span class="name">{{ d.name }}</span>
                <span class="line">{{ status(d) }}</span>
                <span v-if="d.state === 'progress'" class="bar" :class="{ unknown: d.total <= 0 }"><i :style="{ width: d.total > 0 ? `${share(d) * 100}%` : '' }"></i></span>
              </span>
            </button>
            <button v-if="d.state === 'failed' && d.url && !d.noRetry" type="button" class="retry" @click="act(d.id, 'retry')">Retry</button>
            <button type="button" class="icon" :aria-label="`Actions for ${d.name}`" @click="actions(d)"><LineIcon name="more" /></button>
          </li>
        </ul>
      </section>
    </main>
    <ActionSheet v-if="sheet" :name="sheet.name" :detail="sheet.detail" :tiles="sheet.tiles" :rows="sheet.rows" @close="sheet = null" />
    <Dialog v-if="deleting" :title="`Delete ${deleting.name}?`" @close="deleting = null">
      <p class="words">It goes from your phone's Downloads, and from this list.</p>
      <template #buttons>
        <button type="button" class="text-button" @click="deleting = null">Cancel</button>
        <button type="button" class="text-button danger" @click="deleteFile">Delete</button>
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.phone-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--surface);
  color: var(--text);
}

.top {
  flex: none;
  display: flex;
  align-items: center;
  gap: 4px;
  height: 52px;
  padding: 0 4px;
  border-bottom: 1px solid var(--line);
  box-sizing: border-box;
}

.title {
  flex: 1;
  margin: 0 0 0 4px;
  font-size: calc(18rem / 16);
  font-weight: 600;
}

/* Our own ×, not the browser's, which drew in the accent. */
.search::-webkit-search-cancel-button {
  display: none;
}

.search {
  flex: 1;
  min-width: 0;
  height: 40px;
  padding: 0 8px;
  border: 0;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: calc(16rem / 16);
  outline: none;
}

.icon {
  flex: none;
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  border: 0;
  border-radius: 24px;
  background: none;
  color: var(--icon);
}

.list {
  flex: 1;
  overflow: auto;
  padding-bottom: 16px;
}

.empty {
  margin: 32px 16px;
  color: var(--muted);
  font-size: calc(14rem / 16);
  text-align: center;
}

.day {
  margin: 0;
  padding: 16px 16px 4px;
  color: var(--muted);
  font-size: calc(12rem / 16);
  font-weight: 600;
}

ul {
  margin: 0;
  padding: 0;
  list-style: none;
}

.item {
  display: flex;
  align-items: center;
  min-height: 56px;
  padding-right: 4px;
}

.main {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 16px;
  min-height: 56px;
  padding: 8px 0 8px 16px;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
}

.main:disabled {
  opacity: 1;
}

.kind {
  flex: none;
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 8px;
  background: var(--bar);
  color: var(--icon);
}

.lines {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.name,
.line {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.name {
  font-size: calc(15rem / 16);
}

.line {
  color: var(--muted);
  font-size: calc(13rem / 16);
}

.failed .line {
  color: var(--danger);
}

.bar {
  height: 2px;
  margin-top: 4px;
  border-radius: 1px;
  background: var(--line);
  overflow: hidden;
}

.bar i {
  display: block;
  height: 100%;
  background: var(--accent);
}

.bar.unknown i {
  width: 30%;
}

.retry {
  flex: none;
  height: 48px;
  padding: 0 12px;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--icon);
  font: inherit;
  font-weight: 600;
}

.main:active,
.icon:active,
.retry:active {
  background: var(--accent-tint);
}

.main:focus-visible,
.icon:focus-visible,
.retry:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

.words {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: calc(14rem / 16);
}

.hidden-heading {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
</style>
