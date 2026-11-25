<script setup>
import { computed, inject, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import { siteTile } from '@/ui/tint.js';
import { useTints } from '@/ui/useTints.js';
import Row from './Row.vue';
import SyncBackup from './SyncBackup.vue';
import { KEYRING } from './keyring.js';
import KeyringHelp from './KeyringHelp.vue';
import { numbered, send, state, syncAnswers } from './state.js';
import { useSyncRun } from './syncRun.js';

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

// The backup code's rows and its card (SyncBackup.vue). A bucket this device just made
// goes on to the card, as the second step of setting sync up: once the card's rows are
// there, which is once Go has said sync is on.
const backup = ref(null);
let offerBackup = false;
watch(backup, (rows) => {
  if (!rows || !offerBackup) return;
  offerBackup = false;
  rows.offer();
});

const NO_KEY_DETAIL = "The key that seals this profile's secrets isn't on this device. That happens when a profile folder is moved from another device.";
const TOKEN_PLACEHOLDER = 'Only if your server gave you one';
const MIN = 12;

// The server's row says of a run what Sync details' state row says, and offers the same.
const { s, holding, lastSynced, serverRow, failed, setUpAgain, unsent, syncNow, again } = useSyncRun();

// The server's mark: its letter on its tile, as a site's. For a moment after the app starts
// sync is on with its server not opened yet: the tile is the page's quiet one then, with no
// letter, so nothing changes colour when the server comes.
const tints = useTints();
// The row's label is the server, once sync has opened it: until then sync is starting, or
// can't, for a keyring that gave no key (keyring.js).
const serverLabel = computed(() => s.value.host || (s.value.problem === 'keyring' ? "Sync can't start" : 'Sync is starting'));
// What is the bucket's is shown once sync has opened it, and while it isn't reset.
const bucketKnown = computed(() => !!s.value.host && s.value.problem !== 'reset');
// Its hint, while sync is starting, says it comes by itself: the wait after a start is ten
// seconds (docs/startup.md), long enough to wonder whether something is to be done.
const serverHint = computed(() => (s.value.host ? lastSynced.value : 'Starts in a few seconds'));
const tile = computed(() => (tints.value, s.value.host ? siteTile(`https://${s.value.host}`) : { background: 'var(--bar)', border: 'var(--line)', letter: '' }));

// The Dialog. step is '' while it is shut, 'address', 'choose', 'enter', 'change' or
// 'turnOff'; fromLink is the address step as a link opened it. Each thing it asks of Go has a
// number, which the answer brings back (numbered), and the Dialog takes the answer to what
// it asked last and no other: one to what was asked before Cancel is dropped, though it come
// after the Dialog was opened again and has asked anew.
const d = reactive({
  step: '',
  fromLink: false,
  address: '',
  token: '',
  host: '',
  addressError: '',
  tokenError: '',
  tokenWay: false,
  pass: '',
  again: '',
  passShown: false,
  againShown: false,
  tokenShown: false,
  passError: '',
  againError: '',
  wrong: false,
  busy: false,
  detail: '',
  // byCode is the enter step taking the backup code in place of the passphrase; renewed
  // the change step that follows getting in with one.
  byCode: false,
  renewed: false,
});
let asked = 0;
function ask(m) {
  const q = numbered(m);
  asked = q.n;
  send(q);
}

const addressField = ref(null);
const tokenField = ref(null);
const passField = ref(null);
const againField = ref(null);
const primaryButton = ref(null);
const fields = { address: addressField, token: tokenField, pass: passField, again: againField, primary: primaryButton };
const focus = (name) => nextTick(() => fields[name].value?.focus());

function open(o = {}) {
  asked = 0;
  Object.assign(d, {
    step: 'address',
    fromLink: false,
    address: '',
    token: '',
    host: '',
    addressError: '',
    tokenError: '',
    tokenWay: false,
    pass: '',
    again: '',
    passShown: false,
    againShown: false,
    tokenShown: false,
    passError: '',
    againError: '',
    wrong: false,
    busy: false,
    detail: '',
    // The bucket sync is on with, set up anew after a reset: Go holds its address and token.
    anew: false,
    byCode: false,
    renewed: false,
    ...o,
  });
}

function shut() {
  asked = 0;
  d.step = '';
}

// The set-up starts on the server to start with, ours, for a person to change to their own.
const setUp = () => {
  open({ address: state.syncStart });
  focus('address');
};
// The page where a person gets a bucket on that server, in a tab; its Open in Project B comes
// back here as a link, so this Dialog goes.
function getBucket() {
  send({ t: 'syncGetBucket' });
  shut();
}
// Reconnect… after the server signed the device out asks for the token again. Set up again…
// after the bucket was reset asks only for a passphrase: Go still holds the address and the
// token, so it looks at the bucket at once, and an empty one has the first device choose a
// passphrase, one in use has it entered.
const reconnect = () => {
  if (state.sync.problem === 'reset') {
    open({ address: state.syncAddress, host: state.sync.host, anew: true, busy: true });
    ask({ t: 'syncInspect', again: true });
    return;
  }
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
let linkAsked = 0;
function fromLink(link) {
  const q = numbered({ t: 'syncPropose', link });
  linkAsked = q.n;
  send(q);
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

// Sync details' Reconnect… comes back here for the set-up.
watch(
  () => state.syncReconnect,
  (asked) => {
    if (!asked) return;
    state.syncReconnect = false;
    reconnect();
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
  () => {
    d.tokenError = '';
    d.tokenWay = false;
  },
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
    // The server wants a token: the one given isn't it, or none was given.
    case 'token':
      if (d.token.trim()) return { field: 'token', text: "The server didn't accept that token" };
      d.tokenWay = !!startHost.value && hostOf(d.address) === startHost.value;
      return { field: 'token', text: d.tokenWay ? 'This server needs a token.' : 'This server needs a token' };
    case 'unreachable':
      return { field: 'address', text: `Couldn't reach ${host}` };
    case 'server':
      return { field: 'address', text: `${host} answered with an error` };
    case 'stalled':
      return { field: 'address', text: `Nothing arrived from ${host} for 30 seconds` };
    case 'notBucket':
      return { field: 'address', text: "That address isn't a sync bucket" };
    case 'keyring':
      return { field: 'address', text: KEYRING };
    case 'address':
      return { field: 'address', text: isHttps(d.address) ? "That address isn't a sync bucket" : 'Sync needs an https address' };
  }
  return { field: 'address', text: `Couldn't set up sync with ${host}` };
}

const startHost = computed(() => hostOf(state.syncStart));

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
  d.tokenWay = false;
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
  if (d.anew) return ask({ t: 'syncConnect', again: true, passphrase: d.pass });
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
    if (!linkAsked || m.n !== linkAsked) return;
    linkAsked = 0;
    if (m.error) {
      // A link's address that isn't one: the plain set-up, with the address and its error.
      open({ address: m.address || '' });
      d.addressError = problem(m).text;
      focus('address');
    } else {
      // A link may give a token too, which is in its field, hidden, for the person to see.
      open({ fromLink: true, address: m.address, host: m.host, token: m.token || '' });
      focus('primary');
    }
    return;
  }
  if (!d.step || !asked || m.n !== asked) return;
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
      if (!m.error) {
        // In with the backup code: a new passphrase can be chosen now, as nobody who used
        // the code has to know the old one. The code still works.
        if (d.byCode) {
          Object.assign(d, { step: 'change', renewed: true, byCode: false, pass: '', again: '', passShown: false, againShown: false, passError: '', againError: '', wrong: false });
          focus('pass');
          return;
        }
        // A bucket just made: its backup code, as the second step.
        if (d.step === 'choose') {
          offerBackup = true;
          if (backup.value) {
            offerBackup = false;
            backup.value.offer();
          }
        }
        return shut();
      }
      if (m.wrongPassphrase) {
        d.passError = d.byCode ? "That's not this sync's backup code" : "That's not the sync passphrase";
        d.wrong = true;
      } else {
        d.passError = problem(m).text;
      }
      focus('pass');
      return;
    case 'syncBackupScanned':
      if (d.step !== 'enter' || !d.byCode) return;
      if (m.ok) Object.assign(d, { pass: m.code, passError: '', wrong: false });
      focus(m.ok ? 'primary' : 'pass');
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
      enter: d.byCode ? 'Enter your backup code' : 'Enter your sync passphrase',
      change: d.renewed ? 'Choose a new sync passphrase' : 'Change sync passphrase',
      turnOff: 'Turn off sync on this device?',
    })[d.step],
);
// A phone's scanner reads the code's QR into the field; the person still turns sync on.
const scan = () => ask({ t: 'syncBackupScan' });

