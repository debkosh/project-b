<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue';
import { activeTab, bookmark, chooseSuggestion, focusPage, isBookmarked, send, state, suggest, tabLabel, typedFirst } from '@/sdk/vue.js';
import SuggestionList from './SuggestionList.vue';
import { hasSite, openSite, popupsWord, promptFor } from '@/sdk/sites.js';
import PermissionPrompt from '@/ui/PermissionPrompt.vue';
import { pushEscape } from '@/ui/escape.js';
import SitePanel from '@/ui/SitePanel.vue';
import BookmarkPanel from '@/ui/BookmarkPanel.vue';
import { addressBar, nextQuery } from './state.js';

const input = ref(null);
const focused = ref(false);
const text = ref('');
const typed = ref('');
const items = ref([]);
const highlight = ref(-1);
// Whether the bar holds what the user typed rather than the page's address. Typed text
// stays through a blur, the window losing focus or the page loading, as browsers keep it,
// until Enter, Escape or another tab takes it away.
const edited = ref(false);
// The inline completion: the rest of the first row's address after what was typed, shown
// selected after the caret, as Chrome and Safari show it (docs/daily-driver.md). A delete
// says the person wants what they typed, so the next answer completes nothing.
const inline = ref('');
// Whether an input method is composing: it holds what it composes, and a completion put in
// the field under it is typed over with it again. While it composes, an answer lists but
// doesn't complete; the input that ends the composition asks again.
let composing = false;
function compose(on) {
  composing = on;
}
let deleting = false;

let latestQuery = 0;
// The text the latest ask was for, and the text the list on screen answers: until Go's
// answer comes, the list is an earlier text's.
let asked = '';
let shownFor = '';

const open = computed(() => focused.value && items.value.length > 0);
// A site's question about the active tab hangs from the field's start, as the site's own.
const prompt = computed(() => promptFor(state, state.active));
// The site panel, which the site icon opens over the page; it closes with another tab.
const panel = ref(false);
watch(() => state.active, () => (panel.value = false));
const site = computed(() => openSite(state, activeTab.value));
// A tab of ours (settings, and our other pages) shows its icon in the slot and its title in
// the field, with a close control; there is nothing to type.
const ours = computed(() => !!activeTab.value.page);
// What the slot shows: the page's own icon on a tab of ours, the site's settings on a web
// page, marked when its connection is anything but secure, and search with no site or while
// the field is being typed in.
const slot = computed(() => {
  const t = activeTab.value;
  if (ours.value) return 'page';
  if (focused.value || !hasSite(t)) return 'search';
  return t.connection && t.connection !== 'secure' ? 'insecure' : 'site';
});

const status = computed(() => {
  const t = activeTab.value;
  if (t.blocked) return 'Blocked';
  if (t.crashed) return 'Crashed';
  if (t.error) return 'Failed';
  return '';
});

function close() {
  items.value = [];
  highlight.value = -1;
  inline.value = '';
  latestQuery = 0;
}

function ask() {
  latestQuery = nextQuery();
  asked = typed.value;
  suggest(latestQuery, typed.value);
}

function onInput(e) {
  edited.value = true;
  typed.value = text.value;
  highlight.value = -1;
  inline.value = '';
  deleting = e?.inputType?.startsWith('delete') ?? false;
  if (text.value.trim() === '') {
    close();
    return;
  }
  ask();
}

// showInline puts the completion after what was typed, selected, so the next key replaces
// it, Backspace takes it away, and Enter goes where the field shows.
function showInline(rest) {
  inline.value = rest;
  text.value = typed.value + rest;
  nextTick(() => input.value?.setSelectionRange(typed.value.length, text.value.length, 'backward'));
}

// A key that matches the completion's next letter keeps the rest of it on show, rather than
// dropping it until Go answers again.
function onBeforeInput(e) {
  const rest = inline.value;
  if (!rest || e.inputType !== 'insertText' || !e.data || highlight.value >= 0) return;
  const el = input.value;
  if (el.selectionStart !== typed.value.length || el.selectionEnd !== text.value.length) return;
  if (!rest.toLowerCase().startsWith(e.data.toLowerCase())) return;
  e.preventDefault();
  edited.value = true;
  typed.value += e.data;
  showInline(rest.slice(e.data.length));
  if (!inline.value) items.value = items.value.map((r, i) => (i === 0 ? { ...r, inline: '' } : r));
  ask();
}

// Moving through the list shows that item in the bar. Up from the first item, or down
// from the last, comes back to what the user typed.
function move(delta) {
  inline.value = '';
  const next = highlight.value + delta;
  if (next < 0 || next >= items.value.length) {
    highlight.value = -1;
    text.value = typed.value;
    return;
  }
  highlight.value = next;
  text.value = items.value[next].url || items.value[next].title;
}

