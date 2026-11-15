<script setup>
import { computed, ref } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import RowMenu from '@/ui/RowMenu.vue';
import Segmented from '@/ui/Segmented.vue';
import { send, state } from './state.js';

// Each site's settings (docs/daily-driver.md), in two views of one section. At "sites",
// every site with a setting: a row each, saying how each setting is by Go's title for it,
// which opens the site's own view, and a menu to reset one or Remove them all; a filter
// once the list is long. At "sites/<origin>", that site alone: every setting Go lets
// settings set, each Ask, Allow or Block, and Remove all for this site. The section's
// address is all it needs, so it shows the same wherever settings is hosted. Go owns the
// list and names the kinds.

const host = (origin) => origin.replace(/^https:\/\//, '');
const word = { allow: 'allowed', block: 'blocked' };
const summary = (s) => s.settings.map((x) => `${x.title} ${word[x.value]}`).join(', ');

const forget = (origin, kind) => send({ t: 'forgetSiteSetting', origin, kind });

function items(s) {
  return [
    ...s.settings.map((x) => ({ label: `Reset ${x.title.toLowerCase()}`, run: () => forget(s.origin, x.kind) })),
    { separator: true },
    { label: 'Remove', danger: true, run: () => forget(s.origin, '') },
  ];
}

// The row menus by origin, so a right-click on a row opens its menu.
const menus = {};

// A filter, once there are more sites than fit at a glance.
const filterAfter = 8;
const filter = ref('');
const shown = computed(() => {
  const f = filter.value.trim().toLowerCase();
  return f ? state.siteSettings.filter((s) => s.origin.toLowerCase().includes(f)) : state.siteSettings;
});

// The one site shown, from the section's address, or '' for every site.
const origin = computed(() => (state.section.startsWith('sites/') ? state.section.slice('sites/'.length) : ''));
const open = (o) => (state.section = o ? `sites/${o}` : 'sites');

const choices = [
  { value: '', label: 'Ask' },
  { value: 'allow', label: 'Allow' },
  { value: 'block', label: 'Block' },
];
const settingsOf = (o) => state.siteSettings.find((s) => s.origin === o)?.settings || [];
const valueOf = (kind) => settingsOf(origin.value).find((x) => x.kind === kind)?.value || '';
const set = (kind, value) => send({ t: 'setSiteSetting', origin: origin.value, kind, value });

// How the site's open pages reached it, first in its view; nothing when none is open.
const connection = computed(() => state.siteConnections[origin.value] || null);

// Remove all for this site asks first, then goes back to every site.
const removing = ref(false);
function removeAll() {
  removing.value = false;
  forget(origin.value, '');
  open('');
}
</script>

<template>
  <section v-if="origin" :aria-label="host(origin)">
    <div class="heading">
      <button type="button" class="text-button quiet back" @click="open('')">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10 3.5 5.5 8l4.5 4.5" /></svg>
        All sites
      </button>
    </div>
    <h2 class="site" :title="origin">{{ host(origin) }}</h2>
    <div class="card">
      <div v-if="connection" class="row connection" :class="connection.connection">
        <svg class="glyph" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <template v-if="connection.connection === 'secure'">
            <rect x="3.25" y="7" width="9.5" height="6.5" rx="1.5" />
            <path d="M5.5 7V5a2.5 2.5 0 0 1 5 0v2" />
          </template>
          <template v-else>
            <path d="M8 2.25 14.25 13.25H1.75z" />
            <path d="M8 6.5v3M8 11.5v.01" />
          </template>
        </svg>
        <span class="label">
          {{ connection.connection === 'secure' ? 'Connection is secure' : 'Not secure' }}
          <span v-if="connection.note" class="secondary">{{ connection.note }}</span>
        </span>
      </div>
      <div v-for="k in state.siteKinds" :key="k.kind" class="row">
        <span class="label">{{ k.title }}</span>
        <Segmented :label="k.title" :options="choices" :model-value="valueOf(k.kind)" @update:model-value="set(k.kind, $event)" />
      </div>
    </div>
    <div class="after">
      <button type="button" class="text-button" :disabled="settingsOf(origin).length === 0" @click="removing = true">Remove all for this site…</button>
    </div>
    <Dialog v-if="removing" :title="`Remove all for ${host(origin)}?`" @close="removing = false">
      <p>It asks again the next time it wants to use any of them.</p>
      <template #buttons>
        <button type="button" class="text-button" @click="removing = false">Cancel</button>
        <button type="button" class="text-button danger" @click="removeAll">Remove</button>
      </template>
    </Dialog>
  </section>
  <section v-else aria-label="Site settings">
    <p class="intro">Sites that asked to use your camera, microphone or notifications, and what you chose.</p>
    <input
      v-if="state.siteSettings.length > filterAfter"
      v-model="filter"
      class="text-field filter"
      type="search"
      aria-label="Filter sites"
      placeholder="Filter sites"
      spellcheck="false"
    />
    <div class="card">
      <div v-if="state.siteSettings.length === 0" class="row empty">No site has asked for a permission yet</div>
      <div v-else-if="shown.length === 0" class="row empty">No site matches</div>
      <div
        v-for="s in shown"
        :key="s.origin"
        class="row site-row"
        :data-origin="s.origin"
        @contextmenu.prevent="menus[s.origin]?.open()"
      >
        <button type="button" class="label open" :aria-label="host(s.origin)" @click="open(s.origin)">
          <span class="text">
            <span class="name" :title="s.origin">{{ host(s.origin) }}</span>
            <span class="secondary">{{ summary(s) }}</span>
          </span>
          <svg class="chevron" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 3.5 10.5 8 6 12.5" /></svg>
        </button>
        <RowMenu :ref="(m) => (menus[s.origin] = m)" :items="items(s)" :label="`Actions for ${host(s.origin)}`" />
      </div>
    </div>
  </section>
</template>

<style scoped>
.heading {
  display: flex;
  align-items: center;
  height: 30px;
}

.back {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-left: -8px;
}

/* The site is the view's subject. */
.site {
  margin: 4px 0 0;
  overflow: hidden;
  color: var(--text);
  font-size: 15px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.connection .glyph {
  flex: none;
  color: var(--icon);
}

.connection.insecure .glyph {
  color: var(--danger);
}

.connection .secondary {
  display: block;
  font-weight: 400;
}

.after {
  margin-top: 16px;
}

.intro {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}

.filter {
  width: 100%;
  margin-top: 12px;
}

/* The row opens the site's view: the label is the button, as wide as the row allows, with
   the chevron at its end. */
.open {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
  padding: 0;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.open:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
  border-radius: 4px;
}

.text {
  flex: 1;
  min-width: 0;
}

.chevron {
  flex: none;
  color: var(--icon);
}

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
