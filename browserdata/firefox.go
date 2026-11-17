package browserdata

import (
	"bufio"
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Firefox's files: profiles.ini in its folder names its profiles; each keeps bookmarks and
// history in places.sqlite, and its search engines in search.json.mozlz4.

// firefoxProfiles lists the profiles profiles.ini names, in its order, that have places.
func firefoxProfiles(dir string) []Profile {
	f, err := os.Open(filepath.Join(dir, "profiles.ini"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Profile
	var name, path string
	relative := true
	flush := func() {
		if path != "" {
			if relative {
				path = filepath.Join(dir, filepath.FromSlash(path))
			}
			if exists(filepath.Join(path, "places.sqlite")) {
				out = append(out, Profile{Browser: "firefox", Dir: path, Name: cmp.Or(name, filepath.Base(path))})
			}
		}
		name, path, relative = "", "", true
	}
	sc := bufio.NewScanner(f)
	profile := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			flush()
			profile = strings.HasPrefix(line, "[Profile")
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || !profile {
			continue
		}
		switch k {
		case "Name":
			name = v
		case "Path":
			path = v
		case "IsRelative":
			relative = v == "1"
		}
	}
	flush()
	return out
}

// Firefox's top folders, by their guids, in the order it shows them.
var firefoxRoots = []struct{ guid, name string }{
	{"toolbar_____", "Bookmarks Toolbar"},
	{"menu________", "Bookmarks Menu"},
	{"unfiled_____", "Other Bookmarks"},
	{"mobile______", "Mobile Bookmarks"},
}

func firefoxBookmarks(dir string) ([]Bookmark, error) {
	type entry struct {
		id, parent int64
		folder     bool
		b          Bookmark
		guid       string
	}
	var entries []entry
	// Type 1 is a bookmark, 2 a folder, 3 a separator; place: addresses are Firefox's own
	// saved searches.
	err := query(filepath.Join(dir, "places.sqlite"), `
		SELECT b.id, b.parent, b.type, COALESCE(b.title, ''), b.dateAdded, b.guid, COALESCE(p.url, '')
		FROM moz_bookmarks b LEFT JOIN moz_places p ON b.fk = p.id
		WHERE b.type IN (1, 2)
		ORDER BY b.parent, b.position`, func(r row) {
		e := entry{id: r.int(0), parent: r.int(1), folder: r.int(2) == 2, guid: r.text(5)}
		e.b = Bookmark{Title: r.text(3), Added: time.UnixMicro(r.int(4)).UTC()}
		if !e.folder {
			e.b.URL = r.text(6)
			if e.b.URL == "" || strings.HasPrefix(e.b.URL, "place:") {
				return
			}
		}
		entries = append(entries, e)
	})
	if err != nil {
		return nil, err
	}
	children := map[int64][]entry{}
	for _, e := range entries {
		children[e.parent] = append(children[e.parent], e)
	}
	var build func(e entry) Bookmark
	build = func(e entry) Bookmark {
		b := e.b
		for _, c := range children[e.id] {
			b.Children = append(b.Children, build(c))
		}
		return b
	}
	var out []Bookmark
	for _, r := range firefoxRoots {
		for _, e := range entries {
			if e.guid == r.guid {
				root := build(e)
				root.Title = r.name
				out = append(out, root)
			}
		}
	}
	return out, nil
}

const firefoxPagesWhere = `FROM moz_places WHERE hidden = 0 AND visit_count > 0 AND last_visit_date IS NOT NULL AND (url LIKE 'http://%' OR url LIKE 'https://%')`

func firefoxHistory(dir string) ([]Page, error) {
	var out []Page
	err := query(filepath.Join(dir, "places.sqlite"), `SELECT url, COALESCE(title, ''), visit_count, last_visit_date `+firefoxPagesWhere, func(r row) {
		out = append(out, Page{URL: r.text(0), Title: r.text(1), Visits: int(r.int(2)), Last: time.UnixMicro(r.int(3)).UTC()})
	})
	return out, err
}

func firefoxPages(dir string) (int, error) {
	n := 0
	err := query(filepath.Join(dir, "places.sqlite"), `SELECT COUNT(*) `+firefoxPagesWhere, func(r row) { n = int(r.int(0)) })
	return n, err
}

// The engines Firefox ships, by their ids, whose addresses search.json leaves out.
var firefoxEngines = map[string]Search{
	"google":    google,
	"bing":      bing,
	"ddg":       {Name: "DuckDuckGo", URL: "https://duckduckgo.com/?q={searchTerms}"},
	"ecosia":    {Name: "Ecosia", URL: "https://www.ecosia.org/search?q={searchTerms}"},
	"wikipedia": {Name: "Wikipedia", URL: "https://en.wikipedia.org/wiki/Special:Search?search={searchTerms}"},
}

// firefoxSearch is the search engine chosen in search.json.mozlz4, or none when Firefox's
// own is kept.
func firefoxSearch(dir string) (Search, error) {
	b, err := os.ReadFile(filepath.Join(dir, "search.json.mozlz4"))
	if os.IsNotExist(err) {
		return Search{}, nil
	}
	if err != nil {
		return Search{}, err
	}
	if b, err = mozlz4(b); err != nil {
		return Search{}, err
	}
	var f struct {
		Meta struct {
			Default string `json:"defaultEngineId"`
		} `json:"metaData"`
		Engines []struct {
			ID   string `json:"id"`
			Name string `json:"_name"`
			URLs []struct {
				Template string `json:"template"`
				Type     string `json:"type"`
			} `json:"_urls"`
		} `json:"engines"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return Search{}, err
	}
	if f.Meta.Default == "" {
		return Search{}, nil
	}
	for _, e := range f.Engines {
		if e.ID != f.Meta.Default {
			continue
		}
		for _, u := range e.URLs {
			if u.Type == "" || u.Type == "text/html" {
				return Search{Name: e.Name, URL: u.Template}, nil
			}
		}
	}
	// Engines Firefox ships have ids like google or ddg, sometimes with an @suffix.
	id, _, _ := strings.Cut(f.Meta.Default, "@")
	return firefoxEngines[id], nil
}