function onKeydown(e) {
  selectOnMouseUp = false;
  switch (e.key) {
    case 'ArrowDown':
      if (open.value) {
        e.preventDefault();
        move(1);
      }
      break;
    case 'ArrowUp':
      if (open.value) {
        e.preventDefault();
        move(-1);
      }
      break;
  }
}

// Escape, while the field has the keyboard: the first closes the list and drops the
// completion, leaving what was typed; the next puts the page's address back and gives the
// page the keyboard. The field takes Escape ahead of anything else open, such as the find
// bar, by being the innermost layer of ui/escape.js while it has focus: before, an open find
// bar took the second Escape and the typed text stayed.
function escape() {
  if (open.value) {
    text.value = typed.value;
    close();
    return;
  }
  edited.value = false;
  text.value = activeTab.value.url;
  input.value?.blur();
  focusPage();
}
let dropEscape = null;
onUnmounted(() => dropEscape?.());

// go takes a row of Go's list: one the arrow keys or a click chose (i), or for Enter the
// first, but only while it belongs to the text in the field now: the completion shown is
// that row's rest after what was typed, or the list answers what is typed now. Otherwise,
// as before any list comes, Enter takes what was typed, so a fast typist's Enter never
// opens a row of the list for an earlier text.
function go(i = highlight.value) {
  const completes = !!inline.value && text.value === typed.value + inline.value;
  const current = completes || shownFor === typed.value;
  const row = i >= 0 ? items.value[i] : current ? items.value[0] : undefined;
  const value = (i >= 0 || completes ? text.value : typed.value).trim();
  if (!row && value === '') return;
  edited.value = false;
  close();
  if (row) chooseSuggestion(row);
  else send({ t: 'navigate', text: value, typed: true });
  input.value?.blur();
  focusPage();
}

// Focusing the bar selects everything. When a click did the focusing, its mouse up
// would put the caret back down and clear the selection, so that one mouse up selects
// again. The first key clears this, so later clicks place the caret as usual.
let selectOnMouseUp = false;

function onFocus() {
  focused.value = true;
  dropEscape?.();
  dropEscape = pushEscape(escape);
  typed.value = text.value;
  selectOnMouseUp = true;
  nextTick(() => input.value.select());
}

function onMouseUp(e) {
  if (selectOnMouseUp) {
    e.preventDefault();
    input.value.select();
    selectOnMouseUp = false;
  }
}

function onBlur() {
  focused.value = false;
  dropEscape?.();
  dropEscape = null;
  if (inline.value) text.value = typed.value;
  close();
  if (!edited.value) text.value = activeTab.value.url;
}

onMounted(() => {
  addressBar.suggestions = (id, list) => {
    if (id !== latestQuery) return;
    const first = list[0];
    if (first?.inline && deleting) list = typedFirst(list, asked);
    items.value = list;
    shownFor = asked;
    highlight.value = -1;
    // Completed only while the field holds what was typed with the caret at its end, or
    // with the completion still selected after it.
    const el = input.value;
    if (!el) return;
    const end = el.selectionEnd === text.value.length;
    const plain = text.value === typed.value && el.selectionStart === el.selectionEnd && end;
    const shown = !!inline.value && text.value === typed.value + inline.value && el.selectionStart === typed.value.length && end;
    if (list[0]?.inline && !composing && (plain || shown)) showInline(list[0].inline);
    else if (shown) {
      inline.value = '';
      text.value = typed.value;
    }
  };
  addressBar.focus = () => input.value?.focus();
});

watch(
  () => activeTab.value.url,
  (url) => {
    if (!focused.value && !edited.value) text.value = url;
  },
);

// A tab that becomes active with a page wants the page; one with nothing in it, or a window
// with no tabs, shows the command box, which takes the keyboard itself. The bar is made once
// Go's data is in, so the state it is made in counts too.
function follow() {
  close();
  edited.value = false;
  text.value = activeTab.value.url;
  // A new tab's command box takes the keyboard (CommandBox.vue).
  input.value?.blur();
  if (activeTab.value.url) focusPage();
}
watch(() => state.active, follow);
onMounted(follow);
// The star: the page shown is bookmarked, or not. A press asks Go to bookmark it, into the
// folder last saved in, or to find its bookmark; Go answers with the panel to show, as it
// does for Cmd+D (docs/bookmark-folders.md).
const starred = computed(() => isBookmarked(state, activeTab.value.url));
// The star's panel Go asked for, and a new key for each ask, so each starts afresh. It
// closes with another tab, and gives the site panel its place.
const marked = ref(null);
let asks = 0;
watch(
  () => state.bookmarkPanel,
  (p) => {
    marked.value = p && { ...p, key: ++asks };
    if (p) panel.value = false;
  },
);
watch(() => state.active, () => (marked.value = null));
watch(panel, (on) => on && (marked.value = null));
const markedBookmark = computed(() => marked.value && state.bookmarks.find((b) => b.id === marked.value.id));
function star() {
  if (markedBookmark.value) closeMarked();
  else bookmark(activeTab.value.url, activeTab.value.title);
}
// Closing gives the page the focus back, which Cmd+D took for the panel.
function closeMarked() {
  marked.value = null;
  if (activeTab.value.url) focusPage();
}
</script>

