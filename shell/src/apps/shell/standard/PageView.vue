<script setup>
import { computed } from 'vue';
import CrashNotice from './CrashNotice.vue';
import FindBar from '@/ui/FindBar.vue';
import PageDialog from '@/ui/PageDialog.vue';
import UpdateNotice from './UpdateNotice.vue';
import PageNotice from '@/ui/PageNotice.vue';
import CommandBox from './CommandBox.vue';
import { activeTab, isNewTab, state } from '@/sdk/vue.js';

// The command box shows where the page goes on a new tab, one with nothing loaded, which has
// no page to show.
const shown = computed(() => !state.pageFullScreen && isNewTab(activeTab.value));
</script>

<!-- The page hole shows the active tab. With no tabs no page shows there, and the shell
     draws the area itself, as its own: a press there reaches no page and doesn't drag. -->
<template>
  <div class="main">
    <div v-if="!shown" v-page="state.active" class="page-view"></div>
    <CommandBox v-if="shown" :key="state.active" />
    <PageNotice v-if="activeTab.notice" v-shell :tab="activeTab" />
    <CrashNotice />
    <FindBar />
    <PageDialog />
    <UpdateNotice />
  </div>
</template>

<style scoped>
/* The page is a plain rectangle: a hole in the shell with nothing painted on it. The
   crash notice and the update bubble are placed against this box, so
   each sits in the page's corner in either layout. It is a layer of its own: what lies over
   the page (a notice, the find bar, a page's dialog) stays under the shell's bars and the
   menus and panels they open (design-system.md, Layers). */
.main {
  position: relative;
  isolation: isolate;
  flex: 1;
  min-width: 0;
  min-height: 0;
}

.page-view {
  height: 100%;
}

</style>
