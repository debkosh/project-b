<script setup>
import { onMounted, ref } from 'vue';
import BrowserIcon from '@/ui/BrowserIcon.vue';
import Dialog from '@/ui/Dialog.vue';
import ImportChoices from '@/ui/ImportChoices.vue';
import { broughtLines, choicesOf, holds, lastLine } from '@/ui/imports.js';
import { send, state } from './state.js';

// Import (docs/onboarding.md): each other browser's profile on this Mac, with its own logo,
// what it holds or what it last brought over, and Import…, which asks what to bring in a
// dialog: the welcome's choices, any time. Go looks again each time the section opens.
onMounted(() => send({ t: 'findBrowsers' }));

// The profile whose dialog is open, and what is ticked in it: all it holds, at first.
const asking = ref(null);
const what = ref([]);
function ask(p) {
  asking.value = p;
  what.value = choicesOf(p).map((c) => c.id);
}
function bring() {
  send({ t: 'import', key: asking.value.key, what: what.value });
  asking.value = null;
}

const line = (p) => (p.last ? lastLine(p.last, state.searchEngines) : holds(p));
</script>

<template>
  <section aria-label="Import">
    <div v-if="state.browsers === null" class="card">
      <div class="row"><span class="label secondary">Looking for other browsers…</span></div>
    </div>
    <div v-else-if="!state.browsers.length" class="card">
      <div class="row"><span class="label secondary">No other browser's bookmarks or history were found on this Mac.</span></div>
    </div>
    <div v-else class="card">
      <div v-for="p in state.browsers" :key="p.key" class="row browser">
        <BrowserIcon :icon="p.icon" :title="p.title" />
        <span class="label">
          <span class="name">{{ p.title }}</span>
          <span class="secondary">{{ state.importing.includes(p.key) ? 'Bringing over…' : line(p) }}</span>
          <ul v-if="state.imported[p.key]" class="brought" :aria-label="`Brought over from ${p.title}`">
            <li v-for="(l, i) in broughtLines(state.imported[p.key], state.searchEngines)" :key="i" :class="{ problem: l.problem }">
              {{ l.text }}<span v-if="l.note" class="note"> · {{ l.note }}</span>
            </li>
          </ul>
        </span>
        <button
          type="button"
          class="text-button"
          :disabled="!choicesOf(p).length || state.importing.includes(p.key)"
          :aria-label="`Import from ${p.title}`"
          @click="ask(p)"
        >Import…</button>
      </div>
    </div>

    <Dialog v-if="asking" :title="`Import from ${asking.title}`" @close="asking = null">
      <p class="ui-dialog-part">Bookmarks go into a folder of their own. Nothing leaves this Mac.</p>
      <ImportChoices v-model="what" class="ui-dialog-part" :profile="asking" />
      <template #buttons>
        <button type="button" class="text-button quiet" @click="asking = null">Cancel</button>
        <button type="button" class="text-button primary" :disabled="!what.length" @click="bring">Import</button>
      </template>
    </Dialog>
  </section>
</template>

<style scoped>
.browser {
  gap: 12px;
}

.label {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.brought {
  margin: 6px 0 0;
  padding: 0;
  list-style: none;
  font-size: 12px;
  color: var(--text);
}

.brought .note {
  color: var(--muted);
}

.brought .problem {
  color: var(--danger);
}
</style>
