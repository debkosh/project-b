// Escape closes the innermost thing open: a menu, then a dialog, then whatever opened
// them. Each layer pushes its close here while it is open, and one listener, ahead of
// every other, runs the last one pushed and keeps Escape from reaching anything else.
// A page that closes itself on Escape (settings) does so only when no layer took it.
//
// Our pages keep their tab's history too (pageHistory, docs/shells.md "Back on our
// pages"): each place a page goes to inside itself, as a settings list or a bookmarks
// folder, and each layer open over it, is an entry in its tab's history. So the phone's
// back key and Back close the layer, or go back to where the person was, as Escape does,
// before they leave the page. A shell never calls pageHistory: its own back is the shell's.

const layers = [];
// Set by pageHistory: this document is one of our pages, whose history is its own.
let paged = false;
// Backs asked for by layers that closed themselves, not yet arrived. Until they have,
// nothing is added to the history: added before, a back would undo it. Then the history is
// made to match what is open: an entry for each layer opened meanwhile, and one for the
// place the page went to.
let backs = 0;
let waiting = null;

function onKey(e) {
  if (e.key !== 'Escape' || !layers.length) return;
  e.preventDefault();
  e.stopImmediatePropagation();
  layers[layers.length - 1]();
}

if (typeof window !== 'undefined') window.addEventListener('keydown', onKey, true);

// An entry for each open layer the history lacks, over the place the page is at.
function catchUp() {
  if (waiting !== null && waiting !== history.state?.address) {
    history.pushState({ address: waiting, before: history.state?.address, layer: history.state?.layer || 0 }, '');
  }
  waiting = null;
  for (let n = (history.state?.layer || 0) + 1; n <= layers.length; n++) history.pushState({ ...history.state, layer: n }, '');
}

// pushEscape adds a layer and returns the function that takes it off again. On one of our
// pages the layer is an entry in the tab's history while it is open; closed by itself, by
// a button or Escape, its entry goes too.
export function pushEscape(close) {
  layers.push(close);
  if (paged && !backs) catchUp();
  return () => {
    const i = layers.lastIndexOf(close);
    if (i < 0) return;
    layers.splice(i, 1);
    if (paged && (history.state?.layer || 0) > layers.length + backs) {
      backs++;
      history.back();
    }
  };
}

// upTo goes from a place up to the one that holds it, as a list page's "Back to Privacy"
// arrow does, the way Android's Up does: back, when the entry before is that place, or in
// this entry's stead, so it never adds one and back never returns to where it left.
export function upTo(address, show) {
  if (paged && !backs && (history.state?.layer || 0) === 0 && history.state?.before === address) {
    history.back();
    return;
  }
  if (paged) history.replaceState({ ...history.state, address }, '');
  show(address);
}

// pageHistory makes this page's places entries in its tab's history. current is where
// the page is now, a string, and show takes it to one; the returned function is called with
// each place the page goes to, and adds an entry for a new one. Back to an entry closes the
// layers opened after it, then shows its place.
export function pageHistory(current, show) {
  paged = true;
  history.replaceState({ address: current(), layer: 0 }, '');
  window.addEventListener('popstate', (e) => {
    if (backs) {
      // A layer's own back has arrived; once the last has, the history catches up.
      backs--;
      if (!backs) catchUp();
      return;
    }
    const to = e.state || { address: '', layer: 0 };
    // Each layer above the entry closes, the innermost first; each takes itself off.
    for (const close of layers.slice(to.layer || 0).reverse()) close();
    if (to.address !== current()) show(to.address);
  });
  return (address) => {
    waiting = address;
    if (!backs) catchUp();
  };
}

// closeInnermost closes the innermost layer, as Escape would, for a phone's back key that
// a shell hears from Go. It says whether there was one.
export function closeInnermost() {
  if (!layers.length) return false;
  layers[layers.length - 1]();
  return true;
}
