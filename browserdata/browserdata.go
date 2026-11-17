// Package browserdata reads what other browsers on this Mac keep: which are installed and
// their profiles, and from a profile its bookmarks with their folders, its history and its
// search engine. It only reads, each database from a copy, as the browser may have it open;
// what to do with what it finds is the caller's.
//
// Chrome, Edge, Brave and Arc share Chromium's files; Firefox and Safari have their own,
// and Safari's are behind Full Disk Access. SQLite is macOS's own libsqlite3, so nothing is
// fetched to build it.
package browserdata

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Browser is a browser whose profiles were found: its id, its name, the app it is, when
// installed, and its profiles.
type Browser struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	App      string    `json:"app,omitempty"`
	Profiles []Profile `json:"profiles"`
}

// Profile is one profile of a browser: the browser's id, the profile's folder and its name,
// none for a browser of one profile, as Safari is.
type Profile struct {
	Browser string `json:"browser"`
	Dir     string `json:"dir"`
	Name    string `json:"name,omitempty"`
}

// Bookmark is a bookmark, or a folder, which has no URL, with what it holds.
type Bookmark struct {
	Title    string     `json:"title"`
	URL      string     `json:"url,omitempty"`
	Added    time.Time  `json:"added,omitzero"`
	Children []Bookmark `json:"children,omitempty"`
}

// Folder says whether b is a folder.
func (b Bookmark) Folder() bool { return b.URL == "" }

// Page is an address in the history: its title, how many visits it had and the last.
type Page struct {
	URL    string    `json:"url"`
	Title  string    `json:"title"`
	Visits int       `json:"visits"`
	Last   time.Time `json:"last"`
}

// Search is a search engine: its name, and its address with {searchTerms} where the words go.
type Search struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Counts is what a profile holds, for a row that says so.
type Counts struct {
	Bookmarks int    `json:"bookmarks"`
	Folders   int    `json:"folders"`
	Pages     int    `json:"pages"`
	Search    string `json:"search,omitempty"`
	// Extensions is how many of its extensions can be installed again from the store.
	Extensions int `json:"extensions,omitempty"`
}

// ErrNotOnThisSystem is what a read that needs macOS returns elsewhere.
var ErrNotOnThisSystem = errors.New("browserdata: not on this system")

// family is how a browser keeps its profiles: Chromium's files, Firefox's or Safari's.
type family int

const (
	chromium family = iota
	firefox
	safari
)

// kind is a browser this package knows: where its data is under Application Support (Safari's
// is in Library), its app's name, and the search engine it has until its person picks one.
type kind struct {
	id, name, data, app string
	family              family
	search              Search
}

var (
	google = Search{Name: "Google", URL: "https://www.google.com/search?q={searchTerms}"}
	bing   = Search{Name: "Bing", URL: "https://www.bing.com/search?q={searchTerms}"}
	brave  = Search{Name: "Brave Search", URL: "https://search.brave.com/search?q={searchTerms}"}
)

var kinds = []kind{
	{id: "chrome", name: "Chrome", data: "Google/Chrome", app: "Google Chrome.app", family: chromium, search: google},
	{id: "edge", name: "Edge", data: "Microsoft Edge", app: "Microsoft Edge.app", family: chromium, search: bing},
	{id: "brave", name: "Brave", data: "BraveSoftware/Brave-Browser", app: "Brave Browser.app", family: chromium, search: brave},
	{id: "arc", name: "Arc", data: "Arc/User Data", app: "Arc.app", family: chromium, search: google},
	{id: "firefox", name: "Firefox", data: "Firefox", app: "Firefox.app", family: firefox, search: google},
	{id: "safari", name: "Safari", app: "Safari.app", family: safari, search: google},
}

func kindOf(id string) (kind, bool) {
	for _, k := range kinds {
		if k.id == id {
			return k, true
		}
	}
	return kind{}, false
}

