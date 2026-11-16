// What a shell sends about suggestions: one ranked list from Go for any field that takes an
// address or a search, each row {kind, title, detail, url, tab, action, inline}, kind being
// address, search, tab, history, action, bookmark or closed (docs/daily-driver.md). The
// answer comes as the suggestions message, {id, items}; how the list looks is the shell's
// own.

import { send } from './channel.js';

// Every kind of row, for a shell that draws them all. A closed row, a tab closed lately, comes
// only with nothing typed.
export const suggestionKinds = ['address', 'search', 'tab', 'history', 'action', 'bookmark', 'closed'];

// suggest asks for the list for text, with rows of the kinds named; id comes back with the
// answer, so a late one for an earlier text is told apart. With nothing typed the list is
// what a new tab offers, eight rows at most: with tab and closed asked for, the window's
// other tabs, the most recently used first, and the tabs closed lately, three of each;
// then the sites visited most, as history rows, and the newest bookmarks.
export function suggest(id, text, kinds = suggestionKinds) {
  send({ t: 'suggest', id, text, kinds });
}

// chooseSuggestion runs a row: Go loads its address or search in the active tab, or in a
// new one, shows its tab, in whichever window it is, reopens a closed tab, or runs its
// action.
export function chooseSuggestion(row, newTab = false) {
  send({ t: 'chooseSuggestion', row, newTab });
}
