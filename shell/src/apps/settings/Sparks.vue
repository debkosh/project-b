<script setup>
import { computed, inject, ref, watch } from 'vue';
import { send, state } from './state.js';

// Sparks (docs/site-features.md): the person's scripts for pages, which Go keeps, parses and
// matches. Each row has its switch, Edit, Export and Delete; New opens an empty editor,
// Import takes a .user.js file. A save that Go refuses says why under the editor.

// On a page of its own the page's heading names the list.
const ownPage = inject('settingsListPage', false);

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
      download(s.fileName, s.source);
    }
  },
);

function download(name, source) {
  const a = document.createElement('a');
  a.href = URL.createObjectURL(new Blob([source], { type: 'text/javascript' }));
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

const picker = ref(null);
async function imported(e) {
  const file = e.target.files[0];
  e.target.value = '';
  if (!file) return;
  // Go holds a spark to 140 KB; a larger file is refused as it is read.
  if (file.size > 150 * 1024) {
    startNew('');
    state.sparkProblem = `${file.name} is larger than 150 KB`;
    return;
  }
  startNew(await file.text());
  save();
}

const full = computed(() => state.sparks.length >= 25);
const where = (s) => (s.match || []).join(', ');
</script>

<template>
  <section aria-label="Sparks">
    <h2 v-if="!ownPage">Sparks</h2>
    <p class="intro">Your own scripts for the pages they match, each run apart from the page's own. At most 25.</p>
    <div class="card">
      <div v-for="s in state.sparks" :key="s.id" class="row spark" role="group" :aria-label="s.name || 'A spark that has a problem'">
        <span class="label">
          {{ s.name || 'A spark that has a problem' }}
          <span class="secondary" :class="{ failed: s.error }">{{ s.error || where(s) }}</span>
        </span>
        <input type="checkbox" class="switch" :aria-label="`${s.name} on`" :checked="s.on" @change="send({ t: 'sparkSetOn', id: s.id, on: $event.target.checked })" />
        <button type="button" class="text-button" @click="ask(s.id, 'edit')">Edit</button>
        <button type="button" class="text-button" :disabled="!!s.error" @click="ask(s.id, 'export')">Export</button>
        <button type="button" class="text-button" @click="send({ t: 'sparkRemove', id: s.id })">Delete</button>
      </div>
      <div class="row">
        <span class="label secondary">{{ state.sparks.length ? `${state.sparks.length} of 25` : 'No sparks yet' }}</span>
        <button type="button" class="text-button" :disabled="full" @click="startNew()">New</button>
        <button type="button" class="text-button" :disabled="full" @click="picker.click()">Import</button>
        <input ref="picker" type="file" accept=".js,text/javascript" hidden @change="imported" />
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
  </section>
</template>

<style scoped>
.intro {
  margin: 0 0 12px;
  color: var(--muted);
  font-size: 13px;
}

.spark .label {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.spark .secondary {
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

textarea {
  width: 100%;
  box-sizing: border-box;
  padding: 8px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
  color: var(--text);
  font: 12px/1.5 ui-monospace, SFMono-Regular, Menlo, monospace;
  resize: vertical;
}

.problem {
  margin: 8px 0 0;
  font-size: 13px;
}

.buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}
</style>
