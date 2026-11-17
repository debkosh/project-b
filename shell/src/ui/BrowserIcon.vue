<script setup>
import { computed } from 'vue';
import chrome from './logos/chrome.svg?raw';
import firefox from './logos/firefox.svg?raw';

// A browser's logo: Chrome's and Firefox's ship with us (ui/logos), so they show whether or
// not the app is installed; another browser's is the one Go reads from its app
// (browserdata.Icon), or its first letter when there is none to read.
const props = defineProps({ browser: String, icon: String, title: String });
const logos = { chrome, firefox };
const src = computed(() => (logos[props.browser] ? `data:image/svg+xml,${encodeURIComponent(logos[props.browser])}` : props.icon));
</script>

<template>
  <img v-if="src" class="ui-browser-icon" :src="src" alt="" />
  <span v-else class="ui-browser-icon letter" aria-hidden="true">{{ title.slice(0, 1) }}</span>
</template>

<style>
.ui-browser-icon {
  flex: none;
  width: 32px;
  height: 32px;
}

.ui-browser-icon.letter {
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: var(--accent-soft);
  color: var(--text);
  font-size: 14px;
  font-weight: 600;
}
</style>
