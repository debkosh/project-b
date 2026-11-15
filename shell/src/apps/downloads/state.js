// The Downloads page's copy of what Go keeps (docs/daily-driver.md): the profile's
// downloads, newest first, each {id, name, url, path, state, received, total, paused}, as
// the shells get them in state.downloads. Only Go changes it; the page starts from a snapshot.

import { reactive } from 'vue';
import { bridge } from '@/bridge.js';

const channel = bridge('downloads');
export const send = channel.send;
export const { mark, firstPaint, inputs } = channel.timeline;

export const state = reactive({
  ready: false,
  theme: 'system',
  tones: null,
  downloads: [],
});

// act does one thing to download id: 'open' or 'reveal' its file once done, 'cancel',
// 'pause' or 'resume' it while it runs, or 'remove' it from the list, leaving the file.
export const act = (id, action) => send({ t: 'downloadAction', id, action });

// clear takes every finished download off the list, leaving the files.
export const clear = () => send({ t: 'clearDownloads' });

export function connect() {
  channel.listen(
    (m) => {
      switch (m.t) {
        case 'snapshot':
          Object.assign(state, { ready: true, theme: m.theme, tones: m.tones, downloads: m.downloads || [] });
          break;
        case 'downloads':
          state.downloads = m.downloads || [];
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
