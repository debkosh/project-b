package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

// The release's manifest (docs/blocking.md, "Building and publishing them"). The app takes only
// a manifest signed with our key, and every list it ends with, whole or patched, must match
// its sha256 here.
type Manifest struct {
	Format    int               `json:"format"`
	Published string            `json:"published"`
	Lists     map[string]*Entry `json:"lists"`
	// The scriptlets and stand-ins, adblock-rust's resources JSON, by id like the lists: each
	// is a JSON array, and the app gives the library all of them as one. One today, "resources".
	Resources map[string]*Entry `json:"resources"`
}

type Entry struct {
	SHA256  string            `json:"sha256"`  // of the uncompressed text
	Size    int64             `json:"size"`    // of the uncompressed text
	File    string            `json:"file"`    // the whole text, zstd
	Changed string            `json:"changed"` // when it last changed, RFC 3339
	Patches map[string]string `json:"patches"` // an earlier version's sha256: the patch from it to this one
	// The earlier versions there are patches from, newest first, at most maxHistory. Their texts
	// stay in the release, in <id>-versions.tar.zst, as the next change's patches are made
	// from them.
	History []string `json:"history"`
	// Each source's size at its last good download, for the shrink check. Only this program
	// reads it.
	Sources map[string]int64 `json:"sources,omitempty"`
}

const (
	maxHistory     = 8
	manifestName   = "manifest.json"
	signatureName  = "manifest.json.sig"
	licencesName   = "LICENSES.txt"
	resourcesID    = "resources"
	maxTextSize    = 256 << 20
	maxSourceShare = 2 // a source that shrank to less than 1/maxSourceShare of its last size is refused
)

type config struct {
	catalog       *Catalog
	out           string // the new files, for the upload
	keep          string // where the names of every file the release keeps are written
	prev          store
	fetch         *fetcher
	workers       int
	resources     string // adblock-rust's resources JSON: a file, or an https address
	resourcesFrom origin // the commits a resources file was built from
	ownResources  string // a folder of our own scriptlets, *.js
	key           ed25519.PrivateKey
	now           func() time.Time
	// Whether the library takes a list's text; nil takes every one.
	check func(id string, text []byte) error
}

// item is one thing the manifest names: a list, or the resources.
type item struct {
	id, ext string
	text    []byte           // nil: keep the previous version
	sources map[string]int64 // the size of each source this text came from
}

func (it item) file(sha string) string { return fmt.Sprintf("%s-%s.%s.zst", it.id, sha[:16], it.ext) }
func (it item) versions() string       { return it.id + "-versions.tar.zst" }
func (it item) patch(old, new string) string {
	return fmt.Sprintf("%s-%s-%s.patch.zst", it.id, old[:16], new[:16])
}

