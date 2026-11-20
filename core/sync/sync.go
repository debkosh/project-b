// Package sync syncs a profile's bookmarks, history and typed addresses through a synckosh
// server (docs/sync.md). synckosh's replica keeps its bookkeeping in the profile's store, and
// reads and writes the features' own rows through each one's Mapping.
package sync

import (
	"github.com/sudiptadeb/synckosh"
	"github.com/sudiptadeb/synckosh/client"

	"projectb/store"
)

// setup brings sync's tables up to date, on the worker, after the features' own.
func setup(st *store.Store) {
	st.Schema("sync", func(tx *store.Tx) error {
		_, err := tx.Exec(tables)
		return err
	}, func(tx *store.Tx) error {
		// A mark sync couldn't hand over is set aside, with why, until its row changes again,
		// the app starts or a person asks for a run: one row that can't go mustn't hold the
		// others back, run after run.
		_, err := tx.Exec(`ALTER TABLE sync_changes ADD COLUMN stuck TEXT NOT NULL DEFAULT ''`)
		return err
	})
}

// replicaConfig is the replica of a bucket over the store: it keeps deleted items, so it
// never removes an entry, and so a row, for a delete; and the datatypes whose Join merges use
// it as their merger.
func replicaConfig(c *client.Client, l *local, key *synckosh.DataKey, maps map[string]*Mapping) client.Config {
	mergers := map[string]client.Merger{}
	for _, m := range maps {
		if m.Merges {
			mergers[m.Datatype] = merger(m)
		}
	}
	return client.Config{Client: c, Local: l, Key: key, KeepDeleted: true, Mergers: mergers, Identity: l.identity}
}

// merger is a datatype's Join as synckosh's merger. Between a delete and an edit the newer
// wins: a join would bring the deleted back.
func merger(m *Mapping) client.Merger {
	return func(local, remote synckosh.Item) synckosh.Item {
		if local.Deleted || remote.Deleted {
			if local.Timestamp > remote.Timestamp {
				return local
			}
			return remote
		}
		out := remote
		out.Private = m.Join(local.Private, remote.Private)
		return out
	}
}
