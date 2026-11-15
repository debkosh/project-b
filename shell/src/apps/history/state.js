// The History page's copy of what Go keeps (docs/daily-driver.md): the pages visited, the
// latest first, for what is searched. Only Go changes it; the page starts from a snapshot.

import { reactive } from 'vue';
import { bridge } from '@/bridge.js';

const channel = bridge('history');
export const send = channel.send;
export const { mark, firstPaint, inputs } = channel.timeline;

export const state = reactive({
  ready: false,
  theme: 'system',
  tones: null,
  // What is searched for, and the visits that match, each {url, title, lastVisit}, Unix
  // seconds; more is whether an older page of them may follow.
  text: '',
  items: [],
  more: false,
});

// How many visits Go sends at a time: a full page may have more after it.
const page = 100;

export function connect() {
  channel.listen(
    (m) => {
      switch (m.t) {
        case 'snapshot':
          Object.assign(state, { ready: true, theme: m.theme, tones: m.tones, items: m.items || [], more: (m.items || []).length === page });
          break;
        case 'items':
          // An answer for older text than what is searched now is dropped.
          if (m.text !== state.text) break;
          state.items = m.before ? [...state.items, ...(m.items || [])] : m.items || [];
          state.more = (m.items || []).length === page;
          break;
        case 'theme':
          state.theme = m.theme;
          break;
        case 'tones':
          state.tones = m.tones;
          break;
      }
    },
    () => send({ t: 'ready' }),
  );
}
