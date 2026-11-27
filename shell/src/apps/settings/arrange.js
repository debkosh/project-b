// What the settings page holds, and how its sections arrange it (docs/settings.md). Every
// setting is a part: a row, a few rows, or a block with cards of its own.

import { computed } from 'vue';
import { extensionsSection } from '@extensions';
import { phone } from '@/phone.js';
import { framePresets } from '@/ui/colours.js';
import { size } from '@/ui/size.js';
import { state } from './state.js';

// Where a part shows: desktop only, phone only, or both; and whether the page has what it
// needs. Parts not named here show everywhere, always.
const where = {
  tabLayout: () => !phone,
  addressBar: () => phone,
  // On a desktop the profile loads the shell, so a copy on a folder named outright has
  // none to choose; Android chooses among its own.
  shell: () => (phone ? !!state.androidShells : !!state.profile),
  profile: () => !!state.profile,
  keys: () => !phone,
  keyboard: () => !phone,
  // A phone has no Save panel: a download goes to its Downloads.
  askWhereToSave: () => !phone,
  // A phone saves to its shared Downloads.
  downloadFolder: () => !phone,
  // A phone's app has no quit of its own: Android ends it when it likes.
  clearOnQuit: () => !phone,
  // Android keeps each app's data from the others, so a phone has no browser to bring
  // things over from.
  import: () => !phone,
  extensions: () => !phone && !!extensionsSection?.shown(),
  // Secure DNS's provider, once it's on, and the custom template while Custom is chosen.
  dnsProvider: () => !!state.prefs.secureDns && state.prefs.secureDns !== 'off',
  dnsCustom: () =>
    !!state.prefs.secureDns && state.prefs.secureDns !== 'off' && (state.prefs.dnsProvider === 'custom' || state.dnsCustomOpen),
  // The Mac has its own menu bar; a phone has none.
  menuBar: () => !phone && state.keyboard.platform !== 'mac',
  // Linux has one Flatpak branch, Stable, with no Dev to choose (docs/research/linux.md). A
  // phone's keys follow Linux's too, but it has both channels.
  channel: () => phone || state.keyboard.platform !== 'linux',
};

export const shown = (part) => !where[part] || where[part]();

// The long lists. Each is a block of its own; a section has it as it is, or as a row that
// opens it as a page of its own, with a back link. The row's words, and what
// it shows on its right, are here.
const words = (n, one, many) => `${n} ${n === 1 ? one : many}`;
export const lists = {
  workspaces: {
    label: 'Workspaces',
    hint: 'Sets of tabs, each with its own cookies and site data',
    summary: () => state.workspaces.map((w) => w.name).join(', '),
  },
  sites: {
    label: 'Sites',
    hint: 'Cookies and site data, camera, microphone, notifications, each site\'s ad blocker and links',
    summary: () => {
      if (!state.sites.length) return '';
      const bytes = state.sites.reduce((n, s) => n + s.bytes, 0);
      return [words(state.sites.length, 'site', 'sites'), bytes ? size(bytes) : ''].filter((x) => x).join(' · ');
    },
  },
  passwords: {
    label: 'Passwords',
    hint: 'The logins Project B keeps for your sites',
    summary: () => (state.passwordCount ? words(state.passwordCount, 'password', 'passwords') : ''),
  },
  passkeys: {
    label: 'Passkeys',
    hint: 'The passkeys Project B keeps for your sites',
    summary: () => (state.passkeyCount ? words(state.passkeyCount, 'passkey', 'passkeys') : ''),
  },
  keyboard: {
    label: 'Keyboard shortcuts',
    hint: 'Every action\'s keys, and whether it is in the menu',
    summary: () => '',
  },
  sparks: {
    label: 'Sparks',
    hint: 'Your own scripts for the pages they match',
    summary: () => {
      const on = state.sparks.filter((s) => s.on).length;
      return state.sparks.length ? `${on} on` : '';
    },
  },
  extensions: {
    label: 'Extensions',
    hint: 'From the Chrome Web Store, or loaded from a folder',
    summary: () => '',
  },
  // Reached from a row of the Sync card, a block, which within names.
  syncDetails: {
    label: 'Sync details',
    hint: 'What synced, and when',
    summary: () => '',
    within: 'sync',
  },
  webFeatures: {
    label: 'Web features',
    hint: 'How web features behave in Project B, and why',
    summary: () => '',
  },
};

// While search has words, a block that isn't made of rows shows as a row of its own, which
// goes to it: each list, and these.
export const blockRows = {
  ...lists,
  import: {
    label: 'Import from other browsers',
    hint: 'Bookmarks, history, passwords and extensions, from another browser or a file',
    summary: () => '',
  },
};

// A card's parts that show here, a list's as its row; a section's cards that have any.
function arranged(sections) {
  return sections
    .map((s) => ({
      ...s,
      cards: s.cards
        .map((c) => {
          if (typeof c === 'string') return shown(c) ? c : null;
          const rows = (Array.isArray(c) ? c : c.rows).filter(shown);
          if (!rows.length) return null;
          return Array.isArray(c) ? rows : { ...c, rows };
        })
        .filter((c) => c),
    }))
    .filter((s) => s.cards.length);
}