// run publishes one round: it reports whether anything changed.
func run(ctx context.Context, c config) (bool, error) {
	prev, err := loadPrevious(c.prev)
	if err != nil {
		return false, err
	}
	var urls []string
	for _, l := range c.catalog.Lists {
		for _, s := range l.Sources {
			if !slices.Contains(urls, s.URL) {
				urls = append(urls, s.URL)
			}
		}
	}
	resFile := c.resources != "" && !strings.HasPrefix(c.resources, "https://")
	if c.resources != "" && !resFile {
		urls = append(urls, c.resources)
	}
	got := c.fetch.fetchAll(ctx, urls, c.workers)
	if resFile {
		// Built by the workflow; missing when that failed, which keeps the previous version.
		b, err := os.ReadFile(c.resources)
		if err == nil {
			err = checkText(b, "")
		}
		got[c.resources] = fetched{b, err}
	}

	var items []item
	for _, l := range c.catalog.Lists {
		items = append(items, buildList(l, got, prev.Lists[l.ID]))
	}
	checkLists(c, items, prev)
	if c.resources != "" {
		it, err := buildResources(c, got[c.resources], prev.Resources[resourcesID])
		if err != nil {
			return false, err
		}
		items = append(items, it)
	}

	now := c.now().UTC().Format(time.RFC3339)
	next := &Manifest{Format: 1, Lists: map[string]*Entry{}, Resources: map[string]*Entry{}}
	type made struct {
		entry *Entry
		files map[string][]byte
	}
	results := make([]made, len(items))
	var wg sync.WaitGroup
	sem := make(chan struct{}, c.workers)
	errs := make([]error, len(items))
	for i, it := range items {
		var old *Entry
		if it.id == resourcesID {
			old = prev.Resources[resourcesID]
		} else {
			old = prev.Lists[it.id]
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			e, files, err := publishItem(c.prev, it, old, now)
			results[i], errs[i] = made{e, files}, err
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return false, err
	}
	files := map[string][]byte{}
	for i, it := range items {
		e := results[i].entry
		if e == nil {
			continue
		}
		if it.id == resourcesID {
			next.Resources[resourcesID] = e
		} else {
			next.Lists[it.id] = e
		}
		for n, b := range results[i].files {
			files[n] = b
		}
	}
	licences := licencesText(c.catalog, c.resourcesFrom)

	changed := len(files) > 0 || !sameManifest(prev, next) || !bytes.Equal(prev.licences, licences)
	if !changed {
		next.Published = prev.Published
	} else {
		next.Published = now
	}
	if err := writeKeep(c.keep, next); err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}
	mb, err := json.MarshalIndent(next, "", " ")
	if err != nil {
		return false, err
	}
	mb = append(mb, '\n')
	files[manifestName] = mb
	files[signatureName] = []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(c.key, mb)) + "\n")
	files[licencesName] = licences
	if err := os.MkdirAll(c.out, 0o755); err != nil {
		return false, err
	}
	for n, b := range files {
		if err := os.WriteFile(filepath.Join(c.out, n), b, 0o644); err != nil {
			return false, err
		}
	}
	return true, nil
}

type previous struct {
	*Manifest
	licences []byte
}

func loadPrevious(s store) (previous, error) {
	p := previous{Manifest: &Manifest{Format: 1, Lists: map[string]*Entry{}, Resources: map[string]*Entry{}}}
	b, err := s.get(manifestName)
	if errors.Is(err, errNotFound) {
		log.Printf("no manifest in the release yet: publishing every list whole")
		return p, nil
	}
	if err != nil {
		// Never start again from nothing over a failed download: that would drop every list's
		// history, and every app would fetch every list whole.
		return p, fmt.Errorf("the release's manifest: %w", err)
	}
	if err := json.Unmarshal(b, p.Manifest); err != nil {
		return p, fmt.Errorf("the release's manifest: %w", err)
	}
	if p.Format != 1 {
		return p, fmt.Errorf("the release's manifest has format %d", p.Format)
	}
	if p.Lists == nil {
		p.Lists = map[string]*Entry{}
	}
	if p.Resources == nil {
		p.Resources = map[string]*Entry{}
	}
	if p.licences, err = s.get(licencesName); err != nil && !errors.Is(err, errNotFound) {
		return p, fmt.Errorf("the release's %s: %w", licencesName, err)
	}
	return p, nil
}

// buildList makes a list's text from its sources, or keeps the previous text when any source
// failed, isn't text, or shrank by more than half.
func buildList(l List, got map[string]fetched, old *Entry) item {
	it := item{id: l.ID, ext: "txt", sources: map[string]int64{}}
	var text bytes.Buffer
	for _, s := range l.Sources {
		f := got[s.URL]
		err := f.err
		if err == nil {
			err = shrunk(f.body, s.URL, old)
		}
		if err != nil {
			log.Printf("%s: keeping the previous text: %s: %v", l.ID, s.URL, err)
			return item{id: l.ID, ext: "txt"}
		}
		body := f.body
		if s.Format == "hosts" {
			body = hostsToRules(body, s.URL)
		}
		it.sources[s.URL] = int64(len(f.body))
		if text.Len() > 0 {
			text.WriteByte('\n')
		}
		text.Write(body)
		if !bytes.HasSuffix(body, []byte("\n")) {
			text.WriteByte('\n')
		}
	}
	it.text = text.Bytes()
	return it
}

