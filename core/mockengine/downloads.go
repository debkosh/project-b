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

// downloads is what the mock keeps of downloads: those running, and the notices it showed.
// The files the app opens or shows are mocksystem's.
type downloads struct {
	mu      sync.Mutex
	next    engine.DownloadID
	running map[engine.DownloadID]*mockDownload
	notices []engine.Download // What ShowDownload was asked to show, in order.
	started []engine.Download // Each download as it started, in order.
}

func (e *Engine) dl() *downloads {
	e.downloads.mu.Lock()
	defer e.downloads.mu.Unlock()
	if e.downloads.running == nil {
		e.downloads.running = map[engine.DownloadID]*mockDownload{}
	}
	return &e.downloads
}

func (e *Engine) DownloadURL(tab engine.TabID, u, referrer string, saveAs bool) {
	e.download(tab, u, referrer, saveAs)
}

// download starts one, as a page or Save Link As does: it asks the app where it goes, a
// Save panel taking what is offered, and reports it until it is done. It comes from
// referrer, or from the tab's page when that is "", as the engine's do.
func (e *Engine) download(tab engine.TabID, u, referrer string, saveAs bool) {
	if referrer == "" && tab != 0 {
		if t := e.tabs[tab]; t != nil && t.pos >= 0 && t.pos < len(t.history) {
			referrer = t.history[t.pos]
		}
	}
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
	m := &mockDownload{tab: tab, d: engine.Download{ID: id, Path: t.Path, URL: u, Mime: "application/octet-stream", Total: downloadSize, Referrer: referrer}}
	d.mu.Lock()
	d.running[id] = m
	d.started = append(d.started, m.d)
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
		m.d.Paused, m.d.State, m.d.Reason = false, engine.DownloadProgress, ""
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
			m.d.State, m.d.Reason = engine.DownloadFailed, "connection"
			failing = append(failing, m)
		}
	}
	d.mu.Unlock()
	for _, m := range failing {
		e.listener.DownloadUpdated(m.d)
	}
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

// Started is each download as it started, in order: its address and the page it came from.
func (e *Engine) Started() []engine.Download {
	d := e.dl()
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]engine.Download(nil), d.started...)
}
