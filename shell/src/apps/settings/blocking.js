// What the Block lists page and its row say of the lists Go shows (docs/blocking.md), in
// the words of ux's review.

import { size } from '@/ui/size.js';
import { state } from './state.js';

// "20 minutes ago", "2 h ago", "3 days ago", from a time Go gave.
export function ago(when, now = Date.now()) {
  const min = Math.floor((now - Date.parse(when)) / 60000);
  if (!(min >= 1)) return 'just now';
  if (min < 60) return `${min} ${min === 1 ? 'minute' : 'minutes'} ago`;
  if (min < 60 * 24) return `${Math.floor(min / 60)} h ago`;
  const days = Math.floor(min / (60 * 24));
  return `${days} ${days === 1 ? 'day' : 'days'} ago`;
}

// The Block lists row's line: how many of the lists are on, and when they were updated.
export function listsLine() {
  const b = state.blocking;
  const on = b.groups.reduce((n, g) => n + g.onCount, 0);
  const all = b.groups.reduce((n, g) => n + g.total, 0);
  const when = b.downloaded && b.published ? `updated ${ago(b.published)}` : 'not downloaded yet';
  return `${on} of ${all} lists on · ${when}`;
}

// A sentence of Go's as a line's first part, without its full stop.
export const clause = (s) => (s || '').replace(/\.$/, '');

// A Safe Browsing group's name under the Dangerous sites heading: "Malware" for
// "Dangerous sites: malware".
export function groupName(g) {
  const t = g.title.replace(/^Dangerous sites: /, '');
  return t.charAt(0).toUpperCase() + t.slice(1);
}

// A list's home, by its host: "easylist.to".
export function homeHost(home) {
  try {
    return new URL(home).host.replace(/^www\./, '');
  } catch {
    return home || '';
  }
}

// What a list says on its right: when it last changed and its size, or that it isn't here
// yet.
export function listMeta(l) {
  if (!l.downloaded) return 'Not downloaded yet';
  return [l.changed ? ago(l.changed) : '', l.size ? size(l.size) : ''].filter((x) => x).join(' · ');
}
