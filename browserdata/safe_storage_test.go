package browserdata

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// quokka copies the test home's Chrome · Personal, its passwords sealed under
// "test-safe-storage", to a made-up browser's folder of profiles under Application
// Support, dir relative to it, and returns the profile as a folder chosen has it.
func quokka(t *testing.T, dir string) Profile {
	t.Helper()
	support := Support(t.TempDir())
	browser := filepath.Join(support, dir)
	if err := os.CopyFS(filepath.Join(browser, "Default"), os.DirFS("testdata/home/Library/Application Support/Google/Chrome/Default")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(browser, "Local State"), []byte(`{"profile":{"info_cache":{"Default":{"name":"Me"}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return Profile{Browser: OtherChromium, Dir: filepath.Join(browser, "Default")}
}

func services(items []KeychainItem) []string {
	var out []string
	for _, i := range items {
		out = append(out, i.Service)
	}
	return out
}

func item(name string) KeychainItem {
	return KeychainItem{Service: name + " Safe Storage", Account: name}
}

// A browser known only by its folder: the keychain's item named after its folder is its
// key, sure; else the vendor's folder above; else one its folder's name begins with; and
// when a few half match, or none does, they are for the person to choose from.
func TestSafeStorageFromTheFolder(t *testing.T) {
	items := []KeychainItem{item("Chrome"), item("Quokka"), item("Quokka Nightly"), item("Wombat")}
	for _, c := range []struct {
		dir   string
		items []KeychainItem
		want  []string
		sure  bool
	}{
		{"Bramblewood/Quokka", items, []string{"Quokka Safe Storage"}, true},
		{"Bramblewood/quokka nightly", items, []string{"Quokka Nightly Safe Storage"}, true},
		{"Quokka/User Data", items, []string{"Quokka Safe Storage"}, true},
		{"io.bramblewood.Wombat", items, []string{"Wombat Safe Storage"}, true},
		{"Wombat/Wombat-Labs", items, []string{"Wombat Safe Storage"}, true},
		{"Bramblewood/Wombat-Browser", items, []string{"Wombat Safe Storage"}, true},
		{"Bramblewood/Quokka Nightly Beta", items, []string{"Quokka Safe Storage", "Quokka Nightly Safe Storage"}, false},
		{"Bramblewood/Numbat", items, []string{"Chrome Safe Storage", "Quokka Safe Storage", "Quokka Nightly Safe Storage", "Wombat Safe Storage"}, false},
		{"Bramblewood/Numbat", []KeychainItem{item("Wombat")}, []string{"Wombat Safe Storage"}, false},
		{"Bramblewood/Numbat", nil, nil, false},
	} {
		got, sure := quokka(t, c.dir).SafeStorage(c.items)
		if !slices.Equal(services(got), c.want) || sure != c.sure {
			t.Errorf("%s: %q, sure %v; want %q, %v", c.dir, services(got), sure, c.want, c.sure)
		}
	}
	if got, sure := (Profile{Browser: "chrome"}).SafeStorage(items); got != nil || sure {
		t.Errorf("a known browser's: %q %v", services(got), sure)
	}
}

// A folder-known browser's passwords open with the item Keys names, or with none named
// with the one its folder's name gives, read straight from the keychain.
func TestLoginsOfABrowserKnownByItsFolder(t *testing.T) {
	p := quokka(t, "Bramblewood/Quokka")
	asked := []string{}
	keychain := func(want string) func(string, string) ([]byte, error) {
		return func(service, account string) ([]byte, error) {
			asked = append(asked, service)
			if service == want+" Safe Storage" && account == want {
				return []byte("test-safe-storage"), nil
			}
			return nil, ErrNoSecret
		}
	}
	if ls, err := p.Logins(Keys{Secret: keychain("Quokka")}); len(ls) != 3 || err != nil || asked[0] != "Quokka Safe Storage" {
		t.Errorf("by its folder's name: %d, %v, asked %q", len(ls), err, asked)
	}
	asked = nil
	if ls, err := p.Logins(Keys{Secret: keychain("Wombat"), Item: item("Wombat")}); len(ls) != 3 || err != nil || !slices.Equal(asked, []string{"Wombat Safe Storage"}) {
		t.Errorf("the item named: %d, %v, asked %q", len(ls), err, asked)
	}
	asked = nil
	if _, err := p.Logins(Keys{Secret: keychain("Wombat")}); err != ErrNoSecret || !slices.Equal(asked, []string{"Quokka Safe Storage", "Bramblewood Safe Storage", "Chromium Safe Storage"}) {
		t.Errorf("none of its names: %v, asked %q", err, asked)
	}
}
