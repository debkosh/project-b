<script setup>
import { computed } from 'vue';
import { send, state } from '@/sdk/vue.js';

// The update manager's state, shown the way a browser shows a link's address: a small
// bubble in the page's bottom-left corner, there only while something is happening. While a
// release downloads it has Retry and Cancel, as every progress does (docs/ux-guide.md).
function relaunch() {
  send({ t: 'relaunch' });
}

// How much of the release has arrived, as a whole percentage, while it downloads.
const percent = computed(() => {
  const u = state.update;
  return u.total ? Math.floor((100 * (u.done || 0)) / u.total) : 0;
});
</script>

<template>
  <div v-if="state.update.phase" v-shell class="update" :class="state.update.phase">
    <template v-if="state.update.phase === 'downloading'">
      <span role="status">Downloading Project B {{ state.update.version }}… {{ percent }}%</span>
      <span
        class="progress"
        role="progressbar"
        aria-label="Update download"
        :aria-valuenow="percent"
        aria-valuemin="0"
        aria-valuemax="100"
        :style="{ width: percent + '%' }"
      ></span>
      <button type="button" class="act" @click="send({ t: 'retryUpdate' })">Retry</button>
      <button type="button" class="act" @click="send({ t: 'cancelUpdate' })">Cancel</button>
    </template>
    <button v-else-if="state.update.phase === 'ready'" type="button" @click="relaunch">
      Project B {{ state.update.version }} is ready. <b>Relaunch to update</b>
    </button>
    <template v-else>
      <span role="status" :title="state.update.error">Update{{ state.update.version ? ' to ' + state.update.version : '' }} failed: {{ state.update.error }}</span>
      <button type="button" class="act" @click="send({ t: 'retryUpdate' })">Try again</button>
    </template>
  </div>
</template>

<style scoped>
/* Over the page, flush with the corner, taking no room: the page's own bottom-left corner
   is where every browser puts this bubble. A hairline and the bar colour, no shadow. */
.update {
  position: absolute;
  left: 0;
  bottom: 0;
  /* Over the page's notice, as a crashed tab's own notice must not hide it. */
  z-index: 3;
  display: flex;
  align-items: center;
  height: 22px;
  max-width: 70%;
  padding: 0 10px;
  box-sizing: border-box;
  border: 1px solid var(--line);
  border-left: 0;
  border-bottom: 0;
  border-radius: 0 6px 0 0;
  background: var(--bar);
  color: var(--text);
  font-size: 12px;
  white-space: nowrap;
}

.update > * {
  overflow: hidden;
  text-overflow: ellipsis;
}

/* The download's progress: a line along the bubble's foot, in the accent. */
.progress {
  position: absolute;
  left: 0;
  bottom: 0;
  height: 2px;
  background: var(--accent);
}

.update.failed {
  color: var(--danger);
}

button {
  padding: 0;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  cursor: pointer;
}

button b {
  color: var(--icon);
  font-weight: 500;
}

button:hover b {
  text-decoration: underline;
}

/* Retry and Cancel, after the percentage: quiet words in the link colour, never cut. */
.update > .act {
  flex: none;
  margin-left: 10px;
  color: var(--icon);
  font-weight: 500;
}

.act:hover {
  text-decoration: underline;
}

button:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
  border-radius: 4px;
}
</style>
