package passkeys

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"projectb/keys"
	"projectb/store"
)

// checks is a device check for a test: what it was asked, and what it answers.
type checks struct {
	asked  []string
	answer error
	// wait, when set, is what the check waits on before it answers, as a person would.
	wait chan struct{}
}

func (c *checks) verify(ctx context.Context, why Why, rpID string) error {
	c.asked = append(c.asked, map[Why]string{ToSave: "save ", ToUse: "use "}[why]+rpID)
	if c.wait != nil {
		select {
		case <-c.wait:
		case <-ctx.Done():
			return ErrCancelled
		}
	}
	return c.answer
}

// profile is a profile's passkeys over a store and keys of its own, in a folder named
// outright, so no keychain is asked.
func profile(t *testing.T) (*Manager, *checks, *store.Store) {
	t.Helper()
	m, c, st, _ := profileIn(t, t.TempDir())
	return m, c, st
}

func profileIn(t *testing.T, dir string) (*Manager, *checks, *store.Store, *keys.Keys) {
	t.Helper()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	c := &checks{}
	k := keys.New(dir, "", st, nil)
	return New(st, k, c.verify), c, st, k
}

var ctx = context.Background()

// public is the P-256 key a COSE key holds, checked field by field.
func public(t *testing.T, cose []byte) *ecdsa.PublicKey {
	t.Helper()
	head := []byte{0xa5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}
	if len(cose) != 77 || !bytes.Equal(cose[:10], head) || !bytes.Equal(cose[42:45], []byte{0x22, 0x58, 0x20}) {
		t.Fatalf("not an ES256 COSE key: %x", cose)
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(cose[10:42]), Y: new(big.Int).SetBytes(cose[45:])}
}

