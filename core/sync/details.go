package sync

import (
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/sudiptadeb/synckosh"

	"projectb/store"
)

// What Settings' Sync details shows (docs/sync-details.md). Nothing is stored for it: the
// counts and the queue are read from synckosh's bookkeeping when the page asks, and the runs
// are kept in memory, since the app started.

const (
	runsKept     = 100 // rows of runs
	namesKept    = 20  // names a run keeps each way, and the queue shows
	requestsKept = 20  // requests a run keeps
)

// Why a run started.
const (
	WhyAsked      = "asked"      // Sync now, Retry, Try again
	WhyChange     = "change"     // a change here
	WhyTimer      = "timer"      // every five minutes
	WhyStart      = "start"      // the app starting
	WhyOn         = "on"         // turning sync on
	WhyPassphrase = "passphrase" // changing the sync passphrase
)

// Moved is one item a run received or sent, or one waiting to go out, as the feature names
// it: a bookmark's title, an address. A deleted one has no name left, and All is everything
// of its collection, cleared.
type Moved struct {
	Collection string `json:"collection"`
	Name       string `json:"name"`
	Deleted    bool   `json:"deleted,omitempty"`
	All        bool   `json:"all,omitempty"`
	// Since is when it began to wait, Unix milliseconds, 0 when sync hasn't looked at it yet.
	Since int64 `json:"since,omitempty"`
}

// Request is one request of a run, by the last part of its path: the address before it may
// carry the token.
type Request struct {
	At     int64  `json:"at"` // Unix milliseconds
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"` // 0 for no answer
	Out    int64  `json:"out"`    // bytes sent
	In     int64  `json:"in"`     // bytes received
	Took   int64  `json:"took"`   // milliseconds
}

// Run is a row of the page's runs: one run, or several that sync started itself, one after
// another, that moved nothing and ended alike.
type Run struct {
	Began int64  `json:"began"` // when the first of them began, Unix milliseconds
	Last  int64  `json:"last"`  // when the last of them began
	Times int    `json:"times"`
	Why   string `json:"why"`
	Took  int64  `json:"took"` // the last one's, milliseconds
	// Error is the last one's error in words (Words), Problem its name (Problem), and Cut says
	// a person cut it short.
	Error   string `json:"error,omitempty"`
	Problem string `json:"problem,omitempty"`
	Cut     bool   `json:"cut,omitempty"`
	// Received and Sent count what it moved, by collection; the items are the first of them,
	// namesKept at most each way.
	Received      map[string]int `json:"received"`
	Sent          map[string]int `json:"sent"`
	ReceivedItems []Moved        `json:"receivedItems"`
	SentItems     []Moved        `json:"sentItems"`
	// The last one's requests, requestsKept at most, with how many there were and their bytes.
	Requests     []Request `json:"requests"`
	RequestCount int       `json:"requestCount"`
	Out          int64     `json:"out"`
	In           int64     `json:"in"`
	// Asking is the request out now with no answer yet, of the run going on: nil between
	// requests, while sync works on what arrived, and for a run that is over.
	Asking *Request `json:"asking,omitempty"`
}

func (r *Run) moved() bool { return len(r.Received) > 0 || len(r.Sent) > 0 }

// Kind is one collection's counts.
type Kind struct {
	Collection string `json:"collection"`
	Items      int    `json:"items"`
	Deletions  int    `json:"deletions"` // deleted items kept, so other devices delete them too
	Waiting    int    `json:"waiting"`
}

// Details is what the page shows beside the status.
type Details struct {
	// Server is the bucket's address with its token left out: Rest is the path, "/sync/•••"
	// for an address that carries its token.
	Scheme string `json:"scheme"`
	Host   string `json:"host"`
	Rest   string `json:"rest"`
	// Next is when the next run comes if nothing changes, Unix milliseconds, 0 for unknown.
	Next int64 `json:"next"`
	// Cursor is the sequence number of the last item this device has from the server.
	Cursor uint64 `json:"cursor"`
	Kinds  []Kind `json:"kinds"`
	// Waiting is the queue's first namesKept, the longest waiting first, and WaitingMore how
	// many more there are.
	Waiting     []Moved `json:"waiting"`
	WaitingMore int     `json:"waitingMore"`
	// Running is the run going on, nil between runs; Runs the ones ended, newest first.
	Running *Run  `json:"running"`
	Runs    []Run `json:"runs"`
}

// tally is what a run has moved so far, kept by the Local as synckosh hands items over.
type tally struct {
	received, sent           map[string]int
	receivedItems, sentItems []Moved
}

func (t *tally) add(sent bool, m Moved) {
	counts, items := &t.received, &t.receivedItems
	if sent {
		counts, items = &t.sent, &t.sentItems
	}
	if *counts == nil {
		*counts = map[string]int{}
	}
	(*counts)[m.Collection]++
	if len(*items) < namesKept {
		*items = append(*items, m)
	}
}

// name is what the feature calls the row of key: its Name, or the key itself.
func name(tx *store.Tx, m *Mapping, key string) string {
	if m.Name != nil {
		return m.Name(tx, key)
	}
	return key
}

// started notes a run beginning, for why.
func (m *Manager) started(why string) {
	now := time.Now().UnixMilli()
	m.mu.Lock()
	m.cur = &Run{Began: now, Last: now, Times: 1, Why: why}
	m.mu.Unlock()
}

// noteAsking notes a request of the run going on as out, with no answer yet.
func (m *Manager) noteAsking(r Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cur != nil {
		m.cur.Asking = &r
	}
}

// noteRequest adds a request to the run going on, over now. Requests outside a run, as
// Connect's look at the bucket, are none of the page's.
func (m *Manager) noteRequest(r Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cur == nil {
		return
	}
	m.cur.Asking = nil
	m.cur.RequestCount++
	m.cur.Out += r.Out
	m.cur.In += r.In
	if len(m.cur.Requests) < requestsKept {
		m.cur.Requests = append(m.cur.Requests, r)
	}
}

