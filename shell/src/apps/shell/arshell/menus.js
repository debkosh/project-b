// The menus Curve's rows open, each a list of items: {label, run, action} where action is
// the id whose key the item shows, or {separator: true}. Curve's Menu draws them.

import { activeTab, openPin, pinTab, send, state, unpin } from '@/sdk/svelte.svelte.js';
import { next } from '@/ui/hues.js';
import * as g from './arshell.svelte.js';

const sep = { separator: true };

export function tabMenu(tab) {
  const items = [];
  const pinned = g.isPinned(tab);
  // Go pins a web tab with an address; a pinned tab leaves its split once Go lists it so
  // (fitSplits).
  if (pinned) items.push({ label: 'Unpin Tab', run: () => unpin(tab.pin), action: 'tab.pin' });
  else if (tab.url && !tab.page) items.push({ label: 'Pin Tab', run: () => pinTab(tab.id), action: 'tab.pin' });
  if (tab.url) {
    items.push(
      g.isFavorite(tab)
        ? { label: 'Remove from Favorites', run: () => g.removeFavorite(g.favorites(g.spaceOf(tab)).find((f) => f.url === tab.url), g.spaceOf(tab)) }
        : { label: 'Add to Favorites', run: () => g.addFavorite(tab), action: 'arshell.favorite' },
    );
  }
  if (!pinned) {
    const split = g.splitOf(tab.id);
    const active = activeTab(state);
    if (split) {
      items.push({ label: 'Take Out of Split View', run: () => g.takeOut(tab.id) });
      items.push({ label: 'Separate Split View', run: () => g.separate(tab.id), action: 'arshell.splitRemove' });
    } else if (active.id && tab.id !== active.id && !g.isPinned(active)) {
      items.push({ label: 'Open in Split View', run: () => g.addToSplit(tab) });
    }
    items.push(sep);
    items.push({ label: 'New Tab in Folder', run: () => send({ t: 'newTab', groupWith: tab.id }) });
    const others = (state.groups || []).filter((gr) => gr.id !== tab.group && gr.tabs.some((id) => g.tabsIn(g.spaceOf(tab)).some((t) => t.id === id)));
    for (const gr of others) items.push({ label: `Move to ${gr.name || 'Folder'}`, run: () => send({ t: 'addToGroup', id: tab.id, group: gr.id }) });
    if (tab.group) items.push({ label: 'Remove from Folder', run: () => send({ t: 'removeFromGroup', id: tab.id }) });
    else items.push({ label: 'Add to New Folder', run: () => send({ t: 'createGroup', ids: [tab.id] }) });
  }
  items.push(sep);
  if (tab.url) items.push({ label: 'Copy Address', run: () => copy(tab.url), action: 'arshell.copyAddress' });
  items.push({ label: 'Close Tab', run: () => g.closeTab(tab), action: 'tab.close' });
  return items;
}

export function folderMenu(group) {
  return [
    { label: 'New Tab in Folder', run: () => send({ t: 'newTab', groupWith: group.tabs[0] }) },
    {
      label: 'Rename Folder…',
      run: () =>
        (g.ui.dialog = {
          title: 'Folder',
          name: group.name,
          color: group.color,
          save: (name, color) => send({ t: 'editGroup', group: group.id, name, color }),
        }),
    },
    sep,
    { label: 'Ungroup', run: () => send({ t: 'ungroup', group: group.id }) },
    { label: 'Close Folder', run: () => send({ t: 'closeGroup', group: group.id }) },
  ];
}

// A Space's menu. Deleting one, which erases its storage, is in Settings, Workspaces.
export function spaceMenu(space) {
  return [
    {
      label: 'Edit Space…',
      run: () =>
        (g.ui.dialog = {
          title: 'Space',
          name: space.name,
          color: space.color,
          save: (name, color) => send({ t: 'editWorkspace', workspace: space.id, text: name, color }),
        }),
    },
    { label: 'New Space…', run: newSpace, action: 'arshell.newSpace' },
    sep,
    { label: 'Settings', run: () => send({ t: 'run', text: 'app.settings' }), action: 'app.settings' },
  ];
}

export function newSpace() {
  g.ui.dialog = {
    title: 'New Space',
    name: '',
    color: next(g.spaces().map((s) => s.color)),
    create: true,
    // Go lists the new workspace last; the shell shows it once it hears of it.
    save: (name, color) => {
      g.ui.created = state.workspaces.length;
      send({ t: 'createWorkspace', text: name, color });
    },
  };
}

export function favoriteMenu(fav) {
  return [
    { label: 'Open', run: () => g.openFavorite(fav) },
    { label: 'Copy Address', run: () => copy(fav.url) },
    sep,
    { label: 'Remove from Favorites', run: () => g.removeFavorite(fav) },
  ];
}

export function pinMenu(entry) {
  return [
    { label: 'Open', run: () => openPin(entry.id) },
    { label: 'Copy Address', run: () => copy(entry.url) },
    sep,
    { label: 'Unpin', run: () => unpin(entry.id) },
  ];
}

// Go puts it on the clipboard, as it does a context menu's Copy Link: the shell's own write
// is refused without a click, and Copy Address from the menu bar or its keys has none.
export function copy(url) {
  send({ t: 'copyText', text: url });
  g.toast('Copied');
}
