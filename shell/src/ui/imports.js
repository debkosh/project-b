// Bringing things over from another browser (docs/onboarding.md): what the welcome's step
// and Settings' Import say of each profile Go found, and of what it brought over. A profile
// is {key, browser, title, locked, counts: {bookmarks, folders, pages, search, extensions},
// last}, last being what it brought over last: {at, bookmarks: {bookmarks, folders,
// skipped}, folder, readingList, pages, search, extensions: {installed, already, failed,
// skipped}, errors}, the extensions by name, failed and skipped each {name, why}. A locked
// one, Safari's until the app has Full Disk Access, has nothing counted. Extensions are
// counted only while the Extensions flag is on, and only Settings offers them.

const number = (n) => n.toLocaleString('en');
const plural = (n, one, many) => `${number(n)} ${n === 1 ? one : many}`;

// choicesOf is what a profile can bring over, each {id, label, note}, as Go's import
// message names them; extensions only where the page offers them.
export function choicesOf(p, { extensions = false } = {}) {
  const c = p.counts;
  return [
    c.bookmarks && { id: 'bookmarks', label: 'Bookmarks', note: number(c.bookmarks) },
    c.pages && { id: 'history', label: 'History', note: number(c.pages) },
    c.search && { id: 'search', label: 'Search engine', note: c.search },
    extensions && c.extensions && { id: 'extensions', label: 'Extensions', note: number(c.extensions) },
  ].filter(Boolean);
}

// holds is what a profile holds, in a line.
export function holds(p, { extensions = false } = {}) {
  if (p.locked) return 'Needs Full Disk Access, in System Settings';
  const c = p.counts;
  const parts = [
    c.bookmarks && plural(c.bookmarks, 'bookmark', 'bookmarks'),
    c.pages && plural(c.pages, 'page', 'pages'),
    c.search && `${c.search} for search`,
    extensions && c.extensions && plural(c.extensions, 'extension', 'extensions'),
  ].filter(Boolean);
  return parts.length ? parts.join(' · ') : 'Nothing to bring over';
}

const engineName = (engines, id) => engines.find((e) => e.id === id)?.name || id;

// broughtLines is what an import brought, a line each, {text, note, problem, left}: a
// kind of thing each, with how many, then each extension that didn't come and why. left
// is a line of something left out, which isn't a problem and didn't come.
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
  if (b.readingList) lines.push({ text: `${plural(b.readingList, 'Reading List item', 'Reading List items')}, in its own folder` });
  if (b.pages) lines.push({ text: `${plural(b.pages, 'page', 'pages')} of history` });
  if (b.search) lines.push({ text: `${engineName(engines, b.search)} as the search engine` });
  const x = b.extensions;
  if (x) {
    if (x.installed.length) lines.push({ text: `${plural(x.installed.length, 'extension', 'extensions')} installed`, note: x.installed.join(', ') });
    if (x.already.length) lines.push({ text: `${plural(x.already.length, 'extension', 'extensions')} installed already`, note: x.already.join(', ') });
    for (const f of x.failed) lines.push({ text: `${f.name} didn't install`, note: f.why, problem: true });
    for (const s of x.skipped) lines.push({ text: `${s.name} left out`, note: s.why, left: true });
    if (!x.installed.length && !x.already.length && !x.failed.length && !x.skipped.length) lines.push({ text: 'No extensions to bring over' });
  }
  for (const e of b.errors || []) lines.push({ text: e, problem: true });
  if (!lines.length) lines.push({ text: 'Nothing new to bring over' });
  return lines;
}

// lastLine is a row's line once it has brought something over: when, and what.
export function lastLine(b, engines = []) {
  const when = new Date(b.at);
  const today = new Date().toDateString() === when.toDateString();
  const bm = b.bookmarks || {};
  const parts = [
    bm.bookmarks && plural(bm.bookmarks, 'bookmark', 'bookmarks'),
    b.pages && plural(b.pages, 'page', 'pages'),
    b.search && engineName(engines, b.search),
    b.extensions?.installed.length && plural(b.extensions.installed.length, 'extension', 'extensions'),
  ].filter(Boolean);
  const day = today ? 'today' : when.toLocaleDateString('en', { day: 'numeric', month: 'short' });
  return `Brought over ${day}${parts.length ? `: ${parts.join(', ')}` : ''}`;
}

// busyLine is a row's line while it is being brought over: how far its extensions are,
// once they are being installed.
export function busyLine(progress) {
  return progress ? `Installing ${number(progress.done + 1)} of ${plural(progress.total, 'extension', 'extensions')}…` : 'Bringing over…';
}
