// The settings page's copy of what the managers own. It is only ever replaced by what
// they send, and the page starts from a snapshot.

import { reactive } from 'vue';
import { bridge } from '@/bridge.js';
import { extensionsMessage } from '@extensions';
import { importMessage } from '@/ui/imports.js';

const channel = bridge('settings');
export const send = channel.send;
export const { mark, end, firstPaint } = channel.timeline;

export const state = reactive({
  ready: false,
  windowColor: '',
  // Its tones, light and dark, from Go, which the page paints itself with: null until the
  // snapshot.
  tones: null,
  tabLayout: 'vertical',
  // Where a phone shell puts its address bar: 'bottom' or 'top'.
  addressBarPosition: 'bottom',
  // A phone's shells to choose from, and the one in use: {shell, shells}, null off a phone.
  phoneShells: null,
  // What a start opens: 'continue', the windows and tabs as they were, or 'fresh', a window opened
  // as the shell opens one.
  start: 'continue',
  // 'system', 'light' or 'dark', as Go's theme pref says.
  theme: 'system',
  // The search engine typed words go to, by its id, and the list to choose from, each
  // {id, name}.
  searchEngine: '',
  searchEngines: [],
  autoUpdate: true,
  update: { phase: '' },
  // What Clear browsing data last cleared, as Go said, {since, history, siteData, cache,
  // downloads}; null before it has. clearCounts is how many pages each of its ranges
  // would take, by the range's name, as Go last said; null before the dialog asks.
  cleared: null,
  clearCounts: null,
  // The Downloads folder, the whole path, where downloads go while the pref downloadFolder
  // is "".
  downloadsFolder: '',
  // The person's home folder, which Save to shows as ~.
  homeFolder: '',
  // Whether the app is the system's default browser, as the page opened or since it asked.
  isDefault: false,
  running: '',
  // The section showing: the one Go asks for when it opens the page at one, or the one the
  // user picked. '' is the first.
  section: '',
  keyboard: { platform: 'mac', templates: [], template: '', following: true, groups: [], missing: [], gone: [], problems: [] },
  // Every flag with its saved value, whether its build has it, and whether it waits for a
  // relaunch.
  flags: [],
  // The web-features registry for this platform: each feature, what a page gets of it here,
  // and why. Read only.
  webFeatures: [],
  // This profile in the list: its name, the template it was made from or last reset to,
  // the templates, the id of the shell it loads, and the shells to choose from, each with
  // its id and name. null in a copy on a folder named outright, which isn't listed.
  profile: null,
  // The profile's workspaces, each with its id, name and colour, in the user's order.
  workspaces: [],
  // The number of tabs open in each workspace, by its id; one with none isn't in it.
  workspaceTabs: {},
  // Each site's settings, by origin: each {origin, settings}, a setting being {kind, title,
  // value}, value 'allow' or 'block'.
  siteSettings: [],
  // Sites, one row per site, its registrable domain: {site, origins, hosts, cookies,
  // bytes}, its origins with a setting and its hosts whose storage keeps something, in
  // every workspace, as the engine last counted.
  sites: [],
  // The person's sparks (docs/site-features.md): {id, name, description, match, runAt, on,
  // error}; the one whose source Go last sent, {id, source, fileName}; and Go's word on the
  // last save, '' when it took, null before one.
  sparks: [],
  sparkSource: null,
  sparkProblem: null,
  // The kinds of setting settings can set for a site, in order, each {kind, title}.
  siteKinds: [],
  // How each site's open pages reached them, by origin: {connection, note}, connection
  // 'secure' or 'insecure', note why in one line.
  siteConnections: {},
  // Go's prefs, all of them, by their names in the file, as a shell has them: what setPref
  // sets. Bookmarks reads where bookmarks show from them.
  prefs: {},
  // The bookmark tree (docs/bookmark-folders.md), for the bookmarks bar's folder list.
  bookmarks: [],
});

// setPref sets one of Go's prefs by its name in prefs, as a shell's does.
export function setPref(name, value) {
  send({ t: 'setPref', name, value: String(value) });
}

export function connect() {
  channel.listen(
    (m) => {
      extensionsMessage(m);
      if (importMessage(m)) return;
      switch (m.t) {
        case 'snapshot':
          Object.assign(state, {
            windowColor: m.windowColor,
            tones: m.tones,
            tabLayout: m.tabLayout,
            addressBarPosition: m.addressBarPosition,
            phoneShells: m.phoneShells || null,
            theme: m.theme,
            start: m.start,
            searchEngine: m.searchEngine,
            searchEngines: m.searchEngines || [],
            autoUpdate: m.autoUpdate,
            isDefault: !!m.isDefault,
            update: m.update,
            running: m.running,
            keyboard: m.keyboard,
            flags: m.flags,
            webFeatures: m.webFeatures,
            section: m.section,
            profile: m.profile || null,
            workspaces: m.workspaces,
            workspaceTabs: m.workspaceTabs,
            siteSettings: m.siteSettings,
            sites: m.sites || [],
            downloadsFolder: m.downloadsFolder || '',
            homeFolder: m.homeFolder || '',
            sparks: m.sparks || [],
            siteKinds: m.siteKinds,
            siteConnections: m.siteConnections,
            prefs: m.prefs || {},
            bookmarks: m.bookmarks || [],
            ready: true,
          });
          mark('settings.snapshot');
          break;
        case 'searchEngine':
          state.searchEngine = m.searchEngine;
          break;
        case 'start':
          state.start = m.start;
          break;
        case 'theme':
          state.theme = m.theme;
          break;
        case 'tabLayout':
          state.tabLayout = m.tabLayout;
          break;
        case 'addressBarPosition':
          state.addressBarPosition = m.addressBarPosition;
          break;
        case 'phoneShells':
          state.phoneShells = m.phoneShells;
          break;
        case 'autoUpdate':
          state.autoUpdate = m.autoUpdate;
          break;
        case 'update':
          state.update = m.update;
          break;
        case 'dataCleared':
          state.cleared = m;
          break;
        case 'clearDataCounts':
          state.clearCounts = m.pages;
          break;
        case 'isDefault':
          state.isDefault = m.isDefault;
          break;
        case 'windowColor':
          state.windowColor = m.windowColor;
          state.tones = m.tones;
          break;
        case 'section':
          state.section = m.section;
          break;
        case 'keyboard':
          state.keyboard = m.keyboard;
          break;
        case 'flags':
          state.flags = m.flags;
          break;
        case 'profile':
          state.profile = m.profile;
          break;
        case 'workspaces':
          state.workspaces = m.workspaces;
          break;
        case 'workspaceTabs':
          state.workspaceTabs = m.workspaceTabs;
          break;
        case 'siteConnections':
          state.siteConnections = m.siteConnections;
          break;
        case 'sparks':
          state.sparks = m.sparks;
          break;
        case 'sparkSource':
          state.sparkSource = { id: m.id, source: m.source, fileName: m.fileName };
          break;
        case 'sparkSaved':
          state.sparkProblem = m.problem;
          break;
        case 'siteSettings':
          state.siteSettings = m.siteSettings;
          break;
        case 'sites':
          state.sites = m.sites;
          break;
        case 'prefs':
          state.prefs = m.prefs;
          break;
        case 'bookmarks':
          state.bookmarks = m.bookmarks;
          break;
      }
    },
    () => send({ t: 'ready' }),
  );
}
