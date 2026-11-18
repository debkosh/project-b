// What every shell's passkey card does alike (docs/passkeys.md): Go words each step, and
// these turn its code and its PIN rules into what a card draws and says.

// The modules either side of a QR code that stay light, so a camera finds its edge.
export const QUIET = 4;

// codePath is the SVG path of a code's dark modules, in a box of codeSize(rows) a side.
export function codePath(rows) {
  let d = '';
  rows.forEach((row, y) => {
    for (let x = 0; x < row.length; x++) {
      if (row[x] === '1') d += `M${x + QUIET} ${y + QUIET}h1v1h-1z`;
    }
  });
  return d;
}

export function codeSize(rows) {
  return rows.length + 2 * QUIET;
}

// checkPin is what is wrong with the PINs typed into a step's fields before the key sees
// them, as {field, words}, or null: too short, or a confirmation that differs.
export function checkPin(pin, values) {
  if ([...values[0]].length < pin.minLength) return { field: 0, words: pin.short };
  if (pin.labels.length > 1 && values[0] !== values[1]) return { field: 1, words: pin.mismatch };
  return null;
}

// firstControl is where a step's card puts the keyboard: its first field, else its
// primary button, else Cancel, the last button.
export function firstControl(card) {
  return card.querySelector('input') || card.querySelector('.text-button.primary') || [...card.querySelectorAll('.buttons button')].at(-1);
}

// trapTab keeps Tab and Shift+Tab among the card's own controls while the page waits.
export function trapTab(card, e) {
  if (e.key !== 'Tab') return;
  const all = [...card.querySelectorAll('input, button')];
  const i = all.indexOf(document.activeElement);
  e.preventDefault();
  all[(i + (e.shiftKey ? -1 : 1) + all.length) % all.length]?.focus();
}
