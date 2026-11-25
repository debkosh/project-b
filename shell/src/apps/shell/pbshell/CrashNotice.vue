<script setup>
import { crashLine, crashPaths, send, state } from '@/sdk/vue.js';

// Says the last run crashed, where its log is and, on the Mac, where macOS's report is, until the user
// closes it. Go decides whether there is one. Share report hands one file with all of it
// to the system's share sheet, beside the button; Save report keeps it as a finished download, which the Downloads panel opens to show.
function close() {
  send({ t: 'dismissCrash' });
}

function share(e) {
  const r = e.currentTarget.getBoundingClientRect();
  send({ t: 'shareReport', rect: { x: r.x, y: r.y, w: r.width, h: r.height } });
}
</script>

<template>
  <div v-if="state.crashLog" v-shell class="crash" role="status">
    <span :title="crashPaths()">Project B crashed last time. {{ crashLine() }}</span>
    <button class="share" type="button" @click="share">Share report</button>
    <button class="share" type="button" @click="send({ t: 'saveReport' })">Save report</button>
    <button class="close" type="button" aria-label="Close" @click="close">
      <svg width="9" height="9" viewBox="0 0 9 9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M7.43.9.75 7.58M.9.75l6.68 6.68"/></svg>
    </button>
  </div>
</template>

<style scoped>
/* A strip of its own above the page, the page's width, with a tab's close button at its
   end. It wraps, so both paths show whole. */
.crash {
  flex: none;
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 28px;
  padding: 3px 12px;
  box-sizing: border-box;
  border-bottom: 1px solid var(--line);
  background: var(--bar);
  color: var(--text);
  font-size: calc(12 * var(--capped-rem) / 16);
  line-height: calc(16 * var(--capped-rem) / 16);
}

.crash span {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}

.share {
  flex: none;
  height: 20px;
  padding: 0 8px;
  border: 1px solid var(--line);
  border-radius: 4px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
  font-size: calc(12 * var(--capped-rem) / 16);
}

.share:hover {
  background: var(--accent-tint);
}

.close {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 16px;
  height: 16px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--muted);
  cursor: pointer;
}

.close:hover {
  background: var(--press);
  color: var(--text);
}

.close:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}
</style>
