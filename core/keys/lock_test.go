package keys

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"projectb/store"
)

// waitingNow is the calls waiting on the unlock Dialog.
func (k *Keys) waitingNow() []int {
	k.mu.Lock()
	defer k.mu.Unlock()
	var ids []int
	for id := range k.waiting {
		ids = append(ids, id)
	}
	return ids
}

// The tests make a passphrase's key cheaply: argon2id's real cost is half a second a try.
func init() { lockCost = cost{time: 1, memory: 64, threads: 1} }

const pass = "river meadow lantern"

// has says a file of the profile folder is there.
func has(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// locked is a profile with something sealed, Profile Lock on, as a start finds it: the keys
// opened again over the folder, with the same device key.
func locked(t *testing.T, mode Mode) (dir string, d *device, k *Keys, sealed []byte) {
	t.Helper()
	dir, d = t.TempDir(), &device{}
	st := openStore(t, dir)
	first := New(dir, "p1", st, d.get)
	sealed, err := first.Seal("logins", "row1", "password", []byte("hunter2"))
	if err != nil {
		t.Fatal(err)
	}
	if err := first.TurnOnLock(pass); err != nil {
		t.Fatal(err)
	}
	if mode != ModeClose {
		if err := first.SetLockMode(pass, mode, 15); err != nil {
			t.Fatal(err)
		}
	}
	return dir, d, New(dir, "p1", st, d.get), sealed
}

// Turning it on writes profile.lock and, in the mode it starts in, drops profile.key: the
// profile stays unlocked until the browser closes, and the next start is locked.
func TestTurningOnLocksAtTheNextStart(t *testing.T) {
	dir, d := t.TempDir(), &device{}
	st := openStore(t, dir)
	k := New(dir, "p1", st, d.get)
	sealed, err := k.Seal("logins", "row1", "password", []byte("hunter2"))
	if err != nil {
		t.Fatal(err)
	}
	if s := k.LockState(); s.On || s.Locked {
		t.Fatalf("before: %+v", s)
	}
	if err := k.TurnOnLock("short"); !errors.Is(err, ErrShort) {
		t.Fatalf("a short passphrase: %v", err)
	}
	var heard []Lock
	k.OnLockChange(func(s Lock) { heard = append(heard, s) })
	if err := k.TurnOnLock(pass); err != nil {
		t.Fatal(err)
	}
	if s := k.LockState(); !s.On || s.Locked || s.Mode != ModeClose {
		t.Fatalf("turned on: %+v", s)
	}
	if len(heard) != 1 || !heard[0].On {
		t.Fatalf("the change was heard as %+v", heard)
	}
	if !has(dir, "profile.lock") || has(dir, "profile.key") {
		t.Fatalf("the files: profile.lock %v, profile.key %v", has(dir, "profile.lock"), has(dir, "profile.key"))
	}
	if got, err := k.Open("logins", "row1", "password", sealed); err != nil || string(got) != "hunter2" {
		t.Fatalf("still unlocked: %q, %v", got, err)
	}
	if err := k.TurnOnLock(pass); !errors.Is(err, ErrLockOn) {
		t.Fatalf("turning it on twice: %v", err)
	}

	// The next start: nothing opens with the device key alone.
	k = New(dir, "p1", st, d.get)
	if s := k.LockState(); !s.On || !s.Locked {
		t.Fatalf("after a start: %+v", s)
	}
	if _, err := k.Open("logins", "row1", "password", sealed); !errors.Is(err, ErrLocked) {
		t.Fatalf("opening while locked: %v", err)
	}
	if _, err := k.Seal("logins", "row2", "password", []byte("x")); !errors.Is(err, ErrLocked) {
		t.Fatalf("sealing while locked: %v", err)
	}
	if err := k.Check(); !errors.Is(err, ErrLocked) {
		t.Fatalf("Check while locked: %v", err)
	}
	if has(dir, "profile.key") {
		t.Fatal("sealing while locked made a new profile.key")
	}
	if err := k.Unlock("river meadow lanterm"); !errors.Is(err, ErrPassphrase) {
		t.Fatalf("a wrong passphrase: %v", err)
	}
	if s := k.LockState(); !s.Locked {
		t.Fatal("a wrong passphrase unlocked it")
	}
	if err := k.Unlock(pass); err != nil {
		t.Fatal(err)
	}
	if got, err := k.Open("logins", "row1", "password", sealed); err != nil || string(got) != "hunter2" {
		t.Fatalf("unlocked: %q, %v", got, err)
	}
	if has(dir, "profile.key") {
		t.Fatal("unlocking in this mode kept profile.key")
	}
}

// Each mode's files while unlocked, after Lock, and at the next start.
func TestTheModesFiles(t *testing.T) {
	for _, c := range []struct {
		mode        Mode
		kept, start bool // profile.key while unlocked; the next start is unlocked
	}{{ModeManual, true, true}, {ModeClose, false, false}, {ModeIdle, false, false}} {
		dir, d, k, sealed := locked(t, c.mode)
		st := k.st
		if s := k.LockState(); s.Mode != c.mode || s.Locked == c.start {
			t.Fatalf("%s at a start: %+v", c.mode, s)
		}
		if err := k.Unlock(pass); err != nil {
			t.Fatalf("%s: %v", c.mode, err)
		}
		if has(dir, "profile.key") != c.kept {
			t.Fatalf("%s unlocked: profile.key %v", c.mode, has(dir, "profile.key"))
		}
		before, _ := os.ReadFile(filepath.Join(dir, "profile.lock"))
		if err := k.LockNow(); err != nil {
			t.Fatal(err)
		}
		if s := k.LockState(); !s.Locked || has(dir, "profile.key") {
			t.Fatalf("%s after Lock: %+v, profile.key %v", c.mode, s, has(dir, "profile.key"))
		}
		if _, err := k.Open("logins", "row1", "password", sealed); !errors.Is(err, ErrLocked) {
			t.Fatalf("%s: opening after Lock: %v", c.mode, err)
		}
		if after, _ := os.ReadFile(filepath.Join(dir, "profile.lock")); string(after) != string(before) {
			t.Fatalf("%s: profile.lock changed at a lock", c.mode)
		}
		// A start after Lock is locked in every mode.
		if s := New(dir, "p1", st, d.get).LockState(); !s.Locked {
			t.Fatalf("%s: a start after Lock: %+v", c.mode, s)
		}
		if err := k.Unlock(pass); err != nil {
			t.Fatal(err)
		}
		again := New(dir, "p1", st, d.get)
		if s := again.LockState(); s.Locked == c.start {
			t.Fatalf("%s: a start after an unlock: %+v", c.mode, s)
		}
		if got, err := again.Open("logins", "row1", "password", sealed); (err == nil) != c.start || c.start && string(got) != "hunter2" {
			t.Fatalf("%s: opening at that start: %q, %v", c.mode, got, err)
		}
	}
}

// Changing the mode or the passphrase, and turning it off, each need the passphrase, even
// while unlocked.
func TestChangesNeedThePassphrase(t *testing.T) {
	dir, d, k, sealed := locked(t, ModeClose)
	if err := k.Unlock(pass); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(string) error{
		"mode":     func(p string) error { return k.SetLockMode(p, ModeIdle, 30) },
		"change":   func(p string) error { return k.ChangeLockPassphrase(p, "copper orbit maple") },
		"turn off": k.TurnOffLock,
	} {
		if err := change("not the passphrase"); !errors.Is(err, ErrPassphrase) {
			t.Fatalf("%s with a wrong passphrase: %v", name, err)
		}
	}
	if s := k.LockState(); !s.On || s.Mode != ModeClose {
		t.Fatalf("the wrong ones changed it: %+v", s)
	}
	if err := k.SetLockMode(pass, ModeIdle, 7); err == nil {
		t.Fatal("7 minutes was taken")
	}
	if err := k.SetLockMode(pass, ModeIdle, 30); err != nil {
		t.Fatal(err)
	}
	if s := k.LockState(); s.Mode != ModeIdle || s.Minutes != 30 {
		t.Fatalf("the mode: %+v", s)
	}
	if err := k.SetLockMode(pass, ModeManual, 0); err != nil || !has(dir, "profile.key") {
		t.Fatalf("to when I lock it: %v, profile.key %v", err, has(dir, "profile.key"))
	}
	if err := k.ChangeLockPassphrase(pass, "short"); !errors.Is(err, ErrShort) {
		t.Fatalf("a short new passphrase: %v", err)
	}
	if err := k.ChangeLockPassphrase(pass, "copper orbit maple"); err != nil {
		t.Fatal(err)
	}
	if s := k.LockState(); s.Mode != ModeManual {
		t.Fatalf("a new passphrase changed the mode: %+v", s)
	}
	k.LockNow()
	if err := k.Unlock(pass); !errors.Is(err, ErrPassphrase) {
		t.Fatalf("the old passphrase after a change: %v", err)
	}
	if err := k.Unlock("copper orbit maple"); err != nil {
		t.Fatal(err)
	}
	// A mode changed while locked unlocks: whoever gave the passphrase may open it.
	k.LockNow()
	if err := k.SetLockMode("copper orbit maple", ModeClose, 0); err != nil || k.LockState().Locked || has(dir, "profile.key") {
		t.Fatalf("the mode changed while locked: %v, %+v", err, k.LockState())
	}
	if err := k.TurnOffLock("copper orbit maple"); err != nil {
		t.Fatal(err)
	}
	if s := k.LockState(); s.On || has(dir, "profile.lock") || !has(dir, "profile.key") {
		t.Fatalf("turned off: %+v, profile.lock %v, profile.key %v", s, has(dir, "profile.lock"), has(dir, "profile.key"))
	}
	if got, err := New(dir, "p1", k.st, d.get).Open("logins", "row1", "password", sealed); err != nil || string(got) != "hunter2" {
		t.Fatalf("after turning it off, at a start: %q, %v", got, err)
	}
	if err := k.Unlock(pass); !errors.Is(err, ErrLockOff) {
		t.Fatalf("Unlock with it off: %v", err)
	}
}

// A call a person made waits on the unlock Dialog, several on one; one nobody asked for is
// answered locked at once and never raises it.
func TestCallsWaitOnOneDialog(t *testing.T) {
	_, _, k, _ := locked(t, ModeClose)
	asks, overs := make(chan Ask, 8), make(chan bool, 8)
	k.OnAsk(func(a Ask) { asks <- a }, func() { overs <- true })
	if err := k.Unlocked(context.Background(), false); !errors.Is(err, ErrLocked) {
		t.Fatalf("a call nobody asked for: %v", err)
	}
	select {
	case a := <-asks:
		t.Fatalf("it raised the Dialog: %+v", a)
	default:
	}
	done := make(chan error, 3)
	ctx := Asking(context.Background(), Ask{Why: "passkey", Host: "shop.example.com", Tab: 7})
	go func() { done <- k.Unlocked(ctx, true) }()
	if a := <-asks; a.Why != "passkey" || a.Host != "shop.example.com" || a.Tab != 7 {
		t.Fatalf("what the Dialog is for: %+v", a)
	}
	for range 2 {
		go func() { done <- k.Unlocked(context.Background(), true) }()
	}
	for len(k.waitingNow()) < 3 {
		time.Sleep(time.Millisecond)
	}
	select {
	case a := <-asks:
		t.Fatalf("a second Dialog: %+v", a)
	default:
	}
	if err := k.Unlock("wrong one here"); !errors.Is(err, ErrPassphrase) {
		t.Fatal(err)
	}
	if n := len(k.waitingNow()); n != 3 {
		t.Fatalf("a wrong passphrase left %d waiting", n)
	}
	if err := k.Unlock(pass); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := <-done; err != nil {
			t.Fatalf("a waiting call after the unlock: %v", err)
		}
	}
	if err := k.Unlocked(context.Background(), false); err != nil {
		t.Fatalf("unlocked: %v", err)
	}
}

