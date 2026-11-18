<script setup>
import { inject, provide } from 'vue';
import Cards from './Cards.vue';
import { lists } from './arrange.js';

// One of the long lists on a page of its own (docs/settings.md), with a link back to the
// section that has its row. The list's own heading gives way to the page's.
const props = defineProps({ list: String, back: Object });
const go = inject('settingsGo');
provide('settingsListPage', true);
</script>

<template>
  <div class="list-page">
    <div class="list-head">
      <button type="button" class="icon-button back" :aria-label="`Back to ${back.label}`" @click="go(back.id)">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10 3.5 5.5 8l4.5 4.5" /></svg>
      </button>
      <h2 class="list-title">{{ lists[list].label }}</h2>
    </div>
    <Cards :section="{ id: list, label: lists[list].label, cards: [props.list] }" />
  </div>
</template>

<style scoped>
.list-head {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  margin: 0 0 8px -8px;
}

.list-title {
  color: var(--text);
  font-size: 17px;
  font-weight: 600;
}
</style>
