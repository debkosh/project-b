// This release's notes, release-notes.md beside this file, built into the page: its version,
// the `# <version>` heading, and its changes, each a `- ` line, with any indented lines under
// one going on with it. tools/release.sh publishes the same file beside the release's
// manifest (docs/releasing.md).
import text from './release-notes.md?raw';

export function parse(md) {
  const notes = { version: '', changes: [] };
  for (const line of md.split('\n')) {
    const head = /^# (\S+)$/.exec(line);
    if (head) notes.version = head[1];
    else if (line.startsWith('- ')) notes.changes.push(line.slice(2).trim());
    else if (/^\s+\S/.test(line) && notes.changes.length) notes.changes[notes.changes.length - 1] += ' ' + line.trim();
  }
  return notes;
}

export const notes = parse(text);
