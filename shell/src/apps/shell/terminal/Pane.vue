<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { isNewTab, keyLabel, send, state, tabLabel } from '@/sdk/vue.js';
import { close, shown, smallest, split, zoom, zoomed } from './panes.js';
import FindBar from '@/ui/FindBar.vue';
import PageDialog from '@/ui/PageDialog.vue';
import PageNotice from '@/ui/PageNotice.vue';
import PermissionPrompt from '@/ui/PermissionPrompt.vue';
import { promptFor } from '@/sdk/sites.js';

// One pane: a title bar over the place its tab's page shows. A press on the bar makes the
// pane active.
const props = defineProps({ id: { type: Number, required: true } });

const tab = computed(() => state.tabs.find((t) => t.id === props.id));
// A site's question about this pane's page, drawn while the pane is the active one.
const asking = computed(() => promptFor(state, props.id));
const active = computed(() => props.id === state.active);
const host = computed(() => (tab.value.title && tab.value.url ? new URL(tab.value.url).host : ''));
const status = computed(() => {
  const t = tab.value;
  if (t.blocked) return 'Blocked';
  if (t.crashed) return 'Crashed';
  if (t.error) return 'Failed';
  return '';
});

// The pane's size, so a split that would leave a pane under its smallest is disabled.
const box = ref(null);
const size = ref({ w: Infinity, h: Infinity });
let watcher;
onMounted(() => {
  watcher = new ResizeObserver(([e]) => (size.value = { w: e.contentRect.width, h: e.contentRect.height }));
  watcher.observe(box.value);
});
onBeforeUnmount(() => watcher.disconnect());
const roomRight = computed(() => size.value.w >= 2 * smallest.row + 1);
const roomDown = computed(() => size.value.h >= 2 * smallest.column + 1);

function press() {
  if (!active.value) send({ t: 'activateTab', id: props.id });
}

// A control's tooltip: its name, and its key when it has one.
function hint(name, action) {
  const key = keyLabel(action);
  return key ? `${name} (${key})` : name;
}
</script>

<template>
  <div ref="box" class="pane">
    <div v-shell class="title" :class="{ active }" :data-pane="id" @pointerdown="press">
      <span class="label">{{ tabLabel(tab) }}</span>
      <span v-if="host" class="host">{{ host }}</span>
      <span v-if="status" class="status">{{ status }}</span>
      <span class="gap"></span>
      <button class="control" type="button" aria-label="Split right" :disabled="!roomRight" :title="hint('Split right', 'terminal.splitRight')" @pointerdown.stop @click="split(id, 'row')">
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2"><rect x=".6" y=".6" width="10.8" height="10.8"/><path d="M6 .6v10.8"/></svg>
      </button>
      <button class="control" type="button" aria-label="Split down" :disabled="!roomDown" :title="hint('Split down', 'terminal.splitDown')" @pointerdown.stop @click="split(id, 'column')">
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2"><rect x=".6" y=".6" width="10.8" height="10.8"/><path d="M.6 6h10.8"/></svg>
      </button>
      <button v-if="shown?.parts" class="control" type="button" aria-label="Zoom" :aria-pressed="zoomed === id" :class="{ on: zoomed === id }" :title="hint(zoomed === id ? 'Unzoom' : 'Zoom', 'terminal.zoom')" @pointerdown.stop @click="press(); zoom(id)">
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2"><path d="M1 4.5V1h3.5M7.5 1H11v3.5M11 7.5V11H7.5M4.5 11H1V7.5"/></svg>
      </button>
      <button class="control" type="button" :aria-label="`Close ${tabLabel(tab)}`" title="Close" @pointerdown.stop @click="close(id)">
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"><path d="M9.5 2.5l-7 7M2.5 2.5l7 7"/></svg>
      </button>
      <span class="progress" :class="{ on: tab.loading }" :style="{ width: (tab.progress || 0) * 100 + '%' }"></span>
    </div>
    <!-- The active pane's page carries its dialog and its find bar, as Standard's does. -->
    <div class="body">
      <div v-page="id" class="page"></div>
      <!-- A new tab loads nothing, so no page shows here: a backdrop in the page's colour. A
           press on it makes its pane active, as a press in a page does. -->
      <div v-if="isNewTab(tab)" v-shell class="blank" @pointerdown="press"></div>
      <!-- Go's notice over a page with nothing to show, on any pane; it fills the page. -->
      <PageNotice v-if="tab.notice" v-shell :tab="tab" />
      <template v-if="active">
        <PermissionPrompt v-if="asking" :key="asking.id" v-shell class="asking" :prompt="asking" />
        <PageDialog />
        <FindBar />
      </template>
    </div>
  </div>
</template>

<style scoped>
.pane {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
}

/* Quiet on the bar; the active pane's is in the accent. */
.title {
  position: relative;
  display: flex;
  align-items: center;
  flex: none;
  gap: 8px;
  height: 20px;
  padding: 0 4px 0 8px;
  box-sizing: border-box;
  background: var(--bar);
  color: var(--muted);
  cursor: default;
  user-select: none;
}

.title.active {
  background: var(--accent);
  color: var(--on-accent);
}

.label,
.host {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.label {
  min-width: 0;
}

.host {
  flex: none;
  max-width: 40%;
}

.status {
  flex: none;
  padding: 0 4px;
  background: var(--danger);
  color: var(--on-danger);
}

.gap {
  flex: 1;
}

.control {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 16px;
  height: 16px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: none;
  color: inherit;
  cursor: pointer;
  opacity: 0.8;
}

.control:disabled {
  opacity: 0.5;
  cursor: default;
}

.control:hover:not(:disabled) {
  background: var(--hover);
  opacity: 1;
}

/* On the accent bar the toolbar's hover would wash the icon out, so it darkens instead,
   and the focus ring, in the accent itself, is drawn in the bar's ink. */
.title.active .control:hover:not(:disabled) {
  background: var(--press);
}

.control.on {
  background: var(--press);
  opacity: 1;
}

.control:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
  opacity: 1;
}

.title.active .control:focus-visible {
  box-shadow: inset 0 0 0 1px var(--on-accent);
}

/* The page's load progress, along the foot of the bar. */
.progress {
  position: absolute;
  left: 0;
  bottom: 0;
  height: 2px;
  background: var(--accent);
  opacity: 0;
  transition: width 0.15s ease-out;
}

.title.active .progress {
  background: var(--on-accent);
}

.progress.on {
  opacity: 1;
}

.asking {
  position: absolute;
  top: 8px;
  left: 8px;
  z-index: 6;
}

/* The page's area is a layer of its own: what lies over the page (a notice, the find bar, a
   page's dialog) stays under the shell's bars and the menus and panels they open
   (design-system.md, Layers). */
.body {
  position: relative;
  isolation: isolate;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.page {
  flex: 1;
  min-height: 0;
}

.blank {
  position: absolute;
  inset: 0;
  background: Canvas;
}
</style>
