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
// and in a window with no tabs.
const blank = computed(() => empty.value || (state.active !== 0 && activeTab.value.url === '' && !activeTab.value.loading));
const shown = computed(() => !state.pageFullScreen && (view.box || blank.value));
// Over a page (New Tab), the page stays shown under the box's translucent scrim; the box is
// the shell's, so a press on it is the shell's. Elsewhere there is no page to show.
const overPage = computed(() => view.box && !blank.value);
</script>

<!-- The page hole shows the active tab. With no tabs no page shows there, and the shell
     draws the area itself, as its own: a press there reaches no page and doesn't drag. -->
<template>
  <div class="main">
    <div v-if="!shown || overPage" v-page="state.active" class="page-view"></div>
    <CommandBox v-if="shown" :key="state.active" :over-page="overPage" />
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
