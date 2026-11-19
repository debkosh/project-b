<script setup>
import { computed, onMounted, ref, watch } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import { phone } from '@/phone.js';
import { siteTile } from '@/ui/tint.js';
import { useTints } from '@/ui/useTints.js';
import { send, state } from './state.js';

// Passkeys (docs/passkeys.md, Seeing and deleting them): the passkeys Project B keeps, a
// row each with its site's mark, the site, the account and when it was made, and Delete,
// which asks first. The accounts are sealed, so the page asks Go for the list as it opens,
// and again when the count changes under it, as when sync brings one.
onMounted(() => send({ t: 'passkeysList' }));
watch(
  () => state.passkeyCount,
  () => send({ t: 'passkeysList' }),
);

const tints = useTints();
const tile = (site) => (tints.value, siteTile(`https://${site}/`, 'surface', 32));
const letter = (site) => site.replace(/^www\./, '').slice(0, 1).toUpperCase();
const account = (p) => (p.secondary ? `${p.title} · ${p.secondary}` : p.title);

// "Made today", or its day, with the year once it is another.
function made(ms) {
  const d = new Date(ms);
  const now = new Date();
  if (d.toDateString() === now.toDateString()) return 'Made today';
  const opts = d.getFullYear() === now.getFullYear() ? { day: 'numeric', month: 'short' } : { day: 'numeric', month: 'short', year: 'numeric' };
  return `Made ${d.toLocaleDateString('en-GB', opts)}`;
}

const deleting = ref(null);
const rows = computed(() => state.passkeys || []);
function remove() {
  send({ t: 'passkeyDelete', id: deleting.value.id });
  deleting.value = null;
}
</script>

<template>
  <section aria-label="Passkeys">
    <div class="card">
      <div v-if="state.passkeysNotHere" class="row empty">These passkeys were made on another device and only open there.</div>
      <div v-else-if="state.passkeys && rows.length === 0" class="row empty">No passkeys yet. When a site offers to create one, Project B can save it here.</div>
      <div v-for="p in rows" :key="p.id" class="row passkey" :data-site="p.site">
        <span class="mark" aria-hidden="true" :style="{ background: tile(p.site).background, boxShadow: `inset 0 0 0 1px ${tile(p.site).border}`, color: tile(p.site).letter }">{{ letter(p.site) }}</span>
        <span class="who">
          <span class="name" :title="p.site">{{ p.site }}</span>
          <span class="secondary name" :class="{ unnamed: p.unnamed }" :title="account(p)">{{ account(p) }}</span>
        </span>
        <span v-if="!phone" class="when secondary">{{ made(p.created) }}</span>
        <button v-if="phone" type="button" class="icon-button" :aria-label="`Delete passkey for ${p.site}, ${p.title}`" @click="deleting = p">
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 6h12M8 6V4h4v2M6 6l1 10h6l1-10" /></svg>
        </button>
        <button v-else type="button" class="text-button" :aria-label="`Delete passkey for ${p.site}, ${p.title}`" @click="deleting = p">Delete…</button>
      </div>
    </div>
    <p class="secondary after">Passkeys sync with your other devices when sync is on.</p>
    <Dialog v-if="deleting" title="Delete this passkey?" @close="deleting = null">
      <p>
        Your passkey for <b>{{ deleting.site }}</b> ({{ deleting.title }}) is deleted here and on your synced devices. {{ deleting.site }} still lists it until you remove it in its settings.
      </p>
      <template #buttons>
        <button type="button" class="text-button" @click="deleting = null">Cancel</button>
        <button type="button" class="text-button danger" @click="remove">Delete</button>
      </template>
    </Dialog>
  </section>
</template>

<style scoped>
.passkey {
  gap: 12px;
  min-height: 56px;
}

.mark {
  flex: none;
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  font-size: 18px;
  font-weight: 600;
  line-height: 1;
}

.who {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.unnamed {
  font-style: normal;
}

/* An empty list is one muted row. */
.empty {
  color: var(--muted);
}

.when {
  flex: none;
  white-space: nowrap;
}

.after {
  margin: 8px 4px 0;
}
</style>
