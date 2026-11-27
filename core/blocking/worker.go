package blocking

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// The lists' folder:
//
//	manifest.json, manifest.json.sig, manifest.etag  the last manifest taken, and its ETag
//	texts/<list>-<sha256>.txt                        each list's text
//	resources/<id>-<sha256>.json                     the scriptlets and stand-ins
//	engines/<blocker>-<unit>-<key>.engine            each unit's compiled engine
//
// Every file is written aside and renamed into place, so another profile's copy of the app
// reads a whole one or none.

// worker is the manager's goroutine's: after Load, nothing else touches it.
type worker struct {
	m       *Manager
	choices Choices
	// manifest is the last one taken, nil before any; etag its ETag.
	manifest *Manifest
	etag     string
	// texts and resources are the SHA-256 of each text on disk, by id.
	texts, resources map[string]string
	// loaded is each unit's key loaded, by its file's prefix; "" for none.
	loaded map[string]string
	// safe is each safe list's blocker, made once: the library has no way to free one.
	safe    map[string]Blocker
	builtin bool
	started bool
}

// engineAge is how long an engine no copy of the app loaded stays on disk: another
// profile may have other lists on.
const engineAge = 30 * 24 * time.Hour

func (w *worker) run() {
	defer close(w.m.done)
	var tick <-chan time.Time
	var timer *time.Timer
	for {
		select {
		case <-w.m.ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-w.m.wake:
		case <-tick:
			w.m.mu.Lock()
			w.m.fetch = true
			w.m.mu.Unlock()
		}
		w.m.mu.Lock()
		pending, fetch, started := w.m.pending, w.m.fetch, w.m.started
		w.m.pending, w.m.fetch = nil, false
		w.m.mu.Unlock()
		if pending != nil {
			w.choices = *pending
		}
		if started && !w.started {
			w.started = true
			timer = time.NewTimer(w.m.s.Every)
			tick = timer.C
		}
		resources := false
		if fetch && w.m.s.BaseURL != "" {
			var err error
			if resources, err = w.fetch(); err != nil {
				errorf("%v", err)
			}
			if w.started {
				timer.Reset(w.m.s.Every)
			}
		}
		if resources {
			w.giveResources()
		}
		w.reconcile(true, resources)
		w.m.statusChanged(w.statusNow())
	}
}

// scan reads what is on disk: the texts, and the manifest last taken.
func (w *worker) scan() {
	w.texts = scanTexts(filepath.Join(w.m.s.Dir, "texts"), ".txt")
	w.resources = scanTexts(filepath.Join(w.m.s.Dir, "resources"), ".json")
	b, err1 := os.ReadFile(filepath.Join(w.m.s.Dir, "manifest.json"))
	sig, err2 := os.ReadFile(filepath.Join(w.m.s.Dir, "manifest.json.sig"))
	if err1 != nil || err2 != nil {
		return
	}
	man, err := Verify(w.m.s.Key, b, sig)
	if err != nil {
		errorf("the manifest on disk: %v", err)
		return
	}
	w.manifest = man
	if e, err := os.ReadFile(filepath.Join(w.m.s.Dir, "manifest.etag")); err == nil {
		w.etag = strings.TrimSpace(string(e))
	}
}

// scanTexts is the texts in dir, <id>-<sha256><ext>, by id: the newest where there are two.
func scanTexts(dir, ext string) map[string]string {
	out := map[string]string{}
	newest := map[string]time.Time{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ext)
		i := strings.LastIndexByte(name, '-')
		if !ok || i < 0 || !sha256Hex.MatchString(name[i+1:]) || !fileID.MatchString(name[:i]) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if id := name[:i]; info.ModTime().After(newest[id]) {
			out[id], newest[id] = name[i+1:], info.ModTime()
		}
	}
	return out
}

func (w *worker) textPath(id string) string {
	return filepath.Join(w.m.s.Dir, "texts", id+"-"+w.texts[id]+".txt")
}

func (w *worker) resourcePath(id string) string {
	return filepath.Join(w.m.s.Dir, "resources", id+"-"+w.resources[id]+".json")
}

