<script setup>
import { computed } from 'vue';
import { phone } from '@/phone.js';
import Row from './Row.vue';
import { send, state } from './state.js';

// About's first row, in its card (docs/settings.md): the app, the version it runs, and what the update
// manager is doing, with its one button: Check for updates; Cancel and Retry while a release
// downloads (docs/ux-guide.md: every progress has both), its progress a line under the words;
// or Relaunch (Install update on a phone) once one is ready. Go owns it; the page shows it.

// What the update manager is doing, or what the check the person asked for found, in one
// plain line. A failure's own words go in the line's tooltip.
const update = computed(() => {
  const s = state.update;
  if (s.check === 'checking') return { text: 'Checking…' };
  if (s.phase === 'downloading') return { text: `Downloading ${s.version || 'an update'}… ${percent.value}%` };
  if (s.phase === 'ready') return { text: s.version ? `${s.version} is ready` : 'An update is ready', ready: true };
  if (s.phase === 'failed') return { text: s.version ? `Couldn't update to ${s.version}` : "Couldn't update", detail: s.error, failed: true };
  if (s.check === 'current') return { text: 'Up to date' };
  if (s.check === 'failed') return { text: "Couldn't check for updates", detail: s.checkError, failed: true };
  return { text: '' };
});

// How much of the release has arrived, as a whole percentage, while it downloads.
const percent = computed(() => {
  const u = state.update;
  return u.total ? Math.floor((100 * (u.done || 0)) / u.total) : 0;
});

const busy = computed(() => state.update.check === 'checking');
</script>

<template>
  <Row class="version-row" role="region" aria-label="Version" label="Project B" words="version updates check for updates">
    <template #hint>
      <span class="version-line">
        <span>Version <span class="version">{{ state.running || 'Development build' }}</span></span>
        <span v-if="update.text" class="dot" aria-hidden="true">·</span>
        <span v-if="update.text" class="update" :class="{ failed: update.failed, ready: update.ready }" role="status" :title="update.detail">{{ update.text }}<span
            v-if="state.update.phase === 'downloading'"
            class="progress"
            role="progressbar"
            aria-label="Update download"
            :aria-valuenow="percent"
            aria-valuemin="0"
            aria-valuemax="100"
            :style="{ width: percent + '%' }"
          ></span></span>
      </span>
      <!-- Why it failed, in words, on its own line: a title alone shows on no phone. -->
      <span v-if="update.failed && update.detail" class="why">{{ update.detail.charAt(0).toUpperCase() + update.detail.slice(1) }}</span>
    </template>
    <span class="actions">
      <!-- A ready release is applied from here: a relaunch on a desktop, Android's
           installer on a phone. Both start with the same message to Go. -->
      <button v-if="state.update.phase === 'ready'" type="button" class="text-button" @click="send({ t: 'relaunch' })">
        {{ phone ? 'Install update' : 'Relaunch' }}
      </button>
      <template v-else-if="state.update.phase === 'downloading'">
        <!-- Retry last and a little apart, as it throws away what has arrived (docs/ux-guide.md). -->
        <button type="button" class="text-button" @click="send({ t: 'cancelUpdate' })">Cancel</button>
        <button type="button" class="text-button apart" @click="send({ t: 'retryUpdate' })">Retry</button>
      </template>
      <button v-else type="button" class="text-button" :disabled="busy" @click="send({ t: 'checkUpdate' })">
        <!-- Try again where it can help; a release refused waits for a newer one, or this. -->
        {{ update.failed && (state.update.phase !== 'failed' || state.update.retry) ? 'Try again' : 'Check for updates' }}
      </button>
    </span>
  </Row>
</template>

<style scoped>
.version-row :deep(.name) {
  font-size: calc(15rem / 16);
  font-weight: 600;
}

/* The version, then what the update manager is doing: two pieces that wrap apart on a
   narrow page, never inside the update's words, so its progress line stays under them. */
.version-line {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 2px 6px;
}

.version {
  color: var(--text);
}


.update {
  position: relative;
  white-space: nowrap;
}

.why {
  display: block;
  color: var(--danger);
}

.update.failed {
  color: var(--danger);
}

.update.ready {
  color: var(--accent);
  font-weight: 500;
}

.actions {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 8px;
}

.apart {
  margin-left: 8px;
}

/* The download's progress: a line under what the update manager says, in the accent, as the
   shell's bubble has. */
.progress {
  position: absolute;
  left: 0;
  bottom: -3px;
  height: 2px;
  background: var(--accent);
}
</style>
