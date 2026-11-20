// Package suggest answers what a field that takes an address or a search should suggest:
// one ranked list for the address field, a Command Bar and a new tab's command box, each
// row tagged with where it comes from (docs/daily-driver.md). It owns no data: history,
// the open tabs, the actions and the search engine are its sources, so none of them
// learns about address bars.
package suggest

import (
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"projectb/engine"

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
	// KindClosed is a tab closed lately, offered with nothing typed: choosing it reopens it.
	KindClosed = "closed"
)

// How many rows a list has (docs/daily-driver.md, What the address field suggests). A
// typed list has at most typed rows, or fewer under three letters, unless the shell says
// how many it has room for; the actions come on top. With nothing typed, how many rows in
// all, how many of them the sites visited most keep when there are bookmarks too, and how
// many closed tabs come first.
const (
	maxTabs    = 2
	maxActions = 2
	typedRows  = 8
	shortRows  = 6
	maxRows    = 10
	// candidates is how many pages history is asked for: more than a list shows, since a
	// page open in a tab is listed as the tab.
	candidates  = 10
	maxFirst    = 8
	firstSites  = 5
	firstClosed = 3
	// short is how many letters text needs to match anywhere: shorter, it matches only where
	// a host or a word of a title starts, as history's search does.
	short = 3
	// typedWeight is what a visit typed in the address field counts, against 1 for a click.
	typedWeight = 3
	// legacy is how many a shell that names no kinds gets: history alone, as it was.
	legacy = 6
)

