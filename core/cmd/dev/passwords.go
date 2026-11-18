package main

import (
	"strings"

	"github.com/sudiptadeb/project-b/browserdata"
)

// keychainOf is -keychain: a login keychain that gives secret for every browser, or nil,
// the Mac's own.
func keychainOf(secret string) func(string, string) ([]byte, error) {
	if secret == "" {
		return nil
	}
	return func(string, string) ([]byte, error) { return []byte(secret), nil }
}

// countingStore is -password-store: a passwords store that keeps nothing but which logins
// it has, by origin and username, as the real one counts them, so an import's counts can
// be tested before core/passwords is there. An origin not a web page's is left out.
type countingStore struct {
	saved map[string]bool
}

func (s *countingStore) Import(logins []browserdata.Login) (added, already, skipped int) {
	for _, l := range logins {
		switch key := l.Origin + " " + l.Username; {
		case !strings.HasPrefix(l.Origin, "http://") && !strings.HasPrefix(l.Origin, "https://") || l.Password == "":
			skipped++
		case s.saved[key]:
			already++
		default:
			s.saved[key] = true
			added++
		}
	}
	return added, already, skipped
}
