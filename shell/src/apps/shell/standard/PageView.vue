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
    <CrashNotice v-if="!state.pageFullScreen" />
    <div class="stage">
      <div v-if="!shown" v-page="state.active" class="page-view"></div>
      <CommandBox v-if="shown" :key="state.active" />
      <PageNotice v-if="activeTab.notice" v-shell :tab="activeTab" />
      <FindBar />
      <PageDialog />
      <UpdateNotice />
    </div>
  </div>
</template>

<style scoped>
/* The last run's crash notice is a strip of its own above the page, which it pushes down,
   so it covers nothing of the page. Under it the page is a plain rectangle: a hole in the
   shell with nothing painted on it. The update bubble is placed against the stage, so it
   sits in the page's corner in either layout. The stage is a layer of its own: what lies
   over the page (a notice, the find bar, a page's dialog) stays under the shell's bars and
   the menus and panels they open (design-system.md, Layers). */
.main {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  min-height: 0;
}

.stage {
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
