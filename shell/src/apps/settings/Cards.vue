<script setup>
import { computed } from 'vue';
import { extensionsSection } from '@extensions';
import Bookmarks from './Bookmarks.vue';
import Flags from './Flags.vue';
import Import from './Import.vue';
import Keyboard from './Keyboard.vue';
import KeysRow from './KeysRow.vue';
import ProfileRows from './ProfileRows.vue';
import Row from './Row.vue';
import Rows from './Rows.vue';
import SiteSettings from './SiteSettings.vue';
import Sparks from './Sparks.vue';
import WebFeatures from './WebFeatures.vue';
import Workspaces from './Workspaces.vue';
import { lists, rowsOf } from './arrange.js';

// A section's cards, as its layout arranges them (arrange.js): a card of rows, one of them
// maybe a list's, which opens it; a block, which brings its own cards; or a card under a
// heading of its own.
const props = defineProps({ section: Object });

const blocks = {
  bookmarks: Bookmarks,
  flags: Flags,
  import: Import,
  keyboard: Keyboard,
  sites: SiteSettings,
  sparks: Sparks,
  webFeatures: WebFeatures,
  workspaces: Workspaces,
  extensions: extensionsSection?.component,
};

// Each card as {heading, rows}, or a block's name.
const cards = computed(() => props.section.cards.map((c) => (typeof c === 'string' ? c : { heading: c.heading || '', rows: rowsOf(c) })));
</script>

<template>
  <template v-for="(card, i) in cards" :key="i">
    <component :is="blocks[card]" v-if="typeof card === 'string'" class="block" :data-part="card" />
    <section v-else class="block" :aria-label="card.heading || undefined">
      <h2 v-if="card.heading">{{ card.heading }}</h2>
      <div class="card">
        <template v-for="p in card.rows" :key="p">
          <Row v-if="lists[p]" :label="lists[p].label" :hint="lists[p].hint" :summary="lists[p].summary()" :opens="p" />
          <KeysRow v-else-if="p === 'keys'" />
          <ProfileRows v-else-if="p === 'profile'" />
          <Rows v-else :part="p" />
        </template>
      </div>
    </section>
  </template>
</template>
