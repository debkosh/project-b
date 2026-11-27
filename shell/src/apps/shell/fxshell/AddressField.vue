<script setup>
import { paint } from '@/ui/hues.js';
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import { addressField } from './address.js';
import { activeTab, focusPage, send, state, workspaceOf } from '@/sdk/vue.js';
import { hasSite, openSite, popupsWord, promptFor, siteMark } from '@/sdk/sites.js';
import PermissionPrompt from '@/ui/PermissionPrompt.vue';
import SitePanel from '@/ui/SitePanel.vue';

// The active tab's address. Focused, it selects everything and grows a little; typing
// asks Go for suggestions, which open in a panel attached under it, as wide as it. Up and
// Down choose one, Enter or a click goes. Escape closes the panel, then puts the address
// back and gives the keyboard to the page.
const input = ref(null);
const focused = ref(false);
const text = ref('');
const typed = ref('');
const items = ref([]);
const highlight = ref(-1);

let latestQuery = 0;

const open = computed(() => focused.value && items.value.length > 0);
const workspace = computed(() => workspaceOf(activeTab.value));

function close() {
  items.value = [];
  highlight.value = -1;
  latestQuery++;
}

function onInput() {
  typed.value = text.value;
  highlight.value = -1;
  if (text.value.trim() === '') {
    close();
    return;
  }
  send({ t: 'suggest', id: ++latestQuery, text: text.value });
}

// Moving through the list shows that item in the field. Up from the first item, or down
// from the last, comes back to what the user typed.
function move(delta) {
  const next = highlight.value + delta;
  if (next < 0 || next >= items.value.length) {
    highlight.value = -1;
    text.value = typed.value;
    return;
  }
  highlight.value = next;
  text.value = items.value[next].url;
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
        text.value = activeTab.value.url;
        input.value.blur();
        focusPage();
      }
      break;
  }
}

function go(target) {
  const value = (target ?? text.value).trim();
  if (value === '') return;
  text.value = value;
  close();
  send({ t: 'navigate', text: value, typed: true });
  input.value.blur();
  focusPage();
}

// Focusing selects everything. When a click did the focusing, its mouse up would put the
// caret down and clear the selection, so that one mouse up selects again. The first key
// clears this, so later clicks place the caret as usual.
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
  text.value = activeTab.value.url;
}

// The shell page has to hold the keyboard before the field can.
function focusField() {
  send({ t: 'focusShell' });
  input.value.focus();
}

onMounted(() => {
  addressField.suggestions = (id, list) => {
    if (id === latestQuery) {
      items.value = list;
      highlight.value = -1;
    }
  };
  addressField.focus = () => input.value.focus();
});

watch(
  () => activeTab.value.url,
  (url) => {
    if (!focused.value) text.value = url;
  },
);

// A tab that becomes active with nothing in it wants the address field; one with a page
// wants the page. The field is made once Go's data is in, with a tab already active, so
// that one counts too.
function follow() {
  close();
  text.value = activeTab.value.url;
  if (activeTab.value.url === '') {
    focusField();
  } else {
    input.value.blur();
    focusPage();
  }
}
watch(() => state.active, follow);
onMounted(follow);

// A site's question about the active tab hangs under the field, and its settings open from
// the field's start (docs/daily-driver.md).
const prompt = computed(() => promptFor(state, state.active));
// The site panel, which the site control opens over the page; it closes with another tab.
const panel = ref(false);
watch(() => state.active, () => (panel.value = false));
const site = computed(() => openSite(state, activeTab.value));
// Dangerous or Not secure: the site control's warning, and the start of its name.
const marked = computed(() => siteMark(activeTab.value));
</script>

