package links

import (
	"os"
	"path/filepath"
	"testing"
)

// Every help page is a file in the repository, at the address GitHub shows it at, and a name
// with none has no address.
func TestHelpPagesAreThere(t *testing.T) {
	for name, file := range HelpPages {
		if _, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(file))); err != nil {
			t.Errorf("help page %q: %v", name, err)
		}
		if Help(name) != Repo+file {
			t.Errorf("help page %q is at %q", name, Help(name))
		}
	}
	if Help("no such page") != "" {
		t.Errorf("a name with no page has an address: %q", Help("no such page"))
	}
}
