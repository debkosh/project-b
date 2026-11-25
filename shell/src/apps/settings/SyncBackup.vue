<script setup>
import { computed, nextTick, onMounted, onUnmounted, reactive, ref } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import { codePath, codeSize } from '@/ui/passkey.js';
import Row from './Row.vue';
import { numbered, send, state, syncAnswers } from './state.js';

// The backup code (docs/sync.md, "Backup code"): a second, random passphrase the app shows
// once, for a person to keep apart from the device. Its row in the Sync card says when it
// was made, checks one typed, and makes a new one; with none, the row offers to make one.
// The code's card is the second step of setting sync up, where it can be skipped, and what
// New code… and Make one… open. A code is made as its card opens. "I've saved it" keeps it,
// and takes away the one before it; Cancel, Skip for now and Escape take the new one away,
// and leave what was. The code is in this page only while its card shows.
defineProps({ words: String });

const s = computed(() => state.sync);
const made = computed(() => {
  const ms = s.value.backup?.made;
  return ms ? `Made ${new Date(ms).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })}` : 'Made on another device';
});

// The Dialog: step is '' while shut, 'confirm' (a new code in place of one), 'code' (the
// card) or 'check'. setup is the card as the second step of setting sync up.
const b = reactive({ step: '', setup: false, code: '', qr: null, showQR: false, busy: false, error: '', said: '', typed: '', is: null });
// Each thing asked of Go has a number, which its answer brings back (numbered), and the card
// takes the answer to what it asked last and no other. Go answers in turn, so the answer to a
// making that was left can come after the card was opened again and asked for a code of its
// own.
let asked = 0;
function ask(m) {
  const q = numbered(m);
  asked = q.n;
  send(q);
}
function open(o) {
  asked = 0;
  Object.assign(b, { step: '', setup: false, code: '', qr: null, showQR: false, busy: false, error: '', said: '', typed: '', is: null, ...o });
}
function shut() {
  asked = 0;
  Object.assign(b, { step: '', code: '', qr: null, typed: '' });
}

const field = ref(null);
const primary = ref(null);
const focus = (r) => nextTick(() => r.value?.focus());

// Making a code: as the second step of setting sync up, or from the row.
function make(setup) {
  open({ step: 'code', setup: !!setup, busy: true });
  ask({ t: 'syncBackupMake' });
}
defineExpose({ offer: () => make(true) });
const fresh = () => (s.value.backup ? open({ step: 'confirm' }) : make(false));

function keep() {
  if (b.busy || !b.code) return;
  b.busy = true;
  ask({ t: 'syncBackupKeep' });
}
// Leaving the card any other way: the code just made goes, as nobody said they saved it. So
// does one still being made, which this page hasn't got yet: Go takes what is asked of sync
// in turn, so the drop comes after the making, and with nothing made it does nothing.
function leave() {
  send({ t: 'syncBackupDrop' });
  shut();
}

// The code as its card shows it: two lines of three groups.
const lines = computed(() => {
  const g = b.code.split('-');
  return [g.slice(0, 3).join('-'), g.slice(3).join('-')];
});
let saidFor = 0;
function say(text) {
  b.said = text;
  clearTimeout(saidFor);
  saidFor = setTimeout(() => (b.said = ''), 3000);
}
const copy = () => ask({ t: 'syncBackupCopy', code: b.code });
// The file goes where this device's downloads go.
function save() {
  const text = `Project B sync backup code\n\n${b.code}\n\nFor ${s.value.host}, made ${new Date().toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })}.\nIf you forget your sync passphrase, this code opens your sync. Keep it apart from your devices.\n`;
  const a = document.createElement('a');
  a.href = URL.createObjectURL(new Blob([text], { type: 'text/plain' }));
  a.download = 'Project B backup code.txt';
  a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  say('Saved to your downloads');
}

function check() {
  open({ step: 'check' });
  focus(field);
}
// A phone's scanner reads the code into the field, and it is checked at once.
const scan = () => ask({ t: 'syncBackupScan' });
function checkNow() {
  if (b.busy || !b.typed.trim()) return;
  b.busy = true;
  b.is = null;
  ask({ t: 'syncBackupCheck', code: b.typed });
}

