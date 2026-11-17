// Bringing things over from another browser (docs/onboarding.md): what the welcome's step
// and Settings' Import say of each profile Go found, and of what it brought over. A profile
// is {key, browser, title, icon, locked, counts: {bookmarks, folders, pages, search}, last},
// last being what it brought over last: {at, bookmarks: {bookmarks, folders, skipped},
// folder, pages, search, errors}. A locked one, Safari's until the app has Full Disk Access,
// has nothing counted.

const number = (n) => n.toLocaleString('en');
const plural = (n, one, many) => `${number(n)} ${n === 1 ? one : many}`;

// choicesOf is what a profile can bring over, each {id, label, note}, as Go's import
// message names them.
export function choicesOf(p) {
  const c = p.counts;
  return [
    c.bookmarks && { id: 'bookmarks', label: 'Bookmarks', note: number(c.bookmarks) },
    c.pages && { id: 'history', label: 'History', note: number(c.pages) },
    c.search && { id: 'search', label: 'Search engine', note: c.search },
  ].filter(Boolean);
}

// holds is what a profile holds, in a line.
export function holds(p) {
  if (p.locked) return 'Needs Full Disk Access, in System Settings';
  const c = p.counts;
  const parts = [c.bookmarks && plural(c.bookmarks, 'bookmark', 'bookmarks'), c.pages && plural(c.pages, 'page', 'pages'), c.search && `${c.search} for search`].filter(Boolean);
  return parts.length ? parts.join(' · ') : 'Nothing to bring over';
}

const engineName = (engines, id) => engines.find((e) => e.id === id)?.name || id;

// broughtLines is what an import brought, a line each, {text, note, problem}.
export function broughtLines(b, engines = []) {
  const lines = [];
  const bm = b.bookmarks || {};
  if (bm.bookmarks) {
    lines.push({
      text: `${plural(bm.bookmarks, 'bookmark', 'bookmarks')}${bm.folders ? `, in ${plural(bm.folders, 'folder', 'folders')}` : ''}, under ${b.folder}`,
      note: bm.skipped ? `${number(bm.skipped)} bookmarked already` : '',
    });
  } else if (bm.skipped) {
    lines.push({ text: 'No new bookmarks', note: `${number(bm.skipped)} bookmarked already` });
  }
  if (b.pages) lines.push({ text: `${plural(b.pages, 'page', 'pages')} of history` });
  if (b.search) lines.push({ text: `${engineName(engines, b.search)} as the search engine` });
  for (const e of b.errors || []) lines.push({ text: e, problem: true });
  if (!lines.length) lines.push({ text: 'Nothing new to bring over' });
  return lines;
}

// lastLine is a row's line once it has brought something over: when, and what.
export function lastLine(b, engines = []) {
  const when = new Date(b.at);
  const today = new Date().toDateString() === when.toDateString();
  const bm = b.bookmarks || {};
  const parts = [bm.bookmarks && plural(bm.bookmarks, 'bookmark', 'bookmarks'), b.pages && plural(b.pages, 'page', 'pages'), b.search && engineName(engines, b.search)].filter(Boolean);
  const day = today ? 'today' : when.toLocaleDateString('en', { day: 'numeric', month: 'short' });
  return `Brought over ${day}${parts.length ? `: ${parts.join(', ')}` : ''}`;
}
