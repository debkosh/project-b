<script setup>
import CrashBar from './CrashBar.vue';
import NavBar from './NavBar.vue';
import TabStrip from './TabStrip.vue';
import UpdateBar from './UpdateBar.vue';
import { setFrame, state } from '@/sdk/vue.js';

// The tabs on top, the navigation bar under them, any notice under that, then the page,
// inside the window's 4px band. The frame's sides and foot are strips of their own, so
// nothing is painted under the page: it shows through the hole.
setFrame(4);
</script>

<!-- Until Go's snapshot is in, the shell knows nothing of the window: its splash is the
     window's own colour, which the engine paints under the shell, and the app's name. When
     the data comes the UI is made, and the two cross-fade. -->
<template>
  <div class="splash" :aria-busy="!state.loaded">Project B</div>
  <div v-if="state.loaded" class="window">
    <TabStrip />
    <NavBar />
    <CrashBar />
    <UpdateBar />
    <div class="body">
      <div class="edge side"></div>
      <div v-page="state.active" class="page"></div>
      <div class="edge side"></div>
    </div>
    <div class="edge foot"></div>
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

.page {
  flex: 1;
  min-width: 0;
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
