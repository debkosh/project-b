<script setup>
import { onMounted, onUnmounted, ref } from 'vue';
import { pushEscape } from './escape.js';

// A modal dialog, the one pattern for every question: 360px, a 15px title, the body, and
// the buttons at the bottom right, which the caller puts in the buttons slot. A title with
// a word set apart, a host in bold, is drawn in the title slot, title still naming the
// dialog. Mount it with v-if to open it. Escape and the backdrop's own cancel emit close; when it goes,
// focus goes back to what had it before it opened.
defineProps({ title: String });
const emit = defineEmits(['close']);

const dialog = ref(null);
let before = null;
let pop = null;

onMounted(() => {
  before = document.activeElement;
  dialog.value.showModal();
  pop = pushEscape(() => emit('close'));
});
onUnmounted(() => {
  pop();
  if (before && before.isConnected) before.focus();
});
</script>

<template>
  <dialog ref="dialog" class="ui-dialog" :aria-label="title" @cancel.prevent="emit('close')">
    <h2 class="ui-dialog-title"><slot name="title">{{ title }}</slot></h2>
    <slot />
    <div class="ui-dialog-buttons"><slot name="buttons" /></div>
  </dialog>
</template>

<style>
.ui-dialog {
  width: 360px;
  max-width: calc(100% - 32px);
  padding: 20px;
  box-sizing: border-box;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
  box-shadow: var(--shadow-float);
}

/* The page behind fades toward the bar's colour. */
.ui-dialog::backdrop {
  background: color-mix(in srgb, var(--bar) 70%, transparent);
}

.ui-dialog-title {
  margin: 0 0 16px;
  color: var(--text);
  font-size: 15px;
  font-weight: 600;
  line-height: 1.35;
}

.ui-dialog p {
  margin: 0;
  color: var(--muted);
}

/* A field in a dialog: its label above it, 12px and muted. A swatch, a choice to bring
   over, or an item picked with a checkbox, that is a label keeps its own box. */
.ui-dialog label:not(.ui-swatch, .ui-import-choice, .ui-dialog-pick) {
  display: block;
  margin: 0 0 4px;
  color: var(--muted);
  font-size: 12px;
}

/* An item picked from a list: its checkbox, then its name. */
.ui-dialog-pick {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  padding: 6px 0;
}

.ui-dialog .ui-dialog-part + .ui-dialog-part {
  margin-top: 16px;
}

.ui-dialog-buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 20px;
}
</style>