// Cancel answers every waiting call cancelled; a call taken back is answered cancelled, and
// the Dialog is told it is over when none waits any more.
func TestCancelAndCallsTakenBack(t *testing.T) {
	_, _, k, _ := locked(t, ModeClose)
	asks, overs := make(chan Ask, 8), make(chan bool, 8)
	k.OnAsk(func(a Ask) { asks <- a }, func() { overs <- true })
	done := make(chan error, 2)
	for range 2 {
		go func() { done <- k.Unlocked(context.Background(), true) }()
	}
	<-asks
	for len(k.waitingNow()) < 2 {
		time.Sleep(time.Millisecond)
	}
	k.CancelAsk()
	for range 2 {
		if err := <-done; !errors.Is(err, ErrCancelled) {
			t.Fatalf("a call after Cancel: %v", err)
		}
	}
	if !k.LockState().Locked {
		t.Fatal("Cancel unlocked it")
	}

	first, back := context.WithCancel(context.Background())
	second, back2 := context.WithCancel(context.Background())
	go func() { done <- k.Unlocked(first, true) }()
	<-asks
	go func() { done <- k.Unlocked(second, true) }()
	for len(k.waitingNow()) < 2 {
		time.Sleep(time.Millisecond)
	}
	back()
	if err := <-done; !errors.Is(err, ErrCancelled) {
		t.Fatalf("a call taken back: %v", err)
	}
	select {
	case <-overs:
		t.Fatal("the Dialog was closed while a call still waits")
	case <-time.After(20 * time.Millisecond):
	}
	back2()
	if err := <-done; !errors.Is(err, ErrCancelled) {
		t.Fatalf("the second call taken back: %v", err)
	}
	select {
	case <-overs:
	case <-time.After(time.Second):
		t.Fatal("the Dialog wasn't told it is over")
	}
	// A context already done never raises it.
	if err := k.Unlocked(first, true); !errors.Is(err, ErrCancelled) {
		t.Fatalf("a call made already taken back: %v", err)
	}
}

