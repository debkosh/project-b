package update

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/debkosh/amarit/appupdate"

	"projectb/system"
)

// What the update's notification says for each state, on a phone, which shows the download
// too, and elsewhere, which shows only the release ready (docs/update-notifications.md).
func TestTheNoteForEachState(t *testing.T) {
	for _, c := range []struct {
		s       appupdate.State
		phone   bool
		title   string
		actions []system.NoteAction
		ongoing bool
	}{
		{appupdate.State{Phase: appupdate.Ready, Version: "0.4.0"}, false, "Project B 0.4.0 is ready", []system.NoteAction{{ID: "relaunch", Label: "Relaunch"}}, false},
		{appupdate.State{Phase: appupdate.Ready, Version: "0.4.0"}, true, "Project B 0.4.0 is ready", []system.NoteAction{{ID: "relaunch", Label: "Install"}}, false},
		{appupdate.State{Phase: appupdate.Downloading, Version: "0.4.0", Done: 1, Total: 4}, true, "Downloading Project B 0.4.0", []system.NoteAction{{ID: "cancel", Label: "Cancel"}}, true},
		{appupdate.State{Phase: appupdate.Verifying, Version: "0.4.0"}, true, "Checking Project B 0.4.0", []system.NoteAction{{ID: "cancel", Label: "Cancel"}}, true},
		{appupdate.State{Phase: appupdate.Failed, Version: "0.4.0", Kind: appupdate.Network}, true, "Project B 0.4.0 didn't download", []system.NoteAction{{ID: "retry", Label: "Try again"}}, false},
	} {
		n, ok := note(c.s, c.phone, c.phone)
		if !ok || n.ID != NoteID || n.Title != c.title || !reflect.DeepEqual(n.Actions, c.actions) || n.Ongoing != c.ongoing {
			t.Errorf("%+v on a phone %v: %+v, shown %v", c.s, c.phone, n, ok)
		}
	}
	if n, _ := note(appupdate.State{Phase: appupdate.Downloading, Done: 1, Total: 4}, true, true); n.Progress != 0.25 {
		t.Errorf("a quarter downloaded shows %v", n.Progress)
	}
	for _, s := range []appupdate.State{
		{Phase: appupdate.Downloading, Total: 4},
		{Phase: appupdate.Failed, Kind: appupdate.Network},
	} {
		if _, ok := note(s, false, false); ok {
			t.Errorf("%+v shows a note where only the release ready does", s)
		}
	}
	if _, ok := note(appupdate.State{Phase: appupdate.Failed, Kind: appupdate.Refused}, true, true); ok {
		t.Error("a release refused for good offers Try again")
	}
}

// notes records what the manager shows and takes away.
type notes struct {
	mu      sync.Mutex
	shown   []system.Notification
	removed int
}

func (n *notes) ShowNotification(x system.Notification) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.shown = append(n.shown, x)
}

func (n *notes) RemoveNotification(string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.removed++
}

func (n *notes) last() (system.Notification, int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.shown) == 0 {
		return system.Notification{}, n.removed
	}
	return n.shown[len(n.shown)-1], n.removed
}

// A newer release ready shows its notification, and its Install, pressed on the
// notification, takes the notification away and hands the release to the installer.
func TestTheReadyNotesInstallRelaunches(t *testing.T) {
	s := serve(t, "0.2.0", "")
	var o opened
	var shown notes
	dir := t.TempDir()
	cfg := Config{Project: "project-b", Version: "0.1.0", Releases: s.url, AutoUpdate: true, StateDir: dir, Notes: &shown, ProgressNotes: true,
		Installer: func(func(func()), func()) appupdate.Installer {
			return appupdate.File{Dir: dir, Open: func(f string) error {
				o.mu.Lock()
				defer o.mu.Unlock()
				o.files = append(o.files, f)
				return nil
			}}
		}}
	m := New(cfg, func(f func()) { f() }, func() {})
	m.CheckNow()
	eventually(t, m, func(st State) bool { return st.Phase == Ready })
	n, _ := shown.last()
	if n.Title != "Project B 0.2.0 is ready" || len(n.Actions) != 1 || n.Actions[0].ID != "relaunch" {
		t.Fatalf("the notification is %+v", n)
	}
	_, removedBefore := shown.last()
	m.NoteClicked("relaunch")
	deadline := time.Now().Add(5 * time.Second)
	for len(o.list()) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if _, removed := shown.last(); len(o.list()) != 1 || removed <= removedBefore {
		t.Fatalf("installed %v, the notification taken away %d times", o.list(), removed-removedBefore)
	}
}

// The download's progress shows once a second at most, and a change of phase at once: Android
// sheds updates past a few a second, and the release ready mustn't be the one shed.
func TestProgressShowsOnceASecondAndReadyAtOnce(t *testing.T) {
	var shown notes
	m := &Manager{cfg: Config{Notes: &shown, ProgressNotes: true}}
	for i := range 50 {
		m.present(appupdate.State{Phase: appupdate.Downloading, Version: "0.4.0", Done: int64(i), Total: 50})
	}
	m.present(appupdate.State{Phase: appupdate.Ready, Version: "0.4.0"})
	shown.mu.Lock()
	defer shown.mu.Unlock()
	if len(shown.shown) != 2 || shown.shown[1].Title != "Project B 0.4.0 is ready" {
		t.Fatalf("shown %d notes, the last %+v", len(shown.shown), shown.shown[len(shown.shown)-1])
	}
}
