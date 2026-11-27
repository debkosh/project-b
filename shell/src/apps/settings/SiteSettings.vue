<script setup>
import { computed, ref, useId } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import RowMenu from '@/ui/RowMenu.vue';
import Segmented from '@/ui/Segmented.vue';
import { size } from '@/ui/size.js';
import { find } from '@/ui/filter.js';
import { send, state } from './state.js';

// Sites (docs/settings.md), in two views of one section. At "sites", a row per site, its
// registrable domain, as Go joins them: the settings of each of its hosts and what every
// workspace's storage keeps for them, each row opening the site's own view, with a menu to
// reset a setting, clear its data or Remove its settings, and Clear all site data at the
// foot; a filter once the list is long. At "sites/<site>", that site alone: what it keeps,
// with Clear, and for each of its hosts with a setting every setting Go lets settings set,
// each Ask, Allow or Block; and Remove all for this site. "sites/<origin>", as a site panel
// opens it, is the site of that origin, with that origin's settings whether or not it has
// any. The section's address is all it needs. Go owns the list and names the kinds.

const host = (origin) => origin.replace(/^https:\/\//, '');
const word = { allow: 'allowed', block: 'blocked', on: 'on', off: 'off' };
const settingsOf = (o) => state.siteSettings.find((s) => s.origin === o)?.settings || [];
const summaryOf = (o) => settingsOf(o).map((x) => `${x.title} ${word[x.value]}`).join(', ');
const words = (n, one, many) => `${n.toLocaleString()} ${n === 1 ? one : many}`;
const keeps = (r) => r.cookies > 0 || r.bytes > 0;
// What a site keeps: its size, or its cookies when nothing else.
const kept = (r) => (r.bytes ? size(r.bytes) : words(r.cookies, 'cookie', 'cookies'));

// Whether a site's cookies stay as the app quits, while the app clears the others'.
const isKept = (site) => !!state.prefs.clearCookiesOnQuit && (state.prefs.keepCookies || []).includes(site);
const keep = (site, on) => send({ t: 'keepCookies', site, keep: on });

// A row's second line: its settings, one host's alone or each host's by name, then how many
// hosts keep something and what, and whether it is kept as the app quits.
function summary(r) {
  const parts = r.origins.length === 1 ? [summaryOf(r.origins[0])] : r.origins.map((o) => `${host(o)}: ${summaryOf(o)}`);
  if (r.hosts.length > 1) parts.push(words(r.hosts.length, 'host', 'hosts'));
  if (keeps(r)) parts.push(kept(r));
  if (isKept(r.site)) parts.push('Kept');
  return parts.filter((x) => x).join(' · ');
}

const forget = (origin, kind) => send({ t: 'forgetSiteSetting', origin, kind });

function items(r) {
  const one = r.origins.length === 1;
  const resets = r.origins.flatMap((o) =>
    settingsOf(o).map((x) => ({ label: `Reset ${x.title.toLowerCase()}${one ? '' : ` on ${host(o)}`}`, run: () => forget(o, x.kind) })),
  );
  const list = [...resets];
  if (keeps(r)) list.push({ label: 'Clear data…', run: () => (clearing.value = r.site) });
  if (r.origins.length) list.push({ separator: true }, { label: 'Remove', danger: true, run: () => r.origins.forEach((o) => forget(o, '')) });
  return list;
}

// The row menus by site, so a right-click on a row opens its menu.
const menus = {};

// A filter, once there are more sites than fit at a glance.
const filterAfter = 8;
const filter = ref('');
const shown = computed(() => find(state.sites, (r) => [r.site, ...r.hosts, ...r.origins], filter.value));

// The one site shown, from the section's address: a site, or an origin of one. '' is every
// site.
const address = computed(() => (state.section.startsWith('sites/') ? state.section.slice('sites/'.length) : ''));
const asked = computed(() => (address.value.includes('://') ? address.value : ''));
const row = computed(() => {
  const a = address.value;
  if (!a) return null;
  if (!asked.value) return state.sites.find((r) => r.site === a) || null;
  const h = new URL(a).hostname;
  return state.sites.find((r) => r.origins.includes(a) || r.hosts.includes(h)) || null;
});
const title = computed(() => row.value?.site || host(asked.value || address.value));
// The origins whose settings the view shows: the site's own, and the one asked for. A site
// that only keeps data shows its own name's.
const origins = computed(() => {
  const list = [...(row.value?.origins || [])];
  if (asked.value && !list.includes(asked.value)) list.unshift(asked.value);
  if (!list.length && address.value) list.push(`https://${address.value}`);
  return list;
});
const open = (x) => (state.section = x ? `sites/${x}` : 'sites');

const choices = [
  { value: '', label: 'Ask' },
  { value: 'allow', label: 'Allow' },
  { value: 'block', label: 'Block' },
];
// One of our own kinds has no Ask: a site without a setting has its default, the kind's value.
const valueOf = (o, k) => settingsOf(o).find((x) => x.kind === k.kind)?.value || k.value || '';
// A site's own way of naming its downloads is one of Settings' three, or as Settings has it.
const ourWords = { on: 'On', off: 'Off', allow: 'Allow', block: 'Block', default: 'As in Settings', auto: 'Automatic', title: 'Title and file name', file: "File's own name" };
const choicesOf = (k) => (k.choices ? k.choices.map((v) => ({ value: v, label: ourWords[v] || v })) : choices);
const set = (origin, kind, value) => send({ t: 'setSiteSetting', origin, kind, value });

// The ad blocker off on every site leaves a site nothing to turn off.
const kinds = computed(() => state.siteKinds.filter((k) => k.kind !== 'ads' || state.prefs.blockAds));

// How an origin's open pages reached it; nothing when none is open.
const connectionOf = (o) => state.siteConnections[o] || null;
// What the dangerous sites' lists say of a site, in the notice's words.
const dangerNote = (d) => `${d.list} lists it as a ${d.kind} site`;

// Clearing a site's data, or every site's, asks first: the site by name, or '*' for all.
const clearing = ref('');
const total = computed(() => state.sites.reduce((n, r) => n + r.bytes, 0));
function clear() {
  if (clearing.value === '*') send({ t: 'clearAllSiteData' });
  else send({ t: 'clearSiteData', site: clearing.value });
  clearing.value = '';
}

// Remove all for this site asks first, then goes back to every site.
const removing = ref(false);
function removeAll() {
  removing.value = false;
  origins.value.forEach((o) => forget(o, ''));
  open('');
}

// Names for the switches, by the words beside them.
const uid = useId();
</script>

<template>
  <section v-if="address" :aria-label="title">
    <h2 class="site" :title="title">{{ title }}</h2>
    <!-- What the site keeps, and while the app clears cookies as it quits, whether it is
         kept: together, in one card. -->
    <div v-if="row && (keeps(row) || state.prefs.clearCookiesOnQuit)" class="card">
      <div v-if="keeps(row)" class="row">
        <span class="label">
          Cookies and site data
          <span class="secondary">{{ [words(row.cookies, 'cookie', 'cookies'), row.bytes ? size(row.bytes) : ''].filter((x) => x).join(' · ') }} on {{ row.hosts.join(', ') }}</span>
        </span>
        <button type="button" class="text-button" @click="clearing = row.site">Clear…</button>
      </div>
      <label v-if="state.prefs.clearCookiesOnQuit" class="row">
        <span class="label">
          <span>Keep cookies when the app quits</span>
          <span :id="uid + '-keep-hint'" class="secondary">Stay signed in here when the others are cleared</span>
        </span>
        <input
          aria-label="Keep cookies when the app quits"
          :aria-describedby="uid + '-keep-hint'"
          type="checkbox"
          role="switch"
          class="switch"
          :checked="isKept(row.site)"
          :aria-checked="String(isKept(row.site))"
          @change="keep(row.site, $event.target.checked)"
        />
      </label>
    </div>
    <template v-for="o in origins" :key="o">
      <h3 v-if="origins.length > 1" class="host" :title="o">{{ host(o) }}</h3>
      <div class="card" :aria-label="origins.length > 1 ? host(o) : undefined" :role="origins.length > 1 ? 'group' : undefined">
        <!-- A site the dangerous sites' lists name says so first, and which list. -->
        <div v-if="connectionOf(o)?.danger" class="row connection insecure dangerous">
          <svg class="glyph" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M8 2.25 14.25 13.25H1.75z" />
            <path d="M8 6.5v3M8 11.5v.01" />
          </svg>
          <span class="label">
            Dangerous
            <span class="secondary">{{ dangerNote(connectionOf(o).danger) }}</span>
          </span>
        </div>
        <div v-if="connectionOf(o)?.connection" class="row connection" :class="connectionOf(o).connection">
          <svg class="glyph" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <template v-if="connectionOf(o).connection === 'secure'">
              <rect x="3.25" y="7" width="9.5" height="6.5" rx="1.5" />
              <path d="M5.5 7V5a2.5 2.5 0 0 1 5 0v2" />
            </template>
            <template v-else>
              <path d="M8 2.25 14.25 13.25H1.75z" />
              <path d="M8 6.5v3M8 11.5v.01" />
            </template>
          </svg>
          <span class="label">
            {{ connectionOf(o).connection === 'secure' ? 'Connection is secure' : 'Not secure' }}
            <span v-if="connectionOf(o).note" class="secondary">{{ connectionOf(o).note }}</span>
          </span>
        </div>
        <div v-for="k in kinds" :key="k.kind" class="row">
          <span class="label">{{ k.title }}</span>
          <Segmented :label="k.title" :options="choicesOf(k)" :model-value="valueOf(o, k)" @update:model-value="set(o, k.kind, $event)" />
        </div>
      </div>
    </template>
    <div class="after">
      <button type="button" class="text-button" :disabled="origins.every((o) => settingsOf(o).length === 0)" @click="removing = true">Remove all for this site…</button>
    </div>
    <Dialog v-if="removing" :title="`Remove all for ${title}?`" @close="removing = false">
      <p>It asks again the next time it wants to use any of them.</p>
      <template #buttons>
        <button type="button" class="text-button" @click="removing = false">Cancel</button>
        <button type="button" class="text-button danger" @click="removeAll">Remove</button>
      </template>
    </Dialog>
  </section>
  <section v-else aria-label="Site settings">
    <p class="intro">Sites that keep cookies or data, or asked to use your camera, microphone or notifications, and what you chose.</p>
    <input
      v-if="state.sites.length > filterAfter"
      v-model="filter"
      class="text-field filter"
      type="search"
      aria-label="Filter sites"
      placeholder="Filter sites"
      spellcheck="false"
    />
    <div class="card">
      <div v-if="state.sites.length === 0" class="row empty">No site keeps anything or has asked for a permission yet</div>
      <div v-else-if="shown.length === 0" class="row empty">No site matches</div>
      <div
        v-for="r in shown"
        :key="r.site"
        class="row site-row"
        :data-site="r.site"
        @contextmenu.prevent="menus[r.site]?.open()"
      >
        <button type="button" class="label open" :aria-label="r.site" @click="open(r.site)">
          <span class="text">
            <span class="name" :title="r.site">{{ r.site }}</span>
            <span class="secondary">{{ summary(r) }}</span>
          </span>
          <svg class="chevron" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 3.5 10.5 8 6 12.5" /></svg>
        </button>
        <RowMenu :ref="(m) => (menus[r.site] = m)" :items="items(r)" :label="`Actions for ${r.site}`" />
      </div>
    </div>
    <div v-if="state.sites.some(keeps)" class="after foot">
      <span class="secondary">{{ words(state.sites.length, 'site', 'sites') }}{{ total ? ` · ${size(total)}` : '' }}</span>
      <button type="button" class="text-button" @click="clearing = '*'">Clear all site data…</button>
    </div>
  </section>
  <Dialog v-if="clearing" :title="clearing === '*' ? 'Clear all site data?' : `Clear data for ${clearing}?`" @close="clearing = ''">
    <p>{{ clearing === '*' ? 'Signs you out of every site, in every workspace.' : `Signs you out of ${clearing} and every host of it, in every workspace.` }} The settings stay.</p>
    <template #buttons>
      <button type="button" class="text-button" @click="clearing = ''">Cancel</button>
      <button type="button" class="text-button danger" @click="clear">Clear</button>
    </template>
  </Dialog>
</template>

<style scoped>
/* The site is the view's subject. */
.site {
  margin: 4px 0 0;
  overflow: hidden;
  color: var(--text);
  font-size: calc(15rem / 16);
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

/* Under the list, how much every site keeps and Clear all at the end. */
.foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

/* A host's own settings, under its name, when the site has more than one. */
.host {
  margin: 16px 0 6px;
  overflow: hidden;
  color: var(--muted);
  font-size: calc(13rem / 16);
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.intro {
  margin: 0;
  color: var(--muted);
  font-size: calc(13rem / 16);
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
