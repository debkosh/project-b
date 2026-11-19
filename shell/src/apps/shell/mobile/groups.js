// What the shell needs about Go's tab groups (docs/groups.md).

export function groupOf(groups, tab) {
  return groups.find((g) => g.tabs.includes(tab)) || null;
}

// What a group is called: its name, else how many tabs it has.
export function groupLabel(g) {
  return g.name || `${g.tabs.length} tabs`;
}
