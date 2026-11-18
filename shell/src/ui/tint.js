// The colours our pages make up for an item, a site's letter tile or a bookmark folder's,
// from gulal (docs/colour-util.md): the same colour for the same text every time, made to
// sit with the theme the user picked. gulal takes the scheme, the background the colour
// sits on and the accent it lives beside; ours are light or dark as painted, the tones'
// --bar or --surface, and the frame colour, left out for None so None's colours are quiet.
//
// paintTheme (theme.js) hands each painted palette here; the environments are made again
// then, and tints tells every mark to draw again. It has no framework in it: tints is a
// store as Svelte's are, and useTints (useTints.js) makes it a Vue ref.

import { env } from 'gulal';

// Each painted palette's environments, one per background a colour sits on, by name.
let palette = null;
let dark = false;
let none = false;
const envs = new Map();

let version = 0;
const listeners = new Set();

// retint takes the palette just painted, light or dark, and whether the frame is None.
export function retint(p, isDark, isNone) {
  palette = p;
  dark = isDark;
  none = isNone;
  envs.clear();
  version++;
  for (const fn of listeners) fn(version);
}

// tints is a store: subscribe(fn) calls fn now and on each new palette, and returns the
// call that stops it.
export const tints = {
  subscribe(fn) {
    listeners.add(fn);
    fn(version);
    return () => listeners.delete(fn);
  },
};

// The environment for colours on one of the tones' backgrounds: 'bar' or 'surface'.
function on(where) {
  let g = envs.get(where);
  if (!g) {
    const background = palette?.['--' + where];
    const accent = none ? undefined : palette?.['--frame'];
    g = env({ scheme: dark ? 'dark' : 'light', background: hex(background), accent: hex(accent) });
    envs.set(where, g);
  }
  return g;
}

// hex is a "#rrggbb" colour, or undefined for anything else, as gulal takes only those.
function hex(c) {
  return typeof c === 'string' && /^#[0-9a-f]{6}$/i.test(c) ? c : undefined;
}

// siteHost is the text a site's colour comes from: its host in lower case, without www.
export function siteHost(url) {
  try {
    return new URL(url).host.toLowerCase().replace(/^www\./, '') || url;
  } catch {
    return url || '';
  }
}

// siteTile is a site's letter tile on where: {background, border, letter}, the letter in the
// tile's own hue.
export function siteTile(url, where = 'surface') {
  const s = on(where).fromText(siteHost(url)).soft;
  return { background: s.background, border: s.border, letter: s.tonal };
}

// A folder's colour comes from its id, so a rename keeps it.
const folderText = (id) => 'folder:' + id;

// folderMark is a folder's colour as a dot or an icon on where.
export function folderMark(id, where = 'surface') {
  return on(where).fromText(folderText(id)).mark;
}

// folderTile is a folder's tile on where: {background, border}.
export function folderTile(id, where = 'surface') {
  const s = on(where).fromText(folderText(id)).soft;
  return { background: s.background, border: s.border };
}
