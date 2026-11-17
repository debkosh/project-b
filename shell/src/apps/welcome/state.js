// The welcome's copy of what Go keeps (docs/onboarding.md): the prefs its steps set, the
// tones of the frame colour, and whether it has its window to itself. Each choice is sent
// to Go as it is made and comes back as a change, so the page, its window and the picture
// of the browser take it together.

import { reactive } from 'vue';
import { bridge } from '@/bridge.js';

const channel = bridge('welcome');
export const send = channel.send;
export const { mark, inputs } = channel.timeline;

export const state = reactive({
  ready: false,
  // Whether the welcome has its window to itself, as a first start opens it, or is in a
  // tab, as Run setup again opens it.
  whole: false,
  phone: false,
  theme: 'system',
  tones: null,
  // Go's prefs, by their names in the file: shell, tabLayout, frameColor, theme,
  // addressBarPosition, searchEngine.
  prefs: {},
  searchEngines: [],
  autoUpdate: true,
  // The ways to work the style step offers, each {id, shell, tabLayout, name}.
  layouts: [],
});

export function setPref(name, value) {
  send({ t: 'setPref', name, value });
}

export function connect() {
  channel.listen(
    (m) => {
      switch (m.t) {
        case 'snapshot':
          Object.assign(state, {
            ready: true,
            whole: m.whole,
            phone: m.phone,
            theme: m.theme,
            tones: m.tones,
            prefs: m.prefs,
            searchEngines: m.searchEngines || [],
            autoUpdate: m.autoUpdate,
            layouts: m.layouts || [],
          });
          break;
        case 'prefs':
          state.prefs = m.prefs;
          break;
        case 'theme':
          state.theme = m.theme;
          break;
        case 'tones':
          state.tones = m.tones;
          break;
        case 'autoUpdate':
          state.autoUpdate = m.autoUpdate;
          break;
      }
    },
    () => send({ t: 'ready' }),
  );
}