function heard(m) {
  if (!b.step || !asked || m.n !== asked) return;
  switch (m.t) {
    case 'syncBackupMade':
      if (b.step !== 'code') return;
      b.busy = false;
      if (m.error) b.error = m.error;
      else Object.assign(b, { code: m.code, qr: m.qr });
      focus(primary);
      return;
    case 'syncBackupSettled':
      if (b.step !== 'code') return;
      b.busy = false;
      if (m.error) b.error = m.error;
      else shut();
      return;
    case 'syncBackupCopied':
      if (b.step === 'code') say('Copied');
      return;
    case 'syncBackupScanned':
      if (b.step !== 'check' || !m.ok) return;
      b.typed = m.code;
      checkNow();
      return;
    case 'syncBackupChecked':
      if (b.step !== 'check') return;
      b.busy = false;
      if (m.error) b.error = m.error;
      else b.is = !!m.is;
      focus(field);
  }
}
onMounted(() => syncAnswers.add(heard));
onUnmounted(() => {
  syncAnswers.delete(heard);
  clearTimeout(saidFor);
  if (b.step === 'code') send({ t: 'syncBackupDrop' });
});
</script>

<template>
  <Row v-if="s.backup" label="Backup code" :hint="made" :words="words">
    <span class="two">
      <button type="button" class="text-button" @click="check">Check…</button>
      <button type="button" class="text-button" @click="fresh">New code…</button>
    </span>
  </Row>
  <Row v-else label="No backup code" hint="A way back in if you forget your passphrase" :words="words">
    <button type="button" class="text-button" @click="fresh">Make one…</button>
  </Row>

  <Dialog v-if="b.step === 'confirm'" title="Make a new backup code?" @close="shut">
    <div class="ui-dialog-part">
      <p>Your current code stops working once you've saved the new one.</p>
    </div>
    <template #buttons>
      <button type="button" class="text-button" @click="shut">Cancel</button>
      <button type="button" class="text-button primary" @click="make(false)">Make new code</button>
    </template>
  </Dialog>

  <Dialog v-else-if="b.step === 'code'" class="backup-code" title="Save your backup code" @close="leave">
    <template #title
      ><span v-if="b.setup" class="stepof">Step 2 of 2</span>Save your backup code</template
    >
    <div class="ui-dialog-part">
      <p>If you forget your passphrase, this code opens your sync. Keep it somewhere apart from this device.</p>
    </div>
    <div class="ui-dialog-part">
      <div v-if="b.error" class="text-field-error" role="alert">{{ b.error }}</div>
      <div v-else-if="!b.code" class="code making" role="status">Making your backup code…</div>
      <template v-else>
        <div v-if="b.showQR && b.qr" class="qr">
          <svg role="img" aria-label="QR code of your backup code" :viewBox="`0 0 ${codeSize(b.qr)} ${codeSize(b.qr)}`" shape-rendering="crispEdges">
            <rect width="100%" height="100%" fill="#fff" />
            <path :d="codePath(b.qr)" fill="#000" />
          </svg>
          <div class="small" role="group" aria-label="Backup code">{{ b.code }}</div>
        </div>
        <!-- A group named Backup code, with the code as its words: a name given to the code's
             own element took the code's place for a screen reader, which then read no code.
             The dash between its two lines is there for what reads or copies the text, and
             isn't drawn. -->
        <div v-else class="code" role="group" aria-label="Backup code">
          <span>{{ lines[0] }}</span><span class="between">-</span><span>{{ lines[1] }}</span>
        </div>
        <div class="acts">
          <button type="button" class="text-button" @click="copy">Copy</button>
          <button type="button" class="text-button" @click="save">Save as file</button>
          <button v-if="b.qr" type="button" class="text-button" :aria-pressed="String(b.showQR)" @click="b.showQR = !b.showQR">{{ b.showQR ? 'Hide QR' : 'Show as QR' }}</button>
          <span v-if="b.said" class="secondary said" role="status">{{ b.said }}</span>
        </div>
      </template>
    </div>
    <template #buttons>
      <button type="button" class="text-button quiet leave" @click="leave">{{ b.setup ? 'Skip for now' : 'Cancel' }}</button>
      <button ref="primary" type="button" class="text-button primary" :disabled="!b.code || b.busy" @click="keep">I've saved it</button>
    </template>
  </Dialog>

  <Dialog v-else-if="b.step === 'check'" title="Check your backup code" @close="shut">
    <div class="ui-dialog-part">
      <label for="sync-backup-typed">Backup code</label>
      <div class="scans">
        <input
          id="sync-backup-typed"
          ref="field"
          v-model="b.typed"
          class="text-field typed"
          :class="{ scanned: state.canScan }"
          type="text"
          spellcheck="false"
          autocomplete="off"
          autocapitalize="characters"
          placeholder="XXXX-XXXX-XXXX-XXXX-XXXX-XXXX"
          @input="b.is = null"
          @keydown.enter.prevent="checkNow"
        />
        <!-- A phone's scanner, at the field's end: a saved code may be only its QR. -->
        <button v-if="state.canScan" type="button" class="scan" aria-label="Scan QR code" title="Scan QR code" @click="scan">
          <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <rect x="2" y="2" width="4.5" height="4.5" rx="0.5" />
          <rect x="9.5" y="2" width="4.5" height="4.5" rx="0.5" />
          <rect x="2" y="9.5" width="4.5" height="4.5" rx="0.5" />
          <path d="M9.5 9.5h1.5v1.5M14 9.5v1.5M9.5 14h1.5M13 13h1v1" />
          </svg>
        </button>
      </div>
      <div v-if="b.error" class="text-field-error" role="alert">{{ b.error }}</div>
      <div v-else-if="b.is === true" class="answer yes" role="status">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 8.5l3.25 3.25L13 5" /></svg>
        This is your backup code.
      </div>
      <div v-else-if="b.is === false" class="answer" role="status">That's not this sync's backup code.</div>
    </div>
    <template #buttons>
      <button type="button" class="text-button" @click="shut">Done</button>
      <button type="button" class="text-button primary" :disabled="b.busy || !b.typed.trim()" @click="checkNow">Check</button>
    </template>
  </Dialog>
