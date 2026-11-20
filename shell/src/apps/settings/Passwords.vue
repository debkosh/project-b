<script setup>
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import LineIcon from '@/ui/LineIcon.vue';
import RowMenu from '@/ui/RowMenu.vue';
import { phone } from '@/phone.js';
import { siteTile } from '@/ui/tint.js';
import { useTints } from '@/ui/useTints.js';
import { send, state } from './state.js';

// Passwords (docs/passwords.md): the logins Project B keeps, one row each: its site's mark,
// the site and the username, and at its end Show, Copy and a menu. Neither the site nor the
// username is sealed, so the list asks nothing. Show brings the password to its row until
// it is hidden again or the page is left; Copy never brings it here, Go puts it on the
// clipboard. The device checks the person at the first of them, and Go holds that check
// for this page for five minutes: the page opening, and its closing, tell Go, which ends it.
onMounted(() => send({ t: 'passwordsList', fresh: true }));
watch(
  () => state.passwordCount,
  () => send({ t: 'passwordsList' }),
);

const tints = useTints();
const tile = (site) => (tints.value, siteTile(`https://${site}/`, 'surface', 32));
const letter = (site) => site.replace(/^www\./, '').slice(0, 1).toUpperCase();

// "Saved today", or its day, with the year once it is another.
function saved(ms) {
  const d = new Date(ms);
  const now = new Date();
  if (d.toDateString() === now.toDateString()) return 'Saved today';
  const opts = d.getFullYear() === now.getFullYear() ? { day: 'numeric', month: 'short' } : { day: 'numeric', month: 'short', year: 'numeric' };
  return `Saved ${d.toLocaleDateString('en-GB', opts)}`;
}

// The filter keeps the logins whose site or username has what was typed.
const filter = ref('');
const all = computed(() => state.passwords || []);
const rows = computed(() => {
  const q = filter.value.trim().toLowerCase();
  return q ? all.value.filter((l) => l.site.toLowerCase().includes(q) || l.username.toLowerCase().includes(q)) : all.value;
});

// What a row shows of itself: its password, while shown, and a line that answers the last
// press, "Password copied" or why a password didn't open.
const shown = reactive({});
const said = reactive({});
const WHY = {
  noScreenLock: 'Set a screen lock on this device to show passwords',
  notHere: "This profile's passwords can't be opened on this device",
  gone: "This password isn't saved any more",
  failed: "Couldn't open this password",
};
const show = (l) => (shown[l.id] ? delete shown[l.id] : send({ t: 'passwordShow', id: l.id }));
const copy = (l, what) => send({ t: 'passwordCopy', id: l.id, what });

// Go's answers, to this page alone. A check the person said no to says nothing.
const saidFor = {};
function say(id, text, bad) {
  said[id] = { text, bad };
  clearTimeout(saidFor[id]);
  if (!bad) saidFor[id] = setTimeout(() => delete said[id], 2000);
}
// A password shown goes back to dots when the window or the page is left, and when the
// check that showed it holds no more: nothing still shows it on return.
const hideShown = () => Object.keys(shown).forEach((id) => delete shown[id]);
const hideWhenHidden = () => document.hidden && hideShown();
let heldFor = 0;
function heldUntil(ms) {
  clearTimeout(heldFor);
  if (ms) heldFor = setTimeout(hideShown, Math.max(0, ms - Date.now()));
}
watch(
  () => state.passwordAnswer,
  (m) => {
    if (!m) return;
    if (m.heldUntil !== undefined) heldUntil(m.heldUntil);
    if (m.error) {
      if (m.error !== 'denied') say(m.id, WHY[m.error] || WHY.failed, true);
    } else if (m.t === 'passwordShown') {
      delete said[m.id];
      shown[m.id] = m.password;
    } else {
      say(m.id, m.what === 'username' ? 'Username copied' : 'Password copied', false);
    }
  },
);
onMounted(() => {
  window.addEventListener('blur', hideShown);
  document.addEventListener('visibilitychange', hideWhenHidden);
});
onUnmounted(() => {
  window.removeEventListener('blur', hideShown);
  document.removeEventListener('visibilitychange', hideWhenHidden);
  Object.values(saidFor).forEach(clearTimeout);
  clearTimeout(heldFor);
  state.passwordAnswer = null;
  send({ t: 'passwordsClosed' });
});

