// The Bookmarks page's copy of what Go keeps (docs/bookmark-folders.md): the profile's
// bookmarks and folders. Only Go changes it; the page starts from a snapshot.

import { reactive } from 'vue';
import { bridge } from '@/bridge.js';

const channel = bridge('bookmarks');
export const send = channel.send;
export const { mark, firstPaint, inputs } = channel.timeline;

export const state = reactive({
  ready: false,
  theme: 'system',
  tones: null,
  // The tree as a flat list in its places, each {id, parent, kind, title, url, added,
  // modified, place} (ui/bookmarks.js).
  bookmarks: [],
  // Go's refusal of the last edit, {id, error}, or null.
  refused: null,
});

export function connect() {
  channel.listen(
    (m) => {
      switch (m.t) {
        case 'snapshot':
          Object.assign(state, { ready: true, theme: m.theme, tones: m.tones, bookmarks: m.bookmarks || [] });
          break;
        case 'bookmarks':
          state.bookmarks = m.bookmarks || [];
          state.refused = null;
          break;
        case 'error':
          state.refused = { id: m.id, error: m.error };
          break;
        case 'theme':
          state.theme = m.theme;
          break;
        case 'tones':
          state.tones = m.tones;
          break;
      }
    },
    () => send({ t: 'ready' }),
  );
}
