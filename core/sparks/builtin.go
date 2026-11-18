package sparks

import (
	"embed"
	"slices"

	"projectb/timeline"
)

// The built-in sparks (docs/site-features.md): three of the owner's sparks, cleaned up, that
// come with the app to give a feel of what sparks do. A profile is given them once, off, and
// from then on they are the person's own, as any other spark.

//go:embed builtin/*.user.js
var builtinFS embed.FS

// builtinFiles is the built-in sparks' files, in the order they are added.
var builtinFiles = []string{
	"builtin/video-downloader.user.js",
	"builtin/screen-time-limit.user.js",
	"builtin/page-cleanup.user.js",
}

// AddBuiltins gives the profile the built-in sparks, off, after any it has, while there is
// room, the first time it is called for the profile; it does nothing after that, even once
// they are deleted.
func (m *Manager) AddBuiltins() {
	if m.builtins != nil {
		return
	}
	m.builtins = []string{}
	for _, name := range builtinFiles {
		b, err := builtinFS.ReadFile(name)
		if err != nil {
			timeline.Errorf("sparks: %v", err)
			continue
		}
		e, err := m.add(string(b))
		if err != nil {
			timeline.Errorf("sparks: %s: %v", name, err)
			continue
		}
		e.on = false
		m.builtins = append(m.builtins, e.id)
	}
	m.changed()
}

// Builtins is the built-in sparks the profile still has, in the order they were added.
func (m *Manager) Builtins() []Spark {
	var out []Spark
	for _, e := range m.list {
		if slices.Contains(m.builtins, e.id) {
			out = append(out, e.spark())
		}
	}
	return out
}
