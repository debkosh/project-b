<script setup>
import { computed } from 'vue';

// A browser's logo, by browserdata's id for it: they ship with us (ui/logos, whose README
// says where each is from), the same for every profile, whether or not the app is
// installed. A file per browser is all a new one needs. One with none of its own shows its
// family's (chrome-beta's is chrome's), and one with neither, as a folder chosen of a
// browser no one named, a plain mark. Each page is one file, so they go in it as data:
// URLs.
const svgs = import.meta.glob('./logos/*.svg', { query: '?raw', import: 'default', eager: true });
const pngs = import.meta.glob('./logos/*.png', { query: '?inline', import: 'default', eager: true });
const logos = {};
for (const [file, svg] of Object.entries(svgs)) logos[file.slice(8, -4)] = `data:image/svg+xml,${encodeURIComponent(svg)}`;
for (const [file, png] of Object.entries(pngs)) logos[file.slice(8, -4)] = png;

const props = defineProps({ browser: String });
const src = computed(() => logos[props.browser] || logos[props.browser?.split('-')[0]]);
</script>

<template>
  <img v-if="src" class="ui-browser-icon" :src="src" alt="" />
  <span v-else class="ui-browser-icon other" aria-hidden="true">…</span>
</template>

<style>
.ui-browser-icon {
  flex: none;
  width: 32px;
  height: 32px;
  object-fit: contain;
}

.ui-browser-icon.other {
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: var(--accent-tint);
  color: var(--muted);
  font-weight: 600;
}
</style>
