<script setup>
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue';
import { pushEscape } from '@/ui/escape.js';
import { downloadsPanel } from '@/ui/downloads.js';
import { size } from '@/ui/size.js';
import { clearDownloads, downloadAction, focusPage, send, state } from '@/sdk/vue.js';

// The toolbar's Downloads control, there once the profile has a download, with the running
// ones' progress round it, and the list it opens: each download by its file name, its
// progress or how it ended, and what can be done to it (docs/daily-driver.md). Each opens
// out at its Details control: where it came from, the whole address with Copy link, or what
// the app made it as, and the folder it is saved in.
//
// With peek, the list also opens by itself when Go says a download began in this window
// (downloadStarted), so the person sees it start. It closes after peekFor unless the pointer
// is over it or a control in it has the keyboard, and at once on Escape or a press outside
// it. It has no cover while it peeks, so the page under it stays usable: it takes the
// keyboard while it shows, so Escape reaches it and a press on the page, which takes the
// keyboard from the shell, closes it; and it gives the keyboard back as it closes. A press
// on the control makes it the person's list, as if the control had opened it.
const props = defineProps({ peek: Boolean });
const peekFor = 5000;

const open = ref(false);
// Whether the list opened by itself, and not yet touched.
const peeking = ref(false);
let peekTimer = 0;
let pointerIn = false;
// Whether the page had the keyboard when the list peeked, to give it back.
let tookKeyboard = false;
const button = ref(null);
const panel = ref(null);
// Under the control, its right edge with the control's.
const at = ref({ top: 0, right: 0 });
let pop = null;

const running = computed(() => state.downloads.filter((d) => d.state === 'progress'));
// How far the running ones have got together, 0 to 1, or -1 when a size is unknown.
const progress = computed(() => {
  const r = running.value;
  if (!r.length) return 0;
  if (r.some((d) => d.total <= 0)) return -1;
  return r.reduce((n, d) => n + d.received, 0) / r.reduce((n, d) => n + d.total, 0);
});

// Why one failed, in plain words, from the engine's reason (docs/daily-driver.md).
const failedWords = { connection: 'Failed: the connection stopped', server: 'Failed: the server refused it', disk: 'Failed: not enough disk space' };

function status(d) {
  switch (d.state) {
    case 'progress':
      return (d.paused ? 'Paused, ' : '') + (d.total > 0 ? `${size(d.received)} of ${size(d.total)}` : size(d.received));
    case 'done':
      return [d.total > 0 ? size(d.total) : 'Done', d.from].filter(Boolean).join(' · ');
    case 'cancelled':
      return 'Cancelled';
    default:
      return failedWords[d.reason] || 'Failed';
  }
}

// The rows opened out, by id, and the one whose link was just copied.
const opened = reactive(new Set());
const copied = ref(0);
let copiedTimer = 0;
const toggle = (id) => (opened.has(id) ? opened.delete(id) : opened.add(id));

function copyLink(d) {
  downloadAction(d.id, 'copyLink');
  copied.value = d.id;
  clearTimeout(copiedTimer);
  copiedTimer = setTimeout(() => (copied.value = 0), 1500);
}

const folder = (path) => path.slice(0, path.lastIndexOf('/')) || '/';

async function show() {
  const r = button.value.getBoundingClientRect();
  at.value = { top: r.bottom + 6, right: Math.max(8, window.innerWidth - r.right) };
  open.value = true;
  pop = pushEscape(() => close(true));
  await nextTick();
  panel.value?.querySelector('button')?.focus();
}

function close(refocus) {
  if (!open.value) return;
  open.value = false;
  pop?.();
  pop = null;
  const back = peeking.value && tookKeyboard;
  stopPeeking();
  tookKeyboard = false;
  if (refocus) button.value?.focus();
  else if (back) focusPage();
}

// The control: it opens or closes the list, and makes a list that peeked the person's.
function onControl() {
  if (peeking.value) {
    stopPeeking();
    panel.value?.querySelector('button')?.focus();
  } else if (open.value) close(true);
  else show();
}

function stopPeeking() {
  peeking.value = false;
  clearTimeout(peekTimer);
  removeEventListener('blur', closePeek);
  removeEventListener('pointerdown', pressWhilePeeking, true);
}

function closePeek() {
  if (peeking.value) close(false);
}

// A press in the shell outside the list and its control closes a list that peeked.
function pressWhilePeeking(e) {
  if (!panel.value?.contains(e.target) && !button.value?.contains(e.target)) closePeek();
}

// peekOpen opens the list by itself for a download that began, unless the person has it
// open already, where it shows the download anyway.
async function peekOpen() {
  if (!props.peek || document.visibilityState !== 'visible' || (open.value && !peeking.value)) return;
  // The control comes with the list's first download, drawn after the list arrived.
  await nextTick();
  if (!button.value) return;
  const r = button.value.getBoundingClientRect();
  at.value = { top: r.bottom + 6, right: Math.max(8, window.innerWidth - r.right) };
  if (!open.value) {
    open.value = true;
    peeking.value = true;
    pop = pushEscape(() => close(false));
    tookKeyboard = !document.hasFocus();
    if (tookKeyboard) send({ t: 'focusShell' });
    await nextTick();
    panel.value?.focus();
    addEventListener('pointerdown', pressWhilePeeking, true);
    // The shell losing the keyboard means a press on the page, or another window.
    addEventListener('blur', closePeek);
  }
  waitThenClose();
}

