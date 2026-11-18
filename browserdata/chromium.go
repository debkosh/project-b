package browserdata

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Chromium's files: Local State in the browser's folder names its profiles, each a folder
// with Bookmarks (JSON), History (SQLite) and Preferences (JSON).

// chromiumProfiles lists the profiles in dir: those Local State names, in its order of
// folders, that hold bookmarks or history; with no Local State, the folders that do.
func chromiumProfiles(browser, dir string) []Profile {
	var state struct {
		Profile struct {
			InfoCache map[string]struct {
				Name string `json:"name"`
			} `json:"info_cache"`
		} `json:"profile"`
	}
	names := map[string]string{}
	if b, err := os.ReadFile(filepath.Join(dir, "Local State")); err == nil && json.Unmarshal(b, &state) == nil {
		for d, p := range state.Profile.InfoCache {
			names[d] = p.Name
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Profile
	for _, e := range entries {
		d := e.Name()
		if !e.IsDir() || d == "System Profile" || d == "Guest Profile" {
			continue
		}
		if _, named := names[d]; !named && d != "Default" && !strings.HasPrefix(d, "Profile ") {
			continue
		}
		path := filepath.Join(dir, d)
		if !exists(filepath.Join(path, "Bookmarks")) && !exists(filepath.Join(path, "History")) {
			continue
		}
		out = append(out, Profile{Browser: browser, Dir: path, Name: cmp.Or(names[d], d)})
	}
	// Default first, then Profile 1, Profile 2, … by number.
	slices.SortFunc(out, func(a, b Profile) int { return profileOrder(filepath.Base(a.Dir)) - profileOrder(filepath.Base(b.Dir)) })
	return out
}

func profileOrder(dir string) int {
	if dir == "Default" {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimPrefix(dir, "Profile "))
	if err != nil {
		return 1 << 20
	}
	return n + 1
}

type chromiumNode struct {
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	URL       string         `json:"url"`
	DateAdded string         `json:"date_added"`
	Children  []chromiumNode `json:"children"`
}

// The top folders, in the order Chrome shows them, and their names where a file has none.
var chromiumRoots = []struct{ key, name string }{
	{"bookmark_bar", "Bookmarks bar"},
	{"other", "Other bookmarks"},
	{"synced", "Mobile bookmarks"},
}

func chromiumBookmarks(dir string) ([]Bookmark, error) {
	b, err := os.ReadFile(filepath.Join(dir, "Bookmarks"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f struct {
		Roots map[string]json.RawMessage `json:"roots"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	var out []Bookmark
	for _, r := range chromiumRoots {
		raw, ok := f.Roots[r.key]
		if !ok {
			continue
		}
		var n chromiumNode
		if json.Unmarshal(raw, &n) != nil {
			continue
		}
		root := chromiumBookmark(n)
		root.Title = cmp.Or(n.Name, r.name)
		out = append(out, root)
	}
	return out, nil
}

func chromiumBookmark(n chromiumNode) Bookmark {
	b := Bookmark{Title: n.Name, Added: webkitTime(n.DateAdded)}
	if n.Type == "url" {
		b.URL = n.URL
		return b
	}
	for _, c := range n.Children {
		if c.Type == "url" && c.URL == "" {
			continue
		}
		b.Children = append(b.Children, chromiumBookmark(c))
	}
	return b
}

const chromiumPagesWhere = `FROM urls WHERE hidden = 0 AND visit_count > 0 AND (url LIKE 'http://%' OR url LIKE 'https://%')`

func chromiumHistory(dir string) ([]Page, error) {
	path := filepath.Join(dir, "History")
	if !exists(path) {
		return nil, nil
	}
	var out []Page
	err := query(path, `SELECT url, title, visit_count, last_visit_time `+chromiumPagesWhere, func(r row) {
		out = append(out, Page{URL: r.text(0), Title: r.text(1), Visits: int(r.int(2)), Last: webkitTime(strconv.FormatInt(r.int(3), 10))})
	})
	return out, err
}

func chromiumPages(dir string) (int, error) {
	path := filepath.Join(dir, "History")
	if !exists(path) {
		return 0, nil
	}
	n := 0
	err := query(path, `SELECT COUNT(*) `+chromiumPagesWhere, func(r row) { n = int(r.int(0)) })
	return n, err
}

// webkitTime is a time Chromium keeps: microseconds since 1601, as text or a number.
func webkitTime(s string) time.Time {
	us, err := strconv.ParseInt(s, 10, 64)
	if err != nil || us <= 0 {
		return time.Time{}
	}
	return time.UnixMicro(us - 11644473600*1_000_000).UTC()
}
