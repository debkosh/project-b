// What a shell sends about bookmarks (docs/bookmark-folders.md): the star, and the panel it
// opens. What Go sends is state.bookmarks, the tree, and state.bookmarkPanel, the panel to
// show (shell.js). The Bookmarks page, a tab of ours, makes folders and moves entries.

import { send } from './channel.js';

export { folders, topFolder } from '@/ui/bookmarks.js';

// bookmark is the star: Go bookmarks url with title into the folder last saved in, or finds
// its bookmark, and answers with bookmarkPanel, the panel for it.
export function bookmark(url, title) {
  send({ t: 'bookmark', url, title });
}

// saveBookmark is the panel's Enter: the bookmark's title and folder, where the next goes too.
export function saveBookmark(id, title, parent) {
  send({ t: 'saveBookmark', bookmark: id, title, parent });
}

// removeBookmark is the panel's Remove.
export function removeBookmark(id) {
  send({ t: 'removeBookmark', bookmark: id });
}

// isBookmarked says whether url is one of state's bookmarks.
export function isBookmarked(state, url) {
  return !!url && state.bookmarks.some((b) => b.kind === 'bookmark' && b.url === url);
}