// Row is one suggestion: its kind, a title and a second line, and what choosing it needs:
// an address, a tab or an action. Inline is set on the first row only, when it completes
// what was typed: the rest of the address after the typed text, for the field to show
// selected after the caret. Typed is set on every row of a list for typed text: choosing an
// address from it counts the address as typed (docs/store.md).
type Row struct {
	Kind   string `json:"kind"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	URL    string `json:"url,omitempty"`
	Tab    int64  `json:"tab,omitempty"`
	Action string `json:"action,omitempty"`
	Inline string `json:"inline,omitempty"`
	Typed  bool   `json:"typed,omitempty"`
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

// Closed is a tab closed lately, as the list shows it.
type Closed struct {
	Title string
	URL   string
}

// Sources are where the rows come from, beside history: every open tab of the profile but
// the one window w shows, as a list never offers the tab it is typed over,
// the actions that can run now, the search engine's name and what it makes of words, the
// bookmarks, the newest first; and for nothing typed, the tabs closed lately, the newest
// first.
type Sources struct {
	Tabs      func(w engine.WindowID) []Tab
	Actions   func() []Action
	Engine    func() (name string, search func(words string) string)
	Bookmarks func() []Bookmark
	Closed    func() []Closed
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

// Query answers, on the main thread, with the list for what has been typed so far in window
// w, with rows of the kinds named, at most most of them besides the actions: 0 is the
// default, typedRows, or shortRows under three letters. With no kinds it is history alone,
// for a shell that draws nothing else. With nothing typed it is what a new tab offers
// (first). History answers on its worker, so the list comes once it has.
func (m *Manager) Query(w engine.WindowID, text string, kinds []string, most int, answer func([]Row)) {
	if len(kinds) == 0 {
		m.hist.Search(text, legacy, func(es []history.Entry) { answer(markTyped(historyRows(es))) })
		return
	}
	want := map[string]bool{}
	for _, k := range kinds {
		want[k] = true
	}
	q := strings.ToLower(strings.TrimSpace(text))
	if q == "" {
		if !want[KindHistory] {
			answer(m.first(want, nil))
			return
		}
		m.hist.TopSites(maxFirst, func(sites []history.Entry) { answer(m.first(want, sites)) })
		return
	}
	if most <= 0 {
		most = typedRows
		if utf8.RuneCountInString(q) < short {
			most = shortRows
		}
	}
	if !want[KindHistory] {
		answer(markTyped(m.rows(w, text, want, most, nil, nil)))
		return
	}
	// The completion looks for the text without its scheme and www., the list for the text
	// itself: one search when they are the same.
	m.hist.Search(text, candidates, func(list []history.Entry) {
		c := completing(text)
		if c == "" || c == q {
			answer(markTyped(m.rows(w, text, want, most, list, list)))
			return
		}
		m.hist.Search(c, candidates, func(found []history.Entry) { answer(markTyped(m.rows(w, text, want, most, list, found))) })
	})
}

// markTyped marks the rows as a list for typed text.
func markTyped(rows []Row) []Row {
	for i := range rows {
		rows[i].Typed = true
	}
	return rows
}

// rows is the list for text, from what history found for it, list, and for its completion,
// found, at most most rows besides the actions. First what Enter does: the completion, else
// the address when the text reads as one, else the search. Then up to two open tabs, then the
// pages from history and the bookmarks by their visits, a typed visit counting typedWeight,
// a bookmark ahead of a page with as many; then up to two actions, and the search last when
// it wasn't first. An address shows once, as the highest of its kinds: an open tab, then a
// bookmark, then history; a page from history that is bookmarked is a bookmark row.
func (m *Manager) rows(w engine.WindowID, text string, want map[string]bool, most int, list, found []history.Entry) []Row {
	q := strings.ToLower(strings.TrimSpace(text))
	match := matcher(q)
	rows := make([]Row, 0, most+maxActions)
	shown := map[string]bool{}
	var open []Tab
	if want[KindTab] && m.src.Tabs != nil {
		open = m.src.Tabs(w)
	}
	var marks []Bookmark
	marked := map[string]bool{}
	if want[KindBookmark] && m.src.Bookmarks != nil {
		marks = m.src.Bookmarks()
		for _, b := range marks {
			marked[b.URL] = true
		}
	}
	completed := int64(0)
	if c, ok := m.completion(text, want, found); ok {
		// A completed page open in a tab is that tab's row: Enter shows the tab rather than
		// loading the page a second time.
		for _, t := range open {
			if t.URL != "" && strings.TrimSuffix(t.URL, "/") == strings.TrimSuffix(c.URL, "/") {
				c = Row{Kind: KindTab, Title: orURL(t.Title, t.URL), Detail: t.Detail, URL: t.URL, Tab: t.ID, Inline: c.Inline}
				completed = t.ID
				break
			}
		}
		if c.Kind == KindHistory && marked[c.URL] {
			c.Kind = KindBookmark
		}
		rows = append(rows, c)
		shown[c.URL] = true
	}
	var search *Row
	if m.src.Engine != nil && want[KindSearch] {
		name, find := m.src.Engine()
		search = &Row{Kind: KindSearch, Title: strings.TrimSpace(text), Detail: "Search " + name, URL: find(text)}
	}
	if tabs.LooksLikeAddress(text) && want[KindAddress] {
		u := tabs.NormalizeInput(text, func(string) string { return "" })
		if !shown[u] {
			rows = append(rows, Row{Kind: KindAddress, Title: strings.TrimSpace(text), Detail: "Go to address", URL: u})
		}
	} else if search != nil && len(rows) == 0 {
		rows = append(rows, *search)
		search = nil
	}
	// room is how many more rows fit, the search's place at the end kept.
	room := func() int {
		if search != nil {
			return most - len(rows) - 1
		}
		return most - len(rows)
	}

	n := 0
	for _, t := range open {
		if n == maxTabs || room() <= 0 {
			break
		}
		if t.URL == "" || t.ID == completed || shown[t.URL] || !match(t.Title, t.URL) {
			continue
		}
		rows = append(rows, Row{Kind: KindTab, Title: orURL(t.Title, t.URL), Detail: t.Detail, URL: t.URL, Tab: t.ID})
		shown[t.URL] = true
		n++
	}

	// The pages and the bookmarks, by their visits.
	type page struct {
		row    Row
		weight int
	}
	var pages []page
	visits := map[string]int{}
	if want[KindHistory] {
		for _, e := range list {
			w := e.Visits + (typedWeight-1)*e.Typed
			visits[e.URL] = w
			if shown[e.URL] {
				continue
			}
			kind := KindHistory
			if marked[e.URL] {
				kind = KindBookmark
			}
			pages = append(pages, page{Row{Kind: kind, Title: orURL(e.Title, e.URL), Detail: host(e.URL), URL: e.URL}, w})
			shown[e.URL] = true
		}
	}
	for _, b := range marks {
		if b.URL == "" || shown[b.URL] || !match(b.Title, b.URL) {
			continue
		}
		pages = append(pages, page{Row{Kind: KindBookmark, Title: orURL(b.Title, b.URL), Detail: host(b.URL), URL: b.URL}, visits[b.URL]})
		shown[b.URL] = true
	}
	slices.SortStableFunc(pages, func(a, b page) int {
		if a.weight != b.weight {
			return b.weight - a.weight
		}
		if (a.row.Kind == KindBookmark) != (b.row.Kind == KindBookmark) {
			if a.row.Kind == KindBookmark {
				return -1
			}
			return 1
		}
		return 0
	})
	for _, p := range pages[:min(len(pages), max(room(), 0))] {
		rows = append(rows, p.row)
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
		n := maxActions
		if only {
			n = maxRows
		}
		rows = append(rows, all[:min(len(all), n)]...)
	}
	if search != nil {
		rows = append(rows, *search)
	}
	return rows
}

// matcher is whether a tab or a bookmark, by its title and address, matches q: under three
// letters only where its host or a word of its title starts with it, from three anywhere.
func matcher(q string) func(title, u string) bool {
	if utf8.RuneCountInString(q) >= short {
		return func(title, u string) bool {
			return strings.Contains(strings.ToLower(title), q) || strings.Contains(strings.ToLower(u), q)
		}
	}
	return func(title, u string) bool {
		t := strings.ToLower(title)
		return strings.HasPrefix(strings.ToLower(bare(u)), q) || strings.HasPrefix(t, q) || strings.Contains(t, " "+q)
	}
}

// first is the list with nothing typed, what a new tab offers, at most maxFirst rows in all
// and each address once: asked for, the tabs closed lately, the newest first, up to
// firstClosed; then the sites visited most, as history rows, and
// the newest bookmarks. Asked for both, the sites keep up to firstSites of the rows left,
// the bookmarks fill the rest, and the sites any rows the bookmarks leave. top is what
// history.TopSites answered.
func (m *Manager) first(want map[string]bool, top []history.Entry) []Row {
	var rows, sites, marks []Row
	shown := map[string]bool{}
	if want[KindClosed] && m.src.Closed != nil {
		for _, c := range m.src.Closed() {
			if len(rows) == firstClosed {
				break
			}
			if c.URL != "" && !shown[c.URL] {
				rows = append(rows, Row{Kind: KindClosed, Title: orURL(c.Title, c.URL), Detail: host(c.URL), URL: c.URL})
				shown[c.URL] = true
			}
		}
	}
	room := maxFirst - len(rows)
	if want[KindHistory] {
		for _, e := range top {
			if !shown[e.URL] {
				sites = append(sites, Row{Kind: KindHistory, Title: orURL(e.Title, e.URL), Detail: host(e.URL), URL: e.URL})
				shown[e.URL] = true
			}
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
	n := min(len(marks), room-min(len(sites), firstSites, room))
	rows = append(rows, sites[:min(len(sites), room-n)]...)
	rows = append(rows, marks[:n]...)
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
// may start with a scheme or www., which the completion keeps. pages are what history
// found for the text completing reads.
func (m *Manager) completion(text string, want map[string]bool, pages []history.Entry) (Row, bool) {
	q := completing(text)
	if q == "" {
		return Row{}, false
	}
	var found []Row
	if want[KindHistory] {
		for _, e := range pages {
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

// completing is what the completion looks for: the text lowercased, without its scheme and
// www., or "" for none, as for text with a space.
func completing(text string) string {
	if text == "" || strings.ContainsAny(text, " \t") {
		return ""
	}
	return strings.ToLower(bare(text))
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
