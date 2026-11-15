<script setup>
import { computed, nextTick, ref, watchEffect } from 'vue';
import { paintTheme } from '@/theme.js';
import { send, state } from './state.js';

// The Bookmarks page: the profile's bookmarks, newest first, a search among them, and each
// one's edit and removal (docs/daily-driver.md). A page is bookmarked from the address
// bar's star or Bookmark This Page.
watchEffect(() => paintTheme(state.tones, state.theme));

const text = ref('');
const editing = ref(null);
const title = ref('');
const url = ref('');
const titleField = ref(null);

const shown = computed(() => {
  const q = text.value.trim().toLowerCase();
  return q ? state.bookmarks.filter((b) => b.title.toLowerCase().includes(q) || b.url.toLowerCase().includes(q)) : state.bookmarks;
});

async function edit(b) {
  editing.value = b.id;
  title.value = b.title;
  url.value = b.url;
  state.refused = null;
  await nextTick();
  titleField.value?.[0]?.focus();
}

function save(b) {
  send({ t: 'editBookmark', id: b.id, title: title.value.trim(), url: url.value.trim() });
  editing.value = null;
}

function open(b, e) {
  send({ t: 'open', url: b.url });
  e.preventDefault();
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
    <h1 class="hidden-heading">Bookmarks</h1>
    <header class="header column">
      <input v-model="text" class="text-field search" type="search" aria-label="Search bookmarks" placeholder="Search bookmarks" />
    </header>
    <main class="content"><div class="column">
      <p v-if="!state.bookmarks.length" class="empty">Bookmark a page with the star in the address bar, and it shows here.</p>
      <p v-else-if="!shown.length" class="empty">No bookmarks match.</p>
      <p v-if="state.refused" class="refused" role="alert">{{ state.refused.error.replace(/^bookmarks: /, '') }}</p>
      <ul v-if="shown.length" class="card" role="list">
        <li v-for="b in shown" :key="b.id" class="row" role="listitem" :aria-label="b.title">
          <form v-if="editing === b.id" class="edit" @submit.prevent="save(b)" @keydown.esc="editing = null">
            <input ref="titleField" v-model="title" class="text-field" aria-label="Name" placeholder="Name" />
            <input v-model="url" class="text-field" aria-label="Address" placeholder="Address" />
            <button type="submit" class="text-button primary">Save</button>
            <button type="button" class="text-button" @click="editing = null">Cancel</button>
          </form>
          <template v-else>
            <a class="link" :href="b.url" :title="b.url" @click="open(b, $event)">
              <span class="title">{{ b.title }}</span>
              <span class="host">{{ hostOf(b.url) }}</span>
            </a>
            <button type="button" class="text-button quiet act" :aria-label="`Edit ${b.title}`" @click="edit(b)">Edit</button>
            <button type="button" class="icon-button remove act" :aria-label="`Remove bookmark ${b.title}`" title="Remove bookmark" @click="send({ t: 'removeBookmark', id: b.id })">
              <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="m2 2 6 6M8 2 2 8" /></svg>
            </button>
          </template>
        </li>
      </ul>
    </div></main>
  </div>
</template>

<style>
body {
  background: var(--bar);
}

.page {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.header {
  flex: none;
  display: flex;
  align-items: center;
  gap: 12px;
  height: 52px;
  padding: 0 24px;
  box-sizing: border-box;
}

.search {
  flex: 1;
}

.content {
  flex: 1;
  overflow: auto;
  padding: 16px 24px 24px;
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

.row:hover {
  background: var(--accent-tint);
}

/* The whole row opens the bookmark: plain words, the host in --muted. */
.link,
.link:hover {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  min-height: 40px;
  color: inherit;
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

.row:focus-within {
  background: var(--accent-tint);
}

.title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.host {
  flex: none;
  color: var(--muted);
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

/* The tab and the address area name the page, so its heading is for screen readers. */
.hidden-heading {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}

/* Settings' width: the search, the headings and the cards. The header keeps its own 24px
   sides, so its row lines up with the cards. */
.column {
  width: 100%;
  max-width: 720px;
  margin: 0 auto;
  box-sizing: border-box;
}

.header.column {
  max-width: 768px;
}
</style>