// A folder copied to another device doesn't open there, even with the passphrase; and a
// passphrase forgotten is Start fresh: what was sealed is gone, and Profile Lock is off.
func TestACopiedFolderAndStartFresh(t *testing.T) {
	dir, _, k, sealed := locked(t, ModeManual)
	if err := k.Unlock(pass); err != nil {
		t.Fatal(err)
	}
	other := &device{key: random32()}
	there := New(dir, "p1", k.st, other.get)
	if err := there.Unlock(pass); !errors.Is(err, ErrNotHere) {
		t.Fatalf("the passphrase on another device: %v", err)
	}
	if _, err := there.Open("logins", "row1", "password", sealed); !errors.Is(err, ErrNotHere) {
		t.Fatalf("opening there: %v", err)
	}

	k.LockNow()
	gone := false
	k.OnStartFresh(func(tx *store.Tx) error { gone = true; return nil })
	waited := make(chan error, 1)
	k.OnAsk(func(Ask) {}, func() {})
	go func() { waited <- k.Unlocked(context.Background(), true) }()
	for len(k.waitingNow()) < 1 {
		time.Sleep(time.Millisecond)
	}
	if err := k.StartFresh(); err != nil {
		t.Fatal(err)
	}
	if err := <-waited; !errors.Is(err, ErrCancelled) {
		t.Fatalf("a call waiting through Start fresh: %v", err)
	}
	if s := k.LockState(); s.On || has(dir, "profile.lock") || !has(dir, "profile.key") || !gone {
		t.Fatalf("after Start fresh: %+v, profile.lock %v, profile.key %v, the features' rows deleted %v", s, has(dir, "profile.lock"), has(dir, "profile.key"), gone)
	}
	if _, err := k.Open("logins", "row1", "password", sealed); err == nil {
		t.Fatal("what was sealed before still opens")
	}
	if _, err := k.Seal("logins", "row2", "password", []byte("new")); err != nil {
		t.Fatalf("sealing after Start fresh: %v", err)
	}
}
