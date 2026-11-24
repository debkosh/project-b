<script setup>
import { computed, nextTick, ref, watch, watchEffect } from 'vue';
import { paintTheme } from '@/theme.js';
import Dialog from '@/ui/Dialog.vue';
import { pageHistory, upTo } from '@/ui/escape.js';
import RowMenu from '@/ui/RowMenu.vue';
import { find } from '@/ui/filter.js';
import { bookmarksUnder, folders, inFolder, path, topFolder } from '@/ui/bookmarks.js';
import { send, state } from './state.js';

// The Bookmarks page (docs/bookmark-folders.md): the folders on the left, Bookmarks at the
// top with the folders under it, each with how many bookmarks it holds, and New folder at
// the foot; the folder chosen on the right, its folders first, then its bookmarks, a path
// above it back to the top. Search looks in every folder. An entry moves by a drag onto a
// folder, or with its menu's Move to…; its menu also opens, edits and deletes it. A
// phone has no room for the left: the path and New folder are above the list.
watchEffect(() => paintTheme(state.tones, state.theme));

// The folder shown: '' is the top. It goes back to the top when the folder is deleted.
const at = ref('');
watch(
  () => state.bookmarks,
  (list) => {
    if (at.value && !list.some((e) => e.id === at.value)) at.value = '';
  },
);

const tree = computed(() => folders(state.bookmarks));
const trail = computed(() => path(state.bookmarks, at.value));
const here = computed(() => trail.value[trail.value.length - 1]);
const count = (id) => bookmarksUnder(state.bookmarks, id);

const text = ref('');
// Typed, the bookmarks in every folder whose title and address have every word typed, in any
// order (filter.js); else the
// folder's entries.
const shown = computed(() => {
  const q = text.value.trim().toLowerCase();
  if (!q) return inFolder(state.bookmarks, at.value);
  return find(state.bookmarks.filter((e) => e.kind === 'bookmark'), (e) => [e.title, e.url], q);
});
const folderName = (id) => (id ? state.bookmarks.find((e) => e.id === id)?.title || '' : topFolder);

function openFolder(id) {
  at.value = id;
  text.value = '';
}

// Each folder opened is an entry in the tab's history, so the phone's back key and Back go
// to the folder before it, not out of the page (docs/shells.md, "Back on our pages").
const went = pageHistory(() => at.value, openFolder);
watch(at, went);

function open(e, ev) {
  ev?.preventDefault();
  send({ t: 'open', url: e.url });
}

// Editing in place: a bookmark's name and address, a folder's name.
const editing = ref(null);
const title = ref('');
const url = ref('');
const titleField = ref(null);

async function edit(e) {
  editing.value = e.id;
  title.value = e.title;
  url.value = e.url || '';
  state.refused = null;
  await nextTick();
  titleField.value?.[0]?.focus();
}

function save(e) {
  send({ t: 'editBookmark', id: e.id, title: title.value.trim(), url: url.value.trim() });
  editing.value = null;
}

// New folder: a name, in the folder shown.
const naming = ref(false);
const newName = ref('');
const newField = ref(null);

async function newFolder() {
  naming.value = true;
  newName.value = '';
  await nextTick();
  newField.value?.focus();
}

function makeFolder() {
  const name = newName.value.trim();
  if (name) send({ t: 'newFolder', parent: at.value, title: name });
  naming.value = false;
}

// Delete: a folder that holds anything asks first, saying how much goes with it.
const deleting = ref(null);
function remove(e) {
  if (e.kind === 'folder' && inFolder(state.bookmarks, e.id).length) deleting.value = e;
  else send({ t: 'removeBookmark', id: e.id });
}
function holds(e) {
  const n = count(e.id);
  const f = inFolder(state.bookmarks, e.id).filter((x) => x.kind === 'folder').length;
  const parts = [];
  if (n) parts.push(n === 1 ? '1 bookmark' : `${n} bookmarks`);
  if (f && !n) parts.push(f === 1 ? '1 folder' : `${f} folders`);
  return parts.join(', ');
}