// waitThenClose closes a list that peeked after peekFor, unless the pointer is over it or a
// control in it has the keyboard then.
function waitThenClose() {
  clearTimeout(peekTimer);
  peekTimer = setTimeout(() => {
    const inside = panel.value?.contains(document.activeElement) && document.activeElement !== panel.value;
    if (peeking.value && !pointerIn && !inside) close(false);
  }, peekFor);
}

function onPointer(over) {
  pointerIn = over;
  if (peeking.value && !over) waitThenClose();
}

// A control in the list taking the keyboard makes it the person's: it stays open.
function onFocusIn(e) {
  if (e.target !== panel.value) stopPeeking();
}

onMounted(() => {
  if (props.peek) downloadsPanel.started = peekOpen;
});

// Another tab shown closes it, as the menus close, and so does the list emptying, which
// takes the control away.
watch(() => state.active, () => close(false));
watch(
  () => state.downloads.length,
  (n) => n === 0 && close(false),
);

onUnmounted(() => {
  pop?.();
  stopPeeking();
  if (downloadsPanel.started === peekOpen) downloadsPanel.started = () => {};
});
</script>

<template>
  <button
    v-if="state.downloads.length"
    ref="button"
    class="icon-button downloads"
    type="button"
    aria-label="Downloads"
    title="Downloads"
    aria-haspopup="dialog"
    :aria-expanded="open"
    @click="onControl"
  >
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d="M8 2.5v8M4.5 7 8 10.5 11.5 7M3 13.5h10" />
    </svg>
    <svg v-if="running.length" class="ring" width="26" height="26" viewBox="0 0 26 26" aria-hidden="true">
      <circle cx="13" cy="13" r="11.5" fill="none" stroke="var(--line)" stroke-width="2" />
      <circle
        cx="13"
        cy="13"
        r="11.5"
        fill="none"
        stroke="var(--accent)"
        stroke-width="2"
        stroke-linecap="round"
        :class="{ spin: progress < 0 }"
        :stroke-dasharray="72.3"
        :stroke-dashoffset="progress < 0 ? 54 : 72.3 * (1 - progress)"
        transform="rotate(-90 13 13)"
      />
    </svg>
  </button>
  <Teleport to="body">
    <!-- Opened by the control, a cover takes every press outside; peeking, only the list is
         the shell's. -->
    <div v-if="open" v-shell :class="peeking ? 'downloads-peek' : 'downloads-cover'" @pointerdown.self="close(false)" @contextmenu.self.prevent="close(false)">
      <div
        ref="panel"
        v-shell
        class="downloads-panel"
        role="dialog"
        aria-label="Downloads"
        tabindex="-1"
        :style="{ top: at.top + 'px', right: at.right + 'px' }"
        @pointerenter="onPointer(true)"
        @pointerleave="onPointer(false)"
        @focusin="onFocusIn"
      >
        <div class="head">
          <span>Downloads</span>
          <button type="button" class="text-button quiet" :disabled="!state.downloads.some((d) => d.state !== 'progress')" @click="clearDownloads()">Clear</button>
        </div>
        <ul role="list">
          <li v-for="d in state.downloads" :key="d.id" role="listitem" :aria-label="d.name" :class="d.state">
            <svg class="file" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" aria-hidden="true">
              <path d="M4 1.75h5L12.25 5v9.25H4z" /><path d="M9 1.75V5h3.25" />
            </svg>
            <div class="name" :title="d.path">{{ d.name }}</div>
            <div v-if="d.state === 'progress'" class="bar" role="progressbar" :aria-label="d.name" aria-valuemin="0" :aria-valuemax="d.total > 0 ? d.total : undefined" :aria-valuenow="d.received">
              <span :style="{ width: d.total > 0 ? (100 * d.received) / d.total + '%' : '30%' }" :class="{ unknown: d.total <= 0 }"></span>
            </div>
            <div class="status">{{ status(d) }}</div>
            <div class="actions">
              <template v-if="d.state === 'done'">
                <!-- Icons, so the name has the row: Open, an arrow out; Show in Finder, a folder. -->
                <button type="button" class="icon-button act" aria-label="Open" title="Open" @click="downloadAction(d.id, 'open')">
                  <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9.5 2.5h4v4M13.5 2.5 8 8M12 9.5v4H2.5V4h4" /></svg>
                </button>
                <button type="button" class="icon-button act" aria-label="Show in Finder" title="Show in Finder" @click="downloadAction(d.id, 'reveal')">
                  <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" aria-hidden="true"><path d="M1.75 3.75h4.5l1.5 1.5h6.5v7.5H1.75z" /></svg>
                </button>
              </template>
              <template v-else-if="d.state === 'progress'">
                <button type="button" class="text-button quiet" @click="downloadAction(d.id, d.paused ? 'resume' : 'pause')">{{ d.paused ? 'Resume' : 'Pause' }}</button>
                <button type="button" class="text-button quiet" @click="downloadAction(d.id, 'cancel')">Cancel</button>
                <!-- Retry throws away what arrived, so it sits apart from the everyday two. -->
                <button type="button" class="text-button quiet apart" @click="downloadAction(d.id, 'retry')">Retry</button>
              </template>
              <template v-else-if="d.url">
                <button type="button" class="text-button quiet" @click="downloadAction(d.id, 'retry')">Retry</button>
              </template>
              <button
                type="button"
                class="icon-button more"
                :aria-expanded="opened.has(d.id)"
                :aria-controls="`download-details-${d.id}`"
                :aria-label="`Details of ${d.name}`"
                title="Details"
                @click="toggle(d.id)"
              >
                <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m3 4.5 3 3 3-3" /></svg>
              </button>
              <!-- A running one has Cancel; the room goes to its name and its buttons. -->
              <button v-if="d.state !== 'progress'" type="button" class="icon-button remove" :aria-label="`Remove ${d.name} from the list`" :title="'Remove from the list'" @click="downloadAction(d.id, 'remove')">
                <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="m2 2 6 6M8 2 2 8" /></svg>
              </button>
            </div>
            <dl v-if="opened.has(d.id)" :id="`download-details-${d.id}`" class="details">
              <dt>From</dt>
              <dd v-if="d.url">
                <span class="value">{{ d.url }}</span>
                <button type="button" class="text-button quiet copy" @click="copyLink(d)">{{ copied === d.id ? 'Copied' : 'Copy link' }}</button>
              </dd>
              <dd v-else><span class="value">{{ d.from || 'This app' }}</span></dd>
              <template v-if="d.path">
                <dt>{{ d.state === 'done' ? 'Saved in' : 'Saving to' }}</dt>
                <dd><span class="value">{{ folder(d.path) }}</span></dd>
              </template>
            </dl>
          </li>
        </ul>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.downloads {
  position: relative;
}

