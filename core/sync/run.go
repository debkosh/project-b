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

// HTTPClient carries the requests of a Manager made from now on: the mock engine's dev server
// points it at a sync server of its own.
var HTTPClient = &http.Client{Timeout: 2 * time.Minute}

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
	// device out, "unreachable" when it gave no answer, "server" when it answered that it
	// failed, "notBucket" when the bucket is gone, and "" for anything else, which the page
	// calls by no more than that sync failed.
	Problem string `json:"problem"`
	// Locked says the profile's key doesn't open with this device's key, so sync is off
	// until StartFresh.
	Locked bool `json:"locked"`
	// Unsent is how many rows couldn't be sent: those set aside, and those the server
	// refused in the last run. Everything else syncs; Sync details says which and why.
	Unsent int `json:"unsent"`
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
	address  string // the bucket's, while sync is on
	onStatus []func(Status) // the main thread's
	// stopRun cuts the run going on short, nil between runs, and cut says CancelRun did.
	stopRun func()
	cut     bool
	// For Sync details (details.go): the run going on, the rows of those ended since the app
	// started, newest first, and when the next comes if nothing changes.
	cur     *Run
	runs    []Run
	nextRun time.Time

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
	m := &Manager{st: st, keys: k, post: post, maps: map[string]*Mapping{}, ops: make(chan func(), 64), wake: make(chan struct{}, 1),
		ctx: ctx, cancel: cancel, done: make(chan struct{})}
	m.http = checked(HTTPClient, m.noteAsking, m.noteRequest)
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
// at each start from the mappings, so they always match them. A mark takes the place of the
// row's earlier one by deleting it first: an INSERT OR REPLACE here would be overridden by the
// statement that fired the trigger, and an upsert's update aborts on a conflict, so a row
// changed again before a run would fail the feature's own write.
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
				DELETE FROM sync_changes WHERE collection = '%s' AND key = %s."%s";
				INSERT INTO sync_changes (collection, key, deleted) VALUES ('%s', %s."%s", %s); END`,
				name, t.when, mp.Table, mp.Collection, t.row, mp.Key, mp.Collection, t.row, mp.Key, t.deleted)); err != nil {
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

// Address is the bucket's address while sync is on, as it was connected to, for the settings
// page's Reconnect; "" while it is off.
func (m *Manager) Address() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.address
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
				m.run(WhyAsked)
			}
		case <-tick.C:
			if m.conn == nil || time.Now().Before(m.next) {
				break
			}
			if m.marked() {
				m.run(WhyChange)
			} else if time.Since(m.last) >= every {
				m.run(WhyTimer)
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
		m.setStatus(func(s *Status) { *s = Status{Error: Words(err), Problem: Problem(err)} })
		return
	}
	if m.conn != nil {
		m.run(WhyStart)
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
	l := newLocal(m.st, m.maps)
	r, err := client.Open(replicaConfig(&client.Client{URL: address, Token: token, HTTP: m.http}, l, key, m.maps))
	if err != nil {
		return err
	}
	m.conn = &connection{address: address, token: token, key: key, local: l, replica: r}
	m.wait, m.next = 0, time.Time{}
	m.mu.Lock()
	m.address = address
	m.mu.Unlock()
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
	m.mu.Lock()
	m.address = ""
	m.cur, m.runs, m.nextRun = nil, nil, time.Time{}
	m.mu.Unlock()
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
	m.st.Wait(func(tx *store.Tx) error { return tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM sync_changes WHERE stuck = '')`).Scan(&has) })
	return has
}

// run drains the marks into synckosh, then syncs, and tells the features whose rows changed.
// A failure waits longer each time before the next run that isn't asked for. why is what
// started it, for Sync details.
func (m *Manager) run(why string) error {
	c := m.conn
	m.started(why)
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
	// A run a person asked for, and the first of a start, try again what was set aside.
	unsent, err := m.drain(c, why != WhyTimer && why != WhyChange)
	var untaken []Moved
	refusedNow := 0
	if err == nil {
		err = c.replica.Sync(ctx)
		// Items the server refused, and items of the server's this device didn't take, are
		// named by synckosh, and everything else was synced: the run went, and says which.
		var refused *client.RefusedError
		var rejected *client.RejectedError
		if errors.As(err, &refused) || errors.As(err, &rejected) {
			errors.As(err, &refused)
			errors.As(err, &rejected)
			by := m.named(refused, nil)
			refusedNow = len(by)
			unsent, untaken = append(unsent, by...), m.named(nil, rejected)
			timeline.Warnf("sync: %s", Words(err))
			err = nil
		}
	}
	for collection := range c.local.wrote {
		if f := m.maps[collection].Changed; f != nil {
			m.post(f)
		}
	}
	clear(c.local.wrote)
	moved := c.local.moved
	c.local.moved = tally{}
	m.mu.Lock()
	cut := m.cut
	m.mu.Unlock()
	if cut && err != nil {
		m.ended(moved, nil, true, unsent, nil)
		m.setStatus(func(s *Status) { s.Syncing = false })
		return err
	}
	m.last = time.Now()
	if err != nil {
		m.wait = min(max(2*m.wait, firstWait), longestWait)
		m.next = m.last.Add(m.wait)
		timeline.Warnf("sync: %s; the next run in %s", Words(err), m.wait)
	} else {
		m.wait, m.next = 0, time.Time{}
	}
	m.ended(moved, err, false, unsent, untaken)
	// What couldn't be sent: every mark set aside, by this run or an earlier one, and what
	// the server refused in this one.
	stuck := refusedNow
	m.st.Wait(func(tx *store.Tx) error {
		var aside int
		if tx.QueryRow(`SELECT COUNT(*) FROM sync_changes WHERE stuck <> ''`).Scan(&aside) == nil {
			stuck += aside
		}
		return nil
	})
	m.mu.Lock()
	if m.nextRun = m.next; err == nil {
		m.nextRun = m.last.Add(every)
	}
	m.mu.Unlock()
	m.setStatus(func(s *Status) {
		s.Syncing = false
		if s.Error, s.Problem = "", ""; err != nil {
			s.Error, s.Problem = Words(err), Problem(err)
		} else {
			s.LastSynced, s.Unsent = m.last.UnixMilli(), stuck
		}
	})
	return err
}

