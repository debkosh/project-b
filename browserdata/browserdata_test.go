package browserdata

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// testdata/home is made by testdata/make.sh.
const home = "testdata/home"

func found(t *testing.T) map[string]Profile {
	t.Helper()
	out := map[string]Profile{}
	for _, b := range Find(home) {
		for _, p := range b.Profiles {
			out[b.ID+"/"+p.Name] = p
		}
	}
	return out
}

func TestFindListsEachBrowsersProfiles(t *testing.T) {
	bs := Find(home)
	var got []string
	for _, b := range bs {
		for _, p := range b.Profiles {
			got = append(got, strings.TrimSuffix(b.Name+" · "+p.Name, " · "))
		}
	}
	// The System Profile and a Firefox profile whose folder is gone aren't listed; Safari
	// has one profile, with no name.
	want := []string{"Chrome · Personal", "Chrome · Work", "Firefox · default-release", "Safari"}
	if !slices.Equal(got, want) {
		t.Fatalf("found %q, want %q", got, want)
	}
}

// titles is a tree as text: a folder's title, then what it holds in brackets.
func titles(bs []Bookmark) string {
	var b bytes.Buffer
	for i, x := range bs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(x.Title)
		if x.Folder() {
			b.WriteString(" [" + titles(x.Children) + "]")
		}
	}
	return b.String()
}

func TestBookmarksKeepTheirFolders(t *testing.T) {
	ps := found(t)
	for key, want := range map[string]string{
		// Empty top folders are left out.
		"chrome/Personal": "Bookmarks bar [Docs, Work [Tracker, Deep [Deep link]]], Other bookmarks [News]",
		"chrome/Work":     "Bookmarks bar [Wiki]",
		// Firefox's tags, separators and saved searches aren't bookmarks.
		"firefox/default-release": "Bookmarks Toolbar [Fx], Bookmarks Menu [Reading [Read]]",
		// Safari's History proxy isn't a folder; its top folders take the names Safari shows.
		"safari/": "Favorites [Apple, Recipes [Soup, Bread]], Reading List [Article]",
	} {
		bs, err := ps[key].Bookmarks()
		if err != nil {
			t.Fatal(key, err)
		}
		if got := titles(bs); got != want {
			t.Errorf("%s: %s, want %s", key, got, want)
		}
	}
	bs, _ := ps["chrome/Personal"].Bookmarks()
	if d := bs[0].Children[0]; d.URL != "https://example.com/docs" || d.Added.Year() != 2024 {
		t.Errorf("Docs is %+v", d)
	}
	bs, _ = ps["firefox/default-release"].Bookmarks()
	if fx := bs[0].Children[0]; fx.URL != "https://example.net/fx" || !fx.Added.Equal(time.UnixMicro(1700000000000000)) {
		t.Errorf("Fx is %+v", fx)
	}
	bs, _ = ps["safari/"].Bookmarks()
	added := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	if soup := bs[0].Children[1].Children[0]; soup.URL != "https://example.com/soup" || !soup.Added.Equal(added) {
		t.Errorf("Soup is %+v", soup)
	}
	if a := bs[1].Children[0]; !a.Added.Equal(added) {
		t.Errorf("the Reading List's Article is %+v", a)
	}
}

