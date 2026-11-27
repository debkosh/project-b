package sparks

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
	"time"
)

// The limits on what an import reads: a zip of more files than a profile keeps sparks is
// still read, as one may hold other files beside them, but no more than this.
const (
	MaxPicked     = 8 << 20
	maxZipEntries = 500
)

// Picked is a file the person picked to import: its name and what it holds, or, for one
// too large to send, only its size.
type Picked struct {
	Name string
	Data []byte
	Size int64
}

// Found is a spark in what was picked: the file it came from, its source, and its header.
type Found struct {
	File   string
	Source string
	meta   Meta
}

// Skipped is a file or a spark not imported, and why, as a sentence to show the person.
type Skipped struct {
	File   string `json:"file"`
	Name   string `json:"name,omitempty"`
	Reason string `json:"reason"`
}

// Unpack finds the sparks in the files picked. A .zip gives each .user.js inside it, at
// any depth, and leaves out the rest, as a Mac's __MACOSX folder and hidden files; any
// other file is taken as one spark's source. What doesn't read is skipped, with why.
func Unpack(files []Picked) ([]Found, []Skipped) {
	var found []Found
	var skipped []Skipped
	take := func(file string, data []byte) {
		source := string(data)
		meta, err := Parse(source)
		if err != nil {
			skipped = append(skipped, Skipped{File: file, Reason: err.Error()})
			return
		}
		found = append(found, Found{File: file, Source: source, meta: meta})
	}
	for _, f := range files {
		switch {
		case len(f.Data) > MaxPicked || f.Size > MaxPicked:
			skipped = append(skipped, Skipped{File: f.Name, Reason: "The file is larger than 8 MB."})
		case strings.EqualFold(path.Ext(f.Name), ".zip"):
			if err := unzip(f, take); err != nil {
				skipped = append(skipped, Skipped{File: f.Name, Reason: err.Error()})
			}
		default:
			take(f.Name, f.Data)
		}
	}
	return found, skipped
}

// unzip hands take each .user.js in a zip, by its name in the zip.
func unzip(f Picked, take func(file string, data []byte)) error {
	r, err := zip.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)))
	if err != nil {
		return errors.New("The file is not a zip that can be read.")
	}
	if len(r.File) > maxZipEntries {
		return fmt.Errorf("The zip holds more than %d files.", maxZipEntries)
	}
	for _, e := range r.File {
		name := e.Name
		if e.FileInfo().IsDir() || !inZip(name) {
			continue
		}
		file := f.Name + " › " + path.Base(name)
		rc, err := e.Open()
		if err != nil {
			take(file, nil)
			continue
		}
		// One byte past the limit is enough for Parse to say it is too large.
		data, err := io.ReadAll(io.LimitReader(rc, MaxSource+1))
		rc.Close()
		if err != nil {
			take(file, nil)
			continue
		}
		take(file, data)
	}
	return nil
}

// inZip says whether a zip's entry is a spark to import: a .user.js, outside __MACOSX and
// any hidden folder, and not hidden itself.
func inZip(name string) bool {
	if !strings.HasSuffix(strings.ToLower(name), ".user.js") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "__MACOSX" || strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

// Zip is a zip of the sparks ids names, every spark when it names none: the person's in
// order, then the system sparks, each as the .user.js it is kept as, by its name: what
// Unpack, and other browsers' userscript managers, read back. A spark whose file doesn't
// read is left out. It says how many are in it.
func (m *Manager) Zip(ids []string) ([]byte, int) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	used := map[string]bool{}
	n := 0
	for _, e := range m.all() {
		if e.err != "" || len(ids) > 0 && !slices.Contains(ids, e.id) {
			continue
		}
		name := m.FileName(e.id)
		stem := strings.TrimSuffix(name, ".user.js")
		for i := 2; used[strings.ToLower(name)]; i++ {
			name = fmt.Sprintf("%s (%d).user.js", stem, i)
		}
		used[strings.ToLower(name)] = true
		fw, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			continue
		}
		fw.Write([]byte(e.source))
		n++
	}
	w.Close()
	return buf.Bytes(), n
}
