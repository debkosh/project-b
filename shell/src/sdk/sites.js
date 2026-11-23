// A site's settings, as a shell meets them. What Go sends is state.permissionPrompts
// (shell.js): each prompt open in the profile, {id, tab, origin, kinds, once}, one per tab
// at most. A shell draws those of its own tabs; how is its own (docs/daily-driver.md).

import { send } from './channel.js';

// promptFor is the prompt open for tab, or null.
export function promptFor(state, tab) {
  return state.permissionPrompts.find((p) => p.tab === tab) || null;
}

// answerPermission answers a prompt: 'allow', 'once' (offered when prompt.once), 'block',
// or 'dismiss' when it was closed without an answer.
export function answerPermission(prompt, choice) {
  send({ t: 'permissionAnswer', id: prompt.id, choice });
}

// permissionHost is the site as a prompt names it: the origin, without https://.
export function permissionHost(prompt) {
  return prompt.origin.replace(/^https:\/\//, '');
}

// permissionAsk is what the site asks for, as the end of a sentence: "use your camera and
// microphone", "show notifications".
export function permissionAsk(prompt) {
  const use = [];
  if (prompt.kinds.includes('camera')) use.push('camera');
  if (prompt.kinds.includes('microphone')) use.push('microphone');
  const parts = [];
  if (use.length) parts.push('use your ' + use.join(' and '));
  if (prompt.kinds.includes('notifications')) parts.push('show notifications');
  return parts.join(' and ');
}

// A prompt with os set is the OS refusing what the site is allowed, its camera or microphone
// (docs/media-permissions.md): osTitle and osLine are its words, ux's, for Android and macOS.
// Its answers are 'settings', which opens the OS's settings and keeps it, and 'dismiss',
// Not now.
function osName(prompt) {
  return prompt.os === 'macos' ? 'macOS' : 'Android';
}

function devices(prompt) {
  return ['camera', 'microphone'].filter((k) => prompt.kinds.includes(k));
}

export function osTitle(prompt) {
  return `${osName(prompt)} hasn't let Project B use your ${devices(prompt).join(' and ')}`;
}

export function osLine(prompt) {
  const both = devices(prompt).length > 1;
  const so = `So ${permissionHost(prompt)} can't use ${both ? 'them' : 'it'}, though you've allowed ${both ? 'them' : 'it'}.`;
  if (prompt.os === 'macos') {
    const pane = devices(prompt).map((k) => k[0].toUpperCase() + k.slice(1)).join(' and ');
    return `${so} Turn on Project B in System Settings › Privacy & Security › ${pane}.`;
  }
  return `${so} Turn ${both ? 'them' : 'it'} on in Project B's app settings.`;
}

// osOffNote is a site setting's line while the OS refuses the app what the site is allowed
// (setting.osOff, docs/media-permissions.md), or '' when it doesn't: ux's words.
export function osOffNote(setting) {
  if (!setting.osOff || setting.value !== 'allow') return '';
  return `Turned off for Project B in ${setting.osOff === 'macos' ? 'System Settings' : 'Android settings'}`;
}

// openMediaSettings opens the OS's settings at kind, the camera or the microphone, where the
// person can let the app use it.
export function openMediaSettings(kind) {
  send({ t: 'openMediaSettings', kind });
}

// openSiteSettings opens settings on the site of the window's active tab.
export function openSiteSettings() {
  send({ t: 'openSiteSettings' });
}

// hasSite says whether a tab's page is a site, which has settings of its own.
export function hasSite(tab) {
  return /^https?:\/\//.test(tab?.url || '');
}

// The site panel (docs/site-features.md). Go sends state.openSites: by origin, each site a
// web tab is open on, {settings: [{kind, title, value, choices?}], sparks: [{id, name, on}]}.
// Settings with choices are ours (ads: on or off; links: allow or block); the rest
// are a page's permissions, "" (ask), "allow" or "block". A tab's adsBlocked counts what the
// ad blocker kept from its page, and its popupsBlocked the tabs its page opened that
// Links to other sites closed.

// siteOrigin is the origin of a tab's page, "" for one that isn't a site.
export function siteOrigin(tab) {
  if (!hasSite(tab)) return '';
  try {
    return new URL(tab.url).origin;
  } catch {
    return '';
  }
}

// openSite is the settings of the site a tab shows, or null.
export function openSite(state, tab) {
  return state.openSites?.[siteOrigin(tab)] || null;
}

// setSiteSetting changes one of a site's settings: a kind's value, or a spark's on the site,
// kind "spark:<id>" with "on" or "off".
export function setSiteSetting(origin, kind, value) {
  send({ t: 'setSiteSetting', origin, kind, value });
}

// reloadPage reloads the active tab, for a change that applies when the page loads again.
export function reloadPage() {
  send({ t: 'reload' });
}

// siteSetting is the setting of kind in a site's list, or null.
export function siteSetting(site, kind) {
  return site?.settings.find((s) => s.kind === kind) || null;
}

// sparkNames is the sparks that run on a page, as the site panel says them: "A", "A and B",
// or "A, B and 2 more".
export function sparkNames(names) {
  if (names.length <= 2) return names.join(' and ');
  return `${names[0]}, ${names[1]} and ${names.length - 2} more`;
}

// The words for our own kinds' values, and a permission's.
export const linkChoices = [
  { value: 'allow', label: 'Allow' },
  { value: 'block', label: 'Block' },
];

// popupsWord says how many popups Links to other sites closed on a tab's page.
export function popupsWord(n) {
  return n === 1 ? '1 popup blocked' : `${n} popups blocked`;
}

export function permissionWord(value) {
  return { allow: 'Allowed', block: 'Blocked' }[value] || 'Ask';
}
