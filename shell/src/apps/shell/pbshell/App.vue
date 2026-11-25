<script setup>
import { computed, watchEffect } from 'vue';
import BookmarkBar from './BookmarkBar.vue';
import MenuBar from '@/ui/MenuBar.vue';
import FloatMenu from './FloatMenu.vue';
import PasswordOffer from './PasswordOffer.vue';
import PageMenu from '@/ui/PageMenu.vue';
import TabMenu from './TabMenu.vue';
import PageView from './PageView.vue';
import Sidebar from './Sidebar.vue';
import TabStrip from './TabStrip.vue';
import TopBar from './TopBar.vue';
import { setFrame, state } from '@/sdk/vue.js';
import { tabsOnTop } from './state.js';

// Every layout shows the same tabs from the same state; only where they sit differs.

// The window draws a 4px band of the frame colour around its edge, and the shell paints
// its own strips in that colour and width, as CSS variables on the root, so there is one
// source for both; every other tone of the shell derives from that colour. The window's
// corner radius comes from the managers. The curve over the corners is the window's own
// border, drawn by the engine. Light or dark follows Go's theme pref.
// A page in full screen has the whole window: no band, no bars, nothing over it.
const pageFull = computed(() => state.pageFullScreen !== 0 && state.pageFullScreen === state.active);
watchEffect(() => setFrame(pageFull.value ? 0 : 4));
// The welcome of a first start has the window to itself but for the frame's band, which
// takes its colour as it is picked, and a strip across the top to move the window by,
// over the page's top, which it keeps clear (docs/onboarding.md).
const welcome = computed(() => state.welcome !== 0 && state.welcome === state.active);
const bare = computed(() => pageFull.value || welcome.value);
watchEffect(() => {
  document.documentElement.style.setProperty('--window-radius', state.cornerRadius + 'px');
});
</script>

<!-- Every component the shell tracks starts as frame colour and draws its own surface on
     top; the frame is what shows where nothing was drawn. The page is a plain rectangle
     between them, with the left, right and bottom edges as plain strips of their own. -->
<template>
  <!-- Until Go's snapshot is in, the shell knows nothing of the window: its splash is the
       window's own colour, which the engine paints under the shell, and the app's name.
       When the data comes the UI is made, and the two cross-fade. -->
  <div class="splash" :aria-busy="!state.loaded">Project B</div>
  <div v-if="state.loaded" class="app" :class="{ 'page-full': pageFull }">
    <!-- Hidden, not taken away, while a page fills the screen or the welcome the window:
         each keeps what it holds, as the address it shows. -->
    <MenuBar :hidden="bare" />
    <TabStrip v-if="tabsOnTop" v-show="!bare" />
    <TopBar v-show="!bare" />
    <div v-if="welcome" class="edge top"></div>
    <div v-if="welcome" v-drag class="welcome-drag"></div>
    <div class="body">
      <div v-show="!pageFull" class="edge left"></div>
      <Sidebar v-if="!tabsOnTop" v-show="!bare" />
      <!-- The bookmarks bar, when on, takes the top of the page's column, so the page is
           laid out under it and its region measured there. -->
      <div class="pages">
        <BookmarkBar v-if="state.prefs.bookmarksBar" v-show="!bare" />
        <PageView />
      </div>
      <div v-show="!pageFull" class="edge right"></div>
    </div>
    <div v-show="!pageFull" class="edge bottom"></div>
    <FloatMenu />
    <PasswordOffer />
    <PageMenu />
    <TabMenu />
  </div>
</template>

<style>
/* Colours from the browser design. The frame's colour and width come from Go with the
   snapshot, the same values the window paints natively, and the frame colour's tones
   (--frame, --accent, --bar, --icon, --accent-soft, --accent-tint), which Go makes from it
   (core/tones), are set on the root by the SDK. Icons and the sidebar's New Tab take a deeper ink, so the frame is the one
   thing in the full accent. The root has no background: the page shows through wherever
   nothing is painted. */
.app {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  transition: opacity var(--motion) var(--ease);
}

/* The UI fades in as it is made, as the splash fades out. */
@starting-style {
  .app {
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
  font: 500 calc(15rem / 16)/1 system-ui, sans-serif;
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

.pages {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.edge {
  flex: none;
  background: var(--frame);
}

.edge.left,
.edge.right {
  width: var(--frame-width);
}

.edge.bottom,
.edge.top {
  height: var(--frame-width);
}

/* Over the welcome's top: nothing painted, so the page shows through, and the window moves
   from it. */
.welcome-drag {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 40px;
}

/* Touch: a finger and no mouse, a tablet's (docs/large-screens.md, Standard's touch
   density). Targets grow to 44px, the icons stay their size, and a mouse brings the
   desktop's density back. Each part grows its own rows; the buttons grow here. */
@media (pointer: coarse) and (not (any-pointer: fine)) {
  .icon-button {
    width: 44px;
    height: 44px;
  }
  /* What the bar opens is touch-sized too: the ☰ menu and the right-click menus, with no
     keys shown, as a finger has none. */
  .ui-menu.ui-context-menu .ui-menu-item {
    height: 44px;
  }
  .ui-menu-key {
    display: none;
  }
}
</style>
