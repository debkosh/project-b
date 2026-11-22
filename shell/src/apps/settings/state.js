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
  // The channel updates come from, 'stable' or 'dev'.
  channel: 'stable',
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
  // The Linux desktop, "gnome", "kde" or "other", whose own settings set the default browser;
  // "" off Linux.
  desktop: '',
  running: '',
  // The section showing: the one Go asks for when it opens the page at one, or the one the
  // user picked. '' is the first.
  section: '',
  // How many times Go has asked for a section since the page started, so asking again for
  // the one showing goes to it again.
  sectionAsked: 0,
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
  // The profile's own passkeys (docs/passkeys.md): how many, and, once the Passkeys page
  // asked for them, each as {id, site, title, secondary, unnamed, created}; null before.
  passkeyCount: 0,
  passkeys: null,
  // The profile's key doesn't open on this device, so its passkeys can't be shown.
  passkeysNotHere: false,
  // The profile's saved logins (docs/passwords.md): how many, and, once the Passwords page
  // has asked, each {id, site, origin, username, created}, with no password. passwordAnswer
  // is Go's last answer to this page's Show or Copy: passwordShown {id, password} or
  // passwordCopied {id, what}, either with error in place of what was asked.
  // passwordsNever is the sites the person said never to offer to save a password on, each
  // {origin, site}, which comes with the list.
  passwordCount: 0,
  passwords: null,
  passwordsNever: [],
  passwordAnswer: null,
  // The person's sparks (docs/site-features.md): {id, name, description, match, runAt, on,
  // error}; the one whose source Go last sent, {id, source, fileName}; and Go's word on the
  // last save, '' when it took, null before one. An import's sparks the profile already
  // has, [{index, file, name, version, keptVersion, renameTo}], and then what it did,
  // {added, replaced, renamed, skipped: [{file, name, reason}]}; and the last export's zip,
  // {data (base64), count, fileName}.
  sparks: [],
  sparkSource: null,
  sparkProblem: null,
  sparkClashes: null,
  sparkImported: null,
  sparkZip: null,
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
  // Sync (docs/sync.md): its status as Go sends it, {on, syncing, host, lastSynced, error,
  // problem, locked}; the bucket's address while it is on, which Reconnect fills in; and a
  // project-b://sync link Go was handed, which opens the set-up, '' once it has.
  sync: { on: false, syncing: false, host: '', lastSynced: 0, error: '', problem: '', locked: false },
  syncAddress: '',
  syncLink: '',
  // Sync details (docs/sync-details.md): what Go last answered syncDetails with, null before;
  // and whether its Reconnect… asked the Sync card to open its set-up.
  syncDetails: null,
  syncReconnect: false,
});

// Go's answers to what Sync asked, each heard by every function here.
export const syncAnswers = new Set();
const syncAnswerTypes = new Set(['syncProposal', 'syncBucket', 'syncConnected', 'syncPassphraseChanged', 'syncDisconnected', 'syncStartedFresh', 'syncSuggested']);

// setPref sets one of Go's prefs by its name in prefs, as a shell's does.
export function setPref(name, value) {
  send({ t: 'setPref', name, value: String(value) });
}

export function connect() {
  channel.listen(
    (m) => {
      extensionsMessage(m);
      if (importMessage(m)) return;
      if (syncAnswerTypes.has(m.t)) {
        for (const f of syncAnswers) f(m);
        return;
      }
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
            channel: m.channel || 'stable',
            isDefault: !!m.isDefault,
            desktop: m.desktop || '',
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
            passkeyCount: m.passkeyCount || 0,
            passwordCount: m.passwordCount || 0,
            downloadsFolder: m.downloadsFolder || '',
            homeFolder: m.homeFolder || '',
            sparks: m.sparks || [],
            siteKinds: m.siteKinds,
            siteConnections: m.siteConnections,
            prefs: m.prefs || {},
            bookmarks: m.bookmarks || [],
            sync: m.sync || state.sync,
            syncAddress: m.syncAddress || '',
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
        case 'channel':
          state.channel = m.channel;
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
          state.sectionAsked++;
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
        case 'sparkClashes':
          state.sparkClashes = m.clashes;
          break;
        case 'sparkImported':
          state.sparkImported = m.imported;
          break;
        case 'sparkZip':
          state.sparkZip = { data: m.data, count: m.count, fileName: m.fileName };
          break;
        case 'siteSettings':
          state.siteSettings = m.siteSettings;
          break;
        case 'sites':
          state.sites = m.sites;
          break;
        case 'passkeyCount':
          state.passkeyCount = m.passkeyCount;
          break;
        case 'passwordCount':
          state.passwordCount = m.passwordCount;
          break;
        case 'passwords':
          state.passwords = m.passwords;
          state.passwordsNever = m.never || [];
          break;
        case 'passwordShown':
        case 'passwordCopied':
          state.passwordAnswer = m;
          break;
        case 'passkeys':
          state.passkeys = m.passkeys;
          state.passkeysNotHere = !!m.notHere;
          break;
        case 'prefs':
          state.prefs = m.prefs;
          break;
        case 'bookmarks':
          state.bookmarks = m.bookmarks;
          break;
        case 'sync':
          state.sync = m.sync;
          state.syncAddress = m.address || '';
          break;
        case 'syncDetails':
          state.syncDetails = m.details;
          break;
        case 'syncLink':
          state.syncLink = m.link;
          break;
      }
    },
    () => send({ t: 'ready' }),
  );
}
