<script setup>
import { computed, ref, watchEffect } from 'vue';
import { paintTheme } from '@/theme.js';
import { send, state } from './state.js';

// The History page: the pages visited, the latest first and grouped by day, a search, a
// row's removal, and Clear for the last hour, today or all of it (docs/daily-driver.md).
watchEffect(() => paintTheme(state.tones, state.theme));

const clearing = ref(false);
let typing = 0;

function search(text) {
  state.text = text;
  clearTimeout(typing);
  typing = setTimeout(() => send({ t: 'search', text }), 120);
}

function more() {
  const last = state.items[state.items.length - 1];
  if (last) send({ t: 'more', text: state.text, before: last.lastVisit });
}

function open(item, e) {
  send({ t: 'open', url: item.url });
  e.preventDefault();
}

function remove(item) {
  send({ t: 'delete', text: state.text, urls: [item.url] });
}

// Clear: from an hour ago, from the start of today, or everything.
function clear(span) {
  const now = new Date();
  const since = span === 'hour' ? Math.floor(now / 1000) - 3600 : span === 'today' ? Math.floor(new Date(now.getFullYear(), now.getMonth(), now.getDate()) / 1000) : 0;
  send({ t: 'clear', text: state.text, since });
  clearing.value = false;
}

function dayOf(seconds) {
  const d = new Date(seconds * 1000);
  const today = new Date();
  const yesterday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 1);
  if (d.toDateString() === today.toDateString()) return 'Today';
  if (d.toDateString() === yesterday.toDateString()) return 'Yesterday';
  return d.toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long', year: d.getFullYear() === today.getFullYear() ? undefined : 'numeric' });
}

const days = computed(() => {
  const out = [];
  for (const item of state.items) {
    const day = dayOf(item.lastVisit);
    if (!out.length || out[out.length - 1].day !== day) out.push({ day, items: [] });
    out[out.length - 1].items.push(item);
  }
  return out;
});

const timeOf = (seconds) => new Date(seconds * 1000).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
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
    <h1 class="hidden-heading">History</h1>
    <header class="header column">
      <input class="text-field search" type="search" aria-label="Search history" placeholder="Search history" :value="state.text" @input="search($event.target.value)" />
      <div class="clear">
        <button type="button" class="text-button" aria-haspopup="true" :aria-expanded="clearing" @click="clearing = !clearing">Clear history…</button>
        <div v-if="clearing" class="clear-menu" role="group" aria-label="Clear history">
          <button type="button" class="text-button quiet" @click="clear('hour')">The last hour</button>
          <button type="button" class="text-button quiet" @click="clear('today')">Today</button>
          <button type="button" class="text-button quiet danger" @click="clear('all')">All history</button>
        </div>
      </div>
    </header>
    <main class="content"><div class="column">
      <p v-if="!state.items.length" class="empty">{{ state.text ? 'No pages in your history match.' : 'Pages you visit show here.' }}</p>
      <section v-for="d in days" :key="d.day" :aria-label="d.day">
        <h2>{{ d.day }}</h2>
        <ul class="card" role="list">
          <li v-for="item in d.items" :key="item.url" class="row" role="listitem" :aria-label="item.title || item.url">
            <span class="time">{{ timeOf(item.lastVisit) }}</span>
            <a class="link" :href="item.url" :title="item.url" @click="open(item, $event)">
              <span class="title">{{ item.title || item.url }}</span>
              <span class="host">{{ hostOf(item.url) }}</span>
            </a>
            <button type="button" class="icon-button remove" :aria-label="`Remove ${item.title || item.url} from history`" title="Remove from history" @click="remove(item)">
              <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="m2 2 6 6M8 2 2 8" /></svg>
            </button>
          </li>
        </ul>
      </section>
      <button v-if="state.more" type="button" class="text-button more" @click="more">Show more</button>
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

.clear {
  position: relative;
  margin-left: auto;
}

.clear-menu {
  position: absolute;
  top: calc(100% + 4px);
  right: 0;
  z-index: 2;
  display: flex;
  flex-direction: column;
  min-width: 180px;
  padding: 4px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  box-shadow: var(--shadow-float);
}

.clear-menu .text-button {
  justify-content: flex-start;
  text-align: left;
}

.clear-menu .danger {
  color: var(--danger);
}

.content {
  flex: 1;
  overflow: auto;
  padding: 8px 24px 24px;
}

h2 {
  margin: 16px 0 8px;
  color: var(--muted);
  font-size: calc(12rem / 16);
  font-weight: 600;
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
  gap: 12px;
  height: 40px;
  padding: 0 8px 0 16px;
}

.row:hover {
  background: var(--accent-tint);
}

.time {
  flex: none;
  width: 64px;
  color: var(--muted);
  font-size: calc(12rem / 16);
}

.link {
  flex: 1;
  display: flex;
  gap: 8px;
  min-width: 0;
  color: inherit;
  text-decoration: none;
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

.remove {
  opacity: 0;
}

.row:hover .remove,
.row:focus-within .remove {
  opacity: 1;
}

.empty {
  margin: 48px 0;
  color: var(--muted);
  text-align: center;
}

.more {
  margin: 16px auto 0;
  display: block;
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
