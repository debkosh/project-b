// What a shell sends about a page's dialog. What Go sends is state.dialogs (shell.js); how
// one looks is the shell's own. Every shell draws them: the page waits until answered.

import { send } from './channel.js';

// answerDialog answers dialog id: ok or not, and a prompt's text, or a sign-in's username
// and password.
export function answerDialog(id, ok, text = '', password = '') {
  send({ t: 'answerDialog', id, ok, text, password });
}

// answerPasskey says what the person did at a passkey request's step (docs/passkeys.md):
// cancel, pin with the PIN, account with the row's index, phone, key, bluetoothOn,
// bluetoothSettings or retry. The card stays until Go sends the next step or drops it.
export function answerPasskey(id, action, text = '') {
  send({ t: 'answerPasskey', id, action, text });
}

// dialogFor is the dialog the page of tab waits on, or null: the oldest, as the page asks
// one at a time.
export function dialogFor(state, tab) {
  return state.dialogs.find((d) => d.tab === tab) || null;
}
