// Package sparks keeps the profile's Sparks: userscripts, in JavaScript, that run on the
// pages their header names. Each is a .user.js source with a ==UserScript== header, which
// gives its name, the match patterns of the addresses it runs on, and when it runs. The
// package keeps them, reads their headers and says which run on a frame's address; the
// engine puts them in the page. They live in the profile folder's sparks folder: each
// spark's source as <id>.user.js, and sparks.json for the list, in order, with whether
// each is on and when it was added and last changed, and the built-in sparks it was given,
// so that sync can carry each as an item (docs/site-features.md).
package sparks

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"projectb/timeline"
)

// MaxSparks is how many sparks a profile keeps.
const MaxSparks = 25

// Spark is one spark as the list shows it. A spark whose file no longer reads has only its
// ID, On and Error, and never runs.
type Spark struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Match        []string `json:"match"`
	ExcludeMatch []string `json:"excludeMatch,omitempty"`
	RunAt        string   `json:"runAt"`
	NoFrames     bool     `json:"noFrames,omitempty"`
	On           bool     `json:"on"`
	Error        string   `json:"error,omitempty"`
}

// Script is a spark for the engine to put in a page: its source, and when it runs.
type Script struct {
	ID     string
	Source string
	RunAt  string
}

// entry is one spark as the manager holds it.
type entry struct {
	id string
	on bool
	// added and modified are Unix milliseconds: when it was kept, and when its source or
	// its switch last changed.
	added    int64
	modified int64
	source   string
	meta     Meta
	match    []matcher
	exclude  []matcher
	// err is why the source doesn't read, "" when it does.
	err string
}

// listed is one spark in sparks.json.
type listed struct {
	ID       string `json:"id"`
	On       bool   `json:"on"`
	Added    int64  `json:"added"`
	Modified int64  `json:"modified"`
}

// kept is sparks.json: version 2. Version 1, what a profile had before, is the list alone,
// with only each one's id and switch.
type kept struct {
	Version int      `json:"version"`
	Sparks  []listed `json:"sparks"`
	// Builtins is the ids the built-in sparks were added as, once; nil until they were.
	Builtins []string `json:"builtins,omitempty"`
}

// now is the time in Unix milliseconds, a variable for the tests.
var now = func() int64 { return time.Now().UnixMilli() }

// Manager keeps one profile's sparks. It is not safe for concurrent use: every call comes
// from one thread.
type Manager struct {
	dir       string
	list      []*entry
	builtins  []string
	onChanged []func()
}

// New reads the sparks kept in the profile folder dir. A missing folder is no sparks.
func New(dir string) *Manager {
	m := &Manager{dir: filepath.Join(dir, "sparks")}
	b, err := os.ReadFile(m.listPath())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			timeline.Errorf("sparks: %v", err)
		}
		return m
	}
	var k kept
	if b = bytes.TrimSpace(b); len(b) > 0 && b[0] == '[' {
		// Version 1: each takes a time it was added, a millisecond apart in list order.
		err = json.Unmarshal(b, &k.Sparks)
		t := now() - int64(len(k.Sparks))
		for i := range k.Sparks {
			k.Sparks[i].Added, k.Sparks[i].Modified = t+int64(i), t+int64(i)
		}
	} else {
		err = json.Unmarshal(b, &k)
	}
	if err != nil {
		timeline.Errorf("sparks: %s: %v", m.listPath(), err)
		return m
	}
	m.builtins = k.Builtins
	for _, l := range k.Sparks {
		if !validID(l.ID) || m.find(l.ID) != nil {
			continue
		}
		e := &entry{id: l.ID, on: l.On, added: l.Added, modified: l.Modified}
		if b, err := os.ReadFile(m.sourcePath(l.ID)); err != nil {
			e.err = "The script's file can't be read."
		} else {
			e.set(string(b))
		}
		m.list = append(m.list, e)
	}
	return m
}

// List is every spark, in order.
func (m *Manager) List() []Spark {
	out := make([]Spark, len(m.list))
	for i, e := range m.list {
		out[i] = e.spark()
	}
	return out
}

// OnChanged subscribes to any change to the sparks.
func (m *Manager) OnChanged(f func()) { m.onChanged = append(m.onChanged, f) }

// Add keeps a new spark from its source, on, after the others.
func (m *Manager) Add(source string) (Spark, error) {
	e, err := m.add(source)
	if err != nil {
		return Spark{}, err
	}
	m.changed()
	return e.spark(), nil
}

// Update puts a new source in place of a spark's, which mends one that didn't read.
func (m *Manager) Update(id, source string) error {
	e := m.find(id)
	if e == nil {
		return errors.New("There is no such spark.")
	}
	if err := m.put(e, source); err != nil {
		return err
	}
	m.changed()
	return nil
}

