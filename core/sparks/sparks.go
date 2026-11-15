// Package sparks keeps the profile's Sparks: userscripts, in JavaScript, that run on the
// pages their header names. Each is a .user.js source with a ==UserScript== header, which
// gives its name, the match patterns of the addresses it runs on, and when it runs. The
// package keeps them, reads their headers and says which run on a frame's address; the
// engine puts them in the page. They live in the profile folder's sparks folder: each
// spark's source as <id>.user.js, and sparks.json for the list, in order, with whether
// each is on.
package sparks

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	id      string
	on      bool
	source  string
	meta    Meta
	match   []matcher
	exclude []matcher
	// err is why the source doesn't read, "" when it does.
	err string
}

// listed is one spark in sparks.json.
type listed struct {
	ID string `json:"id"`
	On bool   `json:"on"`
}

// Manager keeps one profile's sparks. It is not safe for concurrent use: every call comes
// from one thread.
type Manager struct {
	dir       string
	list      []*entry
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
	var list []listed
	if err := json.Unmarshal(b, &list); err != nil {
		timeline.Errorf("sparks: %s: %v", m.listPath(), err)
		return m
	}
	for _, l := range list {
		if !validID(l.ID) || m.find(l.ID) != nil {
			continue
		}
		e := &entry{id: l.ID, on: l.On}
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
	if len(m.list) >= MaxSparks {
		return Spark{}, errors.New("You can keep up to 25 sparks.")
	}
	e := &entry{id: m.newID(), on: true}
	if err := e.set(source); err != nil {
		return Spark{}, err
	}
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		timeline.Errorf("sparks: %v", err)
		return Spark{}, errors.New("The spark could not be saved.")
	}
	if err := writeFile(m.sourcePath(e.id), []byte(source)); err != nil {
		return Spark{}, errors.New("The spark could not be saved.")
	}
	m.list = append(m.list, e)
	m.changed()
	return e.spark(), nil
}

// Update puts a new source in place of a spark's, which mends one that didn't read.
func (m *Manager) Update(id, source string) error {
	e := m.find(id)
	if e == nil {
		return errors.New("There is no such spark.")
	}
	if _, err := Parse(source); err != nil {
		return err
	}
	if err := writeFile(m.sourcePath(id), []byte(source)); err != nil {
		return errors.New("The spark could not be saved.")
	}
	e.set(source)
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

// Scripts is the sparks to put in a frame: those of Matching that are on, but for those
// offHere names.
func (m *Manager) Scripts(url string, mainFrame bool, offHere func(id string) bool) []Script {
	var out []Script
	for _, e := range m.matching(url, mainFrame) {
		if e.on && (offHere == nil || !offHere(e.id)) {
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

// newID is eight lowercase hex digits no spark has and no file in the folder uses.
func (m *Manager) newID() string {
	for {
		b := make([]byte, 4)
		rand.Read(b)
		id := hex.EncodeToString(b)
		if _, err := os.Stat(m.sourcePath(id)); m.find(id) == nil && errors.Is(err, os.ErrNotExist) {
			return id
		}
	}
}

// validID says whether id is eight lowercase hex digits, so it is safe in a file name.
func validID(id string) bool {
	if len(id) != 8 {
		return false
	}
	for _, c := range id {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return false
		}
	}
	return true
}

func (m *Manager) listPath() string { return filepath.Join(m.dir, "sparks.json") }

func (m *Manager) sourcePath(id string) string { return filepath.Join(m.dir, id+".user.js") }

func (m *Manager) changed() {
	list := make([]listed, len(m.list))
	for i, e := range m.list {
		list[i] = listed{ID: e.id, On: e.on}
	}
	b, _ := json.MarshalIndent(list, "", "  ")
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