// ended notes the run going on as over, with what the Local counted: a row of its own, or one
// more of the newest row's when sync started both itself and it moved nothing and ended as
// that one did. A run a person asked for always has its own row, so a press shows its result.
func (m *Manager) ended(t tally, err error, cut bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.cur
	m.cur = nil
	if r == nil {
		return
	}
	r.Took, r.Asking = time.Now().UnixMilli()-r.Began, nil
	r.Received, r.Sent, r.ReceivedItems, r.SentItems = t.received, t.sent, t.receivedItems, t.sentItems
	if cut {
		r.Cut = true
	} else if err != nil {
		r.Error, r.Problem = Words(err), Problem(err)
	}
	if len(m.runs) > 0 {
		own := r.Why == WhyTimer || r.Why == WhyChange
		if was := &m.runs[0]; own && !r.moved() && !was.moved() && was.Why == r.Why && was.Error == r.Error && was.Cut == r.Cut {
			r.Began, r.Times = was.Began, was.Times+1
			*was = *r
			return
		}
	}
	m.runs = append([]Run{*r}, m.runs...)
	if len(m.runs) > runsKept {
		m.runs = m.runs[:runsKept]
	}
}

// Details reads what the page shows and answers on the main thread. It never waits for a run:
// it reads the store beside one.
func (m *Manager) Details(answer func(Details)) {
	go func() {
		d := m.details()
		m.post(func() { answer(d) })
	}()
}

func (m *Manager) details() Details {
	d := Details{Kinds: []Kind{}, Waiting: []Moved{}, Runs: []Run{}}
	m.mu.Lock()
	address := m.address
	if m.cur != nil {
		r := *m.cur
		r.Requests = append([]Request(nil), r.Requests...)
		d.Running = &r
	}
	d.Runs = append(d.Runs, m.runs...)
	if !m.nextRun.IsZero() {
		d.Next = m.nextRun.UnixMilli()
	}
	m.mu.Unlock()
	if address == "" {
		return d
	}
	if u, err := url.Parse(address); err == nil {
		d.Scheme, d.Host, d.Rest = u.Scheme+"://", u.Host, u.EscapedPath()
		// An address that goes on past /sync carries its token as its last part (Propose).
		if p, err := Propose(address); err == nil && !p.NeedsToken {
			if i := strings.LastIndex(d.Rest, "/"); i >= 0 {
				d.Rest = d.Rest[:i+1] + "•••"
			}
		}
	}
	collections := make([]string, 0, len(m.maps))
	for c := range m.maps {
		collections = append(collections, c)
	}
	sort.Strings(collections)
	m.st.Wait(func(tx *store.Tx) error {
		var cursor []byte
		if tx.QueryRow(`SELECT value FROM sync_meta WHERE name = 'after'`).Scan(&cursor) == nil {
			json.Unmarshal(cursor, &d.Cursor)
		}
		type wait struct {
			guid, key string
			w         Moved
		}
		var queue []wait
		for _, c := range collections {
			k := Kind{Collection: c}
			if err := tx.QueryRow(`SELECT COALESCE(SUM(NOT deleted), 0), COALESCE(SUM(deleted), 0) FROM sync_items WHERE collection = ? AND guid <> ?`,
				c, synckosh.DeleteBeforeItem).Scan(&k.Items, &k.Deletions); err != nil {
				return err
			}
			// What waits: the items synckosh holds to push, then the rows marked since, which it
			// hasn't seen yet. A row in both waits once.
			seen := map[string]bool{}
			r, err := tx.Query(`SELECT guid, COALESCE(key, ''), deleted, timestamp FROM sync_items WHERE collection = ? AND pending ORDER BY timestamp`, c)
			if err != nil {
				return err
			}
			var waits []wait
			for r.Next() {
				var guid, key string
				w := Moved{Collection: c}
				if err := r.Scan(&guid, &key, &w.Deleted, &w.Since); err != nil {
					r.Close()
					return err
				}
				waits = append(waits, wait{guid, key, w})
			}
			r.Close()
			r, err = tx.Query(`SELECT key, deleted FROM sync_changes WHERE collection = ? ORDER BY rowid`, c)
			if err != nil {
				return err
			}
			for r.Next() {
				var key string
				w := Moved{Collection: c}
				if err := r.Scan(&key, &w.Deleted); err != nil {
					r.Close()
					return err
				}
				waits = append(waits, wait{"", key, w})
			}
			r.Close()
			for _, e := range waits {
				w := e.w
				id := e.key
				switch {
				case e.guid == synckosh.DeleteBeforeItem || e.guid == "" && e.key == "":
					w.All, w.Deleted, id = true, true, "\x00all"
				case e.key == "":
					id = "\x00" + e.guid
				}
				if seen[id] {
					continue
				}
				seen[id] = true
				k.Waiting++
				queue = append(queue, wait{e.guid, e.key, w})
			}
			d.Kinds = append(d.Kinds, k)
		}
		// The longest waiting first; what sync hasn't looked at yet has no time, and goes last.
		sort.SliceStable(queue, func(i, j int) bool {
			a, b := queue[i].w.Since, queue[j].w.Since
			return a != 0 && (b == 0 || a < b)
		})
		if len(queue) > namesKept {
			d.WaitingMore = len(queue) - namesKept
			queue = queue[:namesKept]
		}
		for _, e := range queue {
			if !e.w.Deleted {
				e.w.Name = name(tx, m.maps[e.w.Collection], e.key)
			}
			d.Waiting = append(d.Waiting, e.w)
		}
		return nil
	})
	return d
}