// A passkey made signs what its public key verifies, after the device's check each time,
// and gives back its user handle.
func TestMakeThenSign(t *testing.T) {
	m, c, _ := profile(t)
	id, cose, err := m.Make(ctx, true, "example.com", User{Handle: []byte("u1"), Name: "asha@example.com", DisplayName: "Asha Rao"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 16 {
		t.Fatalf("the credential ID is %d bytes", len(id))
	}
	data := append(AssertionData("example.com", false), bytes.Repeat([]byte{7}, 32)...)
	sig, handle, err := m.Sign(ctx, true, "example.com", id, data)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if !ecdsa.VerifyASN1(public(t, cose), digest[:], sig) {
		t.Fatal("the signature doesn't verify with the public key Make gave")
	}
	if string(handle) != "u1" {
		t.Fatalf("user handle %q", handle)
	}
	if want := []string{"save example.com", "use example.com"}; len(c.asked) != 2 || c.asked[0] != want[0] || c.asked[1] != want[1] {
		t.Fatalf("the device checked %v, want %v", c.asked, want)
	}
}

// Nothing is made or signed when the person doesn't pass the check, or the device has
// nothing to check with.
func TestNoCheckNoPasskey(t *testing.T) {
	m, c, _ := profile(t)
	id, _, err := m.Make(ctx, true, "example.com", User{Handle: []byte("u1")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, answer := range []error{ErrDenied, ErrNoScreenLock} {
		c.answer = answer
		if _, _, err := m.Make(ctx, true, "example.com", User{Handle: []byte("u2")}, nil); !errors.Is(err, answer) {
			t.Fatalf("Make after %v: %v", answer, err)
		}
		if sig, _, err := m.Sign(ctx, true, "example.com", id, []byte("x")); !errors.Is(err, answer) || sig != nil {
			t.Fatalf("Sign after %v: %v, signature %x", answer, err, sig)
		}
	}
	if n, _ := m.Count("example.com", nil); n != 1 {
		t.Fatalf("%d passkeys after two refused makes, want the first alone", n)
	}
}

// A call waiting on the person ends when its caller takes it back.
func TestCancelDuringTheCheck(t *testing.T) {
	m, c, _ := profile(t)
	c.wait = make(chan struct{})
	mine, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		_, _, err := m.Make(mine, true, "example.com", User{Handle: []byte("u1")}, nil)
		done <- err
	}()
	cancel()
	if err := <-done; !errors.Is(err, ErrCancelled) {
		t.Fatalf("Make, cancelled: %v", err)
	}
	if n, _ := m.Total(); n != 0 {
		t.Fatalf("%d passkeys after a cancelled make", n)
	}
}

// A passkey signs only for its own site.
func TestSignOnlyForItsSite(t *testing.T) {
	m, c, _ := profile(t)
	id, _, _ := m.Make(ctx, true, "example.com", User{Handle: []byte("u1")}, nil)
	if _, _, err := m.Sign(ctx, true, "other.example", id, []byte("x")); !errors.Is(err, ErrNone) {
		t.Fatalf("Sign for another site: %v", err)
	}
	c.asked = nil
	if _, _, err := m.Sign(ctx, true, "example.com", []byte("no such id"), []byte("x")); !errors.Is(err, ErrNone) {
		t.Fatalf("Sign with an id nothing has: %v", err)
	}
	if len(c.asked) != 0 {
		t.Fatalf("the device checked %v for a passkey that isn't here", c.asked)
	}
}

// A second passkey for one account takes the first's place; another account's stays. List
// has the site's, the newest first, and only those the page allows when it names any.
func TestReplaceListAndCount(t *testing.T) {
	m, _, _ := profile(t)
	asha := User{Handle: []byte("u1"), Name: "asha@example.com", DisplayName: "Asha Rao"}
	first, _, _ := m.Make(ctx, true, "example.com", asha, nil)
	work, _, _ := m.Make(ctx, true, "example.com", User{Handle: []byte("u2"), Name: "asha.work@example.com"}, nil)
	m.Make(ctx, true, "other.example", asha, nil)
	second, _, err := m.Make(ctx, true, "example.com", asha, nil)
	if err != nil {
		t.Fatal(err)
	}
	list, err := m.List(ctx, true, "example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("example.com has %d passkeys, want 2: %+v", len(list), list)
	}
	ids := map[string]Passkey{}
	for _, p := range list {
		ids[string(p.ID)] = p
	}
	if _, kept := ids[string(first)]; kept {
		t.Fatal("the replaced passkey is still listed")
	}
	if p := ids[string(second)]; p.UserName != "asha@example.com" || p.DisplayName != "Asha Rao" || string(p.UserHandle) != "u1" || p.RPID != "example.com" || p.Created == 0 {
		t.Fatalf("the new passkey reads %+v", p)
	}
	if n, _ := m.Count("example.com", nil); n != 2 {
		t.Fatalf("Count %d, want 2", n)
	}
	if n, _ := m.Count("example.com", [][]byte{work, []byte("elsewhere")}); n != 1 {
		t.Fatalf("Count of the allowed %d, want 1", n)
	}
	only, _ := m.List(ctx, true, "example.com", [][]byte{work})
	if len(only) != 1 || !bytes.Equal(only[0].ID, work) {
		t.Fatalf("List of the allowed: %+v", only)
	}
	if n, _ := m.Total(); n != 3 {
		t.Fatalf("Total %d, want 3", n)
	}
	all, _ := m.All(ctx, true)
	if len(all) != 3 || all[0].RPID != "example.com" || all[2].RPID != "other.example" {
		t.Fatalf("All: %+v", all)
	}
}

// A passkey the site says it already has, kept here, stops a make before the person is
// asked anything.
func TestExcluded(t *testing.T) {
	m, c, _ := profile(t)
	id, _, _ := m.Make(ctx, true, "example.com", User{Handle: []byte("u1")}, nil)
	c.asked = nil
	if _, _, err := m.Make(ctx, true, "example.com", User{Handle: []byte("u1")}, [][]byte{[]byte("a key's"), id}); !errors.Is(err, ErrExcluded) {
		t.Fatalf("Make with ours excluded: %v", err)
	}
	if len(c.asked) != 0 {
		t.Fatalf("the device checked %v for an excluded make", c.asked)
	}
	// Another site's passkey of that id excludes nothing here.
	if _, _, err := m.Make(ctx, true, "other.example", User{Handle: []byte("u1")}, [][]byte{id}); err != nil {
		t.Fatalf("Make at another site: %v", err)
	}
}

// Delete takes one away, and says so.
func TestDelete(t *testing.T) {
	m, _, _ := profile(t)
	changed := 0
	m.OnChanged(func() { changed++ })
	id, _, _ := m.Make(ctx, true, "example.com", User{Handle: []byte("u1")}, nil)
	if err := m.Delete(ctx, true, id); err != nil {
		t.Fatal(err)
	}
	if n, _ := m.Total(); n != 0 || changed != 2 {
		t.Fatalf("%d passkeys after the delete, changed %d times", n, changed)
	}
	if _, _, err := m.Sign(ctx, true, "example.com", id, []byte("x")); !errors.Is(err, ErrNone) {
		t.Fatalf("Sign with a deleted passkey: %v", err)
	}
}

// On disk the site is plain and nothing else of the person is: not the names, the user
// handle or the key. A sealed value doesn't open in another row, nor for another site.
func TestWhatTheDiskHolds(t *testing.T) {
	m, _, st := profile(t)
	id, _, _ := m.Make(ctx, true, "example.com", User{Handle: []byte("handle-u1"), Name: "asha@example.com", DisplayName: "Asha Rao"}, nil)
	other, _, _ := m.Make(ctx, true, "example.com", User{Handle: []byte("handle-u2")}, nil)
	var rp string
	var handle, user, display, key []byte
	if err := st.Wait(func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT rp_id, user_handle, user_name, display_name, key FROM passkeys WHERE id = ?`, name(id)).Scan(&rp, &handle, &user, &display, &key)
	}); err != nil {
		t.Fatal(err)
	}
	if rp != "example.com" {
		t.Fatalf("rp_id %q", rp)
	}
	for column, sealed := range map[string][]byte{"user_handle": handle, "user_name": user, "display_name": display} {
		for _, plain := range []string{"handle-u1", "asha@example.com", "Asha Rao"} {
			if bytes.Contains(sealed, []byte(plain)) {
				t.Fatalf("%s holds %q in the clear", column, plain)
			}
		}
	}
	if got, err := m.open(name(id), "example.com", "user_name", user); err != nil || string(got) != "asha@example.com" {
		t.Fatalf("user_name opens as %q, %v", got, err)
	}
	if _, err := m.open(name(other), "example.com", "user_name", user); err == nil {
		t.Fatal("a sealed name opens in another row")
	}
	if _, err := m.open(name(id), "other.example", "key", key); err == nil {
		t.Fatal("a sealed key opens for another site")
	}
	if _, err := m.open(name(id), "example.com", "user_handle", key); err == nil {
		t.Fatal("a sealed key opens as another column")
	}
}

// The authenticator data, byte for byte (WebAuthn, 6.1): the site's hash, the flags, a
// count of 0, and for a new passkey our AAGUID, its id's length and id, and its key.
func TestAuthenticatorData(t *testing.T) {
	hash := sha256.Sum256([]byte("example.com"))
	get := AssertionData("example.com", false)
	if len(get) != 37 || !bytes.Equal(get[:32], hash[:]) || get[32] != 0x0d || !bytes.Equal(get[33:], []byte{0, 0, 0, 0}) {
		t.Fatalf("a sign-in's data: %x", get)
	}
	if synced := AssertionData("example.com", true); synced[32] != 0x1d {
		t.Fatalf("flags with sync on: %#x, want 0x1d", synced[32])
	}
	id, cose := bytes.Repeat([]byte{9}, 16), bytes.Repeat([]byte{5}, 77)
	made := AttestedData("example.com", id, cose, true)
	if made[32] != 0x5d {
		t.Fatalf("a new passkey's flags: %#x, want 0x5d", made[32])
	}
	want := append(append([]byte{}, AAGUID[:]...), 0, 16)
	want = append(append(want, id...), cose...)
	if !bytes.Equal(made[37:], want) {
		t.Fatalf("the credential's data: %x", made[37:])
	}
	if got := "81dc80639ac34b1fab009e8795c47c60"; got != hexOf(AAGUID[:]) {
		t.Fatalf("AAGUID %s", hexOf(AAGUID[:]))
	}
}

func hexOf(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, 2*len(b))
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&15])
	}
	return string(out)
}

// Of two passkeys two devices made for one account while apart, List shows the newer;
// the older stays, where Settings can delete it.
func TestTwoForOneAccount(t *testing.T) {
	m, _, st := profile(t)
	older := row{Passkey: Passkey{ID: []byte("older-credential"), RPID: "example.com", UserHandle: []byte("u1"), UserName: "asha", Created: 100}, key: []byte("k")}
	newer := row{Passkey: Passkey{ID: []byte("newer-credential"), RPID: "example.com", UserHandle: []byte("u1"), UserName: "asha", Created: 200}, key: []byte("k")}
	if err := st.Wait(func(tx *store.Tx) error {
		if err := m.put(tx, older); err != nil {
			return err
		}
		return m.put(tx, newer)
	}); err != nil {
		t.Fatal(err)
	}
	list, _ := m.List(ctx, true, "example.com", nil)
	if len(list) != 1 || string(list[0].ID) != "newer-credential" {
		t.Fatalf("List: %+v", list)
	}
	if all, _ := m.All(ctx, true); len(all) != 2 {
		t.Fatalf("All has %d, want both", len(all))
	}
}

// A profile whose key doesn't open on this device, as a folder from another: every call
// that needs the key says so, with the person asked nothing, while Count still counts.
// Starting fresh deletes the passkeys with the key, and the profile makes new ones.
func TestAKeyThatDoesntOpenHere(t *testing.T) {
	dir := t.TempDir()
	m, c, st, k := profileIn(t, dir)
	id, _, err := m.Make(ctx, true, "example.com", User{Handle: []byte("u1")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The folder's own key is another's now.
	if err := os.WriteFile(filepath.Join(dir, "folder.key"), bytes.Repeat([]byte{1}, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	// Keys made anew, as a start makes them, with nothing opened yet.
	k = keys.New(dir, "", st, nil)
	m = New(st, k, c.verify)
	c.asked = nil
	if _, err := m.List(ctx, true, "example.com", nil); !errors.Is(err, ErrNotHere) {
		t.Fatalf("List: %v", err)
	}
	if _, err := m.All(ctx, true); !errors.Is(err, ErrNotHere) {
		t.Fatalf("All: %v", err)
	}
	if _, _, err := m.Make(ctx, true, "example.com", User{Handle: []byte("u2")}, nil); !errors.Is(err, ErrNotHere) {
		t.Fatalf("Make: %v", err)
	}
	if _, _, err := m.Sign(ctx, true, "example.com", id, []byte("x")); !errors.Is(err, ErrNotHere) {
		t.Fatalf("Sign: %v", err)
	}
	if len(c.asked) != 0 {
		t.Fatalf("the device checked %v with a key that doesn't open", c.asked)
	}
	if n, _ := m.Count("example.com", nil); n != 1 {
		t.Fatalf("Count %d, want the row that is still there", n)
	}
	if err := k.StartFresh(); err != nil {
		t.Fatal(err)
	}
	if n, _ := m.Total(); n != 0 {
		t.Fatalf("%d passkeys after starting fresh", n)
	}
	if _, _, err := m.Make(ctx, true, "example.com", User{Handle: []byte("u1")}, nil); err != nil {
		t.Fatalf("Make after starting fresh: %v", err)
	}
	if list, err := m.List(ctx, true, "example.com", nil); err != nil || len(list) != 1 {
		t.Fatalf("List after starting fresh: %v, %v", list, err)
	}
}
