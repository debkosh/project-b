// Package passkeys is a profile's own passkeys (docs/passkeys.md): each a row of the
// profile's store, its site plain and who the person is there sealed with the profile key,
// with the P-256 key that signs. It makes one, lists a site's, and signs with one, checking
// the person with the device's own lock right before it makes or signs; it never hands a
// key out. The rows sync as they are (sync.go).
//
// Every call but Count waits: on the store's worker, on the device's check, and, once
// Profile Lock is built, on its unlock Dialog. None is for the main thread.
package passkeys

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"projectb/keys"
	"projectb/store"
)

// What a call answers besides success and an error of the store's.
var (
	// ErrCancelled: the caller's context ended while the call waited.
	ErrCancelled = errors.New("passkeys: cancelled")
	// ErrLocked: the profile is locked, and the person pressed Cancel on the unlock Dialog,
	// or nobody asked for the call.
	ErrLocked = errors.New("passkeys: the profile is locked")
	// ErrDenied: the person didn't pass the device's check, or said no to it.
	ErrDenied = errors.New("passkeys: the person wasn't verified")
	// ErrNoScreenLock: the device has nothing to check the person with.
	ErrNoScreenLock = errors.New("passkeys: this device has no screen lock")
	// ErrExcluded: the site named a passkey kept here as one it already has.
	ErrExcluded = errors.New("passkeys: the site already has a passkey kept here")
	// ErrNotHere: the profile's key doesn't open on this device, as for a folder that came
	// from another: nothing sealed opens until the profile starts fresh.
	ErrNotHere = errors.New("passkeys: this profile's passkeys don't open on this device")
	// ErrNone: no passkey has that id for that site.
	ErrNone = errors.New("passkeys: no such passkey")
)

// Why is what the device's check is for, which its sheet says.
type Why int

const (
	ToSave Why = iota
	ToUse
)

// Verify checks the person is the device's owner, with whatever the device has, for a
// passkey of the site rpID. It waits for the person, and answers nil, ErrDenied,
// ErrNoScreenLock, or ErrCancelled when ctx ended first.
type Verify func(ctx context.Context, why Why, rpID string) error

// Passkey is one passkey, without its key. ID is the credential ID.
type Passkey struct {
	ID          []byte
	RPID        string
	UserHandle  []byte
	UserName    string
	DisplayName string
	// Created is when it was made, Unix milliseconds.
	Created int64
}

// Manager is one profile's passkeys.
type Manager struct {
	st     *store.Store
	keys   *keys.Keys
	verify Verify
	// changed runs, on whatever thread, after a passkey was made or deleted here.
	changed func()
}

// New is the passkeys of the profile whose store is st and whose keys are k. verify is the
// device's check.
func New(st *store.Store, k *keys.Keys, verify Verify) *Manager {
	st.Schema("passkeys", func(tx *store.Tx) error {
		_, err := tx.Exec(`
CREATE TABLE passkeys (
  id TEXT PRIMARY KEY,
  rp_id TEXT NOT NULL,
  created INTEGER NOT NULL,
  user_handle BLOB NOT NULL,
  user_name BLOB NOT NULL,
  display_name BLOB NOT NULL,
  key BLOB NOT NULL
) WITHOUT ROWID;
CREATE INDEX passkeys_rp ON passkeys (rp_id)`)
		return err
	})
	// Starting fresh makes a new profile key, which opens none of these rows.
	k.OnStartFresh(func(tx *store.Tx) error {
		_, err := tx.Exec(`DELETE FROM passkeys`)
		return err
	})
	return &Manager{st: st, keys: k, verify: verify, changed: func() {}}
}

// here says the profile's key opens on this device, or nothing was sealed yet: nil, or
// ErrNotHere. It may ask the keychain, once.
func (m *Manager) here() error {
	// A profile Profile Lock holds is here: sealed asks for its passphrase.
	if err := m.keys.Check(); errors.Is(err, keys.ErrNotHere) {
		return ErrNotHere
	} else if err != nil && !errors.Is(err, keys.ErrLocked) {
		return err
	}
	return nil
}

// OnChanged has changed run after a passkey is made or deleted here, on whatever thread
// the call was on. Set once, at start.
func (m *Manager) OnChanged(changed func()) { m.changed = changed }

// name is a credential ID as its row's id: base64url, as a page's JSON carries one.
func name(id []byte) string { return base64.RawURLEncoding.EncodeToString(id) }

func unname(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

// among is SQL's "AND id IN (...)" for the credential IDs a page named, with its values; ""
// when it named none, which is any.
func among(ids [][]byte) (string, []any) {
	if len(ids) == 0 {
		return "", nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = name(id)
	}
	return " AND id IN (?" + strings.Repeat(", ?", len(ids)-1) + ")", args
}

// Count is how many passkeys the site has here, of those allow names when it names any. It
// reads the plain columns alone, so it never needs the profile's key.
func (m *Manager) Count(rpID string, allow [][]byte) (int, error) {
	n := 0
	in, args := among(allow)
	err := m.st.Wait(func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM passkeys WHERE rp_id = ?`+in, append([]any{rpID}, args...)...).Scan(&n)
	})
	return n, err
}

// Total is how many passkeys the profile has, from the plain columns alone.
func (m *Manager) Total() (int, error) {
	n := 0
	err := m.st.Wait(func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM passkeys`).Scan(&n)
	})
	return n, err
}

