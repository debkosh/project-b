// What a shell sends about suggestions: one ranked list from Go for any field that takes an
// address or a search, each row {kind, title, detail, url, tab, action}, kind being address,
// search, tab, history or action (docs/daily-driver.md). The answer comes as the
// suggestions message, {id, items}; how the list looks is the shell's own.

import { send } from './channel.js';

// Every kind of row, for a shell that draws them all.
export const suggestionKinds = ['address', 'search', 'tab', 'history', 'action'];

// suggest asks for the list for text, with rows of the kinds named; id comes back with the
// answer, so a late one for an earlier text is told apart. With nothing typed the list is
// the pages visited most.
export function suggest(id, text, kinds = suggestionKinds) {
  send({ t: 'suggest', id, text, kinds });
}

// chooseSuggestion runs a row: Go loads its address or search in the active tab, or in a
// new one, shows its tab, in whichever window it is, or runs its action.
export function chooseSuggestion(row, newTab = false) {
  send({ t: 'chooseSuggestion', row, newTab });
}
