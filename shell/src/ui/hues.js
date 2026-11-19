// The colours of what the user makes and names, a workspace or a tab group (core/tones/hues.go,
// docs/design-system.md "Made up, or picked"). One is kept as one of gulal's seven distinct
// hues, "181", which Go draws for each theme and sends with the tones, painted here as
// --hue-181; or as a colour the user picked with +, "#rrggbb", drawn as it is.

// The seven, most distinct first, as Go sends them, { hue, name, light, dark }: paintTheme
// (theme.js) hands them here.
let hues = [];

export function setHues(list) {
  hues = list || [];
}

// isHue says whether a kept colour is one of the hues rather than a colour picked.
export const isHue = (c) => /^\d{1,3}$/.test(c ?? '');

// paint is a kept colour as CSS.
export const paint = (c) => (isHue(c) ? `var(--hue-${c})` : c);

// swatches are the seven as a swatch row takes them: { color, name, paint }.
export function swatches() {
  return hues.map((h) => ({ color: String(h.hue), name: h.name, paint: paint(String(h.hue)) }));
}

// next is the colour a new one starts on, among those in use, as Go's tones.Next picks it:
// the swatch used least, and of those the one whose hue is farthest from theirs.
export function next(existing) {
  const used = existing.map((c) => (isHue(c) ? Number(c) : hueOfHex(c))).filter((h) => h !== null);
  const uses = (h) => existing.filter((c) => c === String(h)).length;
  let best = null;
  for (const { hue: h } of hues) {
    const gap = Math.min(360, ...used.map((x) => Math.min(Math.abs(h - x), 360 - Math.abs(h - x))));
    if (!best || uses(h) < best.uses || (uses(h) === best.uses && gap > best.gap)) best = { h, uses: uses(h), gap };
  }
  return best ? String(best.h) : '';
}

// The OKLCH of a '#rrggbb' colour, and the colour of an OKLCH, for the Custom colour sheet,
// which steps a hue's lightness. A colour outside sRGB loses chroma until it fits.
export function oklchOf(hex) {
  const lin = [1, 3, 5].map((i) => {
    const v = parseInt(hex.slice(i, i + 2), 16) / 255;
    return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
  });
  const [r, g, b] = lin;
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  const L = 0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s;
  const A = 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s;
  const B = 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s;
  return { l: L, c: Math.hypot(A, B), h: ((Math.atan2(B, A) * 180) / Math.PI + 360) % 360 };
}

function hueOfHex(hex) {
  if (!/^#[0-9a-f]{6}$/i.test(hex ?? '')) return null;
  const { c, h } = oklchOf(hex);
  return c < 0.02 ? null : h;
}

export function hexOf(L, C, H) {
  const rad = (H * Math.PI) / 180;
  for (let c = C; ; c = Math.max(0, c - 0.004)) {
    const a = c * Math.cos(rad);
    const b = c * Math.sin(rad);
    const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
    const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
    const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
    const rgb = [
      4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
      -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
      -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
    ];
    if (c === 0 || rgb.every((v) => v >= -1e-4 && v <= 1 + 1e-4)) {
      return (
        '#' +
        rgb
          .map((v) => {
            v = Math.min(1, Math.max(0, v));
            v = v <= 0.0031308 ? 12.92 * v : 1.055 * v ** (1 / 2.4) - 0.055;
            return Math.round(v * 255).toString(16).padStart(2, '0');
          })
          .join('')
      );
    }
  }
}
