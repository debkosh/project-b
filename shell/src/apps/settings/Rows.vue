<script setup>
import { computed } from 'vue';
import { phone } from '@/phone.js';
import Segmented from '@/ui/Segmented.vue';
import Swatches from '@/ui/Swatches.vue';
import { framePresets } from '@/ui/colours.js';
import Row from './Row.vue';
import { send, setPref, state } from './state.js';

// The settings that are one row each, by part: what Go owns, shown, and the user's changes
// sent back. A row changes only when Go says so.
defineProps({ part: String });

// Where updates come from (docs/release-channels.md): Stable, each release after its full
// run, or Dev, each finished change as it lands.
const channels = [
  { value: 'stable', label: 'Stable' },
  { value: 'dev', label: 'Dev' },
];

const themes = [
  { value: 'system', label: 'System' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
];

// On Android, whether the window's size chooses the shell, or one stays at every size: the
// phone-sized window's or the wide one's, by its name.
const oneShells = computed(() => {
  const a = state.androidShells;
  const name = (list, id) => list.find((s) => s.id === id)?.name || id;
  return [
    { value: '', label: 'By size' },
    { value: 'phone', label: name(a.phones, a.phone) },
    { value: 'wide', label: name(a.wides, a.wide) },
  ];
});

// A phone's address bar: at the foot, by the thumb, or on top.
const positions = [
  { value: 'top', label: 'Top' },
  { value: 'bottom', label: 'Bottom' },
];

// What a start opens: the windows and tabs as they were, or a window opened fresh, as the
// shell opens one: Standard on its command box, another shell on a new tab.
const starts = [
  { value: 'continue', label: 'Where you left off' },
  { value: 'fresh', label: 'Start fresh' },
];

// Block ads says which sites have it off, which the switch leaves be.
const adsHint = computed(() => {
  if (!state.prefs.blockAds) return 'Off on every site';
  const n = state.siteSettings.filter((s) => s.settings.some((x) => x.kind === 'ads' && x.value === 'off')).length;
  return n ? `On every site, but ${n === 1 ? 'the one' : `the ${n}`} you turned it off for` : 'On every site';
});

// Save to: the folder downloads go to, the pref's or the Downloads folder Go named, by its
// own name, the whole path under it with the home folder as ~, and a long one cut in the
// middle, which keeps both ends.
const folder = computed(() => state.prefs.downloadFolder || state.downloadsFolder);
const folderName = computed(() => folder.value.split('/').filter((x) => x).pop() || folder.value);
const folderPath = computed(() => {
  const home = state.homeFolder;
  const p = home && (folder.value === home || folder.value.startsWith(`${home}/`)) ? `~${folder.value.slice(home.length)}` : folder.value;
  const most = 56;
  return p.length > most ? `${p.slice(0, 24)}…${p.slice(p.length - (most - 25))}` : p;
});

// Clear cookies when the app quits says how many sites are kept.
const quitHint = computed(() => {
  const n = state.prefs.keepCookies?.length || 0;
  return n ? `Except the ${n === 1 ? 'site' : `${n} sites`} you keep` : 'Every site\'s, as the app quits';
});

const layouts = [
  { value: 'vertical', label: 'Vertical' },
  { value: 'horizontal', label: 'Horizontal' },
];

</script>

<template>
  <Row v-if="part === 'theme'" label="Theme">
    <Segmented label="Theme" :options="themes" :model-value="state.theme" @update:model-value="send({ t: 'setTheme', text: $event })" />
  </Row>

  <Row v-else-if="part === 'frameColour'" label="Frame colour" class="colours">
    <Swatches
      label="Frame colour"
      custom
      :colors="framePresets"
      :model-value="state.windowColor"
      @update:model-value="send({ t: 'setFrameColor', text: $event })"
    />
  </Row>

  <Row v-else-if="part === 'tabLayout'" label="Tab layout">
    <Segmented label="Tab layout" :options="layouts" :model-value="state.tabLayout" @update:model-value="send({ t: 'setTabLayout', text: $event })" />
  </Row>

  <Row v-else-if="part === 'addressBar'" label="Address bar">
    <Segmented label="Address bar" :options="positions" :model-value="state.addressBarPosition" @update:model-value="send({ t: 'setAddressBarPosition', text: $event })" />
  </Row>

  <!-- Android's shells, side by side: first whether the window's size chooses or one
       stays, then the phone shell's way of drawing, Web or Native, while a phone-sized
       window can show it, and a wide window's shell when there is more than one. A
       desktop's are the profile's, and every window loads the one chosen at once. -->
  <template v-else-if="part === 'shell' && phone">
    <Row label="Shell" hint="Switches with the window">
      <Segmented
        label="Shell"
        :options="oneShells"
        :model-value="state.androidShells.one"
        @update:model-value="send({ t: 'setOneShell', text: $event })"
      />
    </Row>
    <Row v-if="state.androidShells.phones.length > 1 && state.androidShells.one !== 'wide'" label="Phone layout">
      <Segmented
        label="Phone layout"
        :options="state.androidShells.phones.map((s) => ({ value: s.id, label: s.native ? 'Native' : 'Web' }))"
        :model-value="state.androidShells.phone"
        @update:model-value="send({ t: 'setPhoneShell', text: $event })"
      />
    </Row>
    <Row v-if="state.androidShells.wides.length > 1 && state.androidShells.one !== 'phone'" label="Wide window">
      <Segmented
        label="Wide window"
        :options="state.androidShells.wides.map((s) => ({ value: s.id, label: s.name }))"
        :model-value="state.androidShells.wide"
        @update:model-value="send({ t: 'setShell', text: $event })"
      />
    </Row>
  </template>
  <Row v-else-if="part === 'shell'" label="Shell" hint="Every window loads it" for="profile-shell">
    <select id="profile-shell" :value="state.profile.shell" @change="send({ t: 'setShell', text: $event.target.value })">
      <option v-for="s in state.profile.shells" :key="s.id" :value="s.id">{{ s.name }}</option>
    </select>
  </Row>

  <Row v-else-if="part === 'start'" label="On start, open">
    <Segmented label="On start, open" :options="starts" :model-value="state.start" @update:model-value="send({ t: 'setStart', text: $event })" />
  </Row>

  <Row v-else-if="part === 'searchEngine'" label="Search engine" for="search-engine">
    <select id="search-engine" :value="state.searchEngine" @change="send({ t: 'setSearchEngine', text: $event.target.value })">
      <option v-for="e in state.searchEngines" :key="e.id" :value="e.id">{{ e.name }}</option>
    </select>
  </Row>

  <Row v-else-if="part === 'autoUpdate'" v-slot="{ named }" tag="label" label="Automatic updates" hint="Updates download by themselves, and apply at the next start">
    <input
      v-bind="named"
      type="checkbox"
      role="switch"
      class="switch"
      :checked="state.autoUpdate"
      :aria-checked="String(state.autoUpdate)"
      @change="send({ t: 'setAutoUpdate', on: $event.target.checked })"
    />
  </Row>

  <Row
    v-else-if="part === 'channel'"
    label="Updates from"
    :hint="state.channel === 'stable' && /-dev\./.test(state.running || '') ? 'Stays on this Dev build until a newer Stable' : 'Dev brings each change as it lands, before its full tests'"
  >
    <Segmented label="Updates from" :options="channels" :model-value="state.channel" @update:model-value="send({ t: 'setChannel', text: $event })" />
  </Row>

  <!-- The ad blocker on every site; each site can still turn it off for itself. -->
  <Row v-else-if="part === 'blockAds'" v-slot="{ named }" tag="label" label="Block ads" :hint="adsHint">
    <input
      v-bind="named"
      type="checkbox"
      role="switch"
      class="switch"
      :checked="!!state.prefs.blockAds"
      :aria-checked="String(!!state.prefs.blockAds)"
      @change="setPref('blockAds', $event.target.checked)"
    />
  </Row>

  <!-- Cookies and site storage erased as the app quits, but the sites kept, each kept from
       its page in Sites. -->
  <Row v-else-if="part === 'clearOnQuit'" v-slot="{ named }" tag="label" label="Clear cookies when the app quits" :hint="quitHint">
    <input
      v-bind="named"
      type="checkbox"
      role="switch"
      class="switch"
      :checked="!!state.prefs.clearCookiesOnQuit"
      :aria-checked="String(!!state.prefs.clearCookiesOnQuit)"
      @change="setPref('clearCookiesOnQuit', $event.target.checked)"
    />
  </Row>

  <!-- Where downloads go: Change… asks in the system's panel, through Go and the engine. -->
  <Row v-else-if="part === 'downloadFolder'" label="Save to" :hint="folderPath">
    <button v-if="state.prefs.downloadFolder" type="button" class="text-button quiet" @click="setPref('downloadFolder', '')">Use Downloads</button>
    <button type="button" class="text-button folder" :aria-label="`Save to ${folderName}, Change…`" @click="send({ t: 'chooseDownloadFolder' })">
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" aria-hidden="true"><path d="M2 4.25c0-.69.56-1.25 1.25-1.25h3.1l1.5 1.75h4.9c.69 0 1.25.56 1.25 1.25v6.25c0 .69-.56 1.25-1.25 1.25h-9.5C2.56 13.5 2 12.94 2 12.25z" /></svg>
      {{ folderName }}
    </button>
  </Row>

  <Row v-else-if="part === 'menuBar'" v-slot="{ named }" tag="label" label="Show menu bar" hint="Alt shows it for a moment">
    <input
      v-bind="named"
      type="checkbox"
      role="switch"
      class="switch"
      :checked="!!state.prefs.showMenuBar"
      :aria-checked="String(!!state.prefs.showMenuBar)"
      @change="setPref('showMenuBar', $event.target.checked)"
    />
  </Row>

  <Row v-else-if="part === 'askWhereToSave'" v-slot="{ named }" tag="label" label="Ask where to save each file" hint="Each download opens the Save panel, as Save Link As does; off, files go straight to the Save to folder">
    <input
      v-bind="named"
      type="checkbox"
      role="switch"
      class="switch"
      :checked="!!state.prefs.askWhereToSave"
      :aria-checked="String(!!state.prefs.askWhereToSave)"
      @change="setPref('askWhereToSave', $event.target.checked)"
    />
  </Row>

  <!-- The welcome again: marked unfinished, and the app closes, for the next start to open
       it as the first did. -->
  <Row
    v-else-if="part === 'runSetup'"
    label="Run setup again"
    hint="Closes the app. Open it again for the welcome over the whole window, as on a first start; your tabs come back under it as On start, open says."
  >
    <button type="button" class="text-button" @click="send({ t: 'welcomeAgain' })">Close the app</button>
  </Row>
</template>

<style scoped>
/* On a phone the colours take a line of their own. */
.touch .colours :deep(.ui-swatches) {
  flex-basis: 100%;
  flex-wrap: wrap;
}

/* Save to's folder: its icon before its name. */
.folder {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
</style>
