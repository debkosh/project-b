<script setup>
import { computed, provide, ref, watch, watchEffect } from 'vue';
import { paintTheme } from '@/theme.js';
import PageLayout from './PageLayout.vue';
import SectionsLayout from './SectionsLayout.vue';
import { state } from './state.js';

// The settings page: one of our pages, in a tab of ours. Its colours come from the frame
// colour and the theme, as the shell's do.
watchEffect(() => paintTheme(state.tones, state.theme));

// What the page shows is state.section, an address (docs/settings.md): a section, a part
// or one of the long lists, which Go may name as it opens the page. go shows one, and
// counts a jump, so asking for the one showing goes to it again.
const jumps = ref(0);
provide('settingsJumps', jumps);
const go = (address) => {
  state.section = address;
  jumps.value++;
};
provide('settingsGo', go);

// The layout is the Settings layout flag's, a live one (docs/flags.md): Go sends the flags
// again as it changes, and the page takes the other layout at once, at the same address.
// Off is the page.
const layout = computed(() => (state.ready && state.flags.find((f) => f.id === 'settingsLayout')?.value === 'sections' ? SectionsLayout : PageLayout));
// The flag is set in Flags, so the other layout shows Flags, where the person still is. The
// snapshot's layout is no change.
watch(
  () => state.ready && layout.value,
  (now, was) => was && go('flags'),
);
</script>

<template>
  <div v-if="state.ready" class="page">
    <!-- The page's name is the tab's title, which the shell shows; the heading is kept for
         a screen reader, and out of sight. -->
    <h1 class="hidden-heading">Settings</h1>
    <component :is="layout" />
  </div>
</template>

<style>
/* Surfaces from the preferences design, at a page's size. Type and the controls are
   design.css's; the frame colour's tones come from Go (core/tones). */
body {
  background: var(--bar);
}

.page {
  display: flex;
  flex-direction: column;
  height: 100%;
}


.hidden-heading {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}

.body {
  flex: 1;
  display: flex;
  min-height: 0;
  padding-top: 16px;
  box-sizing: border-box;
}

.nav {
  flex: none;
  width: 200px;
  padding: 8px 12px;
  box-sizing: border-box;
}

.nav-item {
  display: block;
  width: 100%;
  height: 34px;
  padding: 0 12px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: inherit;
  font: inherit;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
}

.nav-item:hover {
  background: var(--accent-tint);
}

.nav-item[aria-current] {
  background: var(--accent-soft);
}

.content {
  flex: 1;
  min-width: 0;
  overflow: auto;
  padding: 8px 32px 32px 8px;
}

.column {
  max-width: 720px;
}

/* With no list of sections at its left, a phone's cards keep the same gutter each side. */
.content.phone {
  padding: 8px 16px 32px;
}

.content.phone .column {
  max-width: none;
}

.block + .block,
section + section {
  margin-top: 24px;
}

/* A heading row is 30px high, with a button on its right or none. */
h2 {
  margin: 0;
  line-height: 30px;
  font-size: 13px;
  font-weight: 500;
  color: var(--muted);
}

.card {
  margin-top: 8px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
}

.row {
  display: flex;
  align-items: center;
  gap: 16px;
  min-height: 48px;
  padding: 8px 16px;
  box-sizing: border-box;
}

.row + .row {
  border-top: 1px solid var(--line);
}

.label {
  flex: 1;
  min-width: 0;
  font-weight: 500;
}

.nav-item:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

/* Fingers need bigger targets than a mouse does, and a touch screen has no hover. */
.touch .row {
  flex-wrap: wrap;
  row-gap: 8px;
}

</style>
