// The welcome's copy of what Go keeps (docs/onboarding.md): the prefs its steps set, the
// tones of the frame colour, and whether it has its window to itself. Each choice is sent
// to Go as it is made and comes back as a change, so the page, its window and the picture
// of the browser take it together.

import { reactive } from 'vue';
import { bridge } from '@/bridge.js';
import { importMessage } from '@/ui/imports.js';

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
  // Whether the app is the default browser, as the system says.
  isDefault: false,
  // The Linux desktop, "gnome", "kde" or "other", whose own settings set the default browser;
  // "" off Linux.
  desktop: '',
  // Whether the app may show its notifications, a download's: a phone asks the person.
  notifications: true,
  // The ways to work the style step offers, each {id, shell, tabLayout, name}.
  layouts: [],
  // The keys How you work offers, each keymap's first keys by action ("Shift+Cmd+T"), the
  // one in use, and the platform they are written for.
  keymaps: {},
  keymap: 'project-b',
  platform: 'mac',
  // The profile's workspaces, each {id, name, color}.
  workspaces: [],
  // The built-in sparks the profile still has, each {id, name, description, on}.
  sparks: [],
  // The Protection step's switches, each {pref, title, about}: a pref of prefs, set with
  // setPref (docs/blocking.md).
  protection: [],
});

export function setPref(name, value) {
  send({ t: 'setPref', name, value });
}

export function connect() {
  channel.listen(
    (m) => {
      if (importMessage(m)) return;
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
            isDefault: m.isDefault,
            desktop: m.desktop || '',
            notifications: m.notifications,
            layouts: m.layouts || [],
            keymaps: m.keymaps || {},
            keymap: m.keymap,
            platform: m.platform,
            workspaces: m.workspaces || [],
            sparks: m.sparks || [],
            protection: m.protection || [],
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
        case 'isDefault':
          state.isDefault = m.isDefault;
          break;
        case 'notifications':
          state.notifications = m.notifications;
          break;
        case 'keymap':
          state.keymap = m.keymap;
          break;
        case 'workspaces':
          state.workspaces = m.workspaces || [];
          break;
        case 'sparks':
          state.sparks = m.sparks || [];
          break;
      }
    },
    () => {
      send({ t: 'ready' });
      send({ t: 'findBrowsers' });
    },
  );
}