func shrunk(body []byte, url string, old *Entry) error {
	if old == nil {
		return nil
	}
	if was := old.Sources[url]; was > 0 && int64(len(body))*maxSourceShare < was {
		return fmt.Errorf("shrank from %d to %d bytes", was, len(body))
	}
	return nil
}

// hostsToRules turns a hosts file (an address and a host, or a host alone, per line) into the
// rules adblock-rust makes of one, ||host^, since the library reads every list as standard.
func hostsToRules(b []byte, url string) []byte {
	var out bytes.Buffer
	fmt.Fprintf(&out, "! Made from the hosts file %s, each host as ||host^\n", url)
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '!' {
			continue
		}
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		var host string
		switch f := strings.Fields(line); len(f) {
		case 1:
			host = f[0]
		case 2:
			host = f[1]
		default:
			continue
		}
		host = strings.ToLower(host)
		if !validHost(host) {
			continue
		}
		out.WriteString("||" + host + "^\n")
	}
	return out.Bytes()
}

func validHost(h string) bool {
	if !strings.Contains(h, ".") || strings.HasPrefix(h, ".") || strings.HasSuffix(h, ".") {
		return false
	}
	switch h {
	case "0.0.0.0", "127.0.0.1", "255.255.255.255", "localhost.localdomain":
		return false
	}
	for _, r := range h {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '.' || r == '_') {
			return false
		}
	}
	return true
}

// buildResources takes the resources JSON, uBO's and Brave's as the workflow builds it, or
// Brave's alone, and adds our own scriptlets, a file per one.
func buildResources(c config, brave fetched, old *Entry) (item, error) {
	it := item{id: resourcesID, ext: "json"}
	err := brave.err
	if err == nil {
		err = shrunk(brave.body, c.resources, old)
	}
	var all []json.RawMessage
	if err == nil {
		err = json.Unmarshal(brave.body, &all)
	}
	if err == nil {
		err = checkResources(all)
	}
	if err != nil {
		log.Printf("resources: keeping the previous version: %v", err)
		return it, nil
	}
	if c.ownResources != "" {
		names, err := filepath.Glob(filepath.Join(c.ownResources, "*.js"))
		if err != nil {
			return it, err
		}
		sort.Strings(names)
		for _, n := range names {
			b, err := os.ReadFile(n)
			if err != nil {
				return it, err
			}
			r, err := json.Marshal(map[string]any{
				"name":    filepath.Base(n),
				"aliases": []string{},
				"kind":    map[string]string{"mime": "application/javascript"},
				"content": base64.StdEncoding.EncodeToString(b),
			})
			if err != nil {
				return it, err
			}
			all = append(all, r)
		}
		if err := checkResources(all); err != nil {
			return it, fmt.Errorf("our own resources: %w", err)
		}
	}
	text, err := json.Marshal(all)
	if err != nil {
		return it, err
	}
	it.text = append(text, '\n')
	it.sources = map[string]int64{c.resources: int64(len(brave.body))}
	return it, nil
}

