<script setup>
import { computed } from 'vue';
import { send, setPref, state } from '@/sdk/vue.js';

// A release ready to relaunch into says so once (docs/updates.md): a card over the page's
// bottom-left corner, with Later and Relaunch. Either one records the version as seen in the
// profile's prefs (updateNoticed), so the card shows once a release, not at every start;
// Settings › About still offers it, and shows a download on its way.
// One with no version, as a Flatpak's whose version wasn't found, is seen once as "".
const ready = computed(() => state.update.phase === 'ready');
const version = computed(() => (ready.value ? state.update.version || '' : ''));
const shown = computed(() => ready.value && state.prefs.updateNoticed !== version.value);

function later() {
  setPref('updateNoticed', version.value);
}

function relaunch() {
  setPref('updateNoticed', version.value);
  send({ t: 'relaunch' });
}
</script>

<template>
  <div v-if="shown" v-shell class="update-notice" role="dialog" aria-label="Update ready">
    <h2 class="title">{{ version ? `Project B ${version} is ready` : 'An update is ready' }}</h2>
    <p class="words">Relaunch to update now, or later from Settings&nbsp;›&nbsp;About.</p>
    <div class="buttons">
      <button type="button" class="text-button" @click="later">Later</button>
      <button type="button" class="text-button primary" @click="relaunch">Relaunch</button>
    </div>
  </div>
</template>

<style scoped>
/* Over the page, in its bottom-left corner, as small as its words: a card of the design
   system's, floating, so the page under it stays in use. */
.update-notice {
  position: absolute;
  left: 12px;
  bottom: 12px;
  /* Over the page's notice, as a crashed tab's own notice must not hide it. */
  z-index: 4;
  width: 300px;
  max-width: calc(100% - 24px);
  padding: 14px 16px;
  box-sizing: border-box;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  color: var(--text);
  box-shadow: var(--shadow-float);
  animation: rise var(--motion) var(--ease);
}

.title {
  margin: 0;
  font-size: calc(13 * var(--capped-rem) / 16);
  font-weight: 600;
}

.words {
  margin: 4px 0 12px;
  color: var(--muted);
  font-size: calc(12 * var(--capped-rem) / 16);
  line-height: 1.45;
}

.buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

@keyframes rise {
  from {
    transform: translateY(8px);
    opacity: 0;
  }
}
</style>
