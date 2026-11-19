package keys

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"projectb/store"
)

func openStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	return st
}

// device is a device key held in memory, counting how often it is asked for.
type device struct {
	key   []byte
	asked int
}

func (d *device) get(create bool) ([]byte, error) {
	d.asked++
	if d.key == nil {
		if !create {
			return nil, ErrNoDeviceKey
		}
		d.key = random32()
	}
	return d.key, nil
}

// A sealed value opens only for its own table, row, column and bound values.
func TestSealBindsItsPlace(t *testing.T) {
	dir := t.TempDir()
	k := New(dir, "", openStore(t, dir), nil)
	sealed, err := k.Seal("logins", "row1", "password", []byte("hunter2"), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("hunter2")) {
		t.Fatal("the sealed value holds the plain one")
	}
	again, _ := k.Seal("logins", "row1", "password", []byte("hunter2"), "https://example.com")
	if bytes.Equal(sealed, again) {
		t.Fatal("two seals of one value are the same: the nonce isn't fresh")
	}
	got, err := k.Open("logins", "row1", "password", sealed, "https://example.com")
	if err != nil || string(got) != "hunter2" {
		t.Fatalf("opened %q, %v", got, err)
	}
	for _, other := range [][]string{
		{"other", "row1", "password", "https://example.com"},
		{"logins", "row2", "password", "https://example.com"},
		{"logins", "row1", "username", "https://example.com"},
		{"logins", "row1", "password", "https://evil.example"},
		{"logins", "row1", "password"},
		{"logins", "row1password", "", "https://example.com"},
	} {
		if _, err := k.Open(other[0], other[1], other[2], sealed, other[3:]...); err == nil {
			t.Fatalf("opened as %v", other)
		}
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := k.Open("logins", "row1", "password", sealed, "https://example.com"); err == nil {
		t.Fatal("a changed value opened")
	}
}

// A folder named outright keeps its own key beside its profile key, and opens again with it.
func TestAFolderNamedOutrightKeepsItsOwnKey(t *testing.T) {
	dir := t.TempDir()
	st := openStore(t, dir)
	k := New(dir, "", st, nil)
	if err := k.Check(); err != nil {
		t.Fatalf("before anything is sealed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "profile.key")); !os.IsNotExist(err) {
		t.Fatal("a profile key before anything was sealed")
	}
	if err := st.Wait(func(tx *store.Tx) error { return k.PutSecret(tx, "sync.token", []byte("t0ken")) }); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"profile.key", "folder.key"} {
		fi, err := os.Stat(filepath.Join(dir, f))
		if err != nil || fi.Mode().Perm() != 0o600 {
			t.Fatalf("%s: %v %v", f, fi, err)
		}
	}
	k = New(dir, "", st, nil)
	var got []byte
	st.Wait(func(tx *store.Tx) (err error) {
		got, _, err = k.Secret(tx, "sync.token")
		return err
	})
	if string(got) != "t0ken" {
		t.Fatalf("got %q", got)
	}
}

// A listed profile's folder, opened outright, asks no device key: its profile key is locked,
// and no folder key is made.
func TestAListedProfileOpenedOutrightAsksNoDeviceKey(t *testing.T) {
	dir := t.TempDir()
	st := openStore(t, dir)
	d := &device{}
	if _, err := New(dir, "p1", st, d.get).Seal("secrets", "a", "value", []byte("x")); err != nil {
		t.Fatal(err)
	}
	asked := d.asked
	k := New(dir, "", st, nil)
	if err := k.Check(); !errors.Is(err, ErrLocked) {
		t.Fatalf("check: %v", err)
	}
	if d.asked != asked {
		t.Fatal("a folder named outright asked for the device key")
	}
	if _, err := os.Stat(filepath.Join(dir, "folder.key")); !os.IsNotExist(err) {
		t.Fatal("a folder key was made beside a profile key it can't open")
	}
}

// A profile key the device key doesn't open is locked: nothing seals or opens, nothing is
// overwritten, until StartFresh deletes the secrets and makes a new key.
func TestALockedKeyAndStartingFresh(t *testing.T) {
	dir := t.TempDir()
	st := openStore(t, dir)
	first := &device{}
	k := New(dir, "p1", st, first.get)
	if err := st.Wait(func(tx *store.Tx) error { return k.PutSecret(tx, "sync.token", []byte("t0ken")) }); err != nil {
		t.Fatal(err)
	}
	was, _ := os.ReadFile(filepath.Join(dir, "profile.key"))

	for name, d := range map[string]*device{"another device's key": {key: random32()}, "no device key": {}} {
		k = New(dir, "p1", st, d.get)
		if err := k.Check(); !errors.Is(err, ErrLocked) {
			t.Fatalf("%s: check: %v", name, err)
		}
		if _, err := k.Seal("secrets", "b", "value", []byte("y")); !errors.Is(err, ErrLocked) {
			t.Fatalf("%s: seal: %v", name, err)
		}
		err := st.Wait(func(tx *store.Tx) error { _, _, err := k.Secret(tx, "sync.token"); return err })
		if !errors.Is(err, ErrLocked) {
			t.Fatalf("%s: secret: %v", name, err)
		}
		if d.key == nil && d.asked == 0 {
			t.Fatalf("%s: the device key wasn't asked for", name)
		}
		if now, _ := os.ReadFile(filepath.Join(dir, "profile.key")); !bytes.Equal(now, was) {
			t.Fatalf("%s: the profile key was overwritten", name)
		}
	}
	// Listed under another id, the same device key doesn't open it either.
	if err := New(dir, "p2", st, first.get).Check(); !errors.Is(err, ErrLocked) {
		t.Fatalf("under another id: %v", err)
	}

	second := &device{}
	k = New(dir, "p1", st, second.get)
	if err := k.StartFresh(); err != nil {
		t.Fatal(err)
	}
	if err := k.Check(); err != nil {
		t.Fatal(err)
	}
	var has bool
	st.Wait(func(tx *store.Tx) (err error) { _, has, err = k.Secret(tx, "sync.token"); return err })
	if has {
		t.Fatal("the secret sealed with the old key is still there")
	}
	if err := New(dir, "p1", st, second.get).Check(); err != nil {
		t.Fatalf("the new key, opened again: %v", err)
	}
}

// The device key in a file, as the Mac keeps it beside the profile list: made the first time a
// listed profile seals, readable by this user alone, and one for every profile there. A profile
// folder copied to another device's root is locked, and so is every profile once the file is
// gone, with nothing overwritten, until StartFresh makes a new key and a new device key.
func TestTheDeviceKeyFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "device.key")
	device := File(path)
	if _, err := device(false); err != ErrNoDeviceKey {
		t.Fatalf("before anything is sealed: %v", err)
	}
	open := func(dir, id string, device Device) (*Keys, *store.Store) {
		st := openStore(t, dir)
		return New(dir, id, st, device), st
	}
	p1, st1 := open(filepath.Join(root, "p1"), "p1", device)
	if err := st1.Wait(func(tx *store.Tx) error { return p1.PutSecret(tx, "sync.token", []byte("t0ken")) }); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Mode().Perm() != 0o600 || fi.Size() != 32 {
		t.Fatalf("device.key: %v %v", fi, err)
	}
	made, _ := os.ReadFile(path)
	p2, _ := open(filepath.Join(root, "p2"), "p2", device)
	if _, err := p2.Seal("secrets", "a", "value", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if now, _ := os.ReadFile(path); !bytes.Equal(now, made) {
		t.Fatal("a second profile made another device key")
	}
	if _, err := os.Stat(filepath.Join(root, "p1", "folder.key")); !os.IsNotExist(err) {
		t.Fatal("a listed profile made a folder key")
	}

	// Its profile key copied to another device, whose root has its own device key.
	other := t.TempDir()
	wrapped, _ := os.ReadFile(filepath.Join(root, "p1", "profile.key"))
	if err := writeNew(filepath.Join(other, "p1", "profile.key"), wrapped); err != nil {
		t.Fatal(err)
	}
	if _, err := File(filepath.Join(other, "device.key"))(true); err != nil {
		t.Fatal(err)
	}
	copied, _ := open(filepath.Join(other, "p1"), "p1", File(filepath.Join(other, "device.key")))
	if err := copied.Check(); !errors.Is(err, ErrLocked) {
		t.Fatalf("a profile copied to another device: %v", err)
	}

	// The device key file gone.
	was, _ := os.ReadFile(filepath.Join(root, "p1", "profile.key"))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	p1 = New(filepath.Join(root, "p1"), "p1", st1, device)
	if err := p1.Check(); !errors.Is(err, ErrLocked) {
		t.Fatalf("with no device key file: %v", err)
	}
	if _, err := p1.Seal("secrets", "b", "value", []byte("y")); !errors.Is(err, ErrLocked) {
		t.Fatalf("sealing with no device key file: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("a device key was made in place of the lost one while profiles held keys")
	}
	if now, _ := os.ReadFile(filepath.Join(root, "p1", "profile.key")); !bytes.Equal(now, was) {
		t.Fatal("the profile key was overwritten")
	}
	if err := p1.StartFresh(); err != nil {
		t.Fatal(err)
	}
	if now, _ := os.ReadFile(path); len(now) != 32 || bytes.Equal(now, made) {
		t.Fatalf("after starting fresh, device.key is %x", now)
	}
	if err := New(filepath.Join(root, "p1"), "p1", st1, File(path)).Check(); err != nil {
		t.Fatalf("started fresh, opened again: %v", err)
	}
	var has bool
	st1.Wait(func(tx *store.Tx) (err error) { _, has, err = p1.Secret(tx, "sync.token"); return err })
	if has {
		t.Fatal("the secret sealed with the old key is still there")
	}
}
