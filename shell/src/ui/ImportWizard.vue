<script setup>
import { computed, nextTick, ref, watch } from 'vue';
import AllowAccess from './AllowAccess.vue';
import BrowserIcon from './BrowserIcon.vue';
import ImportChoices from './ImportChoices.vue';
import { aboutBrowser, choicesOf, holds, imports, passwordsLine, startRun, stepLines } from './imports.js';

// Bringing things over, as a wizard (docs/onboarding.md), the welcome's step and Settings'
// Import alike: which browser, then which of its profiles and what to bring, then each kind
// ticked as it lands, then what came. A browser this Mac doesn't show, or one kept
// elsewhere, is a folder the person chooses. Safari asks for Full Disk Access first.
// Passwords come once there is a store for them, a Firefox profile's with its primary
// password, used once. With files, as Settings has it, a file a browser exported comes
// too: bookmarks as HTML, passwords as CSV, which the page reads and hands to Go. send is
// the page's way to Go; heading is the level of the wizard's headings.
const props = defineProps({ send: Function, heading: { type: String, default: 'h2' }, files: Boolean });

const view = ref('browsers');
const browser = ref(null);
const key = ref(null);
const what = ref([]);
const asked = ref([]);
const folderError = ref('');
const choosing = ref(false);
const title = ref(null);
const primary = ref('');
const safeStorage = ref('');
const fileName = ref('');
const fileKind = ref('');

const profile = computed(() => browser.value?.profiles.find((p) => p.key === key.value) || null);
const profileTitle = computed(() => (profile.value?.name ? `${browser.value.name} · ${profile.value.name}` : browser.value?.name || ''));
const run = computed(() => imports.runs[key.value]);
// A browser known only by its folder whose folder doesn't say which keychain item holds
// its passwords' key: the person picks one before its passwords can come. One that didn't
// open them (failed) is said so, and another is picked from those Go then names.
const failed = ref(null);
const keychains = computed(() => (what.value.includes('passwords') && (failed.value?.keychains || profile.value?.keychains)) || []);
const wrong = computed(() => {
  const s = run.value?.done && run.value.steps.passwords;
  return s && s.wrongKey && (s.keychains || []).length > 1 ? s : null;
});
function pickAnother() {
  failed.value = { service: wrong.value.keychain, keychains: wrong.value.keychains };
  safeStorage.value = '';
  what.value = ['passwords'];
  view.value = 'profile';
}

// Each view's heading takes the keyboard's focus, so a screen reader reads where it is.
watch(view, () => nextTick(() => title.value?.focus()));

// A profile's choices, ticked but its extensions, which install things.
function pick(p) {
  key.value = p.key;
  safeStorage.value = '';
  failed.value = null;
  what.value = choicesOf(p).map((c) => c.id).filter((id) => id !== 'extensions');
}

function open(b) {
  folderError.value = '';
  browser.value = b;
  if (b.locked) {
    view.value = 'allow';
    props.send({ t: 'openAccess' });
    return;
  }
  pick(b.profiles[0]);
  view.value = 'profile';
}

// Safari, given the access: Go looks again, and its profiles have counts.
watch(
  () => imports.browsers,
  (list) => {
    if (view.value !== 'allow') return;
    const b = list?.find((x) => x.id === browser.value?.id);
    if (b && !b.locked) open(b);
  },
);

function chooseFolder() {
  folderError.value = '';
  imports.folder = null;
  choosing.value = true;
  props.send({ t: 'chooseFolder' });
}
watch(
  () => imports.folder,
  (f) => {
    if (!choosing.value || !f) return;
    choosing.value = false;
    if (f.error) folderError.value = f.error;
    else if (f.browser) open(f.browser);
  },
);

// An import, and its Retry and Cancel while it runs (docs/ux-guide.md): Retry starts the
// same import over, Go ending the one going on; a Firefox primary password is kept here for
// it until the person moves on, and kept nowhere else, as is the keychain item they chose.
let lastPrimary = '';
let lastKeychain = '';
function bring() {
  asked.value = [...what.value];
  lastPrimary = primary.value;
  lastKeychain = keychains.value.length ? safeStorage.value : '';
  primary.value = '';
  retry();
  view.value = 'importing';
}
function retry() {
  startRun(key.value);
  props.send({ t: 'import', key: key.value, what: asked.value, primaryPassword: lastPrimary, keychain: lastKeychain });
}
const cancel = () => props.send({ t: 'cancelImport', key: key.value });

