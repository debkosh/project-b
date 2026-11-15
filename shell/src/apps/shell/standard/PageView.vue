<script setup>
import { computed } from 'vue';
import CrashNotice from './CrashNotice.vue';
import FindBar from '@/ui/FindBar.vue';
import PageDialog from '@/ui/PageDialog.vue';
import UpdateStatus from './UpdateStatus.vue';
import PageNotice from '@/ui/PageNotice.vue';
import CommandBox from './CommandBox.vue';
import { empty, view } from './state.js';
import { activeTab, state } from '@/sdk/vue.js';

// The command box shows over the page area: opened by New Tab, on a tab with nothing in it,
// and in a window with no tabs; no page shows under it.
const blank = computed(() => empty.value || (state.active !== 0 && activeTab.value.url === '' && !activeTab.value.loading));
const shown = computed(() => !state.pageFullScreen && (view.box || blank.value));
</script>

<!-- The page hole shows the active tab. With no tabs no page shows there, and the shell
     draws the area itself, as its own: a press there reaches no page and doesn't drag. -->
<template>
  <div class="main">
    <!-- While the command box shows, it has the page area to itself: no page is shown under
         it, so every press there is the shell's, in the real window as on the mock. -->
    <div v-if="!shown" v-page="state.active" class="page-view"></div>
    <CommandBox v-if="shown" :key="state.active" :over-page="view.box && !blank" />
    <PageNotice v-if="activeTab.notice" v-shell :tab="activeTab" />
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

</style>