<template>
  <form class="field" :class="{ focused, open }" @submit.prevent="go()">
    <button
      v-if="hasSite(activeTab)"
      type="button"
      class="site"
      :class="{ insecure: marked }"
      :aria-label="marked ? `${marked} · Site settings` : 'Site settings'"
      :title="activeTab.danger ? 'Dangerous · Site settings' : activeTab.connectionNote || 'Site settings'"
      @mousedown.prevent
      :aria-expanded="panel"
      @click="panel = !panel"
    >
      <svg v-if="marked" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M8 2.25 14.25 13.25H1.75z" />
        <path d="M8 6.5v3M8 11.5v.01" />
      </svg>
      <svg v-else width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true">
        <path d="M2.5 5h5M11.5 5h2M2.5 11h2M8.5 11h5" />
        <circle cx="9.5" cy="5" r="1.75" />
        <circle cx="6.5" cy="11" r="1.75" />
      </svg>
    </button>
    <input
      ref="input"
      v-model="text"
      aria-label="Address"
      placeholder="Search or enter address"
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
    <!-- The popups Links to other sites closed on this page: a press opens the site panel. -->
    <button
      v-if="activeTab.popupsBlocked && !focused"
      type="button"
      class="popups"
      :aria-label="popupsWord(activeTab.popupsBlocked)"
      :title="popupsWord(activeTab.popupsBlocked)"
      @mousedown.prevent
      @click="panel = true"
    >
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <rect x="2" y="3.5" width="9" height="8" rx="1.5" />
        <path d="M5 1.75h7.75a1.5 1.5 0 0 1 1.5 1.5V9M2.5 13.5l11-11" />
      </svg>
      <span>{{ activeTab.popupsBlocked }}</span>
    </button>
    <span v-if="workspace" class="workspace">
      <span class="dot" :style="{ background: paint(workspace.color) }"></span>
      {{ workspace.name }}
    </span>
    <ul v-if="open" id="suggestions" v-shell class="list" role="listbox" @mousedown.prevent>
      <li
        v-for="(item, i) in items"
        :id="`suggestion-${i}`"
        :key="item.url"
        role="option"
        :aria-selected="i === highlight"
        :class="{ active: i === highlight }"
        @mouseenter="highlight = i"
        @click="go(item.url)"
      >
        <span class="title">{{ item.title || item.url }}</span>
        <span v-if="item.title" class="dash" aria-hidden="true"></span>
        <span v-if="item.title" class="url">{{ item.url }}</span>
      </li>
    </ul>
    <PermissionPrompt v-if="prompt && !open && !panel" :key="prompt.id" v-shell class="prompt" :prompt="prompt" />
    <SitePanel v-if="panel && !open && hasSite(activeTab)" v-shell class="prompt" :tab="activeTab" :site="site" @close="panel = false" />
  </form>
</template>

<style scoped>
/* A rounded field with a hairline. Focused, it grows past its column by 4px on each side
   and 4px in height, without moving what is beside it, and shows the focus ring. With
   suggestions it floats, and the panel continues it downwards. */
.field {
  position: relative;
  display: flex;
  align-items: center;
  min-width: 0;
  height: 32px;
  margin: 0;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  box-sizing: border-box;
}

.field.focused {
  height: 36px;
  margin: 0 -4px;
  box-shadow: var(--focus-ring);
}

.field.open {
  border-radius: 8px 8px 0 0;
  box-shadow: var(--shadow-float);
}

/* The site's settings, a quiet icon button at the field's start. */
.site {
  flex: none;
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  margin-left: 4px;
  margin-right: -8px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--icon);
  cursor: pointer;
}

.site.insecure {
  color: var(--danger);
}

.site:hover {
  background: var(--accent-tint);
}

.site:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.popups {
  flex: none;
  display: flex;
  align-items: center;
  gap: 3px;
  height: 22px;
  margin-right: 4px;
  padding: 0 6px;
  border: 0;
  border-radius: 4px;
  background: var(--accent-tint);
  color: var(--icon);
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.prompt {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 10;
}

input {
  flex: 1;
  min-width: 0;
  height: 100%;
  padding: 0 12px;
  border: 0;
  border-radius: 8px;
  outline: none;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: 13px;
}

.field.focused input {
  padding: 0 16px;
}

input::placeholder {
  color: var(--muted);
  opacity: 1;
}

/* The workspace the active tab is in: its colour and its name, at the field's right. */
.workspace {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: none;
  max-width: 40%;
  margin-right: 12px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dot {
  flex: none;
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

/* Under the field, as wide as it, one surface with it. */
.list {
  position: absolute;
  top: 100%;
  left: -1px;
  right: -1px;
  z-index: 10;
  margin: 0;
  padding: 4px;
  list-style: none;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 0 0 8px 8px;
  box-shadow: var(--shadow-float);
}

li {
  display: flex;
  align-items: center;
  height: 32px;
  padding: 0 12px;
  border-radius: 4px;
  font-size: 13px;
  cursor: pointer;
}

li.active {
  background: var(--accent-tint);
}

.title,
.url {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.title {
  flex: none;
  max-width: 60%;
}

.url {
  flex: 1;
  min-width: 0;
  color: var(--muted);
}

.dash {
  flex: none;
  width: 5px;
  height: 1px;
  margin: 0 8px;
  background: var(--muted);
}
</style>