// A file the person picked, read here and handed to Go whole.
async function readFile(kind, event) {
  const file = event.target.files?.[0];
  event.target.value = '';
  if (!file) return;
  imports.file = null;
  fileKind.value = kind;
  fileName.value = file.name;
  view.value = 'file';
  props.send({ t: 'importFile', kind, name: file.name, text: await file.text() });
}
const fileDone = computed(() => imports.file && imports.file.name === fileName.value && imports.file.kind === fileKind.value ? imports.file : null);
const fileLines = computed(() => {
  const f = fileDone.value;
  if (!f) return [{ text: fileKind.value === 'passwords' ? 'Passwords' : 'Bookmarks', state: 'doing' }];
  if (f.error) return [{ text: f.error, state: 'problem' }];
  return f.kind === 'passwords' ? [passwordsLine(f.result)] : stepLines('bookmarks', { state: 'done', result: f.result });
});

const ended = computed(() => !!run.value?.done && run.value.how !== 'done');
const lines = computed(() => asked.value.flatMap((k) => stepLines(k, run.value?.steps[k], ended.value).map((l) => ({ ...l, kind: k }))));
const importingTitle = computed(() => {
  if (!run.value?.done) return `Importing from ${profileTitle.value}`;
  return { cancelled: `Cancelled importing from ${profileTitle.value}`, failed: `Couldn't finish importing from ${profileTitle.value}` }[run.value.how] || `Imported from ${profileTitle.value}`;
});

function again() {
  lastPrimary = '';
  lastKeychain = '';
  failed.value = null;
  browser.value = null;
  key.value = null;
  view.value = 'browsers';
  props.send({ t: 'findBrowsers' });
}
</script>