<template>
  <!-- The field is the shell's all over, in the bar that drags: its slot, a tab of ours's
       title, and the field, so a drag across it never moves the window. -->
  <form v-shell class="field" :class="{ open }" @submit.prevent="go()">
    <!-- The field's own icon, in a slot fused to its start (docs/design-system.md, The
         address field's slot): a page of ours's own icon; the site's settings on a web page,
         a button; search while there is no site or the field is being typed in. -->
    <span v-if="slot === 'page'" class="slot" aria-hidden="true">
      <img v-if="activeTab.favicon" :src="activeTab.favicon" alt="" width="16" height="16" />
      <svg v-else width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" aria-hidden="true">
        <path d="M4 2.5h5.5L12 5v8.5H4z" />
      </svg>
    </span>
    <button
      v-else-if="slot !== 'search'"
      type="button"
      class="slot site"
      :class="{ insecure: slot === 'insecure' }"
      :aria-label="slot === 'insecure' ? 'Not secure · Site settings' : 'Site settings'"
      :title="slot === 'insecure' ? 'Not secure · Site settings' : 'Site settings'"
      @mousedown.prevent
      :aria-expanded="panel"
      @click="panel = !panel"
    >
      <!-- Not secure says so in words, as Chrome and Edge do: an http page, or a certificate
           passed with Open anyway. -->
      <template v-if="slot === 'insecure'">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M8 2.5 14 13H2zM8 6.5v3M8 11.2v.1" />
        </svg>
        <span class="words" aria-hidden="true">Not secure</span>
      </template>
      <svg v-else width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true">
        <path d="M2.5 5h5M11.5 5h2M2.5 11h2M8.5 11h5" />
        <circle cx="9.5" cy="5" r="1.75" />
        <circle cx="6.5" cy="11" r="1.75" />
      </svg>
    </button>
    <span v-else class="slot" aria-hidden="true" @mousedown.prevent="input.focus()">
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true">
        <circle cx="7" cy="7" r="4.25" />
        <path d="m10.25 10.25 3.25 3.25" />
      </svg>
    </span>
    <!-- A tab of ours: its title where the address would be, and a close control. -->
    <template v-if="ours">
      <span class="page-title" :title="tabLabel(activeTab)">{{ tabLabel(activeTab) }}</span>
      <button type="button" class="page-close" :aria-label="`Close ${tabLabel(activeTab)}`" @click="send({ t: 'closeTab', id: state.active })">
        <svg width="9" height="9" viewBox="0 0 9 9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true"><path d="M7.43.9.75 7.58M.9.75l6.68 6.68" /></svg>
      </button>
    </template>
    <input
      v-else
      ref="input"
      v-model="text"
      aria-label="Address"
      :placeholder="state.searchEngine.name ? `Search ${state.searchEngine.name} or type a URL` : 'Search or type a URL'"
      role="combobox"
      :aria-expanded="open"
      aria-controls="suggestions"
      :aria-activedescendant="highlight >= 0 ? `suggestion-${highlight}` : undefined"
      autocomplete="off"
      autocapitalize="off"
      spellcheck="false"
      @beforeinput="onBeforeInput"
      @input="onInput"
      @compositionstart="compose(true)"
      @compositionend="compose(false)"
      @keydown="onKeydown"
      @mouseup="onMouseUp"
      @focus="onFocus"
      @blur="onBlur"
    />
    <span v-if="status && !ours" class="status">{{ status }}</span>
    <!-- The popups Links to other sites closed on this page: a press opens the site panel. -->
    <button
      v-if="activeTab.popupsBlocked && !ours && !focused"
      type="button"
      class="popups"
      :aria-label="popupsWord(activeTab.popupsBlocked)"
      :title="popupsWord(activeTab.popupsBlocked)"
      @mousedown.prevent
      @click="panel = true"
    >
      <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <rect x="2" y="3.5" width="9" height="8" rx="1.5" />
        <path d="M5 1.75h7.75a1.5 1.5 0 0 1 1.5 1.5V9M2.5 13.5l11-11" />
      </svg>
      <span>{{ activeTab.popupsBlocked }}</span>
    </button>
    <button
      v-if="activeTab.url && !ours && !focused"
      type="button"
      class="star"
      :class="{ on: starred }"
      data-bookmark-star
      :aria-label="starred ? 'Edit bookmark' : 'Bookmark this page'"
      :aria-pressed="starred"
      :aria-expanded="!!markedBookmark"
      :title="starred ? 'Edit bookmark' : 'Bookmark this page'"
      @mousedown.prevent
      @click="star"
    >
      <svg width="15" height="15" viewBox="0 0 16 16" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round" :fill="starred ? 'currentColor' : 'none'" aria-hidden="true">
        <path d="M8 1.8l1.9 3.9 4.3.6-3.1 3 .7 4.3L8 11.6l-3.8 2 .7-4.3-3.1-3 4.3-.6z" />
      </svg>
    </button>
    <SuggestionList
      v-if="open"
      id="suggestions"
      class="list"
      :items="items"
      :highlight="highlight"
      @hover="(i) => (highlight = i)"
      @choose="go"
    />
    <PermissionPrompt v-if="prompt && !open && !panel && !markedBookmark" :key="prompt.id" v-shell class="prompt" :prompt="prompt" />
    <SitePanel v-if="panel && !open && hasSite(activeTab)" v-shell class="prompt" :tab="activeTab" :site="site" @close="panel = false" />
    <BookmarkPanel
      v-if="markedBookmark && !open"
      :key="marked.key"
      v-shell
      class="prompt end"
      :bookmark="markedBookmark"
      :list="state.bookmarks"
      :added="marked.new"
      @close="closeMarked"
    />
  </form>
