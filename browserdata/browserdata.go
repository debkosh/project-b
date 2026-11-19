// Package browserdata reads what other browsers on this Mac keep: which are installed and
// their profiles, and from a profile its bookmarks with their folders, its history and the
// extensions its person installed. It only reads, each database from a copy, as the browser may have it open;
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

// Browser is a browser whose profiles were found: its id, its name and its profiles.
type Browser struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
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

// Counts is what a profile holds, for a row that says so.
type Counts struct {
	Bookmarks int `json:"bookmarks"`
	Folders   int `json:"folders"`
	Pages     int `json:"pages"`
	// Extensions is how many of its extensions can be installed again from the store.
	Extensions int `json:"extensions,omitempty"`
	// Passwords is how many saved passwords of a site's it has, counted without unlocking.
	Passwords int `json:"passwords,omitempty"`
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

// kind is a browser this package knows: its id and name, the folder under Application
// Support its profiles are in (Safari's are in Library), and whose files it keeps. A fork
// keeps its parent's, so another one is a line here and nothing else.
type kind struct {
	id, name, data string
	family         family
	// keychain is the names a Chromium browser keeps its passwords' key under in the login
	// keychain, "<name> Safe Storage", the first the keychain has taken.
	keychain []string
}

var kinds = []kind{
	{id: "chrome", name: "Chrome", data: "Google/Chrome", family: chromium, keychain: []string{"Chrome"}},
	{id: "chrome-beta", name: "Chrome Beta", data: "Google/Chrome Beta", family: chromium, keychain: []string{"Chrome", "Chromium"}},
	{id: "chrome-canary", name: "Chrome Canary", data: "Google/Chrome Canary", family: chromium, keychain: []string{"Chrome", "Chromium"}},
	{id: "edge", name: "Edge", data: "Microsoft Edge", family: chromium, keychain: []string{"Microsoft Edge"}},
	{id: "edge-beta", name: "Edge Beta", data: "Microsoft Edge Beta", family: chromium, keychain: []string{"Microsoft Edge"}},
	{id: "edge-dev", name: "Edge Dev", data: "Microsoft Edge Dev", family: chromium, keychain: []string{"Microsoft Edge"}},
	{id: "brave", name: "Brave", data: "BraveSoftware/Brave-Browser", family: chromium, keychain: []string{"Brave Browser", "Brave"}},
	{id: "arc", name: "Arc", data: "Arc/User Data", family: chromium, keychain: []string{"Arc"}},
	{id: "chromium", name: "Chromium", data: "Chromium", family: chromium, keychain: []string{"Chromium"}},
	{id: "vivaldi", name: "Vivaldi", data: "Vivaldi", family: chromium, keychain: []string{"Vivaldi"}},
	{id: "opera", name: "Opera", data: "com.operasoftware.Opera", family: chromium, keychain: []string{"Opera"}},
	{id: "opera-gx", name: "Opera GX", data: "com.operasoftware.OperaGX", family: chromium, keychain: []string{"Opera"}},
	{id: "yandex", name: "Yandex Browser", data: "Yandex/YandexBrowser", family: chromium, keychain: []string{"Yandex"}},
	{id: "firefox", name: "Firefox", data: "Firefox", family: firefox},
	{id: "zen", name: "Zen", data: "zen", family: firefox},
	{id: "librewolf", name: "LibreWolf", data: "librewolf", family: firefox},
	{id: "waterfox", name: "Waterfox", data: "Waterfox", family: firefox},
	{id: "floorp", name: "Floorp", data: "Floorp", family: firefox},
	{id: "safari", name: "Safari", family: safari},
	// A folder the person chose of a browser this package doesn't know by name: the
	// Chromium kind or the Firefox kind, named after the folder.
	{id: OtherChromium, family: chromium},
	{id: OtherFirefox, family: firefox},
}

// The ids of a browser known only by the folder the person chose.
const (
	OtherChromium = "other-chromium"
	OtherFirefox  = "other-firefox"
)

func kindOf(id string) (kind, bool) {
	for _, k := range kinds {
		if k.id == id {
			return k, true
		}
	}
	return kind{}, false
}

// Support is where the browsers keep their folders under the home folder home: the Mac's
// Application Support.
func Support(home string) string { return filepath.Join(home, "Library/Application Support") }

// Find lists the browsers with a profile in the home folder home, each with its profiles,
// in the order this package knows them.
func Find(home string) []Browser {
	var out []Browser
	for _, k := range kinds {
		if k.data == "" && k.family != safari {
			continue
		}
		dir := filepath.Join(Support(home), k.data)
		var profiles []Profile
		switch k.family {
		case chromium:
			profiles = chromiumProfiles(k.id, dir)
		case firefox:
			profiles = firefoxProfiles(k.id, dir)
		case safari:
			profiles = safariProfiles(home)
		}
		if len(profiles) == 0 {
			continue
		}
		out = append(out, Browser{ID: k.id, Name: k.name, Profiles: profiles})
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
	exts, eerr := p.Extensions()
	for _, e := range exts {
		if e.Skip == "" {
			c.Extensions++
		}
	}
	var lerr error
	c.Passwords, lerr = countLogins(p)
	return c, errors.Join(err, herr, eerr, lerr)
}

// web says whether an address is a web page's, as history keeps.
func web(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
