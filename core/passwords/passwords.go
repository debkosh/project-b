// Package passwords is a profile's saved logins (docs/passwords.md): a site's origin and a
// username, plain, and the password, sealed with the profile key and bound to its row and its
// site. They come from other browsers and from files (Import); a person sees them listed in
// Settings, and a password is opened only after the device has checked the person.
package passwords

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/sudiptadeb/project-b/browserdata"

	"projectb/keys"
	"projectb/store"
	"projectb/timeline"
)

// What a call answers besides success and an error of the store's.
var (
	// ErrNone: no login has that id.
	ErrNone = errors.New("passwords: no such login")
	// ErrNotHere: the profile's key doesn't open on this device, so no password opens and
	// none can be kept.
	ErrNotHere = errors.New("passwords: this profile's passwords don't open on this device")
	// ErrDenied: the person didn't pass the device's check, or said no to it.
	ErrDenied = errors.New("passwords: the person wasn't verified")
	// ErrNoScreenLock: the device has no screen lock to check the person with.
	ErrNoScreenLock = errors.New("passwords: the device has no screen lock")
	// ErrCancelled: the caller's context ended while the check waited.
	ErrCancelled = errors.New("passwords: cancelled")
)

// Login is a saved login, without its password.
type Login struct {
	ID string `json:"id"`
	// Origin is the site's, scheme://host[:port].
	Origin   string `json:"origin"`
	Username string `json:"username"`
	// Created is when it was saved, and Used when it was last used, 0 for never or unknown:
	// Unix milliseconds.
	Created int64 `json:"created"`
	Used    int64 `json:"used"`
}

// Verify is the device checking the person before a password of the site is opened: nil
// when they passed, an error of its own otherwise. It may wait on them.
type Verify func(ctx context.Context, origin string) error

// Manager is one profile's saved logins.
type Manager struct {
	st     *store.Store
	keys   *keys.Keys
	verify Verify
	// changed runs, on whatever thread, after logins came or one went.
	changed func()
}

// New is the saved logins of the profile whose store is st and whose keys are k. verify is
// the device's check before a password is shown.
func New(st *store.Store, k *keys.Keys, verify Verify) *Manager {
	st.Schema("passwords", func(tx *store.Tx) error {
		_, err := tx.Exec(`
CREATE TABLE logins (
  id TEXT PRIMARY KEY,
  origin TEXT NOT NULL,
  username TEXT NOT NULL,
  password BLOB NOT NULL,
  created INTEGER NOT NULL,
  edited INTEGER NOT NULL,
  used INTEGER NOT NULL
) WITHOUT ROWID;
CREATE INDEX logins_site ON logins (origin, username)`)
		return err
	})
	// Starting fresh makes a new profile key, which opens none of these rows.
	k.OnStartFresh(func(tx *store.Tx) error {
		_, err := tx.Exec(`DELETE FROM logins`)
		return err
	})
	return &Manager{st: st, keys: k, verify: verify, changed: func() {}}
}

// OnChanged has changed run after logins came or one went, on whatever thread the call was
// on. Set once, at start.
func (m *Manager) OnChanged(changed func()) { m.changed = changed }

// Total is how many logins the profile has. It reads no sealed column.
func (m *Manager) Total() (int, error) {
	n := 0
	err := m.st.Wait(func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM logins`).Scan(&n)
	})
	return n, err
}

// List is every login, by its site and then its username. It reads no sealed column, so it
// needs no key and asks the person nothing.
func (m *Manager) List() ([]Login, error) {
	out := []Login{}
	err := m.st.Wait(func(tx *store.Tx) error {
		r, err := tx.Query(`SELECT id, origin, username, created, used FROM logins ORDER BY origin, username, created`)
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			var l Login
			if err := r.Scan(&l.ID, &l.Origin, &l.Username, &l.Created, &l.Used); err != nil {
				return err
			}
			out = append(out, l)
		}
		return r.Err()
	})
	return out, err
}

// Password is the login's password, after the device has checked the person: ErrNone for a
// login that isn't there, which asks them nothing, ErrNotHere when the profile's key doesn't
// open here, or the check's own error.
func (m *Manager) Password(ctx context.Context, id string) (string, error) {
	var origin string
	var sealed []byte
	err := m.st.Wait(func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT origin, password FROM logins WHERE id = ?`, id).Scan(&origin, &sealed)
	})
	if err == sql.ErrNoRows {
		return "", ErrNone
	}
	if err != nil {
		return "", err
	}
	if err := m.verify(ctx, origin); err != nil {
		return "", err
	}
	plain, err := m.keys.Open("logins", id, "password", sealed, origin)
	if errors.Is(err, keys.ErrLocked) {
		return "", ErrNotHere
	}
	return string(plain), err
}

// Delete deletes a login. One that isn't there is no error.
func (m *Manager) Delete(id string) error {
	err := m.st.Wait(func(tx *store.Tx) error {
		_, err := tx.Exec(`DELETE FROM logins WHERE id = ?`, id)
		return err
	})
	if err == nil {
		m.changed()
	}
	return err
}

// Import keeps logins brought from another browser or a file, and counts them: added;
// already, the ones saved here as they are, the same site, username and password; and
// skipped, one whose site isn't a web page's or that has no password, or that couldn't be
// sealed. A login for a site and username saved here with another password is added beside
// it: both stay, and the person chooses. Not on the main thread.
func (m *Manager) Import(logins []browserdata.Login) (added, already, skipped int) {
	err := m.st.Wait(func(tx *store.Tx) error {
		added, already, skipped = 0, 0, 0
		// The passwords saved here for a site and username, opened as they are first met.
		saved := map[[2]string]map[string]bool{}
		have := func(origin, username string) (map[string]bool, error) {
			at := [2]string{origin, username}
			if set, ok := saved[at]; ok {
				return set, nil
			}
			set := map[string]bool{}
			r, err := tx.Query(`SELECT id, password FROM logins WHERE origin = ? AND username = ?`, origin, username)
			if err != nil {
				return nil, err
			}
			defer r.Close()
			for r.Next() {
				var id string
				var sealed []byte
				if err := r.Scan(&id, &sealed); err != nil {
					return nil, err
				}
				plain, err := m.keys.Open("logins", id, "password", sealed, origin)
				if err != nil {
					return nil, err
				}
				set[string(plain)] = true
			}
			saved[at] = set
			return set, r.Err()
		}
		now := store.Now()
		for _, l := range logins {
			origin := strings.TrimRight(l.Origin, "/")
			if !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") || l.Password == "" {
				skipped++
				continue
			}
			set, err := have(origin, l.Username)
			if err != nil {
				return err
			}
			if set[l.Password] {
				already++
				continue
			}
			id := store.NewID()
			sealed, err := m.keys.Seal("logins", id, "password", []byte(l.Password), origin)
			if err != nil {
				return err
			}
			created, used := now, int64(0)
			if !l.Created.IsZero() {
				created = l.Created.UnixMilli()
			}
			if !l.LastUsed.IsZero() {
				used = l.LastUsed.UnixMilli()
			}
			if _, err := tx.Exec(`INSERT INTO logins (id, origin, username, password, created, edited, used) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				id, origin, l.Username, sealed, created, now, used); err != nil {
				return err
			}
			set[l.Password] = true
			added++
		}
		return nil
	})
	if err != nil {
		// Nothing was kept: the job's transaction went back.
		timeline.Errorf("passwords: bringing %d logins: %v", len(logins), err)
		return 0, 0, len(logins)
	}
	if added > 0 {
		m.changed()
	}
	return added, already, skipped
}
