<script setup>
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import { phone } from '@/phone.js';
import { siteTile } from '@/ui/tint.js';
import { useTints } from '@/ui/useTints.js';
import { send, state } from './state.js';

// Passwords (docs/passwords.md): the logins Project B keeps, a row each with its site's
// mark, the site and the username. Neither is sealed, so the list asks nothing. A row opens
// in place to its username and its password, hidden; Show and Copy each have the device
// check the person first. Show brings the password to this page until it is hidden again,
// the row closes or the page is left; Copy never does, Go puts it on the clipboard.
onMounted(() => send({ t: 'passwordsList' }));
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

// One row is open at a time. What it shows of itself: the password, while shown, and a line
// that answers the last press, "Copied" or why a password didn't open.
const opened = ref('');
const shown = reactive({});
const said = reactive({});
const WHY = {
  noScreenLock: 'Set a screen lock on this device to show passwords',
  notHere: "This profile's passwords can't be opened on this device",
  gone: "This password isn't saved any more",
  failed: "Couldn't open this password",
};
const hide = (id) => {
  delete shown[id];
  delete said[id];
};
function toggle(id) {
  if (opened.value) hide(opened.value);
  opened.value = opened.value === id ? '' : id;
}
const show = (l) => (shown[l.id] ? delete shown[l.id] : send({ t: 'passwordShow', id: l.id }));
const copy = (l, what) => send({ t: 'passwordCopy', id: l.id, what });

// Go's answers, to this page alone. A check the person said no to says nothing.
let saidFor = 0;
function say(id, text, bad) {
  said[id] = { text, bad };
  clearTimeout(saidFor);
  if (!bad) saidFor = setTimeout(() => delete said[id], 2000);
}
watch(
  () => state.passwordAnswer,
  (m) => {
    if (!m || m.id !== opened.value) return;
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
// A password shown goes back to dots when the window or the page is left, as it does when
// its row closes: nothing still shows it on return.
const hideShown = () => Object.keys(shown).forEach((id) => delete shown[id]);
const hideWhenHidden = () => document.hidden && hideShown();
onMounted(() => {
  window.addEventListener('blur', hideShown);
  document.addEventListener('visibilitychange', hideWhenHidden);
});
onUnmounted(() => {
  window.removeEventListener('blur', hideShown);
  document.removeEventListener('visibilitychange', hideWhenHidden);
  clearTimeout(saidFor);
  state.passwordAnswer = null;
});

const deleting = ref(null);
function remove() {
  send({ t: 'passwordDelete', id: deleting.value.id });
  if (opened.value === deleting.value.id) toggle(opened.value);
  deleting.value = null;
}
// While sync is on the logins are on the sync server too, sealed, and a delete goes round.
const syncing = computed(() => !!state.sync?.on);
const count = computed(() => (all.value.length === 1 ? '1 saved' : `${all.value.length.toLocaleString('en-US')} saved`));
</script>

<template>
  <section aria-label="Passwords" class="passwords" :class="{ phone }">
    <p class="secondary intro">{{ syncing ? "Sealed for this profile; your sync server can't read them." : 'Sealed on this device for this profile.' }} Showing or copying a password asks for {{ phone ? 'your screen lock' : "Touch ID or your Mac's password" }} first.</p>
    <input v-if="all.length" v-model="filter" class="text-field filter" type="text" spellcheck="false" placeholder="Search passwords" aria-label="Search passwords" @keydown.esc.stop="filter = ''" />
    <div class="card list">
      <div v-if="state.passwords && all.length === 0" class="row empty">No saved passwords yet. {{ phone ? 'With sync on, the ones on your other devices come here.' : 'Bring them from another browser under Bring your things over.' }}</div>
      <div v-else-if="all.length && rows.length === 0" class="row empty">No saved password matches “{{ filter.trim() }}”</div>
      <template v-for="l in rows" :key="l.id">
        <button type="button" class="row login" :data-site="l.site" :aria-expanded="opened === l.id" @click="toggle(l.id)">
          <span class="mark" aria-hidden="true" :style="{ background: tile(l.site).background, boxShadow: `inset 0 0 0 1px ${tile(l.site).border}`, color: tile(l.site).letter }">{{ letter(l.site) }}</span>
          <span class="who">
            <span class="name" :title="l.site">{{ l.site }}</span>
            <span class="secondary name" :class="{ none: !l.username }" :title="l.username">{{ l.username || 'No username' }}</span>
          </span>
          <svg class="chevron" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3.5 6 8 10.5 12.5 6" /></svg>
        </button>
        <div v-if="opened === l.id" class="panel">
          <div class="kv">
            <span class="k">Username</span>
            <span class="v" :class="{ none: !l.username }">{{ l.username || 'None' }}</span>
            <button v-if="l.username" type="button" class="text-button" :aria-label="`Copy the username for ${l.site}`" @click="copy(l, 'username')">Copy</button>
          </div>
          <div class="kv">
            <span class="k">Password</span>
            <span class="v secret" :class="{ plain: shown[l.id] }">{{ shown[l.id] || '••••••••••' }}</span>
            <button type="button" class="text-button" :aria-label="`${shown[l.id] ? 'Hide' : 'Show'} the password for ${l.site}`" @click="show(l)">{{ shown[l.id] ? 'Hide' : 'Show' }}</button>
            <button type="button" class="text-button" :aria-label="`Copy the password for ${l.site}`" @click="copy(l, 'password')">Copy</button>
          </div>
          <div class="foot">
            <span class="secondary" :class="{ err: said[l.id]?.bad }" role="status">{{ said[l.id]?.text || saved(l.created) }}</span>
            <button type="button" class="text-button" :aria-label="`Delete the password for ${l.site}, ${l.username}`" @click="deleting = l">Delete…</button>
          </div>
        </div>
      </template>
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

/* A login's row is a button that opens it in place: a chevron down, which turns up. */
.login {
  width: 100%;
  gap: 12px;
  min-height: 56px;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.login + .login,
.panel + .login {
  border-top: 1px solid var(--line);
}

.login:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
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

.none,
.empty {
  color: var(--muted);
}

.chevron {
  flex: none;
  color: var(--icon);
  transition: transform var(--motion) var(--ease);
}

.login[aria-expanded='true'] .chevron {
  transform: rotate(180deg);
}

.panel {
  padding: 8px 16px 12px;
  border-top: 1px solid var(--line);
  background: var(--accent-tint);
}

.kv {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 40px;
}

.k {
  flex: none;
  width: 96px;
  color: var(--muted);
}

.v {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  user-select: text;
}

.secret {
  letter-spacing: 2px;
  color: var(--muted);
}

.secret.plain {
  letter-spacing: 0;
  color: var(--text);
  font:
    13px/20px ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
}

.foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 40px;
  margin-top: 4px;
}

.err {
  color: var(--danger);
}

.after {
  margin: 8px 4px 0;
}

/* On a phone a label sits over its value, and the buttons keep their 48px. */
.phone .kv {
  flex-wrap: wrap;
  gap: 0 12px;
  padding: 4px 0;
}

.phone .k {
  width: 100%;
}
</style>
