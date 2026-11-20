package sync

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/sudiptadeb/synckosh"
	"github.com/sudiptadeb/synckosh/client"

	"projectb/store"
)

// Mapping is how a feature's table syncs as one collection (docs/sync.md). The feature keeps
// writing its rows; sync reads an item's private part from its row and writes the row from
// it, so an item's content is never stored twice.
type Mapping struct {
	Collection, Datatype string
	// Table is the feature's table and Key the column that names a row: the marks of changed
	// rows name them by it.
	Table, Key string
	// Derived makes an item's guid from its key with the sync key, so one key on two devices is
	// one item, and the server can't read the key off the guid. Otherwise the key is the guid.
	Derived bool
	// KeyOf is the key of the row an item is, from its guid and private part.
	KeyOf func(guid string, private json.RawMessage) (string, error)
	// Read is the row of key as an item's private part, and whether there is one.
	Read func(tx *store.Tx, key string) (json.RawMessage, bool, error)
	// Write makes the row of key hold private, new or changed. One it can't keep, it leaves.
	Write func(tx *store.Tx, key string, private json.RawMessage) error
	// Delete deletes the row of key.
	Delete func(tx *store.Tx, key string) error
	// Join is a row after taking in another version of it: it never loses what the row holds
	// that is newer, and gives the same whichever of the two is the row.
	Join func(row, other json.RawMessage) json.RawMessage
	// Merges makes Join synckosh's merger for the datatype too, for two versions that never
	// saw each other; without it the newest version wins there.
	Merges bool
	// Changed runs on the main thread after a run wrote the feature's rows.
	Changed func()
	// Name is what a person calls the row of key, for Sync details: a bookmark's title. Without
	// it the key is the name, as an address is.
	Name func(tx *store.Tx, key string) string
}

// The tables (docs/sync.md). sync_items is synckosh's bookkeeping, an item's JSON with its
// private part left out when the feature's row is it; key is that row, for an item of a
// mapped collection. sync_changes is a rowid table, so a row marked again takes a new rowid
// and a run deletes only the marks it read.
const tables = `
CREATE TABLE sync_items (
  collection TEXT NOT NULL,
  guid TEXT NOT NULL,
  key TEXT,
  deleted INTEGER NOT NULL,
  timestamp INTEGER NOT NULL,
  pending INTEGER NOT NULL,
  sent INTEGER NOT NULL,
  locked INTEGER NOT NULL,
  item BLOB NOT NULL,
  PRIMARY KEY (collection, guid)
) WITHOUT ROWID;
CREATE INDEX sync_items_key ON sync_items (collection, key) WHERE key IS NOT NULL;
CREATE INDEX sync_items_pending ON sync_items (collection) WHERE pending;
CREATE TABLE sync_meta (name TEXT PRIMARY KEY, value BLOB NOT NULL) WITHOUT ROWID;
CREATE TABLE sync_changes (collection TEXT NOT NULL, key TEXT NOT NULL, deleted INTEGER NOT NULL, UNIQUE (collection, key));
CREATE TABLE sync_state (marking INTEGER NOT NULL);
INSERT INTO sync_state VALUES (0);
`

// local is synckosh's client.Local over the profile's store. Each call is one job on the
// store's worker, waited for: only the sync goroutine calls it, never the main thread.
type local struct {
	st   *store.Store
	maps map[string]*Mapping
	ids  []byte // the key a derived guid is made with
	// wrote is the collections whose rows a version from the server changed, since the run
	// began. Only the sync goroutine reads it, after the jobs that set it.
	wrote map[string]bool
	// moved is what was received and sent since the run began, for Sync details. As wrote.
	moved tally
}

var _ client.Local = (*local)(nil)

func newLocal(st *store.Store, maps map[string]*Mapping, key *synckosh.DataKey) *local {
	ids := hmac.New(sha256.New, key.Bytes())
	ids.Write([]byte("project-b item ids"))
	return &local{st: st, maps: maps, ids: ids.Sum(nil), wrote: map[string]bool{}}
}

