<script setup>
import { send, state } from './state.js';

// The status line at the top of Settings, in both layouts (docs/settings.md): whether the
// app is the default browser, with Make default browser when it isn't. Go owns it; the page
// shows it. The version and its updates are About's (Version.vue).
</script>

<template>
  <section class="status-line" aria-label="Status">
    <span class="part default" :class="{ yes: state.isDefault }">
      <template v-if="state.isDefault">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3.5 8.5 6.5 11.5 12.5 4.5" /></svg>
        Your default browser
      </template>
      <template v-else>
        Not your default browser
        <button type="button" class="text-button" @click="send({ t: 'makeDefault' })">Make default browser</button>
      </template>
    </span>
  </section>
</template>

<style scoped>
.status-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 24px;
  margin-bottom: 16px;
  color: var(--muted);
  font-size: 13px;
}

.part {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 8px;
}

.default.yes {
  color: var(--accent);
  font-weight: 500;
}
</style>
