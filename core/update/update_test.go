package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sudiptadeb/amarit"
)

// fakeApp writes a bundle the way tools/package.sh lays one out: the plist and an
// executable, here a script that records that it ran.
func fakeApp(t *testing.T, dir, version string) string {
	t.Helper()
	app := filepath.Join(dir, "Project B.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key><string>project_b</string>
  <key>CFBundleShortVersionString</key><string>%s</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
`, version)
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\necho %s > \"$(dirname \"$0\")/ran\"\n", version)
	if err := os.WriteFile(filepath.Join(app, "Contents", "MacOS", "project_b"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return app
}

// archive zips a bundle as tools/release.sh does.
func archive(t *testing.T, app string) []byte {
	t.Helper()
	zip := filepath.Join(t.TempDir(), "app.zip")
	if out, err := exec.Command("/usr/bin/ditto", "-c", "-k", "--keepParent", app, zip).CombinedOutput(); err != nil {
		t.Fatalf("ditto: %v %s", err, out)
	}
	b, err := os.ReadFile(zip)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// releaseServer serves a manifest pointing stable at Version, and the asset. The manifest's
// digest is of Good; Served is what the client gets. Both can change between checks.
type releaseServer struct {
	mu      sync.Mutex
	Version string
	Good    []byte
	Served  []byte
	// Dev, when set, is the Dev channel's release, whose bundle DevGood is served as it is.
	Dev       string
	DevGood   []byte
	downloads atomic.Int32
	manifests atomic.Int32
	url       string
	serve     func(w http.ResponseWriter) // serves the asset instead of Served, when set
}

func release(t *testing.T, version string, good, served []byte) *releaseServer {
	t.Helper()
	rs := &releaseServer{Version: version, Good: good, Served: served}
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/releases.json", func(w http.ResponseWriter, r *http.Request) {
		rs.manifests.Add(1)
		rs.mu.Lock()
		defer rs.mu.Unlock()
		entry := func(version, path string, good []byte) string {
			sum := sha256.Sum256(good)
			return fmt.Sprintf(`"%s":{"assets":{"%s/%s":{"url":"%s/%s","sha256":"%s","size":%d}}}`,
				version, runtime.GOOS, runtime.GOARCH, srv.URL, path, hex.EncodeToString(sum[:]), len(good))
		}
		channels, releases := fmt.Sprintf(`"stable":"%s"`, rs.Version), entry(rs.Version, "asset.zip", rs.Good)
		if rs.Dev != "" {
			channels += fmt.Sprintf(`,"dev":"%s"`, rs.Dev)
			releases += "," + entry(rs.Dev, "dev.zip", rs.DevGood)
		}
		fmt.Fprintf(w, `{"spec":1,"project":"project-b","published":"2026-11-09T00:00:00Z","channels":{%s},"releases":{%s}}`, channels, releases)
	})
	mux.HandleFunc("/dev.zip", func(w http.ResponseWriter, r *http.Request) {
		rs.downloads.Add(1)
		rs.mu.Lock()
		defer rs.mu.Unlock()
		w.Write(rs.DevGood)
	})
	mux.HandleFunc("/asset.zip", func(w http.ResponseWriter, r *http.Request) {
		rs.downloads.Add(1)
		if rs.serve != nil {
			rs.serve(w)
			return
		}
		rs.mu.Lock()
		defer rs.mu.Unlock()
		w.Write(rs.Served)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	rs.url = srv.URL + "/releases.json"
	return rs
}

func (rs *releaseServer) set(version string, good, served []byte) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.Version, rs.Good, rs.Served = version, good, served
}

type rig struct {
	m      *Manager
	mu     sync.Mutex // the download's progress is told from its own goroutine
	states []State
	quit   bool
	app    string
}

func start(t *testing.T, running, releases string, verify func(string) error) *rig {
	t.Helper()
	installed := t.TempDir()
	r := &rig{app: fakeApp(t, installed, running)}
	cfg := Config{Project: "project-b", Version: running, Releases: releases, StateDir: filepath.Join(t.TempDir(), "updates"), App: r.app, Verify: verify}
	r.m = New(cfg, func(f func()) { f() }, func() { r.quit = true })
	r.m.OnChanged(func(s State) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.states = append(r.states, s)
	})
	if r.m.client == nil {
		t.Fatal("no client")
	}
	return r
}

func (r *rig) check() { r.m.check(context.Background(), false) }

// phases lists the phases the shell was shown, once each in a row: a download's progress
// is more news in the same phase.
func (r *rig) phases() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var p []string
	for _, s := range r.states {
		if len(p) == 0 || p[len(p)-1] != s.Phase {
			p = append(p, s.Phase)
		}
	}
	return strings.Join(p, " ")
}

func TestDevelopmentBuildNeverChecks(t *testing.T) {
	m := New(Config{}, func(f func()) { f() }, func() {})
	m.Start()
	defer m.Stop()
	if m.client != nil || m.State().Phase != Idle {
		t.Fatalf("a build with no version must stay idle: %+v", m.State())
	}
}

func TestSameVersionShowsNothing(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	r := start(t, "0.2.0", rs.url, nil)
	r.check()
	if len(r.states) != 0 || rs.downloads.Load() != 0 {
		t.Fatalf("states %v downloads %d", r.states, rs.downloads.Load())
	}
}

func TestNewerReleaseIsStagedAndRelaunchSwapsTheBundle(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	verified := ""
	r := start(t, "0.1.0", rs.url, func(app string) error { verified = app; return nil })
	r.check()
	if r.phases() != "downloading ready" || r.states[1].Version != "0.2.0" {
		t.Fatalf("phases %q states %+v", r.phases(), r.states)
	}
	if !strings.HasPrefix(verified, r.m.downloadDir()) {
		t.Fatalf("verify saw %q", verified)
	}
	if v, _ := Version(r.app); v != "0.1.0" {
		t.Fatalf("installed app changed before relaunch: %s", v)
	}
	r.check()
	if rs.downloads.Load() != 1 {
		t.Fatalf("a release already ready was downloaded again: %d", rs.downloads.Load())
	}

	r.m.Relaunch()
	if !r.quit {
		t.Fatal("the app was not asked to quit")
	}
	if v, _ := Version(r.app); v != "0.2.0" {
		t.Fatalf("installed app is %s after relaunch", v)
	}
	if v, _ := Version(filepath.Join(r.m.cfg.StateDir, "previous", "Project B.app")); v != "0.1.0" {
		t.Fatalf("previous release is %s", v)
	}
	if _, err := os.Stat(r.m.stageDir()); !os.IsNotExist(err) {
		if entries, _ := os.ReadDir(r.m.stageDir()); len(entries) != 0 {
			t.Fatalf("stage still holds %v", entries)
		}
	}
}

func TestCorruptedDownloadIsRefusedAndTriedAgain(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	tampered := append([]byte(nil), good...)
	tampered[len(tampered)/2] ^= 0xff
	rs := release(t, "0.2.0", good, tampered)
	r := start(t, "0.1.0", rs.url, nil)
	r.check()
	if r.phases() != "downloading failed" || !strings.Contains(r.states[1].Error, "sha256") {
		t.Fatalf("phases %q states %+v", r.phases(), r.states)
	}
	if v, _ := Version(r.app); v != "0.1.0" {
		t.Fatalf("installed app changed: %s", v)
	}
	if _, err := os.Stat(r.m.stageDir()); err == nil {
		t.Fatal("a refused download left a stage")
	}
	r.m.Relaunch()
	if r.quit {
		t.Fatal("relaunch with nothing ready must do nothing")
	}
	r.check()
	if rs.downloads.Load() != 2 {
		t.Fatalf("a failed download should be tried again next check: %d", rs.downloads.Load())
	}

	// The publisher pulls the release: there is nothing to say any more.
	rs.set("0.1.0", good, good)
	r.check()
	if r.m.State().Phase != Idle {
		t.Fatalf("a withdrawn release should clear the failure: %+v", r.m.State())
	}
}

func TestBundleOfAnotherVersionIsRefusedOnce(t *testing.T) {
	wrong := archive(t, fakeApp(t, t.TempDir(), "0.1.5"))
	rs := release(t, "0.2.0", wrong, wrong)
	r := start(t, "0.1.0", rs.url, nil)
	r.check()
	if r.phases() != "downloading failed" || !strings.Contains(r.states[1].Error, "says it is 0.1.5") {
		t.Fatalf("phases %q states %+v", r.phases(), r.states)
	}
	r.check()
	if rs.downloads.Load() != 1 || r.m.State().Phase != Failed {
		t.Fatalf("a refused bundle must not be downloaded again, and stays on show: %d %+v", rs.downloads.Load(), r.m.State())
	}
	// A newer release is tried.
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.1"))
	rs.set("0.2.1", good, good)
	r.check()
	if s := r.m.State(); s.Phase != Ready || s.Version != "0.2.1" {
		t.Fatalf("the next release should be taken: %+v", s)
	}
}

func TestBundleFailingVerifyIsRefused(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	r := start(t, "0.1.0", rs.url, func(string) error { return fmt.Errorf("%w: it isn't signed with Project B's key", errRefused) })
	r.check()
	if r.phases() != "downloading failed" || !strings.Contains(r.states[1].Error, "isn't signed with Project B's key") {
		t.Fatalf("phases %q states %+v", r.phases(), r.states)
	}
	if _, err := os.Stat(r.m.stageDir()); err == nil {
		t.Fatal("a refused bundle left a stage")
	}
	r.check()
	if n := rs.downloads.Load(); n != 1 {
		t.Fatalf("a refused release was downloaded %d times", n)
	}
}

// A check that couldn't be made, as codesign's was for want of open files, is no answer
// about the release: it isn't rejected, and the next check, automatic or asked for, tries
// it again.
func TestACheckThatCouldNotBeMadeIsTriedAgain(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	var broken atomic.Bool
	broken.Store(true)
	r := start(t, "0.1.0", rs.url, func(string) error {
		if broken.Load() {
			return errors.New("its signature couldn't be checked: Too many open files")
		}
		return nil
	})
	r.check()
	if s := r.m.State(); s.Phase != Failed || !strings.Contains(s.Error, "couldn't be checked: Too many open files") {
		t.Fatalf("state %+v", s)
	}
	r.m.CheckNow()
	eventually(t, r.m, func(s State) bool { return s.Phase == Failed && rs.downloads.Load() == 2 })
	broken.Store(false)
	r.check()
	if s := r.m.State(); s.Phase != Ready || s.Version != "0.2.0" {
		t.Fatalf("the next check should take it: %+v", s)
	}
}

func TestUnwritableInstallLocationIsReportedWithoutDownloading(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	r := start(t, "0.1.0", rs.url, nil)
	dir := filepath.Dir(r.app)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	r.check()
	if r.phases() != "failed" || !strings.Contains(r.states[0].Error, "move Project B to Applications") || rs.downloads.Load() != 0 {
		t.Fatalf("phases %q states %+v downloads %d", r.phases(), r.states, rs.downloads.Load())
	}
	// Moved somewhere writable: the next check goes ahead.
	os.Chmod(dir, 0o755)
	r.check()
	if r.m.State().Phase != Ready {
		t.Fatalf("after the folder became writable: %+v", r.m.State())
	}
}

func TestBundleOfFindsTheApp(t *testing.T) {
	if got := BundleOf("/Applications/Project B.app/Contents/MacOS/project_b"); got != "/Applications/Project B.app" {
		t.Fatal(got)
	}
	if got := BundleOf("/Users/me/project-b/chromium/src/out/pb/project_b"); got != "" {
		t.Fatal(got)
	}
}

// eventually waits for the manager's state to match, as a check asked for runs on its own
// goroutine.
func eventually(t *testing.T, m *Manager, match func(State) bool) State {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if s := m.State(); match(s) {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("state stayed %+v", m.State())
	return State{}
}

func TestCheckNowSaysUpToDate(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	r := start(t, "0.2.0", rs.url, nil)
	r.m.CheckNow()
	eventually(t, r.m, func(s State) bool { return s.Check == UpToDate && s.Phase == Idle })
	if rs.manifests.Load() != 1 || rs.downloads.Load() != 0 {
		t.Fatalf("manifests %d downloads %d", rs.manifests.Load(), rs.downloads.Load())
	}
}

func TestCheckNowSaysWhyTheManifestCouldNotBeRead(t *testing.T) {
	r := start(t, "0.2.0", "http://127.0.0.1:1/releases.json", nil)
	r.m.CheckNow()
	s := eventually(t, r.m, func(s State) bool { return s.Check == CheckFailed })
	if s.CheckError == "" || s.Phase != Idle {
		t.Fatalf("state %+v", s)
	}
}

func TestCheckNowTakesANewerReleaseTheWayAnAutomaticCheckDoes(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	r := start(t, "0.1.0", rs.url, nil)
	r.m.CheckNow()
	s := eventually(t, r.m, func(s State) bool { return s.Phase == Ready })
	if s.Version != "0.2.0" || s.Check != "" {
		t.Fatalf("state %+v", s)
	}
}

func TestDevelopmentBuildSaysItDoesNotUpdate(t *testing.T) {
	m := New(Config{}, func(f func()) { f() }, func() {})
	m.CheckNow()
	if s := m.State(); s.Check != CheckFailed || !strings.Contains(s.CheckError, "development build") {
		t.Fatalf("state %+v", s)
	}
}

// Off, the timer doesn't check; a check asked for still does. On again, the timer starts
// as it does at launch.
func TestAutomaticChecksTurnOffAndOnAtOnce(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	installed := t.TempDir()
	cfg := Config{Project: "project-b", Version: "0.2.0", Releases: rs.url, StateDir: filepath.Join(t.TempDir(), "updates"), App: fakeApp(t, installed, "0.2.0")}
	m := New(cfg, func(f func()) { f() }, func() {})
	m.SetAuto(false)
	m.Start()
	defer m.Stop()
	time.Sleep(firstCheck + time.Second)
	if n := rs.manifests.Load(); n != 0 {
		t.Fatalf("off, the timer still checked %d times", n)
	}
	m.CheckNow()
	eventually(t, m, func(s State) bool { return s.Check == UpToDate })
	if n := rs.manifests.Load(); n != 1 {
		t.Fatalf("a check asked for made %d requests", n)
	}
	m.SetAuto(true)
	deadline := time.Now().Add(firstCheck + 3*time.Second)
	for rs.manifests.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if n := rs.manifests.Load(); n != 2 {
		t.Fatalf("on again, the timer made %d requests in all", n)
	}
}

func TestInstallTakesTheDownloadAsItIs(t *testing.T) {
	apk := []byte("an APK, as far as the updater cares")
	rs := release(t, "0.2.0", apk, apk)
	var installed []string
	cfg := Config{Project: "project-b", Version: "0.1.0", Releases: rs.url, StateDir: filepath.Join(t.TempDir(), "updates"),
		Install: func(file string) { installed = append(installed, file) }}
	quit := false
	m := New(cfg, func(f func()) { f() }, func() { quit = true })
	m.check(context.Background(), false)
	if s := m.State(); s.Phase != Ready || s.Version != "0.2.0" {
		t.Fatalf("state %+v", s)
	}
	m.Relaunch()
	m.Relaunch()
	if len(installed) != 2 || installed[0] != installed[1] || !strings.HasPrefix(installed[0], m.stageDir()) {
		t.Fatalf("installed %v", installed)
	}
	if b, err := os.ReadFile(installed[0]); err != nil || string(b) != string(apk) {
		t.Fatalf("staged %q %v", b, err)
	}
	if quit {
		t.Fatal("the app quit; the installer replaces it")
	}
}

func TestDownloadShowsHowMuchHasArrived(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	half := make(chan struct{})
	rest := make(chan struct{})
	rs.serve = func(w http.ResponseWriter) {
		w.Write(good[:len(good)/2])
		w.(http.Flusher).Flush()
		close(half)
		<-rest
		w.Write(good[len(good)/2:])
	}
	r := start(t, "0.1.0", rs.url, nil)
	done := make(chan struct{})
	go func() { r.check(); close(done) }()
	<-half
	s := eventually(t, r.m, func(s State) bool { return s.Done > 0 })
	close(rest)
	<-done
	if s.Phase != Downloading || s.Done != int64(len(good)/2) || s.Total != int64(len(good)) {
		t.Fatalf("mid-download state %+v, want %d of %d", s, len(good)/2, len(good))
	}
	if r.phases() != "downloading ready" {
		t.Fatalf("phases %q", r.phases())
	}
}

// sharing is a manager on a folder another copy may share, installed as running.
func sharing(t *testing.T, folder, running, releases string, post func(func()), quit func()) *Manager {
	t.Helper()
	cfg := Config{Project: "project-b", Version: running, Releases: releases, StateDir: folder, App: fakeApp(t, t.TempDir(), running)}
	return New(cfg, post, quit)
}

// Two copies on one folder: while one checks and downloads, holding the lock, the other's
// round does nothing; once the release is staged, the other reads it from the folder and
// offers it too, without downloading it again.
func TestOneCopyAtATimeChecksAndEachReadsTheStagedRelease(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	half := make(chan struct{})
	rest := make(chan struct{})
	rs.serve = func(w http.ResponseWriter) {
		w.Write(good[:len(good)/2])
		w.(http.Flusher).Flush()
		close(half)
		<-rest
		w.Write(good[len(good)/2:])
	}
	folder := filepath.Join(t.TempDir(), "update")
	inline := func(f func()) { f() }
	first := sharing(t, folder, "0.1.0", rs.url, inline, func() {})
	second := sharing(t, folder, "0.1.0", rs.url, inline, func() {})
	done := make(chan struct{})
	go func() { first.round(); close(done) }()
	<-half
	second.round()
	if n := rs.manifests.Load(); n != 1 || second.State().Phase != Idle {
		t.Fatalf("while the first copy downloads, the second read the manifest %d times in all and shows %+v", n, second.State())
	}
	close(rest)
	<-done
	if s := first.State(); s.Phase != Ready || s.Version != "0.2.0" {
		t.Fatalf("the first copy shows %+v", s)
	}
	second.round()
	if s := second.State(); s.Phase != Ready || s.Version != "0.2.0" || rs.downloads.Load() != 1 {
		t.Fatalf("the second copy shows %+v after %d downloads", s, rs.downloads.Load())
	}
	// With automatic checks off, a round still reads the staged release.
	third := sharing(t, folder, "0.1.0", rs.url, inline, func() {})
	third.SetAuto(false)
	third.round()
	if s := third.State(); s.Phase != Ready || rs.manifests.Load() != 2 {
		t.Fatalf("with automatic checks off the copy shows %+v, after %d manifests", s, rs.manifests.Load())
	}
}

// The automatic-updates setting is the folder's: every copy sharing it reads the same, a
// change in one holds in the others, and it is on until someone turns it off.
func TestTheAutomaticUpdatesSettingIsShared(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "update")
	inline := func(f func()) { f() }
	one := sharing(t, folder, "0.1.0", "http://127.0.0.1:1/releases.json", inline, func() {})
	other := sharing(t, folder, "0.1.0", "http://127.0.0.1:1/releases.json", inline, func() {})
	if !one.Auto() || !other.Auto() {
		t.Fatal("automatic updates start off")
	}
	var heard []bool
	one.OnAutoChanged(func(on bool) { heard = append(heard, on) })
	one.SetAuto(false)
	one.SetAuto(false)
	if other.Auto() || len(heard) != 1 || heard[0] {
		t.Fatalf("the other copy reads %v; this one heard %v", other.Auto(), heard)
	}
	if later := sharing(t, folder, "0.1.0", "http://127.0.0.1:1/releases.json", inline, func() {}); later.Auto() {
		t.Fatal("a copy started later reads automatic updates on")
	}
	other.SetAuto(true)
	if !one.Auto() {
		t.Fatal("turned on in the other copy, this one reads it off")
	}
}

// A copy follows Stable until Dev is chosen, which checks at once and takes the Dev build;
// every copy sharing the folder follows the same channel. Back on Stable, the Dev build
// stays: Stable has nothing newer than it.
func TestDevIsChosenForEveryCopyAndTakenAtOnce(t *testing.T) {
	stable := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	dev := archive(t, fakeApp(t, t.TempDir(), "0.2.1-dev.202611201415"))
	rs := release(t, "0.2.0", stable, stable)
	rs.Dev, rs.DevGood = "0.2.1-dev.202611201415", dev
	r := start(t, "0.2.0", rs.url, nil)
	if r.m.Channel() != Stable {
		t.Fatalf("a new folder follows %q", r.m.Channel())
	}
	r.check()
	if rs.downloads.Load() != 0 {
		t.Fatalf("on Stable, %d downloads", rs.downloads.Load())
	}
	var heard []string
	r.m.OnChannelChanged(func(ch string) { heard = append(heard, ch) })
	r.m.SetChannel("nightly")
	r.m.SetChannel(Dev)
	r.m.SetChannel(Dev)
	if len(heard) != 1 || heard[0] != Dev {
		t.Fatalf("heard %v", heard)
	}
	s := eventually(t, r.m, func(s State) bool { return s.Phase == Ready })
	if s.Version != "0.2.1-dev.202611201415" {
		t.Fatalf("state %+v", s)
	}
	other := sharing(t, r.m.cfg.StateDir, "0.2.0", rs.url, func(f func()) { f() }, func() {})
	if other.Channel() != Dev {
		t.Fatalf("the other copy follows %q", other.Channel())
	}

	on := start(t, "0.2.1-dev.202611201415", rs.url, nil)
	on.m.SetChannel(Dev)
	on.m.SetChannel(Stable)
	eventually(t, on.m, func(s State) bool { return s.Check == UpToDate })
}

// A copy on a Stable release that chooses Dev isn't offered the Dev builds that came before
// it: 0.2.1-dev.<minute> is older than 0.2.1, whatever the phone's versionCode says, as the
// updater orders by version alone. The minute's twelve digits order as a number.
func TestDevNeverOffersABuildOlderThanTheStableRunning(t *testing.T) {
	for _, order := range [][2]string{
		{"0.2.0", "0.2.1-dev.202611201415"},
		{"0.2.1-dev.202611201415", "0.2.1-dev.202611201416"},
		{"0.2.1-dev.202611201416", "0.2.1"},
		{"0.2.1-dev.999999999999", "0.2.1"},
	} {
		if c, err := amarit.CompareVersions(order[0], order[1]); err != nil || c >= 0 {
			t.Fatalf("%s should be before %s: %d %v", order[0], order[1], c, err)
		}
	}
	stable := archive(t, fakeApp(t, t.TempDir(), "0.2.1"))
	dev := archive(t, fakeApp(t, t.TempDir(), "0.2.1-dev.202611201415"))
	rs := release(t, "0.2.1", stable, stable)
	rs.Dev, rs.DevGood = "0.2.1-dev.202611201415", dev
	r := start(t, "0.2.1", rs.url, nil)
	r.m.SetChannel(Dev)
	eventually(t, r.m, func(s State) bool { return s.Check == UpToDate })
	if n := rs.downloads.Load(); n != 0 || r.m.State().Phase != Idle {
		t.Fatalf("on 0.2.1, Dev's older build was taken: %d downloads, %+v", n, r.m.State())
	}
}

// fakeCopies records what Relaunch asks of the other copies, and what the installed app
// was at each step.
type fakeCopies struct {
	app     string
	steps   []string
	started chan struct{}
}

func (c *fakeCopies) QuitOthers() []string {
	v, _ := Version(c.app)
	c.steps = append(c.steps, "quit the others on "+v)
	return []string{"default", "work"}
}

func (c *fakeCopies) StartAfter(executable string, profiles []string) {
	v, _ := Version(c.app)
	c.steps = append(c.steps, fmt.Sprintf("start %s on %s with %s", strings.Join(profiles, " "), v, filepath.Base(executable)))
	close(c.started)
}

// Relaunch with other copies: they quit first, the swap waits for them, and each profile
// starts again on the new release once this copy has ended.
func TestRelaunchQuitsTheOtherCopiesThenSwapsAndStartsEachProfile(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	app := fakeApp(t, t.TempDir(), "0.1.0")
	copies := &fakeCopies{app: app, started: make(chan struct{})}
	quit := make(chan struct{})
	cfg := Config{Project: "project-b", Version: "0.1.0", Releases: rs.url, StateDir: filepath.Join(t.TempDir(), "update"), App: app, Copies: copies}
	m := New(cfg, func(f func()) { f() }, func() {
		copies.steps = append(copies.steps, "quit")
		close(quit)
	})
	m.round()
	if m.State().Phase != Ready {
		t.Fatalf("state %+v", m.State())
	}
	m.Relaunch()
	<-quit
	<-copies.started
	want := "quit the others on 0.1.0, start default work on 0.2.0 with project_b, quit"
	if got := strings.Join(copies.steps, ", "); got != want {
		t.Fatalf("the steps were %q, want %q", got, want)
	}
}

func TestCheckNowTriesARefusedReleaseAgain(t *testing.T) {
	wrong := archive(t, fakeApp(t, t.TempDir(), "0.1.5"))
	rs := release(t, "0.2.0", wrong, wrong)
	r := start(t, "0.1.0", rs.url, nil)
	r.check()
	if rs.downloads.Load() != 1 || r.m.State().Phase != Failed {
		t.Fatalf("the automatic check should refuse it: %d %+v", rs.downloads.Load(), r.m.State())
	}
	// The release is fixed in place; an automatic check still holds the refusal, a person's
	// check takes it.
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs.set("0.2.0", good, good)
	r.check()
	if rs.downloads.Load() != 1 {
		t.Fatalf("an automatic check must not download a refused release again: %d", rs.downloads.Load())
	}
	r.m.CheckNow()
	s := eventually(t, r.m, func(s State) bool { return s.Phase == Ready })
	if s.Version != "0.2.0" || rs.downloads.Load() != 2 {
		t.Fatalf("state %+v, downloads %d", s, rs.downloads.Load())
	}
}

// A download whose connection dies without a word, as across the Mac's sleep, is given up
// once nothing arrives for a while, shown as failed, and the next check gets it.
func TestAStalledDownloadFailsAndTheNextCheckGetsIt(t *testing.T) {
	defer func(d time.Duration) { stalledAfter = d }(stalledAfter)
	stalledAfter = 500 * time.Millisecond
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	hold := make(chan struct{})
	defer close(hold)
	rs.serve = func(w http.ResponseWriter) {
		w.Header().Set("Content-Length", fmt.Sprint(len(good)))
		w.Write(good[:len(good)/2])
		w.(http.Flusher).Flush()
		<-hold
	}
	r := start(t, "0.1.0", rs.url, nil)
	r.check()
	if s := r.m.State(); s.Phase != Failed || s.Error != errStalled.Error() {
		t.Fatalf("a stalled download should fail: %+v", s)
	}
	rs.serve = nil
	r.check()
	if s := r.m.State(); s.Phase != Ready {
		t.Fatalf("the next check should stage it: %+v", s)
	}
}

// stuck serves half the release and then nothing, as a connection that died across the
// Mac's sleep does, with the stall cutoff too far off to help, until the test ends; once
// healed, it serves the whole release.
func stuck(t *testing.T) (*releaseServer, *atomic.Bool) {
	t.Helper()
	old := stalledAfter
	stalledAfter = time.Hour
	t.Cleanup(func() { stalledAfter = old })
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	healed := &atomic.Bool{}
	rs.serve = func(w http.ResponseWriter) {
		if healed.Load() {
			w.Write(good)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(good)))
		w.Write(good[:len(good)/2])
		w.(http.Flusher).Flush()
		<-hold
	}
	return rs, healed
}

// until waits for the manager's state to match.
func (r *rig) until(t *testing.T, what string, match func(State) bool) State {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		s := r.m.State()
		if match(s) {
			return s
		}
		if time.Now().After(deadline) {
			t.Fatalf("never %s: %+v", what, s)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Cancel stops a download stuck mid-way at once and shows nothing; automatic checks leave
// that release alone, and Check for updates fetches it.
func TestCancelStopsAStuckDownloadAtOnce(t *testing.T) {
	rs, healed := stuck(t)
	r := start(t, "0.1.0", rs.url, nil)
	done := make(chan struct{})
	go func() { r.check(); close(done) }()
	r.until(t, "half downloaded", func(s State) bool { return s.Phase == Downloading && s.Done > 0 })
	r.m.Cancel()
	if s := r.m.State(); s.Phase != Idle {
		t.Fatalf("Cancel left %+v", s)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the stopped download never ended")
	}
	if s := r.m.State(); s.Phase != Idle {
		t.Fatalf("the stopped download's end showed %+v", s)
	}
	n := rs.downloads.Load()
	healed.Store(true)
	r.check()
	if s := r.m.State(); s.Phase != Idle || rs.downloads.Load() != n {
		t.Fatalf("an automatic check after Cancel: %+v, downloads %d then %d", s, n, rs.downloads.Load())
	}
	r.m.CheckNow()
	r.until(t, "ready after Check for updates", func(s State) bool { return s.Phase == Ready })
}

// Retry stops a download stuck mid-way and fetches the release again at once.
func TestRetryStartsAStuckDownloadAgain(t *testing.T) {
	rs, healed := stuck(t)
	r := start(t, "0.1.0", rs.url, nil)
	go r.check()
	r.until(t, "half downloaded", func(s State) bool { return s.Phase == Downloading && s.Done > 0 })
	healed.Store(true)
	r.m.Retry()
	r.until(t, "ready after Retry", func(s State) bool { return s.Phase == Ready })
	if n := rs.downloads.Load(); n != 2 {
		t.Fatalf("downloads %d", n)
	}
}

// A check the person asks for waits only so long for the lock another copy, or a download
// still stopping, holds, and then says so.
func TestCheckNowWaitsForTheLockOnlySoLong(t *testing.T) {
	defer func(d time.Duration) { lockWait = d }(lockWait)
	lockWait = 300 * time.Millisecond
	rs := release(t, "0.1.0", nil, nil)
	r := start(t, "0.1.0", rs.url, nil)
	held, ok, err := r.m.lock(false)
	if err != nil || !ok {
		t.Fatalf("lock: %v %v", ok, err)
	}
	defer held.Close()
	r.m.CheckNow()
	s := r.until(t, "a failed check", func(s State) bool { return s.Check == CheckFailed })
	if s.CheckError != errBusy.Error() {
		t.Fatalf("the check said %q", s.CheckError)
	}
}
