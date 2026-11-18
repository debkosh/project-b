package browserdata

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Safari's files, in ~/Library/Safari, which macOS keeps behind Full Disk Access:
// Bookmarks.plist, a binary property list of folders (WebBookmarkTypeList) and bookmarks
// (WebBookmarkTypeLeaf), and History.db, SQLite. Its search engine is macOS's own choice,
// NSPreferredWebServices in the global preferences, which need no access. Its passwords are
// in iCloud Keychain, which no other app can read.

// safariProfiles is Safari's one profile, when its folder is there: whether its files can
// be read is Locked's to say.
func safariProfiles(home string) []Profile {
	dir := filepath.Join(home, "Library/Safari")
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil
	}
	return []Profile{{Browser: "safari", Dir: dir}}
}

// Locked says whether macOS keeps the profile's files from this app until it is given Full
// Disk Access, as it does Safari's.
func (p Profile) Locked() bool {
	if p.Browser != "safari" {
		return false
	}
	_, err := os.ReadDir(p.Dir)
	return errors.Is(err, fs.ErrPermission)
}

// ReadingList is the title of the top folder that holds Safari's Reading List.
const ReadingList = "Reading List"

// safariLists names the top folders of Safari's bookmarks as Safari shows them.
var safariLists = map[string]string{
	"BookmarksBar":          "Favorites",
	"BookmarksMenu":         "Bookmarks Menu",
	"com.apple.ReadingList": ReadingList,
}

func safariBookmarks(dir string) ([]Bookmark, error) {
	b, err := os.ReadFile(filepath.Join(dir, "Bookmarks.plist"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v, err := bplist(b)
	if err != nil {
		return nil, err
	}
	root, _ := v.(map[string]any)
	var out []Bookmark
	for _, c := range lists(root["Children"]) {
		if c["WebBookmarkType"] != "WebBookmarkTypeList" {
			continue
		}
		top := safariBookmark(c)
		if name, ok := safariLists[top.Title]; ok {
			top.Title = name
		}
		out = append(out, top)
	}
	return out, nil
}

// safariBookmark is a folder with what it holds, or a bookmark.
func safariBookmark(n map[string]any) Bookmark {
	title, _ := n["Title"].(string)
	b := Bookmark{Title: title}
	if n["WebBookmarkType"] == "WebBookmarkTypeLeaf" {
		b.URL, _ = n["URLString"].(string)
		uri, _ := n["URIDictionary"].(map[string]any)
		b.Title, _ = uri["title"].(string)
		b.Added, _ = n["dateAdded"].(time.Time)
		if rl, ok := n["ReadingList"].(map[string]any); ok && b.Added.IsZero() {
			b.Added, _ = rl["DateAdded"].(time.Time)
		}
		return b
	}
	for _, c := range lists(n["Children"]) {
		switch c["WebBookmarkType"] {
		case "WebBookmarkTypeList":
			b.Children = append(b.Children, safariBookmark(c))
		case "WebBookmarkTypeLeaf":
			if c := safariBookmark(c); c.URL != "" {
				b.Children = append(b.Children, c)
			}
		}
	}
	return b
}

// lists is a property list's array of dictionaries, those that aren't left out.
func lists(v any) []map[string]any {
	a, _ := v.([]any)
	var out []map[string]any
	for _, x := range a {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// A page's visits are in history_visits, each with the title it had then; visit_time is
// seconds since the start of 2001.
const safariPagesWhere = `FROM history_items i WHERE i.visit_count > 0 AND (i.url LIKE 'http://%' OR i.url LIKE 'https://%')
	AND EXISTS (SELECT 1 FROM history_visits v WHERE v.history_item = i.id)`

func safariHistory(dir string) ([]Page, error) {
	var out []Page
	err := query(filepath.Join(dir, "History.db"), `SELECT i.url,
		COALESCE((SELECT v.title FROM history_visits v WHERE v.history_item = i.id AND v.title <> '' ORDER BY v.visit_time DESC LIMIT 1), ''),
		i.visit_count,
		CAST((SELECT MAX(v.visit_time) FROM history_visits v WHERE v.history_item = i.id) AS INTEGER) `+safariPagesWhere, func(r row) {
		out = append(out, Page{URL: r.text(0), Title: r.text(1), Visits: int(r.int(2)), Last: appleTime(float64(r.int(3)))})
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	return out, err
}

func safariPages(dir string) (int, error) {
	n := 0
	err := query(filepath.Join(dir, "History.db"), `SELECT COUNT(*) `+safariPagesWhere, func(r row) { n = int(r.int(0)) })
	if os.IsNotExist(err) {
		return 0, nil
	}
	return n, err
}

// The engines Safari offers, by the ids macOS keeps.
var safariEngines = map[string]Search{
	"com.google.www": google,
	"com.bing.www":   bing,
	"com.yahoo.www":  {Name: "Yahoo", URL: "https://search.yahoo.com/search?p={searchTerms}"},
	"com.duckduckgo": {Name: "DuckDuckGo", URL: "https://duckduckgo.com/?q={searchTerms}"},
	"org.ecosia.www": {Name: "Ecosia", URL: "https://www.ecosia.org/search?q={searchTerms}"},
}

// safariSearch is the search engine chosen in macOS's preferences, or none when Safari's
// own is kept. dir is Safari's folder, in home/Library.
func safariSearch(dir string) (Search, error) {
	b, err := os.ReadFile(filepath.Join(dir, "../Preferences/.GlobalPreferences.plist"))
	if os.IsNotExist(err) {
		return Search{}, nil
	}
	if err != nil {
		return Search{}, err
	}
	v, err := bplist(b)
	if err != nil {
		return Search{}, err
	}
	prefs, _ := v.(map[string]any)
	services, _ := prefs["NSPreferredWebServices"].(map[string]any)
	web, _ := services["NSWebServicesProviderWebSearch"].(map[string]any)
	id, _ := web["NSProviderIdentifier"].(string)
	if s, ok := safariEngines[id]; ok {
		return s, nil
	}
	if name, _ := web["NSDefaultDisplayName"].(string); name != "" {
		return Search{Name: name}, nil
	}
	return Search{}, nil
}
