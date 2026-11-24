import { reactive } from 'vue';
import { send, state } from '@/sdk/vue.js';

// Dragging a tab in the standard shell: onto another tab's row it moves before or after
// it, by the half of the row it is over, with Go's moveTab; let go outside the window it
// moves to a window of its own. The group it lands among is Go's to settle
// (docs/groups.md).

// The tab being dragged, the tab it is over, and whether it would land after that one.
export const drag = reactive({ id: null, over: null, after: false });

function reset() {
  drag.id = drag.over = null;
  drag.after = false;
}

export function dragStart(e, id) {
  drag.id = id;
  e.dataTransfer.effectAllowed = 'move';
  e.dataTransfer.setData('text/plain', String(id));
}

// dragOver marks the row under the pointer; along is 'y' for the sidebar's list and 'x'
// for the strip.
export function dragOver(e, id, along) {
  if (drag.id === null || drag.id === id) return;
  e.preventDefault();
  e.dataTransfer.dropEffect = 'move';
  const r = e.currentTarget.getBoundingClientRect();
  drag.over = id;
  drag.after = along === 'y' ? e.clientY > r.top + r.height / 2 : e.clientX > r.left + r.width / 2;
}

// drop sends the move: the index is the tab's place in the window's order once it has
// moved.
export function drop(e) {
  e.preventDefault();
  const rest = state.tabs.map((t) => t.id).filter((id) => id !== drag.id);
  const at = rest.indexOf(drag.over);
  if (drag.id !== null && at >= 0) send({ t: 'moveTab', id: drag.id, index: at + (drag.after ? 1 : 0) });
  reset();
}

// dragEnd, when the tab was let go outside the window and the window has another, moves it
// to a new window, as Move Tab to New Window does.
export function dragEnd(e) {
  const outside = e.clientX < 0 || e.clientY < 0 || e.clientX > innerWidth || e.clientY > innerHeight;
  if (drag.id !== null && outside && e.dataTransfer.dropEffect === 'none' && state.tabs.length > 1) {
    if (drag.id !== state.active) send({ t: 'activateTab', id: drag.id });
    send({ t: 'run', text: 'tab.moveToNewWindow' });
  }
  reset();
}

// dropClass is the row's mark while a tab would land beside it.
export function dropClass(id) {
  return drag.over === id ? (drag.after ? 'drop-after' : 'drop-before') : '';
}