func checkResources(all []json.RawMessage) error {
	if len(all) == 0 {
		return errors.New("no resources")
	}
	seen := map[string]bool{}
	for _, raw := range all {
		var r struct {
			Name    string          `json:"name"`
			Kind    json.RawMessage `json:"kind"`
			Content string          `json:"content"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return err
		}
		if r.Name == "" || len(r.Kind) == 0 || seen[r.Name] {
			return fmt.Errorf("resource %q: no name or kind, or a repeated name", r.Name)
		}
		if _, err := base64.StdEncoding.DecodeString(r.Content); err != nil {
			return fmt.Errorf("resource %s: %w", r.Name, err)
		}
		seen[r.Name] = true
	}
	return nil
}

// publishItem makes an item's entry: the previous one when its text is the same, else a new
// whole file, a patch from each of its last versions, and the archive of those versions. A nil
// entry: the item has no text yet (its first download failed).
func publishItem(prev store, it item, old *Entry, now string) (*Entry, map[string][]byte, error) {
	if it.text == nil {
		return old, nil, nil
	}
	sum := sha256.Sum256(it.text)
	sha := hex.EncodeToString(sum[:])
	if old != nil && old.SHA256 == sha {
		// The same text: the same entry, sizes and all, so the manifest stays byte for byte
		// what it was and the apps' ETag check finds nothing new.
		return old, nil, nil
	}
	whole, err := compress(it.text, nil)
	if err != nil {
		return nil, nil, err
	}
	e := &Entry{SHA256: sha, Size: int64(len(it.text)), File: it.file(sha), Changed: now,
		Patches: map[string]string{}, History: []string{}, Sources: it.sources}
	files := map[string][]byte{e.File: whole}
	if old != nil {
		texts := previousTexts(prev, it, old)
		var kept [][]byte
		for _, v := range append([]string{old.SHA256}, old.History...) {
			if len(e.History) == maxHistory {
				break
			}
			if v == sha || slices.Contains(e.History, v) {
				continue
			}
			oldText := texts[v]
			if oldText == nil {
				log.Printf("%s: no patch from %s: its text isn't in the release", it.id, v[:16])
				continue
			}
			p, err := compress(it.text, oldText)
			if err != nil {
				return nil, nil, err
			}
			name := it.patch(v, sha)
			files[name] = p
			e.Patches[v] = name
			e.History = append(e.History, v)
			kept = append(kept, oldText)
		}
		if len(kept) > 0 {
			a, err := versionsArchive(e.History, kept)
			if err != nil {
				return nil, nil, err
			}
			files[it.versions()] = a
		}
	}
	log.Printf("%s: changed, %d bytes, %d patches", it.id, len(it.text), len(e.Patches))
	return e, files, nil
}

// previousTexts reads the texts of an entry's version and the versions before it: its whole
// file, and the archive of the rest. Each is checked against its sha256; one that is missing or
// doesn't match is left out, and only loses its patch.
func previousTexts(prev store, it item, old *Entry) map[string][]byte {
	texts := map[string][]byte{}
	keep := func(text []byte) {
		sum := sha256.Sum256(text)
		h := hex.EncodeToString(sum[:])
		if h == old.SHA256 || slices.Contains(old.History, h) {
			texts[h] = text
		}
	}
	if z, err := prev.get(old.File); err == nil {
		if text, err := decompress(z); err == nil {
			keep(text)
		}
	}
	if len(old.History) == 0 {
		return texts
	}
	z, err := prev.get(it.versions())
	if err == nil {
		var tb []byte
		if tb, err = decompress(z); err == nil {
			r := tar.NewReader(bytes.NewReader(tb))
			for {
				if _, err = r.Next(); err != nil {
					break
				}
				var text []byte
				if text, err = io.ReadAll(io.LimitReader(r, maxTextSize)); err != nil {
					break
				}
				keep(text)
			}
		}
	}
	if err != nil && err != io.EOF {
		log.Printf("%s: the earlier versions: %v", it.id, err)
	}
	return texts
}

// versionsArchive is a tar of the earlier versions' texts, newest first, each named by its
// sha256, in one zstd frame whose window spans two of them: the versions differ in a few lines,
// so the archive is little bigger than one. Only this program reads it, to make the next
// change's patches; one file per list keeps the release far below GitHub's 1000 files.
func versionsArchive(shas []string, texts [][]byte) ([]byte, error) {
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	largest := 0
	for i, t := range texts {
		largest = max(largest, len(t))
		if err := w.WriteHeader(&tar.Header{Name: shas[i], Mode: 0o644, Size: int64(len(t)), Format: tar.FormatPAX}); err != nil {
			return nil, err
		}
		if _, err := w.Write(t); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	win := zstd.MinWindowSize
	for win < 2*largest+4096 && win < zstd.MaxWindowSize {
		win *= 2
	}
	e, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBetterCompression), zstd.WithWindowSize(win), zstd.WithEncoderConcurrency(1))
	if err != nil {
		return nil, err
	}
	defer e.Close()
	return e.EncodeAll(b.Bytes(), nil), nil
}

func decompress(z []byte) ([]byte, error) {
	d, err := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(maxTextSize*maxHistory), zstd.WithDecoderConcurrency(1))
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.DecodeAll(z, nil)
}

// compress is zstd of text; with dict, zstd with dict as its raw dictionary (id 0): the patch
// from dict to text. The window spans both, so the whole old text can be referred to.
func compress(text, dict []byte) ([]byte, error) {
	opts := []zstd.EOption{zstd.WithEncoderLevel(zstd.SpeedBestCompression), zstd.WithEncoderConcurrency(1)}
	if dict != nil {
		w := zstd.MinWindowSize
		for w < len(dict)+len(text) && w < zstd.MaxWindowSize {
			w *= 2
		}
		opts = append(opts, zstd.WithEncoderDictRaw(0, dict), zstd.WithWindowSize(w))
	}
	e, err := zstd.NewWriter(nil, opts...)
	if err != nil {
		return nil, err
	}
	defer e.Close()
	return e.EncodeAll(text, nil), nil
}

func sameManifest(a previous, b *Manifest) bool {
	x := *a.Manifest
	y := *b
	x.Published, y.Published = "", ""
	xb, err1 := json.Marshal(x)
	yb, err2 := json.Marshal(y)
	return err1 == nil && err2 == nil && bytes.Equal(xb, yb)
}

// writeKeep writes the names of every file the release keeps: the manifest and its signature,
// the licences, and each entry's whole file, patches and archive of earlier versions. The
// workflow deletes the rest.
func writeKeep(path string, m *Manifest) error {
	if path == "" {
		return nil
	}
	names := []string{manifestName, signatureName, licencesName}
	add := func(it item, e *Entry) {
		names = append(names, e.File)
		for _, p := range e.Patches {
			names = append(names, p)
		}
		if len(e.History) > 0 {
			names = append(names, it.versions())
		}
	}
	for id, e := range m.Lists {
		add(item{id: id, ext: "txt"}, e)
	}
	for id, e := range m.Resources {
		add(item{id: id, ext: "json"}, e)
	}
	sort.Strings(names)
	names = slices.Compact(names)
	return os.WriteFile(path, []byte(strings.Join(names, "\n")+"\n"), 0o644)
}

// origin is the commits a resources file was built from; empty, Brave's alone from its
// address.
type origin struct{ ubo, brave string }

// licencesText is LICENSES.txt: each list's title, licence, home and sources.
func licencesText(c *Catalog, res origin) []byte {
	var b bytes.Buffer
	b.WriteString("Project B's blocking lists\n\n" +
		"Each list here is its sources' text, unchanged in meaning: several sources are joined\n" +
		"one after the other, and a hosts file's hosts are written as ||host^ rules. Each stays\n" +
		"under its own licence, named below with its home. Project B's own lists are MIT.\n")
	for _, l := range c.Lists {
		fmt.Fprintf(&b, "\n%s (%s)\n  Licence: %s\n  Home: %s\n  Sources:\n", l.Title, l.ID, l.Licence, l.Home)
		for _, s := range l.Sources {
			fmt.Fprintf(&b, "    %s\n", s.URL)
		}
	}
	b.WriteString("\nResources (resources)\n")
	if res.ubo == "" {
		b.WriteString("  The scriptlets and stand-in scripts the lists name: Brave's, from its adblock-resources\n" +
			"  repository's dist/resources.json, and Project B's own test scriptlet (MIT).\n" +
			"  Licence: MPL-2.0 (Brave's adblock-resources)\n" +
			"  Home: https://github.com/brave/adblock-resources\n")
		return b.Bytes()
	}
	brave := "https://github.com/brave/adblock-resources"
	if res.brave != "" {
		brave += "/tree/" + res.brave
	}
	fmt.Fprintf(&b, "  The scriptlets and stand-in scripts the lists name, put together as Brave does:\n"+
		"  uBlock Origin's scriptlets and redirect resources, Brave's own from its adblock-resources\n"+
		"  repository's dist/resources.json, and Project B's own test scriptlet (MIT).\n"+
		"  uBlock Origin's:\n    Licence: GPL-3.0\n    Home: https://github.com/gorhill/uBlock/tree/%s\n"+
		"  Brave's:\n    Licence: MPL-2.0\n    Home: %s\n", res.ubo, brave)
	return b.Bytes()
}
