import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { send, state } from './state.js';

// What a failed run was, as the page says it, by the name Go gives it (sync.Problem). A
// failure with no name is no more than that sync failed: its own words are in the row's
// tooltip and on Sync details.
const FAILED = {
  unreachable: "Couldn't reach the server",
  server: 'The server answered with an error',
  stalled: 'Stopped: nothing arrived for 30 seconds',
  notBucket: 'The server has no sync bucket at this address',
  token: 'The server signed this device out',
};
export const failedAs = (problem) => FAILED[problem] || "Couldn't sync";

// A sync run as a row shows it (docs/sync.md): the Sync card's server row and Sync details'
// state row say the same of it and offer the same. Call it in a component's setup.
export function useSyncRun() {
  const s = computed(() => state.sync);

  // The time, each second while the row shows: the syncing row waits a second before it
  // shows, and "Last synced" moves on. A run goes on saying it syncs for as long as it does:
  // Go ends one that stalls, and says so.
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
  // A run a person asked for says so at once, and for a second at least, so one too quick to
  // see still answers the press; it then reads Last synced just now, or what went wrong. For
  // that second Sync now stays under the finger, disabled, so a second tap lands on nothing;
  // Cancel and Retry come only if the run is still going after it.
  const asked = ref(false);
  const holding = ref(false);
  let held = 0;
  function syncNow() {
    asked.value = holding.value = true;
    clearTimeout(held);
    held = setTimeout(() => {
      holding.value = false;
      if (!s.value.syncing) asked.value = false;
      now.value = Date.now();
    }, 1000);
    send({ t: 'syncNow' });
  }
  watch(
    () => s.value.syncing,
    (on) => {
      if (!on && !holding.value) asked.value = false;
    },
  );
  onUnmounted(() => clearTimeout(held));
  const syncingShown = computed(() => asked.value || syncingFor.value >= 1000);

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

  // The row: what it says and what it offers. A failed run says what it was.
  const serverRow = computed(() => {
    if (syncingShown.value) return 'syncing';
    if (s.value.problem === 'token') return 'signedOut';
    if (s.value.error) return 'failed';
    return 'synced';
  });
  const failed = computed(() => failedAs(s.value.problem));
  // Rows that couldn't be sent while everything else synced: Sync details says which and why.
  const unsent = computed(() => {
    const n = s.value.unsent || 0;
    return n ? `${n.toLocaleString('en-US')} ${n === 1 ? 'item' : 'items'} couldn't be sent` : '';
  });

  // Retry, and Try again after a run with nothing back, start the run again.
  function again() {
    send({ t: 'syncCancel' });
    syncNow();
  }

  return { s, now, holding, ago, lastSynced, serverRow, failed, unsent, syncNow, again };
}
