<script setup>
import { computed, inject, ref, watch } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import RowMenu from '@/ui/RowMenu.vue';
import Segmented from '@/ui/Segmented.vue';
import { send, state } from './state.js';

// Sparks (docs/site-features.md): the person's scripts for pages, which Go keeps, parses and
// matches. Each row has its switch and a menu: Edit…, Export and Delete; a tap on the row
// edits it too. New opens an empty editor. Import… takes .user.js files and zips of them in
// one dialog: Go reads them, the dialog asks about each spark the profile already has
// (replace it, keep both under a new name, or ignore it), then says what was done. Export…
// is a dialog that picks the sparks for a zip. A save that Go refuses says why under the
// editor.

// Where the page's own heading names the list, its own gives way, and its buttons go on the
// page's heading line when the page has a place for them.
const titled = inject('settingsTitled', false);
const actionsTo = inject('settingsListActions', null);

const template = `// ==UserScript==
// @name        My spark
// @match       https://example.com/*
// @run-at      document-end
// ==/UserScript==

`;

// The editor: null when closed; else the spark it edits ('' for a new one) and its text.
const editing = ref(null);
const text = ref('');
const saving = ref(false);

function startNew(source = template) {
  editing.value = '';
  text.value = source;
  state.sparkProblem = null;
}

// Edit and Export ask Go for the file as it is kept.
const wanted = ref(null);
function ask(id, then) {
  wanted.value = { id, then };
  send({ t: 'sparkSource', id });
}
watch(
  () => state.sparkSource,
  (s) => {
    if (!s || wanted.value?.id !== s.id) return;
    const then = wanted.value.then;
    wanted.value = null;
    if (then === 'edit') {
      editing.value = s.id;
      text.value = s.source;
      state.sparkProblem = null;
    } else {
      download(s.fileName, new Blob([s.source], { type: 'text/javascript' }));
    }
  },
);

function download(name, blob) {
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
}

function save() {
  saving.value = true;
  state.sparkProblem = null;
  if (editing.value) send({ t: 'sparkUpdate', id: editing.value, source: text.value });
  else send({ t: 'sparkAdd', source: text.value });
}
watch(
  () => state.sparkProblem,
  (p) => {
    if (!saving.value || p === null) return;
    saving.value = false;
    if (p === '') editing.value = null;
  },
);

// The row menus by spark id, so a right-click on a row opens its menu.
const menus = {};
function items(s) {
  return [
    { label: 'Edit…', run: () => ask(s.id, 'edit') },
    { label: 'Export', run: () => ask(s.id, 'export'), disabled: !!s.error },
    { separator: true },
    { label: 'Delete', danger: true, run: () => send({ t: 'sparkRemove', id: s.id }) },
  ];
}
// A tap on a row, outside its switch and its menu, edits it.
function rowClick(s, e) {
  if (!e.target.closest('input, button, .ui-row-menu')) ask(s.id, 'edit');
}

// Import. Each file goes to Go as base64; one over Go's 8 MB goes as its size alone, for Go
// to say so. The dialog's step: 'reading', 'clashes', 'done', or null when it is closed.
const picker = ref(null);
const step = ref(null);
const tooLarge = 8 << 20;
function base64(buffer) {
  const bytes = new Uint8Array(buffer);
  let s = '';
  for (let i = 0; i < bytes.length; i += 0x8000) s += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  return btoa(s);
}
async function imported(e) {
  const files = [...e.target.files];
  e.target.value = '';
  if (!files.length) return;
  state.sparkClashes = state.sparkImported = null;
  step.value = 'reading';
  const sent = await Promise.all(
    files.map(async (f) => (f.size > tooLarge ? { name: f.name, size: f.size } : { name: f.name, data: base64(await f.arrayBuffer()) })),
  );
  send({ t: 'sparkImport', files: sent });
}

// Each clash's choice, by its index; Replace until the person says otherwise.
const choices = ref({});
const options = [
  { value: 'replace', label: 'Replace' },
  { value: 'rename', label: 'Rename' },
  { value: 'ignore', label: 'Ignore' },
];
watch(
  () => state.sparkClashes,
  (c) => {
    if (!c) return;
    choices.value = Object.fromEntries(c.map((x) => [x.index, 'replace']));
    step.value = 'clashes';
  },
);
function keep() {
  step.value = 'reading';
  send({ t: 'sparkImportKeep', choices: choices.value });
}
// Escape or Cancel before the import is kept leaves everything as it was.
function closeImport() {
  if (step.value !== 'done') send({ t: 'sparkImportCancel' });
  step.value = null;
  state.sparkClashes = null;
}
watch(
  () => state.sparkImported,
  (r) => {
    if (r) step.value = 'done';
  },
);

// Versions say which is newer when both have one.
function versions(c) {
  return c.version && c.keptVersion ? `Version ${c.version}, yours is ${c.keptVersion}` : '';
}

