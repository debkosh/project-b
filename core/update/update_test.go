package update

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sudiptadeb/amarit/appupdate"
)

// Each of amarit's failures in the words the pages show, and whether Try again is offered.
// A check the person asked for that couldn't read the list, or found a download stopping,
// is the check's own answer.
func TestEachFailureHasItsWords(t *testing.T) {
	for _, c := range []struct {
		kind  appupdate.Kind
		err   string
		words string
		retry bool
	}{
		{appupdate.Network, "connection reset", "the download failed (connection reset); Try again", true},
		{appupdate.Stalled, "", "the download stopped arriving; Try again", true},
		{appupdate.Damaged, "sha256 x", "the download arrived damaged: it didn't match its checksum; Try again", true},
		{appupdate.Unchecked, "Too many open files", "its signature couldn't be checked (Too many open files); Try again", true},
		{appupdate.Refused, "not signed with the expected key: refused", "it isn't signed with Project B's key, so it won't be tried again", false},
		{appupdate.Refused, "its signature is broken: refused", "its signature is broken, so it won't be tried again", false},
		{appupdate.NotWritable, "", "Project B can't be replaced where it is: move it to Applications", false},
		{appupdate.InstallFailed, "rename", "couldn't put the new app in place (rename)", true},
	} {
		st := words(appupdate.State{Phase: appupdate.Failed, Version: "0.4.0", Kind: c.kind, Error: c.err})
		if st.Phase != Failed || st.Version != "0.4.0" || st.Error != c.words || st.Retry != c.retry {
			t.Errorf("%s: %+v", c.kind, st)
		}
	}
	for _, k := range []appupdate.Kind{appupdate.Offline, appupdate.Busy} {
		if st := words(appupdate.State{Phase: appupdate.Failed, Kind: k, Error: "e"}); st.Phase != Idle || st.Check != CheckFailed || st.CheckError == "" {
			t.Errorf("%s should be the check's answer: %+v", k, st)
		}
	}
	if st := words(appupdate.State{Phase: appupdate.Verifying, Version: "0.4.0", Total: 9}); st.Phase != Downloading || st.Done != 9 || st.Total != 9 {
		t.Errorf("checking the download shows it arrived: %+v", st)
	}
	if st := words(appupdate.State{Phase: appupdate.Checking}); st.Phase != Idle || st.Check != Checking {
		t.Errorf("checking: %+v", st)
	}
}

// server serves a manifest with the stable and, when set, dev releases, each one file.
type server struct {
	url     string
	mu      sync.Mutex
	stable  string
	dev     string
	content []byte
}

func serve(t *testing.T, stable, dev string) *server {
	t.Helper()
	s := &server{stable: stable, dev: dev, content: []byte("a release")}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if strings.HasSuffix(r.URL.Path, ".zip") {
			w.Write(s.content)
			return
		}
		if r.URL.Path == "/notes-"+s.stable+".md" {
			fmt.Fprintf(w, "# %s\n\n- Saved passwords sync\n", s.stable)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/notes-") {
			http.NotFound(w, r)
			return
		}
		sum := sha256.Sum256(s.content)
		entry := func(v string) string {
			return fmt.Sprintf(`"%s":{"assets":{"%s/%s":{"url":"%s/%s.zip","sha256":"%s","size":%d}}}`,
				v, runtime.GOOS, runtime.GOARCH, srv.URL, v, hex.EncodeToString(sum[:]), len(s.content))
		}
		channels, releases := fmt.Sprintf(`"stable":"%s"`, s.stable), entry(s.stable)
		if s.dev != "" {
			channels += fmt.Sprintf(`,"dev":"%s"`, s.dev)
			releases += "," + entry(s.dev)
		}
		fmt.Fprintf(w, `{"spec":1,"project":"project-b","published":"2026-11-20T00:00:00Z","channels":{%s},"releases":{%s}}`, channels, releases)
	}))
	t.Cleanup(srv.Close)
	s.url = srv.URL + "/releases.json"
	return s
}

// opened records what the installer handed to the system.
type opened struct {
	mu    sync.Mutex
	files []string
}

func (o *opened) list() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.files...)
}