// Find lists the browsers with a profile in the home folder home, each with its profiles,
// in the order this package knows them. A browser's app is looked for in /Applications and
// in home's Applications.
func Find(home string) []Browser {
	var out []Browser
	for _, k := range kinds {
		dir := filepath.Join(home, "Library/Application Support", k.data)
		var profiles []Profile
		switch k.family {
		case chromium:
			profiles = chromiumProfiles(k.id, dir)
		case firefox:
			profiles = firefoxProfiles(dir)
		case safari:
			profiles = safariProfiles(home)
		}
		if len(profiles) == 0 {
			continue
		}
		b := Browser{ID: k.id, Name: k.name, Profiles: profiles}
		for _, apps := range []string{"/Applications", filepath.Join(home, "Applications")} {
			if p := filepath.Join(apps, k.app); exists(p) {
				b.App = p
				break
			}
		}
		out = append(out, b)
	}
	return out
}

// Bookmarks is the profile's bookmark tree: its top folders, such as the bookmarks bar,
// with what they hold, those that hold nothing left out.
func (p Profile) Bookmarks() ([]Bookmark, error) {
	k, ok := kindOf(p.Browser)
	if !ok {
		return nil, errors.New("browserdata: no browser " + p.Browser)
	}
	var roots []Bookmark
	var err error
	switch k.family {
	case firefox:
		roots, err = firefoxBookmarks(p.Dir)
	case safari:
		roots, err = safariBookmarks(p.Dir)
	default:
		roots, err = chromiumBookmarks(p.Dir)
	}
	var kept []Bookmark
	for _, r := range roots {
		if len(r.Children) > 0 {
			kept = append(kept, r)
		}
	}
	return kept, err
}

// History is every web page the profile has visited, once each.
func (p Profile) History() ([]Page, error) {
	k, ok := kindOf(p.Browser)
	if !ok {
		return nil, errors.New("browserdata: no browser " + p.Browser)
	}
	switch k.family {
	case firefox:
		return firefoxHistory(p.Dir)
	case safari:
		return safariHistory(p.Dir)
	}
	return chromiumHistory(p.Dir)
}

// Search is the profile's search engine: the one chosen, or the browser's own until one is.
// One this package has no address for has its name alone.
func (p Profile) Search() (Search, error) {
	k, ok := kindOf(p.Browser)
	if !ok {
		return Search{}, errors.New("browserdata: no browser " + p.Browser)
	}
	var s Search
	var err error
	switch k.family {
	case firefox:
		s, err = firefoxSearch(p.Dir)
	case safari:
		s, err = safariSearch(p.Dir)
	default:
		s, err = chromiumSearch(p.Dir)
	}
	if s.Name == "" && err == nil {
		s = k.search
	}
	return s, err
}

// Counts says what the profile holds. A part it can't read counts none, and the first
// error is returned with what was counted.
func (p Profile) Counts() (Counts, error) {
	var c Counts
	roots, err := p.Bookmarks()
	var count func([]Bookmark)
	count = func(bs []Bookmark) {
		for _, b := range bs {
			if b.Folder() {
				c.Folders++
				count(b.Children)
			} else {
				c.Bookmarks++
			}
		}
	}
	for _, r := range roots {
		count(r.Children)
	}
	k, _ := kindOf(p.Browser)
	var n int
	var herr error
	switch k.family {
	case firefox:
		n, herr = firefoxPages(p.Dir)
	case safari:
		n, herr = safariPages(p.Dir)
	default:
		n, herr = chromiumPages(p.Dir)
	}
	c.Pages = n
	s, serr := p.Search()
	c.Search = s.Name
	exts, eerr := p.Extensions()
	for _, e := range exts {
		if e.Skip == "" {
			c.Extensions++
		}
	}
	return c, errors.Join(err, herr, serr, eerr)
}

// web says whether an address is a web page's, as history keeps.
func web(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
