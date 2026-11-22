// Where a Linux desktop's own settings set what an app can't set for itself
// (docs/research/linux.md, 4): no portal makes an app the default browser, so the pages say
// where to go. desktop is Go's, "gnome", "kde" or "other".
export function setDefaultBrowserHint(desktop) {
  if (desktop === 'gnome') return 'Set it in GNOME Settings, under Default Apps.';
  if (desktop === 'kde') return "Set it in KDE's System Settings › Default Applications › Web browser.";
  return "Set it in your desktop's settings, under Default Apps.";
}
