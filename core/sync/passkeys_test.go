package sync_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"

	"projectb/passkeys"
)

// A passkey made on one profile signs in on the other, with the key its site was given;
// the server can read nothing of it; and one deleted on either goes from both.
func TestAPasskeySyncs(t *testing.T) {
	s := newServer(t)
	a, b := newProfile(t), newProfile(t)
	a.startSync(s)
	b.startSync(s)
	ctx := context.Background()
	id, cose, err := a.passkeys.Make(ctx, true, "secret.example", passkeys.User{Handle: []byte("handle-1"), Name: "asha@secret.example", DisplayName: "Asha Rao"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.connect(s.bearer(), "tok", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	if err := b.connect(s.bearer(), "tok", "pass phrase"); err != nil {
		t.Fatal(err)
	}

	n := 0
	for _, it := range s.items("tok") {
		raw, _ := json.Marshal(it)
		if it.Collection == passkeys.Collection {
			n++
		}
		for _, secret := range []string{"secret.example", "asha", "Asha", "handle-1"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("the server can read %q: %s", secret, raw)
			}
		}
	}
	if n != 1 {
		t.Fatalf("the server holds %d passkeys, want 1", n)
	}

	// Sync details names it by its site, the one thing of it that isn't sealed.
	named := false
	for _, r := range b.details().Runs {
		for _, it := range r.ReceivedItems {
			named = named || it.Collection == passkeys.Collection && it.Name == "secret.example"
		}
	}
	if !named {
		t.Fatalf("b's runs don't name the passkey received by its site: %+v", b.details().Runs)
	}

	list, err := b.passkeys.List(ctx, true, "secret.example", nil)
	if err != nil || len(list) != 1 || !bytes.Equal(list[0].ID, id) || list[0].UserName != "asha@secret.example" || list[0].DisplayName != "Asha Rao" || string(list[0].UserHandle) != "handle-1" {
		t.Fatalf("b's passkeys: %+v, %v", list, err)
	}
	data := append(passkeys.AssertionData("secret.example", true), bytes.Repeat([]byte{3}, 32)...)
	sig, handle, err := b.passkeys.Sign(ctx, true, "secret.example", id, data)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	public := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(cose[10:42]), Y: new(big.Int).SetBytes(cose[45:])}
	if !ecdsa.VerifyASN1(public, digest[:], sig) || string(handle) != "handle-1" {
		t.Fatalf("b's signature doesn't verify with the key a's site was given (handle %q)", handle)
	}

	if err := b.passkeys.Delete(ctx, true, id); err != nil {
		t.Fatal(err)
	}
	b.settle()
	run(t, b, a)
	if n, _ := a.passkeys.Total(); n != 0 {
		t.Fatalf("a has %d passkeys after b deleted it", n)
	}
	if _, _, err := a.passkeys.Sign(ctx, true, "secret.example", id, data); !errors.Is(err, passkeys.ErrNone) {
		t.Fatalf("a signs with a passkey deleted on b: %v", err)
	}
}

// Two profiles each make a passkey for one account before either syncs: both keep both,
// as the site kept both public keys, and each lists the newer.
func TestTwoPasskeysForOneAccount(t *testing.T) {
	s := newServer(t)
	a, b := newProfile(t), newProfile(t)
	a.startSync(s)
	b.startSync(s)
	ctx := context.Background()
	user := passkeys.User{Handle: []byte("handle-1"), Name: "asha"}
	a.passkeys.Make(ctx, true, "example.com", user, nil)
	b.passkeys.Make(ctx, true, "example.com", user, nil)
	a.connect(s.bearer(), "tok", "pass phrase")
	b.connect(s.bearer(), "tok", "pass phrase")
	run(t, a)
	for name, p := range map[string]*profile{"a": a, "b": b} {
		if n, _ := p.passkeys.Total(); n != 2 {
			t.Fatalf("%s has %d passkeys, want both", name, n)
		}
		list, _ := p.passkeys.List(ctx, true, "example.com", nil)
		if len(list) != 1 {
			t.Fatalf("%s lists %d for the account, want 1", name, len(list))
		}
	}
}
