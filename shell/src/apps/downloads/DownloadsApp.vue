<script setup>
import { computed, reactive, watchEffect } from 'vue';
import { paintTheme } from '@/theme.js';
import { phone } from '@/phone.js';
import { act, clear, state } from './state.js';
import { status } from './words.js';
import PhoneDownloads from './PhoneDownloads.vue';

// The Downloads page (docs/daily-driver.md): the profile's downloads, newest first. A
// running one shows how far it has got, with Pause or Resume and Cancel; a finished one
// opens at a tap on its name and shows in the phone's Downloads, or Finder; any can be
// taken off the list, and Clear takes off every finished one. The files stay where they are.
// Each row opens out, at its Details control, to say where it came from, its whole address
// with Copy link, or what the app made it as, and where it is saved.
watchEffect(() => paintTheme(state.tones, state.theme));

const reveal = phone ? 'Show in Downloads' : 'Show in Finder';
const finished = computed(() => state.downloads.some((d) => d.state !== 'progress'));

// The rows opened out, by id, and the one whose link was just copied.
const opened = reactive(new Set());
const copied = reactive({ id: 0 });
let copiedTimer = 0;
const toggle = (id) => (opened.has(id) ? opened.delete(id) : opened.add(id));

function copyLink(d) {
  act(d.id, 'copyLink');
  copied.id = d.id;
  clearTimeout(copiedTimer);
  copiedTimer = setTimeout(() => (copied.id = 0), 1500);
}

// Where a download is saved: a phone's shared Downloads, or the Mac's folder it is in.
const savedIn = (d) => (phone ? 'Downloads' : d.path.slice(0, d.path.lastIndexOf('/')) || '/');

const share = (d) => (d.total > 0 ? Math.min(1, d.received / d.total) : 0);
</script>

<template>
  <!-- A phone has its own, denser page (docs/design-system.md, "On a phone"). -->
  <PhoneDownloads v-if="state.ready && phone" />
  <div v-else-if="state.ready" class="page">
    <h1 class="hidden-heading">Downloads</h1>
    <header class="header column">
      <span class="count">{{ state.downloads.length ? `${state.downloads.length} ${state.downloads.length === 1 ? 'file' : 'files'}` : '' }}</span>
      <button v-if="finished" type="button" class="text-button quiet clear" @click="clear">Clear all</button>
    </header>
    <main class="content"><div class="column">
      <p v-if="!state.downloads.length" class="empty">Files you download show here.</p>
      <ul v-else class="card" role="list">
        <li v-for="d in state.downloads" :key="d.id" class="row" role="listitem" :aria-label="d.name" :class="d.state">
          <svg class="mark" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M7 3.5h6.5L18 8v12.5H7zM13.5 3.5V8H18" /></svg>
          <button v-if="d.state === 'done'" type="button" class="lines go" :title="d.path" @click="act(d.id, 'open')">
            <span class="name">{{ d.name }}</span>
            <span class="status">{{ status(d) }}</span>
          </button>
          <span v-else class="lines">
            <span class="name">{{ d.name }}</span>
            <span class="status">{{ status(d) }}</span>
            <span v-if="d.state === 'progress'" class="bar" :class="{ unknown: d.total <= 0 }"><i :style="{ width: d.total > 0 ? `${share(d) * 100}%` : '' }"></i></span>
          </span>
          <template v-if="d.state === 'progress'">
            <button type="button" class="text-button quiet" @click="act(d.id, d.paused ? 'resume' : 'pause')">{{ d.paused ? 'Resume' : 'Pause' }}</button>
            <button type="button" class="text-button quiet" @click="act(d.id, 'cancel')">Cancel</button>
            <!-- Retry throws away what arrived, so it sits apart from the everyday two. -->
            <button type="button" class="text-button quiet apart" @click="act(d.id, 'retry')">Retry</button>
          </template>
          <template v-else-if="d.state === 'done'">
            <button type="button" class="text-button quiet open" @click="act(d.id, 'open')">Open</button>
            <button type="button" class="text-button quiet" @click="act(d.id, 'reveal')">{{ reveal }}</button>
          </template>
          <template v-else-if="d.url">
            <button type="button" class="text-button quiet" @click="act(d.id, 'retry')">Retry</button>
          </template>
          <button
            type="button"
            class="icon-button more"
            :aria-expanded="opened.has(d.id)"
            :aria-controls="`details-${d.id}`"
            :aria-label="`Details of ${d.name}`"
            title="Details"
            @click="toggle(d.id)"
          >
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m3 4.5 3 3 3-3" /></svg>
          </button>
          <button v-if="d.state !== 'progress'" type="button" class="icon-button remove" :aria-label="`Remove ${d.name} from the list`" title="Remove from the list" @click="act(d.id, 'remove')">
            <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="m2 2 6 6M8 2 2 8" /></svg>
          </button>
          <dl v-if="opened.has(d.id)" :id="`details-${d.id}`" class="details">
            <div>
              <dt>From</dt>
              <dd v-if="d.url"><span class="value">{{ d.url }}</span>
                <button type="button" class="text-button quiet" @click="copyLink(d)">{{ copied.id === d.id ? 'Copied' : 'Copy link' }}</button>
              </dd>
              <dd v-else><span class="value">{{ d.from || 'This app' }}</span></dd>
            </div>
            <div v-if="d.path">
              <dt>{{ d.state === 'done' ? 'Saved in' : 'Saving to' }}</dt>
              <dd><span class="value">{{ savedIn(d) }}</span></dd>
            </div>
          </dl>
        </li>
      </ul>
    </div></main>
  </div>
