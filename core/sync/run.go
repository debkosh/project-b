package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	gosync "sync"
	"time"

	"github.com/sudiptadeb/synckosh"
	"github.com/sudiptadeb/synckosh/client"

	"projectb/keys"
	"projectb/store"
	"projectb/timeline"
)

// How often the runs come (docs/sync.md): the marks are looked for every look, a run comes
// at least every while, and a failed one waits from firstWait, twice as long each time, up to
// longestWait.
var (
	look        = 10 * time.Second
	every       = 5 * time.Minute
	firstWait   = 30 * time.Second
	longestWait = 30 * time.Minute
	runFor      = 5 * time.Minute
)

// httpClient carries the requests of a Manager made from now on.
var httpClient = &http.Client{Timeout: 2 * time.Minute}

// The secrets sync keeps, sealed (core/keys).
const (
	secretAddress = "sync.address"
	secretToken   = "sync.token"
	secretKey     = "sync.key"
)

// Status is what the settings page and the shells show of sync.
type Status struct {
	On      bool   `json:"on"`
	Syncing bool   `json:"syncing"`
	Host    string `json:"host"`
	// LastSynced is when a run last ended well, Unix milliseconds, 0 for never.
	LastSynced int64  `json:"lastSynced"`
	Error      string `json:"error"`
	// Problem names the error for the page (Problem): "token" when the server signed this
	// device out, "unreachable" when it couldn't be reached.
	Problem string `json:"problem"`
	// Locked says the profile's key doesn't open with this device's key, so sync is off
	// until StartFresh.
	Locked bool `json:"locked"`
}

// Manager syncs one profile. Everything it does runs on a goroutine of its own, one thing at
// a time; the main thread only asks, and hears back through post.
type Manager struct {
	st   *store.Store
	keys *keys.Keys
	post func(func())
	maps map[string]*Mapping
	http *http.Client

	ops    chan func()
	wake   chan struct{}
	ctx    context.Context
	cancel func()
	done   chan struct{}

	mu       gosync.Mutex
	status   Status
	onStatus []func(Status) // the main thread's
	// stopRun cuts the run going on short, nil between runs, and cut says CancelRun did.
	stopRun func()
	cut     bool

	// The sync goroutine's own.
	conn *connection
	last time.Time // when the last run ended
	wait time.Duration
	next time.Time // no run before, after a failed one
}

// connection is a bucket sync is on with.
type connection struct {
	address, token string
	key            *synckosh.DataKey
	local          *local
	replica        *client.Replica
}

// New starts syncing the profile kept in st, whose keys are k, with the features' mappings,
// after the features have set up their tables. post runs a function on the main thread. If
// sync is on, a run starts at once.
func New(st *store.Store, k *keys.Keys, post func(func()), maps ...Mapping) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{st: st, keys: k, post: post, maps: map[string]*Mapping{}, http: checked(httpClient), ops: make(chan func(), 64), wake: make(chan struct{}, 1),
		ctx: ctx, cancel: cancel, done: make(chan struct{})}
	for i := range maps {
		m.maps[maps[i].Collection] = &maps[i]
	}
	setup(st)
	st.Do(m.triggers)
	go m.loop()
	m.ops <- m.load
	return m
}

// triggers are what mark the features' changed rows while marking is on. They are made again
// at each start from the mappings, so they always match them.
func (m *Manager) triggers(tx *store.Tx) error {
	for _, mp := range m.maps {
		for _, t := range []struct{ name, when, row, deleted string }{
			{"in", "INSERT", "new", "0"}, {"up", "UPDATE", "new", "0"}, {"out", "DELETE", "old", "1"},
		} {
			name := fmt.Sprintf(`"sync_%s_%s"`, mp.Collection, t.name)
			if _, err := tx.Exec(`DROP TRIGGER IF EXISTS ` + name); err != nil {
				return err
			}
			if _, err := tx.Exec(fmt.Sprintf(`CREATE TRIGGER %s AFTER %s ON "%s" WHEN (SELECT marking FROM sync_state) BEGIN
				INSERT OR REPLACE INTO sync_changes (collection, key, deleted) VALUES ('%s', %s."%s", %s); END`,
				name, t.when, mp.Table, mp.Collection, t.row, mp.Key, t.deleted)); err != nil {
				return err
			}
		}
	}
	return nil
}

