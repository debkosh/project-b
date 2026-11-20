<script setup>
import { notes } from './notes.js';

// About's release notes (docs/settings.md): what changed in this release, under its version,
// the notes built into the page with it.
</script>

<template>
  <section class="release-notes" aria-label="Release notes">
    <h2>What's new in {{ notes.version }}</h2>
    <div class="card note">
      <ul class="changes">
        <li v-for="(c, i) in notes.changes" :key="i">{{ c }}</li>
      </ul>
    </div>
  </section>
</template>

<style scoped>
.note {
  padding: 12px 16px 12px 0;
}

.changes {
  margin: 0;
  padding-left: 34px;
  color: var(--text);
  line-height: 1.5;
}

.changes li + li {
  margin-top: 6px;
}
</style>