// What the import did, said once in the title and a row for each spark.
const done = computed(() => state.sparkImported);
function few(list) {
  return list.length <= 2 ? list.join(' and ') : `${list.length} sparks`;
}
const outcome = computed(() => {
  const r = done.value;
  if (!r) return '';
  const added = [...r.added, ...r.renamed.map((x) => x.to)];
  const parts = [];
  if (added.length) parts.push(`Added ${few(added)}`);
  if (r.replaced.length) parts.push(`Replaced ${few(r.replaced)}`);
  if (r.ignored.length) parts.push(`Ignored ${few(r.ignored)}`);
  if (r.unchanged.length) parts.push(`${r.unchanged.length} already here`);
  if (r.skipped.length) parts.push(`${r.skipped.length} not imported`);
  return parts.join(' · ') || 'Nothing to import';
});
const rows = computed(() => {
  const r = done.value;
  if (!r) return [];
  // A file shows only where it says more than the spark's name.
  const from = (s) => (s.name && s.file.split(' › ').pop() !== `${s.name}.user.js` ? `${s.file}: ` : '');
  return [
    ...r.added.map((name) => ({ name, note: 'added' })),
    ...r.renamed.map((x) => ({ name: x.to, note: `added beside your ${x.from}` })),
    ...r.replaced.map((name) => ({ name, note: 'replaced yours' })),
    ...r.ignored.map((name) => ({ name, note: 'ignored, yours kept' })),
    ...r.unchanged.map((name) => ({ name, note: 'already here, unchanged' })),
    ...r.skipped.map((s) => ({ name: s.name || s.file, note: from(s) + s.reason, failed: true })),
  ];
});
const importTitle = computed(() => {
  if (step.value === 'clashes') {
    const n = state.sparkClashes.length;
    return n === 1 ? 'You already have this spark' : `You already have ${n} of these sparks`;
  }
  return step.value === 'done' ? outcome.value : 'Importing…';
});

// Export…: every spark ticked to start with; the zip Go makes is downloaded.
const exporting = ref(false);
const chosen = ref([]);
const exportable = computed(() => state.sparks.filter((s) => !s.error));
const all = computed(() => chosen.value.length === exportable.value.length);
const some = computed(() => chosen.value.length > 0 && !all.value);
function startExport() {
  chosen.value = exportable.value.map((s) => s.id);
  exporting.value = true;
}
function tickAll(on) {
  chosen.value = on ? exportable.value.map((s) => s.id) : [];
}
function exportChosen() {
  send({ t: 'sparkExport', ids: chosen.value });
  exporting.value = false;
}
watch(
  () => state.sparkZip,
  (z) => {
    if (!z) return;
    state.sparkZip = null;
    const bytes = Uint8Array.from(atob(z.data), (c) => c.charCodeAt(0));
    download(z.fileName, new Blob([bytes], { type: 'application/zip' }));
  },
);

// New is the one people reach for; Import… and Export… are in the menu beside it.
const more = computed(() => [
  { label: 'Import…', run: () => picker.value.click() },
  { label: 'Export…', run: startExport, disabled: !exportable.value.length },
]);

const full = computed(() => state.sparks.length >= 25);
// Where it runs: its patterns as written, but <all_urls>, which reads as code.
const where = (s) => (s.match || []).map((m) => (m === '<all_urls>' ? 'every site' : m)).join(', ');
</script>

