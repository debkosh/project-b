package passkeys

import (
	"database/sql"
	"encoding/json"

	"projectb/store"
	"projectb/sync"
)

// Collection is the passkeys' collection in sync.
const Collection = "passkeys"

// item is a passkey as its synced item's private part: every column in the clear, the key
// among them, as synckosh seals the private part end to end under the sync key and sync
// keeps no copy of it. A []byte is base64 in JSON.
type item struct {
	RPID        string `json:"rpId"`
	UserHandle  []byte `json:"userHandle"`
	UserName    string `json:"userName"`
	DisplayName string `json:"displayName"`
	Created     int64  `json:"created"`
	Key         []byte `json:"key"`
}

// Synced is how the passkeys sync: an item each, its credential ID the guid. Read opens a
// row and Write seals one, in sync's own job: sync runs only while the profile's key is at
// hand, and stops altogether while a profile is locked. A passkey never changes once made,
// so of two versions the one made later stays. changed runs on the main thread after a run
// wrote rows.
func (m *Manager) Synced(changed func()) sync.Mapping {
	return sync.Mapping{
		Collection: Collection, Datatype: "passkey", Table: "passkeys", Key: "id",
		KeyOf: func(guid string, _ json.RawMessage) (string, error) { return guid, nil },
		Read: func(tx *store.Tx, id string) (json.RawMessage, bool, error) {
			r, err := m.scan(tx.QueryRow(`SELECT `+columns+` FROM passkeys WHERE id = ?`, id), true)
			if err == sql.ErrNoRows {
				return nil, false, nil
			}
			if err != nil {
				return nil, false, err
			}
			b, err := json.Marshal(item{RPID: r.RPID, UserHandle: r.UserHandle, UserName: r.UserName, DisplayName: r.DisplayName, Created: r.Created, Key: r.key})
			return b, err == nil, err
		},
		Write: func(tx *store.Tx, id string, private json.RawMessage) error {
			var it item
			raw, err := unname(id)
			if err != nil || json.Unmarshal(private, &it) != nil || it.RPID == "" || len(it.Key) == 0 {
				return nil
			}
			return m.put(tx, row{Passkey: Passkey{ID: raw, RPID: it.RPID, UserHandle: it.UserHandle, UserName: it.UserName, DisplayName: it.DisplayName, Created: it.Created}, key: it.Key})
		},
		Delete: func(tx *store.Tx, id string) error {
			_, err := tx.Exec(`DELETE FROM passkeys WHERE id = ?`, id)
			return err
		},
		Join: func(row, other json.RawMessage) json.RawMessage {
			var a, b item
			if json.Unmarshal(row, &a) != nil {
				return other
			}
			if json.Unmarshal(other, &b) != nil || a.Created > b.Created || a.Created == b.Created && string(row) >= string(other) {
				return row
			}
			return other
		},
		Changed: changed,
	}
}
