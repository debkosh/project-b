import { reactive } from 'vue';

// Bringing things over from another browser (docs/onboarding.md, docs/import.md): what the
// import wizard, in the welcome's step and Settings' Import, hears from Go and says of it.
// Each page's state.js hands Go's messages to importMessage.
//
// browsers: null until Go has looked, then each {id, name, locked, profiles: [{key, name,
//   counts: {bookmarks, folders, pages, passwords, extensions}, primaryPassword}]}; a
//   locked one, Safari's until the app has Full Disk Access, has nothing counted.
//   Extensions are counted only while they can be installed (the Extensions flag), and
//   passwords once there is a store for them (passwords says so).
// folder: what the last folder chosen was, {browser} or {error}, or {cancelled}; null
//   while none is asked for.
// runs: each import by its profile's key: {steps: {bookmarks: {state, result, error}}},
//   state "doing" or "done", and done once the last has ended. Nothing outlives the page.
// file: what the last file brought, {kind, name, result} or {kind, name, error}.
export const imports = reactive({ browsers: null, passwords: false, folder: null, runs: {}, file: null });

export function importMessage(m) {
  switch (m.t) {
    case 'browsers':
      imports.browsers = m.browsers || [];
      imports.passwords = !!m.passwords;
      return true;
    case 'fileImported':
      imports.file = m;
      return true;
    case 'folder':
      imports.folder = m;
      return true;
    case 'importStep': {
      const run = imports.runs[m.key] || (imports.runs[m.key] = { steps: {}, done: false });
      run.steps[m.step] = { state: m.state, result: m.result, error: m.error };
      return true;
    }
    case 'imported':
      if (imports.runs[m.key]) imports.runs[m.key].done = true;
      return true;
    case 'importing':
      return true;
  }
  return false;
}

// startRun forgets what an earlier import of the profile showed, for a new one.
export function startRun(key) {
  imports.runs[key] = { steps: {}, done: false };
}

const number = (n) => n.toLocaleString('en');
export const plural = (n, one, many) => `${number(n)} ${n === 1 ? one : many}`;

// The kinds of things an import brings, in the order Go brings them.
export const kinds = [
  { id: 'bookmarks', label: 'Bookmarks' },
  { id: 'history', label: 'History' },
  { id: 'passwords', label: 'Passwords' },
  { id: 'extensions', label: 'Extensions' },
];

// choicesOf is what a profile can bring over, each {id, label, note}, as Go's import
// message names them.
export function choicesOf(p) {
  const c = p.counts || {};
  const notes = { bookmarks: c.bookmarks, history: c.pages, passwords: c.passwords, extensions: c.extensions };
  return kinds.filter((k) => notes[k.id]).map((k) => ({ ...k, note: number(notes[k.id]) }));
}

// holds is what a profile holds, in a line.
export function holds(p) {
  const c = p.counts || {};
  const parts = [
    c.bookmarks && plural(c.bookmarks, 'bookmark', 'bookmarks'),
    c.pages && plural(c.pages, 'page', 'pages'),
    c.passwords && plural(c.passwords, 'password', 'passwords'),
    c.extensions && plural(c.extensions, 'extension', 'extensions'),
  ].filter(Boolean);
  return parts.length ? parts.join(' · ') : 'Nothing to bring over';
}

// aboutBrowser is a browser's line in the list: its lock, or its profiles.
export function aboutBrowser(b) {
  if (b.locked) return 'Needs Full Disk Access, once';
  if (b.profiles.length === 1) return holds(b.profiles[0]);
  return plural(b.profiles.length, 'profile', 'profiles');
}

// stepLines is one kind's lines while it comes and once it has: {text, note, state, left},
// state "doing", "done", "waiting" or "problem"; left is a line of something left out,
// which isn't a problem and didn't come.
export function stepLines(kind, step) {
  const label = kinds.find((k) => k.id === kind)?.label || kind;
  if (!step) return [{ text: label, state: 'waiting' }];
  const r = step.result;
  if (step.state === 'doing') {
    const far = kind === 'extensions' && r?.total ? `${number(r.done)} of ${number(r.total)}` : '';
    return [{ text: label, note: far, state: 'doing' }];
  }
  const lines = [];
  if (kind === 'bookmarks') {
    if (r?.bookmarks) {
      lines.push({
        text: `${plural(r.bookmarks, 'bookmark', 'bookmarks')}${r.folders ? `, in ${plural(r.folders, 'folder', 'folders')}` : ''}`,
        note: [`under ${r.folder}`, r.skipped && `${number(r.skipped)} bookmarked already`].filter(Boolean).join(' · '),
        state: 'done',
      });
      if (r.readingList) lines.push({ text: `${plural(r.readingList, 'Reading List item', 'Reading List items')}, among them`, state: 'done' });
    } else {
      lines.push({ text: 'No new bookmarks', note: r?.skipped ? `${number(r.skipped)} bookmarked already` : '', state: 'done' });
    }
  } else if (kind === 'passwords') {
    if (step.error) return [{ text: "Passwords didn't come", note: step.error, state: 'problem' }];
    lines.push(passwordsLine(r));
  } else if (kind === 'history') {
    lines.push({ text: r?.pages ? `${plural(r.pages, 'page', 'pages')} of history` : 'No history', state: 'done' });
  } else if (kind === 'extensions' && r) {
    if (r.installed.length) lines.push({ text: `${plural(r.installed.length, 'extension', 'extensions')} installed`, note: r.installed.join(', '), state: 'done' });
    if (r.already.length) lines.push({ text: `${plural(r.already.length, 'extension', 'extensions')} installed already`, note: r.already.join(', '), state: 'done' });
    for (const f of r.failed) lines.push({ text: `${f.name} didn't install`, note: f.why, state: 'problem' });
    for (const s of r.skipped) lines.push({ text: `${s.name} left out`, note: s.why, state: 'done', left: true });
    if (!lines.length) lines.push({ text: 'No extensions to bring over', state: 'done' });
  }
  if (step.error) lines.push({ text: `Some couldn't be read: ${step.error}`, state: 'problem' });
  return lines;
}

// passwordsLine is what passwords brought, in a line.
export function passwordsLine(r) {
  if (!r?.added) return { text: 'No new passwords', note: r?.already ? `${number(r.already)} saved already` : '', state: 'done' };
  const note = [r.already && `${number(r.already)} saved already`, r.skipped && `${number(r.skipped)} left out`].filter(Boolean).join(' · ');
  return { text: plural(r.added, 'password', 'passwords'), note, state: 'done' };
}