// Move to…: the folders an entry can go into: not itself, nor one it holds.
const moving = ref(null);
const moveTo = ref('');
function within(id, folder) {
  for (let f = folder; f; f = state.bookmarks.find((e) => e.id === f)?.parent) if (f === id) return true;
  return false;
}
const targets = computed(() => (moving.value ? tree.value.filter((f) => !within(moving.value.id, f.id)) : []));
function startMove(e) {
  moving.value = e;
  moveTo.value = e.parent;
}
function move() {
  if (moveTo.value !== moving.value.parent) send({ t: 'moveBookmark', id: moving.value.id, parent: moveTo.value });
  moving.value = null;
}

const menus = {};
function items(e) {
  return e.kind === 'folder'
    ? [
        { label: 'Open', run: () => openFolder(e.id) },
        { label: 'Rename…', run: () => edit(e) },
        { label: 'Move to…', run: () => startMove(e) },
        { separator: true },
        { label: 'Delete', danger: true, run: () => remove(e) },
      ]
    : [
        { label: 'Open in new tab', run: () => open(e) },
        { label: 'Edit…', run: () => edit(e) },
        { label: 'Move to…', run: () => startMove(e) },
        { separator: true },
        { label: 'Delete', danger: true, run: () => remove(e) },
      ];
}

// Dragging an entry onto a folder, on the left or in the list, moves it there.
const dragged = ref(null);
const over = ref(null);
function dragStart(e, ev) {
  dragged.value = e;
  ev.dataTransfer.effectAllowed = 'move';
  ev.dataTransfer.setData('text/plain', e.url || e.title);
}
function dragOver(folder, ev) {
  const d = dragged.value;
  if (!d || d.parent === folder || within(d.id, folder)) return;
  ev.preventDefault();
  over.value = folder;
}
function drop(folder, ev) {
  ev.preventDefault();
  const d = dragged.value;
  if (d && d.parent !== folder && !within(d.id, folder)) send({ t: 'moveBookmark', id: d.id, parent: folder });
  dragEnd();
}
function dragEnd() {
  dragged.value = null;
  over.value = null;
}

const hostOf = (u) => {
  try {
    return new URL(u).host.replace(/^www\./, '');
  } catch {
    return u;
  }
};
</script>