// The sections, in a list, like Chrome's, one showing at a time (docs/settings.md); each has
// an icon and a line saying what it is set to, which a phone's list of sections shows. Below
// the line at the foot of the list, what is still being tried. A list is in its section as
// it is, but Sites, which is in Privacy with rows of its own, opens as its own page. Each
// name fits a phone's row on one line.
const named = (list, value) => list.find((x) => x.value === value)?.label || '';
const themeNames = [
  { value: 'system', label: 'System' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
];
const startNames = [
  { value: 'continue', label: 'Where you left off' },
  { value: 'fresh', label: 'Start fresh' },
];
const sectionList = () => [
  { id: 'profile', label: 'Profile', icon: 'profile', cards: [['profile'], 'import'], summary: () => state.profile?.name || '' },
  { id: 'syncing', label: 'Sync', icon: 'sync', cards: ['sync'], summary: () => (state.sync.on ? state.sync.host || 'On' : 'Off') },
  {
    id: 'logins',
    label: 'Passwords',
    icon: 'key',
    cards: [['passwords', 'passkeys']],
    summary: () => [lists.passwords.summary(), lists.passkeys.summary()].filter((x) => x).join(' · ') || 'None yet',
  },
  {
    id: 'appearance',
    label: 'Appearance',
    icon: 'appearance',
    cards: [['theme', 'frameColour', 'tabLayout', 'addressBar', 'shell', 'menuBar']],
    summary: () => [named(themeNames, state.theme), colourName(state.windowColor)].join(' · '),
  },
  {
    id: 'start',
    label: 'Start and search',
    icon: 'search',
    cards: [['start', 'searchEngine']],
    summary: () => [named(startNames, state.start), state.searchEngines.find((e) => e.id === state.searchEngine)?.name].filter((x) => x).join(' · '),
  },
  { id: 'workspaces', label: 'Workspaces', icon: 'workspaces', cards: ['workspaces'], summary: lists.workspaces.summary },
  {
    id: 'bookmarks',
    label: 'Bookmarks',
    icon: 'bookmarks',
    cards: ['bookmarks'],
    summary: () => {
      const p = state.prefs;
      const on = [p.bookmarksNewTab && 'New tab page', p.bookmarksSidebar && (phone ? 'Tab switcher' : 'Sidebar'), p.bookmarksCommandBox && (phone ? 'Address bar' : 'Command box'), p.bookmarksBar && 'Bar'];
      return on.filter((x) => x).join(', ') || 'Nowhere';
    },
  },
  {
    id: 'downloads',
    label: 'Downloads',
    icon: 'download',
    cards: [['downloadFolder', 'askWhereToSave', 'nameDownloads']],
    summary: () => {
      // A phone saves to its Downloads and has no Save panel: the section is how its
      // downloads are named.
      if (phone) return { title: 'Named by title and file name', file: "Named by the file's own name" }[state.prefs.nameDownloads] || 'Named automatically';
      if (state.prefs.askWhereToSave) return 'Ask where to save each file';
      const f = state.prefs.downloadFolder;
      return f ? `To ${f.split('/').filter((x) => x).pop() || f}` : 'To the Downloads folder';
    },
  },
  {
    id: 'privacy',
    label: 'Privacy and sites',
    icon: 'privacy',
    cards: [['clearData', 'blockAds', 'clearOnQuit', 'sites'], ['secureDns', 'dnsProvider', 'dnsCustom']],
    summary: () => [state.prefs.blockAds ? 'Ads blocked' : 'Ads shown', lists.sites.summary()].filter((x) => x).join(' · '),
  },
  {
    id: 'extensions',
    label: shown('extensions') ? 'Extensions' : 'Sparks',
    icon: 'extensions',
    cards: ['extensions', 'sparks'],
    // A line under every section's name, as the others have: how many sparks are on.
    summary: () => lists.sparks.summary() || 'None yet',
  },
  { id: 'keyboard', label: 'Keyboard', icon: 'keyboard', cards: [['keys'], 'keyboard'], summary: () => '' },
  { id: 'about', label: 'About Project B', icon: 'about', cards: [['version', 'autoUpdate', 'channel'], 'releaseNotes', ['runSetup']], summary: aboutSummary },
  { id: 'flags', label: 'Flags', icon: 'flags', below: true, cards: ['flags'], summary: () => '' },
  { id: 'webFeatures', label: 'Web features', icon: 'webFeatures', below: true, cards: ['webFeatures'], summary: () => '' },
];

export const sections = computed(() => arranged(sectionList()));

// About's line in a phone's list of sections: the version, and a release ready to apply.
function aboutSummary() {
  const v = state.running || 'Development build';
  return state.update.phase === 'ready' ? `${v} · Update ready` : v;
}

// The page's own name for the frame colour, for a summary: a preset's name or Custom.
export function colourName(c) {
  return framePresets.find((p) => p.color === c)?.name || 'Custom';
}

// The parts a card holds as rows: none for a block.
export const rowsOf = (card) => (typeof card === 'string' ? [] : Array.isArray(card) ? card : card.rows);

// Every part a section holds, its rows' and its blocks' names. The page puts them on the
// section's place in the list, so a test or a link finds a setting by what it is, not by
// the section that holds it today: moving a part to another section is this file alone.
export const partsOf = (section) => section.cards.flatMap((c) => (typeof c === 'string' ? [c] : rowsOf(c)));

// What the page shows for an address, state.section: a list's own page when a section has
// the list as a row (sites/<site> or sites/<origin> is the sites list's page, at that
// site), else the section that is the address, or that holds the part it names. sections
// is the layout's.
export function resolve(address, sections) {
  const id = address.split('/')[0];
  const row = sections.find((s) => s.cards.some((c) => rowsOf(c).includes(id) || (lists[id]?.within && c === lists[id].within)));
  if (row && lists[id]) return { list: id, section: row };
  const section = sections.find((s) => s.id === id || s.cards.includes(id)) || row || null;
  return { list: '', section };
}
