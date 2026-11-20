package bookmarks

import (
	"database/sql"
	"encoding/json"
	"errors"

	"projectb/store"
	"projectb/sync"
)

// Collection is the collection the bookmarks sync as (docs/sync.md).
const Collection = "bookmarks"

// item is an entry as it syncs. Its id is the bookmark's own, the one its children name as
// their parent: it says which row an item is, and an item's guid says nothing of it.
type item struct {
	ID       string `json:"id"`
	Parent   string `json:"parent"`
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Added    int64  `json:"added"`
	Modified int64  `json:"modified"`
	Place    string `json:"place"`
}

// Synced is how the bookmarks sync: an item per entry, the one changed last winning. A run that changed them has the tree read again, and the subscribers told; post
// runs a function back on the main thread.
func (m *Manager) Synced(post func(func())) sync.Mapping {
	return sync.Mapping{
		Collection: Collection, Datatype: "bookmark", Table: "bookmarks", Key: "id",
		KeyOf: func(private json.RawMessage) (string, error) {
			var it item
			if json.Unmarshal(private, &it) != nil || it.ID == "" {
				return "", errors.New("bookmarks: an item with no id")
			}
			return it.ID, nil
		},
		Name: func(tx *store.Tx, id string) string {
			var title, url string
			tx.QueryRow(`SELECT title, url FROM bookmarks WHERE id = ?`, id).Scan(&title, &url)
			if title == "" {
				return url
			}
			return title
		},
		Read: func(tx *store.Tx, id string) (json.RawMessage, bool, error) {
			it := item{ID: id}
			err := tx.QueryRow(`SELECT parent, kind, title, url, added, edited, place FROM bookmarks WHERE id = ?`, id).
				Scan(&it.Parent, &it.Kind, &it.Title, &it.URL, &it.Added, &it.Modified, &it.Place)
			if err == sql.ErrNoRows {
				return nil, false, nil
			}
			if err != nil {
				return nil, false, err
			}
			b, err := json.Marshal(it)
			return b, err == nil, err
		},
		Write: func(tx *store.Tx, id string, private json.RawMessage) error {
			var it item
			if json.Unmarshal(private, &it) != nil || it.Kind != KindBookmark && it.Kind != KindFolder {
				return nil
			}
			return put(tx, Entry{ID: id, Parent: it.Parent, Kind: it.Kind, Title: it.Title, URL: it.URL, Added: it.Added, Modified: it.Modified, Place: it.Place})
		},
		Delete: func(tx *store.Tx, id string) error {
			_, err := tx.Exec(`DELETE FROM bookmarks WHERE id = ?`, id)
			return err
		},
		Join: func(row, other json.RawMessage) json.RawMessage {
			var a, b item
			if json.Unmarshal(row, &a) != nil {
				return other
			}
			if json.Unmarshal(other, &b) != nil || a.Modified > b.Modified || a.Modified == b.Modified && string(row) >= string(other) {
				return row
			}
			return other
		},
		Changed: func() { m.reload(post) },
	}
}

// reload reads the tree again, after a sync run changed its rows. A change made here while
// it is read has it read once more, so the tree is never older than the rows.
func (m *Manager) reload(post func(func())) {
	edits := m.edits
	m.st.Do(func(tx *store.Tx) error {
		es, err := entries(tx)
		post(func() {
			switch {
			case err != nil:
			case m.edits != edits:
				m.reload(post)
			default:
				m.entries = es
				m.changed()
			}
		})
		return err
	})
}