<template>
  <div class="ui-import-wizard">
    <!-- Which browser -->
    <template v-if="view === 'browsers'">
      <component :is="heading" ref="title" tabindex="-1">Bring your things over</component>
      <p class="sub">From another browser on this Mac. Nothing leaves it.</p>
      <div class="list">
        <div v-if="imports.browsers === null" class="item"><span class="state">Looking for other browsers…</span></div>
        <button v-for="b in imports.browsers || []" :key="b.id" type="button" class="item" :aria-label="`From ${b.name}`" @click="open(b)">
          <BrowserIcon :browser="b.id" />
          <span class="what">
            <span class="label">{{ b.name }}</span>
            <span class="state">{{ aboutBrowser(b) }}</span>
          </span>
          <span class="go" aria-hidden="true">›</span>
        </button>
        <button type="button" class="item" aria-label="From another browser" @click="chooseFolder">
          <BrowserIcon browser="other" />
          <span class="what">
            <span class="label">Another browser</span>
            <span class="state">Choose its profiles folder</span>
          </span>
          <span class="go" aria-hidden="true">›</span>
        </button>
      </div>
      <p v-if="folderError" class="problem" role="alert">{{ folderError }}</p>
      <template v-if="files">
        <span class="label from-file">From a file</span>
        <div class="list">
          <label class="item">
            <input type="file" class="file" accept=".html,.htm" aria-label="Bookmarks file" @change="readFile('bookmarks', $event)" />
            <span class="what">
              <span class="label">Bookmarks file</span>
              <span class="state">An HTML file a browser exported</span>
            </span>
            <span class="go" aria-hidden="true">›</span>
          </label>
          <label v-if="imports.passwords" class="item">
            <input type="file" class="file" accept=".csv" aria-label="Passwords file" @change="readFile('passwords', $event)" />
            <span class="what">
              <span class="label">Passwords file</span>
              <span class="state">A CSV file a browser or the Passwords app exported</span>
            </span>
            <span class="go" aria-hidden="true">›</span>
          </label>
        </div>
      </template>
    </template>

    <!-- A file: what it brought -->
    <template v-else-if="view === 'file'">
      <component :is="heading" ref="title" tabindex="-1">{{ fileDone ? 'Imported' : 'Importing' }} from {{ fileName }}</component>
      <ul class="list steps" :aria-label="`From ${fileName}`" :aria-busy="String(!fileDone)">
        <li v-for="(l, i) in fileLines" :key="i" class="item" :class="l.state">
          <span class="mark" aria-hidden="true">
            <svg v-if="l.state === 'done'" viewBox="0 0 16 16"><path d="M4 8.5l2.5 2.5L12 5.5" /></svg>
          </span>
          <span class="what">
            <span class="label">{{ l.text }}</span>
            <span v-if="l.note" class="state">{{ l.note }}</span>
          </span>
        </li>
      </ul>
      <p v-if="fileDone && fileKind === 'passwords' && !fileDone.error" class="sub">Delete {{ fileName }} now: it holds your passwords in plain text.</p>
      <div v-if="fileDone" class="buttons">
        <button type="button" class="text-button" @click="again">Import another</button>
      </div>
    </template>

    <!-- Safari: Full Disk Access first -->
    <template v-else-if="view === 'allow'">
      <component :is="heading" ref="title" tabindex="-1">Allow {{ browser.name }}'s things</component>
      <p class="sub">macOS keeps {{ browser.name }}'s files behind Full Disk Access.</p>
      <AllowAccess class="allow" @open="send({ t: 'openAccess' })" />
      <div class="buttons">
        <button type="button" class="text-button quiet" @click="view = 'browsers'">Not now</button>
      </div>
    </template>

    <!-- Which profile, and what to bring -->
    <template v-else-if="view === 'profile'">
      <component :is="heading" ref="title" tabindex="-1" class="named"><BrowserIcon :browser="browser.id" />{{ browser.name }}</component>
      <p class="sub">
        {{ browser.profiles.length > 1 ? 'Which profile?' : holds(browser.profiles[0]) }}
        <button type="button" class="link" @click="chooseFolder">Choose a different folder…</button>
      </p>
      <div v-if="browser.profiles.length > 1" class="list" role="radiogroup" aria-label="Profile">
        <label v-for="p in browser.profiles" :key="p.key" class="item">
          <input type="radio" name="profile" :checked="key === p.key" @change="pick(p)" />
          <span class="what">
            <span class="label">{{ p.name || browser.name }}</span>
            <span class="state">{{ holds(p) }}</span>
          </span>
        </label>
      </div>
      <p v-if="folderError" class="problem" role="alert">{{ folderError }}</p>
      <template v-if="profile && choicesOf(profile).length">
        <span class="label bring-label">Bring over</span>
        <ImportChoices v-model="what" :profile="profile" :title="profileTitle" />
      </template>
      <label v-if="profile?.primaryPassword && what.includes('passwords')" class="primary">
        <span class="label">{{ browser.name }}'s primary password</span>
        <span class="state">Its passwords are behind it. It is used once, and kept nowhere.</span>
        <input v-model="primary" type="password" autocomplete="off" :aria-label="`${browser.name}'s primary password`" />
      </label>
      <div v-if="keychains.length" class="safe-storage">
        <span class="label">{{ browser.name }}'s key in the keychain</span>
        <span v-if="failed" class="state problem" role="alert">The passwords didn't open with {{ failed.service || 'that key' }}. Pick another and import again.</span>
        <span v-else class="state">Its folder doesn't say which of these holds its passwords' key.</span>
        <div class="list" role="radiogroup" :aria-label="`${browser.name}'s key in the keychain`">
          <label v-for="k in keychains" :key="k" class="item">
            <input v-model="safeStorage" type="radio" name="safe-storage" :value="k" />
            <span class="what"><span class="label">{{ k }}</span></span>
          </label>
        </div>
      </div>
      <div class="buttons">
        <button type="button" class="text-button quiet" @click="view = 'browsers'">Back</button>
        <button type="button" class="text-button primary" :disabled="!what.length || (keychains.length > 0 && !safeStorage)" @click="bring">Import</button>
      </div>
    </template>

    <!-- Importing, then what came -->
    <template v-else>
      <component :is="heading" ref="title" tabindex="-1">{{ importingTitle }}</component>
      <p class="sub">{{ run?.done ? 'Bookmarks are on the Bookmarks page, in their own folder.' : 'Each is ticked as it lands.' }}</p>
      <ul class="list steps" :aria-label="`From ${profileTitle}`" :aria-busy="String(!run?.done)">
        <li v-for="(l, i) in lines" :key="i" class="item" :class="[l.state, { left: l.left }]">
          <span class="mark" aria-hidden="true">
            <svg v-if="l.state === 'done' && !l.left" viewBox="0 0 16 16"><path d="M4 8.5l2.5 2.5L12 5.5" /></svg>
          </span>
          <span class="what">
            <span class="label">{{ l.text }}</span>
            <span v-if="l.note" class="state">{{ l.note }}</span>
          </span>
        </li>
      </ul>
      <div v-if="!run?.done" class="buttons">
        <button type="button" class="text-button quiet" @click="retry">Retry</button>
        <button type="button" class="text-button quiet" @click="cancel">Cancel</button>
      </div>
      <div v-else class="buttons">
        <button v-if="wrong" type="button" class="text-button" @click="pickAnother">Pick another key</button>
        <button v-if="ended" type="button" class="text-button" @click="retry">Try again</button>
        <button type="button" class="text-button" @click="again">Import another</button>
      </div>
    </template>
  </div>
