package keys

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// Profile Lock (docs/profile-lock.md): a passphrase over the profile key. While it is on,
// profile.lock holds the profile key sealed with a key made from the passphrase, and that
// sealed again with the device key. Locked is no key in memory and no profile.key, so nothing
// here gets to the profile key with the device key alone.

// Mode is when Profile Lock locks by itself. Lock works in all of them.
type Mode string

const (
	// ModeManual locks only at Lock: profile.key is kept while unlocked, so a start is unlocked.
	ModeManual Mode = "manual"
	// ModeClose locks when the browser closes: nothing is kept, so every start is locked.
	ModeClose Mode = "close"
	// ModeIdle locks after Minutes without use, and when the browser closes.
	ModeIdle Mode = "idle"
)

// MinPassphrase is the fewest characters a Profile Lock passphrase has.
const MinPassphrase = 8

var (
	// ErrLocked is what sealing and opening fail with while Profile Lock holds the profile
	// locked: the passphrase opens it (Unlock).
	ErrLocked = errors.New("keys: the profile is locked")
	// ErrPassphrase is a passphrase that doesn't open profile.lock.
	ErrPassphrase = errors.New("keys: that passphrase isn't right")
	// ErrShort is a new passphrase under MinPassphrase characters.
	ErrShort = errors.New("keys: a passphrase needs at least 8 characters")
	// ErrCancelled is a wait for the unlock Dialog that ended with no passphrase: the person
	// cancelled it, or the caller took its call back.
	ErrCancelled = errors.New("keys: cancelled")
	// ErrLockOff and ErrLockOn are a call that needs Profile Lock on, or off.
	ErrLockOff = errors.New("keys: Profile Lock is off")
	ErrLockOn  = errors.New("keys: Profile Lock is on already")
)

// Lock is Profile Lock's state.
type Lock struct {
	On     bool `json:"on"`
	Locked bool `json:"locked"`
	// Mode and Minutes say when it locks by itself; Minutes counts only in ModeIdle.
	Mode    Mode `json:"mode"`
	Minutes int  `json:"minutes"`
}

// Ask is what the unlock Dialog is for: Why names the job ("passkey", "sync", "settings", …)
// for the shell to word, Host is the site it is for, if any, and Tab the tab that asked, 0
// for the window in front. Whoever starts a call for a person puts it in the call's context
// (Asking), and the features under it only pass the context on.
type Ask struct {
	Why  string
	Host string
	Tab  int
}

type askKey struct{}

// Asking is ctx saying what a call a person made is for, for the unlock Dialog.
func Asking(ctx context.Context, what Ask) context.Context {
	return context.WithValue(ctx, askKey{}, what)
}

// cost is argon2id's settings. Half a second or so on a laptop, which is a wrong passphrase's
// only brake.
type cost struct {
	time, memory uint32 // memory in KiB
	threads      uint8
}

var lockCost = cost{time: 3, memory: 64 * 1024, threads: 4}

// head is the head of profile.lock, in the clear: the mode, and what makes the passphrase's
// key. Both seals bind it, so a head that was changed opens nothing.
type head struct {
	mode    Mode
	minutes int
	cost    cost
	salt    [16]byte
}

const (
	lockFile  = "profile.lock"
	lockMagic = "PBLK\x01"
	headSize  = len(lockMagic) + 1 + 2 + 4 + 4 + 1 + 16
)

var modeBytes = map[Mode]byte{ModeManual: 0, ModeClose: 1, ModeIdle: 2}

func (h head) bytes() []byte {
	b := append([]byte(lockMagic), modeBytes[h.mode])
	b = binary.BigEndian.AppendUint16(b, uint16(h.minutes))
	b = binary.BigEndian.AppendUint32(b, h.cost.time)
	b = binary.BigEndian.AppendUint32(b, h.cost.memory)
	b = append(b, h.cost.threads)
	return append(b, h.salt[:]...)
}

