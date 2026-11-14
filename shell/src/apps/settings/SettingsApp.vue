<script setup>
import { computed, watchEffect } from 'vue';
import { extensionsSection } from '@extensions';
import { phone } from '@/phone.js';
import { paintTheme } from '@/theme.js';
import Flags from './Flags.vue';
import General from './General.vue';
import Keyboard from './Keyboard.vue';
import Profile from './Profile.vue';
import SiteSettings from './SiteSettings.vue';
import WebFeatures from './WebFeatures.vue';
import Workspaces from './Workspaces.vue';
import { state } from './state.js';

// The settings page: one of our pages, in a tab of ours. Its colours come from the frame
// colour and the theme, as the shell's do.
watchEffect(() => paintTheme(state.tones, state.theme));

// The page shows state.section when it is one of these, else the first. Profile is there
// in a copy on a listed profile; Extensions when the build has them and their flag is on.
const sections = computed(() =>
  [
    { id: 'general', label: 'General', component: General },
    state.profile && { id: 'profile', label: 'Profile', component: Profile },
    { id: 'workspaces', label: 'Workspaces', component: Workspaces },
    { id: 'sites', label: 'Site settings', component: SiteSettings },
    { id: 'keyboard', label: 'Keyboard', component: Keyboard },
    extensionsSection?.shown() && extensionsSection,
    { id: 'flags', label: 'Flags', component: Flags },
    { id: 'webFeatures', label: 'Web features', component: WebFeatures },
  ].filter((s) => s),
);
// A section's address may name more after a slash, which the section reads: sites/<origin>.
const section = computed(() => sections.value.find((s) => s.id === state.section.split('/')[0]) || sections.value[0]);
</script>

<template>
  <div v-if="state.ready" class="page">
    <header class="header">
      <h1>Settings</h1>
    </header>
    <div class="body">
      <!-- A phone has no keyboard shortcuts or extensions, so only General applies there. -->
      <nav v-if="!phone" class="nav" aria-label="Settings sections">
        <button
          v-for="s in sections"
          :key="s.id"
          type="button"
          class="nav-item"
          :aria-current="section === s ? 'page' : undefined"
          @click="state.section = s.id"
        >{{ s.label }}</button>
      </nav>
      <main class="content">
        <General v-if="phone" />
        <component :is="section.component" v-else />
      </main>
    </div>
  </div>
</template>

<style>
/* Surfaces from the Reference preferences design, at a page's size. Type and the controls are
   design.css's; the frame colour's tones come from Go (core/tones). */
body {
  background: var(--bar);
}

.page {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.header {
  flex: none;
  display: flex;
  align-items: center;
  height: 52px;
  padding: 0 16px 0 24px;
  box-sizing: border-box;
}

h1 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.body {
  flex: 1;
  display: flex;
  min-height: 0;
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

.nav-item[aria-current='page'] {
  background: var(--accent-soft);
}

.content {
  flex: 1;
  min-width: 0;
  overflow: auto;
  padding: 8px 32px 32px 8px;
}

.content > * {
  max-width: 720px;
}

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
