<script setup>
import { phone } from '@/phone.js';
import Segmented from '@/ui/Segmented.vue';
import Swatches from '@/ui/Swatches.vue';
import { framePresets } from '@/ui/colours.js';
import Row from './Row.vue';
import { send, state } from './state.js';

// The settings that are one row each, by part: what Go owns, shown, and the user's changes
// sent back. A row changes only when Go says so.
defineProps({ part: String });

const themes = [
  { value: 'system', label: 'System' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
];

// A phone's address bar: at the foot, by the thumb, or on top.
const positions = [
  { value: 'top', label: 'Top' },
  { value: 'bottom', label: 'Bottom' },
];

// What a start opens: the windows and tabs as they were, or a window opened fresh, as the
// shell opens one: Standard on its command box, another shell on a new tab.
const starts = [
  { value: 'continue', label: 'Where you left off' },
  { value: 'fresh', label: 'Start fresh' },
];

const layouts = [
  { value: 'vertical', label: 'Vertical' },
  { value: 'horizontal', label: 'Horizontal' },
];

</script>

<template>
  <Row v-if="part === 'theme'" label="Theme">
    <Segmented label="Theme" :options="themes" :model-value="state.theme" @update:model-value="send({ t: 'setTheme', text: $event })" />
  </Row>

  <Row v-else-if="part === 'frameColour'" label="Frame colour" class="colours">
    <Swatches
      label="Frame colour"
      custom
      :colors="framePresets"
      :model-value="state.windowColor"
      @update:model-value="send({ t: 'setFrameColor', text: $event })"
    />
  </Row>

  <Row v-else-if="part === 'tabLayout'" label="Tab layout">
    <Segmented label="Tab layout" :options="layouts" :model-value="state.tabLayout" @update:model-value="send({ t: 'setTabLayout', text: $event })" />
  </Row>

  <Row v-else-if="part === 'addressBar'" label="Address bar">
    <Segmented label="Address bar" :options="positions" :model-value="state.addressBarPosition" @update:model-value="send({ t: 'setAddressBarPosition', text: $event })" />
  </Row>

  <!-- A phone's shells are its own, a few, side by side; a desktop's are the profile's,
       and every window loads the one chosen at once. -->
  <Row v-else-if="part === 'shell' && phone" label="Shell">
    <Segmented
      label="Shell"
      :options="state.phoneShells.shells.map((s) => ({ value: s.id, label: s.name }))"
      :model-value="state.phoneShells.shell"
      @update:model-value="send({ t: 'setShell', text: $event })"
    />
  </Row>
  <Row v-else-if="part === 'shell'" label="Shell" hint="Every window loads it" for="profile-shell">
    <select id="profile-shell" :value="state.profile.shell" @change="send({ t: 'setShell', text: $event.target.value })">
      <option v-for="s in state.profile.shells" :key="s.id" :value="s.id">{{ s.name }}</option>
    </select>
  </Row>

  <Row v-else-if="part === 'start'" label="On start, open">
    <Segmented label="On start, open" :options="starts" :model-value="state.start" @update:model-value="send({ t: 'setStart', text: $event })" />
  </Row>

  <Row v-else-if="part === 'searchEngine'" label="Search engine" for="search-engine">
    <select id="search-engine" :value="state.searchEngine" @change="send({ t: 'setSearchEngine', text: $event.target.value })">
      <option v-for="e in state.searchEngines" :key="e.id" :value="e.id">{{ e.name }}</option>
    </select>
  </Row>

  <Row v-else-if="part === 'autoUpdate'" tag="label" label="Automatic updates" hint="Updates download by themselves, and apply at the next start">
    <input
      type="checkbox"
      role="switch"
      class="switch"
      :checked="state.autoUpdate"
      :aria-checked="String(state.autoUpdate)"
      @change="send({ t: 'setAutoUpdate', on: $event.target.checked })"
    />
  </Row>

  <!-- The welcome again: marked unfinished, and the app closes, for the next start to open
       it as the first did. -->
  <Row
    v-else-if="part === 'runSetup'"
    label="Run setup again"
    hint="Closes the app. Open it again for the welcome over the whole window, as on a first start; your tabs come back under it as On start, open says."
  >
    <button type="button" class="text-button" @click="send({ t: 'welcomeAgain' })">Close the app</button>
  </Row>
</template>

<style scoped>
/* On a phone the colours take a line of their own. */
.touch .colours :deep(.ui-swatches) {
  flex-basis: 100%;
  flex-wrap: wrap;
}
</style>
