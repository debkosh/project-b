<script setup>
// A browser's own logo, which Go reads from the app installed (browserdata.Icon), or its
// first letter when there is none to read.
defineProps({ icon: String, title: String });
</script>

<template>
  <img v-if="icon" class="ui-browser-icon" :src="icon" alt="" />
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
