// What a shell sends about pinned tabs. What Go sends is state.pins (shell.js), and on each
// tab the pin it is open for; whether and how a shell draws them is its own
// (docs/windows.md, "Pinned tabs").

import { send } from './channel.js';

// pinTab pins tab id: a pin of its address at the end of its workspace's, with the tab open
// for it. A tab of ours, or one with no address yet, isn't pinned.
export function pinTab(id) {
  send({ t: 'pinTab', id });
}

// unpin removes a pin, by its id. A tab open for it stays, as an ordinary tab.
export function unpin(pin) {
  send({ t: 'unpin', pin });
}

// openPin shows this window's tab for a pin, or opens one for it at its address.
export function openPin(pin) {
  send({ t: 'openPin', pin });
}