func readHead(b []byte) (head, error) {
	var h head
	if len(b) < headSize || string(b[:len(lockMagic)]) != lockMagic {
		return h, errors.New("keys: profile.lock isn't one")
	}
	b = b[len(lockMagic):]
	for m, v := range modeBytes {
		if v == b[0] {
			h.mode = m
		}
	}
	if h.mode == "" {
		return h, errors.New("keys: profile.lock names no mode")
	}
	h.minutes = int(binary.BigEndian.Uint16(b[1:]))
	h.cost = cost{binary.BigEndian.Uint32(b[3:]), binary.BigEndian.Uint32(b[7:]), b[11]}
	copy(h.salt[:], b[12:])
	if h.cost.time == 0 || h.cost.memory == 0 || h.cost.threads == 0 {
		return h, errors.New("keys: profile.lock's settings are empty")
	}
	return h, nil
}

func (h head) passKey(passphrase string) []byte {
	return argon2.IDKey([]byte(passphrase), h.salt[:], h.cost.time, h.cost.memory, h.cost.threads, chacha20poly1305.KeySize)
}

// loadLock reads profile.lock's head as the keys start: Profile Lock is on when it is there.
func (k *Keys) loadLock() {
	b, err := os.ReadFile(filepath.Join(k.dir, lockFile))
	if err != nil {
		return
	}
	// One that can't be read is still a lock: nothing opens until Start fresh.
	h, _ := readHead(b)
	if h.mode == "" {
		h.mode = ModeClose
	}
	k.lock = &h
}

// lockedNow says the profile is locked. Call it with mu held.
func (k *Keys) lockedNow() bool {
	if k.lock == nil || k.key != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(k.dir, "profile.key"))
	return errors.Is(err, os.ErrNotExist)
}

func (k *Keys) lockState() Lock {
	if k.lock == nil {
		return Lock{Mode: ModeClose}
	}
	return Lock{On: true, Locked: k.lockedNow(), Mode: k.lock.mode, Minutes: k.lock.minutes}
}

// LockState is Profile Lock's state now.
func (k *Keys) LockState() Lock {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.lockState()
}

// OnLockChange has f hear the state after every change of it, on the goroutine that made
// the change.
func (k *Keys) OnLockChange(f func(Lock)) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.lockChanged = append(k.lockChanged, f)
}

// OnAsk sets who shows the unlock Dialog: ask is told what for when a call a person made
// starts to wait, and over when no call waits any more with the profile still locked, so the
// Dialog closes. Both run on the goroutine of the call that caused them.
func (k *Keys) OnAsk(ask func(Ask), over func()) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.ask, k.askOver = ask, over
}

// told tells the listeners the state, and the waiting calls err when the profile is no
// longer locked or the wait is over. Call it with mu held; it returns what to run once mu is
// released.
func (k *Keys) told(waiters error, release bool) func() {
	state := k.lockState()
	listeners := append([]func(Lock){}, k.lockChanged...)
	var waiting []chan error
	if release {
		for id, w := range k.waiting {
			waiting = append(waiting, w)
			delete(k.waiting, id)
		}
	}
	return func() {
		for _, w := range waiting {
			w <- waiters
		}
		for _, f := range listeners {
			f(state)
		}
	}
}

// Unlocked waits until the profile key is at hand: at once when Profile Lock is off or the
// profile is unlocked. While it is locked, a call nobody asked for is answered ErrLocked at
// once. One a person asked for has the unlock Dialog shown, saying what for, and waits for
// the right passphrase; it ends with ErrCancelled when the Dialog is cancelled or ctx is done,
// which is the caller taking its call back. Calls that arrive while the Dialog shows wait on
// the same one. Not on the main thread.
func (k *Keys) Unlocked(ctx context.Context, asked bool) error {
	what, _ := ctx.Value(askKey{}).(Ask)
	k.mu.Lock()
	if !k.lockedNow() {
		k.mu.Unlock()
		return nil
	}
	ask := k.ask
	if !asked || ask == nil {
		k.mu.Unlock()
		return ErrLocked
	}
	if ctx.Err() != nil {
		k.mu.Unlock()
		return ErrCancelled
	}
	if k.waiting == nil {
		k.waiting = map[int]chan error{}
	}
	k.nextWait++
	id, w := k.nextWait, make(chan error, 1)
	k.waiting[id] = w
	first := len(k.waiting) == 1
	k.mu.Unlock()
	if first {
		ask(what)
	}
	select {
	case err := <-w:
		return err
	case <-ctx.Done():
	}
	k.mu.Lock()
	_, still := k.waiting[id]
	delete(k.waiting, id)
	over := k.askOver
	none := len(k.waiting) == 0 && k.lockedNow()
	k.mu.Unlock()
	if !still {
		// Answered as it was taken back.
		return <-w
	}
	if none && over != nil {
		over()
	}
	return ErrCancelled
}

