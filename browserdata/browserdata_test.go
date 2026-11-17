package browserdata

import (
	"bytes"
	"path/filepath"
	"slices"
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
			got = append(got, b.Name+" · "+p.Name)
		}
	}
	// The System Profile and a Firefox profile whose folder is gone aren't listed.
	want := []string{"Chrome · Personal", "Chrome · Work", "Firefox · default-release"}
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
}

func TestHistoryIsWebPagesVisited(t *testing.T) {
	ps := found(t)
	for key, want := range map[string][]string{
		// Not chrome:// pages, hidden ones, or those never visited.
		"chrome/Personal":         {"http://example.org/old", "https://example.com/docs", "https://example.com/tracker"},
		"chrome/Work":             nil,
		"firefox/default-release": {"https://example.net/fx", "https://example.net/read"},
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
}

func TestSearchIsTheOneChosenOrTheBrowsers(t *testing.T) {
	ps := found(t)
	for key, want := range map[string]string{
		"chrome/Personal":         "Bing https://www.bing.com/search?q={searchTerms}&form=QBLH",
		"chrome/Work":             "Google https://www.google.com/search?q={searchTerms}",
		"firefox/default-release": "DuckDuckGo https://duckduckgo.com/?q={searchTerms}",
	} {
		s, err := ps[key].Search()
		if got := s.Name + " " + s.URL; err != nil || got != want {
			t.Errorf("%s: %s, %v; want %s", key, got, err, want)
		}
	}
}

func TestCounts(t *testing.T) {
	c, err := found(t)["chrome/Personal"].Counts()
	want := Counts{Bookmarks: 4, Folders: 2, Pages: 3, Search: "Bing"}
	if err != nil || c != want {
		t.Fatalf("%+v, %v; want %+v", c, err, want)
	}
}

func TestMozLZ4(t *testing.T) {
	// "abcabcabcabc!": three literals, a match of 9 one offset 3 back, then "!".
	block := []byte{0x35, 'a', 'b', 'c', 3, 0, 0x10, '!'}
	src := append([]byte("mozLz40\x00\x0d\x00\x00\x00"), block...)
	got, err := mozlz4(src)
	if err != nil || string(got) != "abcabcabcabc!" {
		t.Fatalf("%q, %v", got, err)
	}
	if _, err := mozlz4(src[:len(src)-1]); err == nil {
		t.Error("a short file opened")
	}
	// The fixture's, written as literals alone.
	b, err := firefoxSearch(filepath.Join(home, "Library/Application Support/Firefox/Profiles/test.default-release"))
	if err != nil || b.Name != "DuckDuckGo" {
		t.Errorf("%+v, %v", b, err)
	}
}
