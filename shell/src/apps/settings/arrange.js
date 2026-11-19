// What the settings page holds, and how each layout arranges it (docs/settings.md). Every
// setting is a part: a row, a few rows, or a block with cards of its own. The layouts
// differ only in how parts are grouped and where a long list opens; the parts are the same
// components in both.

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
  // none to choose; a phone chooses among its own.
  shell: () => (phone ? state.phoneShells?.shells.length > 1 : !!state.profile),
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
};

export const shown = (part) => !where[part] || where[part]();

// The long lists. Each is a block of its own; a layout puts it in a section as it is, or
// as a row that opens it as a page of its own, with a back link. The row's words, and what
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

// Layout B: one page by task. Each group is a run of cards: an array is one card of rows
// (a list among them is its row, which opens it), a name is a block, and {heading, rows}
// a card under a heading of its own.
const page = [
  { id: 'look', label: 'Look', hint: 'How Project B looks', cards: [['theme', 'frameColour', 'tabLayout', 'addressBar', 'shell', 'settingsLayout']] },
  {
    id: 'browsing',
    label: 'Browsing',
    hint: 'Starting, searching, keys, workspaces and bookmarks',
    cards: [['start', 'searchEngine', 'keys', 'keyboard', 'workspaces'], 'bookmarks', { heading: 'Downloads', rows: ['downloadFolder', 'askWhereToSave'] }],
  },
  { id: 'privacy', label: 'Privacy', hint: 'What sites can do and what they keep', cards: [['clearData', 'sites', 'passkeys', 'clearOnQuit', 'blockAds']] },
  { id: 'data', label: 'Your data', hint: 'This profile and what you bring in', cards: [['profile'], 'sync', 'import', ['runSetup']] },
  {
    id: 'advanced',
    label: 'Advanced',
    hint: 'Updates, your scripts, and what is still being tried',
    cards: [['autoUpdate', 'sparks', 'extensions', 'webFeatures'], 'flags'],
  },
];

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

export const pageGroups = computed(() => arranged(page));

// Layout A: sections in a list, like Chrome's, one showing at a time; each has an icon and
// a line saying what it is set to, which a phone's list of sections shows. Below the line at
// the foot of the list, what is still being tried. A list is in its section as it is, but
// Sites, which is in Privacy and sites with rows of its own, opens as its own page.
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
  {
    id: 'appearance',
    label: 'Appearance',
    icon: 'appearance',
    cards: [['theme', 'frameColour', 'tabLayout', 'addressBar', 'shell', 'settingsLayout']],
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
    id: 'privacy',
    label: 'Privacy and sites',
    icon: 'privacy',
    cards: [['clearData', 'blockAds', 'clearOnQuit', 'sites', 'passkeys']],
    summary: () => [state.prefs.blockAds ? 'Ads blocked' : 'Ads shown', lists.sites.summary()].filter((x) => x).join(' · '),
  },
  {
    id: 'downloads',
    label: 'Downloads',
    icon: 'download',
    cards: [['downloadFolder', 'askWhereToSave']],
    summary: () => {
      if (state.prefs.askWhereToSave) return 'Ask where to save each file';
      const f = state.prefs.downloadFolder;
      return f ? `To ${f.split('/').filter((x) => x).pop() || f}` : 'To the Downloads folder';
    },
  },
  { id: 'keyboard', label: 'Keyboard', icon: 'keyboard', cards: [['keys'], 'keyboard'], summary: () => '' },
  { id: 'profile', label: 'Profile and import', icon: 'profile', cards: [['profile'], 'sync', 'import', ['runSetup']], summary: () => state.profile?.name || '' },
  {
    id: 'extensions',
    label: shown('extensions') ? 'Extensions and sparks' : 'Sparks',
    icon: 'extensions',
    cards: ['extensions', 'sparks'],
    summary: lists.sparks.summary,
  },
  { id: 'about', label: 'About Project B', icon: 'about', cards: [['autoUpdate']], summary: () => state.running || 'Development build' },
  { id: 'flags', label: 'Flags', icon: 'flags', below: true, cards: ['flags'], summary: () => '' },
  { id: 'webFeatures', label: 'Web features', icon: 'webFeatures', below: true, cards: ['webFeatures'], summary: () => '' },
];

export const sections = computed(() => arranged(sectionList()));

// The page's own name for the frame colour, for a summary: a preset's name or Custom.
export function colourName(c) {
  return framePresets.find((p) => p.color === c)?.name || 'Custom';
}

// The parts a card holds as rows: none for a block.
export const rowsOf = (card) => (typeof card === 'string' ? [] : Array.isArray(card) ? card : card.rows);

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
