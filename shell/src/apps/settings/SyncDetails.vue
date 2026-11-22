<script setup>
import { computed, inject, onMounted, onUnmounted, ref, watch } from 'vue';
import { phone } from '@/phone.js';
import { size } from '@/ui/size.js';
import Row from './Row.vue';
import { send, state } from './state.js';
import { failedAs, useSyncRun } from './syncRun.js';

// Sync details (docs/sync-details.md): a page of its own, from a row of the Sync card, that
// shows what sync holds right now and nothing more: the server and the state, what syncs
// with its counts, what waits to go out, and the runs since the app started. Go stores
// nothing for it, so the page asks, and asks again as things change.

const go = inject('settingsGo');
const { s, now, holding, ago, serverRow, failed, setUpAgain, unsent, syncNow, again } = useSyncRun();
const d = computed(() => state.syncDetails);

// Asked as the page opens, whenever the status changes (a run began or ended), every ten
// seconds, which is how often sync itself looks for changes, as the queue grows while a
// person browses, and every two while a run goes on, to say what it waits for.
const ask = () => send({ t: 'syncDetails' });
let asking = 0;
let ticks = 0;
onMounted(() => {
  ask();
  asking = setInterval(() => {
    if (s.value.syncing || ++ticks % 5 === 0) ask();
  }, 2000);
});
onUnmounted(() => clearInterval(asking));
watch(() => state.sync, ask);

// Reconnect… is the Sync card's set-up, so the page goes back to it.
function reconnect() {
  state.syncReconnect = true;
  go('sync');
}

const KINDS = { bookmarks: 'Bookmarks', history: 'History', typed: 'Typed addresses', passkeys: 'Passkeys', logins: 'Passwords' };
const ONE = { bookmarks: 'Bookmark', history: 'History', typed: 'Typed address', passkeys: 'Passkey', logins: 'Password' };
const SMALL = { bookmarks: 'bookmarks', history: 'history', typed: 'typed addresses', passkeys: 'passkeys', logins: 'passwords' };
const WHY = { asked: 'Sync now', change: 'A change here', timer: 'Scheduled', start: 'Starting the app', on: 'Turning sync on', passphrase: 'Changing the sync passphrase', backupCode: 'Changing the backup code' };

