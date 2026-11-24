<script setup>
import { activeTab, send, tabLabel } from '@/sdk/vue.js';

// One of our tabs (docs/shells.md, rule 19): its icon and title where the address goes, and
// a way to close it, with no field, as an address can't take it anywhere.
</script>

<template>
  <div class="page-title">
    <img v-if="activeTab.favicon" class="icon" :src="activeTab.favicon" alt="" width="16" height="16" />
    <span class="title">{{ tabLabel(activeTab) }}</span>
    <button class="icon-button" type="button" :aria-label="`Close ${tabLabel(activeTab)}`" :title="`Close ${tabLabel(activeTab)}`" @click="send({ t: 'closeTab', id: activeTab.id })">
      <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true"><path d="M1.5 1.5l7 7M8.5 1.5l-7 7" /></svg>
    </button>
  </div>
</template>

<style scoped>
/* The address field's box, quiet: the title in it, the close at its end. */
.page-title {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  height: 32px;
  padding: 0 4px 0 10px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  box-sizing: border-box;
  color: var(--text);
}

.icon {
  flex: none;
}

.title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
