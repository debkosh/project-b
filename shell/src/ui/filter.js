// Searching a list: what the shell's and our pages' search fields keep as a person types.
// Every word typed must be in the item, in any order, each with one typo forgiven, so "sbi
// shankuan" finds the sbi.co.in login whose username is shankuand. uFuzzy does the matching
// (github.com/leeoniya/uFuzzy); the list keeps its own order.

import uFuzzy from '@leeoniya/ufuzzy';

const fuzzy = new uFuzzy({ intraMode: 1 });

// find gives the items whose texts, joined, match what was typed; all of them when nothing
// is. texts(item) is the item's searchable words: a site and a username, a title and an
// address. Words uFuzzy can't take apart, such as a script it doesn't split, fall back to
// the text as it is typed.
export function find(items, texts, query) {
  const typed = (query || '').trim();
  if (!typed) return items;
  const hay = items.map((item) => texts(item).filter((t) => t).join(' '));
  const [found] = fuzzy.search(hay, typed, 1, 0);
  if (!found) {
    const lower = typed.toLowerCase();
    return items.filter((_, i) => hay[i].toLowerCase().includes(lower));
  }
  const kept = new Set(found);
  return items.filter((_, i) => kept.has(i));
}
