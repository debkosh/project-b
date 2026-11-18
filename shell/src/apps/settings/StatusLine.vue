<script setup>
import { computed } from 'vue';
import { phone } from '@/phone.js';
import { send, state } from './state.js';

// The status line at the top of Settings, in both layouts (docs/settings.md): whether the
// app is the default browser, with Make default browser when it isn't, and the version with
// what the update manager is doing and Check for updates, Retry and Cancel while a release
// downloads (docs/ux-guide.md: every progress has both), or Relaunch once one is ready. Go
// owns both; the page shows them.

// What the update manager is doing, or what the check the person asked for found, in one
// plain line. A failure's own words go in the line's tooltip.
const update = computed(() => {
  const s = state.update;
  if (s.check === 'checking') return { text: 'Checking…' };
  if (s.phase === 'downloading') return { text: `Downloading ${s.version}… ${percent.value}%` };
  if (s.phase === 'ready') return { text: phone ? `Install to update to ${s.version}` : `Relaunch to update to ${s.version}` };
  if (s.phase === 'failed') return { text: s.version ? `Couldn't update to ${s.version}.` : "Couldn't update.", detail: s.error, failed: true };
  if (s.check === 'current') return { text: 'Up to date' };
  if (s.check === 'failed') return { text: "Couldn't check for updates.", detail: s.checkError, failed: true };
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
  <section class="status-line" aria-label="Status">
    <span class="part default" :class="{ yes: state.isDefault }">
      <template v-if="state.isDefault">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3.5 8.5 6.5 11.5 12.5 4.5" /></svg>
        Your default browser
      </template>
      <template v-else>
        Not your default browser
        <button type="button" class="text-button" @click="send({ t: 'makeDefault' })">Make default browser</button>
      </template>
    </span>
    <!-- The version, then what the update manager is doing with its buttons: two pieces that
         wrap apart on a narrow page, so the buttons stay on the line of what they act on. -->
    <span class="part version-part">
      <span>Version <span class="version">{{ state.running || 'Development build' }}</span></span>
      <span class="update-part">
        <span class="update" :class="{ failed: update.failed }" role="status" :title="update.detail">{{ update.text }}<span
            v-if="state.update.phase === 'downloading'"
            class="progress"
            role="progressbar"
            aria-label="Update download"
            :aria-valuenow="percent"
            aria-valuemin="0"
            aria-valuemax="100"
            :style="{ width: percent + '%' }"
          ></span></span>
        <!-- A ready release is applied from here: a relaunch on a desktop, Android's installer
             on a phone. Both start with the same message to Go. -->
        <button v-if="state.update.phase === 'ready'" type="button" class="text-button" @click="send({ t: 'relaunch' })">
          {{ phone ? 'Install update' : 'Relaunch' }}
        </button>
        <template v-else-if="state.update.phase === 'downloading'">
          <button type="button" class="text-button quiet" @click="send({ t: 'retryUpdate' })">Retry</button>
          <button type="button" class="text-button quiet" @click="send({ t: 'cancelUpdate' })">Cancel</button>
        </template>
        <button v-else type="button" class="text-button quiet" :disabled="busy" @click="send({ t: 'checkUpdate' })">
          {{ update.failed ? 'Try again' : 'Check for updates' }}
        </button>
      </span>
    </span>
  </section>
</template>

<style scoped>
.status-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 24px;
  margin-bottom: 16px;
  color: var(--muted);
  font-size: 13px;
}

.part {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 8px;
}

.default.yes {
  color: var(--accent);
  font-weight: 500;
}

.update-part {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 8px;
}

.version {
  color: var(--text);
}

.update {
  position: relative;
  white-space: nowrap;
}

.update:empty {
  display: none;
}

.update.failed {
  color: var(--danger);
}

/* The download's progress: a line under what the update manager says, in the accent, as the
   shell's bubble has. It is measured against those words alone, which never wrap, so it stays
   under them however the line wraps. */
.progress {
  position: absolute;
  left: 0;
  bottom: -4px;
  height: 2px;
  background: var(--accent);
}
</style>
