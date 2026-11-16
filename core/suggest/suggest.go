// Package suggest answers what a field that takes an address or a search should suggest:
// one ranked list for the address field, a Command Bar and a new tab's command box, each
// row tagged with where it comes from (docs/daily-driver.md). It owns no data: history,
// the open tabs, the actions and the search engine are its sources, so none of them
// learns about address bars.
package suggest

import (
	"net/url"
	"strings"

	"projectb/history"
	"projectb/tabs"
)

// The kinds of row, which a shell names to say what it draws.
const (
	KindAddress  = "address"
	KindSearch   = "search"
	KindTab      = "tab"
	KindHistory  = "history"
	KindAction   = "action"
	KindBookmark = "bookmark"
)

// How many rows of each kind a list has at most, and how many rows in all.
const (
	maxTabs      = 3
	maxHistory   = 5
	maxBookmarks = 3
	maxActions   = 3
	maxRows      = 10
	// With nothing typed: how many rows in all, and how many of them the sites visited most
	// keep when there are bookmarks too.
	maxFirst   = 8
	firstSites = 5
	// legacy is how many a shell that names no kinds gets: history alone, as it was.
	legacy = 6
)

// Row is one suggestion: its kind, a title and a second line, and what choosing it needs:
// an address, a tab or an action. Inline is set on the first row only, when it completes
// what was typed: the rest of the address after the typed text, for the field to show
// selected after the caret.
type Row struct {
	Kind   string `json:"kind"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	URL    string `json:"url,omitempty"`
	Tab    int64  `json:"tab,omitempty"`
	Action string `json:"action,omitempty"`
	Inline string `json:"inline,omitempty"`
}

// Tab is an open tab as the list shows it: Detail is its second line, its workspace.
type Tab struct {
	ID     int64
	Title  string
	URL    string
	Detail string
}

// Action is an action that can run now: Detail is the key it shows.
type Action struct {
	ID     string
	Title  string
	Detail string
}

// Bookmark is a bookmark as the list shows it.
type Bookmark struct {
	Title string
	URL   string
}

// Sources are where the rows come from, beside history: every open tab of the profile,
// the actions that can run now, the search engine's name and what it makes of words, and
// the bookmarks, the newest first.
type Sources struct {
	Tabs      func() []Tab
	Actions   func() []Action
	Engine    func() (name string, search func(words string) string)
	Bookmarks func() []Bookmark
}

// Manager answers suggestion queries.
type Manager struct {
	hist *history.Manager
	src  Sources
}

// New returns a manager reading from hist.
func New(hist *history.Manager) *Manager { return &Manager{hist: hist} }

// SetSources gives the manager its other sources. Until then it has history alone.
func (m *Manager) SetSources(s Sources) { m.src = s }

// Query returns the list for what has been typed so far, with rows of the kinds named.
// With no kinds it is history alone, for a shell that draws nothing else. With nothing
// typed it is what a new tab offers (first).
func (m *Manager) Query(text string, kinds []string) []Row {
	if len(kinds) == 0 {
		return historyRows(m.hist.Search(text, legacy))
	}
	want := map[string]bool{}
	for _, k := range kinds {
		want[k] = true
	}
	q := strings.ToLower(strings.TrimSpace(text))
	if q == "" {
		return m.first(want)
	}

	rows := make([]Row, 0, maxRows)
	shown := map[string]bool{}
	// First, what Enter does: a visited or bookmarked address the text completes, then the
	// address when the text reads as one, the search otherwise.
	var open []Tab
	if want[KindTab] && m.src.Tabs != nil {
		open = m.src.Tabs()
	}
	completed := int64(0)
	if c, ok := m.completion(text, want); ok {
		// A completed page open in a tab is that tab's row: Enter shows the tab rather than
		// loading the page a second time.
		for _, t := range open {
			if t.URL != "" && strings.TrimSuffix(t.URL, "/") == strings.TrimSuffix(c.URL, "/") {
				c = Row{Kind: KindTab, Title: orURL(t.Title, t.URL), Detail: t.Detail, URL: t.URL, Tab: t.ID, Inline: c.Inline}
				completed = t.ID
				break
			}
		}
		rows = append(rows, c)
		shown[c.URL] = true
	}
	var search *Row
	if m.src.Engine != nil && want[KindSearch] {
		name, find := m.src.Engine()
		search = &Row{Kind: KindSearch, Title: strings.TrimSpace(text), Detail: "Search " + name, URL: find(text)}
	}
	isAddress := tabs.LooksLikeAddress(text)
	if isAddress && want[KindAddress] {
		u := tabs.NormalizeInput(text, func(string) string { return "" })
		rows = append(rows, Row{Kind: KindAddress, Title: strings.TrimSpace(text), Detail: "Go to address", URL: u})
	} else if search != nil {
		rows = append(rows, *search)
		search = nil
	}

	if want[KindTab] {
		n := 0
		for _, t := range open {
			if n == maxTabs {
				break
			}
			if t.URL == "" || t.ID == completed || !strings.Contains(strings.ToLower(t.Title), q) && !strings.Contains(strings.ToLower(t.URL), q) {
				continue
			}
			rows = append(rows, Row{Kind: KindTab, Title: orURL(t.Title, t.URL), Detail: t.Detail, URL: t.URL, Tab: t.ID})
			shown[t.URL] = true
			n++
		}
	}
	if want[KindHistory] {
		n := 0
		// A few more than shown, since a page open in a tab is listed as the tab.
		for _, e := range m.hist.Search(text, maxHistory+maxTabs) {
			if n == maxHistory {
				break
			}
			if shown[e.URL] {
				continue
			}
			rows = append(rows, Row{Kind: KindHistory, Title: orURL(e.Title, e.URL), Detail: host(e.URL), URL: e.URL})
			shown[e.URL] = true
			n++
		}
	}
	// The bookmarks whose title or address holds the text, the newest first, past those
	// already shown as a tab or a page from history.
	if want[KindBookmark] && m.src.Bookmarks != nil {
		n := 0
		for _, b := range m.src.Bookmarks() {
			if n == maxBookmarks {
				break
			}
			if b.URL == "" || shown[b.URL] || !strings.Contains(strings.ToLower(b.Title), q) && !strings.Contains(strings.ToLower(b.URL), q) {
				continue
			}
			rows = append(rows, Row{Kind: KindBookmark, Title: orURL(b.Title, b.URL), Detail: host(b.URL), URL: b.URL})
			shown[b.URL] = true
			n++
		}
	}
	// Asked for alone, as a field's actions-only mode does, the actions have the whole list
	// and one letter is enough.
	only := len(want) == 1
	if want[KindAction] && m.src.Actions != nil && (len(q) >= 2 || only) {
		// One whose title starts with the text comes before one that only holds it.
		var starts, holds []Row
		for _, a := range m.src.Actions() {
			low := strings.ToLower(a.Title)
			row := Row{Kind: KindAction, Title: a.Title, Detail: a.Detail, Action: a.ID}
			if strings.HasPrefix(low, q) {
				starts = append(starts, row)
			} else if strings.Contains(low, q) {
				holds = append(holds, row)
			}
		}
		all := append(starts, holds...)
		most := maxActions
		if only {
			most = maxRows
		}
		rows = append(rows, all[:min(len(all), most)]...)
	}
	if search != nil {
		rows = append(rows, *search)
	}
	return rows[:min(len(rows), maxRows)]
}

// first is the list with nothing typed, what a new tab offers: the sites visited most, as
// history rows, and the newest bookmarks not among them, at most maxFirst in all. Asked for
// both, the sites come first and keep up to firstSites rows, the bookmarks fill the rest,
// and the sites any rows the bookmarks leave.
func (m *Manager) first(want map[string]bool) []Row {
	var sites, marks []Row
	shown := map[string]bool{}
	if want[KindHistory] {
		for _, e := range m.hist.TopSites(maxFirst) {
			sites = append(sites, Row{Kind: KindHistory, Title: orURL(e.Title, e.URL), Detail: host(e.URL), URL: e.URL})
			shown[e.URL] = true
		}
	}
	if want[KindBookmark] && m.src.Bookmarks != nil {
		for _, b := range m.src.Bookmarks() {
			if len(marks) == maxFirst {
				break
			}
			if !shown[b.URL] {
				marks = append(marks, Row{Kind: KindBookmark, Title: orURL(b.Title, b.URL), Detail: host(b.URL), URL: b.URL})
			}
		}
	}
	n := min(len(marks), maxFirst-min(len(sites), firstSites))
	rows := append(sites[:min(len(sites), maxFirst-n)], marks[:n]...)
	if rows == nil {
		return []Row{}
	}
	return rows
}

// completion is the row that completes what was typed, as Chrome and Safari complete it in
// the field: a page from history, or a bookmark, whose bare address (no scheme, no www.)
// starts with the text, which has no space. The page visited most wins, then the newest
// bookmark. It completes to the host alone while the text is still inside the host
// (wikip → wikipedia.org): the row for the host's own page when there is one, else an
// address row for the host. Past the host, or with no address rows asked for, it completes
// to the whole address. Query makes it the tab's row when the page is open in one. The text
// may start with a scheme or www., which the completion keeps.
func (m *Manager) completion(text string, want map[string]bool) (Row, bool) {
	if text == "" || strings.ContainsAny(text, " \t") {
		return Row{}, false
	}
	q := strings.ToLower(bare(text))
	if q == "" {
		return Row{}, false
	}
	var found []Row
	if want[KindHistory] {
		for _, e := range m.hist.Search(q, maxHistory+maxTabs) {
			if web(e.URL) && strings.HasPrefix(strings.ToLower(bare(e.URL)), q) {
				found = append(found, Row{Kind: KindHistory, Title: orURL(e.Title, e.URL), Detail: host(e.URL), URL: e.URL})
			}
		}
	}
	if want[KindBookmark] && m.src.Bookmarks != nil {
		for _, b := range m.src.Bookmarks() {
			if web(b.URL) && strings.HasPrefix(strings.ToLower(bare(b.URL)), q) {
				found = append(found, Row{Kind: KindBookmark, Title: orURL(b.Title, b.URL), Detail: host(b.URL), URL: b.URL})
			}
		}
	}
	if len(found) == 0 {
		return Row{}, false
	}
	row := found[0]
	full := bare(row.URL)
	h := full[:hostEnd(full)]
	if !strings.Contains(q, "/") && strings.HasPrefix(strings.ToLower(h), q) {
		// The host alone: its own page's row when one was found, else an address row for it,
		// in the scheme and www. of the page that was. A shell that draws no address rows
		// gets the page's whole address instead.
		var own *Row
		for i, r := range found {
			if strings.EqualFold(strings.TrimSuffix(bare(r.URL), "/"), h) {
				own = &found[i]
				break
			}
		}
		switch {
		case own != nil:
			row, full = *own, h
		case want[KindAddress]:
			row = Row{Kind: KindAddress, Title: h, Detail: "Go to address", URL: row.URL[:len(row.URL)-len(full)] + h + "/"}
			full = h
		}
	}
	if len(full) <= len(q) || !strings.EqualFold(full[:len(q)], q) {
		return Row{}, false
	}
	row.Inline = full[len(q):]
	return row, true
}

// bare is an address without its scheme and www., in its own case.
func bare(u string) string {
	for _, p := range []string{"https://", "http://"} {
		if len(u) >= len(p) && strings.EqualFold(u[:len(p)], p) {
			u = u[len(p):]
			break
		}
	}
	if len(u) >= 4 && strings.EqualFold(u[:4], "www.") {
		u = u[4:]
	}
	return u
}

// hostEnd is where the host ends in a bare address: at its path, query or fragment.
func hostEnd(b string) int {
	if i := strings.IndexAny(b, "/?#"); i >= 0 {
		return i
	}
	return len(b)
}

// web is whether an address is a web page's, the only ones that complete.
func web(u string) bool {
	l := strings.ToLower(u)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

func historyRows(entries []history.Entry) []Row {
	rows := make([]Row, len(entries))
	for i, e := range entries {
		rows[i] = Row{Kind: KindHistory, Title: e.Title, Detail: host(e.URL), URL: e.URL}
	}
	return rows
}

func orURL(title, u string) string {
	if title != "" {
		return title
	}
	return u
}

// host is an address's host, without www., or the address when it has none.
func host(u string) string {
	p, err := url.Parse(u)
	if err != nil || p.Host == "" {
		return u
	}
	return strings.TrimPrefix(p.Host, "www.")
}
