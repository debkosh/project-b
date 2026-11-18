<script setup>
import { onMounted } from 'vue';
import ImportWizard from '@/ui/ImportWizard.vue';
import { send } from './state.js';

// Import (docs/onboarding.md): the import wizard, as the welcome has it. Go looks for the
// browsers each time the section opens.
onMounted(() => send({ t: 'findBrowsers' }));
</script>

<template>
  <section aria-label="Import">
    <ImportWizard :send="send" files />
  </section>
</template>
