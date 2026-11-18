package history

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"projectb/store"
	"projectb/sync"
)

// The collections history syncs as (docs/sync.md): a page per address, and the addresses
// typed. Visits never sync.
const (
	Collection      = "history"
	TypedCollection = "typed"
)

// page is a history row as it syncs.
type page struct {
	URL    string `json:"url"`
	Title  string `json:"title"`
	Visits int    `json:"visits"`
	Last   int64  `json:"last"`
}

// typedRow is a typed row as it syncs.
type typedRow struct {
	URL   string `json:"url"`
	Count int    `json:"count"`
	Last  int64  `json:"last"`
}

// Synced is how history's pages and typed addresses sync: an item per address, its guid made
// from the address, the larger count and the later time winning, with the title of the version
// visited later.
func Synced() []sync.Mapping {
	return []sync.Mapping{{
		Collection: Collection, Datatype: "page", Table: "history", Key: "url", Derived: true, Merges: true,
		KeyOf: urlOf,
		Read: func(tx *store.Tx, url string) (json.RawMessage, bool, error) {
			p := page{URL: url}
			err := tx.QueryRow(`SELECT title, visits, last_visit FROM history WHERE url = ?`, url).Scan(&p.Title, &p.Visits, &p.Last)
			return found(p, err)
		},
		Write: func(tx *store.Tx, url string, private json.RawMessage) error {
			var p page
			if json.Unmarshal(private, &p) != nil || !web(url) {
				return nil
			}
			_, err := tx.Exec(`INSERT INTO history (id, url, title, visits, last_visit, edited, low_url, low_title) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (url) DO UPDATE SET title = excluded.title, low_title = excluded.low_title, visits = excluded.visits,
				last_visit = excluded.last_visit, edited = excluded.edited`,
				store.NewID(), url, p.Title, p.Visits, p.Last, store.Now(), strings.ToLower(url), strings.ToLower(p.Title))
			return err
		},
		Delete: func(tx *store.Tx, url string) error {
			_, err := tx.Exec(`DELETE FROM history WHERE url = ?`, url)
			return err
		},
		Join: joined(func(a, b page) page {
			a.Title = laterTitle(a, b)
			a.Visits, a.Last = max(a.Visits, b.Visits), max(a.Last, b.Last)
			return a
		}),
	}, {
		Collection: TypedCollection, Datatype: "typed", Table: "typed", Key: "url", Derived: true, Merges: true,
		KeyOf: urlOf,
		Read: func(tx *store.Tx, url string) (json.RawMessage, bool, error) {
			t := typedRow{URL: url}
			err := tx.QueryRow(`SELECT count, last FROM typed WHERE url = ?`, url).Scan(&t.Count, &t.Last)
			return found(t, err)
		},
		Write: func(tx *store.Tx, url string, private json.RawMessage) error {
			var t typedRow
			if json.Unmarshal(private, &t) != nil || !web(url) {
				return nil
			}
			_, err := tx.Exec(`INSERT INTO typed (url, id, count, last, edited) VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (url) DO UPDATE SET count = excluded.count, last = excluded.last, edited = excluded.edited`,
				url, store.NewID(), t.Count, t.Last, store.Now())
			return err
		},
		Delete: func(tx *store.Tx, url string) error {
			_, err := tx.Exec(`DELETE FROM typed WHERE url = ?`, url)
			return err
		},
		Join: joined(func(a, b typedRow) typedRow {
			a.Count, a.Last = max(a.Count, b.Count), max(a.Last, b.Last)
			return a
		}),
	}}
}

// laterTitle is the title of the version visited later: a title over none, then the later
// visit, then the greater title, so every device picks the same.
func laterTitle(a, b page) string {
	switch {
	case (a.Title == "") != (b.Title == ""):
		return a.Title + b.Title
	case a.Last > b.Last:
		return a.Title
	case b.Last > a.Last:
		return b.Title
	}
	return max(a.Title, b.Title)
}

// urlOf is the address an item is of.
func urlOf(_ string, private json.RawMessage) (string, error) {
	var v struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(private, &v) != nil || v.URL == "" {
		return "", errors.New("history: an item with no address")
	}
	return v.URL, nil
}

func found[T any](v T, err error) (json.RawMessage, bool, error) {
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	b, err := json.Marshal(v)
	return b, err == nil, err
}

// joined makes a Join of a function of two rows.
func joined[T any](join func(a, b T) T) func(row, other json.RawMessage) json.RawMessage {
	return func(row, other json.RawMessage) json.RawMessage {
		var a, b T
		if json.Unmarshal(row, &a) != nil {
			return other
		}
		if json.Unmarshal(other, &b) != nil {
			return row
		}
		out, _ := json.Marshal(join(a, b))
		return out
	}
}
