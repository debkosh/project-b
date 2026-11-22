// Where a Linux desktop's own settings set what an app can't set for itself
// (docs/research/linux.md, 4): no portal makes an app the default browser, so the pages say
// where to go, and that the app, which reads the desktop's choice as it starts, notices at its
// next start. desktop is Go's, "gnome", "kde" or "other".
export function setDefaultBrowserHint(desktop) {
  const next = ' Project B notices the next time it starts.';
  if (desktop === 'gnome') return 'Set it in GNOME Settings, under Default Apps.' + next;
  if (desktop === 'kde') return "Set it in KDE's System Settings › Default Applications › Web browser." + next;
  return "Set it in your desktop's settings, under Default Apps." + next;
}