// The rest of a row's actions, in its menu.
const deleting = ref(null);
const items = (l) => [
  { label: 'Copy username', run: () => copy(l, 'username'), disabled: !l.username },
  { separator: true },
  { label: 'Delete…', run: () => (deleting.value = l), danger: true },
];
function remove() {
  send({ t: 'passwordDelete', id: deleting.value.id });
  delete shown[deleting.value.id];
  deleting.value = null;
}
// While sync is on the logins are on the sync server too, sealed, and a delete goes round.
const syncing = computed(() => !!state.sync?.on);
const check = computed(() => (phone ? 'your screen lock' : "Touch ID or your Mac's password"));
const count = computed(() => (all.value.length === 1 ? '1 saved' : `${all.value.length.toLocaleString('en-US')} saved`));
</script>

<template>
  <section aria-label="Passwords" class="passwords" :class="{ phone }">
    <p class="secondary intro">
      {{ syncing ? "Sealed for this profile; your sync server can't read them." : 'Sealed on this device for this profile.' }} Showing or copying a password asks for {{ check }}, then not again for 5 minutes while
      this page stays open.
    </p>
    <input v-if="all.length" v-model="filter" class="text-field filter" type="text" spellcheck="false" placeholder="Search passwords" aria-label="Search passwords" @keydown.esc.stop="filter = ''" />
    <div class="card list">
      <div v-if="state.passwords && all.length === 0" class="row empty">No saved passwords yet. {{ phone ? 'With sync on, the ones on your other devices come here.' : 'Bring them from another browser under Bring your things over.' }}</div>
      <div v-else-if="all.length && rows.length === 0" class="row empty">No saved password matches “{{ filter.trim() }}”</div>
      <div v-for="l in rows" :key="l.id" class="row login" :data-site="l.site" :title="saved(l.created)">
        <span class="mark" aria-hidden="true" :style="{ background: tile(l.site).background, boxShadow: `inset 0 0 0 1px ${tile(l.site).border}`, color: tile(l.site).letter }">{{ letter(l.site) }}</span>
        <span class="who">
          <span class="name" :title="l.site">{{ l.site }}</span>
          <span class="secondary name" :class="{ none: !l.username }" :title="l.username">{{ l.username || 'No username' }}</span>
          <span v-if="shown[l.id]" class="secret">{{ shown[l.id] }}</span>
          <span v-if="said[l.id]" class="secondary said" :class="{ err: said[l.id].bad }" role="status">{{ said[l.id].text }}</span>
        </span>
        <button type="button" class="icon-button" :aria-label="`${shown[l.id] ? 'Hide' : 'Show'} the password for ${l.site}`" :title="shown[l.id] ? 'Hide password' : 'Show password'" :aria-pressed="!!shown[l.id]" @click="show(l)">
          <LineIcon :name="shown[l.id] ? 'eyeOff' : 'eye'" />
        </button>
        <button type="button" class="icon-button" :aria-label="`Copy the password for ${l.site}`" title="Copy password" @click="copy(l, 'password')">
          <LineIcon name="copy" />
        </button>
        <RowMenu :items="items(l)" :label="`More for ${l.site}${l.username ? `, ${l.username}` : ''}`" />
      </div>
    </div>
    <p v-if="all.length" class="secondary after">{{ count }}</p>
    <Dialog v-if="deleting" title="Delete this password?" @close="deleting = null">
      <p>
        Your saved password for <b>{{ deleting.site }}</b
        ><template v-if="deleting.username"> ({{ deleting.username }})</template> is deleted {{ syncing ? 'here and on your synced devices' : 'from this profile' }}. Your account on {{ deleting.site }} isn't changed.
      </p>
      <template #buttons>
        <button type="button" class="text-button" @click="deleting = null">Cancel</button>
        <button type="button" class="text-button danger" @click="remove">Delete</button>
      </template>
    </Dialog>
  </section>
</template>

<style scoped>
.intro {
  margin: 0 4px 12px;
}

.filter {
  display: block;
  width: 100%;
  margin-bottom: 12px;
}

/* A login is one row: its mark, who it is, then Show, Copy and its menu. */
.login {
  gap: 4px;
  min-height: 56px;
}

.login + .login {
  border-top: 1px solid var(--line);
}

.mark {
  flex: none;
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  margin-right: 8px;
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

.none,
.empty {
  color: var(--muted);
}

/* The password, once shown, under its username: every character told apart, and all of it
   there, wrapped, as it is read off the page. */
.secret {
  margin-top: 2px;
  overflow-wrap: anywhere;
  user-select: text;
  font:
    13px/20px ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
}

.err {
  color: var(--danger);
}

.after {
  margin: 8px 4px 0;
}
</style>
