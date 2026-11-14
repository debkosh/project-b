<script setup>
import { send, state } from '@/sdk/vue.js';

// Says the last run crashed, where its log is and where macOS's report is, until the user
// closes it. Go decides whether there is one. Share report hands one file with all of it
// to the system's share sheet, beside the button.
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
    <span>Project B crashed last time. The log is at {{ state.crashLog }}, macOS's report at {{ state.crashReport }}</span>
    <button class="share" type="button" @click="share">Share report</button>
    <button class="close" type="button" aria-label="Close" @click="close">
      <svg width="9" height="9" viewBox="0 0 9 9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M7.43.9.75 7.58M.9.75l6.68 6.68"/></svg>
    </button>
  </div>
</template>

<style scoped>
/* The update bubble's look, in the page's top-left corner instead of its bottom-left, with
   a tab's close button. It may take the page's width and wraps, so both paths show whole. */
.crash {
  position: absolute;
  left: 0;
  top: 0;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 22px;
  max-width: 100%;
  padding: 3px 3px 3px 10px;
  box-sizing: border-box;
  border: 1px solid var(--line);
  border-left: 0;
  border-top: 0;
  border-radius: 0 0 6px 0;
  background: var(--bar);
  color: var(--text);
  font-size: 12px;
  line-height: 16px;
}

.crash span {
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
  font-size: 12px;
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