func manager(t *testing.T, dir, running, releases string, o *opened) *Manager {
	t.Helper()
	cfg := Config{Project: "project-b", Version: running, Releases: releases, AutoUpdate: true, StateDir: dir,
		Installer: func(func(func()), func()) appupdate.Installer {
			return appupdate.File{Dir: dir, Open: func(f string) error {
				o.mu.Lock()
				defer o.mu.Unlock()
				o.files = append(o.files, f)
				return nil
			}}
		}}
	return New(cfg, func(f func()) { f() }, func() {})
}

func eventually(t *testing.T, m *Manager, ok func(State) bool) State {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if s := m.State(); ok(s) {
			return s
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("never: %+v", m.State())
	return State{}
}

// A check finds the newer release, downloads and stages it, the page sees it ready with its
// notes, and Relaunch hands it to the installer.
func TestANewerReleaseIsReadyAndInstalled(t *testing.T) {
	s := serve(t, "0.2.0", "")
	var o opened
	m := manager(t, t.TempDir(), "0.1.0", s.url, &o)
	var heard []string
	m.OnChanged(func(st State) { heard = append(heard, st.Phase+"/"+st.Check) })
	m.CheckNow()
	eventually(t, m, func(st State) bool { return st.Phase == Ready && st.Version == "0.2.0" })
	// Its notes, beside the manifest, come with it: About shows them as Coming in 0.2.0.
	eventually(t, m, func(st State) bool { return strings.Contains(st.Notes, "- Saved passwords sync") })
	m.Relaunch()
	deadline := time.Now().Add(5 * time.Second)
	for len(o.list()) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if files := o.list(); len(files) != 1 {
		t.Fatalf("installed %v; the page heard %v", files, heard)
	}
}

// Up to date: the check says so.
func TestACheckOnTheSameVersionSaysUpToDate(t *testing.T) {
	s := serve(t, "0.2.0", "")
	var o opened
	m := manager(t, t.TempDir(), "0.2.0", s.url, &o)
	m.CheckNow()
	eventually(t, m, func(st State) bool { return st.Check == UpToDate && st.Phase == Idle })
}

// The channel and automatic updates are every copy's: chosen in one, read in the other, and
// heard. A copy on Stable 0.2.1 choosing Dev isn't offered the Dev build before it.
func TestTheSettingsAreSharedAndDevNeverGoesBack(t *testing.T) {
	s := serve(t, "0.2.1", "0.2.1-dev.202611201415")
	dir := t.TempDir()
	var o opened
	one := manager(t, dir, "0.2.1", s.url, &o)
	if one.Channel() != Stable || !one.Auto() {
		t.Fatalf("a new folder follows %q, automatic %v", one.Channel(), one.Auto())
	}
	var channels []string
	var autos []bool
	one.OnChannelChanged(func(ch string) { channels = append(channels, ch) })
	one.OnAutoChanged(func(on bool) { autos = append(autos, on) })
	one.SetChannel("nightly")
	one.SetChannel(Dev)
	eventually(t, one, func(st State) bool { return one.Channel() == Dev && st.Check == UpToDate })
	one.SetAuto(false)
	eventually(t, one, func(State) bool { return !one.Auto() })
	if len(channels) != 1 || channels[0] != Dev || len(autos) != 1 || autos[0] {
		t.Fatalf("heard channels %v, automatic %v", channels, autos)
	}
	other := manager(t, dir, "0.2.1", s.url, &o)
	if other.Channel() != Dev || other.Auto() {
		t.Fatalf("the other copy follows %q, automatic %v", other.Channel(), other.Auto())
	}
	if files := o.list(); len(files) != 0 {
		t.Fatalf("an older Dev build was taken: %v", files)
	}
	if _, err := os.Stat(filepath.Join(dir, "amarit", "settings.json")); err != nil {
		t.Fatalf("the settings aren't in amarit's file: %v", err)
	}
}

// A development build has no version and no updater: it says so when asked, and its
// settings read as their defaults.
func TestADevelopmentBuildDoesntUpdate(t *testing.T) {
	m := New(Config{StateDir: t.TempDir(), AutoUpdate: true}, func(f func()) { f() }, func() {})
	m.Start()
	defer m.Stop()
	m.CheckNow()
	if st := m.State(); st.Check != CheckFailed || !strings.Contains(st.CheckError, "development build") {
		t.Fatalf("state %+v", st)
	}
	if m.Channel() != Stable || m.Settle() {
		t.Fatal("a development build has nothing to settle")
	}
}
