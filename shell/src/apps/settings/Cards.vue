<script setup>
import { computed, inject, provide } from 'vue';
import { extensionsSection } from '@extensions';
import Bookmarks from './Bookmarks.vue';
import ClearData from './ClearData.vue';
import Flags from './Flags.vue';
import Import from './Import.vue';
import Keyboard from './Keyboard.vue';
import KeysRow from './KeysRow.vue';
import ProfileRows from './ProfileRows.vue';
import Row from './Row.vue';
import Rows from './Rows.vue';
import SiteSettings from './SiteSettings.vue';
import Sparks from './Sparks.vue';
import Sync from './Sync.vue';
import SyncDetails from './SyncDetails.vue';
import WebFeatures from './WebFeatures.vue';
import Workspaces from './Workspaces.vue';
import { blockRows, lists, rowsOf } from './arrange.js';
import { searching } from './search.js';

// A section's cards, as its layout arranges them (arrange.js): a card of rows, one of them
// maybe a list's, which opens it; a block, which brings its own cards; or a card under a
// heading of its own.
// titled says a heading over the cards names the section, so a block's own gives way.
const props = defineProps({ section: Object, titled: Boolean });
provide('settingsTitled', props.titled || inject('settingsTitled', false));

// Search matches a row by its section's name too, and counts it in its section.
provide('settingsSection', props.section);

const blocks = {
  bookmarks: Bookmarks,
  flags: Flags,
  import: Import,
  keyboard: Keyboard,
  sites: SiteSettings,
  sparks: Sparks,
  sync: Sync,
  syncDetails: SyncDetails,
  webFeatures: WebFeatures,
  workspaces: Workspaces,
  extensions: extensionsSection?.component,
};

// Each card as {heading, rows}, or a block's name.
const cards = computed(() => props.section.cards.map((c) => (typeof c === 'string' ? c : { heading: c.heading || '', rows: rowsOf(c) })));
</script>

<template>
  <template v-for="(card, i) in cards" :key="i">
    <!-- While search has words, a block that isn't made of rows is a row that goes to it. -->
    <div v-if="typeof card === 'string' && searching && blockRows[card]" class="card block">
      <Row :label="blockRows[card].label" :hint="blockRows[card].hint" :summary="blockRows[card].summary()" :opens="card" />
    </div>
    <component :is="blocks[card]" v-else-if="typeof card === 'string'" class="block" :data-part="card" />
    <section v-else class="block" :aria-label="card.heading || undefined">
      <h2 v-if="card.heading">{{ card.heading }}</h2>
      <div class="card">
        <template v-for="p in card.rows" :key="p">
          <Row v-if="lists[p]" :label="lists[p].label" :hint="lists[p].hint" :summary="lists[p].summary()" :opens="p" />
          <KeysRow v-else-if="p === 'keys'" />
          <ProfileRows v-else-if="p === 'profile'" />
          <ClearData v-else-if="p === 'clearData'" />
          <Rows v-else :part="p" />
        </template>
      </div>
    </section>
  </template>
</template>
