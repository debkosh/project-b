<script setup>
import { computed, inject } from 'vue';
import { phone } from '@/phone.js';
import { folders } from '@/ui/bookmarks.js';
import Row from './Row.vue';
import { setPref, state } from './state.js';

// Where bookmarks show, each on or off, and the bookmarks bar over every page with the
// folder it shows (docs/bookmark-folders.md). Go's prefs, set by name.

// Each place, with its pref and its words: a phone's shell has a switcher for the sidebar,
// and its address for the command box.
const places = computed(() => [
  { pref: 'bookmarksNewTab', label: 'On the new tab page', hint: 'Your folders as tiles; one opens in place' },
  phone
    ? { pref: 'bookmarksSidebar', label: 'In the tab switcher', hint: 'Tabs and Bookmarks, side by side' }
    : { pref: 'bookmarksSidebar', label: 'Beside the tabs', hint: 'The folders under the tabs, in the sidebar' },
  phone
    ? { pref: 'bookmarksCommandBox', label: 'In the address bar', hint: 'A row of them over a page, and among the suggestions as you type' }
    : { pref: 'bookmarksCommandBox', label: 'In the command box', hint: 'Type * for bookmarks only' },
]);

// Where the page's own heading names the section, its own gives way.
const titled = inject('settingsTitled', false);

// The folders the bar can show, the top first, each under the one it is in.
const barFolders = computed(() => folders(state.bookmarks));

// The bar's folder, or the top when the one chosen is gone, as the shells show it.
const barFolder = computed(() => (barFolders.value.some((f) => f.id === state.prefs.bookmarksBarFolder) ? state.prefs.bookmarksBarFolder : ''));
</script>

<template>
  <section aria-label="Bookmarks">
    <h2 v-if="!titled">Bookmarks</h2>
    <div class="card">
      <Row v-for="p in places" :key="p.pref" v-slot="{ named }" tag="label" :label="p.label" :hint="p.hint">
        <input
          v-bind="named"
          type="checkbox"
          role="switch"
          class="switch"
          :checked="!!state.prefs[p.pref]"
          :aria-checked="String(!!state.prefs[p.pref])"
          @change="setPref(p.pref, $event.target.checked)"
        />
      </Row>
    </div>
    <div class="card bar-card">
      <Row v-slot="{ named }" tag="label" label="Bookmarks bar" :hint="phone ? 'A strip of one folder above the bars' : 'A bar of one folder over every page'">
        <input
          v-bind="named"
          type="checkbox"
          role="switch"
          class="switch"
          :checked="!!state.prefs.bookmarksBar"
          :aria-checked="String(!!state.prefs.bookmarksBar)"
          @change="setPref('bookmarksBar', $event.target.checked)"
        />
      </Row>
      <Row v-if="state.prefs.bookmarksBar" label="Folder it shows" for="bar-folder">
        <select id="bar-folder" :value="barFolder" @change="setPref('bookmarksBarFolder', $event.target.value)">
          <option v-for="f in barFolders" :key="f.id" :value="f.id">{{ ' '.repeat(f.depth) + f.title }}</option>
        </select>
      </Row>
    </div>
  </section>
</template>

<style scoped>
.bar-card {
  margin-top: 12px;
}
</style>
