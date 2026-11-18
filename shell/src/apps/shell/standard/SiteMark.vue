<script setup>
import { computed } from 'vue';
import { mark } from './bookmarks.js';

// A bookmark's mark, size pixels square: its open tab's favicon, else its site's letter on
// the site's own colour (bookmarks.js).
const props = defineProps({ url: String, size: { type: Number, default: 16 } });
const m = computed(() => mark(props.url));
</script>

<template>
  <img v-if="m.favicon" class="site-mark" :src="m.favicon" alt="" :width="size" :height="size" />
  <span
    v-else
    class="site-mark letter"
    aria-hidden="true"
    :style="{ '--hue': m.hue, width: size + 'px', height: size + 'px', fontSize: Math.round(size * 0.55) + 'px', borderRadius: (size >= 32 ? 8 : 4) + 'px' }"
    >{{ m.letter }}</span
  >
</template>

<style scoped>
.site-mark {
  flex: none;
  display: block;
  object-fit: contain;
}

/* The site's colour, as the Mobile shell's tiles have it: one hue per host, deep enough
   for white letters in light and dark. */
.letter {
  display: grid;
  place-items: center;
  background: hsl(var(--hue) 45% 42%);
  color: #fff;
  font-weight: 600;
  line-height: 1;
}
</style>
