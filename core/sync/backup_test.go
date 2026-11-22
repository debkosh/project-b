package sync_test

import (
	"errors"
	"strings"
	"testing"

	"projectb/sync"
)

// A backup code opens the bucket as the passphrase does, typed any way a person types it;
// it stays when the passphrase changes; a new one takes the old one's place only once it is
// kept, and dropped it leaves the old one as it was. Checking one asks no server, and a
// passphrase is no backup code.
func TestTheBackupCode(t *testing.T) {
	s := newServer(t)
	a, b, c, d := newProfile(t), newProfile(t), newProfile(t), newProfile(t)
	for _, p := range []*profile{a, b, c, d} {
		p.startSync(s)
	}
	a.main(func() { a.bookmarks.Add("Shared", "https://example.com/") })
	a.settle()
	if err := a.connect(s.bearer(), "tok", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	if got := a.sync.Status().Backup; got != nil {
		t.Fatalf("a bucket just made has a backup code: %+v", got)
	}
	make := func() string {
		t.Helper()
		code := ""
		if err := a.call(func(done func(error)) {
			a.sync.MakeBackupCode(func(made string, err error) { code = made; done(err) })
		}); err != nil {
			t.Fatal(err)
		}
		if len(code) != 29 || strings.Count(code, "-") != 5 {
			t.Fatalf("the code made: %d characters", len(code))
		}
		return code
	}
	is := func(typed string) bool {
		t.Helper()
		ok := false
		if err := a.call(func(done func(error)) {
			a.sync.CheckBackupCode(typed, func(is bool, err error) { ok = is; done(err) })
		}); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	first := make()
	if err := a.call(a.sync.KeepBackupCode); err != nil {
		t.Fatal(err)
	}
	if got := a.sync.Status().Backup; got == nil || got.Made == 0 {
		t.Fatalf("the status after a code was kept: %+v", got)
	}
	typed := strings.ToLower(strings.ReplaceAll(first, "-", " "))
	if !is(first) || !is(typed) || is("pass phrase") || is("0000-0000-0000-0000-0000-0000") {
		t.Fatalf("checking: the code %v, as typed %v, the passphrase %v, another code %v", is(first), is(typed), is("pass phrase"), is("0000-0000-0000-0000-0000-0000"))
	}

	// A device joins with the code as typed, and sets a new passphrase: the code stays.
	if err := b.connect(s.bearer(), "tok", typed); err != nil {
		t.Fatalf("joining with the code: %v", err)
	}
	if err := b.call(func(done func(error)) { b.sync.ChangePassphrase("a new passphrase", done) }); err != nil {
		t.Fatal(err)
	}
	run(t, a, b)
	if got := b.sync.Status().Backup; got == nil || !is(first) {
		t.Fatalf("after the passphrase changed, the code: %+v, %v", got, is(first))
	}

	// A new code: the old one still opens until the new one is kept.
	second := make()
	if !is(first) || !is(second) {
		t.Fatal("before the new code is kept, both open")
	}
	if err := c.connect(s.bearer(), "tok", first); err != nil {
		t.Fatalf("the old code, before the new one is kept: %v", err)
	}
	if err := a.call(a.sync.KeepBackupCode); err != nil {
		t.Fatal(err)
	}
	if is(first) || !is(second) {
		t.Fatal("once the new code is kept, only it opens")
	}
	if err := d.connect(s.bearer(), "tok", first); !errors.Is(err, sync.ErrPassphrase) {
		t.Fatalf("the old code, after the new one was kept: %v", err)
	}
	if err := d.connect(s.bearer(), "tok", second); err != nil {
		t.Fatalf("the new code: %v", err)
	}

	// A code made and not kept leaves the one before it.
	third := make()
	if err := a.call(a.sync.DropBackupCode); err != nil {
		t.Fatal(err)
	}
	if is(third) || !is(second) {
		t.Fatal("a code dropped still opens, or the one before it doesn't")
	}
}