// giveResources hands every blocker the resources on disk, as one JSON array.
func (w *worker) giveResources() {
	if len(w.resources) == 0 {
		return
	}
	var all []json.RawMessage
	for id := range w.resources {
		b, err := os.ReadFile(w.resourcePath(id))
		var some []json.RawMessage
		if err == nil {
			err = json.Unmarshal(b, &some)
		}
		if err != nil {
			errorf("resources %s: %v", id, err)
			continue
		}
		all = append(all, some...)
	}
	b, _ := json.Marshal(all)
	for _, bl := range w.blockers() {
		if !bl.Resources(b) {
			errorf("the library refused the resources")
		}
	}
}

func (w *worker) blockers() []Blocker {
	out := []Blocker{w.m.network}
	for _, b := range w.safe {
		out = append(out, b)
	}
	return out
}

// units is every unit there can be, each with the lists of it the choices want and whose
// texts are here; one with none is to be unloaded.
func (w *worker) units() []unit {
	cat, c := w.m.cat, w.choices
	var out []unit
	for _, g := range cat.Groups {
		net := unit{name: g.ID, blocker: Network, group: g, b: w.m.network}
		hasNet := false
		for _, l := range cat.InGroup(g.ID) {
			if cat.BlockerOf(l) == Network {
				hasNet = true
				if c.Network && w.m.On(c, l) && w.texts[l.ID] != "" {
					net.lists = append(net.lists, l)
				}
				continue
			}
			u := unit{name: l.ID, blocker: Safe, group: g}
			if c.Safe && w.m.On(c, l) && w.texts[l.ID] != "" {
				u.lists = []List{l}
			}
			if w.safe[l.ID] == nil && len(u.lists) > 0 {
				w.safe[l.ID] = w.m.s.Library.New()
			}
			u.b = w.safe[l.ID]
			out = append(out, u)
		}
		if hasNet {
			out = append(out, net)
		}
	}
	return out
}

// reconcile loads, compiles or unloads each unit for the choices and the texts on disk.
// compile false is the start's: an engine on disk is loaded or left for later. again loads
// every unit's engine again, as new resources need.
func (w *worker) reconcile(compile, again bool) {
	changed := false
	var safe []safeUnit
	for _, u := range w.units() {
		prefix := u.prefix()
		if len(u.lists) == 0 {
			if w.loaded[prefix] != "" {
				u.b.Unload(u.name)
				delete(w.loaded, prefix)
				changed = changed || u.blocker == Network
				logf("%s off", u.name)
			}
			continue
		}
		k := key(u.lists, w.texts)
		if w.loaded[prefix] == k && !again {
			if u.blocker == Safe {
				safe = append(safe, safeUnit{list: u.lists[0], group: u.group, b: u.b})
			}
			continue
		}
		if w.loadEngine(u, k) || compile && w.compile(u, k) {
			w.loaded[prefix] = k
			changed = changed || u.blocker == Network
			if u.blocker == Safe {
				safe = append(safe, safeUnit{list: u.lists[0], group: u.group, b: u.b})
			}
		}
	}
	w.m.safe.Store(&safe)
	if w.setBuiltin() {
		changed = true
	}
	if changed {
		w.m.rulesChanged()
	}
}

// setBuiltin loads the built-in hosts while Block ads is on and the ads group isn't loaded,
// unless the person turned all of its lists off, and unloads them otherwise. It says
// whether that changed.
func (w *worker) setBuiltin() bool {
	ads := w.m.cat.InGroup("ads")
	adsOn := len(ads) == 0
	for _, l := range ads {
		adsOn = adsOn || w.m.On(w.choices, l)
	}
	want := w.choices.Network && adsOn && w.loaded[unit{name: "ads", blocker: Network}.prefix()] == ""
	if want == w.builtin {
		return false
	}
	w.builtin = want
	if !want {
		w.m.network.Unload(Builtin)
		return true
	}
	var rules strings.Builder
	for _, h := range w.m.s.Hosts {
		rules.WriteString("||" + h + "^$third-party\n")
	}
	engine, err := w.m.s.Library.Compile([][]byte{[]byte(rules.String())}, nil)
	if err != nil || !w.m.network.Load(Builtin, engine) {
		errorf("the built-in hosts didn't load: %v", err)
		w.builtin = false
		return false
	}
	return true
}

