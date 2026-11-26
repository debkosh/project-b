// What the shell needs about Go's tab groups (docs/groups.md).

export function groupOf(groups, tab) {
  return groups.find((g) => g.tabs.includes(tab)) || null;
}

// How many tabs a group has, in words: "1 tab", "3 tabs". A group keeps its last tab.
export function tabCount(g) {
  return g.tabs.length === 1 ? '1 tab' : `${g.tabs.length} tabs`;
}

// What a group is called: its name, else how many tabs it has.
export function groupLabel(g) {
  return g.name || tabCount(g);
}
