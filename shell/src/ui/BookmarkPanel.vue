<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue';
import { folders, removeBookmark, saveBookmark } from '@/sdk/bookmarks.js';
import { pushEscape } from './escape.js';

// The star's panel (docs/bookmark-folders.md): what the star and Cmd+D show once Go has
// bookmarked the page into the folder last saved in. Its name and its folder can change:
// Enter or Done saves them, and the next bookmark goes into that folder too. Escape or a
// press outside closes it with the bookmark kept as it was; Remove takes it away. The root
// floats over the page: the caller marks it the shell's, places it, and closes it on close.
const props = defineProps({ bookmark: Object, list: Array, added: Boolean });
const emit = defineEmits(['close']);

const title = ref(props.bookmark.title);
const parent = ref(props.bookmark.parent);
const places = computed(() => folders(props.list));
const root = ref(null);
const name = ref(null);

function save() {
  saveBookmark(props.bookmark.id, title.value.trim(), parent.value);
  emit('close');
}

// Enter saves from anywhere in the panel, a folder chosen among them, but on a button,
// which it presses.
function enter(e) {
  if (e.target.closest('button')) return;
  e.preventDefault();
  save();
}

function remove() {
  removeBookmark(props.bookmark.id);
  emit('close');
}

function outside(e) {
  if (!root.value?.contains(e.target) && !e.target.closest?.('[data-bookmark-star]')) emit('close');
}

let pop = null;
onMounted(async () => {
  pop = pushEscape(() => emit('close'));
  document.addEventListener('pointerdown', outside, true);
  await nextTick();
  name.value?.focus();
  name.value?.select();
});
onUnmounted(() => {
  pop();
  document.removeEventListener('pointerdown', outside, true);
});
</script>

<template>
  <form ref="root" class="ui-bookmark-panel" role="dialog" :aria-label="added ? 'Bookmarked' : 'Edit bookmark'" @submit.prevent.stop="save" @keydown.enter="enter">
    <h2 class="ui-bookmark-head">{{ added ? 'Bookmarked' : 'Edit bookmark' }}</h2>
    <label class="ui-bookmark-field">
      <span>Name</span>
      <input ref="name" v-model="title" class="text-field" aria-label="Name" />
    </label>
    <fieldset v-if="places.length > 1" class="ui-bookmark-field">
      <legend>Folder</legend>
      <div class="ui-bookmark-folders">
        <label v-for="f in places" :key="f.id" class="ui-bookmark-folder" :class="{ on: parent === f.id }" :style="{ paddingLeft: `${8 + f.depth * 16}px` }">
          <input v-model="parent" type="radio" name="ui-bookmark-folder" :value="f.id" />
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round" aria-hidden="true">
            <path d="M2 4.5a1 1 0 0 1 1-1h3l1.5 1.5H13a1 1 0 0 1 1 1V12a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1z" />
          </svg>
          <span>{{ f.title }}</span>
        </label>
      </div>
    </fieldset>
    <div class="ui-bookmark-buttons">
      <button type="button" class="text-button quiet" @click="remove">Remove</button>
      <button type="submit" class="text-button primary">Done</button>
    </div>
  </form>
</template>

<style>
.ui-bookmark-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 320px;
  max-width: calc(100vw - 16px);
  box-sizing: border-box;
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  box-shadow: var(--shadow-float);
  color: var(--text);
}

.ui-bookmark-head {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.ui-bookmark-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 0;
  padding: 0;
  border: 0;
  min-width: 0;
}

.ui-bookmark-field > span,
.ui-bookmark-field > legend {
  padding: 0;
  margin-bottom: 4px;
  font-size: 12px;
  color: var(--muted);
}

.ui-bookmark-folders {
  max-height: 168px;
  overflow-y: auto;
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 4px;
}

.ui-bookmark-folder {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  padding-right: 8px;
  border-radius: 6px;
  font-size: 13px;
  color: var(--text);
  cursor: default;
}

.ui-bookmark-folder svg {
  flex: none;
  color: var(--icon);
}

.ui-bookmark-folder span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ui-bookmark-folder:hover {
  background: var(--accent-tint);
}

.ui-bookmark-folder.on {
  background: var(--accent-soft);
  font-weight: 600;
}

/* The radio is what the keys move and what a screen reader names; the row shows which. */
.ui-bookmark-folder input {
  position: absolute;
  opacity: 0;
  width: 1px;
  height: 1px;
  margin: 0;
}

.ui-bookmark-folder:has(input:focus-visible) {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}

.ui-bookmark-buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
