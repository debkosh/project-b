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

// openSiteSettings opens settings on the site of the window's active tab.
export function openSiteSettings() {
  send({ t: 'openSiteSettings' });
}

// hasSite says whether a tab's page is a site, which has settings of its own.
export function hasSite(tab) {
  return /^https?:\/\//.test(tab?.url || '');
}