const count = (n) => n.toLocaleString('en-US');
const two = (n) => String(n).padStart(2, '0');
const clock = (ms) => {
  const t = new Date(ms);
  return `${two(t.getHours())}:${two(t.getMinutes())}`;
};
// "Today, 14:32", "Yesterday, 21:05", "18 November, 09:12".
function day(ms) {
  const t = new Date(ms);
  const today = new Date(now.value);
  const yesterday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 1);
  if (t.toDateString() === today.toDateString()) return 'Today';
  if (t.toDateString() === yesterday.toDateString()) return 'Yesterday';
  return t.toLocaleDateString('en-GB', { day: 'numeric', month: 'long' });
}
const when = (ms) => `${day(ms)}, ${clock(ms)}`;
const small = (text) => text[0].toLowerCase() + text.slice(1);
// An address without "https://", as the Downloads list shows one; any other scheme says
// something, and stays.
const plain = (name) => name.replace(/^https:\/\//, '');

// How long something took: "180 ms", "2.1 s", "12 s".
const took = (ms) => (ms < 1000 ? `${ms} ms` : ms < 10000 ? `${(ms / 1000).toFixed(1)} s` : `${Math.round(ms / 1000)} s`);

// How long something has gone on: "8 s", "1 min 12 s".
const lasted = (ms) => {
  const sec = Math.max(0, Math.round(ms / 1000));
  return sec < 60 ? `${sec} s` : `${Math.floor(sec / 60)} min ${sec % 60} s`;
};

// The state's row: what it is called, its line, and when sync tries next after a failure.
const stateLabel = computed(() => (serverRow.value === 'signedOut' || serverRow.value === 'failed' ? failed.value : 'Sync is on'));
const stateLine = computed(() => {
  if (serverRow.value === 'syncing') return 'Syncing…';
  if (serverRow.value === 'synced') return unsent.value ? `${unsent.value}: the rest is synced` : 'After a change here, and every 5 minutes';
  if (serverRow.value === 'signedOut') return '';
  const min = d.value?.next ? Math.ceil((d.value.next - now.value) / 60000) : 0;
  return min > 0 ? `Trying again in ${min} ${min === 1 ? 'minute' : 'minutes'}` : '';
});
const lastSynced = computed(() => (s.value.lastSynced ? `${when(s.value.lastSynced)} · ${ago(s.value.lastSynced)}` : 'Not synced yet'));
const nextSync = computed(() => {
  if (s.value.syncing) return 'Now';
  const next = d.value?.next || 0;
  if (serverRow.value === 'signedOut') return s.value.problem === 'reset' ? 'After you set sync up again' : 'After you reconnect';
  if (serverRow.value === 'failed') return next > now.value ? `About ${clock(next)}` : 'Soon';
  return next > now.value ? `About ${clock(next)}, or sooner after a change here` : 'After a change here';
});

// The queue's rows: what it is, by what name, and since when.
const waiting = computed(() =>
  (d.value?.waiting || []).map((w) => ({
    kind: ONE[w.collection] || w.collection,
    name: w.all ? 'Everything cleared here' : w.deleted ? 'One deleted here' : plain(w.name),
    gone: w.deleted,
    // A phone's line under the name; one that couldn't be sent says so after it, in red.
    line: [ONE[w.collection] || w.collection, w.deleted && !w.all ? 'deleted' : '', !w.error && w.since ? `since ${clock(w.since)}` : ''].filter((x) => x).join(' · '),
    since: w.since ? `Since ${clock(w.since)}` : '',
    // One sync couldn't send: set aside, with the error's own words under it.
    stuck: w.error || '',
  })),
);

// The runs' rows, the one going on first.
const sum = (counts) => Object.values(counts || {}).reduce((a, b) => a + b, 0);
function did(r) {
  if (r.cut) return 'Cancelled';
  if (r.error) return failedAs(r.problem);
  const got = sum(r.received);
  const went = sum(r.sent);
  // What it couldn't send, or didn't take, comes after what it did.
  const left = [
    (r.unsent || []).length ? `${count(r.unsent.length)} couldn't be sent` : '',
    (r.untaken || []).length ? `${count(r.untaken.length)} not taken` : '',
  ].filter((x) => x);
  const moved = got && went ? `Received ${count(got)}, sent ${count(went)}` : got ? `Received ${count(got)}` : went ? `Sent ${count(went)}` : '';
  if (left.length) return [moved, ...left].filter((x) => x).join(', ').replace(/^./, (c) => c.toUpperCase());
  return moved || 'Nothing new';
}
// The rows a run couldn't send, or the server's it didn't take, each with why.
const aside = (items, here) =>
  (items || []).map((it) => ({ kind: ONE[it.collection] || it.collection, name: it.deleted ? `One deleted ${here ? 'here' : 'on another device'}` : plain(it.name || ''), gone: it.deleted, why: it.error }));
// What a run moved one way: its named items, then how many more, by kind when of several.
function moved(counts, items, here) {
  const total = sum(counts);
  const left = { ...counts };
  const rows = (items || []).map((it) => {
    left[it.collection]--;
    const where = here ? 'here' : 'on another device';
    return { kind: ONE[it.collection] || it.collection, name: it.all ? `Everything cleared ${where}` : it.deleted ? `One deleted ${where}` : plain(it.name), gone: it.deleted };
  });
  const more = total - rows.length;
  const kinds = Object.entries(left).filter(([, n]) => n > 0);
  const by = kinds.length > 1 ? `: ${kinds.map(([c, n]) => `${count(n)} ${SMALL[c] || c}`).join(', ')}` : '';
  return { total, rows, more: more > 0 ? `and ${count(more)} more${by}` : '' };
}
// A request as a line of a run's list: when, what, the answer, its size and how long.
const requestLine = (q) =>
  [`${clock(q.at)}:${two(new Date(q.at).getSeconds())}.${String(q.at % 1000).padStart(3, '0')}`, q.method.padEnd(4), q.path.padEnd(4), (q.status ? String(q.status) : 'no answer').padEnd(3), size(q.out + q.in).padStart(7), took(q.took).padStart(7)].join('  ');
const requestCount = (n) => `${count(n)} ${n === 1 ? 'request' : 'requests'}`;
const runs = computed(() => {
  const out = [];
  // The run going on says how long it has gone on, and what it is doing: which request it
  // waits on and since when, or that it works on what arrived.
  const running = d.value?.running;
  if (running && s.value.syncing) {
    const n = running.requestCount || 0;
    const a = running.asking;
    const requests = (running.requests || []).map(requestLine);
    out.push({
      id: 'now',
      when: 'Now',
      why: WHY[running.why] || '',
      did: `Syncing… for ${lasted(now.value - running.began)}`,
      quiet: true,
      took: '',
      said: `${n ? `${requestCount(n)} answered` : 'No request answered yet'} · ${a ? `waiting for ${a.method} ${a.path}, asked ${lasted(now.value - a.at)} ago` : n ? 'working on what arrived' : 'starting'}`,
      panel: n
        ? { several: '', received: null, sent: null, requestsHead: `${requestCount(n)} so far · ${size(running.out)} sent · ${size(running.in)} received`, requests, moreRequests: n > requests.length ? `and ${count(n - requests.length)} more` : '' }
        : null,
    });
  }
  for (const r of d.value?.runs || []) {
    const several = r.times > 1;
    const sameDay = new Date(r.began).toDateString() === new Date(r.last).toDateString();
    const result = did(r);
    const requests = (r.requests || []).map(requestLine);
    const unanswered = r.requestCount === 1 && r.requests?.[0]?.status === 0;
    out.push({
      id: String(r.began),
      when: several ? `${when(r.began)} to ${sameDay ? clock(r.last) : small(when(r.last))}` : when(r.began),
      why: `${WHY[r.why] || ''}${several ? `, ${count(r.times)} times` : ''}`,
      did: result,
      // Only an error is coloured; the text's own colour is for a run that moved something.
      quiet: !r.error && (r.cut || result === 'Nothing new'),
      bad: !!r.error || !!(r.unsent || []).length || !!(r.untaken || []).length,
      took: several ? '' : took(r.took),
      // A failed run's own words, under its row, with nothing to open.
      said: r.error || '',
      words: true,
      panel: {
        several: several ? `Each of the ${count(r.times)} ended the same way, the last at ${clock(r.last)}` : '',
        received: sum(r.received) || sum(r.sent) ? moved(r.received, r.receivedItems, false) : null,
        sent: sum(r.received) || sum(r.sent) ? moved(r.sent, r.sentItems, true) : null,
        unsent: aside(r.unsent, true),
        untaken: aside(r.untaken, false),
        requestsHead: unanswered ? '1 request, no answer' : `${requestCount(r.requestCount)} · ${size(r.out)} sent · ${size(r.in)} received`,
        requests,
        moreRequests: r.requestCount > requests.length ? `and ${count(r.requestCount - requests.length)} more` : '',
      },
    });
  }
  return out;
});
const opened = ref('');
const toggle = (id) => (opened.value = opened.value === id ? '' : id);
</script>

<template>
  <section aria-label="Sync details" class="sync-details" :class="{ phone }">
    <div v-if="!s.on" class="card">
      <Row label="Sync is off" hint="Set it up under Sync" />
    </div>
    <template v-else>
      <div class="card">
        <Row :label="stateLabel" :class="{ bad: serverRow === 'signedOut' || serverRow === 'failed' }" class="state">
          <template v-if="stateLine" #hint
            ><span :class="{ err: serverRow === 'synced' && unsent }">{{ stateLine }}</span></template
          >
          <span v-if="serverRow === 'syncing' && !holding" class="ctl">
            <button type="button" class="text-button quiet" @click="send({ t: 'syncCancel' })">Cancel</button>
            <button type="button" class="text-button quiet apart" @click="again">Retry</button>
          </span>
          <button v-else-if="serverRow === 'signedOut'" type="button" class="text-button" @click="reconnect">{{ setUpAgain }}</button>
          <button v-else-if="serverRow === 'failed'" type="button" class="text-button" @click="again">Try again</button>
          <button v-else type="button" class="text-button" :disabled="holding" @click="syncNow">Sync now</button>
        </Row>
        <div v-if="serverRow === 'signedOut' || serverRow === 'failed'" class="row kv wrong">
          <span class="label">What went wrong</span><span class="val said words">{{ s.error }}</span>
        </div>
        <div class="row kv">
          <span class="label">Server</span>
          <span class="val server"
            ><span class="mut">{{ d?.scheme }}</span
            >{{ d?.host || s.host }}<span class="mut">{{ d?.rest }}</span></span
          >
        </div>
        <div class="row kv">
          <span class="label">Last synced</span><span class="val">{{ lastSynced }}</span>
        </div>
        <div class="row kv">
          <span class="label">Next sync</span><span class="val">{{ nextSync }}</span>
        </div>
        <div class="row kv">
          <span class="label">Server changes read</span><span class="val">{{ count(d?.cursor || 0) }}</span>
        </div>
        <div class="row kv">
          <span class="label">What the server holds</span><span class="val">Sealed items only. The key stays on your devices</span>
        </div>
      </div>

      <h2>What syncs</h2>
      <div v-if="phone" class="card kinds">
        <div v-for="k in d?.kinds || []" :key="k.collection" class="row kind" :data-kind="k.collection">
          <span class="label">
            <span class="top"
              ><span>{{ KINDS[k.collection] || k.collection }}</span
              ><span class="n">{{ count(k.items) }}</span></span
            >
            <span class="secondary">{{ k.waiting ? `${count(k.waiting)} waiting to send` : 'Nothing waiting to send' }}{{ k.deletions ? ` · ${count(k.deletions)} ${k.deletions === 1 ? 'deletion' : 'deletions'}` : '' }}</span>
          </span>
        </div>
      </div>
      <div v-else class="card">
        <table class="kinds">
          <thead>
            <tr>
              <th scope="col"><span class="sr">Kind</span></th>
              <th scope="col" class="num">Items</th>
              <th scope="col" class="num" title="Kept so your other devices remove them too">Deletions</th>
              <th scope="col" class="num">Waiting to send</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="k in d?.kinds || []" :key="k.collection" :data-kind="k.collection">
              <th scope="row">{{ KINDS[k.collection] || k.collection }}</th>
              <td class="num">{{ count(k.items) }}</td>
              <td class="num">{{ count(k.deletions) }}</td>
              <td class="num">{{ count(k.waiting) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <h2>Waiting to send</h2>
      <div class="card queue">
        <div v-if="!waiting.length" class="row"><span class="label none">Nothing is waiting</span></div>
        <template v-for="(w, i) in waiting" :key="i">
          <div v-if="phone" class="row wait">
            <span class="label"
              ><span class="one-line plain" :class="{ mut: w.gone }">{{ w.name }}</span
              ><span class="secondary"
                >{{ w.line }}<template v-if="w.stuck"> · <span class="err">couldn't be sent</span></template></span
              ></span
            >
          </div>
          <div v-else class="row wait">
            <span class="k">{{ w.kind }}</span>
            <span class="label one-line plain" :class="{ mut: w.gone }" :title="w.name">{{ w.name }}</span>
            <span v-if="w.stuck" class="val err">Couldn't be sent</span>
            <span v-else class="val mut">{{ w.since }}</span>
          </div>
          <div v-if="w.stuck" class="said under words">{{ w.stuck }}</div>
        </template>
        <div v-if="d?.waitingMore" class="row"><span class="label none">and {{ count(d.waitingMore) }} more</span></div>
      </div>

      <h2>Runs since the app started</h2>
      <div class="card runs">
        <div v-if="!runs.length" class="row"><span class="label none">No runs yet</span></div>
        <template v-for="r in runs" :key="r.id">
          <button type="button" class="row run" :aria-expanded="r.panel ? opened === r.id : undefined" :disabled="!r.panel" @click="toggle(r.id)">
            <span class="label"
              >{{ r.when
              }}<span class="secondary"
                >{{ r.why }}<span class="on-phone"> · <span :class="{ err: r.bad }">{{ small(r.did) }}</span></span></span
              ></span
            >
            <span class="did" :class="{ none: r.quiet, err: r.bad }">{{ r.did }}</span>
            <span class="took">{{ r.took }}</span>
            <svg class="chevron" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3.5 6 8 10.5 12.5 6" /></svg>
          </button>
          <div v-if="r.said" class="said under" :class="{ words: r.words }">{{ r.said }}</div>
          <div v-if="r.panel && opened === r.id" class="panel">
            <h3 v-if="r.panel.several">{{ r.panel.several }}</h3>
            <div v-if="r.panel.received" class="cols">
              <div v-for="side in [{ name: 'Received', m: r.panel.received }, { name: 'Sent', m: r.panel.sent }]" :key="side.name">
                <h3>{{ side.name }} {{ count(side.m.total) }}</h3>
                <div v-for="(it, i) in side.m.rows" :key="i" class="it">
                  <span class="k">{{ it.kind }}</span><span class="one-line" :class="{ mut: it.gone }" :title="it.name">{{ it.name }}</span>
                </div>
                <div v-if="side.m.more" class="more">{{ side.m.more }}</div>
                <div v-if="!side.m.total" class="more">Nothing</div>
              </div>
            </div>
            <div v-for="side in [{ name: 'Couldn\'t be sent', rows: r.panel.unsent || [] }, { name: 'Not taken from the server', rows: r.panel.untaken || [] }].filter((x) => x.rows.length)" :key="side.name" class="aside">
              <h3>{{ side.name }} {{ count(side.rows.length) }}</h3>
              <template v-for="(it, i) in side.rows" :key="i">
                <div class="it">
                  <span class="k">{{ it.kind }}</span><span class="one-line" :class="{ mut: it.gone }" :title="it.name">{{ it.name }}</span>
                </div>
                <div class="said words">{{ it.why }}</div>
              </template>
            </div>
            <div class="reqs">
              <h3>{{ r.panel.requestsHead }}</h3>
              <div v-for="(q, i) in r.panel.requests" :key="i" class="req">{{ q }}</div>
              <div v-if="r.panel.moreRequests" class="more">{{ r.panel.moreRequests }}</div>
            </div>
          </div>
        </template>
      </div>
    </template>
  </section>
</template>

<style scoped>
h2 {
  margin-top: 24px;
}

.mut {
  color: var(--muted);
}

.err,
.state.bad :deep(.name) {
  color: var(--danger);
}

.none {
  color: var(--muted);
  font-weight: 400;
}

.plain {
  font-weight: 400;
}

.one-line {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* A row's two quiet buttons, 8px apart, Retry a little further, as the Sync card's. */
.ctl {
  flex: none;
  display: flex;
  align-items: center;
  gap: 8px;
}

.apart {
  margin-left: 8px;
}

/* A label and its value at the right; on a phone the value goes under its label. */
.val {
  flex: none;
  max-width: 62%;
  text-align: right;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}

.phone .kv {
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
}

.phone .kv .val {
  max-width: 100%;
  color: var(--muted);
  font-size: 13px;
  text-align: left;
}

/* What syncs: a table on a desktop; on a phone a row each, the count at the right end of the
   name's line. */
table.kinds {
  width: 100%;
  border-collapse: collapse;
  font-variant-numeric: tabular-nums;
}

table.kinds th,
table.kinds td {
  height: 44px;
  padding: 0 14px;
  text-align: left;
  font-weight: 400;
}

table.kinds thead th {
  height: 32px;
  color: var(--muted);
  font-size: 12px;
}

table.kinds tbody tr {
  border-top: 1px solid var(--line);
}

table.kinds th[scope='row'] {
  width: 100%;
  padding-left: 16px;
  font-weight: 500;
}

table.kinds .num {
  text-align: right;
  white-space: nowrap;
}

table.kinds tr > :last-child {
  padding-right: 16px;
}

.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}

.kind .top {
  display: flex;
  gap: 12px;
}

.kind .top .n {
  margin-left: auto;
  font-weight: 400;
  font-variant-numeric: tabular-nums;
}

.phone .kind,
.phone .wait,
.phone .run {
  min-height: 56px;
  flex-wrap: nowrap;
}

/* The queue: the kind, the name, since when. */
.wait .k,
.it .k {
  flex: none;
  width: 96px;
  color: var(--muted);
}

/* A run's row is a button that opens it in place: a chevron down, which turns up. */
.run {
  width: 100%;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.run:disabled {
  cursor: default;
}

.run + .run,
.panel + .run,
.said + .run,
.said + .wait,
.row + .run {
  border-top: 1px solid var(--line);
}

.run:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

.did {
  flex: none;
  text-align: right;
}

.took {
  flex: none;
  width: 52px;
  color: var(--muted);
  font-size: 12px;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.chevron {
  flex: none;
  color: var(--icon);
  transition: transform var(--motion) var(--ease);
}

.run:disabled .chevron {
  visibility: hidden;
}

.run[aria-expanded='true'] .chevron {
  transform: rotate(180deg);
}

/* On a phone the row is two lines: when, then why and what it did. */
.on-phone,
.phone .did,
.phone .took {
  display: none;
}

.phone .on-phone {
  display: inline;
}

.panel {
  padding: 12px 16px 14px;
  border-top: 1px solid var(--line);
  background: var(--accent-tint);
  font-size: 13px;
}

.panel h3 {
  margin: 0 0 4px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 500;
}

.cols {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 24px;
}

.phone .cols {
  grid-template-columns: 1fr;
}

.it {
  display: flex;
  gap: 8px;
  min-width: 0;
  line-height: 22px;
}

.more {
  color: var(--muted);
  line-height: 22px;
}

.cols + .reqs,
.cols + .aside,
.aside + .aside,
.aside + .reqs {
  margin-top: 12px;
}

/* What a run says of itself with nothing to open: the run going on what it waits for, a
   failed one its error's own words, which are monospace as a request's line is. */
.said {
  color: var(--muted);
  font-size: 12px;
  line-height: 18px;
  overflow-wrap: anywhere;
  user-select: text;
}

.said.under {
  padding: 0 16px 10px;
  margin-top: -4px;
}

.req,
.said.words {
  font:
    12px/20px ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
}

.req {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: pre;
}

</style>
