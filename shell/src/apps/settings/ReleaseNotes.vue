<script setup>
import { computed } from 'vue';
import { notes, parse } from './notes.js';
import { state } from './state.js';
import { forPlatform, here } from './platform-notes.js';

// About's release notes (docs/settings.md): what changed in this release, under its version,
// the notes built into the page with it; and above them, while a newer release downloads or
// is ready, its own notes, which Go fetched from beside the manifest. Each shows the lines
// for this platform and for every one (platform-notes.js).
const platform = computed(() => here(state.keyboard.platform, navigator.userAgent));
const coming = computed(() => {
  const u = state.update;
  if (!u.notes || !(u.phase === 'downloading' || u.phase === 'ready')) return null;
  const changes = forPlatform(parse(u.notes).changes, platform.value);
  return changes.length ? { version: u.version, changes } : null;
});
const changes = computed(() => forPlatform(notes.changes, platform.value));
</script>

<template>
  <section v-if="coming" class="release-notes" aria-label="Coming next">
    <h2>Coming in {{ coming.version }}</h2>
    <div class="card note">
      <ul class="changes">
        <li v-for="(c, i) in coming.changes" :key="i">{{ c }}</li>
      </ul>
    </div>
  </section>
  <section class="release-notes" aria-label="Release notes">
    <h2>What's new in {{ notes.version }}</h2>
    <div class="card note">
      <ul class="changes">
        <li v-for="(c, i) in changes" :key="i">{{ c }}</li>
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
