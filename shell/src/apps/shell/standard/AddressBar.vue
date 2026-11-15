<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import { activeTab, chooseSuggestion, focusPage, send, state, suggest } from '@/sdk/vue.js';
import SuggestionList from './SuggestionList.vue';
import { hasSite, openSiteSettings, promptFor } from '@/sdk/sites.js';
import PermissionPrompt from '@/ui/PermissionPrompt.vue';
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

let latestQuery = 0;

const open = computed(() => focused.value && items.value.length > 0);
// A site's question about the active tab hangs from the field's start, as the site's own.
const prompt = computed(() => promptFor(state, state.active));
// What the slot shows: the site's settings on a web page, marked when its connection is
// anything but secure, and search with no site or while the field is being typed in.
const slot = computed(() => {
  const t = activeTab.value;
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
  latestQuery = 0;
}

function onInput() {
  edited.value = true;
  typed.value = text.value;
  highlight.value = -1;
  if (text.value.trim() === '') {
    close();
    return;
  }
  latestQuery = nextQuery();
  suggest(latestQuery, text.value);
}

// Moving through the list shows that item in the bar. Up from the first item, or down
// from the last, comes back to what the user typed.
function move(delta) {
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
    case 'Escape':
      if (open.value) {
        e.preventDefault();
        text.value = typed.value;
        close();
      } else {
        edited.value = false;
        text.value = activeTab.value.url;
        input.value.blur();
        focusPage();
      }
      break;
  }
}

// go takes a row of Go's list, the chosen one or the first, which is what Enter does; with
// no list yet, what was typed.
function go(i = highlight.value) {
  const row = items.value[i >= 0 ? i : 0];
  const value = text.value.trim();
  if (!row && value === '') return;
  edited.value = false;
  close();
  if (row) chooseSuggestion(row);
  else send({ t: 'navigate', text: value });
  input.value.blur();
  focusPage();
}

// Focusing the bar selects everything. When a click did the focusing, its mouse up
// would put the caret back down and clear the selection, so that one mouse up selects
// again. The first key clears this, so later clicks place the caret as usual.
let selectOnMouseUp = false;

function onFocus() {
  focused.value = true;
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
  close();
  if (!edited.value) text.value = activeTab.value.url;
}

onMounted(() => {
  addressBar.suggestions = (id, list) => {
    if (id === latestQuery) {
      items.value = list;
      highlight.value = -1;
    }
  };
  addressBar.focus = () => input.value.focus();
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
  // A blank tab's, or an empty window's, command box takes the keyboard (CommandBox.vue).
  input.value.blur();
  if (activeTab.value.url) focusPage();
}
watch(() => state.active, follow);
onMounted(follow);
</script>

<template>
  <form class="field" :class="{ open }" @submit.prevent="go()">
    <!-- The field's own icon, in a slot fused to its start (docs/design-system.md, The
         address field's slot): the site's settings on a web page, a button; search while
         there is no site or the field is being typed in. -->
    <button
      v-if="slot !== 'search'"
      type="button"
      class="slot site"
      :class="{ insecure: slot === 'insecure' }"
      :aria-label="slot === 'insecure' ? 'Not secure · Site settings' : 'Site settings'"
      :title="slot === 'insecure' ? 'Not secure · Site settings' : 'Site settings'"
      @mousedown.prevent
      @click="openSiteSettings()"
    >
      <svg v-if="slot === 'insecure'" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M8 2.5 14 13H2zM8 6.5v3M8 11.2v.1" />
      </svg>
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
    <input
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
      @input="onInput"
      @keydown="onKeydown"
      @mouseup="onMouseUp"
      @focus="onFocus"
      @blur="onBlur"
    />
    <span v-if="status" class="status">{{ status }}</span>
    <SuggestionList
      v-if="open"
      id="suggestions"
      class="list"
      :items="items"
      :highlight="highlight"
      @hover="(i) => (highlight = i)"
      @choose="go"
    />
    <PermissionPrompt v-if="prompt && !open" :key="prompt.id" v-shell class="prompt" :prompt="prompt" />
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

.slot.insecure {
  color: var(--danger);
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

</style>
