<script setup>
import { computed, watchEffect } from 'vue';
import CrashBar from './CrashBar.vue';
import NavBar from './NavBar.vue';
import TabStrip from './TabStrip.vue';
import UpdateBar from './UpdateBar.vue';
import FindBar from '@/ui/FindBar.vue';
import PageDialog from '@/ui/PageDialog.vue';
import PageMenu from '@/ui/PageMenu.vue';
import PageNotice from '@/ui/PageNotice.vue';
import { activeTab, isNewTab, send, setFrame, state } from '@/sdk/vue.js';

// The tabs on top, the navigation bar under them, any notice under that, then the page,
// inside the window's 4px band. The frame's sides and foot are strips of their own, so
// nothing is painted under the page: it shows through the hole. With no tabs the shell
// draws that area itself, as its own. A page in full screen has the whole window: no band,
// no bars, which are hidden rather than taken away, so each keeps what it holds.
const pageFull = computed(() => state.pageFullScreen !== 0 && state.pageFullScreen === state.active);
watchEffect(() => setFrame(pageFull.value ? 0 : 4));
</script>

<!-- Until Go's snapshot is in, the shell knows nothing of the window: its splash is the
     window's own colour, which the engine paints under the shell, and the app's name. When
     the data comes the UI is made, and the two cross-fade. -->
<template>
  <div class="splash" :aria-busy="!state.loaded">Project B</div>
  <div v-if="state.loaded" class="window">
    <TabStrip v-show="!pageFull" />
    <NavBar v-show="!pageFull" />
    <CrashBar v-show="!pageFull" />
    <UpdateBar v-show="!pageFull" />
    <div class="body">
      <div v-show="!pageFull" class="edge side"></div>
      <div v-page="state.active" class="page">
        <!-- A new tab loads nothing, so no page shows here: a backdrop in the page's colour. -->
        <div v-if="isNewTab(activeTab)" v-shell class="new-tab-backdrop"></div>
        <PageNotice v-if="activeTab.notice" v-shell :tab="activeTab" />
        <FindBar />
        <PageDialog />
      </div>
      <div v-show="!pageFull" class="edge side"></div>
    </div>
    <div v-show="!pageFull" class="edge foot"></div>
    <PageMenu />
  </div>
</template>

<style>
.window {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  transition: opacity var(--motion) var(--ease);
}

/* The UI fades in as it is made, as the splash fades out. */
@starting-style {
  .window {
    opacity: 0;
  }
}

/* The splash is only a picture, so a press goes through it to the UI while it fades; then
   it goes. */
.splash {
  position: fixed;
  inset: 0;
  display: grid;
  place-items: center;
  font: 500 15px/1 system-ui, sans-serif;
  letter-spacing: 0.02em;
  color: rgb(255 255 255 / 0.85);
  user-select: none;
  pointer-events: none;
  transition: opacity var(--motion) var(--ease), display var(--motion) allow-discrete;
}

.splash[aria-busy='false'] {
  display: none;
  opacity: 0;
}

.body {
  display: flex;
  flex: 1;
  min-height: 0;
}

/* The page's area is a layer of its own: what lies over the page (a notice, the find bar, a
   page's dialog) stays under the shell's bars and the menus and panels they open
   (design-system.md, Layers). */
.page {
  position: relative;
  isolation: isolate;
  flex: 1;
  min-width: 0;
}

/* Fox's styles are the whole page's, and TabIcon has a .blank of its own. */
.new-tab-backdrop {
  position: absolute;
  inset: 0;
  background: Canvas;
}

.edge {
  flex: none;
  background: var(--frame);
}

.edge.side {
  width: var(--frame-width);
}

.edge.foot {
  height: var(--frame-width);
}
</style>
