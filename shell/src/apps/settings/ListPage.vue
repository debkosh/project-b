<script setup>
import { computed, inject, provide } from 'vue';
import Cards from './Cards.vue';
import { upTo } from '@/ui/escape.js';
import { lists } from './arrange.js';
import { state } from './state.js';

// One of the long lists on a page of its own (docs/settings.md), with a link back to the
// section that has its row, or to the list from one of its own pages, as a site's in Sites
// (sites/<site>). The list's own heading gives way to the page's, and a list may put its
// buttons at the right of the page's heading line, through the place it names.
const props = defineProps({ list: String, back: Object });
const go = inject('settingsGo');
provide('settingsTitled', true);
provide('settingsListActions', `#list-actions-${props.list}`);
const to = computed(() => (state.section.startsWith(`${props.list}/`) ? { id: props.list, label: lists[props.list].label } : props.back));
</script>

<template>
  <div class="list-page">
    <div class="list-head">
      <button type="button" class="icon-button back" :aria-label="`Back to ${to.label}`" @click="upTo(to.id, go)">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10 3.5 5.5 8l4.5 4.5" /></svg>
      </button>
      <h2 class="list-title">{{ lists[list].label }}</h2>
      <span :id="`list-actions-${list}`" class="list-actions"></span>
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

.list-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

.list-title {
  color: var(--text);
  font-size: calc(17rem / 16);
  font-weight: 600;
}
</style>
