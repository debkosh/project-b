package help

import (
	"os"
	"path/filepath"
	"testing"
)

// Every help page is a file in the repository, at the address GitHub shows it at, and a name
// with none has no address.
func TestPagesAreThere(t *testing.T) {
	for name, file := range Pages {
		if _, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(file))); err != nil {
			t.Errorf("help page %q: %v", name, err)
		}
		if URL(name) != Repo+file {
			t.Errorf("help page %q is at %q", name, URL(name))
		}
	}
	if URL("no such page") != "" {
		t.Errorf("a name with no page has an address: %q", URL("no such page"))
	}
}
