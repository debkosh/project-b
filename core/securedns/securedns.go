// Package securedns is Secure DNS's modes and providers: which DNS-over-HTTPS server a
// profile's lookups go to, if any. The resolving is Chromium's own (net/dns); this package
// holds what the person chooses and checks a typed template's form. docs/secure-dns.md.
package securedns

import (
	"net/url"
	"strings"
)

// Modes, as Chromium's net::SecureDnsMode has them.
const (
	// Off looks names up with the network's own DNS.
	Off = "off"
	// Automatic asks the provider first, once it has answered, and the network's DNS when
	// it fails.
	Automatic = "automatic"
	// Secure asks the provider alone: a name it can't look up fails.
	Secure = "secure"
)

// Number is mode as net::SecureDnsMode numbers it: 0 off, 1 automatic, 2 secure.
func Number(mode string) int {
	switch mode {
	case Automatic:
		return 1
	case Secure:
		return 2
	}
	return 0
}

// IsMode says whether m is one of the modes.
func IsMode(m string) bool { return m == Off || m == Automatic || m == Secure }

// Custom is the provider id of a template the person typed.
const Custom = "custom"

// Default is the provider chosen when a person first turns Secure DNS on.
const Default = "cloudflare"

// Provider is one of the listed DNS-over-HTTPS servers.
type Provider struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Template string `json:"template"`
}

// Providers are the listed ones, in the order the Settings list shows them, with
// Chromium's templates (net/dns/public/doh_provider_entry.cc).
var Providers = []Provider{
	{"cloudflare", "Cloudflare", "https://chrome.cloudflare-dns.com/dns-query"},
	{"google", "Google", "https://dns.google/dns-query{?dns}"},
	{"quad9", "Quad9", "https://dns.quad9.net/dns-query"},
	{"cleanbrowsing", "CleanBrowsing (family)", "https://doh.cleanbrowsing.org/doh/family-filter{?dns}"},
	{"nextdns", "NextDNS", "https://chromium.dns.nextdns.io"},
}

// Get gives the listed provider with id.
func Get(id string) (Provider, bool) {
	for _, p := range Providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// IsProvider says whether id is a listed provider or Custom.
func IsProvider(id string) bool {
	_, ok := Get(id)
	return ok || id == Custom
}

// Template is the template lookups go to for provider, custom's when it is Custom, or ""
// when there is none.
func Template(provider, custom string) string {
	if provider == Custom {
		return custom
	}
	p, _ := Get(provider)
	return p.Template
}

// Host is a template's server, as the Settings line names it.
func Host(template string) string {
	u, err := url.Parse(expand(template))
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// Valid says whether t is a DNS-over-HTTPS template Chromium takes: an https address with
// a host, whose only variable is dns, and not in the host. It is the check
// net::DnsOverHttpsServerConfig::FromString makes, which the engine makes again.
func Valid(t string) bool {
	t = strings.TrimSpace(t)
	if t == "" || strings.ContainsAny(t, " \t\n") {
		return false
	}
	rest := t
	for {
		i := strings.IndexByte(rest, '{')
		if i < 0 {
			break
		}
		j := strings.IndexByte(rest[i:], '}')
		if j < 0 {
			return false
		}
		if v := rest[i+1 : i+j]; v != "dns" && v != "?dns" {
			return false
		}
		rest = rest[i+j+1:]
	}
	if strings.ContainsRune(rest, '}') {
		return false
	}
	u, err := url.Parse(expand(t))
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return false
	}
	return !strings.Contains(hostPart(t), "{")
}

// expand gives t with its variables filled, as Chromium expands them to check it.
func expand(t string) string {
	t = strings.ReplaceAll(t, "{?dns}", "?dns=AAAA")
	return strings.ReplaceAll(t, "{dns}", "AAAA")
}

// hostPart is the part of t between its scheme and its path.
func hostPart(t string) string {
	_, after, ok := strings.Cut(t, "://")
	if !ok {
		return t
	}
	if i := strings.IndexAny(after, "/?"); i >= 0 {
		return after[:i]
	}
	return after
}
