package shellfiles

import "testing"

// Names outside dist, and the folder itself, are no file, whatever the build put there.
func TestReadKeepsToDist(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../shellfiles.go", "/settings/index.html", "settings/../../shellfiles.go", "settings"} {
		if _, ok := Read(name); ok {
			t.Errorf("Read(%q) found a file", name)
		}
	}
}