</template>

<style scoped>
/* Taller than the icon buttons beside it, with 6px of bar above and below. Edged as the
   design system's text field is, a --line hairline; the focus ring replaces it while
   typing, and stays inside the bar. */
.field {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  height: 32px;
  margin: 0;
  border-radius: 6px;
  background: var(--surface);
  box-shadow: 0 0 0 1px var(--line);
  box-sizing: border-box;
}

.field:focus-within {
  box-shadow: var(--focus-ring);
}

.popups {
  flex: none;
  display: flex;
  align-items: center;
  gap: 3px;
  height: 24px;
  padding: 0 6px;
  border: 0;
  border-radius: 6px;
  background: var(--accent-tint);
  color: var(--icon);
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.star {
  flex: none;
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  margin-right: 2px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--icon);
  cursor: pointer;
}

.star:hover {
  background: var(--accent-tint);
}

.star.on {
  color: var(--accent);
}

input {
  flex: 1;
  min-width: 0;
  height: 100%;
  padding: 0 12px;
  border: 0;
  border-radius: 6px;
  outline: none;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: 13px;
}

input::placeholder {
  color: var(--muted);
  opacity: 1;
}

/* A quiet icon button inside the field, 24px, with the field's start padding outside it. */
/* The slot: 34px at the field's start, inside its edge, tinted and parted from the white
   area by a hairline, so it reads as the field's own icon. */
.slot {
  display: grid;
  place-items: center;
  flex: none;
  align-self: stretch;
  width: 34px;
  padding: 0;
  border: 0;
  border-right: 1px solid var(--line);
  border-radius: 6px 0 0 6px;
  background: var(--accent-tint);
  color: var(--muted);
}

.slot.site {
  color: var(--icon);
  cursor: pointer;
}

.slot.site:hover {
  background: var(--accent-soft);
}

.slot.site:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

.page-title {
  flex: 1;
  min-width: 0;
  padding: 0 12px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  font-size: 13px;
  font-weight: 500;
}

.page-close {
  display: grid;
  place-items: center;
  flex: none;
  width: 24px;
  height: 24px;
  margin-right: 4px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--muted);
  cursor: pointer;
}

.page-close:hover {
  background: var(--press);
  color: var(--text);
}

.page-close:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

.slot img {
  display: block;
  width: 16px;
  height: 16px;
}

/* Not secure: the warning and the words in --danger, the slot as wide as they need, the
   words at the size of the field's other small labels. */
.slot.insecure {
  display: flex;
  gap: 6px;
  width: auto;
  padding: 0 10px 0 9px;
  color: var(--danger);
}

.words {
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
}

.status {
  flex: none;
  margin-right: 8px;
  color: var(--danger);
  font-size: 13px;
  font-weight: 500;
}

/* The list floats under the field, as wide as it (SuggestionList.vue draws it). */
.list {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  right: 0;
  z-index: 10;
}

.prompt {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 10;
}

/* The star's panel hangs from the field's end, under the star. */
.prompt.end {
  left: auto;
  right: 0;
}

</style>