// loadEngine loads a unit's engine from disk, if it is there and the library reads it.
func (w *worker) loadEngine(u unit, k string) bool {
	path := filepath.Join(w.m.s.Dir, "engines", u.file(k))
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if !u.b.Load(u.name, b) {
		logf("%s: the library can't read %s, another version's: compiling it again", u.name, filepath.Base(path))
		return false
	}
	now := time.Now()
	os.Chtimes(path, now, now)
	logf("%s loaded from %s", u.name, filepath.Base(path))
	return true
}

// compile compiles a unit's texts, keeps the engine and loads it.
func (w *worker) compile(u unit, k string) bool {
	var texts [][]byte
	var trusted []bool
	for _, l := range u.lists {
		b, err := os.ReadFile(w.textPath(l.ID))
		if err != nil {
			errorf("%s: %v", l.ID, err)
			return false
		}
		texts = append(texts, b)
		trusted = append(trusted, l.Trusted)
	}
	began := time.Now()
	engine, err := w.m.s.Library.Compile(texts, trusted)
	if err != nil {
		errorf("%s didn't compile: %v", u.name, err)
		return false
	}
	dir := filepath.Join(w.m.s.Dir, "engines")
	if err := writeAside(dir, u.file(k), engine); err != nil {
		errorf("%s: %v", u.name, err)
	}
	if !u.b.Load(u.name, engine) {
		errorf("%s: the library didn't load the engine it compiled", u.name)
		return false
	}
	logf("%s compiled from %d lists in %v, %d bytes", u.name, len(texts), time.Since(began).Round(time.Millisecond), len(engine))
	w.dropOldEngines(dir, u, k)
	return true
}

// dropOldEngines removes the unit's other engines that no copy of the app loaded for
// engineAge.
func (w *worker) dropOldEngines(dir string, u unit, k string) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		name := e.Name()
		rest, ok := strings.CutPrefix(name, u.prefix())
		if !ok || name == u.file(k) || !engineKey.MatchString(rest) {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > engineAge {
			os.Remove(filepath.Join(dir, name))
		}
	}
}

// statusNow is what Settings shows of the lists.
func (w *worker) statusNow() Status {
	st := Status{Lists: map[string]ListStatus{}}
	if w.manifest == nil {
		return st
	}
	st.Published = w.manifest.Published
	for id, e := range w.manifest.Lists {
		st.Lists[id] = ListStatus{Size: e.Size, Changed: e.Changed, Have: w.texts[id] != ""}
	}
	return st
}

// fetch takes the manifest, then the text of each list the choices want that changed, and
// the resources while Block ads is on. It says whether the resources changed.
func (w *worker) fetch() (bool, error) {
	if err := w.fetchManifest(); err != nil {
		return false, err
	}
	man := w.manifest
	if man == nil {
		return false, nil
	}
	for id := range w.m.wanted(w.choices) {
		if e, ok := man.Lists[id]; ok && w.texts[id] != e.SHA256 {
			w.update("texts", ".txt", id, e, w.texts)
		}
	}
	resources := false
	if w.choices.Network {
		for id, e := range man.Resources {
			if w.resources[id] != e.SHA256 {
				resources = w.update("resources", ".json", id, e, w.resources) || resources
			}
		}
	}
	return resources, nil
}