func TestHistoryIsWebPagesVisited(t *testing.T) {
	ps := found(t)
	for key, want := range map[string][]string{
		// Not chrome:// pages, hidden ones, or those never visited.
		"chrome/Personal":         {"http://example.org/old", "https://example.com/docs", "https://example.com/tracker"},
		"chrome/Work":             nil,
		"firefox/default-release": {"https://example.net/fx", "https://example.net/read"},
		// Not a file, or a page never visited.
		"safari/": {"http://example.org/old", "https://example.com/soup"},
	} {
		pages, err := ps[key].History()
		if err != nil {
			t.Fatal(key, err)
		}
		var got []string
		for _, p := range pages {
			got = append(got, p.URL)
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s: %q, want %q", key, got, want)
		}
		c, err := ps[key].Counts()
		if err != nil || c.Pages != len(want) {
			t.Errorf("%s counts %+v, %v", key, c, err)
		}
	}
	pages, _ := ps["chrome/Personal"].History()
	for _, p := range pages {
		if p.URL == "https://example.com/docs" && (p.Visits != 12 || p.Title != "Docs" || p.Last.Year() != 2024) {
			t.Errorf("Docs is %+v", p)
		}
	}
	// A Safari page's title is its last visit's that has one; its last visit is the latest.
	pages, _ = ps["safari/"].History()
	for _, p := range pages {
		if p.URL == "https://example.com/soup" && (p.Visits != 3 || p.Title != "Soup" || !p.Last.Equal(appleTime(815200000))) {
			t.Errorf("Soup is %+v", p)
		}
	}
}

func TestCounts(t *testing.T) {
	c, err := found(t)["chrome/Personal"].Counts()
	want := Counts{Bookmarks: 4, Folders: 2, Pages: 3, Extensions: 2}
	if err != nil || c != want {
		t.Fatalf("%+v, %v; want %+v", c, err, want)
	}
}

func TestExtensionsAreThoseItsPersonInstalled(t *testing.T) {
	ps := found(t)
	got, err := ps["chrome/Personal"].Extensions()
	if err != nil {
		t.Fatal(err)
	}
	want := []Extension{
		{ID: "pponmlkjihgfedcbaponmlkjihgfedcb", Name: "my-extension", Skip: SkipUnpacked},
		{ID: "nngceckbapebfimnlniiiahkandclblb", Name: "Old Notes", Version: "1.9", Skip: SkipManifestV2},
		{ID: "abcdefghijklmnopabcdefghijklmnop", Name: "Other Store Helper", Version: "3.0", Skip: SkipNotInStore},
		{ID: "oepjjojhhaclfhmopocgmdfbeiffpheh", Name: "Project B test extension", Version: "1.0"},
		{ID: "ddkjiahejlhfcafbddmgiahcphecmpfh", Name: "uBlock Origin Lite", Version: "2025.1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	for _, key := range []string{"chrome/Work", "firefox/default-release", "safari/"} {
		if e, err := ps[key].Extensions(); len(e) != 0 || err != nil {
			t.Errorf("%s has %+v, %v", key, e, err)
		}
	}
}

func TestSafariIsLockedUntilItsFolderCanBeRead(t *testing.T) {
	ps := found(t)
	if ps["safari/"].Locked() || ps["chrome/Personal"].Locked() {
		t.Fatal("locked with its files there to read")
	}
	// macOS's Full Disk Access refuses as a folder with no permissions does.
	dir := filepath.Join(t.TempDir(), "Library/Safari")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := Profile{Browser: "safari", Dir: dir}
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	if !p.Locked() {
		t.Error("not locked when its folder can't be read")
	}
	os.Chmod(dir, 0o755)
	if p.Locked() {
		t.Error("still locked once it can be read")
	}
}

func TestBplist(t *testing.T) {
	// The fixture's, as Python's plistlib writes them: dictionaries, arrays, strings,
	// dates and booleans.
	b, err := os.ReadFile(filepath.Join(home, "Library/Safari/Bookmarks.plist"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := bplist(b)
	if m, ok := v.(map[string]any); err != nil || !ok || m["WebBookmarkFileVersion"] != int64(1) {
		t.Fatalf("%v, %v", v, err)
	}
	for _, bad := range [][]byte{nil, []byte("bplist00"), b[:len(b)-1], append([]byte("bplist01"), b[8:]...)} {
		if _, err := bplist(bad); err == nil {
			t.Errorf("read %d bytes that aren't a property list", len(bad))
		}
	}
	// A list that holds itself.
	self := []byte("bplist00\xa1\x00\x08\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x0a")
	if _, err := bplist(self); err == nil {
		t.Error("read a list that holds itself")
	}
}
