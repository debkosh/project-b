package mockengine

import (
	"net/url"
	"os"
	"path"
	"sync"
	"time"

	"projectb/engine"
)

// downloadSize is how many bytes a mock download has; it arrives in two halves, a
// loadDelay apart, and lands on disk as that many dots.
const downloadSize = 1000

// mockDownload is one download the mock is running.
type mockDownload struct {
	d    engine.Download
	tab  engine.TabID
	stop bool
	// stalled stops its bytes where they are, still running, as a connection that died
	// across a sleep without a word does.
	stalled bool
}

// downloads is what the mock keeps of downloads: those running, and the files the app
// asked to open or show.
type downloads struct {
	mu      sync.Mutex
	next    engine.DownloadID
	running map[engine.DownloadID]*mockDownload
	notices []engine.Download // What ShowDownload was asked to show, in order.
	opened  []string
	shown   []string
}

func (e *Engine) dl() *downloads {
	e.downloads.mu.Lock()
	defer e.downloads.mu.Unlock()
	if e.downloads.running == nil {
		e.downloads.running = map[engine.DownloadID]*mockDownload{}
	}
	return &e.downloads
}

func (e *Engine) DownloadURL(tab engine.TabID, u string, saveAs bool) { e.download(tab, u, saveAs) }

// download starts one, as a page or Save Link As does: it asks the app where it goes, a
// Save panel taking what is offered, and reports it until it is done.
func (e *Engine) download(tab engine.TabID, u string, saveAs bool) {
	d := e.dl()
	d.mu.Lock()
	d.next++
	id := d.next
	d.mu.Unlock()
	name := "download"
	if p, err := url.Parse(u); err == nil && path.Base(p.Path) != "/" && path.Base(p.Path) != "." {
		name = path.Base(p.Path)
	}
	t := e.listener.DownloadTarget(engine.DownloadRequest{ID: id, Tab: tab, URL: u, Name: name, Mime: "application/octet-stream", SaveAs: saveAs})
	if t.Path == "" {
		return
	}
	m := &mockDownload{tab: tab, d: engine.Download{ID: id, Path: t.Path, URL: u, Mime: "application/octet-stream", Total: downloadSize}}
	d.mu.Lock()
	d.running[id] = m
	d.mu.Unlock()
	e.listener.DownloadUpdated(m.d)
	e.step(m, downloadSize/2)
}

// step brings a download to received bytes after a while, and to done after that.
func (e *Engine) step(m *mockDownload, received int64) {
	time.AfterFunc(loadDelay, func() {
		e.Post(func() {
			if m.stop || m.stalled || m.d.Paused || m.d.State != engine.DownloadProgress {
				return
			}
			m.d.Received = received
			if received >= downloadSize {
				os.WriteFile(m.d.Path, make([]byte, downloadSize), 0o644)
				m.d.State = engine.DownloadDone
			}
			e.listener.DownloadUpdated(m.d)
			if m.d.State == engine.DownloadProgress {
				e.step(m, downloadSize)
			}
		})
	})
}

func (e *Engine) DownloadCommand(id engine.DownloadID, c engine.DownloadCommand) {
	d := e.dl()
	d.mu.Lock()
	m := d.running[id]
	d.mu.Unlock()
	failed := m != nil && m.d.State == engine.DownloadFailed
	if m == nil || m.d.State != engine.DownloadProgress && c != engine.DownloadRemove && !(failed && c == engine.DownloadResume) {
		return
	}
	switch c {
	case engine.DownloadCancel:
		m.d.State = engine.DownloadCancelled
	case engine.DownloadPause:
		m.d.Paused = true
	case engine.DownloadResume:
		// A failed one picks up where it got to, as content resumes one.
		m.d.Paused, m.d.State = false, engine.DownloadProgress
		e.step(m, downloadSize)
	case engine.DownloadRemove:
		m.stop = true
		d.mu.Lock()
		delete(d.running, id)
		d.mu.Unlock()
		return
	}
	e.listener.DownloadUpdated(m.d)
}

// stallDownloads stops every running download's bytes where they are, still running.
func (e *Engine) stallDownloads() {
	d := e.dl()
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, m := range d.running {
		if m.d.State == engine.DownloadProgress {
			m.stalled = true
		}
	}
}

// failDownloads fails every running download where it got to, as the engine does to one
// whose connection died, across a sleep, once content's own resumes gave up.
func (e *Engine) failDownloads() {
	d := e.dl()
	d.mu.Lock()
	var failing []*mockDownload
	for _, m := range d.running {
		if m.d.State == engine.DownloadProgress {
			m.d.State = engine.DownloadFailed
			failing = append(failing, m)
		}
	}
	d.mu.Unlock()
	for _, m := range failing {
		e.listener.DownloadUpdated(m.d)
	}
}

func (e *Engine) OpenFile(p string) {
	d := e.dl()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.opened = append(d.opened, p)
}

func (e *Engine) RevealFile(p string) {
	d := e.dl()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.shown = append(d.shown, p)
}

func (e *Engine) ShowDownload(id int, n engine.Download) {
	d := e.dl()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.notices = append(d.notices, n)
}

// Notices is what ShowDownload was asked to show, in order.
func (e *Engine) Notices() []engine.Download {
	d := e.dl()
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]engine.Download(nil), d.notices...)
}

// Opened and Shown are the files the app asked to open, and to show in the file manager.
func (e *Engine) Opened() []string {
	d := e.dl()
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.opened...)
}

func (e *Engine) Shown() []string {
	d := e.dl()
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.shown...)
}
