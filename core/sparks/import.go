package sparks

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"projectb/timeline"
)

// Importing sparks (docs/site-features.md): the files picked are unpacked into the sparks
// in them, each one the profile already keeps, by @name and @namespace, is a clash the
// person settles, and then they are kept. One with a system spark's @name never replaces
// it: it is added as the person's own under a name no spark has, as Rename does.

// Choice is how the person settles a clash.
type Choice string

const (
	// Replace puts the imported source in place of the kept one's, which keeps its id,
	// whether it is on, and its switches for each site.
	Replace Choice = "replace"
	// Rename keeps both: the imported one is added under a name no spark has.
	Rename Choice = "rename"
	// Ignore keeps the kept one as it is.
	Ignore Choice = "ignore"
)

// Clash is a spark of an import whose @name and @namespace a kept spark has.
type Clash struct {
	Index       int    `json:"index"`
	File        string `json:"file"`
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	KeptVersion string `json:"keptVersion,omitempty"`
	// RenameTo is the name Rename would give it.
	RenameTo string `json:"renameTo"`
}

// Import is an import between being read and being kept.
type Import struct {
	found     []Found
	skipped   []Skipped
	unchanged []string
	// kept is the id of the spark each clash is with, by its index in found.
	kept    map[int]string
	clashes []Clash
	// renames is the name each clash, and each spark named as a system spark is, is added
	// under when renamed, by its index in found; beside is those named as a system spark.
	renames map[int]string
	beside  map[int]bool
}

// Imported is what an import did: the names of the sparks it added, replaced, renamed,
// and ignored as the person chose, those the profile already had as they are, and what it
// skipped, with why.
type Imported struct {
	Added     []string  `json:"added"`
	Replaced  []string  `json:"replaced"`
	Renamed   []Renamed `json:"renamed"`
	Ignored   []string  `json:"ignored"`
	Unchanged []string  `json:"unchanged"`
	Skipped   []Skipped `json:"skipped"`
}

// Renamed is a spark added beside one of the same name: the name it came with and the one
// it has.
type Renamed struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Prepare sorts what Unpack found. A spark the profile keeps with the same source, a
// system spark's own file, or one the same import holds twice, is skipped; one it keeps
// with another source is a clash; one with a system spark's name and another source is
// renamed. The rest are added when the import is kept.
func (m *Manager) Prepare(found []Found, skipped []Skipped) *Import {
	im := &Import{skipped: skipped, kept: map[int]string{}, renames: map[int]string{}, beside: map[int]bool{}}
	if im.skipped == nil {
		im.skipped = []Skipped{}
	}
	seen := map[string]bool{}
	for _, f := range found {
		k := key(f.meta)
		if seen[k] {
			im.skipped = append(im.skipped, Skipped{File: f.File, Name: f.meta.Name, Reason: "in the import twice, the first one used"})
			continue
		}
		seen[k] = true
		e := m.same(f.meta)
		s := m.systemNamed(f.meta.Name)
		if e != nil && e.source == f.Source || e == nil && s != nil && s.source == f.Source {
			im.unchanged = append(im.unchanged, f.meta.Name)
			continue
		}
		i := len(im.found)
		im.found = append(im.found, f)
		switch {
		case e != nil:
			im.kept[i] = e.id
			im.clashes = append(im.clashes, Clash{
				Index: i, File: f.File, Name: f.meta.Name,
				Version: f.meta.Version, KeptVersion: e.meta.Version,
			})
		case s != nil:
			im.beside[i] = true
		}
	}
	// Each rename's new name is free of every spark kept and of the rest of the import.
	taken := m.taken()
	for _, f := range im.found {
		taken[key(f.meta)] = true
	}
	for i, f := range im.found {
		if _, clash := im.kept[i]; !clash && !im.beside[i] {
			continue
		}
		name := freeName(f.meta, taken)
		taken[key(Meta{Name: name, Namespace: f.meta.Namespace})] = true
		im.renames[i] = name
	}
	for i := range im.clashes {
		im.clashes[i].RenameTo = im.renames[im.clashes[i].Index]
	}
	return im
}

// taken is the key of every spark, the person's and the system's.
func (m *Manager) taken() map[string]bool {
	taken := map[string]bool{}
	for _, e := range m.all() {
		taken[key(e.meta)] = true
	}
	return taken
}

// Clashes is the sparks of the import the person has to choose for.
func (im *Import) Clashes() []Clash { return im.clashes }

