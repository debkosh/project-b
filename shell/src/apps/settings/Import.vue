<script setup>
import { computed, onMounted, ref, watch } from 'vue';
import AllowAccess from '@/ui/AllowAccess.vue';
import BrowserIcon from '@/ui/BrowserIcon.vue';
import Dialog from '@/ui/Dialog.vue';
import ImportChoices from '@/ui/ImportChoices.vue';
import { broughtLines, busyLine, choicesOf, holds, lastLine } from '@/ui/imports.js';
import { send, state } from './state.js';

// Import (docs/onboarding.md): each other browser's profile on this Mac, with its own logo,
// what it holds or what it last brought over, and Import…, which asks what to bring in a
// dialog: the welcome's choices, any time. Go looks again each time the section opens.
// While the Extensions flag is on, a profile's extensions can come too, unticked at first;
// a row says how far their installs are. Safari's row is locked until the app has Full
// Disk Access: its Allow access… opens
// System Settings, and its dialog shows how to give it, then the choices once Go can read
// Safari's files.
onMounted(() => send({ t: 'findBrowsers' }));

// The profile whose dialog is open, as the list has it now, and what is ticked in it: all
// it holds but its extensions, at first.
const askingKey = ref(null);
const asking = computed(() => state.browsers?.find((b) => b.key === askingKey.value) || null);
const what = ref([]);
function ask(p) {
  askingKey.value = p.key;
  what.value = ticked(p);
  if (p.locked) send({ t: 'openAccess' });
}
watch(
  () => asking.value?.locked,
  (locked, was) => {
    if (was && !locked) what.value = ticked(asking.value);
  },
);
function bring() {
  send({ t: 'import', key: askingKey.value, what: what.value });
  askingKey.value = null;
}

const choices = (p) => choicesOf(p, { extensions: true });
const ticked = (p) => choices(p).map((c) => c.id).filter((id) => id !== 'extensions');
const line = (p) => (p.last ? lastLine(p.last) : holds(p, { extensions: true }));
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
        <BrowserIcon :browser="p.browser" />
        <span class="label">
          <span class="name">{{ p.title }}</span>
          <span class="secondary">{{ state.importing.includes(p.key) ? busyLine(state.installing[p.key]) : line(p) }}</span>
          <ul v-if="state.imported[p.key]" class="brought" :aria-label="`Brought over from ${p.title}`">
            <li v-for="(l, i) in broughtLines(state.imported[p.key])" :key="i" :class="{ problem: l.problem, left: l.left }">
              {{ l.text }}<span v-if="l.note" class="note"> · {{ l.note }}</span>
            </li>
          </ul>
        </span>
        <button v-if="p.locked" type="button" class="text-button" :aria-label="`Allow access to ${p.title}`" @click="ask(p)">Allow access…</button>
        <button
          v-else
          type="button"
          class="text-button"
          :disabled="!choices(p).length || state.importing.includes(p.key)"
          :aria-label="`Import from ${p.title}`"
          @click="ask(p)"
        >Import…</button>
      </div>
    </div>

    <Dialog v-if="asking?.locked" :title="`Allow ${asking.title}'s things`" @close="askingKey = null">
      <p class="ui-dialog-part">macOS keeps {{ asking.title }}'s files behind Full Disk Access.</p>
      <AllowAccess class="ui-dialog-part" @open="send({ t: 'openAccess' })" />
      <template #buttons>
        <button type="button" class="text-button quiet" @click="askingKey = null">Not now</button>
      </template>
    </Dialog>
    <Dialog v-else-if="asking" :title="`Import from ${asking.title}`" @close="askingKey = null">
      <p class="ui-dialog-part">Bookmarks go into a folder of their own. Nothing leaves this Mac.</p>
      <ImportChoices v-model="what" class="ui-dialog-part" :profile="asking" extensions />
      <template #buttons>
        <button type="button" class="text-button quiet" @click="askingKey = null">Cancel</button>
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

.brought .left {
  color: var(--muted);
}
</style>