.ring {
  position: absolute;
  inset: 50% auto auto 50%;
  transform: translate(-50%, -50%);
  pointer-events: none;
}

.ring .spin {
  animation: spin 1s linear infinite;
  transform-origin: 13px 13px;
}

@keyframes spin {
  to {
    transform: rotate(270deg);
  }
}

/* Peeking, the list alone is the shell's: the page under the rest stays usable. */
.downloads-peek {
  position: fixed;
  z-index: 100;
}

.downloads-cover {
  position: fixed;
  inset: 0;
  z-index: 100;
}

/* The design system's floating surface, as its menus have it. */
.downloads-panel {
  position: fixed;
  outline: none;
  width: 340px;
  max-height: min(480px, calc(100vh - 80px));
  overflow: auto;
  padding: 4px;
  box-sizing: border-box;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  box-shadow: var(--shadow-float);
  color: var(--text);
  font-size: 13px;
}

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 32px;
  padding: 0 4px 0 12px;
  font-weight: 600;
}

ul {
  margin: 0;
  padding: 0;
  list-style: none;
}

li {
  display: grid;
  grid-template-columns: 16px 1fr auto;
  gap: 2px 10px;
  padding: 8px 4px 8px 12px;
  border-radius: 4px;
}

li + li {
  border-top: 1px solid var(--line);
}

.file {
  grid-row: 1;
  grid-column: 1;
  color: var(--icon, var(--muted));
}

.name {
  grid-column: 2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

li.cancelled .name,
li.failed .name {
  color: var(--muted);
  text-decoration: line-through;
}

.bar {
  grid-column: 2;
  height: 4px;
  overflow: hidden;
  border-radius: 2px;
  background: var(--accent-tint);
}

.bar span {
  display: block;
  height: 100%;
  background: var(--accent);
}

.status {
  grid-column: 2;
  color: var(--muted);
  font-size: 12px;
}

.actions {
  display: flex;
  grid-column: 3;
  grid-row: 1 / span 3;
  align-items: center;
  gap: 4px;
}

/* The design system's quiet text button, at the list's size. */
.text-button {
  height: 26px;
  padding: 0 8px;
  font-size: 12px;
}

.remove,
.more,
.act {
  width: 24px;
  height: 24px;
}

.more svg {
  transition: transform 0.15s;
}

.more[aria-expanded='true'] svg {
  transform: rotate(180deg);
}

/* Opened out, under the row: a label column and its values, each address or folder whole,
   broken wherever it must be. */
.details {
  grid-column: 2 / span 2;
  display: grid;
  grid-template-columns: auto 1fr;
  align-items: baseline;
  gap: 4px 10px;
  margin: 6px 0 2px;
  font-size: 12px;
}

.details dt {
  color: var(--muted);
}

.details dd {
  margin: 0;
  min-width: 0;
}

.details .value {
  overflow-wrap: anywhere;
  user-select: text;
}

.details .copy {
  height: 22px;
  margin-left: 4px;
  padding: 0 6px;
  color: var(--accent);
}

/* Retry, a little apart from Pause and Cancel: it throws away what has arrived. */
.apart {
  margin-left: 12px;
}
</style>
