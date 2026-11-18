<script setup>
import { hits, query } from './search.js';

// The line over search's results: how many settings match, or that none does.
</script>

<template>
  <p class="results" role="status">
    <template v-if="hits.all">{{ hits.all }} {{ hits.all === 1 ? 'setting matches' : 'settings match' }} “{{ query.trim() }}”</template>
    <template v-else>No settings match “{{ query.trim() }}”</template>
  </p>
</template>

<style scoped>
.results {
  margin: 12px 0 0;
  color: var(--muted);
  font-size: 13px;
}
</style>
