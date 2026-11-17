// The bookmark tree as Go sends it (docs/bookmark-folders.md): a flat list of entries, each
// {id, parent, kind, title, url, added, modified, place}, kind bookmark or folder, parent ''
// for the top folder, in their places. What the star's panel and the Bookmarks page read
// of it.

// The top folder's name. It is no entry: its id is ''.
export const topFolder = 'Bookmarks';

// folders is every folder as a place a bookmark can go: the top first, then each folder
// under the one it is in, in their places, each {id, title, depth}.
export function folders(list) {
  const under = new Map();
  for (const e of list) {
    if (e.kind !== 'folder') continue;
    if (!under.has(e.parent)) under.set(e.parent, []);
    under.get(e.parent).push(e);
  }
  const out = [{ id: '', title: topFolder, depth: 0 }];
  const walk = (parent, depth) => {
    for (const f of under.get(parent) || []) {
      out.push({ id: f.id, title: f.title, depth });
      walk(f.id, depth + 1);
    }
  };
  walk('', 1);
  return out;
}

// inFolder is what folder id holds: its folders first, then its bookmarks, each in their
// places.
export function inFolder(list, id) {
  const here = list.filter((e) => e.parent === id);
  return [...here.filter((e) => e.kind === 'folder'), ...here.filter((e) => e.kind !== 'folder')];
}

// bookmarksUnder is how many bookmarks folder id holds, in it and in its folders.
export function bookmarksUnder(list, id) {
  let n = 0;
  for (const e of inFolder(list, id)) n += e.kind === 'folder' ? bookmarksUnder(list, e.id) : 1;
  return n;
}

// path is the folders from the top down to folder id, the top and id among them.
export function path(list, id) {
  const out = [];
  for (let at = id; at; ) {
    const f = list.find((e) => e.id === at);
    if (!f) break;
    out.unshift({ id: f.id, title: f.title });
    at = f.parent;
  }
  return [{ id: '', title: topFolder }, ...out];
}
