<script setup>
import { onMounted, onUnmounted, ref } from 'vue';
import LineIcon from './LineIcon.vue';
import { pushEscape } from './escape.js';

// A sheet of actions on a phone (docs/design-system.md, "On a phone"): a one-line head, the
// name and, under it in --muted, its address; then the most used actions, five at most, as a
// row of 64px tiles, a 20px icon over an 11.5px label; then the rest as 48px rows. It is as
// tall as what it holds, over the page dimmed behind it. Mount it with v-if to open it; a
// tap on the page behind, Escape and the back key emit close. Each action closes it as it
// runs. An action is {icon, label, run}; a row may be danger.
const props = defineProps({ name: String, detail: String, tiles: { type: Array, default: () => [] }, rows: { type: Array, default: () => [] } });
const emit = defineEmits(['close']);
const sheet = ref(null);
let pop = null;
let before = null;
onMounted(() => {
  before = document.activeElement;
  pop = pushEscape(() => emit('close'));
  sheet.value.querySelector('button')?.focus();
});
onUnmounted(() => {
  pop();
  if (before && before.isConnected) before.focus();
});

function run(action) {
  emit('close');
  action.run();
}
</script>

<template>
  <div class="ui-sheet-backdrop" role="presentation" @click="emit('close')"></div>
  <div ref="sheet" class="ui-sheet" role="dialog" :aria-label="props.name">
    <div class="ui-sheet-head">
      <span class="ui-sheet-name">{{ props.name }}</span>
      <span v-if="props.detail" class="ui-sheet-detail">{{ props.detail }}</span>
    </div>
    <div v-if="props.tiles.length" class="ui-sheet-tiles">
      <button v-for="a in props.tiles" :key="a.label" type="button" class="ui-sheet-tile" @click="run(a)">
        <LineIcon :name="a.icon" />
        <span>{{ a.label }}</span>
      </button>
    </div>
    <button v-for="a in props.rows" :key="a.label" type="button" class="ui-sheet-row" :class="{ danger: a.danger }" @click="run(a)">
      <LineIcon :name="a.icon" />
      <span>{{ a.label }}</span>
    </button>
  </div>
</template>

<style>
.ui-sheet-backdrop {
  position: fixed;
  inset: 0;
  z-index: 20;
  background: color-mix(in srgb, var(--bar) 70%, transparent);
}

.ui-sheet {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 21;
  max-height: 85%;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 4px 0 8px;
  border-radius: 16px 16px 0 0;
  background: var(--surface);
  color: var(--text);
  box-shadow: var(--shadow-float);
  animation: ui-sheet-rise var(--motion) var(--ease);
}

.ui-sheet-head {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 12px 20px 8px;
  min-width: 0;
}

.ui-sheet-name,
.ui-sheet-detail {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.ui-sheet-name {
  font-size: 14px;
  font-weight: 600;
}

.ui-sheet-detail {
  color: var(--muted);
  font-size: 12px;
}

.ui-sheet-tiles {
  display: flex;
  padding: 4px 8px;
}

.ui-sheet-tile {
  flex: 1;
  min-width: 0;
  height: 64px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--icon);
  font: inherit;
  font-size: 11.5px;
}

.ui-sheet-tile span {
  max-width: 100%;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  color: var(--text);
}

.ui-sheet-row {
  display: flex;
  align-items: center;
  gap: 16px;
  width: 100%;
  height: 48px;
  padding: 0 20px;
  border: 0;
  background: none;
  color: var(--icon);
  font: inherit;
  font-size: 14px;
  text-align: left;
}

.ui-sheet-row span {
  color: var(--text);
}

.ui-sheet-row.danger,
.ui-sheet-row.danger span {
  color: var(--danger);
}

.ui-sheet-tile:active,
.ui-sheet-row:active {
  background: var(--accent-tint);
}

.ui-sheet-tile:focus-visible,
.ui-sheet-row:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

@keyframes ui-sheet-rise {
  from { transform: translateY(100%); }
}
</style>
