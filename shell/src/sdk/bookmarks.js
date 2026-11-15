// What a shell sends about bookmarks: a star on the address bar. What Go sends is
// state.bookmarks (shell.js). The Bookmarks page, a tab of ours, edits and removes them.

import { send } from './channel.js';

// bookmark bookmarks url with title, or takes its bookmark away with on false.
export function bookmark(url, title, on) {
  send({ t: 'bookmark', url, title, on });
}

// isBookmarked says whether url is one of state's bookmarks.
export function isBookmarked(state, url) {
  return !!url && state.bookmarks.some((b) => b.url === url);
}