// guid is the guid of the row key of a mapped collection. A key that is the guid itself may
// start with an underscore, as a passkey's id in base64url does one time in 64, which a
// synckosh guid can't: it goes with a ~ before it, as does one that starts with a ~, so key
// reads every guid back one way.
func (l *local) guid(m *Mapping, key string) string {
	if !m.Derived {
		if strings.HasPrefix(key, "_") || strings.HasPrefix(key, "~") {
			return "~" + key
		}
		return key
	}
	h := hmac.New(sha256.New, l.ids)
	h.Write([]byte(m.Collection + "\x00" + key))
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// plain is the feature's own key in a guid that is one (guid): what a Mapping's KeyOf is
// given for a collection whose guid isn't derived.
func plain(m *Mapping, guid string) string {
	if !m.Derived {
		return strings.TrimPrefix(guid, "~")
	}
	return guid
}

func (l *local) do(job func(tx *store.Tx) error) error { return l.st.Wait(job) }

// side is one row of sync_items as read.
type side struct {
	collection string
	key        sql.NullString
	pending    bool
	sent       int64
	locked     bool
	item       []byte
}

const sideColumns = `collection, key, pending, sent, locked, item`

func scanSide(r interface{ Scan(...any) error }) (side, error) {
	var s side
	err := r.Scan(&s.collection, &s.key, &s.pending, &s.sent, &s.locked, &s.item)
	return s, err
}

// entry is a side row as synckosh's entry: its private part read from its row. A row that is
// gone, its delete not yet handed to synckosh, is reported deleted when gone is true, so a
// push or a refill sends a delete, not an empty item.
func (l *local) entry(tx *store.Tx, s side, gone bool) (client.Entry, error) {
	var it synckosh.Item
	if err := json.Unmarshal(s.item, &it); err != nil {
		return client.Entry{}, err
	}
	e := client.Entry{Item: it, Pending: s.pending, Sent: s.sent, Locked: s.locked}
	if m := l.maps[s.collection]; m != nil && s.key.Valid && !it.Deleted {
		private, has, err := m.Read(tx, s.key.String)
		if err != nil {
			return client.Entry{}, err
		}
		e.Item.Private = private
		e.Item.Deleted = !has && gone
	}
	return e, nil
}

func (l *local) Get(collection, guid string) (client.Entry, bool, error) {
	var e client.Entry
	var has bool
	err := l.do(func(tx *store.Tx) error {
		s, err := scanSide(tx.QueryRow(`SELECT `+sideColumns+` FROM sync_items WHERE collection = ? AND guid = ?`, collection, guid))
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		has = true
		e, err = l.entry(tx, s, false)
		return err
	})
	return e, has, err
}

// each reads the side rows a query gives, as entries, then calls fn with each outside the
// job: fn may call back into the store.
func (l *local) each(query string, args []any, fn func(client.Entry) error) error {
	var out []client.Entry
	err := l.do(func(tx *store.Tx) error {
		r, err := tx.Query(`SELECT `+sideColumns+` FROM sync_items `+query, args...)
		if err != nil {
			return err
		}
		var sides []side
		for r.Next() {
			s, err := scanSide(r)
			if err != nil {
				r.Close()
				return err
			}
			sides = append(sides, s)
		}
		r.Close()
		if err := r.Err(); err != nil {
			return err
		}
		for _, s := range sides {
			e, err := l.entry(tx, s, true)
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, e := range out {
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

func (l *local) Each(collection string, fn func(client.Entry) error) error {
	if collection == "" {
		return l.each(``, nil, fn)
	}
	return l.each(`WHERE collection = ?`, []any{collection}, fn)
}

func (l *local) Pending() ([]client.Entry, error) {
	var out []client.Entry
	err := l.each(`WHERE pending`, nil, func(e client.Entry) error {
		out = append(out, e)
		return nil
	})
	return out, err
}

// Put stores entries. A version of a mapped item is joined into its row, with the triggers
// quiet, unless its row waits for a delete to go out; when the row then differs from it, the
// row holds something newer and is marked to go out. A deleted item deletes its row, unless
// the row was changed here since; a delete-before deletes the rows at or before its cutoff.
func (l *local) Put(entries ...client.Entry) error {
	if len(entries) == 0 {
		return nil
	}
	return l.do(func(tx *store.Tx) error {
		return quietly(tx, func() error {
			for _, e := range entries {
				if err := l.put(tx, e); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func (l *local) put(tx *store.Tx, e client.Entry) error {
	it := e.Item
	m := l.maps[it.Collection]
	var key sql.NullString
	switch {
	case synckosh.IsDeleteBefore(it) && !e.Locked && !it.Deleted:
		// A delete-before is the library's own item, and says which collection it clears.
		collection, cutoff, err := synckosh.Cutoff(it)
		if err != nil {
			return err
		}
		if cleared := l.maps[collection]; cleared != nil {
			if err := l.cut(tx, cleared, cutoff); err != nil {
				return err
			}
			// One this device wrote itself is pending: its rows were cleared here already.
			// Once the server has taken it, it comes back neither pending nor sent.
			var pending bool
			var sent int64
			err := tx.QueryRow(`SELECT pending, sent FROM sync_items WHERE collection = ? AND guid = ?`, it.Collection, it.GUID).Scan(&pending, &sent)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			switch {
			case e.Pending:
			case pending && sent != 0:
				l.moved.add(true, Moved{Collection: collection, Deleted: true, All: true})
			case !pending:
				l.moved.add(false, Moved{Collection: collection, Deleted: true, All: true})
			}
		}
	case m == nil || e.Locked:
	case it.Deleted:
		k, err := l.keyOf(tx, m, it)
		if err != nil {
			return err
		}
		key = sql.NullString{String: k, Valid: k != ""}
		if key.Valid {
			gone, err := l.deleteRow(tx, m, k)
			if err != nil {
				return err
			}
			if gone {
				l.moved.add(false, Moved{Collection: m.Collection, Deleted: true})
			}
		}
	default:
		// One whose row can't be told is kept whole, and writes no row.
		k, err := m.KeyOf(plain(m, it.GUID), it.Private)
		if err != nil {
			m = nil
			break
		}
		key = sql.NullString{String: k, Valid: true}
		if err := l.join(tx, m, k, it.Private); err != nil {
			return err
		}
	}
	// An item the server has now taken was pending and sent, and comes back neither.
	if mapped := l.maps[it.Collection]; mapped != nil && !e.Pending && !e.Locked {
		var pending bool
		var sent int64
		err := tx.QueryRow(`SELECT pending, sent FROM sync_items WHERE collection = ? AND guid = ?`, it.Collection, it.GUID).Scan(&pending, &sent)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if pending && sent != 0 {
			went := Moved{Collection: it.Collection, Deleted: it.Deleted}
			if !it.Deleted && key.Valid {
				went.Name = name(tx, mapped, key.String)
			}
			l.moved.add(true, went)
		}
	}
	// What a mapped item holds is its row's; a deleted one keeps nothing of it.
	stored := it
	if m != nil && !e.Locked {
		stored.Private = nil
	}
	b, err := synckosh.Marshal(stored)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT OR REPLACE INTO sync_items (collection, guid, key, deleted, timestamp, pending, sent, locked, item) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		it.Collection, it.GUID, key, it.Deleted, it.Timestamp, e.Pending, e.Sent, e.Locked, b)
	return err
}

// keyOf is the row a deleted item was: from its bookkeeping, or its guid when the guid is the
// key; "" for one this device never had.
func (l *local) keyOf(tx *store.Tx, m *Mapping, it synckosh.Item) (string, error) {
	var key sql.NullString
	err := tx.QueryRow(`SELECT key FROM sync_items WHERE collection = ? AND guid = ?`, it.Collection, it.GUID).Scan(&key)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if key.Valid {
		return key.String, nil
	}
	if !m.Derived {
		return plain(m, it.GUID), nil
	}
	return "", nil
}

// join takes a version into the row of key.
func (l *local) join(tx *store.Tx, m *Mapping, key string, private json.RawMessage) error {
	row, has, err := m.Read(tx, key)
	if err != nil {
		return err
	}
	want := private
	if has {
		want = m.Join(row, private)
	}
	if !has || !sameJSON(want, row) {
		marked, deleted, err := mark(tx, m.Collection, key)
		if err != nil {
			return err
		}
		if marked && deleted {
			return nil
		}
		if err := m.Write(tx, key, want); err != nil {
			return err
		}
		l.wrote[m.Collection] = true
		if row, has, err = m.Read(tx, key); err != nil {
			return err
		}
		// A version the feature couldn't keep wrote no row, and so is none received.
		if has {
			l.moved.add(false, Moved{Collection: m.Collection, Name: name(tx, m, key)})
		}
	}
	if has && !sameJSON(row, private) {
		_, err := tx.Exec(`INSERT OR IGNORE INTO sync_changes (collection, key, deleted) VALUES (?, ?, 0)`, m.Collection, key)
		return err
	}
	return nil
}

// deleteRow deletes the row of key for a delete from the server, unless it was changed here
// since: then it goes out again. It says whether it deleted one.
func (l *local) deleteRow(tx *store.Tx, m *Mapping, key string) (bool, error) {
	marked, _, err := mark(tx, m.Collection, key)
	if err != nil || marked {
		return false, err
	}
	_, has, err := m.Read(tx, key)
	if err != nil || !has {
		return false, err
	}
	l.wrote[m.Collection] = true
	return true, m.Delete(tx, key)
}

// cut follows a delete-before: the rows of its items at or before its cutoff go, but those
// changed here since, and so does their bookkeeping, as the server forgets them too.
func (l *local) cut(tx *store.Tx, m *Mapping, cutoff int64) error {
	r, err := tx.Query(`SELECT guid, key FROM sync_items WHERE collection = ? AND timestamp <= ? AND NOT locked`, m.Collection, cutoff)
	if err != nil {
		return err
	}
	type gone struct {
		guid string
		key  sql.NullString
	}
	var all []gone
	for r.Next() {
		var g gone
		if err := r.Scan(&g.guid, &g.key); err != nil {
			r.Close()
			return err
		}
		all = append(all, g)
	}
	r.Close()
	if err := r.Err(); err != nil {
		return err
	}
	for _, g := range all {
		if g.key.Valid {
			if _, err := l.deleteRow(tx, m, g.key.String); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`DELETE FROM sync_items WHERE collection = ? AND guid = ?`, m.Collection, g.guid); err != nil {
			return err
		}
	}
	return nil
}

// Remove forgets entries, which synckosh does only for a device away past the server's
// cleanup, starting over, and for deleted entries it cleans up: the rows stay, and one still
// here is marked to go out again.
func (l *local) Remove(collection string, guids ...string) error {
	if len(guids) == 0 {
		return nil
	}
	return l.do(func(tx *store.Tx) error {
		m := l.maps[collection]
		for _, g := range guids {
			var key sql.NullString
			err := tx.QueryRow(`DELETE FROM sync_items WHERE collection = ? AND guid = ? RETURNING key`, collection, g).Scan(&key)
			if err == sql.ErrNoRows {
				continue
			}
			if err != nil {
				return err
			}
			if m == nil || !key.Valid {
				continue
			}
			if _, has, err := m.Read(tx, key.String); err != nil {
				return err
			} else if has {
				if _, err := tx.Exec(`INSERT OR IGNORE INTO sync_changes (collection, key, deleted) VALUES (?, ?, 0)`, collection, key.String); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (l *local) Meta(name string) ([]byte, error) {
	var v []byte
	err := l.do(func(tx *store.Tx) error {
		err := tx.QueryRow(`SELECT value FROM sync_meta WHERE name = ?`, name).Scan(&v)
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	})
	return v, err
}

func (l *local) SetMeta(name string, value []byte) error {
	return l.do(func(tx *store.Tx) error {
		var err error
		if value == nil {
			_, err = tx.Exec(`DELETE FROM sync_meta WHERE name = ?`, name)
		} else {
			_, err = tx.Exec(`INSERT OR REPLACE INTO sync_meta (name, value) VALUES (?, ?)`, name, value)
		}
		return err
	})
}

// mark is whether a row has a change waiting to go out, and whether it is a delete.
func mark(tx *store.Tx, collection, key string) (marked, deleted bool, err error) {
	err = tx.QueryRow(`SELECT deleted FROM sync_changes WHERE collection = ? AND key = ?`, collection, key).Scan(&deleted)
	if err == sql.ErrNoRows {
		return false, false, nil
	}
	return err == nil, deleted, err
}

// quietly runs what sync writes to the features' rows with the triggers that mark changes
// off, in the same transaction, so no other job sees them off.
func quietly(tx *store.Tx, f func() error) error {
	var was int
	if err := tx.QueryRow(`SELECT marking FROM sync_state`).Scan(&was); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE sync_state SET marking = 0`); err != nil {
		return err
	}
	if err := f(); err != nil {
		return err
	}
	_, err := tx.Exec(`UPDATE sync_state SET marking = ?`, was)
	return err
}

// sameJSON says whether two parts hold the same, compacted.
func sameJSON(a, b json.RawMessage) bool {
	var ca, cb bytes.Buffer
	if json.Compact(&ca, a) != nil || json.Compact(&cb, b) != nil {
		return bytes.Equal(a, b)
	}
	return bytes.Equal(ca.Bytes(), cb.Bytes())
}
