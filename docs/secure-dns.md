# Secure DNS

A profile's lookups can go to a DNS-over-HTTPS provider instead of the network's own DNS, so
the network can't see or change which sites are looked up. It is Chromium's own secure DNS,
in //net: we choose the mode and the provider, and the network service does the rest.

## What a person sees

Settings, Privacy and sites, a card of its own below the first (mocks:
`~/PW_TOOLS/ux-logs/secure-dns-mock/`, approved 2026-11-26):

- **Secure DNS**: Off | Automatic | Secure only, its hint changing with it:
  - Off: "Your network can see which sites you look up"
  - Automatic: "Encrypted when your provider answers, otherwise as before"
  - Secure only: "Always encrypted. Sites it can't look up this way won't load"
- **Provider**, when not Off, always set: Cloudflare (the one chosen when a person first
  turns it on), Google, Quad9, CleanBrowsing (family), NextDNS, Custom. Its hint is the last
  check's answer: "Cloudflare answers"; one that didn't answer says what that means, in
  Automatic as information, "Couldn't reach Cloudflare, so lookups go the usual way until it
  answers", and in Secure only as an error, "Couldn't reach Cloudflare, so pages may not
  load".
- **Custom provider**, when Custom: a template such as
  `https://dns.example/dns-query{?dns}` (`{?dns}` may be left out: Chromium then asks by
  POST). One that isn't a DoH address is refused as typed, "Not a DNS-over-HTTPS address,
  such as …", once typing pauses; one whose server doesn't answer keeps, with the
  Provider's line as above, the error in Secure only being the field's own.
- Search finds the card by "DoH", "DNS over HTTPS", "encrypted DNS" and each provider's name.

## How each mode behaves

Chromium's, unchanged (net/dns/dns_client.cc, net/dns/resolve_context.cc):

- **Off**: the network's own DNS, as today.
- **Automatic**: the chosen provider, checked in the background from the start. Until it has
  answered once, lookups use the network's DNS. After that each lookup goes to the provider
  first; one that fails is retried on the network's DNS. A provider that fails several
  times in a row is marked down and checked again in the background, and lookups go back
  to it once it answers. No setting turns Automatic off by itself: the rules in Chrome that
  do (enterprise policy, parental controls, a domain-joined Windows machine) are //chrome
  code we don't take.
- **Secure only**: the provider and nothing else. A lookup it can't answer fails, and the
  page shows the usual "can't be reached". `.local` names still go to the system.

The setting is the profile's, in prefs.json, and so the process's: one process per profile
(docs/profiles.md), and Chromium's resolver is one per process. A change applies at once to
every workspace's tabs; lookups in flight at that moment fail and are retried by the page.

## How it's built

- **Go**: core/securedns holds the modes, the listed providers (Chromium's names and
  templates, net/dns/public/doh_provider_entry.cc) and the template's form, the check
  `net::DnsOverHttpsServerConfig::FromString` makes. core/prefs keeps `secureDns` (off,
  automatic, secure), `dnsProvider` (a provider's id, or `custom`) and `dnsCustom` (the
  template, kept when another provider is chosen); a value that isn't one changes nothing.
  core/app (securedns.go) hands the engine the mode and template at start, before any tab,
  and on each change, checks the provider each time it changes, and sends Settings the
  answer (`dnsCheck`). The page checks a typed template's form as Go does, and sends it once
  it's valid and typing has paused.
- **ABI**: `set_secure_dns(mode, template)`, mode as net::SecureDnsMode numbers it;
  `check_secure_dns(template)`, answered by `PB_EVENT_SECURE_DNS_CHECKED` (template,
  whether it looked a name up). The mock engine's check fails a host ending `.invalid`.
- **Engine**: `set_secure_dns` calls `content::GetNetworkService()->ConfigureStubHostResolver`
  (services/network/public/mojom/network_service.mojom), and again from
  `OnNetworkServiceCreated` whenever the network service restarts, as Chrome's
  SystemNetworkContextManager does (engine/browser/secure_dns.cc). The check is a port of
  Chrome's DnsProbeRunner (chrome/browser/net/dns_probe_runner.cc, mojo calls only): it
  resolves a known name in Secure mode against only that template, through a resolver of
  its own on the default network context. A template `net::DnsOverHttpsConfig::FromString`
  refuses is Off.

## Checking it

- Go: `core/securedns` (the template's form), `core/prefs` (kept, and refused values), and
  `core/app/securedns_test.go`: the engine hears Off at start, then each change, and
  Settings shows the check, both answers.
- Settings on the mock engine (`shell/e2e/settings.cjs`): each mode and its words, a
  provider chosen, a custom template refused as typed and kept when valid, an unreachable
  one's error, and on a phone the three modes on a line of their own.
- The real app (`tools/drive_first_app.py securedns`), against a DNS-over-HTTPS server of
  the driver's own on 127.0.0.1 that knows names no one else does: Off never asks it;
  Secure only from the start asks it alone and the page loads; the check asks it too; and
  changed live to Off, then Automatic, the engine follows.

## Per platform

The same on the Mac, Linux and Android. Only the secure part is ours: plain lookups, Off's
and Automatic's fallback, stay with the system's own resolver, as content starts it
(`ContentBrowserClient::GetInitialInsecureDnsMode`), so turning Secure DNS on changes
nothing else. A provider is asked by its template alone, never matched against the system's
servers, so Chromium's limits on its own automatic upgrade (several resolvers on a Mac with a
split VPN, some Linux resolv.conf options, a local nameserver) don't apply. On Android with
Private DNS on, Automatic's fallback goes through Private DNS.

## With a proxy

DoH requests don't go through a proxy (net/dns/dns_http_attempt.cc): they go straight to
the provider on port 443. With an HTTP or HTTPS proxy (bd76942e), a site's name is looked
up by the proxy, so Secure DNS only covers the proxy's own name; with SOCKS5 the same. On a
network that allows nothing but the proxy, the provider can't be reached.

## Decisions

Sudipta, 2026-11-26:

- A new profile starts at **Off**: nothing goes to a provider until a person chooses one.
- **Secure only means secure only**: a Wi-Fi sign-in page that can't be looked up through
  the provider doesn't load, and the hint already says so; a person switches to Automatic
  to sign in. No captive-portal detection.
- **Behind a network that allows only a proxy**, Secure only loads nothing, and the
  provider's check says so ("Couldn't reach Cloudflare, so pages may not load"). Nothing
  else changes.
