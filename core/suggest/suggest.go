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
	maxTabs    = 3
	maxHistory = 5
	maxActions = 3
	maxRows    = 10
	// With nothing typed: how many rows in all, and how many of them the sites visited most
	// keep when there are bookmarks too.
	maxFirst   = 8
	firstSites = 5
	// legacy is how many a shell that names no kinds gets: history alone, as it was.
	legacy = 6
)

// Row is one suggestion: its kind, a title and a second line, and what choosing it needs:
// an address, a tab or an action.
type Row struct {
	Kind   string `json:"kind"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	URL    string `json:"url,omitempty"`
	Tab    int64  `json:"tab,omitempty"`
	Action string `json:"action,omitempty"`
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
	// First, what Enter does: the address when the text reads as one, the search otherwise.
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

	shown := map[string]bool{}
	if want[KindTab] && m.src.Tabs != nil {
		n := 0
		for _, t := range m.src.Tabs() {
			if n == maxTabs {
				break
			}
			if t.URL == "" || !strings.Contains(strings.ToLower(t.Title), q) && !strings.Contains(strings.ToLower(t.URL), q) {
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
