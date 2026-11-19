// The Custom colour sheet, which a colour's + opens on a phone in place of the system's
// picker (docs/design-system.md, "On a phone"): a head with Done; a preview of the item as
// it will look, its dot and its chip; Hue, a slider; Shade, five swatches of that hue from
// light to deep at the theme's strength, the middle one gulal's own; and Hex, for an exact
// colour. Each change goes to onchange at once, as '#rrggbb'; Done, the back key, Escape or
// a tap outside close it, keeping them. It has no framework in it, so the settings page and
// a Svelte shell open the same one.

import { pushEscape } from './escape.js';
import { hexOf, isHue, oklchOf } from './hues.js';
import { hueColour, inkOn } from './tint.js';

// The shades' lightness, from the middle one's.
const steps = [0.2, 0.1, 0, -0.1, -0.2];
const shadeNames = ['Lightest', 'Light', 'Middle', 'Deep', 'Deepest'];

// el makes an element with its attributes and children: the shells' pages take no HTML
// as text (Trusted Types).
function el(tag, attrs = {}, ...children) {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
  e.append(...children);
  return e;
}

function build(sheet) {
  sheet.append(
    el('div', { class: 'ui-colour-head' }, el('h2', {}, 'Custom colour'), el('button', { type: 'button', class: 'text-button quiet', 'data-done': '' }, 'Done')),
    el('div', { class: 'ui-colour-preview' }, el('span', { class: 'ui-colour-dot' }), el('span', { class: 'ui-colour-name' }), el('span', { class: 'ui-colour-chip' })),
    el('label', { class: 'ui-colour-label', for: 'ui-colour-hue' }, 'Hue'),
    el('input', { id: 'ui-colour-hue', class: 'ui-colour-hue', type: 'range', min: '0', max: '359', step: '1' }),
    el('div', { class: 'ui-colour-label', id: 'ui-colour-shade' }, 'Shade'),
    el('div', { class: 'ui-colour-shades', role: 'radiogroup', 'aria-labelledby': 'ui-colour-shade' },
      ...shadeNames.map((n, i) => el('button', { type: 'button', role: 'radio', 'aria-label': n, 'data-shade': String(i) }))),
    el('div', { class: 'ui-colour-hex' }, el('label', { for: 'ui-colour-hex' }, 'Hex'),
      el('input', { id: 'ui-colour-hex', class: 'text-field', type: 'text', maxlength: '7', spellcheck: 'false', autocomplete: 'off', autocapitalize: 'off' })),
  );
}

// openCustomColour opens the sheet on color, a kept colour, for an item called name, and
// returns the call that closes it.
export function openCustomColour({ color, name, onchange, onclose }) {
  const sheet = document.createElement('dialog');
  sheet.className = 'ui-colour-sheet';
  sheet.setAttribute('aria-label', 'Custom colour');
  build(sheet);
  const $ = (s) => sheet.querySelector(s);
  const hueInput = $('.ui-colour-hue');
  const hexInput = $('#ui-colour-hex');
  const shadeButtons = [...sheet.querySelectorAll('[data-shade]')];
  $('.ui-colour-name').textContent = name || '';
  $('.ui-colour-chip').textContent = name || 'Aa';

  // What is shown: the hue, its five shades, the one chosen (-1 for none, as for a hex
  // typed off them) and the colour itself.
  let hue = 0;
  let shades = [];
  let chosen = -1;
  let current = '';

  function shadesOf(h) {
    const mid = hueColour(h).oklch;
    return steps.map((d) => hexOf(Math.min(0.95, Math.max(0.15, mid.l + d)), mid.c, h));
  }

  function draw() {
    const mid = hueColour(hue).oklch;
    const stops = Array.from({ length: 13 }, (_, k) => `oklch(${mid.l} ${mid.c} ${k * 30})`).join(',');
    sheet.style.setProperty('--ui-colour-track', `linear-gradient(90deg, ${stops})`);
    sheet.style.setProperty('--ui-colour-thumb', shades[2]);
    sheet.style.setProperty('--ui-colour', current);
    sheet.style.setProperty('--ui-colour-ink', inkOn(current));
    hueInput.value = String(Math.round(hue));
    hueInput.setAttribute('aria-valuetext', `${Math.round(hue)} degrees`);
    shadeButtons.forEach((b, i) => {
      b.style.setProperty('--ui-shade', shades[i]);
      b.setAttribute('aria-checked', String(i === chosen));
      b.tabIndex = i === (chosen < 0 ? 2 : chosen) ? 0 : -1;
    });
    if (document.activeElement !== hexInput) hexInput.value = current;
  }

  // The changes go to onchange once a frame at most, as the slider moves.
  let pending = 0;
  function emit() {
    if (pending) return;
    pending = requestAnimationFrame(() => {
      pending = 0;
      onchange(current);
    });
  }

  function setHue(h) {
    hue = h;
    shades = shadesOf(hue);
    if (chosen < 0) chosen = 2;
    current = shades[chosen];
    draw();
    emit();
  }

  function setShade(i) {
    chosen = i;
    current = shades[i];
    draw();
    emit();
  }

  // Where it starts: a kept hue at its middle shade, a picked colour as it is, its shade
  // chosen when it is one.
  if (isHue(color)) {
    hue = Number(color);
    shades = shadesOf(hue);
    chosen = 2;
  } else {
    current = /^#[0-9a-f]{6}$/i.test(color ?? '') ? color.toLowerCase() : hueColour(0).hex;
    hue = oklchOf(current).h;
    shades = shadesOf(hue);
    chosen = shades.indexOf(current);
  }
  current ||= shades[chosen];
  draw();

  hueInput.addEventListener('input', () => setHue(Number(hueInput.value)));
  shadeButtons.forEach((b, i) => b.addEventListener('click', () => setShade(i)));
  sheet.querySelector('.ui-colour-shades').addEventListener('keydown', (e) => {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
    if (!step) return;
    e.preventDefault();
    setShade(((chosen < 0 ? 2 : chosen) + step + 5) % 5);
    shadeButtons[chosen].focus();
  });
  hexInput.addEventListener('input', () => {
    const v = hexInput.value.trim().toLowerCase();
    const typed = /^#?[0-9a-f]{6}$/.test(v) ? (v[0] === '#' ? v : '#' + v) : '';
    if (!typed || typed === current) return;
    current = typed;
    hue = oklchOf(current).h;
    shades = shadesOf(hue);
    chosen = shades.indexOf(current);
    draw();
    emit();
  });
  hexInput.addEventListener('blur', () => (hexInput.value = current));
  hexInput.addEventListener('keydown', (e) => e.key === 'Enter' && hexInput.blur());

  let closed = false;
  let pop = null;
  function close() {
    if (closed) return;
    closed = true;
    pop();
    if (pending) {
      cancelAnimationFrame(pending);
      pending = 0;
      onchange(current);
    }
    sheet.close();
    sheet.remove();
    onclose?.();
  }
  $('[data-done]').addEventListener('click', close);
  // The back key and Escape: the dialog's own cancel, and the shells' Escape layers.
  sheet.addEventListener('cancel', (e) => {
    e.preventDefault();
    close();
  });
  // A tap outside the sheet lands on the dialog itself, beside its box.
  sheet.addEventListener('click', (e) => {
    if (e.target !== sheet) return;
    const r = sheet.getBoundingClientRect();
    if (e.clientY < r.top || e.clientY > r.bottom || e.clientX < r.left || e.clientX > r.right) close();
  });

  document.body.append(sheet);
  sheet.showModal();
  pop = pushEscape(close);
  $('[data-done]').focus();
  return close;
}
