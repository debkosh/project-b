<script setup>
import CrashNotice from './CrashNotice.vue';
import FindBar from './FindBar.vue';
import PageDialog from './PageDialog.vue';
import UpdateStatus from './UpdateStatus.vue';
import { shown } from '@/keys.js';
import { send, state } from '@/sdk/vue.js';
import { empty } from './state.js';
</script>

<!-- The page hole shows the active tab. With no tabs no page shows there, and the shell
     draws the area itself, as its own: a press there reaches no page and doesn't drag. -->
<template>
  <div class="main">
    <div v-page="state.active" class="page-view"></div>
    <div v-if="empty" v-shell class="empty">
      <p>No tabs</p>
      <button class="text-button" type="button" @click="send({ t: 'newTab' })">
        New Tab<span v-if="state.keys['tab.new']" class="key" aria-hidden="true">{{ shown(state.keys['tab.new'], state.platform) }}</span>
      </button>
    </div>
    <CrashNotice />
    <FindBar />
    <PageDialog />
    <UpdateStatus />
  </div>
</template>

<style scoped>
/* The page is a plain rectangle: a hole in the shell with nothing painted on it. The
   crash notice and the update bubble are placed against this box, so
   each sits in the page's corner in either layout. */
.main {
  position: relative;
  flex: 1;
  min-width: 0;
}

.page-view {
  height: 100%;
}

/* Where the page would be, in the surface a card has, flat as the page it stands for. */
.empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  background: var(--surface);
  color: var(--text);
}

.empty p {
  margin: 0;
  color: var(--muted);
  font-weight: 500;
}

.key {
  margin-left: 8px;
  color: var(--muted);
  font-weight: 400;
}

</style>
