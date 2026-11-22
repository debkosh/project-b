package downloads

import (
	"path/filepath"
	"reflect"
	"testing"

	"projectb/engine"
	"projectb/mockengine"
	"projectb/mocksystem"
)

// On a phone, a finished download's path becomes its shared copy's content:// address once
// the engine has made it, as Chromium's does: Open and Share pressed before that wait for it,
// its notification is shown again with it and the file's own name, it is kept, and Delete
// hands the engine the address.
func TestAPhonesDownloadTakesItsSharedCopysAddress(t *testing.T) {
	publishes = true
	defer func() { publishes = false }()
	e, sys := mockengine.New(), mocksystem.New(func(f func()) { f() })
	profile, dir := t.TempDir(), t.TempDir()
	m := New(e, sys, profile, dir)
	path := filepath.Join(dir, "report.pdf")
	id := m.Updated(engine.Download{ID: 1, Path: path, URL: "https://a.test/report.pdf", State: engine.DownloadDone, Received: 10, Total: 10})

	m.Act(id, "open")
	m.Share(id, 1)
	if len(sys.Opened()) != 0 || e.Shared() != "" {
		t.Fatalf("opened %v, shared %q before the copy was made", sys.Opened(), e.Shared())
	}
	if m.Delete(id) {
		t.Fatal("deleted while its copy was being made")
	}

	const address = "content://media/external/downloads/7"
	m.Published(path, address)
	if got := sys.Opened(); !reflect.DeepEqual(got, []string{address}) || e.Shared() != address {
		t.Fatalf("opened %v, shared %q; want the copy's address", got, e.Shared())
	}
	notices := e.Notices()
	if last := notices[len(notices)-1]; last.Path != address || last.Name != "report.pdf" || last.State != engine.DownloadDone {
		t.Fatalf("the last notification = %+v", last)
	}
	m.Act(id, "open")
	if got := sys.Opened(); len(got) != 2 || got[1] != address {
		t.Fatalf("a later Open opened %v", got)
	}

	again := New(e, sys, profile, dir)
	again.Load()
	if l := again.List(); len(l) != 1 || l[0].Path != address || l[0].Name != "report.pdf" {
		t.Fatalf("kept %+v", l)
	}
	if !m.Delete(id) || !reflect.DeepEqual(e.Deleted(), []string{address}) {
		t.Fatalf("Delete gave the engine %v", e.Deleted())
	}
}

// A copy that failed leaves the download's path as it was, and what waited is done with it.
func TestAFailedSharedCopyKeepsThePath(t *testing.T) {
	publishes = true
	defer func() { publishes = false }()
	e, sys := mockengine.New(), mocksystem.New(func(f func()) { f() })
	dir := t.TempDir()
	m := New(e, sys, t.TempDir(), dir)
	path := filepath.Join(dir, "a.bin")
	id := m.Updated(engine.Download{ID: 1, Path: path, State: engine.DownloadDone})
	m.Act(id, "open")
	m.Published(path, "")
	if got := sys.Opened(); !reflect.DeepEqual(got, []string{path}) || m.List()[0].Path != path {
		t.Fatalf("opened %v, path %q", got, m.List()[0].Path)
	}
}

// Elsewhere nothing is copied: a finished download opens at once.
func TestADesktopDownloadOpensAtOnce(t *testing.T) {
	e, sys := mockengine.New(), mocksystem.New(func(f func()) { f() })
	dir := t.TempDir()
	m := New(e, sys, t.TempDir(), dir)
	path := filepath.Join(dir, "a.bin")
	m.Act(m.Updated(engine.Download{ID: 1, Path: path, State: engine.DownloadDone}), "open")
	if got := sys.Opened(); !reflect.DeepEqual(got, []string{path}) {
		t.Fatalf("opened %v", got)
	}
}

// The same name again, once the first's file has gone to the shared Downloads and its path
// is the copy's address, gets a name of its own, so the two rows are told apart and Delete
// takes the one asked: the shared Downloads names its copy the same way.
func TestTheSameNameAfterItsCopyWasPublishedGetsItsOwnName(t *testing.T) {
	publishes = true
	defer func() { publishes = false }()
	e, sys := mockengine.New(), mocksystem.New(func(f func()) { f() })
	dir := t.TempDir()
	m := New(e, sys, t.TempDir(), dir)
	first := m.Target(engine.DownloadRequest{Name: "report.zip"}).Path
	m.Updated(engine.Download{ID: 1, Path: first, State: engine.DownloadDone})
	m.Published(first, "content://media/external/downloads/7")
	if again := m.Target(engine.DownloadRequest{Name: "report.zip"}).Path; again != filepath.Join(dir, "report (1).zip") {
		t.Fatalf("the same name again went to %q", again)
	}
}
