<script setup>
import RowMenu from '@/ui/RowMenu.vue';
import { send, state } from './state.js';

// Each site's settings (docs/daily-driver.md): a row per site saying how each of its
// settings is, by Go's title for it, and a menu to reset one, or Remove them all. Go owns
// the list and names the kinds.

const host = (s) => s.origin.replace(/^https:\/\//, '');
const word = { allow: 'allowed', block: 'blocked' };
const summary = (s) => s.settings.map((x) => `${x.title} ${word[x.value]}`).join(', ');

const forget = (s, kind) => send({ t: 'forgetSiteSetting', origin: s.origin, kind });

function items(s) {
  return [
    ...s.settings.map((x) => ({ label: `Reset ${x.title.toLowerCase()}`, run: () => forget(s, x.kind) })),
    { separator: true },
    { label: 'Remove', danger: true, run: () => forget(s, '') },
  ];
}

// The row menus by origin, so a right-click on a row opens its menu.
const menus = {};
</script>

<template>
  <section aria-label="Site settings">
    <div class="card">
      <div v-if="state.siteSettings.length === 0" class="row empty">No site has its own settings</div>
      <div
        v-for="s in state.siteSettings"
        :key="s.origin"
        class="row"
        :data-origin="s.origin"
        @contextmenu.prevent="menus[s.origin]?.open()"
      >
        <span class="label">
          <span class="name" :title="s.origin">{{ host(s) }}</span>
          <span class="secondary">{{ summary(s) }}</span>
        </span>
        <RowMenu :ref="(m) => (menus[s.origin] = m)" :items="items(s)" :label="`Actions for ${host(s)}`" />
      </div>
    </div>
  </section>
</template>

<style scoped>
.name {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.empty {
  color: var(--muted);
}
</style>