// CancelAsk answers every call waiting on the unlock Dialog ErrCancelled: the person
// cancelled it.
func (k *Keys) CancelAsk() {
	k.mu.Lock()
	then := k.told(ErrCancelled, true)
	k.mu.Unlock()
	then()
}

// TurnOnLock turns Profile Lock on with a passphrase, in ModeClose, leaving the profile
// unlocked. Not on the main thread: argon2id takes a while, and the keychain may ask.
func (k *Keys) TurnOnLock(passphrase string) error {
	if utf8.RuneCountInString(passphrase) < MinPassphrase {
		return ErrShort
	}
	k.op.Lock()
	defer k.op.Unlock()
	h := head{mode: ModeClose, minutes: 15, cost: lockCost}
	rand.Read(h.salt[:])
	pass := h.passKey(passphrase)
	k.mu.Lock()
	if k.lock != nil {
		k.mu.Unlock()
		return ErrLockOn
	}
	key, err := k.profileKey(true)
	if err == nil {
		err = k.writeLock(h, pass, key)
	}
	if err == nil {
		k.lock = &h
		err = k.keepKey()
	}
	then := k.told(nil, false)
	k.mu.Unlock()
	then()
	return err
}

// Unlock opens the profile with its passphrase: ErrPassphrase for a wrong one, ErrNotHere
// when the device key doesn't open profile.lock. Every call waiting on the unlock Dialog
// then runs. Not on the main thread.
func (k *Keys) Unlock(passphrase string) error {
	k.op.Lock()
	defer k.op.Unlock()
	_, _, err := k.proved(passphrase)
	return err
}

// LockNow locks the profile: the key in memory is overwritten and forgotten, and profile.key
// deleted. Nothing when Profile Lock is off.
func (k *Keys) LockNow() error {
	k.op.Lock()
	defer k.op.Unlock()
	k.mu.Lock()
	if k.lock == nil {
		k.mu.Unlock()
		return nil
	}
	clear(k.key)
	k.key = nil
	err := os.Remove(filepath.Join(k.dir, "profile.key"))
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	then := k.told(nil, false)
	k.mu.Unlock()
	then()
	return err
}

// SetLockMode changes when it locks by itself, which asks for the passphrase even while
// unlocked. minutes counts in ModeIdle: 5, 15, 30 or 60.
func (k *Keys) SetLockMode(passphrase string, mode Mode, minutes int) error {
	if _, ok := modeBytes[mode]; !ok {
		return fmt.Errorf("keys: no mode %q", mode)
	}
	if mode == ModeIdle && minutes != 5 && minutes != 15 && minutes != 30 && minutes != 60 {
		return fmt.Errorf("keys: %d minutes isn't offered", minutes)
	}
	k.op.Lock()
	defer k.op.Unlock()
	h, pass, err := k.proved(passphrase)
	if err != nil {
		return err
	}
	h.mode = mode
	if mode == ModeIdle {
		h.minutes = minutes
	}
	return k.rewrite(h, pass)
}

// ChangeLockPassphrase puts a new passphrase over the profile, given the current one.
func (k *Keys) ChangeLockPassphrase(current, passphrase string) error {
	if utf8.RuneCountInString(passphrase) < MinPassphrase {
		return ErrShort
	}
	k.op.Lock()
	defer k.op.Unlock()
	h, _, err := k.proved(current)
	if err != nil {
		return err
	}
	h.cost = lockCost
	rand.Read(h.salt[:])
	return k.rewrite(h, h.passKey(passphrase))
}

