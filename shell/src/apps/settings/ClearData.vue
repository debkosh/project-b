<script setup>
import { computed, ref } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import Row from './Row.vue';
import { send, state } from './state.js';

// Clear browsing data: a dialog asks from when and what goes, as every browser's does, and
// Go clears it (docs/settings.md). Cookies and site data, and apart from them the cache, go
// in every workspace: the engine erases them so. Once Go says what went, the row says so.
const ranges = [
  { id: 'hour', label: 'The last hour' },
  { id: 'day', label: 'The last day' },
  { id: 'week', label: 'The last week' },
  { id: 'fourWeeks', label: 'The last four weeks' },
  { id: 'all', label: 'All time' },
];
const kinds = [
  { id: 'history', label: 'History', hint: 'Every page visited, in every workspace' },
  { id: 'siteData', label: 'Cookies and site data', hint: 'Signs you out of most sites, in every workspace' },
  { id: 'cache', label: 'Cached images and files', hint: 'Sites load more slowly the next time' },
  { id: 'downloads', label: 'The downloads list', hint: 'The files stay where they are' },
];

const asking = ref(false);
const since = ref('all');
const ticked = ref([]);
function ask() {
  since.value = 'all';
  ticked.value = ['history', 'siteData', 'cache'];
  state.clearCounts = null;
  send({ t: 'clearDataCounts' });
  asking.value = true;
}
function clear() {
  send({ t: 'clearData', since: since.value, ...Object.fromEntries(kinds.map((k) => [k.id, ticked.value.includes(k.id)])) });
  asking.value = false;
}

// History's line says how many pages the range takes, once Go has said.
function hintOf(k) {
  const n = state.clearCounts?.[since.value];
  if (k.id !== 'history' || n == null) return k.hint;
  return n === 1 ? '1 page' : `${n.toLocaleString()} pages`;
}

const hint = computed(() => {
  const c = state.cleared;
  const went = c && kinds.filter((k) => c[k.id]).map((k) => k.label.toLowerCase());
  if (!went?.length) return 'History, cookies and site data, cached files, the downloads list';
  const from = ranges.find((r) => r.id === c.since);
  return `Cleared ${went.join(', ')}` + (from && from.id !== 'all' ? `, from ${from.label.toLowerCase()}` : '');
});
</script>

<template>
  <Row label="Clear browsing data…" :hint="hint">
    <button type="button" class="text-button" @click="ask">Clear…</button>
    <Dialog v-if="asking" title="Clear browsing data" @close="asking = false">
      <div class="ui-dialog-part from">
        <label for="clear-since">From</label>
        <select id="clear-since" v-model="since">
          <option v-for="r in ranges" :key="r.id" :value="r.id">{{ r.label }}</option>
        </select>
      </div>
      <div class="ui-dialog-part kinds">
        <label v-for="k in kinds" :key="k.id" class="kind">
          <input v-model="ticked" type="checkbox" :value="k.id" />
          <span>
            {{ k.label }}
            <span class="secondary">{{ hintOf(k) }}</span>
          </span>
        </label>
      </div>
      <template #buttons>
        <button type="button" class="text-button" @click="asking = false">Cancel</button>
        <button type="button" class="text-button danger" :disabled="!ticked.length" @click="clear">Clear</button>
      </template>
    </Dialog>
  </Row>
</template>

<style scoped>
.from {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
}

.kinds {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* Each choice a checkbox before its words, not a field's label over it. */
.kinds .kind {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin: 0;
  color: var(--text);
  font-size: 13px;
}

.kinds .kind input {
  margin: 2px 0 0;
  accent-color: var(--accent);
}
</style>
