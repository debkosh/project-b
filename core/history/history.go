// Package history owns browsing history: a map in memory on the main thread, and a
// worker that appends to a file. The worker holds no state and only gets copies.
package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"projectb/timeline"
	"strings"
	"time"
)

// Entry is one visited URL.
type Entry struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Visits    int    `json:"visits"`
	LastVisit int64  `json:"lastVisit"`

	// What a search compares, lowercased once when the entry is stored, not on each key:
	// the address, the address without its scheme and www., and the title.
	lowURL, bare, lowTitle string
}

// index keeps what a search compares in step with the entry's address and title.
func (e *Entry) index() {
	e.lowURL = strings.ToLower(e.URL)
	e.bare = bareURL(e.lowURL)
	e.lowTitle = strings.ToLower(e.Title)
}

// Manager owns the history.
type Manager struct {
	path    string
	post    func(func())
	entries map[string]*Entry
	loaded  bool
	writes  chan Entry
	done    chan struct{}
}

// New returns a manager storing under dir and starts its writer. post runs a function
// back on the main thread.
func New(dir string, post func(func())) *Manager {
	m := &Manager{
		path:    filepath.Join(dir, "history.jsonl"),
		post:    post,
		entries: map[string]*Entry{},
		writes:  make(chan Entry, 1024),
		done:    make(chan struct{}),
	}
	go m.writer()
	return m
}

// Load reads saved history on a worker and calls ready on the main thread when it's in.
// Until then, Search just sees what's been visited this run.
func (m *Manager) Load(ready func()) {
	go func() {
		saved := readAll(m.path)
		m.post(func() {
			for url, e := range saved {
				if cur, ok := m.entries[url]; ok {
					cur.Visits += e.Visits
					if cur.Title == "" {
						cur.Title = e.Title
						cur.index()
					}
				} else {
					e.index()
					m.entries[url] = e
				}
			}
			m.loaded = true
			if ready != nil {
				ready()
			}
		})
	}()
}

// AddVisit records a visit. Only http and https pages count.
func (m *Manager) AddVisit(url, title string) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return
	}
	e, ok := m.entries[url]
	if !ok {
		e = &Entry{URL: url}
		m.entries[url] = e
	}
	e.Visits++
	e.LastVisit = time.Now().Unix()
	if title != "" {
		e.Title = title
	}
	e.index()
	m.save(e)
}

// UpdateTitle sets the title of a URL already visited. Titles usually arrive after
// the visit.
func (m *Manager) UpdateTitle(url, title string) {
	e, ok := m.entries[url]
	if !ok || e.Title == title {
		return
	}
	e.Title = title
	e.index()
	m.save(e)
}

// Search returns the best matches for text, best first. It keeps only the best limit
// as it goes, so a long history costs one pass and no sort of every hit.
func (m *Manager) Search(text string, limit int) []Entry {
	q := strings.ToLower(strings.TrimSpace(text))
	if q == "" || limit <= 0 {
		return nil
	}
	return m.best(limit, func(e *Entry) int { return score(e, q) })
}

// Top returns the pages visited most, most first.
func (m *Manager) Top(limit int) []Entry {
	return m.best(limit, func(e *Entry) int { return e.Visits })
}

// best is the limit entries score rates highest, above 0, best first: by score, then the
// latest visit, then the address.
func (m *Manager) best(limit int, score func(*Entry) int) []Entry {
	type scored struct {
		e     *Entry
		score int
	}
	before := func(a, b scored) bool {
		if a.score != b.score {
			return a.score > b.score
		}
		if a.e.LastVisit != b.e.LastVisit {
			return a.e.LastVisit > b.e.LastVisit
		}
		return a.e.URL < b.e.URL
	}
	hits := make([]scored, 0, limit+1)
	for _, e := range m.entries {
		s := score(e)
		if s <= 0 {
			continue
		}
		h := scored{e, s}
		if len(hits) == limit && !before(h, hits[limit-1]) {
			continue
		}
		// Into its place among the few kept.
		i := len(hits)
		hits = append(hits, h)
		for i > 0 && before(h, hits[i-1]) {
			hits[i] = hits[i-1]
			i--
		}
		hits[i] = h
		if len(hits) > limit {
			hits = hits[:limit]
		}
	}
	out := make([]Entry, len(hits))
	for i, h := range hits {
		out[i] = *h.e
	}
	return out
}

// Close waits for pending writes.
func (m *Manager) Close() {
	close(m.writes)
	<-m.done
}

// score rates an entry against q, already lowercased.
func score(e *Entry, q string) int {
	base := 0
	switch {
	case strings.HasPrefix(e.bare, q):
		base = 1000
	case strings.Contains(e.lowURL, q):
		base = 100
	case strings.Contains(e.lowTitle, q):
		base = 50
	default:
		return 0
	}
	return base + min(e.Visits, 50)*2
}

// bareURL is a lowercased address without its scheme and www.
func bareURL(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.TrimPrefix(u, "www.")
}

func (m *Manager) save(e *Entry) {
	select {
	case m.writes <- *e:
	default:
		timeline.Warnf("history: write queue full, dropped a visit")
	}
}

func (m *Manager) writer() {
	defer close(m.done)
	var w *os.File
	for e := range m.writes {
		if w == nil {
			if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
				timeline.Errorf("history: %v", err)
				continue
			}
			f, err := os.OpenFile(m.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if err != nil {
				timeline.Errorf("history: %v", err)
				continue
			}
			w = f
			defer w.Close()
		}
		line, _ := json.Marshal(e)
		if _, err := w.Write(append(line, '\n')); err != nil {
			timeline.Errorf("history: %v", err)
		}
	}
}

// readAll reads the append-only file. The last line for a URL wins.
func readAll(path string) map[string]*Entry {
	out := map[string]*Entry{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.URL != "" {
			out[e.URL] = &e
		}
	}
	return out
}