// fetchManifest asks for the manifest with the last ETag and takes a new one only signed,
// and no older than the one it has.
func (w *worker) fetchManifest() error {
	req, err := http.NewRequestWithContext(w.m.ctx, http.MethodGet, w.m.s.BaseURL+"manifest.json", nil)
	if err != nil {
		return err
	}
	if w.manifest != nil && w.etag != "" {
		req.Header.Set("If-None-Match", w.etag)
	}
	resp, err := w.m.s.Client.Do(req)
	if err != nil {
		return fmt.Errorf("the manifest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified && w.manifest != nil {
		logf("the manifest hasn't changed")
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the manifest: %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("the manifest: %v", err)
	}
	sig, err := w.get("manifest.json.sig", 1<<10)
	if err != nil {
		return err
	}
	man, err := Verify(w.m.s.Key, b, sig)
	if err != nil {
		return err
	}
	if w.manifest != nil && man.Published.Before(w.manifest.Published) {
		return fmt.Errorf("the manifest of %v is older than the one here, of %v", man.Published, w.manifest.Published)
	}
	dir := w.m.s.Dir
	etag := resp.Header.Get("ETag")
	if err := errors.Join(writeAside(dir, "manifest.json", b), writeAside(dir, "manifest.json.sig", sig), writeAside(dir, "manifest.etag", []byte(etag+"\n"))); err != nil {
		errorf("%v", err)
	}
	w.manifest, w.etag = man, etag
	logf("the manifest of %v", man.Published)
	return nil
}

// update takes one text: by its patch from the text here, if there is one, else whole; a
// text that isn't the manifest's, by its SHA-256, is thrown away. It says whether it took
// one.
func (w *worker) update(folder, ext, id string, e Entry, have map[string]string) bool {
	dir := filepath.Join(w.m.s.Dir, folder)
	old := have[id]
	oldPath := filepath.Join(dir, id+"-"+old+ext)
	var text []byte
	if p, ok := e.Patches[old]; ok && old != "" {
		base, err := os.ReadFile(oldPath)
		if err == nil {
			text, err = w.patched(p, base, e)
		}
		if err != nil {
			logf("%s: the patch %s: %v; fetching it whole", id, p, err)
			text = nil
		}
	}
	if text == nil {
		var err error
		if text, err = w.whole(e); err != nil {
			errorf("%s: %v", id, err)
			return false
		}
	}
	if err := writeAside(dir, id+"-"+e.SHA256+ext, text); err != nil {
		errorf("%s: %v", id, err)
		return false
	}
	have[id] = e.SHA256
	if old != "" {
		os.Remove(oldPath)
	}
	logf("%s: %d bytes, of %v", id, len(text), e.Changed)
	return true
}

func (w *worker) patched(file string, base []byte, e Entry) ([]byte, error) {
	b, err := w.get(file, maxText)
	if err != nil {
		return nil, err
	}
	text, err := unzstd(b, base, e.Size)
	return checked(text, err, e)
}

func (w *worker) whole(e Entry) ([]byte, error) {
	b, err := w.get(e.File, maxText)
	if err != nil {
		return nil, err
	}
	text, err := unzstd(b, nil, e.Size)
	return checked(text, err, e)
}

// checked is text if its SHA-256 is the manifest's.
func checked(text []byte, err error, e Entry) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(text)
	if hex.EncodeToString(sum[:]) != e.SHA256 {
		return nil, errors.New("isn't the text the manifest names: thrown away")
	}
	return text, nil
}

// unzstd decodes one zstd frame, with dict as its raw dictionary (id 0) when there is one,
// to the manifest's size: the decoder's memory is bounded by it, with room for a frame's
// window, which is never under a kilobyte.
func unzstd(b, dict []byte, size int64) ([]byte, error) {
	opts := []zstd.DOption{zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(uint64(max(2*size, 1<<20)))}
	if dict != nil {
		opts = append(opts, zstd.WithDecoderDictRaw(0, dict))
	}
	d, err := zstd.NewReader(nil, opts...)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	text, err := d.DecodeAll(b, nil)
	if err == nil && int64(len(text)) != size {
		err = fmt.Errorf("is %d bytes, not the manifest's %d", len(text), size)
	}
	return text, err
}

// get fetches a release file, of at most limit bytes.
func (w *worker) get(file string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(w.m.ctx, http.MethodGet, w.m.s.BaseURL+file, nil)
	if err != nil {
		return nil, err
	}
	resp, err := w.m.s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", file, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", file, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err == nil && int64(len(b)) > limit {
		err = errors.New("too large")
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %v", file, err)
	}
	return b, nil
}

// writeAside writes a file beside its place and renames it there.
func writeAside(dir, name string, b []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+name+".*")
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err2 := f.Close(); err == nil {
		err = err2
	}
	if err == nil {
		err = os.Rename(f.Name(), filepath.Join(dir, name))
	}
	if err != nil {
		os.Remove(f.Name())
	}
	return err
}
