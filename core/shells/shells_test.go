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

// A desktop offers its desktop shells and no phone shell, and starts on Standard; it loads
// any shell named outright. Android offers the phone shells for a narrow window and
// Standard for a wide one, and loads only those.
func TestEachPlatformOffersItsOwnShells(t *testing.T) {
	defer func(was bool) { OnAndroid = was }(OnAndroid)
	OnAndroid = false
	for _, s := range Listed() {
		if s.Phone {
			t.Fatalf("a desktop offers %s", s.ID)
		}
	}
	if Default() != "pbshell" || len(Listed()) < 2 || !Fits("tmshell") || !Fits("mwshell") || Fits("nothing") {
		t.Fatalf("on a desktop: default %s, listed %v", Default(), Listed())
	}
	OnAndroid = true
	if len(Listed()) != 1 || Default() != "pbshell" || DefaultPhone() != "mwshell" {
		t.Fatalf("on Android: listed %v, default %s, phone %s", Listed(), Default(), DefaultPhone())
	}
	if !Fits("pbshell") || Fits("tmshell") || Fits("mwshell") || !FitsPhone("mkshell") || FitsPhone("pbshell") {
		t.Fatal("on Android, Fits takes a shell it doesn't list")
	}
}

// A window is narrow under 600 dp wide or 480 tall, so a phone on its side is too, and
// changes only 40 dp past either line.
func TestAWindowIsNarrowUnder600WideOr480Tall(t *testing.T) {
	for _, c := range []struct {
		w, h      float64
		was, want bool
	}{
		{412, 915, false, true},   // a phone
		{891, 412, false, true},   // a phone on its side
		{841, 673, true, false},   // a foldable, opened
		{1280, 800, true, false},  // a tablet
		{800, 1280, true, false},  // a tablet, upright
		{580, 900, false, false},  // just under the line, was wide
		{580, 900, true, true},    // the same, was narrow
		{630, 900, true, true},    // 30 past, still narrow
		{650, 900, true, false},   // 50 past
		{1000, 500, true, true},   // 20 over 480 tall, was narrow
		{1000, 430, false, true},  // 50 under 480 tall
	} {
		if got := Narrow(c.w, c.h, c.was); got != c.want {
			t.Errorf("%v by %v, was narrow %v: narrow %v", c.w, c.h, c.was, got)
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

// The APK carries the pages of every web shell Android loads, the phone's and the desktop
// shells it lists, and of no other (engine/build_go.py's ANDROID_PAGES).
func TestTheAPKCarriesEveryShellAndroidLoads(t *testing.T) {
	b, err := os.ReadFile("../../engine/build_go.py")
	if err != nil {
		t.Fatal(err)
	}
	var line string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "ANDROID_PAGES = ") {
			line = l
		}
	}
	for _, s := range All() {
		carried := strings.Contains(line, `"shell/`+s.ID+`"`)
		if want := !s.Native && (s.Phone || s.Android); carried != want {
			t.Errorf("the APK carries %s: %v", s.ID, carried)
		}
	}
}
