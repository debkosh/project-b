<script setup>
import PaneTree from './PaneTree.vue';
import PromptLine from './PromptLine.vue';
import TopStrip from './TopStrip.vue';
import { connect, setFrame, state } from '@/sdk/vue.js';
import { focusToward, follow, load, resizeToward, shown, split, zoom, zoomed } from './panes.js';
import { prompt } from './prompt.js';

// A terminal's window: a strip of windows on top, the shown window's panes in the body, and
// a prompt line at the foot that is the active pane's address bar, inside the window's
// 4px band.
setFrame(4);

// Its own actions, as the registry lists them, which Go sends while this shell is on: the
// splits, as the panes' buttons do; moving to the pane beside, by Ctrl+Cmd and an arrow;
// moving a divider, with Shift too; and zooming the active pane.
const actions = {
  'terminal.splitRight': () => split(state.active, 'row'),
  'terminal.splitDown': () => split(state.active, 'column'),
  'terminal.focusLeft': () => focusToward('left'),
  'terminal.focusRight': () => focusToward('right'),
  'terminal.focusUp': () => focusToward('up'),
  'terminal.focusDown': () => focusToward('down'),
  'terminal.resizeLeft': () => resizeToward('left'),
  'terminal.resizeRight': () => resizeToward('right'),
  'terminal.resizeUp': () => resizeToward('up'),
  'terminal.resizeDown': () => resizeToward('down'),
  'terminal.zoom': () => zoom(),
};

connect({
  snapshot: () => load(state.data),
  tabs: follow,
  suggestions: (m) => prompt.suggestions(m.id, m.items || []),
  focusAddressBar: () => prompt.focus(),
  action: (m) => actions[m.id](),
});
</script>

<!-- The frame's bands are painted as the window paints them, and nothing is painted under
     the panes, so their pages show through. Until Go's snapshot is in, the shell knows
     nothing of the window: its splash is the window's own colour, which the engine paints
     under the shell, and the app's name. When the data comes the UI is made, and the two
     cross-fade. -->
<template>
  <div class="splash" :aria-busy="!state.loaded">Project B</div>
  <div v-if="state.loaded" class="app">
    <div v-drag class="band across"></div>
    <div class="middle">
      <div class="band side"></div>
      <div class="inner">
        <TopStrip />
        <div class="body">
          <PaneTree v-if="shown" :node="zoomed ? { tab: zoomed } : shown" />
        </div>
        <PromptLine />
      </div>
      <div class="band side"></div>
    </div>
    <div class="band across"></div>
  </div>
</template>

<style>
.app {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  font-family: ui-monospace, 'SF Mono', Menlo, monospace;
  font-size: 12px;
  line-height: 16px;
  transition: opacity var(--motion) var(--ease);
}

/* The UI fades in as it is made, as the splash fades out. */
@starting-style {
  .app {
    opacity: 0;
  }
}

/* The splash is only a picture, so a press goes through it to the UI while it fades; then
   it goes. In the terminal's own type. */
.splash {
  position: fixed;
  inset: 0;
  display: grid;
  place-items: center;
  font: 500 15px/1 ui-monospace, 'SF Mono', Menlo, monospace;
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

.band {
  flex: none;
  background: var(--frame);
}

.band.across {
  height: var(--frame-width);
}

.band.side {
  width: var(--frame-width);
}

.middle {
  display: flex;
  flex: 1;
  min-height: 0;
}

.inner {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.body {
  display: flex;
  flex: 1;
  min-height: 0;
}
</style>
