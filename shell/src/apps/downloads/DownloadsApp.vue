<script setup>
import { computed, watchEffect } from 'vue';
import { paintTheme } from '@/theme.js';
import { phone } from '@/phone.js';
import { act, clear, state } from './state.js';

// The Downloads page (docs/daily-driver.md): the profile's downloads, newest first. A
// running one shows how far it has got, with Pause or Resume and Cancel; a finished one
// opens at a tap on its name and shows in the phone's Downloads, or Finder; any can be
// taken off the list, and Clear takes off every finished one. The files stay where they are.
watchEffect(() => paintTheme(state.tones, state.theme));

const reveal = phone ? 'Show in Downloads' : 'Show in Finder';
const finished = computed(() => state.downloads.some((d) => d.state !== 'progress'));

// A size as a phone or Finder gives it: 1000 to a kilobyte, one decimal place under ten.
function size(n) {
  if (n < 1000) return `${n} B`;
  for (const [unit, by] of [['KB', 1e3], ['MB', 1e6], ['GB', 1e9], ['TB', 1e12]]) {
    const v = n / by;
    if (v < 1000 || unit === 'TB') return `${v < 10 ? +v.toFixed(1) : Math.round(v)} ${unit}`;
  }
}

function status(d) {
  switch (d.state) {
    case 'progress':
      return (d.paused ? 'Paused · ' : '') + (d.total > 0 ? `${size(d.received)} of ${size(d.total)}` : size(d.received));
    case 'done':
      return `${d.total > 0 ? size(d.total) : 'Done'} · ${hostOf(d.url)}`;
    case 'cancelled':
      return 'Cancelled';
    default:
      return 'Failed';
  }
}

const hostOf = (u) => {
  try {
    return new URL(u).host.replace(/^www\./, '');
  } catch {
    return '';
  }
};
const share = (d) => (d.total > 0 ? Math.min(1, d.received / d.total) : 0);
</script>

<template>
  <div v-if="state.ready" class="page">
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
          </template>
          <template v-else-if="d.state === 'done'">
            <button type="button" class="text-button quiet open" @click="act(d.id, 'open')">Open</button>
            <button type="button" class="text-button quiet" @click="act(d.id, 'reveal')">{{ reveal }}</button>
          </template>
          <button v-if="d.state !== 'progress'" type="button" class="icon-button remove" :aria-label="`Remove ${d.name} from the list`" title="Remove from the list" @click="act(d.id, 'remove')">
            <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="m2 2 6 6M8 2 2 8" /></svg>
          </button>
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

.touch .row .remove {
  margin-left: auto;
}

.touch .clear {
  height: 48px;
}
</style>
