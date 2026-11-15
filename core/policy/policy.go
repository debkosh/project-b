// Package policy decides what's allowed. The first app has a host blocklist, and the
// shell's own page, which no tab may load.
package policy

import (
	"net/url"
	"strings"
)

// Manager holds the current policy.
type Manager struct {
	blocked []string
	// dotted is each blocked host with a dot before it, the suffix its subdomains end in,
	// made once so a check builds no string.
	dotted []string
}

// New returns a manager blocking these hosts and their subdomains.
func New(blocked []string) *Manager {
	m := &Manager{}
	for _, h := range blocked {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			m.blocked = append(m.blocked, h)
			m.dotted = append(m.dotted, "."+h)
		}
	}
	return m
}

// IsAllowedURL is the one function every URL decision goes through. It's in memory
// and never waits.
func (m *Manager) IsAllowedURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	// The shell's page talks to Go as the shell, so a tab must never hold a copy of it.
	// Other chrome:// pages content serves load as they would in Chrome.
	if u.Scheme == "chrome" && host == "shell" {
		return false
	}
	for i, b := range m.blocked {
		if host == b || strings.HasSuffix(host, m.dotted[i]) {
			return false
		}
	}
	return true
}
