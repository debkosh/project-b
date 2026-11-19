<script setup>
// The line icons of our pages on a phone: 20px on a 24px box at a 1.5px stroke, in the
// colour around them, as the Mobile shell's (shell/src/apps/shell/mobile/Icon.svelte).
const props = defineProps({ name: String, size: { type: Number, default: 20 } });
const paths = {
  back: 'M15 5l-7 7 7 7',
  search: 'M10.5 17a6.5 6.5 0 1 0 0-13 6.5 6.5 0 0 0 0 13zM15.5 15.5L20 20',
  more: 'M12 5.5h.01M12 12h.01M12 18.5h.01',
  close: 'M7 7l10 10M17 7L7 17',
  file: 'M7 3.5h6.5L18 8v12.5H7zM13.5 3.5V8H18',
  image: 'M4 5h16v14H4zM4 16l5-5 4 4 2-2 5 5M15.5 9.5h.01',
  video: 'M4 6h12v12H4zM16 10l4-2.5v9L16 14',
  audio: 'M9 17.5V6l10-2v11.5M9 17.5a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0zM19 15.5a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0z',
  archive: 'M5 4h14v16H5zM11 4v2h2v2h-2v2h2v2h-2',
  share: 'M12 15V4M8 8l4-4 4 4M5 12v7h14v-7',
  folder: 'M4 6.5h6l2 2h8v10H4z',
  link: 'M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1',
  remove: 'M5 12h14',
  pause: 'M9 6v12M15 6v12',
  play: 'M8 5.5l10 6.5-10 6.5z',
  stop: 'M7 7h10v10H7z',
  retry: 'M19 12a7 7 0 1 1-2.05-4.95M19 4.5V8h-3.5',
  trash: 'M5 7h14M10 7V5h4v2M7 7l1 12h8l1-12',
};
</script>

<template>
  <svg :width="props.size" :height="props.size" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path :d="paths[props.name]" :stroke-width="props.name === 'more' ? 2.5 : 1.5" />
  </svg>
</template>