// The enter step's other way in.
function otherWay() {
  Object.assign(d, { byCode: !d.byCode, pass: '', passError: '', wrong: false, passShown: false });
  focus('pass');
}
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
        <Row :label="serverLabel" :hint="serverHint" :words="words" class="server" :title="serverRow === 'failed' ? s.error : undefined">
          <template #lead>
            <span class="mark" aria-hidden="true" :style="{ background: tile.background, boxShadow: `inset 0 0 0 1px ${tile.border}`, color: tile.letter }">{{ (s.host[0] || '').toUpperCase() }}</span>
          </template>
          <template #hint>
            <template v-if="serverRow === 'syncing'">Syncing…</template>
            <span v-else-if="serverRow === 'signedOut'" class="err">{{ failed }}</span>
            <template v-else-if="serverRow === 'failed'"
              ><span class="err">{{ failed }}</span> <KeyringHelp v-if="s.problem === 'keyring'" /><template v-if="s.lastSynced">
                · {{ lastSynced }}</template
              ></template
            >
            <template v-else
              ><template v-if="unsent"><span class="err">{{ unsent }}</span> · </template>{{ serverHint }}</template
            >
          </template>
          <span v-if="serverRow === 'syncing' && !holding" class="ctl">
            <button type="button" class="text-button quiet" @click="send({ t: 'syncCancel' })">Cancel</button>
            <button type="button" class="text-button quiet apart" @click="again">Retry</button>
          </span>
          <button v-else-if="serverRow === 'signedOut'" type="button" class="text-button" @click="reconnect">{{ setUpAgain }}</button>
          <button v-else-if="serverRow === 'failed'" type="button" class="text-button" @click="again">Try again</button>
          <button v-else type="button" class="text-button" :disabled="holding" @click="syncNow">Sync now</button>
        </Row>
        <Row label="Sync details" hint="What synced, and when" opens="syncDetails" :words="words" />
        <!-- The passphrase and the backup code are the bucket's, known once sync has opened it:
             while it is starting, or can't, neither row is there to say what isn't known. -->
        <Row v-if="bucketKnown" label="Change sync passphrase" hint="A device you add then asks for the new one" :words="words">
          <button type="button" class="text-button" @click="change">Change…</button>
        </Row>
        <SyncBackup v-if="bucketKnown" ref="backup" :words="words + ' backup code'" />
        <Row label="Turn off sync" :hint="s.problem === 'reset' ? 'Keeps everything here' : 'Keeps everything here and on the server'" :words="words">
          <button type="button" class="text-button" @click="turnOff">Turn off…</button>
        </Row>
      </template>
      <Row v-else label="Set up sync" hint="Bookmarks, history, passwords and passkeys, through your own sync server" :words="words">
        <button type="button" class="text-button" @click="setUp">Set up…</button>
      </Row>
    </div>

    <Dialog v-if="d.step" class="sync-dialog" :title="title" @close="shut">
      <template v-if="d.step === 'address' && d.fromLink" #title><span class="w">Sync with</span> {{ d.host }}<span class="w">?</span></template>

      <template v-if="d.step === 'address'">
        <div class="ui-dialog-part">
          <p v-if="d.fromLink">A link asked to set up sync with this server. Connect only if it's yours.</p>
          <template v-else>
            <p>Sync through your own server, or try ours.</p>
            <p class="secondary start">{{ startHost }} is free to try. It can't read your data, and you can host your own.</p>
            <div class="start"><button type="button" class="text-button" @click="getBucket">Get a free sync bucket</button></div>
          </template>
        </div>
        <div class="ui-dialog-part">
          <label for="sync-address">Server address</label>
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
          <div class="secret">
            <input
              id="sync-token"
              ref="tokenField"
              v-model="d.token"
              class="text-field"
              :class="{ invalid: d.tokenError }"
              :type="d.tokenShown ? 'text' : 'password'"
              spellcheck="false"
              autocomplete="off"
              autocapitalize="off"
              :placeholder="TOKEN_PLACEHOLDER"
              :aria-invalid="d.tokenError ? 'true' : undefined"
              @keydown.enter.prevent="primary"
            />
            <button v-if="d.token" type="button" class="show" :aria-pressed="String(d.tokenShown)" @click="d.tokenShown = !d.tokenShown">{{ d.tokenShown ? 'Hide' : 'Show' }}</button>
          </div>
          <div v-if="d.tokenError" class="text-field-error" role="alert">
            {{ d.tokenError }}
            <!-- The server to start with wants a token, and this is where one comes from. -->
            <template v-if="d.tokenWay"><button type="button" class="link" @click="getBucket">Get a free sync bucket</button> to make one.</template>
          </div>
        </div>
      </template>

      <template v-else-if="d.step === 'choose' || d.step === 'change' || d.step === 'enter'">
        <div class="ui-dialog-part">
          <p v-if="d.step === 'choose'">Nothing is synced on <b>{{ d.host }}</b> yet. Every device you add asks for this passphrase.</p>
          <p v-else-if="d.step === 'enter' && d.byCode"><b>{{ d.host }}</b> already has synced data. {{ state.canScan ? 'Type or scan' : 'Type' }} the backup code you saved. Then you can choose a new passphrase.</p>
          <p v-else-if="d.step === 'enter'"><b>{{ d.host }}</b> already has synced data. Type the passphrase you chose for it.</p>
          <p v-else-if="d.renewed">You're in with your backup code, which still works. Choose a new passphrase: a device you add asks for it.</p>
          <p v-else>Devices that sync now keep syncing. A device you add asks for the new one.</p>
        </div>
        <div class="ui-dialog-part">
          <label for="sync-pass">{{ d.step === 'change' ? 'New passphrase' : d.byCode ? 'Backup code' : 'Passphrase' }}</label>
          <div class="secret">
            <input
              id="sync-pass"
              ref="passField"
              v-model="d.pass"
              class="text-field"
              :class="{ invalid: d.passError, code: d.byCode }"
              :type="d.passShown || d.byCode ? 'text' : 'password'"
              spellcheck="false"
              autocomplete="off"
              :autocapitalize="d.byCode ? 'characters' : 'off'"
              :placeholder="d.byCode ? 'XXXX-XXXX-XXXX-XXXX-XXXX-XXXX' : undefined"
              :aria-invalid="d.passError ? 'true' : undefined"
              :title="d.passError && d.detail && !d.wrong ? d.detail : undefined"
              @keydown.enter.prevent="primary"
            />
            <button v-if="!d.byCode" type="button" class="show" :aria-pressed="String(d.passShown)" @click="d.passShown = !d.passShown">{{ d.passShown ? 'Hide' : 'Show' }}</button>
            <!-- A phone's scanner, at the field's end: what it reads goes into the field. -->
            <button v-else-if="state.canScan" type="button" class="scan" aria-label="Scan QR code" title="Scan QR code" @click="scan">
              <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <rect x="2" y="2" width="4.5" height="4.5" rx="0.5" />
                <rect x="9.5" y="2" width="4.5" height="4.5" rx="0.5" />
                <rect x="2" y="9.5" width="4.5" height="4.5" rx="0.5" />
                <path d="M9.5 9.5h1.5v1.5M14 9.5v1.5M9.5 14h1.5M13 13h1v1" />
              </svg>
            </button>
          </div>
          <div v-if="d.passError || d.step !== 'enter'" class="under">
            <span v-if="d.passError" class="text-field-error" role="alert"
              >{{ d.passError }} <KeyringHelp v-if="d.passError === KEYRING"
            /></span>
            <span v-else-if="d.step !== 'enter'" class="secondary">{{ strength }}</span>
            <button v-if="d.step !== 'enter'" type="button" class="link" @click="suggest">Suggest one</button>
          </div>
          <div v-if="d.wrong && !d.byCode" class="under"><span class="secondary">Forgot it? Use your backup code, or set a new one on a device that's still syncing.</span></div>
          <div v-if="d.step === 'enter'" class="other"><button type="button" class="link" @click="otherWay">{{ d.byCode ? 'Use the passphrase instead' : 'Use backup code instead' }}</button></div>
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
        <p>Bookmarks, history, passwords and passkeys stay here and on <b>{{ s.host }}</b>. To sync again, you'll need the sync passphrase.</p>
      </div>

      <template #buttons>
        <button type="button" class="text-button" @click="shut">{{ d.renewed ? 'Not now' : 'Cancel' }}</button>
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
  font-size: calc(18rem / 16);
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
  line-height: calc(30rem / 16);
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
  font-size: calc(12rem / 16);
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
  font-size: calc(12rem / 16);
  font-weight: 500;
}

/* The set-up's offer of a server to start with: what it is, then the way to a bucket on it. */
.start {
  margin-top: 8px;
}

/* The enter step's other way in, on a line of its own under the field. */
.other {
  margin-top: 8px;
}

/* A phone's scanner: an icon button at the field's end, 40px in a 48px field. */
.scan {
  position: absolute;
  top: 4px;
  right: 4px;
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--icon);
  cursor: pointer;
}

.other .link {
  font-size: calc(13rem / 16);
  font-weight: 500;
}

.text-field.code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-variant-numeric: slashed-zero;
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

/* The whole code in the field at a phone's width, 29 characters, beside its scanner. */
.touch .sync-dialog .text-field.code {
  padding-right: 48px;
  font-size: calc(14rem / 16);
}

.touch .dress {
  padding: 0 13px;
  line-height: calc(48rem / 16);
}

.touch .show {
  top: 0;
  right: 0;
  height: 48px;
  padding: 0 14px;
  font-size: calc(13rem / 16);
}
</style>
