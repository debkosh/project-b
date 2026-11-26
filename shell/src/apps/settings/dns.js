// Secure DNS's words and the form a custom template takes (docs/secure-dns.md): the check
// Go's securedns.Valid makes, so a template the page sends is one Go keeps.

export const dnsModes = [
  { value: 'off', label: 'Off' },
  { value: 'automatic', label: 'Automatic' },
  { value: 'secure', label: 'Secure only' },
];

export const dnsHints = {
  off: 'Your network can see which sites you look up',
  automatic: 'Encrypted when your provider answers, otherwise as before',
  secure: "Always encrypted. Sites it can't look up this way won't load",
};

export const notDoh = 'Not a DNS-over-HTTPS address, such as https://dns.example/dns-query{?dns}';

// The template with its variables filled, as Chromium fills them to check it.
const expand = (t) => t.replaceAll('{?dns}', '?dns=AAAA').replaceAll('{dns}', 'AAAA');

// Whether t is an https address with a host, whose only variable is dns, and not in the host.
export function validDoh(t) {
  t = t.trim();
  if (!t || /\s/.test(t)) return false;
  const vars = t.match(/\{[^}]*\}?/g) || [];
  if (vars.some((v) => v !== '{dns}' && v !== '{?dns}')) return false;
  if (t.replace(/\{\??dns\}/g, '').match(/[{}]/)) return false;
  let u;
  try {
    u = new URL(expand(t));
  } catch {
    return false;
  }
  if (u.protocol !== 'https:' || !u.hostname || u.username || u.password) return false;
  const host = t.split('://')[1]?.split(/[/?]/)[0] || '';
  return !host.includes('{');
}

// A template's host, as the check's line names it.
export function dnsHost(t) {
  try {
    return new URL(expand(t)).hostname;
  } catch {
    return '';
  }
}

// What the last check said, by the provider's name, or the custom server's host: '' before
// it answered. A provider that didn't answer means pages won't load in Secure only, and in
// Automatic only that lookups go the usual way.
export function dnsStatus(check, name, mode) {
  if (!check.host) return '';
  if (check.ok) return `${name} answers`;
  return mode === 'secure'
    ? `Couldn't reach ${name}, so pages may not load`
    : `Couldn't reach ${name}, so lookups go the usual way until it answers`;
}

// The words search finds Secure DNS by, beside its own.
export const dnsWords = 'DNS over HTTPS, DoH, encrypted DNS, Cloudflare, Google, Quad9, CleanBrowsing, NextDNS';