// sealed runs job, which reads or writes what the profile key seals, on the store's
// worker. asked says a person asked for it. It is the one place a call meets the profile
// key. While Profile Lock holds the profile, a person's job waits here on the unlock Dialog
// (keys.Unlocked, which says what for from the context), and one nobody asked for is
// answered ErrLocked, as one whose Dialog the person cancelled is.
func (m *Manager) sealed(ctx context.Context, asked bool, job func(tx *store.Tx) error) error {
	if err := m.unlocked(ctx, asked); err != nil {
		return err
	}
	return m.st.Wait(job)
}

// unlocked is sealed's wait alone, for a call that checks the person with the device before
// its job: the profile's passphrase comes first, so nobody passes the device's check for a
// profile that then stays locked.
func (m *Manager) unlocked(ctx context.Context, asked bool) error {
	if ctx.Err() != nil {
		return ErrCancelled
	}
	switch err := m.keys.Unlocked(ctx, asked); {
	case err == nil:
		return nil
	case ctx.Err() != nil:
		return ErrCancelled
	case errors.Is(err, keys.ErrCancelled), errors.Is(err, keys.ErrLocked):
		return ErrLocked
	default:
		return err
	}
}

// row is a passkey as its row keeps it.
type row struct {
	Passkey
	key []byte // the private key, PKCS #8
}

const columns = `id, rp_id, created, user_handle, user_name, display_name, key`

// The sealed columns. Each seal binds the row's id and its site, so a value moved to
// another row, or a row to another site, doesn't open.
func (m *Manager) seal(id, rpID, column string, plain []byte) ([]byte, error) {
	return m.keys.Seal("passkeys", id, column, plain, rpID)
}

func (m *Manager) open(id, rpID, column string, sealed []byte) ([]byte, error) {
	return m.keys.Open("passkeys", id, column, sealed, rpID)
}

// put writes r as its row, new or in place of the one with its id.
func (m *Manager) put(tx *store.Tx, r row) error {
	id := name(r.ID)
	var sealed [4][]byte
	for i, c := range []struct {
		column string
		plain  []byte
	}{{"user_handle", r.UserHandle}, {"user_name", []byte(r.UserName)}, {"display_name", []byte(r.DisplayName)}, {"key", r.key}} {
		var err error
		if sealed[i], err = m.seal(id, r.RPID, c.column, c.plain); err != nil {
			return err
		}
	}
	_, err := tx.Exec(`INSERT OR REPLACE INTO passkeys (`+columns+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, r.RPID, r.Created, sealed[0], sealed[1], sealed[2], sealed[3])
	return err
}

// scan reads a row and opens it; withKey says whether its private key is wanted too.
func (m *Manager) scan(r interface{ Scan(...any) error }, withKey bool) (row, error) {
	var out row
	var id string
	var handle, user, display, key []byte
	if err := r.Scan(&id, &out.RPID, &out.Created, &handle, &user, &display, &key); err != nil {
		return row{}, err
	}
	var err error
	if out.ID, err = unname(id); err != nil {
		return row{}, fmt.Errorf("passkeys: a row's id: %w", err)
	}
	if out.UserHandle, err = m.open(id, out.RPID, "user_handle", handle); err != nil {
		return row{}, err
	}
	if user, err = m.open(id, out.RPID, "user_name", user); err != nil {
		return row{}, err
	}
	if display, err = m.open(id, out.RPID, "display_name", display); err != nil {
		return row{}, err
	}
	out.UserName, out.DisplayName = string(user), string(display)
	if withKey {
		if out.key, err = m.open(id, out.RPID, "key", key); err != nil {
			return row{}, err
		}
	}
	return out, nil
}

