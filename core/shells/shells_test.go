package shells

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTheListedShellsAreThoseOfThisKindOfDevice(t *testing.T) {
	var listed []string
	for _, s := range Listed() {
		listed = append(listed, s.ID)
	}
	if len(listed) == 0 || listed[0] != "pbshell" || !Has("mnshell") || Has("Minimal") {
		t.Fatalf("listed %v", listed)
	}
	for _, s := range Listed() {
		if s.Phone {
			t.Fatalf("a phone's shell is listed on a desktop: %+v", s)
		}
	}
}

// Every folder of shell/src/apps/shell is a shell the registry has, and every web shell
// the registry has is a folder there. A native shell has none.
func TestTheRegistryAndTheFoldersNameTheSameShells(t *testing.T) {
	entries, err := os.ReadDir("../../shell/src/apps/shell")
	if err != nil {
		t.Fatal(err)
	}
	var folders, registered []string
	for _, e := range entries {
		if e.IsDir() {
			folders = append(folders, e.Name())
		}
	}
	for _, s := range All() {
		if !s.Native {
			registered = append(registered, s.ID)
		}
	}
	slices.Sort(registered)
	if !slices.Equal(folders, registered) {
		t.Fatalf("the folders are %v, the registry has %v", folders, registered)
	}
}

// A desktop offers no phone shell, a phone only those, and each starts on the first it
// offers: standard on a desktop, mobile on a phone. A desktop loads either named outright;
// a phone doesn't load standard, which the profile templates name.
func TestEachKindOfDeviceOffersItsOwnShells(t *testing.T) {
	defer func(was bool) { OnPhone = was }(OnPhone)
	for _, phone := range []bool{false, true} {
		OnPhone = phone
		for _, s := range Listed() {
			if s.Phone != phone {
				t.Fatalf("on a phone %v, %s is offered", phone, s.ID)
			}
		}
		want := map[bool]string{false: "pbshell", true: "mwshell"}[phone]
		if Default() != want || !Fits(want) || Fits("pbshell") == phone || !Fits("mwshell") || Fits("nothing") {
			t.Fatalf("on a phone %v, the default is %s", phone, Default())
		}
	}
}

// A shell's name and description are written in the registry only: the app's code, the
// menus, the welcome and the homepage read them from there, so renaming a shell is one
// line. Any of them quoted as a string in our source is a copy that would go stale.
func TestNoShellNameIsWrittenOutsideTheRegistry(t *testing.T) {
	var quoted []string
	for _, s := range All() {
		for _, q := range []string{`"`, `'`} {
			quoted = append(quoted, q+s.Name+q, q+s.Description+q)
		}
	}
	for _, root := range []string{"..", "../../shell/src", "../../engine/android"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			switch filepath.Ext(path) {
			case ".go", ".js", ".vue", ".svelte", ".html", ".json", ".kt", ".java":
			default:
				return nil
			}
			if strings.HasSuffix(path, "_test.go") || filepath.Base(path) == "registry.json" {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, q := range quoted {
				if strings.Contains(string(b), q) {
					t.Errorf("%s writes %s, which only the registry should", path, q)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
