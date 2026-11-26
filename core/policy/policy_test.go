package policy

import (
	"net/url"
	"strings"
	"testing"

	"projectb/engine"
)

// ourPage names our pages as the Chromium engine does, at chrome://<page>/.
func ourPage(raw string) (engine.Page, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "chrome" || u.Host == "" {
		return "", false
	}
	return engine.Page(strings.ToLower(u.Host) + strings.TrimSuffix(u.Path, "/")), true
}

func TestIsAllowedURL(t *testing.T) {
	m := New([]string{"blocked.test"}, ourPage)
	for raw, want := range map[string]bool{
		"https://example.com/":              true,
		"https://blocked.test/":             false,
		"https://www.blocked.test/x":        false,
		"chrome://shell/":                   false,
		"chrome://shell/pbshell/":           false,
		"CHROME://SHELL/index.html":         false,
		"chrome://gpu/":                     true,
		"chrome://process-internals/":       true,
		"https://shell/":                    true,
		"view-source:https://example.com/":  true,
		"view-source:https://blocked.test/": false,
		"view-source:chrome://shell/":       false,
	} {
		if got := m.IsAllowedURL(raw); got != want {
			t.Errorf("IsAllowedURL(%q) = %v, want %v", raw, got, want)
		}
	}
}