</template>

<style>
.ui-import-wizard {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-width: 600px;
}

.ui-import-wizard h1,
.ui-import-wizard h2 {
  margin: 0;
  outline: none;
}

/* Settings' own h2 is a small section label; the wizard's is its title. */
.ui-import-wizard h2 {
  font-size: 20px;
  font-weight: 600;
  color: var(--text);
  letter-spacing: normal;
  text-transform: none;
}

.ui-import-wizard .named {
  display: flex;
  align-items: center;
  gap: 12px;
}

.ui-import-wizard .sub {
  margin: 0 0 8px;
  color: var(--muted);
}

.ui-import-wizard .link {
  padding: 0;
  border: 0;
  background: none;
  color: var(--accent);
  font: inherit;
  text-decoration: underline;
  cursor: pointer;
}

.ui-import-wizard .list {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
  border-radius: 8px;
  background: var(--surface);
  box-shadow: inset 0 0 0 1px var(--line);
}

.ui-import-wizard .item {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 56px;
  padding: 8px 16px;
  border: 0;
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
}

.ui-import-wizard .item + .item {
  border-top: 1px solid var(--line);
}

.ui-import-wizard input[type='radio'] {
  accent-color: var(--accent);
}

.ui-import-wizard button.item {
  cursor: pointer;
}

.ui-import-wizard button.item:hover {
  background: var(--accent-tint);
}

.ui-import-wizard .what {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
}

.ui-import-wizard .label {
  font-weight: 500;
}

.ui-import-wizard .state {
  color: var(--muted);
  font-size: 12px;
}

.ui-import-wizard .go {
  color: var(--muted);
  font-size: 18px;
}

.ui-import-wizard .problem {
  margin: 0;
  color: var(--danger);
}

.ui-import-wizard .bring-label,
.ui-import-wizard .from-file {
  margin-top: 8px;
}

.ui-import-wizard label.item {
  cursor: pointer;
}

.ui-import-wizard label.item:hover {
  background: var(--accent-tint);
}

/* The file input is the row: hidden, its label the row a person presses. */
.ui-import-wizard .file {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
}

.ui-import-wizard .primary {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.ui-import-wizard .safe-storage {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.ui-import-wizard .safe-storage .list {
  margin-top: 4px;
}

.ui-import-wizard .primary input {
  max-width: 320px;
  height: 32px;
  padding: 0 10px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
}

.ui-import-wizard .buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}

/* Each kind: a ring while it waits, a spinner while it comes, a tick once it has. */
.ui-import-wizard .steps .mark {
  display: grid;
  place-items: center;
  flex: none;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  box-sizing: border-box;
}

.ui-import-wizard .steps .waiting .mark,
.ui-import-wizard .steps .left .mark {
  border: 1.5px dashed var(--line);
}

.ui-import-wizard .steps .doing .mark {
  border: 2px solid var(--accent-soft);
  border-top-color: var(--accent);
  animation: ui-import-spin 0.9s linear infinite;
}

.ui-import-wizard .steps .done:not(.left) .mark {
  background: var(--accent);
}

.ui-import-wizard .steps .problem .mark {
  border: 2px solid var(--danger);
}

.ui-import-wizard .steps .mark svg {
  width: 14px;
  height: 14px;
  fill: none;
  stroke: var(--on-accent);
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.ui-import-wizard .steps .waiting .label,
.ui-import-wizard .steps .left .label {
  color: var(--muted);
  font-weight: 400;
}

.ui-import-wizard .steps .problem .label {
  color: var(--danger);
}

@media (prefers-reduced-motion: reduce) {
  .ui-import-wizard .steps .doing .mark {
    animation-duration: 3s;
  }
}

@keyframes ui-import-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
