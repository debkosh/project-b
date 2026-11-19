// Package keys is a profile's keys (docs/sync.md): the profile key, 32 random bytes made the
// first time something is sealed, kept in the profile folder's profile.key wrapped by the
// device key; what it seals, field by field; and the secrets table, named values sealed with
// it. A folder named outright wraps its profile key with a key of its own, folder.key beside
// it, and never asks for a device key.
package keys

import (
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"

	"projectb/store"
)

// Device is the key that wraps a profile's key: the device key, or a folder's own. It
// returns ErrNoDeviceKey when there is none, and makes one first when create is true.
type Device func(create bool) ([]byte, error)

// ErrNoDeviceKey is a Device's answer when it holds no key and wasn't asked to make one.
var ErrNoDeviceKey = errors.New("keys: no device key")

// ErrLocked is what sealing and opening fail with when the device key doesn't open the
// profile's key: the folder came from another device, or the keychain's item was lost or
// replaced. Nothing is overwritten; StartFresh makes a new key.
var ErrLocked = errors.New("keys: this profile's key doesn't open with this device's key")

// errNone is the profile having no key yet: nothing was sealed in it.
var errNone = errors.New("keys: nothing has been sealed in this profile")

// Keys is one profile's keys.
type Keys struct {
	dir, profile string
	st           *store.Store
	device       Device

	mu  sync.Mutex
	key []byte // the profile key, once opened
	// fresh are what StartFresh deletes besides the secrets: each feature's own sealed rows.
	fresh []func(tx *store.Tx) error
}

// OnStartFresh has job run in StartFresh's own job, beside the secrets' delete: a feature
// that keeps rows sealed with the profile key deletes them there, as they never open
// again once the key is new. Set as the feature starts.
func (k *Keys) OnStartFresh(job func(tx *store.Tx) error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.fresh = append(k.fresh, job)
}

// New returns the keys of the profile folder dir, whose id in the profile list is profile, ""
// for a folder named outright. device is the device key for a listed profile, nil for a
// folder named outright, whose profile key is wrapped by its folder.key.
func New(dir, profile string, st *store.Store, device Device) *Keys {
	if device == nil {
		device = folderKey(dir)
	}
	st.Schema("keys", func(tx *store.Tx) error {
		_, err := tx.Exec(`CREATE TABLE secrets (name TEXT PRIMARY KEY, value BLOB NOT NULL) WITHOUT ROWID`)
		return err
	})
	return &Keys{dir: dir, profile: profile, st: st, device: device}
}

// Check opens the profile key: nil when it opens or nothing was sealed yet, ErrLocked when
// the device key doesn't open it, or why the device key couldn't be read. Call it off the main
// thread: it may ask the keychain.
func (k *Keys) Check() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	_, err := k.profileKey(false)
	if err == errNone {
		return nil
	}
	return err
}

// profileKey is the profile key, made when there is none and create is true. Call it with mu
// held.
func (k *Keys) profileKey(create bool) ([]byte, error) {
	if k.key != nil {
		return k.key, nil
	}
	path := filepath.Join(k.dir, "profile.key")
	wrapped, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if !create {
			return nil, errNone
		}
		return k.makeKey(path)
	}
	if err != nil {
		return nil, err
	}
	wrap, err := k.device(false)
	if errors.Is(err, ErrNoDeviceKey) {
		return nil, ErrLocked
	}
	if err != nil {
		return nil, fmt.Errorf("keys: the device key: %w", err)
	}
	key, err := open(wrap, wrapped, k.wrapping())
	if err != nil || len(key) != chacha20poly1305.KeySize {
		return nil, ErrLocked
	}
	k.key = key
	return key, nil
}

// makeKey makes the profile key and keeps it at path, wrapped. path isn't there.
func (k *Keys) makeKey(path string) ([]byte, error) {
	wrap, err := k.device(true)
	if err != nil {
		return nil, fmt.Errorf("keys: the device key: %w", err)
	}
	key := random32()
	wrapped, err := seal(wrap, key, k.wrapping())
	if err != nil {
		return nil, err
	}
	if err := writeNew(path, wrapped); err != nil {
		return nil, err
	}
	k.key = key
	return key, nil
}

// wrapping is what the profile key's wrapping binds: the profile's id, so a folder listed
// under another id doesn't open.
func (k *Keys) wrapping() []byte { return bound("profile key", k.profile) }

