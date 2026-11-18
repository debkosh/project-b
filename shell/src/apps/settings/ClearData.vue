<script setup>
import { computed, ref } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import Row from './Row.vue';
import { send, state } from './state.js';

// Clear browsing data: a dialog asks what goes, as every browser's does, and Go clears it,
// all of it, from the start (docs/settings.md). Cookies, site data and the cache go
// together, in every workspace: the engine erases them so. Once Go says what went, the row
// says so.
const kinds = [
  { id: 'history', label: 'History', hint: 'Every page visited, in every workspace' },
  { id: 'siteData', label: 'Cookies, site data and cached files', hint: 'Signs you out of most sites, in every workspace' },
  { id: 'downloads', label: 'The downloads list', hint: 'The files stay where they are' },
];

const asking = ref(false);
const ticked = ref([]);
function ask() {
  ticked.value = ['history', 'siteData'];
  asking.value = true;
}
function clear() {
  send({ t: 'clearData', ...Object.fromEntries(kinds.map((k) => [k.id, ticked.value.includes(k.id)])) });
  asking.value = false;
}

const hint = computed(() => {
  const c = state.cleared;
  const went = c && kinds.filter((k) => c[k.id]).map((k) => k.label.toLowerCase());
  return went?.length ? `Cleared ${went.join(', ')}` : 'History, cookies and site data, cached files, the downloads list';
});
</script>

<template>
  <Row label="Clear browsing data…" :hint="hint">
    <button type="button" class="text-button" @click="ask">Clear…</button>
    <Dialog v-if="asking" title="Clear browsing data" @close="asking = false">
      <p class="ui-dialog-part">Everything ticked goes, from the first day.</p>
      <div class="ui-dialog-part kinds">
        <label v-for="k in kinds" :key="k.id" class="kind">
          <input v-model="ticked" type="checkbox" :value="k.id" />
          <span>
            {{ k.label }}
            <span class="secondary">{{ k.hint }}</span>
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