</template>

<style>
body {
  background: var(--bar);
}

.page {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.header {
  flex: none;
  display: flex;
  align-items: center;
  gap: 12px;
  height: 52px;
  padding: 0 24px;
  box-sizing: border-box;
}

.count {
  flex: 1;
  color: var(--muted);
  font-size: 12px;
}

.content {
  flex: 1;
  overflow: auto;
  padding: 0 24px 24px;
}

.card {
  margin: 0;
  padding: 4px 0;
  list-style: none;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
}

.row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  min-height: 56px;
  padding: 4px 8px 4px 16px;
  box-sizing: border-box;
}

.row + .row {
  border-top: 1px solid var(--line);
}

.mark {
  flex: none;
  color: var(--icon);
}

.cancelled .mark,
.failed .mark {
  color: var(--muted);
}

.lines {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 4px 4px;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
}

.go {
  cursor: pointer;
  border-radius: 6px;
}

.go:hover {
  background: var(--accent-tint);
}

.name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status {
  color: var(--muted);
  font-size: 12px;
}

.failed .status {
  color: var(--danger);
}

.bar {
  position: relative;
  height: 3px;
  margin-top: 4px;
  overflow: hidden;
  border-radius: 2px;
  background: var(--line);
}

.bar i {
  position: absolute;
  inset: 0 auto 0 0;
  background: var(--accent);
}

.bar.unknown i {
  width: 30%;
  animation: slide 1.2s linear infinite;
}

@keyframes slide {
  from { transform: translateX(-100%); }
  to { transform: translateX(340%); }
}

@media (prefers-reduced-motion: reduce) {
  .bar.unknown i { animation: none; }
}

.more svg {
  transition: transform 0.15s;
}

.more[aria-expanded='true'] svg {
  transform: rotate(180deg);
}

/* Opened out: under the name, a label column and its values, each address or folder whole,
   broken wherever it must be. */
.details {
  flex-basis: 100%;
  margin: 0 0 8px;
  padding: 8px 0 0 32px;
  display: grid;
  gap: 6px;
  font-size: 12px;
}

.details > div {
  display: grid;
  grid-template-columns: 72px 1fr;
  align-items: baseline;
  gap: 8px;
}

.details dt {
  color: var(--muted);
}

.details dd {
  margin: 0;
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 8px;
}

.details .value {
  min-width: 0;
  overflow-wrap: anywhere;
  user-select: text;
}

.details .text-button {
  height: 24px;
  padding: 0 6px;
  font-size: 12px;
  color: var(--accent);
}

.open,
.clear {
  color: var(--accent);
  font-weight: 600;
}

.empty {
  margin: 48px 0;
  color: var(--muted);
  text-align: center;
}

.hidden-heading {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}

.column {
  width: 100%;
  max-width: 720px;
  margin: 0 auto;
  box-sizing: border-box;
}

.header.column {
  max-width: 768px;
}

/* A phone: the page's sides are the screen's, each control a finger's 48px, and a row's
   actions under its name, so the name has the width. */
.touch .header,
.touch .content {
  padding-left: 12px;
  padding-right: 12px;
}

.touch .card {
  border-radius: 12px;
}

.touch .row {
  flex-wrap: wrap;
  padding: 8px 4px 4px 12px;
}

.touch .lines {
  flex-basis: calc(100% - 40px);
}

.touch .row .text-button,
.touch .row .icon-button {
  min-width: 48px;
  height: 48px;
}

.touch .row .text-button:first-of-type {
  margin-left: 32px;
}

.touch .row .more {
  margin-left: auto;
}

.touch .details {
  padding-left: 32px;
  font-size: 13px;
}

.touch .details > div {
  grid-template-columns: 1fr;
  gap: 2px;
}

.touch .row .details .text-button {
  height: 40px;
  min-width: 0;
  margin-left: 0;
}

.touch .clear {
  height: 48px;
}

/* Retry, a little apart from Pause and Cancel: it throws away what has arrived. */
.apart {
  margin-left: 12px;
}
</style>
