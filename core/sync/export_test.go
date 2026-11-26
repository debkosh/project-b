package sync

import (
	"errors"
	"net/http"

	"github.com/debkosh/synckosh"
	"github.com/debkosh/synckosh/client"

	"projectb/store"
)

// What the tests, in package sync_test so they can use the features, reach inside.

// Setup is setup.
func Setup(st *store.Store) { setup(st) }

// Local is the Local over the store.
type Local = local

// NewLocal is a Local of maps over st.
func NewLocal(st *store.Store, maps []Mapping) (*Local, map[string]*Mapping) {
	byName := map[string]*Mapping{}
	for i := range maps {
		byName[maps[i].Collection] = &maps[i]
	}
	return newLocal(st, byName), byName
}

// GuidOf is guidOf: the guid the row of key goes out under.
func GuidOf(tx *store.Tx, m *Mapping, key string) (string, error) { return guidOf(tx, m, key) }

// ReplicaConfig is replicaConfig.
func ReplicaConfig(c *client.Client, l *Local, key *synckosh.DataKey, maps map[string]*Mapping) client.Config {
	return replicaConfig(c, l, key, maps)
}

// SetHTTPClient has every request go through c, and returns what puts it back.
func SetHTTPClient(c *http.Client) func() {
	was := HTTPClient
	HTTPClient = c
	return func() { HTTPClient = was }
}

// Begin turns sync on with a bucket and its key, and runs.
func (m *Manager) Begin(address, token string, key *synckosh.DataKey) error {
	return m.await(func() error {
		if err := m.begin(address, token, key); err != nil {
			return err
		}
		return m.run(WhyOn)
	})
}

// RunNow runs, as a person asking for it, and returns its error.
func (m *Manager) RunNow() error { return m.RunAs(WhyAsked) }

// RunAs runs, started for why, and returns its error.
func (m *Manager) RunAs(why string) error {
	return m.await(func() error {
		if m.conn == nil {
			return errors.New("sync is off")
		}
		return m.run(why)
	})
}

// await runs f on the sync goroutine and returns its error.
func (m *Manager) await(f func() error) error {
	got := make(chan error, 1)
	m.ops <- func() { got <- f() }
	return <-got
}

// Idle returns once the sync goroutine has done what was asked of it before: at start, read
// what sync is on with.
func (m *Manager) Idle() { m.await(func() error { return nil }) }