// TurnOffLock turns Profile Lock off, given the passphrase: profile.key is written and
// profile.lock deleted.
func (k *Keys) TurnOffLock(passphrase string) error {
	k.op.Lock()
	defer k.op.Unlock()
	if _, _, err := k.proved(passphrase); err != nil {
		return err
	}
	k.mu.Lock()
	k.lock = nil
	err := k.keepKey()
	if err == nil {
		err = os.Remove(filepath.Join(k.dir, lockFile))
	}
	if err != nil {
		k.loadLock()
	}
	then := k.told(nil, false)
	k.mu.Unlock()
	then()
	return err
}

// proved opens profile.lock with the passphrase and leaves the profile unlocked, as whoever
// gave it has shown they may open it: the head, the passphrase's key, and the waiting calls
// run. Call it with op held and mu not.
func (k *Keys) proved(passphrase string) (head, []byte, error) {
	k.mu.Lock()
	on := k.lock != nil
	k.mu.Unlock()
	if !on {
		return head{}, nil, ErrLockOff
	}
	b, err := os.ReadFile(filepath.Join(k.dir, lockFile))
	if err != nil {
		return head{}, nil, err
	}
	h, err := readHead(b)
	if err != nil {
		return head{}, nil, err
	}
	wrap, err := k.device(false)
	if errors.Is(err, ErrNoDeviceKey) {
		return head{}, nil, ErrNotHere
	}
	if err != nil {
		return head{}, nil, fmt.Errorf("keys: the device key: %w", err)
	}
	inner, err := open(wrap, b[headSize:], k.lockBinds("device", h))
	if err != nil {
		return head{}, nil, ErrNotHere
	}
	pass := h.passKey(passphrase)
	key, err := open(pass, inner, k.lockBinds("passphrase", h))
	if err != nil || len(key) != chacha20poly1305.KeySize {
		return head{}, nil, ErrPassphrase
	}
	k.mu.Lock()
	if k.key == nil {
		k.key = key
	}
	err = k.keepKey()
	then := k.told(nil, true)
	k.mu.Unlock()
	then()
	return h, pass, err
}

// rewrite writes profile.lock again with a new head or a new passphrase's key, and keeps or
// drops profile.key as the mode has it. The profile is unlocked: proved ran.
func (k *Keys) rewrite(h head, pass []byte) error {
	k.mu.Lock()
	err := k.writeLock(h, pass, k.key)
	if err == nil {
		k.lock = &h
		err = k.keepKey()
	}
	then := k.told(nil, false)
	k.mu.Unlock()
	then()
	return err
}

// keepKey has profile.key there or not, as the mode has it while unlocked: there with
// Profile Lock off and in ModeManual, gone in the others. Call it with mu held and the key
// in memory.
func (k *Keys) keepKey() error {
	path := filepath.Join(k.dir, "profile.key")
	if k.lock != nil && k.lock.mode != ModeManual {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	wrap, err := k.device(false)
	if err != nil {
		return fmt.Errorf("keys: the device key: %w", err)
	}
	wrapped, err := seal(wrap, k.key, k.wrapping())
	if err != nil {
		return err
	}
	return writeNew(path, wrapped)
}

// lockBinds is what a seal of profile.lock binds: which layer it is, the profile's id, and the
// head.
func (k *Keys) lockBinds(layer string, h head) []byte {
	return bound("profile lock", layer, k.profile, string(h.bytes()))
}

// writeLock writes profile.lock, in place of one that is there. Call it with mu held.
func (k *Keys) writeLock(h head, pass, key []byte) error {
	wrap, err := k.device(false)
	if err != nil {
		return fmt.Errorf("keys: the device key: %w", err)
	}
	inner, err := seal(pass, key, k.lockBinds("passphrase", h))
	if err != nil {
		return err
	}
	outer, err := seal(wrap, inner, k.lockBinds("device", h))
	if err != nil {
		return err
	}
	path := filepath.Join(k.dir, lockFile)
	next := path + ".new"
	os.Remove(next)
	if err := writeNew(next, append(h.bytes(), outer...)); err != nil {
		return err
	}
	return os.Rename(next, path)
}
