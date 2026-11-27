package sparks

import (
	"embed"
	"errors"
	"path"
	"strings"

	"projectb/timeline"
)

// The system sparks (docs/site-features.md, "The built-in sparks"): three of the owner's
// sparks, cleaned up, that come with the app to give a feel of what sparks do. Their source
// is always the app's own file, read as the app starts and never kept in the profile, so an
// update brings their fixes. The profile keeps only whether each is on, by an id that is its
// file's name. They start off, can't be edited or deleted, and don't count toward
// MaxSparks; Duplicate makes one the person's own.

//go:embed builtin/*.user.js
var builtinFS embed.FS

// builtinFiles is the system sparks' files, in the order they are listed.
var builtinFiles = []string{
	"builtin/video-downloader.user.js",
	"builtin/screen-time-limit.user.js",
	"builtin/page-cleanup.user.js",
}

// errSystem is what a change to a system spark's source, or its delete, says.
var errSystem = errors.New("A built-in spark can't be changed. Duplicate it to make one of your own.")

// systemSparks reads the system sparks from the app's files, each off.
func systemSparks() []*entry {
	var out []*entry
	for _, name := range builtinFiles {
		b, err := builtinFS.ReadFile(name)
		if err != nil {
			timeline.Errorf("sparks: %v", err)
			continue
		}
		e := &entry{id: strings.TrimSuffix(path.Base(name), ".user.js"), system: true}
		if err := e.set(string(b)); err != nil {
			timeline.Errorf("sparks: %s: %v", name, err)
			continue
		}
		out = append(out, e)
	}
	return out
}

// System is the system sparks, in their order.
func (m *Manager) System() []Spark {
	out := make([]Spark, len(m.system))
	for i, e := range m.system {
		out[i] = e.spark()
	}
	return out
}

// systemNamed is the system spark of that @name, or nil.
func (m *Manager) systemNamed(name string) *entry {
	for _, e := range m.system {
		if e.meta.Name == name {
			return e
		}
	}
	return nil
}

// Duplicate keeps a copy of a spark as the person's own, on, named "<Name> copy" (or the
// first of "<Name> copy 2", "<Name> copy 3"... no spark has), and turns the spark it copies
// off, so a page never gets both.
func (m *Manager) Duplicate(id string) (Spark, error) {
	e := m.any(id)
	if e == nil || e.err != "" {
		return Spark{}, errors.New("There is no such spark.")
	}
	name := fit(e.meta.Name, " copy")
	taken := m.taken()
	if taken[key(Meta{Name: name, Namespace: e.meta.Namespace})] {
		name = freeName(Meta{Name: name, Namespace: e.meta.Namespace}, taken)
	}
	c, err := m.add(renamed(e.source, name))
	if err != nil {
		return Spark{}, err
	}
	if e.on {
		e.on = false
		e.modified = now()
	}
	m.changed()
	return c.spark(), nil
}
