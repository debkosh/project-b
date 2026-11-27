// Package fetch fetches addresses into files as a page would, through the engine's own
// network (docs/video-streams.md): the pieces of a streamed video, its playlists and its
// keys. The engine's network decides which cookies go and refuses a server on a more private
// address than the request is made from; Go sees no cookie. The manager numbers each fetch,
// gives its end to whoever asked, and holds the two rules on its own side too: only http and
// https, and nothing from this computer or its network for a request made from the internet.
package fetch

import (
	"net/url"
	"os"

	"projectb/engine"
)

// Engine is what the manager asks of the engine.
type Engine interface {
	Fetch(f engine.FetchRequest)
	CancelFetch(id engine.FetchID)
}

// Manager runs the fetches. Main thread, as the engine's calls are.
type Manager struct {
	e       Engine
	next    engine.FetchID
	running map[engine.FetchID]running
}

type running struct {
	r    engine.FetchRequest
	done func(engine.Fetched)
}

// New returns a manager fetching through e.
func New(e Engine) *Manager {
	return &Manager{e: e, running: map[engine.FetchID]running{}}
}

// Allowed says an address may be fetched: http or https, with a host.
func Allowed(address string) bool {
	u, err := url.Parse(address)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// Fetch fetches r, whose ID the manager sets, and calls done once with how it ended, unless
// the cancel it returns is called first, which stops it and removes its file. An address
// that isn't http or https is refused at once, before this returns.
func (m *Manager) Fetch(r engine.FetchRequest, done func(engine.Fetched)) (cancel func()) {
	m.next++
	r.ID = m.next
	if !Allowed(r.URL) {
		done(engine.Fetched{ID: r.ID, Outcome: engine.FetchRefused, Space: engine.SpacePublic, URL: r.URL})
		return func() {}
	}
	m.running[r.ID] = running{r, done}
	m.e.Fetch(r)
	return func() {
		if _, ok := m.running[r.ID]; ok {
			delete(m.running, r.ID)
			m.e.CancelFetch(r.ID)
			os.Remove(r.Path)
		}
	}
}

// Fetched takes the engine's news of a fetch's end. One that came from a more private
// address than it was asked from, or by a redirect from an address that isn't http or https,
// is refused here too, and its file goes.
func (m *Manager) Fetched(f engine.Fetched) {
	w, ok := m.running[f.ID]
	if !ok {
		return
	}
	delete(m.running, f.ID)
	if f.Outcome == engine.FetchOK && (w.r.From != engine.SpaceAny && engine.MorePrivate(f.Space, w.r.From) || f.URL != "" && !Allowed(f.URL)) {
		os.Remove(w.r.Path)
		f.Outcome = engine.FetchRefused
	}
	w.done(f)
}
