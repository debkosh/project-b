// A size as Finder gives it: 1000 to a kilobyte, one decimal place under ten.
export function size(n) {
  if (n < 1000) return `${n} B`;
  for (const [unit, by] of [['KB', 1e3], ['MB', 1e6], ['GB', 1e9], ['TB', 1e12]]) {
    const v = n / by;
    if (v < 1000 || unit === 'TB') return `${v < 10 ? +v.toFixed(1) : Math.round(v)} ${unit}`;
  }
}