<template>
  <div v-if="state.ready" class="page">
    <nav class="tree" aria-label="Folders">
      <button
        v-for="f in tree"
        :key="f.id"
        type="button"
        class="folder-row"
        :class="{ on: at === f.id && !text, drop: over === f.id }"
        :style="{ paddingLeft: `${8 + f.depth * 16}px` }"
        :aria-current="at === f.id && !text ? 'true' : undefined"
        @click="openFolder(f.id)"
        @dragover="dragOver(f.id, $event)"
        @dragleave="over === f.id && (over = null)"
        @drop="drop(f.id, $event)"
      >
        <svg class="glyph" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round" aria-hidden="true">
          <path d="M2 4.5a1 1 0 0 1 1-1h3l1.5 1.5H13a1 1 0 0 1 1 1V12a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1z" />
        </svg>
        <span class="name">{{ f.title }}</span>
        <span class="count">{{ count(f.id) }}</span>
      </button>
      <button type="button" class="folder-row new" @click="newFolder">
        <svg class="glyph" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true">
          <path d="M8 3v10M3 8h10" />
        </svg>
        <span class="name">New folder</span>
      </button>
    </nav>

    <div class="main">
      <header class="header">
        <div class="where">
          <ol v-if="trail.length > 1 && !text" class="trail" aria-label="Path">
            <li v-for="f in trail.slice(0, -1)" :key="f.id">
              <button
                type="button"
                class="crumb"
                :class="{ drop: over === f.id }"
                @click="upTo(f.id, openFolder)"
                @dragover="dragOver(f.id, $event)"
                @dragleave="over === f.id && (over = null)"
                @drop="drop(f.id, $event)"
              >{{ f.title }}</button>
              <span aria-hidden="true">›</span>
            </li>
          </ol>
          <h1>{{ text ? 'Search results' : here.title }}</h1>
        </div>
        <button type="button" class="text-button quiet new-here" @click="newFolder">New folder</button>
        <input v-model="text" class="text-field search" type="search" aria-label="Search bookmarks" placeholder="Search bookmarks" />
      </header>

      <main class="content">
        <form v-if="naming" class="naming" @submit.prevent="makeFolder" @keydown.esc.stop="naming = false">
          <input ref="newField" v-model="newName" class="text-field" aria-label="New folder name" :placeholder="`New folder in ${here.title}`" />
          <button type="submit" class="text-button primary">Make</button>
          <button type="button" class="text-button" @click="naming = false">Cancel</button>
        </form>
        <p v-if="state.refused" class="refused" role="alert">{{ state.refused.error.replace(/^bookmarks: /, '') }}</p>
        <p v-if="!state.bookmarks.length" class="empty">Bookmark a page with the star in the address bar, and it shows here.</p>
        <p v-else-if="!shown.length" class="empty">{{ text ? 'No bookmarks match.' : 'This folder is empty.' }}</p>
        <ul v-if="shown.length" class="card" role="list">
          <li
            v-for="e in shown"
            :key="e.id"
            class="row"
            :class="{ drop: over === e.id, dragged: dragged?.id === e.id }"
            role="listitem"
            :aria-label="e.title"
            :draggable="editing !== e.id"
            @dragstart="dragStart(e, $event)"
            @dragend="dragEnd"
            @dragover="e.kind === 'folder' && dragOver(e.id, $event)"
            @dragleave="over === e.id && (over = null)"
            @drop="e.kind === 'folder' && drop(e.id, $event)"
            @contextmenu.prevent="menus[e.id]?.open()"
          >
            <form v-if="editing === e.id" class="edit" @submit.prevent="save(e)" @keydown.esc.stop="editing = null">
              <input ref="titleField" v-model="title" class="text-field" aria-label="Name" placeholder="Name" />
              <input v-if="e.kind === 'bookmark'" v-model="url" class="text-field" aria-label="Address" placeholder="Address" />
              <button type="submit" class="text-button primary">Save</button>
              <button type="button" class="text-button" @click="editing = null">Cancel</button>
            </form>
            <template v-else>
              <button v-if="e.kind === 'folder'" type="button" class="link" @click="openFolder(e.id)">
                <svg class="glyph" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round" aria-hidden="true">
                  <path d="M2 4.5a1 1 0 0 1 1-1h3l1.5 1.5H13a1 1 0 0 1 1 1V12a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1z" />
                </svg>
                <span class="title">{{ e.title }}</span>
                <span class="host">{{ count(e.id) === 1 ? '1 bookmark' : `${count(e.id)} bookmarks` }}</span>
              </button>
              <a v-else class="link" :href="e.url" :title="e.url" draggable="false" @click="open(e, $event)">
                <svg class="glyph" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" aria-hidden="true">
                  <circle cx="8" cy="8" r="5.75" />
                  <path d="M2.25 8h11.5M8 2.25c1.6 1.6 2.3 3.6 2.3 5.75S9.6 12.15 8 13.75C6.4 12.15 5.7 10.15 5.7 8S6.4 3.85 8 2.25z" />
                </svg>
                <span class="title">{{ e.title }}</span>
                <span class="host">{{ hostOf(e.url) }}</span>
                <span v-if="text" class="in">in {{ folderName(e.parent) }}</span>
              </a>
              <RowMenu :ref="(m) => (menus[e.id] = m)" class="act" :items="items(e)" :label="`Actions for ${e.title}`" />
            </template>
          </li>
        </ul>
      </main>
    </div>

    <Dialog v-if="deleting" :title="`Delete “${deleting.title}”?`" @close="deleting = null">
      <p>It holds {{ holds(deleting) }}, which go with it.</p>
      <template #buttons>
        <button type="button" class="text-button" @click="deleting = null">Cancel</button>
        <button
          type="button"
          class="text-button primary"
          @click="
            send({ t: 'removeBookmark', id: deleting.id });
            deleting = null;
          "
        >Delete</button>
      </template>
    </Dialog>

    <Dialog v-if="moving" :title="`Move “${moving.title}” to`" @close="moving = null">
      <div class="targets" role="radiogroup" aria-label="Folder">
        <label v-for="f in targets" :key="f.id" class="target" :class="{ on: moveTo === f.id }" :style="{ paddingLeft: `${8 + f.depth * 16}px` }">
          <input v-model="moveTo" type="radio" name="move-to" :value="f.id" />
          <svg class="glyph" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round" aria-hidden="true">
            <path d="M2 4.5a1 1 0 0 1 1-1h3l1.5 1.5H13a1 1 0 0 1 1 1V12a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1z" />
          </svg>
          <span>{{ f.title }}</span>
        </label>
      </div>
      <template #buttons>
        <button type="button" class="text-button" @click="moving = null">Cancel</button>
        <button type="button" class="text-button primary" @click="move">Move</button>
      </template>
    </Dialog>
  </div>