// Remove takes a spark out of the list and deletes its file.
func (m *Manager) Remove(id string) {
	i := slices.IndexFunc(m.list, func(e *entry) bool { return e.id == id })
	if i < 0 {
		return
	}
	m.list = slices.Delete(m.list, i, i+1)
	m.changed()
	if err := os.Remove(m.sourcePath(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		timeline.Errorf("sparks: %v", err)
	}
}

// SetOn switches a spark on or off.
func (m *Manager) SetOn(id string, on bool) {
	e := m.find(id)
	if e == nil || e.on == on {
		return
	}
	e.on = on
	e.modified = now()
	m.changed()
}

// Source is a spark's source, and whether there is such a spark with one.
func (m *Manager) Source(id string) (string, bool) {
	e := m.find(id)
	if e == nil || e.source == "" && e.err != "" {
		return "", false
	}
	return e.source, true
}

// FileName is the name to export a spark's source as: its name, with what a file system
// won't take in a name left out, and .user.js.
func (m *Manager) FileName(id string) string {
	name := ""
	if e := m.find(id); e != nil {
		name = e.meta.Name
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r < ' ' || r == 0x7f || strings.ContainsRune(`/\:*?"<>|`, r):
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	safe := strings.Trim(b.String(), " .-")
	if safe == "" {
		safe = "spark"
	}
	return safe + ".user.js"
}

// Matching is every spark, on or off, whose header takes the address of a frame: the tab's
// page when mainFrame, else a frame inside it, which a @noframes spark leaves alone. They
// are in list order. Which of them run is the caller's: those on, and not switched off
// for the site.
func (m *Manager) Matching(url string, mainFrame bool) []Spark {
	var out []Spark
	for _, e := range m.matching(url, mainFrame) {
		out = append(out, e.spark())
	}
	return out
}

// Scripts is the sparks to put in a frame: those of Matching that are on.
func (m *Manager) Scripts(url string, mainFrame bool) []Script {
	var out []Script
	for _, e := range m.matching(url, mainFrame) {
		if e.on {
			out = append(out, Script{ID: e.id, Source: e.source, RunAt: e.meta.RunAt})
		}
	}
	return out
}

func (m *Manager) matching(url string, mainFrame bool) []*entry {
	t, ok := parseTarget(url)
	if !ok {
		return nil
	}
	var out []*entry
	for _, e := range m.list {
		if e.err == "" && (mainFrame || !e.meta.NoFrames) && e.matches(t) {
			out = append(out, e)
		}
	}
	return out
}

// set reads a source into the entry, keeping why when it doesn't read.
func (e *entry) set(source string) error {
	e.source = source
	meta, err := Parse(source)
	if err != nil {
		e.meta, e.match, e.exclude, e.err = Meta{}, nil, nil, err.Error()
		return err
	}
	e.meta, e.err = meta, ""
	e.match, e.exclude = compileAll(meta.Match), compileAll(meta.ExcludeMatch)
	return nil
}

// compileAll compiles patterns Parse has already found good.
func compileAll(patterns []string) []matcher {
	out := make([]matcher, 0, len(patterns))
	for _, p := range patterns {
		mt, _ := compile(p)
		out = append(out, mt)
	}
	return out
}

// matches says whether any @match takes the address and no @exclude-match does.
func (e *entry) matches(t target) bool {
	in := false
	for _, mt := range e.match {
		if mt.match(t) {
			in = true
			break
		}
	}
	if !in {
		return false
	}
	for _, mt := range e.exclude {
		if mt.match(t) {
			return false
		}
	}
	return true
}

func (e *entry) spark() Spark {
	return Spark{
		ID:           e.id,
		Name:         e.meta.Name,
		Description:  e.meta.Description,
		Match:        slices.Clone(e.meta.Match),
		ExcludeMatch: slices.Clone(e.meta.ExcludeMatch),
		RunAt:        e.meta.RunAt,
		NoFrames:     e.meta.NoFrames,
		On:           e.on,
		Error:        e.err,
	}
}

func (m *Manager) find(id string) *entry {
	for _, e := range m.list {
		if e.id == id {
			return e
		}
	}
	return nil
}

// newID is a UUIDv7 (RFC 9562), so ids sort by when they were made, as sync's guids do,
// that no spark has and no file in the folder uses.
func (m *Manager) newID() string {
	for {
		var b [16]byte
		rand.Read(b[:])
		ms := uint64(now())
		for i := 0; i < 6; i++ {
			b[i] = byte(ms >> (40 - 8*i))
		}
		b[6] = b[6]&0x0f | 0x70
		b[8] = b[8]&0x3f | 0x80
		h := hex.EncodeToString(b[:])
		id := h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
		if _, err := os.Stat(m.sourcePath(id)); m.find(id) == nil && errors.Is(err, os.ErrNotExist) {
			return id
		}
	}
}

// validID says whether id is a lowercase UUID, or the eight lowercase hex digits a spark
// kept before sparks had UUIDs has, so it is safe in a file name.
func validID(id string) bool {
	hexOnly := func(s string) bool {
		for _, c := range s {
			if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
				return false
			}
		}
		return true
	}
	if len(id) == 8 {
		return hexOnly(id)
	}
	parts := strings.Split(id, "-")
	if len(id) != 36 || len(parts) != 5 {
		return false
	}
	for i, n := range []int{8, 4, 4, 4, 12} {
		if len(parts[i]) != n || !hexOnly(parts[i]) {
			return false
		}
	}
	return true
}

func (m *Manager) listPath() string { return filepath.Join(m.dir, "sparks.json") }

func (m *Manager) sourcePath(id string) string { return filepath.Join(m.dir, id+".user.js") }

func (m *Manager) changed() {
	k := kept{Version: 2, Sparks: make([]listed, len(m.list)), Builtins: m.builtins}
	for i, e := range m.list {
		k.Sparks[i] = listed{ID: e.id, On: e.on, Added: e.added, Modified: e.modified}
	}
	b, _ := json.MarshalIndent(k, "", "  ")
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		timeline.Errorf("sparks: %v", err)
	} else {
		writeFile(m.listPath(), append(b, '\n'))
	}
	for _, f := range m.onChanged {
		f()
	}
}

// writeFile writes a new file and renames it over the old, so no reader and no crash sees
// half of one.
func writeFile(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		timeline.Errorf("sparks: %v", err)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		timeline.Errorf("sparks: %v", err)
		return err
	}
	return nil
}
