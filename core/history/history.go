// Package history owns browsing history, in the profile's store (docs/store.md): one row per
// address, one per visit made here, one per site, and the addresses typed. Every read and
// write runs on the store's worker, in the order asked, and a read's answer is posted back to
// the main thread.
package history

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"projectb/store"
	"projectb/timeline"
)

// Entry is one visited URL.
type Entry struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Visits    int    `json:"visits"`
	LastVisit int64  `json:"lastVisit"`
}

// Manager owns the history.
type Manager struct {
	st   *store.Store
	post func(func())
	// clearedAll runs in the transaction that clears all history (OnClearAll).
	clearedAll func(tx *store.Tx, collections ...string) error
}

// New returns the history of the profile folder dir, kept in st. post runs a function back on
// the main thread. The first start with the store reads the history.jsonl an earlier version
// kept, once, on the worker.
func New(dir string, st *store.Store, post func(func())) *Manager {
	st.Schema("history", func(tx *store.Tx) error { return made(tx, filepath.Join(dir, "history.jsonl")) })
	return &Manager{st: st, post: post}
}

// made is the first step of the schema: the tables, and what history.jsonl held.
func made(tx *store.Tx, old string) error {
	if _, err := tx.Exec(tables); err != nil {
		return err
	}
	pages := readJSONL(old)
	for _, e := range pages {
		if _, err := tx.Exec(`INSERT INTO history (id, url, title, visits, last_visit, edited, low_url, low_title) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			store.NewID(), e.URL, e.Title, e.Visits, e.LastVisit, store.Now(), strings.ToLower(e.URL), strings.ToLower(e.Title)); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(filled); err != nil {
		return err
	}
	if len(pages) > 0 {
		timeline.Infof("history: %d pages read once from %s", len(pages), old)
	}
	_, err := tx.Exec(triggers)
	return err
}

// AddVisit records a visit. Only http and https pages count.
func (m *Manager) AddVisit(url, title string) { m.visit(url, title, false) }

// AddTypedVisit records a visit to an address reached from the address field: typed and
// entered, or a suggestion chosen after typing. Suggestions list it first.
func (m *Manager) AddTypedVisit(url, title string) { m.visit(url, title, true) }

func (m *Manager) visit(url, title string, typed bool) {
	if !web(url) {
		return
	}
	now, edited, id := time.Now().Unix(), store.Now(), store.NewID()
	m.st.Do(func(tx *store.Tx) error {
		if _, err := tx.Exec(`INSERT INTO history (id, url, title, visits, last_visit, edited, low_url, low_title) VALUES (?1, ?2, ?3, 1, ?4, ?5, ?6, ?7)
			ON CONFLICT (url) DO UPDATE SET visits = visits + 1, last_visit = excluded.last_visit, edited = excluded.edited,
			title = iif(excluded.title <> '', excluded.title, title), low_title = iif(excluded.title <> '', excluded.low_title, low_title)`,
			id, url, title, now, edited, strings.ToLower(url), strings.ToLower(title)); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO visits (page, time) SELECT page, ?2 FROM history WHERE url = ?1`, url, now); err != nil {
			return err
		}
		if !typed {
			return nil
		}
		_, err := tx.Exec(`INSERT INTO typed (url, id, count, last, edited) VALUES (?1, ?2, 1, ?3, ?4)
			ON CONFLICT (url) DO UPDATE SET count = count + 1, last = excluded.last, edited = excluded.edited`, url, store.NewID(), now, edited)
		return err
	})
}

