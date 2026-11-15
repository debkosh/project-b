// A tab's notice, tab.notice: what Go gives a shell to draw over a page that has nothing to
// show, {kind, title, detail, actions}, each action {id, title, primary}. How it looks is
// the shell's own (docs/daily-driver.md).

import { send } from './channel.js';

// runNotice does one of a tab's notice's actions, by its id.
export function runNotice(tab, action) {
  send({ t: 'noticeAction', id: tab.id, action });
}
