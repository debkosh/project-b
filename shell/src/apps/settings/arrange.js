// What the settings page holds, and how each layout arranges it (docs/settings.md). Every
// setting is a part: a row, a few rows, or a block with cards of its own. The layouts
// differ only in how parts are grouped and where a long list opens; the parts are the same
// components in both.

import { computed } from 'vue';
import { extensionsSection } from '@extensions';
import { phone } from '@/phone.js';
import { framePresets } from '@/ui/colours.js';
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
    hint: 'Camera, microphone, notifications, each site\'s ad blocker and links',
    summary: () => (state.siteSettings.length ? words(state.siteSettings.length, 'site', 'sites') : ''),
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
  webFeatures: {
    label: 'Web features',
    hint: 'How web features behave in Project B, and why',
    summary: () => '',
  },
};

// Layout B: one page by task. Each group is a run of cards: an array is one card of rows
// (a list among them is its row, which opens it), a name is a block, and {heading, rows}
// a card under a heading of its own.
const page = [
  { id: 'look', label: 'Look', hint: 'How Project B looks', cards: [['theme', 'frameColour', 'tabLayout', 'addressBar', 'shell']] },
  {
    id: 'browsing',
    label: 'Browsing',
    hint: 'Starting, searching, keys, workspaces and bookmarks',
    cards: [['start', 'searchEngine', 'keys', 'keyboard', 'workspaces'], 'bookmarks'],
  },
  { id: 'privacy', label: 'Privacy', hint: 'What sites can do and what they keep', cards: [['sites']] },
  { id: 'data', label: 'Your data', hint: 'This profile and what you bring in', cards: [['profile'], 'import', ['runSetup']] },
  {
    id: 'advanced',
    label: 'Advanced',
    hint: 'Updates, your scripts, and what is still being tried',
    cards: [['version', 'autoUpdate', 'sparks', 'extensions', 'webFeatures'], 'flags'],
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

// The page's own name for the frame colour, for a summary: a preset's name or Custom.
export const colourName = (c) => framePresets.find((p) => p.color === c)?.name || 'Custom';

// The parts a card holds as rows: none for a block.
export const rowsOf = (card) => (typeof card === 'string' ? [] : Array.isArray(card) ? card : card.rows);

// What the page shows for an address, state.section: a list's own page when a section has
// the list as a row (sites/<origin> is the sites list's page, at that site), else the
// section that is the address, or that holds the part it names. sections is the layout's.
export function resolve(address, sections) {
  const id = address.split('/')[0];
  const row = sections.find((s) => s.cards.some((c) => rowsOf(c).includes(id)));
  if (row && lists[id]) return { list: id, section: row };
  const section = sections.find((s) => s.id === id || s.cards.includes(id)) || row || null;
  return { list: '', section };
}
