<script setup>
import { computed } from 'vue';
import arc from './logos/arc.svg?raw';
import brave from './logos/brave.svg?raw';
import chrome from './logos/chrome.svg?raw';
import edge from './logos/edge.svg?raw';
import firefox from './logos/firefox.svg?raw';
import safari from './logos/safari.png?inline';

// A browser's logo, for each browser browserdata reads: they ship with us (ui/logos), the
// same for every profile, whether or not the app is installed. Each page is one file, so
// they go in it as data: URLs.
const props = defineProps({ browser: String });
const svg = (s) => `data:image/svg+xml,${encodeURIComponent(s)}`;
const logos = { arc: svg(arc), brave: svg(brave), chrome: svg(chrome), edge: svg(edge), firefox: svg(firefox), safari };
const src = computed(() => logos[props.browser]);
</script>

<template>
  <img class="ui-browser-icon" :src="src" alt="" />
</template>

<style>
.ui-browser-icon {
  flex: none;
  width: 32px;
  height: 32px;
  object-fit: contain;
}
</style>