// change is one mark. put is its place among the items a batch hands over, -1 for none.
type change struct {
	rowid           int64
	collection, key string
	deleted         bool
	put             int
	// gone is the guids of the items a row that is gone was.
	gone []string
}

// drain hands the marked rows to synckosh, a few hundred at a time: a delete of everything
// first, then each row changed, read from it, and each deleted. A mark is deleted once it is
// handed over, unless the row was marked again meanwhile. A row that can't be handed over,
// one that doesn't read or that synckosh refuses, is set aside with why (stuck) and the rest
// go: synckosh takes a batch whole or not at all and names every item it refuses
// (client.PutError), so the batch is handed over again without those. Marks set aside are tried again by a run a person asked for, and at the
// start; a row that changes again is marked afresh.
func (m *Manager) drain(c *connection, again bool) (aside []Moved, err error) {
	if again {
		if err := m.st.Wait(func(tx *store.Tx) error {
			_, err := tx.Exec(`UPDATE sync_changes SET stuck = '' WHERE stuck <> ''`)
			return err
		}); err != nil {
			return nil, err
		}
	}
	for {
		var marks []change
		var puts []synckosh.Item
		stuck := map[int64]string{}
		err := m.st.Wait(func(tx *store.Tx) error {
			r, err := tx.Query(`SELECT rowid, collection, key, deleted FROM sync_changes WHERE stuck = '' ORDER BY key <> '', rowid LIMIT 500`)
			if err != nil {
				return err
			}
			for r.Next() {
				ch := change{put: -1}
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
				if mp == nil || ch.key == "" {
					continue
				}
				if !ch.deleted {
					private, has, err := mp.Read(tx, ch.key)
					if err != nil {
						// A row that doesn't read stays as it is everywhere: it isn't said to be gone.
						stuck[ch.rowid] = err.Error()
						continue
					}
					if has {
						guid, err := guidOf(tx, mp, ch.key)
						if err != nil {
							return err
						}
						marks[i].put = len(puts)
						puts = append(puts, synckosh.Item{Collection: mp.Collection, Datatype: mp.Datatype, GUID: guid, Private: private})
						continue
					}
					marks[i].deleted = true
				}
				// A row that is gone: its items go, and one that never went out has none.
				if marks[i].gone, err = liveGuids(tx, mp, ch.key); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil || len(marks) == 0 {
			return aside, err
		}
		for _, ch := range marks {
			if mp := m.maps[ch.collection]; mp != nil && ch.key == "" {
				if err := c.replica.DeleteBefore(ch.collection, time.Now()); err != nil {
					return aside, err
				}
			}
		}
		if len(puts) > 0 {
			if _, err := c.replica.Put(puts...); err != nil {
				// synckosh names every item of the batch it refuses, and has stored none: those
				// are set aside, and the rest are handed over again.
				var refused *client.PutError
				if !errors.As(err, &refused) {
					return aside, err
				}
				why := map[int]string{}
				for _, it := range refused.Items {
					why[it.Index] = it.Reason
				}
				var rest []synckosh.Item
				for _, ch := range marks {
					if ch.put < 0 {
						continue
					}
					if reason, bad := why[ch.put]; bad {
						stuck[ch.rowid] = reason
					} else {
						rest = append(rest, puts[ch.put])
					}
				}
				if len(rest) > 0 {
					if _, err := c.replica.Put(rest...); err != nil {
						return aside, err
					}
				}
			}
		}
		for _, ch := range marks {
			for _, guid := range ch.gone {
				if err := c.replica.Delete(ch.collection, guid); err != nil {
					stuck[ch.rowid] = err.Error()
				}
			}
		}
		if err := m.st.Wait(func(tx *store.Tx) error {
			for _, ch := range marks {
				if why, is := stuck[ch.rowid]; is {
					timeline.Warnf("sync: %s %s can't be sent: %s", ch.collection, ch.key, why)
					if _, err := tx.Exec(`UPDATE sync_changes SET stuck = ? WHERE rowid = ?`, why, ch.rowid); err != nil {
						return err
					}
					one := Moved{Collection: ch.collection, Deleted: ch.deleted, Error: why}
					if mp := m.maps[ch.collection]; mp != nil && !ch.deleted {
						one.Name = name(tx, mp, ch.key)
					}
					aside = append(aside, one)
					continue
				}
				if _, err := tx.Exec(`DELETE FROM sync_changes WHERE rowid = ?`, ch.rowid); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return aside, err
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
