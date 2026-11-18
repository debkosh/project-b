<script setup>
import { nextTick, onMounted, ref } from 'vue';
import { chooseSuggestion, focusPage, send, state, suggest } from '@/sdk/vue.js';
import SuggestionList from './SuggestionList.vue';
import NewTabBookmarks from './NewTabBookmarks.vue';
import { closeBox, newTabBox, nextQuery } from './state.js';

// Standard's command box, the shell's own view in the page area: the bar's quiet tint and
// one box in the middle. Typing there asks Go for the same list the address field gets, and
// draws it under the box; with nothing typed, Go's list for a new tab: the sites visited
// most and the newest bookmarks, and under it the bookmarks themselves while their pref says
// (NewTabBookmarks.vue). Enter takes the first row, or the one the arrow keys chose. Over
// a page (New Tab), a chosen row opens in a new tab and Escape goes back to the page; on a
// tab with nothing in it, the row loads there.
const props = defineProps({ overPage: Boolean });
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

// With nothing typed the box lists the sites visited most and the newest bookmarks, so it
// asks for those two kinds alone, not the open and closed tabs a phone's field lists; the
// sites alone while the bookmarks show as tiles under it.
function onInput() {
  latest = nextQuery();
  asked = text.value.trim();
  picked = false;
  if (asked === '') suggest(latest, '', state.prefs.bookmarksNewTab ? ['history'] : ['history', 'bookmark']);
  else suggest(latest, text.value);
}

// choose takes a row of Go's list, one pressed or the arrow keys' choice. Enter takes the
// first row only once the list answers what is typed now; before that it takes what was
// typed, as the address field does: in a new tab over a page or with no tabs, else in this
// one. So a fast typist's Enter never opens a row of the list for an earlier text.
function choose(i = highlight.value, pressed = false) {
  const current = pressed || picked || shownFor === text.value.trim();
  const row = current ? items.value[i] : undefined;
  const typed = text.value.trim();
  const fresh = props.overPage || state.active === 0;
  if (row) chooseSuggestion(row, fresh);
  else if (typed) send(fresh ? { t: 'newTab', text: typed } : { t: 'navigate', text: typed });
  else return;
  closeBox();
}

function onKeydown(e) {
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
    } else if (props.overPage) {
      closeBox();
      focusPage();
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
  <div v-shell class="command-box" :class="{ 'over-page': overPage }">
    <div class="box">
      <svg class="glyph" viewBox="0 0 16 16" aria-hidden="true"><circle cx="7" cy="7" r="4.25" /><path d="m10.25 10.25 3.25 3.25" /></svg>
      <input
        ref="input"
        v-model="text"
        aria-label="Search tabs, history, actions or the web"
        :placeholder="`Search tabs, history, actions or ${state.searchEngine.name || 'the web'}`"
        role="combobox"
        :aria-expanded="items.length > 0"
        aria-controls="new-tab-list"
        :aria-activedescendant="items.length ? `new-tab-list-${highlight}` : undefined"
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
        @input="onInput"
        @keydown="onKeydown"
      />
    </div>
    <SuggestionList v-if="items.length" id="new-tab-list" class="list" :items="items" :highlight="highlight" :first="text.trim() === ''" @hover="(i) => (highlight = i)" @choose="(i) => choose(i, true)" />
    <NewTabBookmarks v-if="state.prefs.bookmarksNewTab && text.trim() === ''" :over-page="overPage" />
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

/* Over a page, New Tab's: the page shows faintly through a scrim in the bar's tint, and the
   box floats on it. The page is a view of its own under the shell, so the scrim can tint it
   but not blur it. */
.command-box.over-page {
  background: color-mix(in srgb, var(--bar) 80%, transparent);
}

.over-page .box {
  box-shadow: 0 0 0 1px var(--line), var(--shadow-float);
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
