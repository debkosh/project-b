// Package policy decides what's allowed. The first app has a host blocklist, and the
// shell's own page, which no tab may load.
package policy

import (
	"net/url"
	"strings"

	"projectb/engine"
)

// Manager holds the current policy.
type Manager struct {
	blocked []string
	// dotted is each blocked host with a dot before it, the suffix its subdomains end in,
	// made once so a check builds no string.
	dotted  []string
	ourPage func(url string) (engine.Page, bool)
}

// New returns a manager blocking these hosts and their subdomains, and the shell's page,
// which ourPage, the engine's, tells from an address.
func New(blocked []string, ourPage func(url string) (engine.Page, bool)) *Manager {
	m := &Manager{ourPage: ourPage}
	for _, h := range blocked {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			m.blocked = append(m.blocked, h)
			m.dotted = append(m.dotted, "."+h)
		}
	}
	return m
}

// BlockRules are the blocked hosts as the engine refuses them, for every request a page
// makes.
func (m *Manager) BlockRules() engine.BlockRules {
	return engine.BlockRules{Hosts: append([]string(nil), m.blocked...)}
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
	// The engine's other pages load as they would.
	if page, ours := m.ourPage(raw); ours && (page == "shell" || page.Shell() != "") {
		return false
	}
	for i, b := range m.blocked {
		if host == b || strings.HasSuffix(host, m.dotted[i]) {
			return false
		}
	}
	return true
}
