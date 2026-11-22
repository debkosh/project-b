<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { send, state } from '@/sdk/vue.js';
import { pageRect } from '@/sdk/regions.js';
import { pushEscape } from '@/ui/escape.js';

// The saved logins Go offers at a field of the page the person pressed
// (docs/password-autofill.md): the design system's floating menu, drawn by the shell
// beside the field and outside the page, so no script of the page's can reach it or steer a
// press on it. A press on a login fills it; nothing is filled before that. It shows whose
// logins they are when the field is in a frame of another site than the page's.

// The page keeps the keyboard while the list shows: the arrow keys, Enter and Escape
// pressed in the field reach Go through the form script, and Go says which login they
// are at (offer.at), fills it on Enter and takes the list away on Escape.

// The offer, when its tab's page shows in this window.
const offer = computed(() => {
  const o = state.passwordOffer;
  return o && pageRect(Number(o.tab)) ? o : null;
});
// The page's own host, for a field in a frame of another site: the list says both.
const pageHost = computed(() => {
  const tab = offer.value && state.tabs.find((t) => t.id === Number(offer.value.tab));
  try {
    return new URL(tab.url).host;
  } catch {
    return '';
  }
});
// A long host keeps its two ends, where the site is, and loses its middle.
// What fits the narrowest list beside "saved for".
const short = (host) => (host.length > 24 ? `${host.slice(0, 9)}…${host.slice(-14)}` : host);

const el = ref(null);
const pos = ref({ left: 0, top: 0, width: 0 });
// A press in its first half second is no pick: a page can't bring the list up under a
// press already on its way.
let since = 0;
let pop = null;

async function place() {
  const o = offer.value;
  const page = o && pageRect(Number(o.tab));
  if (!page) return;
  const width = Math.min(Math.max(o.box.w, 260), 380);
  const left = Math.max(4, Math.min(page.left + o.box.x, window.innerWidth - width - 4));
  pos.value = { left, top: page.top + o.box.y + o.box.h + 4, width };
  await nextTick();
  const r = el.value?.getBoundingClientRect();
  // With no room under the field, over it.
  if (r && r.bottom > window.innerHeight - 4) pos.value = { left, top: Math.max(4, page.top + o.box.y - r.height - 4), width };
}

watch(
  () => offer.value?.id,
  (id) => {
    pop?.();
    pop = null;
    if (!id) return;
    since = performance.now();
    pop = pushEscape(dismiss);
    place();
  },
  { immediate: true },
);

function dismiss() {
  if (offer.value) send({ t: 'passwordDismiss', offer: offer.value.id });
}
function pick(login) {
  if (performance.now() - since < 500) return;
  send({ t: 'passwordPick', offer: offer.value.id, login: login.id });
}
function manage() {
  dismiss();
  send({ t: 'passwordsManage' });
}
// The list has one current login, whichever of the field's arrow keys and the pointer
// moved last: Go keeps it (offer.at), so Enter in the field fills the one the pointer is
// over too. A pointer that only lies there while a key moves says nothing.
function over(i) {
  if (offer.value.at !== i) send({ t: 'passwordAt', offer: offer.value.id, at: i });
}

// A press on the shell outside it takes it away. A press on the page is the page's, and
// its form script tells Go, which takes it away.
function onOutside(e) {
  if (offer.value && el.value && !el.value.contains(e.target)) dismiss();
}
window.addEventListener('pointerdown', onOutside, true);
window.addEventListener('resize', place);
onBeforeUnmount(() => {
  window.removeEventListener('pointerdown', onOutside, true);
  window.removeEventListener('resize', place);
  pop?.();
});

function onKey(e) {
  const items = [...el.value.querySelectorAll('[role="menuitem"]')];
  const i = items.indexOf(document.activeElement);
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault();
    items[(i + (e.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length]?.focus();
  }
}
</script>

<template>
  <div
    v-if="offer"
    ref="el"
    v-shell
    class="ui-menu float password-offer"
    role="menu"
    :aria-label="`Saved passwords for ${offer.site}`"
    :style="{ left: pos.left + 'px', top: pos.top + 'px', width: pos.width + 'px' }"
    @keydown="onKey"
    @contextmenu.prevent
  >
    <div v-if="offer.framed" class="whose">
      This sign-in is from <b>{{ offer.site }}</b
      ><template v-if="pageHost">, shown inside {{ pageHost }}</template>.
    </div>
    <!-- A page that isn't https: what is filled there can be read on its way. -->
    <div v-if="offer.insecure" class="whose">This page isn't secure: what you fill could be read on the way.</div>
    <button v-for="(l, i) in offer.logins" :key="l.id" type="button" role="menuitem" class="ui-menu-item login" :class="{ at: offer.at === i }" :aria-current="offer.at === i || undefined" @pointermove="over(i)" @click="pick(l)">
      <!-- The key: 16px at a 1.5px stroke, as the design system's line icons are. -->
      <svg class="key" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <circle cx="5.25" cy="10.75" r="3.25" />
        <path d="M7.6 8.4 13.75 2.25M11 5l1.75 1.75M13 3l1.5 1.5" />
      </svg>
      <span class="name" :class="{ none: !l.username }">{{ l.username || 'No username' }}</span>
      <span class="dots" aria-hidden="true">••••••••</span>
      <span v-if="l.savedFor" class="secondary for" :title="l.savedFor">saved for {{ short(l.savedFor) }}</span>
    </button>
    <div class="ui-menu-separator" role="separator"></div>
    <button type="button" role="menuitem" class="ui-menu-item" @click="manage">Manage passwords…</button>
  </div>
</template>

<style scoped>
.password-offer {
  position: fixed;
  box-sizing: border-box;
  z-index: 40;
}

.whose {
  padding: 6px 12px 8px;
  color: var(--muted);
  font-size: 12px;
  line-height: 16px;
}

.whose b {
  color: var(--text);
  font-weight: 600;
}

/* A login: the key, the username and the password's dots on one line; and where it was
   saved for, when that isn't here, on a line of its own under the username, the row's width. */
.login {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr) auto;
  column-gap: 10px;
  align-items: center;
  height: auto;
  min-height: 36px;
  padding-top: 6px;
  padding-bottom: 6px;
  text-align: left;
}

/* One current login, drawn as a row under the pointer is: the one the field's arrow keys
   or the pointer moved to last. A row the pointer only lies on isn't it. */
.login:hover:not(.at) {
  background: none;
}

.login.at {
  background: var(--accent-tint);
}

.key {
  color: var(--icon);
}

.name,
.for {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.for {
  grid-column: 2 / 4;
  font-size: 12px;
  line-height: 16px;
}

.none,
.dots {
  color: var(--muted);
}
</style>