<template>
  <section aria-label="Sparks">
    <!-- Empty, and not shown, when the buttons are on the page's own heading line. -->
    <div class="heading" :class="{ moved: titled && actionsTo }">
      <h2 v-if="!titled">Sparks</h2>
      <Teleport defer :to="actionsTo || 'body'" :disabled="!actionsTo">
        <span class="actions">
          <button type="button" class="text-button" :disabled="full" @click="startNew()">New</button>
          <RowMenu :items="more" label="More for sparks" />
        </span>
      </Teleport>
    </div>
    <input ref="picker" type="file" accept=".js,.zip" multiple hidden aria-label="Sparks to import" @change="imported" />
    <p class="intro">
      Your own scripts for the pages they match, run in the page itself.
      {{ state.sparks.length ? `${state.sparks.length} of 25.` : 'No sparks yet.' }}
    </p>
    <div v-if="state.sparks.length" class="card">
      <div
        v-for="s in state.sparks"
        :key="s.id"
        class="row spark"
        role="group"
        :aria-label="s.name || 'A spark that has a problem'"
        @click="rowClick(s, $event)"
        @contextmenu.prevent="menus[s.id]?.open()"
      >
        <span class="label">
          <span class="name" :title="s.name">{{ s.name || 'A spark that has a problem' }}</span>
          <span class="secondary" :class="{ failed: s.error }">{{ s.error || where(s) }}</span>
        </span>
        <input
          type="checkbox"
          role="switch"
          class="switch"
          :aria-label="`${s.name} on`"
          :checked="s.on"
          :aria-checked="String(s.on)"
          @change="send({ t: 'sparkSetOn', id: s.id, on: $event.target.checked })"
        />
        <RowMenu :ref="(m) => (menus[s.id] = m)" :items="items(s)" :label="`Actions for ${s.name || 'the spark'}`" />
      </div>
    </div>

    <div v-if="editing !== null" class="card editor" role="group" :aria-label="editing ? 'Edit spark' : 'New spark'">
      <textarea v-model="text" aria-label="Spark source" spellcheck="false" rows="16"></textarea>
      <p v-if="state.sparkProblem" class="problem" role="alert">{{ state.sparkProblem }}</p>
      <div class="buttons">
        <button type="button" class="text-button" @click="editing = null">Cancel</button>
        <button type="button" class="text-button primary" :disabled="saving" @click="save">Save</button>
      </div>
    </div>

    <Dialog v-if="step" :title="importTitle" @close="closeImport">
      <template v-if="step === 'clashes'">
        <div v-for="c in state.sparkClashes" :key="c.index" class="clash">
          <span class="clash-name">
            {{ c.name }}
            <span v-if="versions(c)" class="secondary">{{ versions(c) }}</span>
          </span>
          <Segmented v-model="choices[c.index]" :options="options" :label="`For ${c.name}`" />
          <span v-if="choices[c.index] === 'rename'" class="secondary">Adds it as “{{ c.renameTo }}”.</span>
        </div>
      </template>
      <ul v-else-if="step === 'done' && rows.length" class="outcome" aria-label="What was imported">
        <li v-for="(r, i) in rows" :key="i">
          <span class="what">{{ r.name }}</span>
          <span class="secondary" :class="{ failed: r.failed }">{{ r.note }}</span>
        </li>
      </ul>
      <template #buttons>
        <template v-if="step === 'clashes'">
          <button type="button" class="text-button" @click="closeImport">Cancel</button>
          <button type="button" class="text-button primary" @click="keep">Import</button>
        </template>
        <button v-else-if="step === 'done'" type="button" class="text-button primary" @click="closeImport">Done</button>
        <button v-else type="button" class="text-button" @click="closeImport">Cancel</button>
      </template>
    </Dialog>

    <Dialog v-if="exporting" title="Export sparks" @close="exporting = false">
      <label class="ui-dialog-pick all">
        <input type="checkbox" :checked="all" :indeterminate="some" @change="tickAll($event.target.checked)" />
        <span>All</span>
      </label>
      <label v-for="s in exportable" :key="s.id" class="ui-dialog-pick">
        <input v-model="chosen" type="checkbox" :value="s.id" />
        <span class="name">{{ s.name }}</span>
      </label>
      <template #buttons>
        <button type="button" class="text-button" @click="exporting = false">Cancel</button>
        <button type="button" class="text-button primary" :disabled="!chosen.length" @click="exportChosen">
          Export {{ chosen.length === 1 ? '1 spark' : `${chosen.length} sparks` }}
        </button>
      </template>
    </Dialog>
  </section>
</template>

<style scoped>
/* The heading row is 30px high, as it is with no buttons; the buttons sit at its right,
   where every list keeps its add button. */
.heading {
  display: flex;
  align-items: center;
  gap: 16px;
  height: 30px;
}

.heading.moved {
  display: none;
}

.actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
}

.intro {
  margin: 4px 0 12px;
  color: var(--muted);
  font-size: calc(13rem / 16);
}

.spark {
  cursor: default;
}

.spark .label {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.name,
.spark .secondary {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.secondary.failed,
.problem {
  color: var(--danger);
}

.editor {
  margin-top: 12px;
  padding: 12px;
}

/* A clash: its name over its choice. */
.clash {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  padding: 10px 0;
}

.clash + .clash {
  border-top: 1px solid var(--line);
}

.clash-name {
  display: flex;
  flex-direction: column;
}

.clash .secondary,
.outcome .secondary {
  color: var(--muted);
  font-size: calc(12rem / 16);
}

.outcome {
  max-height: 50vh;
  margin: 0;
  padding: 0;
  overflow-y: auto;
  list-style: none;
}

.outcome li {
  display: flex;
  flex-direction: column;
  padding: 4px 0;
  overflow-wrap: anywhere;
}

.all {
  border-bottom: 1px solid var(--line);
  margin-bottom: 4px;
  padding-bottom: 10px;
}

textarea {
  width: 100%;
  box-sizing: border-box;
  padding: 8px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
  color: var(--text);
  font: calc(12rem / 16)/1.5 ui-monospace, SFMono-Regular, Menlo, monospace;
  resize: vertical;
}

.problem {
  margin: 8px 0 0;
  font-size: calc(13rem / 16);
}

.buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}
</style>
