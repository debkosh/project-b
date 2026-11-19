// What the Downloads page says of a download, in either layout (docs/daily-driver.md).

// A size as a phone or Finder gives it: 1000 to a kilobyte, one decimal place under ten.
export function size(n) {
  if (n < 1000) return `${n} B`;
  for (const [unit, by] of [['KB', 1e3], ['MB', 1e6], ['GB', 1e9], ['TB', 1e12]]) {
    const v = n / by;
    if (v < 1000 || unit === 'TB') return `${v < 10 ? +v.toFixed(1) : Math.round(v)} ${unit}`;
  }
}

// Why one failed, in plain words, from the engine's reason.
const failedWords = { connection: 'Failed: the connection stopped', server: 'Failed: the server refused it', disk: 'Failed: not enough disk space' };

export const hostOf = (u) => {
  try {
    return new URL(u).host.replace(/^www\./, '');
  } catch {
    return '';
  }
};

// The line under a download's name: how far it has got, its size and where it came from,
// or why it stopped.
export function status(d) {
  switch (d.state) {
    case 'progress':
      return (d.paused ? 'Paused · ' : '') + (d.total > 0 ? `${size(d.received)} of ${size(d.total)}` : size(d.received));
    case 'done':
      return [d.total > 0 ? size(d.total) : 'Done', d.url ? hostOf(d.url) : d.from].filter(Boolean).join(' · ');
    case 'cancelled':
      return 'Cancelled';
    default:
      return failedWords[d.reason] || 'Failed';
  }
}