</template>

<style>
body {
  background: var(--bar);
}

.page {
  display: flex;
  height: 100%;
}

.tree {
  flex: none;
  width: 240px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 16px 8px;
  box-sizing: border-box;
  overflow-y: auto;
  border-right: 1px solid var(--line);
}

.folder-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
  padding-right: 8px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: default;
}

.folder-row:hover {
  background: var(--accent-tint);
}

.folder-row.on {
  background: var(--accent-soft);
  font-weight: 600;
}

.folder-row.new {
  margin-top: 8px;
  color: var(--muted);
}

.folder-row .name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.count {
  flex: none;
  color: var(--muted);
  font-size: 12px;
  font-weight: 400;
}

.glyph {
  flex: none;
  color: var(--icon);
}

/* Where a drag would drop: the folder under it. */
.drop,
.folder-row.drop {
  background: var(--accent-tint);
  box-shadow: inset 0 0 0 2px var(--accent);
}

.main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.header {
  flex: none;
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 64px;
  padding: 12px 24px 0;
  box-sizing: border-box;
}

.where {
  flex: 1;
  min-width: 0;
}

.where h1 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.trail {
  display: flex;
  flex-wrap: wrap;
  margin: 0 0 2px;
  padding: 0;
  list-style: none;
  font-size: 13px;
  color: var(--muted);
}

.trail li {
  display: flex;
  align-items: center;
  gap: 2px;
}

.crumb {
  padding: 0 4px;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--muted);
  font: inherit;
}

.crumb:hover {
  color: var(--text);
  background: var(--accent-tint);
}

.search {
  flex: none;
  width: 240px;
}

/* The tree has New folder; a phone, with no tree, has it here. */
.new-here {
  display: none;
}

.content {
  flex: 1;
  overflow: auto;
  padding: 16px 24px 24px;
}

.naming {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.naming .text-field {
  flex: 1;
  min-width: 0;
}

.card {
  margin: 0;
  padding: 4px 0;
  list-style: none;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
}

.row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 40px;
  padding: 0 8px 0 16px;
}

.row:hover,
.row:focus-within {
  background: var(--accent-tint);
}

.row.dragged {
  opacity: 0.5;
}

/* The whole row opens the entry: plain words, the host or count in --muted. */
.link,
.link:hover {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  min-height: 40px;
  padding: 0;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
  text-decoration: none;
  outline: none;
}

.act {
  opacity: 0;
}

.row:hover .act,
.row:focus-within .act {
  opacity: 1;
}

.title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.host,
.in {
  flex: none;
  color: var(--muted);
}

.in {
  margin-left: auto;
  font-size: 12px;
}

.edit {
  flex: 1;
  display: flex;
  gap: 8px;
  padding: 6px 0;
}

.edit .text-field {
  flex: 1;
  min-width: 0;
}

.empty {
  margin: 48px 0;
  color: var(--muted);
  text-align: center;
}

.refused {
  margin: 0 0 12px;
  color: var(--danger);
}

.targets {
  max-height: 240px;
  overflow-y: auto;
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 4px;
}

.target {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
  padding-right: 8px;
  border-radius: 6px;
  font-size: 13px;
}

.target:hover {
  background: var(--accent-tint);
}

.target.on {
  background: var(--accent-soft);
  font-weight: 600;
}

.target input {
  position: absolute;
  opacity: 0;
  width: 1px;
  height: 1px;
  margin: 0;
}

.target:has(input:focus-visible) {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}

/* A phone: no room for the tree. The path leads back up, New folder is by the search, and
   a row's actions show at once, with no hover to bring them. */
.touch .tree {
  display: none;
}

.touch .header {
  flex-wrap: wrap;
  padding: 12px 16px 0;
}

.touch .where {
  flex-basis: 100%;
}

.touch .new-here {
  display: inline-flex;
}

.touch .search {
  flex: 1;
  width: auto;
}

.touch .content {
  padding: 12px 16px 24px;
}

.touch .act {
  opacity: 1;
}

.touch .row {
  min-height: 48px;
}
</style>
