<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { linkChoices, openSiteSettings, permissionWord, reloadPage, setSiteSetting, siteOrigin, siteSetting, sparkNames } from '@/sdk/sites.js';
import Segmented from './Segmented.vue';
import { pushEscape } from './escape.js';

// The site panel (docs/site-features.md): what the address field's site icon opens over the
// page, for the active tab's site. Its host and connection; the ad blocker, with what it
// kept from this page; whether sparks run on the site; links to other sites; the permissions;
// and Details, to settings. Every change goes to Go, which answers with the site's new
// settings. The root floats over the page: the caller marks it the shell's, places it, and
// closes it on close (Escape, Details).
const props = defineProps({ tab: Object, site: Object });
const emit = defineEmits(['close']);

const origin = computed(() => siteOrigin(props.tab));
const host = computed(() => origin.value.replace(/^https?:\/\//, ''));
const secure = computed(() => props.tab.connection === 'secure');
const connection = computed(() => {
  if (secure.value) return 'Connection is secure';
  return props.tab.connectionNote ? `Not secure: ${props.tab.connectionNote}` : 'Not secure';
});

const ads = computed(() => siteSetting(props.site, 'ads'));
const links = computed(() => siteSetting(props.site, 'links'));
// The permissions the site has set.
const permissions = computed(() => (props.site?.settings || []).filter((s) => !s.choices && s.value));
const sparksOn = computed(() => siteSetting(props.site, 'sparks'));
// The sparks that run here while the site's sparks are on, by name.
const sparks = computed(() => props.site?.sparks || []);

// The blocker and a spark change what a page gets as it loads.
const reload = ref(false);

function setAds(on) {
  setSiteSetting(origin.value, 'ads', on ? 'on' : 'off');
  reload.value = true;
}
function setSparks(on) {
  setSiteSetting(origin.value, 'sparks', on ? 'on' : 'off');
  reload.value = true;
}
function reloadNow() {
  reloadPage();
  reload.value = false;
}
function details() {
  emit('close');
  openSiteSettings();
}

let pop = null;
onMounted(() => (pop = pushEscape(() => emit('close'))));
onUnmounted(() => pop());
</script>

<template>
  <div class="ui-site-panel" role="dialog" :aria-label="`Site settings for ${host}`">
    <div class="ui-site-head">
      <svg v-if="secure" class="ui-site-glyph" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
        <rect x="3.25" y="7" width="9.5" height="6.5" rx="1.5" />
        <path d="M5.5 7V5a2.5 2.5 0 0 1 5 0v2" />
      </svg>
      <svg v-else class="ui-site-glyph insecure" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M8 2.5 14 13H2zM8 6.5v3M8 11.2v.1" />
      </svg>
      <div>
        <h2 class="ui-site-host">{{ host }}</h2>
        <p class="ui-site-note">{{ connection }}</p>
      </div>
    </div>

    <div class="ui-site-group">
      <label v-if="ads" class="ui-site-row">
        <span class="ui-site-text">
          <span>Ad blocker</span>
          <small v-if="ads.value === 'on'">{{ tab.adsBlocked || 0 }} blocked on this page</small>
        </span>
        <input type="checkbox" class="switch" :checked="ads.value === 'on'" @change="setAds($event.target.checked)" />
      </label>
      <label v-if="sparksOn && sparks.length" class="ui-site-row">
        <span class="ui-site-text">
          <span>Sparks</span>
          <small>{{ sparkNames(sparks) }}</small>
        </span>
        <input type="checkbox" class="switch" :checked="sparksOn.value === 'on'" @change="setSparks($event.target.checked)" />
      </label>
      <div v-if="links" class="ui-site-row stacked">
        <span class="ui-site-text">
          <span id="ui-site-links">Links to other sites</span>
        </span>
        <Segmented :model-value="links.value" :options="linkChoices" label="Links to other sites" @update:model-value="(v) => setSiteSetting(origin, 'links', v)" />
      </div>
    </div>

    <div v-if="permissions.length" class="ui-site-group" role="group" aria-label="Permissions">
      <p class="ui-site-label">Permissions</p>
      <div v-for="p in permissions" :key="p.kind" class="ui-site-row">
        <span class="ui-site-text"><span>{{ p.title }}</span></span>
        <span class="ui-site-value" :class="p.value">{{ permissionWord(p.value) }}</span>
      </div>
    </div>

    <div class="ui-site-foot">
      <span>{{ reload ? 'Changes apply when the page reloads' : '' }}</span>
      <button v-if="reload" type="button" class="text-button" @click="reloadNow">Reload</button>
      <button type="button" class="text-button" @click="details">Details</button>
    </div>
  </div>
</template>

<style>
.ui-site-panel {
  width: 360px;
  max-width: calc(100vw - 16px);
  max-height: calc(100vh - 80px);
  overflow-y: auto;
  box-sizing: border-box;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  box-shadow: var(--shadow-float);
  color: var(--text);
}

.ui-site-head {
  display: flex;
  gap: 10px;
  padding: 14px 16px 10px;
}

.ui-site-glyph {
  flex: none;
  margin-top: 2px;
  color: var(--icon);
}

.ui-site-glyph.insecure {
  color: var(--danger);
}

.ui-site-host {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.ui-site-note {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--muted);
}

.ui-site-group {
  border-top: 1px solid var(--line);
  padding: 4px 0;
}

.ui-site-label {
  margin: 0;
  padding: 8px 16px 2px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--muted);
}

.ui-site-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 40px;
  padding: 4px 16px;
  font-size: 13px;
}

.ui-site-row.stacked {
  flex-wrap: wrap;
  padding-bottom: 8px;
}

.ui-site-text {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.ui-site-text small {
  font-size: 12px;
  color: var(--muted);
}

.ui-site-value {
  font-size: 12px;
  color: var(--muted);
}

.ui-site-value.allow {
  color: var(--icon);
}

.ui-site-value.block {
  color: var(--danger);
}

.ui-site-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border-top: 1px solid var(--line);
  padding: 8px 8px 8px 16px;
  font-size: 12px;
  color: var(--muted);
}

.ui-site-foot > span {
  flex: 1;
}
</style>