</template>

<style scoped>
.two {
  display: flex;
  gap: 8px;
}

.stepof {
  display: block;
  margin-bottom: 4px;
  color: var(--muted);
  font-size: calc(12rem / 16);
  font-weight: 400;
}

/* The code: every character told apart, large enough to copy down by hand. */
.code {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 16px 12px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface-2, var(--surface));
  font:
    600 calc(20rem / 16)/28px ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
  letter-spacing: 0.02em;
  /* The code has 0 and 1 and never O, I or L: its zero is drawn as one. */
  font-variant-numeric: slashed-zero;
  user-select: text;
}

/* In the text, out of sight. */
.between {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}

.code.making {
  color: var(--muted);
  font:
    400 calc(13rem / 16)/28px system-ui,
    sans-serif;
  letter-spacing: 0;
}

.qr {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.qr svg {
  width: 176px;
  height: 176px;
  border-radius: 4px;
}

.small {
  font-variant-numeric: slashed-zero;
  color: var(--muted);
  font:
    calc(12rem / 16)/16px ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
  user-select: text;
}

.acts {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
}

.said {
  margin-left: 4px;
}

.leave {
  margin-right: auto;
}

/* The field with a phone's scanner at its end: an icon button, 40px in a 48px field. */
.scans {
  position: relative;
}

.scan {
  position: absolute;
  top: 4px;
  right: 4px;
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--icon);
  cursor: pointer;
}

.touch .text-field.scanned {
  padding-right: 48px;
  font-size: calc(14rem / 16);
}

.typed {
  font-variant-numeric: slashed-zero;
  display: block;
  width: 100%;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.answer {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  color: var(--muted);
}

.answer.yes {
  color: var(--text);
}

.answer.yes svg {
  color: var(--accent);
}

/* A phone, as the Sync card's own buttons and fields are there (Sync.vue): drawn at 40px,
   each a 48px target; fields 48px. */
.touch .text-button {
  position: relative;
  isolation: isolate;
  height: 48px;
  min-height: 48px;
  margin: -4px 0;
  border: 0;
  background: none;
}

.touch .text-button::before {
  content: '';
  position: absolute;
  inset: 4px 0;
  z-index: -1;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
}

.touch .text-button.primary::before {
  border-color: var(--accent);
  background: var(--accent);
}

.touch .text-button:hover:not(:disabled) {
  background: none;
}

.touch .text-button.quiet::before {
  border-color: transparent;
  background: none;
}

.touch .text-button:focus-visible {
  box-shadow: none;
}

.touch .text-button:focus-visible::before {
  box-shadow: var(--focus-ring);
}

.touch .text-field {
  height: 48px;
  padding: 0 12px;
}

.touch .acts {
  row-gap: 0;
}
</style>
