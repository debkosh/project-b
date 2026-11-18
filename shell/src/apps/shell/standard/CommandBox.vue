<script setup>
import { computed, nextTick, onMounted, ref } from 'vue';
import { chooseSuggestion, send, state, suggest } from '@/sdk/vue.js';
import SuggestionList from './SuggestionList.vue';
import NewTabBookmarks from './NewTabBookmarks.vue';
import BookmarkFinder from './BookmarkFinder.vue';
import { finderFolders, finderResults, openGroup, openHere } from './bookmarks.js';
import { newTabBox, nextQuery } from './state.js';
import { inFolder } from '@/ui/bookmarks.js';

// Standard's command box, the shell's own view in the page area: the bar's quiet tint and
// one box in the middle. Typing there asks Go for the same list the address field gets, and
// draws it under the box; with nothing typed, Go's list for a new tab: the sites visited
// most and the newest bookmarks, and under it the bookmarks themselves while their pref says
// (NewTabBookmarks.vue). Enter takes the first row, or the one the arrow keys chose. Over
// a page (New Tab), a chosen row opens in a new tab and Escape goes back to the page; on a
// tab with nothing in it, the row loads there.
//
// While bookmarksCommandBox is on, a * typed first narrows the box to bookmarks
// (BookmarkFinder.vue), with no list from Go: the arrows up and down choose a bookmark, left
// and right a folder, Enter opens the bookmark and Shift+Enter the folder's bookmarks as a
// group. Escape, or Backspace with nothing typed, leaves it.
const input = ref(null);
const text = ref('');
const items = ref([]);
const highlight = ref(0);
let latest = 0;
// The text the latest ask was for, and the text the list on screen answers: until Go's
// answer comes, the list is an earlier text's, such as the new tab's for nothing typed.
let asked = '';
let shownFor = '';
// The arrow keys chose a row in the list on screen, since the last key typed.
let picked = false;

// Whether the box is narrowed to bookmarks, and the folder chosen on its left, by its place
// in the list.
const marks = ref(false);
const folderAt = ref(0);
const finderList = computed(() => finderFolders(state.bookmarks));
const folder = computed(() => finderList.value[Math.min(folderAt.value, finderList.value.length - 1)]);
const results = computed(() => (marks.value ? finderResults(state.bookmarks, folder.value.id, text.value) : []));

// With nothing typed the box lists the sites visited most and the newest bookmarks, so it
// asks for those two kinds alone, not the open and closed tabs a phone's field lists; the
// sites alone while the bookmarks show as tiles under it.
function onInput() {
  if (!marks.value && state.prefs.bookmarksCommandBox && text.value.startsWith('*')) {
    marks.value = true;
    folderAt.value = 0;
    text.value = text.value.slice(1);
  }
  // Narrowed, the box asks Go nothing, and an answer to an earlier ask is dropped.
  if (marks.value) {
    latest = nextQuery();
    highlight.value = 0;
    return;
  }
  latest = nextQuery();
  asked = text.value.trim();
  picked = false;
  if (asked === '') suggest(latest, '', state.prefs.bookmarksNewTab ? ['history'] : ['history', 'bookmark']);
  else suggest(latest, text.value);
}

// choose takes a row of Go's list, one pressed or the arrow keys' choice. Enter takes the
// first row only once the list answers what is typed now; before that it takes what was
// typed, as the address field does, in the new tab the box is on. So a fast typist's Enter
// never opens a row of the list for an earlier text.
function choose(i = highlight.value, pressed = false) {
  const current = pressed || picked || shownFor === text.value.trim();
  const row = current ? items.value[i] : undefined;
  const typed = text.value.trim();
  if (row) chooseSuggestion(row, false);
  else if (typed) send({ t: 'navigate', text: typed, typed: true });
}

function leaveMarks() {
  marks.value = false;
  text.value = '';
  onInput();
}

function onMarksKeydown(e) {
  const n = results.value.length;
  const step = { ArrowDown: 1, ArrowUp: -1 }[e.key];
  const side = { ArrowRight: 1, ArrowLeft: -1 }[e.key];
  if (step && n) highlight.value = (highlight.value + step + n) % n;
  else if (side) {
    folderAt.value = Math.max(0, Math.min(finderList.value.length - 1, folderAt.value + side));
    highlight.value = 0;
  } else if (e.key === 'Enter' && e.shiftKey) {
    const urls = inFolder(state.bookmarks, folder.value.id).filter((x) => x.kind === 'bookmark').map((x) => x.url);
    if (!folder.value.id || !urls.length) return;
    openGroup(urls, folder.value.title);
  } else if (e.key === 'Enter') {
    if (n) openHere(results.value[highlight.value].e.url);
  } else if (e.key === 'Escape' || (e.key === 'Backspace' && text.value === '')) leaveMarks();
  else return;
  e.preventDefault();
}

