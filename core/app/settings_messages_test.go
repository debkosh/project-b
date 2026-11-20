package app_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sudiptadeb/amarit/appupdate"

	"projectb/app"
	"projectb/commands"
	"projectb/engine"
	"projectb/profiles"
	"projectb/update"
)

// Each thing a person does on the Settings page reaches Go as the page sends it, by name,
// and does what it says (docs/action-coverage.md): On start, the search engine, the address
// bar's place, a flag, and a site's setting forgotten.
func TestSettingsMessagesChangeWhatTheySay(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	got, _ := r.openSettings()

	r.settings(`{"t":"setStart","text":"fresh"}`)
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "start" && m["start"] == "fresh" })

	r.settings(`{"t":"setSearchEngine","text":"ecosia"}`)
	r.settings(`{"t":"setAddressBarPosition","text":"top"}`)
	r.settings(`{"t":"setFlag","id":"features","text":"SomeFeature"}`)
	var engineID, position, features string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.main(func() {
			engineID, position = r.a.Prefs.All().SearchEngine, r.a.Prefs.AddressBarPosition()
			for _, f := range r.a.Flags.View() {
				if f.ID == "features" {
					features = f.Value
				}
			}
		})
		if engineID == "ecosia" && position == "top" && features == "SomeFeature" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if engineID != "ecosia" || position != "top" || features != "SomeFeature" {
		t.Fatalf("search engine %q, address bar %q, features flag %q", engineID, position, features)
	}

	const origin = "https://a.test"
	r.settings(`{"t":"setSiteSetting","origin":"` + origin + `","kind":"notifications","value":"allow"}`)
	r.settings(`{"t":"forgetSiteSetting","origin":"` + origin + `","kind":"notifications"}`)
	var status engine.PermissionStatus
	r.main(func() { status = r.a.Sites.Status(0, origin, "notifications") })
	if status != engine.PermissionAsk {
		t.Fatalf("after forgetSiteSetting, notifications for %s are %v, not asked again", origin, status)
	}
}

// Keyboard on the Settings page, by the messages it sends: a key's reserve, an action hidden
// from its menu, a key removed, an action's keys reset, a keymap template chosen, and the
// notice of keys gone missing dismissed.
func TestSettingsKeyboardMessages(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	got, _ := r.openSettings()
	action := func(id string) (a commands.ActionView) {
		r.main(func() {
			for _, g := range r.a.Commands.View().Groups {
				for _, x := range g.Actions {
					if x.ID == id {
						a = x
					}
				}
			}
		})
		return a
	}
	keyboard := func(ok func() bool) {
		t.Helper()
		r.waitOn(got, func(m map[string]any) bool { return m["t"] == "keyboard" && ok() })
	}
	const id = "tab.duplicate"
	// The key added is the one the action didn't have before, by its place in the action's
	// own keys, which the view names.
	had := map[string]bool{}
	for _, k := range action(id).Keys {
		had[k.Keys] = true
	}
	added := func() (commands.BindingView, bool) {
		for _, k := range action(id).Keys {
			if !had[k.Keys] {
				return k, true
			}
		}
		return commands.BindingView{}, false
	}
	r.settings(`{"t":"addKey","id":"` + id + `","text":"Cmd+Shift+U"}`)
	keyboard(func() bool { _, ok := added(); return ok })
	k, _ := added()
	r.settings(fmt.Sprintf(`{"t":"setReserve","id":"%s","index":%d,"reserve":"always"}`, id, k.Index))
	keyboard(func() bool { k, ok := added(); return ok && k.Reserve == "always" })
	r.settings(`{"t":"setHidden","id":"` + id + `","on":true}`)
	keyboard(func() bool { return action(id).Hidden })
	r.settings(fmt.Sprintf(`{"t":"removeKey","id":"%s","index":%d}`, id, k.Index))
	keyboard(func() bool { _, ok := added(); return !ok })
	r.settings(`{"t":"resetAction","id":"` + id + `"}`)
	keyboard(func() bool { a := action(id); return !a.Hidden && !a.Changed })

	r.settings(`{"t":"useTemplate","text":"firefox"}`)
	keyboard(func() bool {
		var tpl string
		r.main(func() { tpl = r.a.Commands.Template() })
		return tpl == "firefox"
	})
	r.settings(`{"t":"dismissMissing"}`)
	keyboard(func() bool {
		var n int
		r.main(func() { n = len(r.a.Commands.View().Missing) })
		return n == 0
	})
}

// Settings' update buttons, by the messages they send: Check for updates finds a release and
// downloads it; the download's Retry starts it again; Cancel stops it and shows nothing.
func TestSettingsUpdateButtons(t *testing.T) {
	asset := []byte("a release, as far as the updater cares")
	sum := sha256.Sum256(asset)
	// The download sends its first bytes and then waits, so it is still running for Cancel.
	release := make(chan struct{})
	defer close(release)
	var asked atomic.Int32 // the asset's requests
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/asset" {
			asked.Add(1)
			w.Header().Set("Content-Length", fmt.Sprint(len(asset)))
			w.Write(asset[:4])
			w.(http.Flusher).Flush()
			select {
			case <-release:
			case <-req.Context().Done():
			}
			return
		}
		fmt.Fprintf(w, `{"spec":1,"project":"project-b","published":"2026-11-14T00:00:00Z","channels":{"stable":"0.9.0"},
		  "releases":{"0.9.0":{"assets":{"%s/%s":{"url":"%s/asset","sha256":"%s","size":%d}}}}}`,
			runtime.GOOS, runtime.GOARCH, srv.URL, hex.EncodeToString(sum[:]), len(asset))
	}))
	defer srv.Close()
	dir := t.TempDir()
	updates := filepath.Join(dir, "updates")
	r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Update: update.Config{
		Project: "project-b", Version: "0.1.0", Releases: srv.URL, StateDir: updates,
		Installer: func(func()) appupdate.Installer {
			return appupdate.File{Dir: updates, Open: func(string) error { return nil }}
		}}})
	defer r.stop()
	r.openSettings()
	phase := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		var s update.State
		for time.Now().Before(deadline) {
			r.main(func() { s = r.a.Update.State() })
			if s.Phase == want {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("the update is %+v, not %q", s, want)
	}
	r.settings(`{"t":"checkUpdate"}`)
	phase(update.Downloading)
	r.settings(`{"t":"retryUpdate"}`)
	phase(update.Downloading)
	// Retry's new download has begun before Cancel: a Cancel during Retry's restart left
	// the new one running (amarit, reported).
	for deadline := time.Now().Add(5 * time.Second); asked.Load() < 2 && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	r.settings(`{"t":"cancelUpdate"}`)
	phase(update.Idle)
	// The cancelled download clears its file on its own goroutine: the test's folder goes
	// only once it has.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if left, _ := os.ReadDir(filepath.Join(updates, "download")); len(left) == 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
}
