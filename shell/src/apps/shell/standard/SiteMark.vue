<script setup>
import { computed } from 'vue';
import { mark } from './bookmarks.js';
import { siteTile } from '@/ui/tint.js';
import { useTints } from '@/ui/useTints.js';

// A bookmark's mark, size pixels square: its open tab's favicon, else its site's letter on
// its site's tile (tint.js), made for the background it sits on, on: 'surface' or 'bar'.
const props = defineProps({ url: String, size: { type: Number, default: 16 }, on: { type: String, default: 'surface' } });
const m = computed(() => mark(props.url));
const tints = useTints();
const tile = computed(() => (tints.value, siteTile(props.url, props.on)));
</script>

<template>
  <img v-if="m.favicon" class="site-mark" :src="m.favicon" alt="" :width="size" :height="size" />
  <span
    v-else
    class="site-mark letter"
    aria-hidden="true"
    :style="{
      background: tile.background,
      boxShadow: `inset 0 0 0 1px ${tile.border}`,
      color: tile.letter,
      width: size + 'px',
      height: size + 'px',
      fontSize: Math.round(size * 0.55) + 'px',
      borderRadius: (size >= 32 ? 8 : 4) + 'px',
    }"
    >{{ m.letter }}</span
  >
</template>

<style scoped>
.site-mark {
  flex: none;
  display: block;
  object-fit: contain;
}

.letter {
  display: grid;
  place-items: center;
  font-weight: 600;
  line-height: 1;
}
</style>