// ClearedAll is a feature saying, in the transaction that emptied collections, that it did:
// their marks become one each, which goes out as a delete of everything before now.
func ClearedAll(tx *store.Tx, collections ...string) error {
	for _, c := range collections {
		if _, err := tx.Exec(`DELETE FROM sync_changes WHERE collection = ?`, c); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO sync_changes (collection, key, deleted) SELECT ?, '', 1 FROM sync_state WHERE marking`, c); err != nil {
			return err
		}
	}
	return nil
}

// Status is sync's status now.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// OnStatus subscribes to the status, on the main thread.
func (m *Manager) OnStatus(f func(Status)) { m.onStatus = append(m.onStatus, f) }

// setStatus changes the status and tells the subscribers.
func (m *Manager) setStatus(f func(s *Status)) {
	m.mu.Lock()
	f(&m.status)
	s := m.status
	m.mu.Unlock()
	m.post(func() {
		for _, f := range m.onStatus {
			f(s)
		}
	})
}

// SyncNow asks for a run now, even while a failed one waits.
func (m *Manager) SyncNow() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// CancelRun cuts the run going on short, as a person asks: it ends with no error, and the
// next comes when it would have.
func (m *Manager) CancelRun() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopRun != nil {
		m.cut = true
		m.stopRun()
	}
}

// Stop ends syncing: a run going on is cut short. Before the store closes.
func (m *Manager) Stop() {
	m.cancel()
	<-m.done
}

func (m *Manager) loop() {
	defer close(m.done)
	tick := time.NewTicker(look)
	defer tick.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case op := <-m.ops:
			op()
		case <-m.wake:
			if m.conn != nil {
				m.run()
			}
		case <-tick.C:
			if m.conn != nil && !time.Now().Before(m.next) && (time.Since(m.last) >= every || m.marked()) {
				m.run()
			}
		}
	}
}

// do runs f on the sync goroutine, and posts its error to done on the main thread.
func (m *Manager) do(done func(error), f func() error) {
	m.ops <- func() {
		err := f()
		if done != nil {
			m.post(func() { done(err) })
		}
	}
}

// load reads what sync is on with, and runs. A locked profile key leaves it off, saying so.
func (m *Manager) load() {
	if err := m.keys.Check(); err != nil {
		m.setStatus(func(s *Status) { *s = Status{Locked: errors.Is(err, keys.ErrLocked), Error: lockedError(err).Error()} })
		return
	}
	var address, token, key []byte
	err := m.st.Wait(func(tx *store.Tx) (err error) {
		if address, _, err = m.keys.Secret(tx, secretAddress); err != nil || address == nil {
			return err
		}
		if token, _, err = m.keys.Secret(tx, secretToken); err != nil {
			return err
		}
		key, _, err = m.keys.Secret(tx, secretKey)
		return err
	})
	if err == nil && address != nil {
		var dk *synckosh.DataKey
		if dk, err = unkeep(key); err == nil {
			err = m.open(string(address), string(token), dk)
		}
	}
	if err != nil {
		m.setStatus(func(s *Status) { *s = Status{Error: err.Error(), Problem: Problem(err)} })
		return
	}
	if m.conn != nil {
		m.run()
	}
}

// lockedError is what the status says of keys that don't open.
func lockedError(err error) error {
	if errors.Is(err, keys.ErrLocked) {
		return errors.New("sync is off: this profile's keys don't open on this device; start fresh to sync it again")
	}
	return err
}

// open makes the connection to the bucket at address, with its token and key.
func (m *Manager) open(address, token string, key *synckosh.DataKey) error {
	l := newLocal(m.st, m.maps, key)
	r, err := client.Open(replicaConfig(&client.Client{URL: address, Token: token, HTTP: m.http}, l, key, m.maps))
	if err != nil {
		return err
	}
	m.conn = &connection{address: address, token: token, key: key, local: l, replica: r}
	m.wait, m.next = 0, time.Time{}
	m.setStatus(func(s *Status) { *s = Status{On: true, Host: hostOf(address)} })
	return nil
}

// begin turns sync on with a bucket and its key: the address, token and key kept sealed,
// synckosh's bookkeeping started afresh, and every row handed to it once, by marking it.
func (m *Manager) begin(address, token string, key *synckosh.DataKey) error {
	kept, err := keep(key)
	if err != nil {
		return err
	}
	err = m.st.Wait(func(tx *store.Tx) error {
		for name, v := range map[string]string{secretAddress: address, secretToken: token, secretKey: string(kept)} {
			if err := m.keys.PutSecret(tx, name, []byte(v)); err != nil {
				return err
			}
		}
		if err := forget(tx); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE sync_state SET marking = 1`); err != nil {
			return err
		}
		for _, mp := range m.maps {
			if _, err := tx.Exec(fmt.Sprintf(`INSERT OR REPLACE INTO sync_changes (collection, key, deleted) SELECT '%s', "%s", 0 FROM "%s"`,
				mp.Collection, mp.Key, mp.Table)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return m.open(address, token, key)
}

// end turns sync off: the secrets and synckosh's bookkeeping go, and no row is marked. Every
// row stays.
func (m *Manager) end() error {
	m.conn = nil
	err := m.st.Wait(func(tx *store.Tx) error {
		if err := m.keys.DeleteSecrets(tx, secretAddress, secretToken, secretKey); err != nil {
			return err
		}
		if err := forget(tx); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE sync_state SET marking = 0`)
		return err
	})
	m.setStatus(func(s *Status) { *s = Status{} })
	return err
}

// forget deletes synckosh's bookkeeping and the marks.
func forget(tx *store.Tx) error {
	_, err := tx.Exec(`DELETE FROM sync_items; DELETE FROM sync_meta; DELETE FROM sync_changes`)
	return err
}

// keep is a sync key as kept in the secrets, and unkeep the key again.
func keep(key *synckosh.DataKey) ([]byte, error) {
	return json.Marshal(struct {
		ID  string `json:"id"`
		Key []byte `json:"key"`
	}{key.ID, key.Bytes()})
}

func unkeep(b []byte) (*synckosh.DataKey, error) {
	var k struct {
		ID  string `json:"id"`
		Key []byte `json:"key"`
	}
	if err := json.Unmarshal(b, &k); err != nil {
		return nil, fmt.Errorf("sync: the kept sync key: %w", err)
	}
	return synckosh.DataKeyFrom(k.ID, k.Key)
}

// marked says whether rows wait to go out.
func (m *Manager) marked() bool {
	has := false
	m.st.Wait(func(tx *store.Tx) error { return tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM sync_changes)`).Scan(&has) })
	return has
}

// run drains the marks into synckosh, then syncs, and tells the features whose rows changed.
// A failure waits longer each time before the next run that isn't asked for.
func (m *Manager) run() error {
	c := m.conn
	m.setStatus(func(s *Status) { s.Syncing = true })
	ctx, cancel := context.WithTimeout(m.ctx, runFor)
	m.mu.Lock()
	m.stopRun, m.cut = cancel, false
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.stopRun = nil
		m.mu.Unlock()
		cancel()
	}()
	err := m.drain(c)
	if err == nil {
		err = c.replica.Sync(ctx)
	}
	for collection := range c.local.wrote {
		if f := m.maps[collection].Changed; f != nil {
			m.post(f)
		}
	}
	clear(c.local.wrote)
	m.mu.Lock()
	cut := m.cut
	m.mu.Unlock()
	if cut && err != nil {
		m.setStatus(func(s *Status) { s.Syncing = false })
		return err
	}
	m.last = time.Now()
	if err != nil {
		m.wait = min(max(2*m.wait, firstWait), longestWait)
		m.next = m.last.Add(m.wait)
		timeline.Warnf("sync: %v; the next run in %s", err, m.wait)
	} else {
		m.wait, m.next = 0, time.Time{}
	}
	m.setStatus(func(s *Status) {
		s.Syncing = false
		if s.Error, s.Problem = "", ""; err != nil {
			s.Error, s.Problem = err.Error(), Problem(err)
		} else {
			s.LastSynced = m.last.UnixMilli()
		}
	})
	return err
}

