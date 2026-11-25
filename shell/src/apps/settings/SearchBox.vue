<script setup>
import { onMounted, ref } from 'vue';
import { query } from './search.js';

// focus puts the cursor in the field as the page opens, so a person can type what they look
// for at once: a desktop's. A phone's would bring its keyboard up over the list.
const props = defineProps({ focus: Boolean });
const field = ref(null);
onMounted(() => {
  if (props.focus) field.value?.focus({ preventScroll: true });
});

// The field that searches every setting. Escape empties it, and the page goes back to what
// it showed; on an empty field it leaves the field.
function onEscape(e) {
  e.preventDefault();
  if (query.value) query.value = '';
  else e.target.blur();
}
</script>

<template>
  <div class="search-box" role="search">
    <svg class="glass" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M7 2.75a4.25 4.25 0 1 0 0 8.5a4.25 4.25 0 1 0 0-8.5z" /><path d="M10.25 10.25 13.5 13.5" /></svg>
    <input
      ref="field"
      v-model="query"
      class="text-field"
      type="search"
      spellcheck="false"
      aria-label="Search settings"
      placeholder="Search settings"
      @keydown.esc="onEscape"
    />
  </div>
</template>

<style scoped>
.search-box {
  position: relative;
}

.glass {
  position: absolute;
  left: 10px;
  top: 50%;
  translate: 0 -50%;
  color: var(--icon);
  pointer-events: none;
}

.text-field {
  width: 100%;
  padding-left: 32px;
  box-sizing: border-box;
}

/* A finger's target, as tall as the rows under it; the glass stays centred. */
:global(.touch .search-box .text-field) {
  height: 44px;
}
</style>