// UpdateTitle sets the title of a URL already visited. Titles usually arrive after
// the visit.
func (m *Manager) UpdateTitle(url, title string) {
	edited := store.Now()
	m.st.Do(func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE history SET title = ?2, low_title = ?3, edited = ?4 WHERE url = ?1 AND title <> ?2`, url, title, strings.ToLower(title), edited)
		return err
	})
}

// Search answers with the best matches for text, best first, at most limit: the typed
// addresses it starts, then history's (search.go).
func (m *Manager) Search(text string, limit int, answer func([]Entry)) {
	q := strings.ToLower(strings.TrimSpace(text))
	m.read(answer, func(tx *store.Tx) ([]Entry, error) {
		if q == "" || limit <= 0 {
			return nil, nil
		}
		return search(tx, q, limit)
	})
}

// TopSites answers with a page for each of the sites visited most, most first: a site's
// visits are its pages' added up, and its page is the one of them visited most, then latest.
// A site is a host without www.
func (m *Manager) TopSites(limit int, answer func([]Entry)) {
	m.read(answer, func(tx *store.Tx) ([]Entry, error) {
		return entries(tx.Query(`SELECT h.url, h.title, h.visits, h.last_visit FROM sites s INDEXED BY sites_top JOIN history h ON h.url = s.url
			ORDER BY s.visits DESC, s.page_last DESC, s.url LIMIT ?`, limit))
	})
}

// List answers with what the history page shows: the pages visited, the latest first, those
// whose address or title holds text when it isn't "", visited before the Unix time before
// when it isn't 0, at most limit of them.
func (m *Manager) List(text string, before int64, limit int, answer func([]Entry)) {
	q := strings.ToLower(strings.TrimSpace(text))
	where, args := "1", []any{}
	if before != 0 {
		where, args = "last_visit < ?", append(args, before)
	}
	if q != "" {
		where += " AND (instr(low_url, ?) > 0 OR instr(low_title, ?) > 0)"
		args = append(args, q, q)
	}
	m.read(answer, func(tx *store.Tx) ([]Entry, error) {
		return entries(tx.Query(`SELECT url, title, visits, last_visit FROM history INDEXED BY history_last WHERE `+where+
			` ORDER BY last_visit DESC, url LIMIT ?`, append(args, limit)...))
	})
}

// Delete takes the addresses out of the history: their visits, their sites' counts and their
// typed rows go with them.
func (m *Manager) Delete(urls ...string) {
	m.st.Do(func(tx *store.Tx) error {
		for _, u := range urls {
			if _, err := tx.Exec(`DELETE FROM history WHERE url = ?`, u); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteSite removes every page of the site at url, its host with or without www., as
// TopSites counts a site.
func (m *Manager) DeleteSite(url string) {
	host, _, _ := strings.Cut(bareURL(strings.ToLower(url)), "/")
	if host == "" {
		return
	}
	m.st.Do(func(tx *store.Tx) error {
		_, err := tx.Exec(`DELETE FROM history WHERE bare = ?1 OR bare >= ?1 || '/' AND bare < ?1 || '0'`, host)
		return err
	})
}

// Count answers with how many pages were last visited at or after the Unix time since, the
// pages Clear would take out.
func (m *Manager) Count(since int64, answer func(int)) {
	m.st.Do(func(tx *store.Tx) error {
		n := 0
		err := tx.QueryRow(`SELECT count(*) FROM history WHERE last_visit >= ?`, since).Scan(&n)
		m.post(func() { answer(n) })
		return err
	})
}

// Clear takes out every page last visited at or after the Unix time since, or every page
// for 0, as Delete does.
func (m *Manager) Clear(since int64) {
	cleared := m.clearedAll
	m.st.Do(func(tx *store.Tx) error {
		if _, err := tx.Exec(`DELETE FROM history WHERE last_visit >= ?`, since); err != nil || since != 0 || cleared == nil {
			return err
		}
		return cleared(tx, Collection, TypedCollection)
	})
}

// OnClearAll has f run in the transaction that clears all history, with the collections it
// emptied: sync sends one delete of everything for each, not one per page (docs/sync.md).
func (m *Manager) OnClearAll(f func(tx *store.Tx, collections ...string) error) { m.clearedAll = f }

// Import adds pages another browser visited, with their titles, visits and last visit in
// Unix seconds. One here already keeps the most visits of the two, so an import made again
// counts nothing twice, the later visit, and a title. Their visits have no rows of their
// own: when they were is not known.
func (m *Manager) Import(pages []Entry) {
	edited := store.Now()
	m.st.Do(func(tx *store.Tx) error {
		for _, p := range pages {
			if !web(p.URL) {
				continue
			}
			if _, err := tx.Exec(`INSERT INTO history (id, url, title, visits, last_visit, edited, low_url, low_title) VALUES (?1, ?2, ?3, max(?4, 1), ?5, ?6, ?7, ?8)
				ON CONFLICT (url) DO UPDATE SET visits = max(visits, excluded.visits), last_visit = max(last_visit, excluded.last_visit), edited = excluded.edited,
				title = iif(title = '', excluded.title, title), low_title = iif(title = '', excluded.low_title, low_title)
				WHERE excluded.visits > visits OR excluded.last_visit > last_visit OR title = '' AND excluded.title <> ''`,
				store.NewID(), p.URL, p.Title, p.Visits, p.LastVisit, edited, strings.ToLower(p.URL), strings.ToLower(p.Title)); err != nil {
				return err
			}
		}
		return nil
	})
}

// read runs a read on the worker and posts its answer back, nothing when it failed.
func (m *Manager) read(answer func([]Entry), q func(*store.Tx) ([]Entry, error)) {
	m.st.Do(func(tx *store.Tx) error {
		out, err := q(tx)
		if err != nil {
			out = []Entry{}
		}
		m.post(func() { answer(out) })
		return err
	})
}

// entries reads url, title, visits and last_visit from each row.
func entries(r *sql.Rows, err error) ([]Entry, error) {
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out := []Entry{}
	for r.Next() {
		var e Entry
		if err := r.Scan(&e.URL, &e.Title, &e.Visits, &e.LastVisit); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, r.Err()
}

func web(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// bareURL is a lowercased address without its scheme and www., as the table's bare is.
func bareURL(u string) string {
	if s, ok := strings.CutPrefix(u, "https://"); ok {
		u = s
	} else {
		u = strings.TrimPrefix(u, "http://")
	}
	return strings.TrimPrefix(u, "www.")
}

// readJSONL reads the history.jsonl of a version before the store: each line a page, the
// last line for a URL winning, and one marked removed taking it out.
func readJSONL(path string) []Entry {
	f, err := os.Open(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			timeline.Errorf("history: %v", err)
		}
		return nil
	}
	defer f.Close()
	at := map[string]int{}
	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var l struct {
			Entry
			Removed bool `json:"removed"`
		}
		if json.Unmarshal(sc.Bytes(), &l) != nil || l.URL == "" {
			continue
		}
		i, ok := at[l.URL]
		switch {
		case l.Removed && ok:
			out[i].URL = ""
			delete(at, l.URL)
		case l.Removed:
		case ok:
			out[i] = l.Entry
		default:
			at[l.URL] = len(out)
			out = append(out, l.Entry)
		}
	}
	kept := out[:0]
	for _, e := range out {
		if e.URL != "" {
			kept = append(kept, e)
		}
	}
	return kept
}
