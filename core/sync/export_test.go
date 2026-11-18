package sync

import (
	"github.com/sudiptadeb/synckosh"
	"github.com/sudiptadeb/synckosh/client"

	"projectb/store"
)

// What the tests, in package sync_test so they can use the features, reach inside.

// Setup is setup.
func Setup(st *store.Store) { setup(st) }

// Local is the Local over the store.
type Local = local

// NewLocal is a Local of maps over st, for a bucket of key.
func NewLocal(st *store.Store, maps []Mapping, key *synckosh.DataKey) (*Local, map[string]*Mapping) {
	byName := map[string]*Mapping{}
	for i := range maps {
		byName[maps[i].Collection] = &maps[i]
	}
	return newLocal(st, byName, key), byName
}

// GUID is the guid of the row key of m.
func (l *local) GUID(m *Mapping, key string) string { return l.guid(m, key) }

// ReplicaConfig is replicaConfig.
func ReplicaConfig(c *client.Client, l *Local, key *synckosh.DataKey, maps map[string]*Mapping) client.Config {
	return replicaConfig(c, l, key, maps)
}
