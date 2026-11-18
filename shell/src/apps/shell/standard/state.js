// What only the standard shell keeps: whether the sidebar is open, where the tabs go, and
// which workspace the window shows. Everything else is Go's, in the SDK's state.

import { computed, reactive, watch } from 'vue';
import { isNewTab, saveData, send, state } from '@/sdk/vue.js';
export const view = reactive({
  // Whether the sidebar is open.
  sidebar: true,
  // The workspace whose tabs the window lists, by id, or null for all of them. Kept as
  // this window's shell data, so a reload or a crash keeps it.
  workspace: null,
});

// The workspaces, when there are two or more: with one there is nothing to choose.
export const workspaces = computed(() => ((state.workspaces || []).length > 1 ? state.workspaces : []));

// The choice comes back with the snapshot, and goes when its workspace does.
watch(
  () => state.data,
  (d) => {
    if (d && 'workspace' in d) view.workspace = d.workspace;
  },
  { immediate: true },
);
watch(workspaces, (list) => {
  if (view.workspace !== null && !list.some((w) => w.id === view.workspace)) chooseWorkspace(null);
});

// chooseWorkspace shows one workspace's tabs, or all with null. Showing one makes one of its
// tabs active when the active tab isn't in it, the first in the window's order, or opens a
// tab in it when it has none.
export function chooseWorkspace(id) {
  view.workspace = id;
  saveData({ ...(state.data || {}), workspace: id });
  if (id === null) return;
  const active = state.tabs.find((t) => t.id === state.active);
  if (active && active.workspace === id) return;
  const first = state.tabs.find((t) => t.workspace === id);
  send(first ? { t: 'activateTab', id: first.id } : { t: 'newTab', workspace: id });
}

// newTab opens a tab in the workspace shown, or as Go chooses when all are.
export function newTab() {
  send(view.workspace === null ? { t: 'newTab' } : { t: 'newTab', workspace: view.workspace });
}

// waiting says a tab's page waits on a dialog while another tab shows, for its row to say
// so: showing the tab brings the dialog back.
export const waiting = (tab) => tab.id !== state.active && state.dialogs.some((d) => d.tab === tab.id);

// tabsOnTop says the tabs are a strip above the toolbar rather than a sidebar.
export const tabsOnTop = computed(() => state.tabLayout === 'horizontal');

// Go's suggestion answers, sent on to whichever field asked: the address bar or the new
// tab's box. Each asks with an id from nextQuery, so ids never meet, and keeps only the
// answer to its own last one.
let query = 0;
export const nextQuery = () => ++query;
export const newTabBox = { suggestions() {} };

// The address bar's answers to what Go sends it, set once it is mounted.
export const addressBar = {
  suggestions() {},
  focus() {},
};

// toNewTab is Standard's New Tab (docs/windows.md, the owner 2026-11-18): it goes to the
// window's new tab, one with nothing loaded, in the workspace shown, opening one when there
// is none. The command box shows on a new tab, so on the new tab already it only takes the
// keyboard back.
export function toNewTab() {
  const fresh = state.tabs.find((t) => isNewTab(t) && (view.workspace === null || t.workspace === view.workspace));
  if (!fresh) newTab();
  else if (fresh.id !== state.active) send({ t: 'activateTab', id: fresh.id });
  else newTabBox.focus?.();
}
