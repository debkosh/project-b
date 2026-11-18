// What Standard's bookmark places share (docs/bookmark-folders.md, "Where bookmarks show"):
// a bookmark's mark, and the ways one or a folder of them opens. Each place shows only while
// its pref is on, read from state.prefs where it is drawn.

import { reactive, watch } from 'vue';
import { send, state } from '@/sdk/vue.js';
import { closeBox } from './state.js';

export function host(url) {
  try {
    return new URL(url).host.replace(/^www\./, '');
  } catch {
    return url;
  }
}

// mark is what stands for a bookmark: the favicon of a tab that has its address open, else
// its site's first letter on a colour from its host, the same each time, as the Mobile
// shell's tiles have.
export function mark(url) {
  const h = host(url);
  let hue = 0;
  for (const c of h) hue = (hue * 31 + c.charCodeAt(0)) % 360;
  return { favicon: tabFor(url)?.favicon || '', letter: (h[0] || '?').toUpperCase(), hue };
}

// tabFor is this window's tab that has url open, if one does.
export function tabFor(url) {
  return state.tabs.find((t) => t.url === url);
}

// openHere loads url as the command box takes a row: in a new tab over a page or with no
// tabs, else in the tab with nothing in it that the box is over.
export function openHere(url, overPage) {
  send(overPage || state.active === 0 ? { t: 'newTab', text: url } : { t: 'navigate', text: url });
  closeBox();
}

// openTab shows url's tab when one has it open, else opens it in a new tab.
export function openTab(url) {
  const tab = tabFor(url);
  send(tab ? { t: 'activateTab', id: tab.id } : { t: 'newTab', text: url });
}

// What the sidebar's bookmarks show open, for as long as the window's shell runs: the
// section, and the folders unfolded, every one folded at first.
export const sidebarTree = reactive({ open: true, folders: new Set() });

// openGroup opens urls as a new tab group named name, the first tab in front. Go has no one
// message for it: the first opens on its own, and each of the rest joins it once Go has
// made it, which starts the group (docs/groups.md); the group takes the name once it is.
export function openGroup(urls, name) {
  if (!urls.length) return;
  const had = new Set(state.tabs.map((t) => t.id));
  send({ t: 'newTab', text: urls[0] });
  if (urls.length === 1) return;
  const stop = watch(
    () => state.tabs,
    (tabs) => {
      const first = tabs.find((t) => !had.has(t.id));
      if (!first) return;
      stop();
      for (const url of urls.slice(1)) send({ t: 'newTab', groupWith: first.id, text: url, background: true });
      const named = watch(
        () => state.groups,
        (groups) => {
          const g = (groups || []).find((x) => x.tabs.includes(first.id));
          if (!g) return;
          named();
          send({ t: 'editGroup', group: g.id, name });
        },
        { immediate: true },
      );
    },
  );
}