// Keep keeps the import: each clash as choices says by its index, Ignore when it says
// nothing, and the rest added, on, after the others, while there is room.
func (m *Manager) Keep(im *Import, choices map[int]Choice) Imported {
	out := Imported{Added: []string{}, Replaced: []string{}, Renamed: []Renamed{}, Ignored: []string{},
		Unchanged: append([]string{}, im.unchanged...), Skipped: im.skipped}
	changed := false
	for i, f := range im.found {
		source, name := f.Source, f.meta.Name
		if im.beside[i] {
			name = im.renames[i]
			source = renamed(source, name)
		} else if id, ok := im.kept[i]; ok {
			choice := choices[i]
			if e := m.find(id); e != nil && choice == Replace {
				if err := m.put(e, source); err != nil {
					out.Skipped = append(out.Skipped, Skipped{File: f.File, Name: name, Reason: err.Error()})
					continue
				}
				out.Replaced = append(out.Replaced, name)
				changed = true
				continue
			}
			switch choice {
			case Rename:
				name = im.renames[i]
				source = renamed(source, name)
			case Replace:
				// The kept one went while the person chose: this one is added.
			default:
				out.Ignored = append(out.Ignored, name)
				continue
			}
		}
		if _, err := m.add(source); err != nil {
			out.Skipped = append(out.Skipped, Skipped{File: f.File, Name: name, Reason: err.Error()})
			continue
		}
		changed = true
		if name != f.meta.Name {
			out.Renamed = append(out.Renamed, Renamed{From: f.meta.Name, To: name})
		} else {
			out.Added = append(out.Added, name)
		}
	}
	if changed {
		m.changed()
	}
	return out
}

// same is the kept spark with meta's @name and @namespace, or nil.
func (m *Manager) same(meta Meta) *entry {
	for _, e := range m.list {
		if e.err == "" && key(e.meta) == key(meta) {
			return e
		}
	}
	return nil
}

// key is what says which spark a header is: its @name, as other managers compare it, and
// its @namespace.
func key(meta Meta) string { return meta.Namespace + "\x00" + meta.Name }

// freeName is meta's name with the first of " 2", " 3"... that no spark taken has, the
// name cut short to stay within MaxName.
func freeName(meta Meta, taken map[string]bool) string {
	for n := 2; ; n++ {
		name := fit(meta.Name, fmt.Sprintf(" %d", n))
		if !taken[key(Meta{Name: name, Namespace: meta.Namespace})] {
			return name
		}
	}
}

// fit is base with suffix after it, base cut short to stay within MaxName.
func fit(base, suffix string) string {
	for utf8.RuneCountInString(base)+utf8.RuneCountInString(suffix) > MaxName {
		_, size := utf8.DecodeLastRuneInString(base)
		base = base[:len(base)-size]
	}
	return strings.TrimRight(base, " ") + suffix
}

// renamed is source with its header's @name set to name.
func renamed(source, name string) string {
	lines := strings.SplitAfter(source, "\n")
	in := false
	for i, line := range lines {
		text, ok := comment(line)
		switch {
		case !in && ok && text == "==UserScript==":
			in = true
		case in && ok && text == "==/UserScript==":
			return source
		case in:
			if key, value := field(line); key == "@name" {
				j := strings.LastIndex(line, value)
				lines[i] = line[:j] + name + line[j+len(value):]
				return strings.Join(lines, "")
			}
		}
	}
	return source
}

// add keeps a new spark from its source, on, after the others, without telling anyone.
func (m *Manager) add(source string) (*entry, error) {
	if len(m.list) >= MaxSparks {
		return nil, errors.New("You can keep up to 25 sparks.")
	}
	t := now()
	e := &entry{id: m.newID(), on: true, added: t, modified: t}
	if err := e.set(source); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		timeline.Errorf("sparks: %v", err)
		return nil, errors.New("The spark could not be saved.")
	}
	if err := writeFile(m.sourcePath(e.id), []byte(source)); err != nil {
		return nil, errors.New("The spark could not be saved.")
	}
	m.list = append(m.list, e)
	return e, nil
}

// put puts a new source in place of a kept spark's, without telling anyone.
func (m *Manager) put(e *entry, source string) error {
	if _, err := Parse(source); err != nil {
		return err
	}
	if err := writeFile(m.sourcePath(e.id), []byte(source)); err != nil {
		return errors.New("The spark could not be saved.")
	}
	e.set(source)
	e.modified = now()
	return nil
}
