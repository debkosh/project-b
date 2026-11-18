<script setup>
import { computed, inject, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import { siteTile } from '@/ui/tint.js';
import { useTints } from '@/ui/useTints.js';
import Row from './Row.vue';
import { send, state, syncAnswers } from './state.js';

// Sync (docs/sync.md): a card under Your data. Off, one row sets it up; on, the server's row
// says when it last synced, with the passphrase's and Turn off's rows under it. A profile
// whose keys don't open here has one row, a state rather than an error. Setting up is one
// Dialog whose title follows its step: the address (and a token), then a passphrase chosen
// twice for an empty bucket or typed once for one in use. A project-b://sync link opens the
// same Dialog with the address filled in, and nothing goes to the server until Connect.

// Where the page's own heading names the section, its own gives way.
const titled = inject('settingsTitled', false);

// Search finds every row of the card by these, as well as by its words.
const words = 'sync passphrase';

const NO_KEY_DETAIL = "The key that seals this profile's secrets isn't on this device. That happens when a profile folder is moved from another device.";
const TOKEN_PLACEHOLDER = 'Only if your server gave you one';
const MIN = 12;

const s = computed(() => state.sync);

// The time, each second while the card shows: the syncing row waits a second before it
// shows, a run with nothing back for 30 seconds reads as the server out of reach, and "Last
// synced" moves on.
const now = ref(Date.now());
let ticker = 0;
onMounted(() => (ticker = setInterval(() => (now.value = Date.now()), 1000)));
onUnmounted(() => clearInterval(ticker));
const since = ref(0);
watch(
  () => s.value.syncing,
  (on) => {
    since.value = on ? Date.now() : 0;
    now.value = Date.now();
  },
  { immediate: true },
);
const syncingFor = computed(() => (since.value ? now.value - since.value : 0));
const syncingShown = computed(() => syncingFor.value >= 1000 && syncingFor.value < 30000);
const stalled = computed(() => syncingFor.value >= 30000);

// "2 minutes ago", for Last synced.
function ago(ms) {
  const min = Math.floor((now.value - ms) / 60000);
  const n = (k, one) => `${k} ${one}${k === 1 ? '' : 's'} ago`;
  if (min < 1) return 'just now';
  if (min < 60) return n(min, 'minute');
  if (min < 60 * 24) return n(Math.floor(min / 60), 'hour');
  return n(Math.floor(min / (60 * 24)), 'day');
}
const lastSynced = computed(() => (s.value.lastSynced ? `Last synced ${ago(s.value.lastSynced)}` : 'Not synced yet'));

// The server's row: what it says and what it offers.
const serverRow = computed(() => {
  if (syncingShown.value) return 'syncing';
  if (s.value.problem === 'token') return 'signedOut';
  if (s.value.error || stalled.value) return 'unreachable';
  return 'synced';
});

// The server's mark: its letter on its tile, as a site's.
const tints = useTints();
const tile = computed(() => (tints.value, siteTile(`https://${s.value.host}`)));

// Retry, and Try again after a run with nothing back, start the run again.
function again() {
  send({ t: 'syncCancel' });
  send({ t: 'syncNow' });
}

// The Dialog. step is '' while it is shut, 'address', 'choose', 'enter', 'change' or
// 'turnOff'; fromLink is the address step as a link opened it. Each opening counts, so an
// answer to what was asked before Cancel is dropped.
const d = reactive({
  step: '',
  fromLink: false,
  address: '',
  token: '',
  host: '',
  addressError: '',
  tokenError: '',
  pass: '',
  again: '',
  passShown: false,
  againShown: false,
  passError: '',
  againError: '',
  wrong: false,
  busy: false,
  detail: '',
});
let opened = 0;
let askedIn = -1;
function ask(m) {
  askedIn = opened;
  send(m);
}

const addressField = ref(null);
const tokenField = ref(null);
const passField = ref(null);
const againField = ref(null);
const primaryButton = ref(null);
const fields = { address: addressField, token: tokenField, pass: passField, again: againField, primary: primaryButton };
const focus = (name) => nextTick(() => fields[name].value?.focus());

function open(o = {}) {
  opened++;
  Object.assign(d, {
    step: 'address',
    fromLink: false,
    address: '',
    token: '',
    host: '',
    addressError: '',
    tokenError: '',
    pass: '',
    again: '',
    passShown: false,
    againShown: false,
    passError: '',
    againError: '',
    wrong: false,
    busy: false,
    detail: '',
    ...o,
  });
}

function shut() {
  opened++;
  d.step = '';
}

const setUp = () => {
  open();
  focus('address');
};
const reconnect = () => {
  open({ address: state.syncAddress });
  focus('token');
};
const change = () => {
  open({ step: 'change' });
  focus('pass');
};
const turnOff = () => open({ step: 'turnOff' });

// A link Go was handed: Go reads its address, and the Dialog opens on what it would connect
// to, or on the address and its error.
let linkAsked = false;
function fromLink(link) {
  linkAsked = true;
  send({ t: 'syncPropose', link });
}
watch(
  () => state.syncLink,
  (link) => {
    if (!link) return;
    state.syncLink = '';
    fromLink(link);
  },
  { immediate: true },
);

const isHttps = (a) => /^https:\/\//i.test(a.trim());
// Checked as typed: anything that can't become an https address.
const notHttps = (a) => {
  const t = a.trim().toLowerCase();
  return t !== '' && !t.startsWith('https://') && !'https://'.startsWith(t);
};
watch(
  () => d.address,
  (a) => {
    if (d.step !== 'address' || d.busy) return;
    d.addressError = notHttps(a) ? 'Sync needs an https address' : '';
  },
);
watch(
  () => d.token,
  () => (d.tokenError = ''),
);
watch(
  () => d.pass,
  () => {
    d.passError = '';
    d.wrong = false;
  },
);
watch(
  () => d.again,
  () => (d.againError = ''),
);

// A problem as the page words it, where it shows: under Token or under Address.
function problem(m) {
  const host = d.host || hostOf(d.address);
  d.detail = m.error || '';
  switch (m.problem) {
    case 'token':
      return { field: 'token', text: "The server didn't accept that token" };
    case 'needsToken':
      return { field: 'token', text: 'This address needs a token' };
    case 'unreachable':
      return { field: 'address', text: `Couldn't reach ${host}` };
    case 'notBucket':
      return { field: 'address', text: "That address isn't a sync bucket" };
    case 'address':
      return { field: 'address', text: isHttps(d.address) ? "That address isn't a sync bucket" : 'Sync needs an https address' };
  }
  return { field: 'address', text: `Couldn't set up sync with ${host}` };
}

function hostOf(a) {
  try {
    return new URL(a.trim()).host;
  } catch {
    return a;
  }
}

// The address, as the address field shows it while it isn't typed in: the host in the text's
// colour, the rest muted.
const addressFocused = ref(false);
const addressParts = computed(() => {
  const m = /^(https?:\/\/)([^/?#]+)(.*)$/i.exec(d.address.trim());
  return m ? { scheme: m[1], host: m[2], rest: m[3] } : null;
});
const dressed = computed(() => !addressFocused.value && addressParts.value);

function connect() {
  if (d.busy) return;
  if (!isHttps(d.address)) {
    d.addressError = 'Sync needs an https address';
    focus('address');
    return;
  }
  d.addressError = d.tokenError = '';
  d.busy = true;
  d.host = hostOf(d.address);
  ask({ t: 'syncInspect', address: d.address.trim(), token: d.token.trim() });
}

// The passphrase's strength, once it is long enough: words make it strong.
const strength = computed(() => {
  const p = d.pass;
  if (p.length < MIN) return '';
  const n = p.split(/[\s\-_.]+/).filter((w) => w.length >= 3).length;
  return n >= 4 || p.length >= 24 ? 'Strong' : 'Fair: add another word';
});

// A new passphrase, on the first device and on Change: at least 12 characters, twice.
function checked() {
  if (d.pass.length < MIN) {
    d.passError = `Use at least ${MIN} characters`;
    focus('pass');
    return false;
  }
  if (d.again !== d.pass) {
    d.againError = "The passphrases don't match";
    focus('again');
    return false;
  }
  return true;
}

function turnOn() {
  if (d.busy) return;
  if (d.step === 'choose' ? !checked() : !d.pass) return;
  d.busy = true;
  // A profile whose keys don't open starts fresh, and one the server signed out lets the
  // old bucket go, first; Go does them in order.
  if (state.sync.locked) send({ t: 'syncStartFresh' });
  else if (state.sync.on) send({ t: 'syncDisconnect' });
  ask({ t: 'syncConnect', address: d.address.trim(), token: d.token.trim(), passphrase: d.pass });
}

function changePassphrase() {
  if (d.busy || !checked()) return;
  d.busy = true;
  ask({ t: 'syncChangePassphrase', passphrase: d.pass });
}

function stop() {
  if (d.busy) return;
  d.busy = true;
  ask({ t: 'syncDisconnect' });
}

function suggest() {
  ask({ t: 'syncSuggest' });
}

const primary = () => ({ address: connect, choose: turnOn, enter: turnOn, change: changePassphrase, turnOff: stop })[d.step]();

// Go's answers: a link's address, and what this opening of the Dialog asked.
function heard(m) {
  if (m.t === 'syncProposal') {
    if (!linkAsked) return;
    linkAsked = false;
    if (m.error) {
      // A link's address that isn't one: the plain set-up, with the address and its error.
      open({ address: m.address || '' });
      d.addressError = problem(m).text;
      focus('address');
    } else {
      open({ fromLink: true, address: m.address, host: m.host });
      focus(m.needsToken ? 'token' : 'primary');
    }
    return;
  }
  if (!d.step || askedIn !== opened) return;
  switch (m.t) {
    case 'syncBucket':
      if (d.step !== 'address') return;
      d.busy = false;
      if (m.error) {
        const p = problem(m);
        d[p.field + 'Error'] = p.text;
        focus(p.field);
        return;
      }
      d.step = m.new ? 'choose' : 'enter';
      focus('pass');
      return;
    case 'syncConnected':
      if (d.step !== 'choose' && d.step !== 'enter') return;
      d.busy = false;
      if (!m.error) return shut();
      if (m.wrongPassphrase) {
        d.passError = "That's not the sync passphrase";
        d.wrong = true;
      } else {
        d.passError = problem(m).text;
      }
      focus('pass');
      return;
    case 'syncPassphraseChanged':
      if (d.step !== 'change') return;
      d.busy = false;
      if (!m.error) return shut();
      d.passError = problem(m).text;
      focus('pass');
      return;
    case 'syncDisconnected':
      if (d.step === 'turnOff') shut();
      return;
    case 'syncSuggested':
      if (d.step !== 'choose' && d.step !== 'change') return;
      Object.assign(d, { pass: m.passphrase, passShown: true, again: '' });
      focus('again');
  }
}
onMounted(() => syncAnswers.add(heard));
onUnmounted(() => syncAnswers.delete(heard));

const title = computed(
  () =>
    ({
      address: d.fromLink ? `Sync with ${d.host}?` : 'Set up sync',
      choose: 'Choose a sync passphrase',
      enter: 'Enter your sync passphrase',
      change: 'Change sync passphrase',
      turnOff: 'Turn off sync on this device?',
    })[d.step],
);
const primaryLabel = computed(
  () =>
    ({
      address: d.busy ? 'Connecting…' : 'Connect',
      choose: 'Turn on sync',
      enter: 'Turn on sync',
      change: 'Change',
      turnOff: 'Turn off',
    })[d.step],
);
// While Connect waits, there is nothing to press but Cancel.
const connectingNow = computed(() => d.busy && d.step === 'address');
</script>

<template>
  <section aria-label="Sync">
    <h2 v-if="!titled">Sync</h2>
    <div class="card">
      <Row v-if="s.locked" label="Sync can't run on this device" hint="This profile's saved sync details can't be opened here" :words="words" :title="NO_KEY_DETAIL">
        <button type="button" class="text-button" @click="setUp">Set up again…</button>
      </Row>
      <template v-else-if="s.on">
        <Row :label="s.host" :hint="lastSynced" :words="words" class="server" :title="serverRow === 'unreachable' ? s.error : undefined">
          <template #lead>
            <span class="mark" aria-hidden="true" :style="{ background: tile.background, boxShadow: `inset 0 0 0 1px ${tile.border}`, color: tile.letter }">{{ (s.host[0] || '').toUpperCase() }}</span>
          </template>
          <template #hint>
            <template v-if="serverRow === 'syncing'">Syncing…</template>
            <span v-else-if="serverRow === 'signedOut'" class="err">The server signed this device out</span>
            <template v-else-if="serverRow === 'unreachable'"
              ><span class="err">Couldn't reach the server</span><template v-if="s.lastSynced"> · {{ lastSynced }}</template></template
            >
            <template v-else>{{ lastSynced }}</template>
          </template>
          <span v-if="serverRow === 'syncing'" class="ctl">
            <button type="button" class="text-button quiet" @click="send({ t: 'syncCancel' })">Cancel</button>
            <button type="button" class="text-button quiet apart" @click="again">Retry</button>
          </span>
          <button v-else-if="serverRow === 'signedOut'" type="button" class="text-button" @click="reconnect">Reconnect…</button>
          <button v-else-if="serverRow === 'unreachable'" type="button" class="text-button" @click="again">Try again</button>
          <button v-else type="button" class="text-button" @click="send({ t: 'syncNow' })">Sync now</button>
        </Row>
        <Row label="Change sync passphrase" hint="A device you add then asks for the new one" :words="words">
          <button type="button" class="text-button" @click="change">Change…</button>
        </Row>
        <Row label="Turn off sync" hint="Keeps everything here and on the server" :words="words">
          <button type="button" class="text-button" @click="turnOff">Turn off…</button>
        </Row>
      </template>
      <Row v-else label="Set up sync" hint="Bookmarks, history and typed addresses, through your own sync server" :words="words">
        <button type="button" class="text-button" @click="setUp">Set up…</button>
      </Row>
    </div>

    <Dialog v-if="d.step" class="sync-dialog" :title="title" @close="shut">
      <template v-if="d.step === 'address' && d.fromLink" #title><span class="w">Sync with</span> {{ d.host }}<span class="w">?</span></template>

      <template v-if="d.step === 'address'">
        <div class="ui-dialog-part">
          <p v-if="d.fromLink">A link asked to set up sync with this server. Connect only if it's yours.</p>
          <p v-else>Paste the address from your sync server.</p>
        </div>
        <div class="ui-dialog-part">
          <label for="sync-address">Address</label>
          <div class="address">
            <input
              id="sync-address"
              ref="addressField"
              v-model="d.address"
              class="text-field"
              :class="{ invalid: d.addressError, dressed }"
              type="text"
              inputmode="url"
              spellcheck="false"
              autocomplete="off"
              autocapitalize="off"
              placeholder="https://"
              :readonly="d.fromLink"
              :aria-invalid="d.addressError ? 'true' : undefined"
              :title="d.addressError && d.detail ? d.detail : undefined"
              @focus="addressFocused = true"
              @blur="addressFocused = false"
              @keydown.enter.prevent="primary"
            />
            <span v-if="dressed" class="dress" aria-hidden="true"
              ><span class="mut">{{ addressParts.scheme }}</span>{{ addressParts.host }}<span class="mut">{{ addressParts.rest }}</span></span
            >
          </div>
          <div v-if="d.addressError" class="text-field-error" role="alert">{{ d.addressError }}</div>
        </div>
        <div class="ui-dialog-part">
          <label for="sync-token">Token</label>
          <input
            id="sync-token"
            ref="tokenField"
            v-model="d.token"
            class="text-field"
            :class="{ invalid: d.tokenError }"
            type="text"
            spellcheck="false"
            autocomplete="off"
            autocapitalize="off"
            :placeholder="TOKEN_PLACEHOLDER"
            :aria-invalid="d.tokenError ? 'true' : undefined"
            @keydown.enter.prevent="primary"
          />
          <div v-if="d.tokenError" class="text-field-error" role="alert">{{ d.tokenError }}</div>
        </div>
      </template>

      <template v-else-if="d.step === 'choose' || d.step === 'change' || d.step === 'enter'">
        <div class="ui-dialog-part">
          <p v-if="d.step === 'choose'">Nothing is synced on {{ d.host }} yet. Every device you add asks for this passphrase.</p>
          <p v-else-if="d.step === 'enter'">{{ d.host }} already has synced data. Type the passphrase you chose for it.</p>
          <p v-else>Devices that sync now keep syncing. A device you add asks for the new one.</p>
        </div>
        <div class="ui-dialog-part">
          <label for="sync-pass">{{ d.step === 'change' ? 'New passphrase' : 'Passphrase' }}</label>
          <div class="secret">
            <input
              id="sync-pass"
              ref="passField"
              v-model="d.pass"
              class="text-field"
              :class="{ invalid: d.passError }"
              :type="d.passShown ? 'text' : 'password'"
              spellcheck="false"
              autocomplete="off"
              autocapitalize="off"
              :aria-invalid="d.passError ? 'true' : undefined"
              :title="d.passError && d.detail && !d.wrong ? d.detail : undefined"
              @keydown.enter.prevent="primary"
            />
            <button type="button" class="show" :aria-pressed="String(d.passShown)" @click="d.passShown = !d.passShown">{{ d.passShown ? 'Hide' : 'Show' }}</button>
          </div>
          <div v-if="d.passError || d.step !== 'enter'" class="under">
            <span v-if="d.passError" class="text-field-error" role="alert">{{ d.passError }}</span>
            <span v-else-if="d.step !== 'enter'" class="secondary">{{ strength }}</span>
            <button v-if="d.step !== 'enter'" type="button" class="link" @click="suggest">Suggest one</button>
          </div>
          <div v-if="d.wrong" class="under"><span class="secondary">Forgot it? Set a new one on a device that's still syncing.</span></div>
        </div>
        <div v-if="d.step !== 'enter'" class="ui-dialog-part">
          <label for="sync-again">Type it again</label>
          <div class="secret">
            <input
              id="sync-again"
              ref="againField"
              v-model="d.again"
              class="text-field"
              :class="{ invalid: d.againError }"
              :type="d.againShown ? 'text' : 'password'"
              spellcheck="false"
              autocomplete="off"
              autocapitalize="off"
              :aria-invalid="d.againError ? 'true' : undefined"
              @keydown.enter.prevent="primary"
            />
            <button type="button" class="show" :aria-pressed="String(d.againShown)" @click="d.againShown = !d.againShown">{{ d.againShown ? 'Hide' : 'Show' }}</button>
          </div>
          <div v-if="d.againError" class="text-field-error" role="alert">{{ d.againError }}</div>
        </div>
        <div v-if="d.step === 'choose'" class="ui-dialog-part">
          <p>If you forget it, the server can't recover it. A device that's still syncing can set a new one.</p>
        </div>
      </template>

      <div v-else-if="d.step === 'turnOff'" class="ui-dialog-part">
        <p>Bookmarks, history and typed addresses stay here and on {{ s.host }}. To sync again, you'll need the sync passphrase.</p>
      </div>

      <template #buttons>
        <button type="button" class="text-button" @click="shut">Cancel</button>
        <button ref="primaryButton" type="button" class="text-button primary" :disabled="connectingNow" @click="primary">{{ primaryLabel }}</button>
      </template>
    </Dialog>
  </section>
</template>

<style scoped>
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

.err {
  color: var(--danger);
}

/* A link's Dialog names the host in bold, its own words plain. */
.w {
  font-weight: 400;
}

/* A row's two quiet buttons, 8px apart, Retry a little further. */
.ctl {
  flex: none;
  display: flex;
  align-items: center;
  gap: 8px;
}

.apart {
  margin-left: 8px;
}

.sync-dialog .text-field {
  width: 100%;
}

/* The address while it isn't typed in: its host in the text's colour, the rest muted, drawn
   over the field, whose own text gives way. */
.address {
  position: relative;
}

.dressed {
  color: transparent;
}

.dress {
  position: absolute;
  inset: 0;
  padding: 0 9px;
  overflow: hidden;
  line-height: 30px;
  white-space: nowrap;
  text-overflow: ellipsis;
  pointer-events: none;
}

.mut {
  color: var(--muted);
}

/* A passphrase, with Show or Hide at its end, inside the field. */
.secret {
  position: relative;
}

.secret .text-field {
  padding-right: 56px;
}

.show {
  position: absolute;
  top: 4px;
  right: 4px;
  height: 22px;
  padding: 0 6px;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--icon);
  font: inherit;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
}

.show:hover {
  background: var(--accent-tint);
}

.show:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

/* Under a field: its error or its line, the strength, and Suggest one at the right. */
.under {
  display: flex;
  align-items: baseline;
  gap: 12px;
  margin-top: 4px;
}

.under > :first-child {
  flex: 1;
  min-width: 0;
  margin-top: 0;
}

.under .link {
  flex: none;
  margin-left: auto;
  font-size: 12px;
  font-weight: 500;
}

/* A phone: buttons drawn at the touch size, 40px, each a 48px target, as the Mobile shell's
   pill and its field are; fields 48px. The button is the target, and its look is drawn 4px
   inside it, top and bottom. */
.touch .text-button {
  position: relative;
  isolation: isolate;
  height: 48px;
  min-height: 48px;
  margin: -4px 0;
  border: 0;
  background: none;
}

.touch .text-button::before {
  content: '';
  position: absolute;
  inset: 4px 0;
  z-index: -1;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
}

.touch .text-button.primary::before {
  border-color: var(--accent);
  background: var(--accent);
}

/* A touch screen has no hover. */
.touch .text-button:hover:not(:disabled) {
  background: none;
}

.touch .text-button.quiet::before {
  border-color: transparent;
  background: none;
}

.touch .text-button:focus-visible {
  box-shadow: none;
}

.touch .text-button:focus-visible::before {
  box-shadow: var(--focus-ring);
}

.touch .sync-dialog .text-field {
  height: 48px;
  padding: 0 12px;
}

.touch .secret .text-field {
  padding-right: 68px;
}

.touch .dress {
  padding: 0 13px;
  line-height: 48px;
}

.touch .show {
  top: 0;
  right: 0;
  height: 48px;
  padding: 0 14px;
  font-size: 13px;
}
</style>