// change is one mark.
type change struct {
	rowid           int64
	collection, key string
	deleted         bool
}

// drain hands the marked rows to synckosh, a few hundred at a time: a delete of everything
// first, then each row changed, read from it, and each deleted. A mark is deleted once it is
// handed over, unless the row was marked again meanwhile.
func (m *Manager) drain(c *connection) error {
	for {
		var marks []change
		var puts []synckosh.Item
		err := m.st.Wait(func(tx *store.Tx) error {
			r, err := tx.Query(`SELECT rowid, collection, key, deleted FROM sync_changes ORDER BY key <> '', rowid LIMIT 500`)
			if err != nil {
				return err
			}
			for r.Next() {
				var ch change
				if err := r.Scan(&ch.rowid, &ch.collection, &ch.key, &ch.deleted); err != nil {
					r.Close()
					return err
				}
				marks = append(marks, ch)
			}
			r.Close()
			if err := r.Err(); err != nil {
				return err
			}
			for i, ch := range marks {
				mp := m.maps[ch.collection]
				if mp == nil || ch.deleted {
					continue
				}
				private, has, err := mp.Read(tx, ch.key)
				if err != nil {
					return err
				}
				if !has {
					marks[i].deleted = true
					continue
				}
				puts = append(puts, synckosh.Item{Collection: mp.Collection, Datatype: mp.Datatype, GUID: c.local.guid(mp, ch.key), Private: private})
			}
			return nil
		})
		if err != nil || len(marks) == 0 {
			return err
		}
		for _, ch := range marks {
			if mp := m.maps[ch.collection]; mp != nil && ch.key == "" {
				if err := c.replica.DeleteBefore(ch.collection, time.Now()); err != nil {
					return err
				}
			}
		}
		if len(puts) > 0 {
			if _, err := c.replica.Put(puts...); err != nil {
				return err
			}
		}
		for _, ch := range marks {
			if mp := m.maps[ch.collection]; mp != nil && ch.key != "" && ch.deleted {
				if err := c.replica.Delete(ch.collection, c.local.guid(mp, ch.key)); err != nil {
					return err
				}
			}
		}
		if err := m.st.Wait(func(tx *store.Tx) error {
			for _, ch := range marks {
				if _, err := tx.Exec(`DELETE FROM sync_changes WHERE rowid = ?`, ch.rowid); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
}

// hostOf is the host of an address, which is all the status shows of it.
func hostOf(address string) string {
	u, err := url.Parse(address)
	if err != nil {
		return ""
	}
	return u.Host
}