// Seal seals plain for the column of row id in table, with any other values the row binds
// in: it opens only with the same. The answer is the nonce, then the sealed bytes. It makes the
// profile key the first time.
func (k *Keys) Seal(table, id, column string, plain []byte, also ...string) ([]byte, error) {
	k.mu.Lock()
	key, err := k.profileKey(true)
	k.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return seal(key, plain, bound(append([]string{table, id, column}, also...)...))
}

// Open opens what Seal sealed for the same table, row, column and values.
func (k *Keys) Open(table, id, column string, sealed []byte, also ...string) ([]byte, error) {
	k.mu.Lock()
	key, err := k.profileKey(false)
	k.mu.Unlock()
	if err != nil {
		return nil, err
	}
	plain, err := open(key, sealed, bound(append([]string{table, id, column}, also...)...))
	if err != nil {
		return nil, fmt.Errorf("keys: %s %s %s doesn't open: %w", table, id, column, err)
	}
	return plain, nil
}

// PutSecret keeps value under name, sealed, in the job's transaction.
func (k *Keys) PutSecret(tx *store.Tx, name string, value []byte) error {
	sealed, err := k.Seal("secrets", name, "value", value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO secrets (name, value) VALUES (?, ?) ON CONFLICT (name) DO UPDATE SET value = excluded.value`, name, sealed)
	return err
}

// Secret is the value kept under name, and whether there is one.
func (k *Keys) Secret(tx *store.Tx, name string) ([]byte, bool, error) {
	var sealed []byte
	err := tx.QueryRow(`SELECT value FROM secrets WHERE name = ?`, name).Scan(&sealed)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	plain, err := k.Open("secrets", name, "value", sealed)
	return plain, err == nil, err
}

// DeleteSecrets deletes the values kept under names.
func (k *Keys) DeleteSecrets(tx *store.Tx, names ...string) error {
	for _, n := range names {
		if _, err := tx.Exec(`DELETE FROM secrets WHERE name = ?`, n); err != nil {
			return err
		}
	}
	return nil
}

// StartFresh is for a profile whose key is locked: it deletes every secret, the features'
// sealed rows (OnStartFresh) and the profile key, and makes a new key, which the device key
// wraps. History and bookmarks are untouched. Not on the main thread: it waits for the
// store's worker.
func (k *Keys) StartFresh() error {
	k.mu.Lock()
	fresh := append([]func(tx *store.Tx) error(nil), k.fresh...)
	k.mu.Unlock()
	if err := k.st.Wait(func(tx *store.Tx) error {
		if _, err := tx.Exec(`DELETE FROM secrets`); err != nil {
			return err
		}
		for _, job := range fresh {
			if err := job(tx); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	path := filepath.Join(k.dir, "profile.key")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	k.key = nil
	_, err := k.makeKey(path)
	return err
}

// folderKey is a folder's own key, folder.key in it, for a folder named outright.
func folderKey(dir string) Device {
	return func(create bool) ([]byte, error) {
		path := filepath.Join(dir, "folder.key")
		b, err := os.ReadFile(path)
		switch {
		case err == nil && len(b) == chacha20poly1305.KeySize:
			return b, nil
		case err == nil:
			return nil, fmt.Errorf("keys: %s is %d bytes", path, len(b))
		case !errors.Is(err, os.ErrNotExist):
			return nil, err
		case !create:
			return nil, ErrNoDeviceKey
		}
		b = random32()
		return b, writeNew(path, b)
	}
}

func random32() []byte {
	b := make([]byte, chacha20poly1305.KeySize)
	rand.Read(b)
	return b
}

// writeNew writes a file that isn't there, readable by this user alone, and never one that is.
func writeNew(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// bound is the additional data of a seal: each value with its length before it, so no two
// lists of values read the same.
func bound(values ...string) []byte {
	out := []byte("project-b\x00")
	for _, v := range values {
		out = binary.AppendUvarint(out, uint64(len(v)))
		out = append(out, v...)
	}
	return out
}

// seal is XChaCha20-Poly1305 with a fresh random nonce: the nonce, then the sealed bytes.
func seal(key, plain, ad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+len(plain)+aead.Overhead())
	rand.Read(nonce)
	return aead.Seal(nonce, nonce, plain, ad), nil
}

func open(key, sealed, ad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("too short")
	}
	return aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], ad)
}
