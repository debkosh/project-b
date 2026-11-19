// Light or dark for one of our pages: Go's theme pref says light, dark, or as the system
// is, and the system's comes from the page's own prefers-color-scheme. Go makes the tones,
// both palettes of the frame colour (core/tones), and sends them with it; the chosen one
// goes on the root as CSS variables, and color-scheme makes the browser's own controls and
// scrollbars match. The shell's SDK paints them as Go's state changes; the settings page
// and the picker with a watchEffect on theirs.

import { setHues } from '@/ui/hues.js';
import { retint } from '@/ui/tint.js';

const media = window.matchMedia('(prefers-color-scheme: dark)');
// What was last painted, so the page follows the system when it changes.
let painted = { tones: null, theme: 'system' };

// paintTheme puts tones' light or dark palette on the root, as theme says: 'light',
// 'dark', or 'system', as the system is. Nothing is painted before Go sends the tones.
export function paintTheme(tones, theme) {
  painted = { tones, theme };
  if (!tones) return;
  const dark = theme === 'dark' || (theme !== 'light' && media.matches);
  const root = document.documentElement;
  for (const [name, value] of Object.entries(dark ? tones.dark : tones.light)) {
    root.style.setProperty(name, value);
  }
  // The hues a workspace or group keeps, as Go drew them for this theme (hues.js).
  for (const h of tones.hues || []) root.style.setProperty(`--hue-${h.hue}`, dark ? h.dark : h.light);
  setHues(tones.hues);
  root.style.colorScheme = dark ? 'dark' : 'light';
  // The colours made up for items follow. None's frame is a grey of each scheme's own, so
  // its two palettes' frames differ, where a picked frame is the same in both.
  retint(dark ? tones.dark : tones.light, dark, tones.light['--frame'] !== tones.dark['--frame']);
}

media.addEventListener('change', () => paintTheme(painted.tones, painted.theme));