// query is the rows a query finds, opened, without their keys.
func (m *Manager) query(tx *store.Tx, where string, args ...any) ([]Passkey, error) {
	rows, err := tx.Query(`SELECT `+columns+` FROM passkeys `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Passkey
	for rows.Next() {
		r, err := m.scan(rows, false)
		if err != nil {
			return nil, err
		}
		out = append(out, r.Passkey)
	}
	return out, rows.Err()
}

// List is the site's passkeys, of those allow names when it names any, the newest first.
// Two for one account, as two devices can each make while offline, show as the newer: the
// older stays in the store, and in All.
func (m *Manager) List(ctx context.Context, asked bool, rpID string, allow [][]byte) ([]Passkey, error) {
	if err := m.here(); err != nil {
		return nil, err
	}
	var all []Passkey
	in, args := among(allow)
	err := m.sealed(ctx, asked, func(tx *store.Tx) error {
		var err error
		all, err = m.query(tx, `WHERE rp_id = ?`+in+` ORDER BY created DESC, id`, append([]any{rpID}, args...)...)
		return err
	})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := all[:0]
	for _, p := range all {
		if !seen[string(p.UserHandle)] {
			seen[string(p.UserHandle)] = true
			out = append(out, p)
		}
	}
	return out, nil
}

// All is every passkey of the profile, by site and then the newest first, for Settings.
func (m *Manager) All(ctx context.Context, asked bool) ([]Passkey, error) {
	if err := m.here(); err != nil {
		return nil, err
	}
	var all []Passkey
	err := m.sealed(ctx, asked, func(tx *store.Tx) error {
		var err error
		all, err = m.query(tx, `ORDER BY rp_id, created DESC, id`)
		return err
	})
	return all, err
}

// User is who a site says a new passkey is for.
type User struct {
	Handle            []byte
	Name, DisplayName string
}

// Make checks the person, then makes a passkey for user at the site rpID and keeps it, in
// place of any the site already had here for that user. It answers its credential ID and
// its public key, as a COSE key. A passkey kept here that exclude names answers
// ErrExcluded, and a profile whose key doesn't open here ErrNotHere, before the person is
// asked anything.
//
// A locked profile is unlocked first, then the person is checked with the device, then the
// passkey is made.
func (m *Manager) Make(ctx context.Context, asked bool, rpID string, user User, exclude [][]byte) (id, cose []byte, err error) {
	if err := m.here(); err != nil {
		return nil, nil, err
	}
	if len(exclude) > 0 {
		n, err := m.Count(rpID, exclude)
		if err != nil {
			return nil, nil, err
		}
		if n > 0 {
			return nil, nil, ErrExcluded
		}
	}
	if err := m.unlocked(ctx, asked); err != nil {
		return nil, nil, err
	}
	if err := m.verify(ctx, ToSave, rpID); err != nil {
		return nil, nil, err
	}
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, nil, err
	}
	id = make([]byte, 16)
	rand.Read(id)
	made := row{Passkey: Passkey{ID: id, RPID: rpID, UserHandle: user.Handle, UserName: user.Name, DisplayName: user.DisplayName, Created: store.Now()}, key: der}
	err = m.sealed(ctx, asked, func(tx *store.Tx) error {
		// The user handles are sealed, so the site's rows are opened to find the one this
		// replaces: SQL can't compare them.
		had, err := m.query(tx, `WHERE rp_id = ?`, rpID)
		if err != nil {
			return err
		}
		for _, p := range had {
			if string(p.UserHandle) == string(user.Handle) {
				if _, err := tx.Exec(`DELETE FROM passkeys WHERE id = ?`, name(p.ID)); err != nil {
					return err
				}
			}
		}
		return m.put(tx, made)
	})
	if err != nil {
		return nil, nil, err
	}
	m.changed()
	return id, coseKey(&private.PublicKey), nil
}

// Sign checks the person, then signs data with the passkey id of the site rpID: an ES256
// signature in DER, with the passkey's user handle. data is what the site verifies, the
// authenticator data and then the hash of the client data. The person is asked nothing for
// a passkey that isn't here, or a profile whose key doesn't open here.
func (m *Manager) Sign(ctx context.Context, asked bool, rpID string, id, data []byte) (signature, userHandle []byte, err error) {
	if err := m.here(); err != nil {
		return nil, nil, err
	}
	if n, err := m.Count(rpID, [][]byte{id}); err != nil {
		return nil, nil, err
	} else if n == 0 {
		return nil, nil, ErrNone
	}
	if err := m.unlocked(ctx, asked); err != nil {
		return nil, nil, err
	}
	if err := m.verify(ctx, ToUse, rpID); err != nil {
		return nil, nil, err
	}
	var found row
	none := false
	err = m.sealed(ctx, asked, func(tx *store.Tx) error {
		var err error
		found, err = m.scan(tx.QueryRow(`SELECT `+columns+` FROM passkeys WHERE id = ? AND rp_id = ?`, name(id), rpID), true)
		if err == sql.ErrNoRows {
			none, err = true, nil
		}
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	if none {
		return nil, nil, ErrNone
	}
	parsed, err := x509.ParsePKCS8PrivateKey(found.key)
	if err != nil {
		return nil, nil, fmt.Errorf("passkeys: the key of %s: %w", name(id), err)
	}
	private, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("passkeys: the key of %s is not a P-256 key", name(id))
	}
	digest := sha256.Sum256(data)
	signature, err = ecdsa.SignASN1(rand.Reader, private, digest[:])
	return signature, found.UserHandle, err
}

// Delete deletes the passkey id. The site keeps its public key until the person removes it
// there.
func (m *Manager) Delete(ctx context.Context, asked bool, id []byte) error {
	err := m.sealed(ctx, asked, func(tx *store.Tx) error {
		_, err := tx.Exec(`DELETE FROM passkeys WHERE id = ?`, name(id))
		return err
	})
	if err == nil {
		m.changed()
	}
	return err
}
