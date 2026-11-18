<script setup>
import { marked } from './search.js';

// Text with what search found in it marked.
defineProps({ text: String });
</script>

<template>
  <template v-for="(p, i) in marked(text)" :key="i"><mark v-if="p.hit" class="hit">{{ p.text }}</mark><template v-else>{{ p.text }}</template></template>
</template>