function onKeydown(e) {
  if (marks.value) return onMarksKeydown(e);
  const n = items.value.length;
  if (e.key === 'ArrowDown' && n) {
    e.preventDefault();
    highlight.value = (highlight.value + 1) % n;
    picked = true;
  } else if (e.key === 'ArrowUp' && n) {
    e.preventDefault();
    highlight.value = (highlight.value - 1 + n) % n;
    picked = true;
  } else if (e.key === 'Enter') {
    e.preventDefault();
    choose();
  } else if (e.key === 'Escape') {
    e.preventDefault();
    if (text.value) {
      text.value = '';
      onInput();
    }
  }
}

// The box has the keyboard as the tab opens: the shell page takes it from the page first.
function focusBox() {
  send({ t: 'focusShell' });
  nextTick(() => input.value?.focus());
}

onMounted(() => {
  newTabBox.suggestions = (id, list) => {
    if (id === latest) {
      items.value = list;
      shownFor = asked;
      highlight.value = 0;
    }
  };
  newTabBox.focus = focusBox;
  focusBox();
  onInput();
});
</script>

<template>
  <div v-shell class="command-box">
    <div class="box">
      <span v-if="marks" class="chip">
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="m8 2.5 1.7 3.5 3.8.5-2.8 2.6.7 3.8L8 11.1l-3.4 1.8.7-3.8-2.8-2.6 3.8-.5z" /></svg>
        Bookmarks
      </span>
      <svg v-else class="glyph" viewBox="0 0 16 16" aria-hidden="true"><circle cx="7" cy="7" r="4.25" /><path d="m10.25 10.25 3.25 3.25" /></svg>
      <input
        ref="input"
        v-model="text"
        :aria-label="marks ? 'Search bookmarks' : 'Search tabs, history, actions or the web'"
        :placeholder="marks ? 'Search bookmarks' : `Search tabs, history, actions or ${state.searchEngine.name || 'the web'}`"
        role="combobox"
        :aria-expanded="marks || items.length > 0"
        :aria-controls="marks ? 'bookmark-list' : 'new-tab-list'"
        :aria-activedescendant="marks ? (results.length ? `bookmark-list-${highlight}` : undefined) : items.length ? `new-tab-list-${highlight}` : undefined"
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
        @input="onInput"
        @keydown="onKeydown"
      />
      <span v-if="marks" class="hint">Esc leaves bookmarks</span>
    </div>
    <BookmarkFinder
      v-if="marks"
      id="bookmark-list"
      :folders="finderList"
      :at="folderAt"
      :results="results"
      :highlight="highlight"
      :typed="text.trim() !== ''"
      @folder="(i) => ((folderAt = i), (highlight = 0))"
      @hover="(i) => (highlight = i)"
      @choose="(i) => openHere(results[i].e.url)"
    />
    <SuggestionList v-else-if="items.length" id="new-tab-list" class="list" :items="items" :highlight="highlight" :first="text.trim() === ''" @hover="(i) => (highlight = i)" @choose="(i) => choose(i, true)" />
    <NewTabBookmarks v-if="!marks && state.prefs.bookmarksNewTab && text.trim() === ''" />
  </div>
</template>

<style scoped>
/* The page area, ours: the bar's colour, quieter than a white page, with the box a third
   of the way down. It appears with the tab, in the design system's motion. */
.command-box {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: max(48px, 22vh);
  overflow-y: auto;
  scrollbar-width: none;
  box-sizing: border-box;
  background: color-mix(in srgb, var(--bar) 60%, var(--surface));
  animation: appear var(--motion) var(--ease);
}

@keyframes appear {
  from {
    opacity: 0;
  }
}

.box {
  flex: none;
  display: flex;
  align-items: center;
  gap: 10px;
  width: min(520px, 86%);
  height: 44px;
  padding: 0 14px;
  border-radius: 8px;
  background: var(--surface);
  box-shadow: 0 0 0 1px var(--line);
  box-sizing: border-box;
}

.box:focus-within {
  box-shadow: var(--focus-ring);
}

.glyph {
  flex: none;
  width: 16px;
  height: 16px;
  fill: none;
  stroke: var(--muted);
  stroke-width: 1.5;
  stroke-linecap: round;
}

input {
  flex: 1;
  min-width: 0;
  height: 100%;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: 15px;
}

/* Bookmarks only: a chip in the accent before what is typed. */
.chip {
  display: flex;
  align-items: center;
  flex: none;
  gap: 6px;
  height: 26px;
  margin-left: -6px;
  padding: 0 10px 0 8px;
  border-radius: 6px;
  background: var(--accent);
  color: var(--on-accent);
  font-size: 13px;
  font-weight: 500;
}

.chip svg {
  width: 14px;
  height: 14px;
  fill: currentColor;
}

.hint {
  flex: none;
  color: var(--muted);
  font-size: 12px;
}

input::placeholder {
  color: var(--muted);
  opacity: 1;
}

.list {
  flex: none;
  width: min(520px, 86%);
  margin-top: 8px;
  box-sizing: border-box;
}
</style>
