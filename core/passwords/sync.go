package passwords

import (
	"database/sql"
	"encoding/json"
	"errors"

	"projectb/store"
	"projectb/sync"
)

// Collection is the saved logins' collection in sync.
const Collection = "logins"

// item is a login as its synced item's private part: every column in the clear, the password
// among them, as synckosh seals the private part end to end under the sync key and sync
// keeps no copy of it.
type item struct {
	ID       string `json:"id"`
	Origin   string `json:"origin"`
	Username string `json:"username"`
	Password string `json:"password"`
	Created  int64  `json:"created"`
	Edited   int64  `json:"edited"`
	Used     int64  `json:"used"`
}

// Synced is how the saved logins sync (docs/passwords.md): an item each, the one changed
// last winning. Read opens a row's password and Write seals one, in sync's own job, asking
// the person nothing: sync runs only while the profile's key is at hand, and what leaves
// is sealed under the sync key. A row that doesn't open makes Read fail, and sync sets that
// row aside, saying why, while the rest syncs.
//
// What makes two devices' logins one is the site, the username and the password, not the
// row's id: the same login brought to two devices is one entry on both, and a second
// password for a site and username stays a second entry, as Import keeps it.
func (m *Manager) Synced() sync.Mapping {
	return sync.Mapping{
		Collection: Collection, Datatype: "login", Table: "logins", Key: "id",
		KeyOf: func(private json.RawMessage) (string, error) {
			var it item
			if json.Unmarshal(private, &it) != nil || it.ID == "" {
				return "", errors.New("passwords: an item with no id")
			}
			return it.ID, nil
		},
		Identity: func(private json.RawMessage) []byte {
			var it item
			if json.Unmarshal(private, &it) != nil {
				return nil
			}
			b, _ := json.Marshal([]string{it.Origin, it.Username, it.Password})
			return b
		},
		Twin: func(tx *store.Tx, id string, private json.RawMessage) (string, error) {
			var it item
			if json.Unmarshal(private, &it) != nil {
				return "", nil
			}
			r, err := tx.Query(`SELECT id, password FROM logins WHERE origin = ? AND username = ? AND id <> ?`, it.Origin, it.Username, id)
			if err != nil {
				return "", err
			}
			defer r.Close()
			for r.Next() {
				var other string
				var sealed []byte
				if err := r.Scan(&other, &sealed); err != nil {
					return "", err
				}
				if plain, err := m.keys.Open("logins", other, "password", sealed, it.Origin); err == nil && string(plain) == it.Password {
					return other, nil
				}
			}
			return "", r.Err()
		},
		// Its site and username: neither is sealed, and Sync details opens nothing.
		Name: func(tx *store.Tx, id string) string {
			var origin, username string
			tx.QueryRow(`SELECT origin, username FROM logins WHERE id = ?`, id).Scan(&origin, &username)
			if username == "" {
				return origin
			}
			return origin + " · " + username
		},
		Read: func(tx *store.Tx, id string) (json.RawMessage, bool, error) {
			it := item{ID: id}
			var sealed []byte
			err := tx.QueryRow(`SELECT origin, username, password, created, edited, used FROM logins WHERE id = ?`, id).
				Scan(&it.Origin, &it.Username, &sealed, &it.Created, &it.Edited, &it.Used)
			if err == sql.ErrNoRows {
				return nil, false, nil
			}
			if err != nil {
				return nil, false, err
			}
			plain, err := m.keys.Open("logins", id, "password", sealed, it.Origin)
			if err != nil {
				return nil, false, err
			}
			it.Password = string(plain)
			b, err := json.Marshal(it)
			return b, err == nil, err
		},
		Write: func(tx *store.Tx, id string, private json.RawMessage) error {
			var it item
			if json.Unmarshal(private, &it) != nil || it.Origin == "" || it.Password == "" {
				return nil
			}
			sealed, err := m.keys.Seal("logins", id, "password", []byte(it.Password), it.Origin)
			if err != nil {
				return err
			}
			_, err = tx.Exec(`INSERT OR REPLACE INTO logins (id, origin, username, password, created, edited, used) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				id, it.Origin, it.Username, sealed, it.Created, it.Edited, it.Used)
			return err
		},
		Delete: func(tx *store.Tx, id string) error {
			_, err := tx.Exec(`DELETE FROM logins WHERE id = ?`, id)
			return err
		},
		Join: func(row, other json.RawMessage) json.RawMessage {
			var a, b item
			if json.Unmarshal(row, &a) != nil {
				return other
			}
			if json.Unmarshal(other, &b) != nil || a.Edited > b.Edited || a.Edited == b.Edited && string(row) >= string(other) {
				return row
			}
			return other
		},
		Changed: func() { m.changed() },
	}
}
