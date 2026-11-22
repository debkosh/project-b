// A release's notes are one file for every platform (docs/releasing.md). A line that starts
// with the platform it is for, "On Linux, …", "On a Mac, …" or "On Android, …" ("On a phone, …"
// is Android's too), shows only there; every other line shows everywhere.
const prefixes = [
  [/^On Linux\b/, 'linux'],
  [/^On (a Mac|macOS)\b/, 'mac'],
  [/^On (Android|a phone)\b/, 'android'],
];

// platformOf is the platform a line is for, or '' when it is for every one.
export function platformOf(line) {
  for (const [re, platform] of prefixes) if (re.test(line)) return platform;
  return '';
}

// forPlatform keeps the lines for platform and the lines for every platform, in order.
export function forPlatform(changes, platform) {
  return changes.filter((line) => {
    const p = platformOf(line);
    return !p || p === platform;
  });
}

// here is this app's platform: Android by the page's user agent, as Go names Android's keymap
// "linux", and otherwise the keymap's platform, "mac" or "linux".
export function here(keymapPlatform, userAgent) {
  return /Android/.test(userAgent) ? 'android' : keymapPlatform;
}
