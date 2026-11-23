package update

import (
	"fmt"
	"time"

	"github.com/debkosh/amarit/appupdate"

	"projectb/system"
)

// NoteID is the update's notification's id among the app's notifications: a click on it
// comes to NoteClicked.
const NoteID = "update"

// Notes is where the update's notification is shown: core/system, which hands it to the OS
// (oskit/notify), as a page's notification is.
type Notes interface {
	ShowNotification(n system.Notification)
	RemoveNotification(id string)
}

// appName is the app's name in the notification's words.
const appName = "Project B"

// note is what the notification says for s, as amarit's own presenters did
// (docs/update-notifications.md): ready, the version with Relaunch (Install on a phone), on
// every OS; with progress, also the download with Cancel, the check, and a failure a retry
// can help with Try again. ok is false where nothing shows.
func note(s appupdate.State, progress, phone bool) (n system.Notification, ok bool) {
	n.ID = NoteID
	switch s.Phase {
	case appupdate.Ready:
		n.Title = fmt.Sprintf("%s %s is ready", appName, s.Version)
		n.Body = "Relaunch to update."
		label := "Relaunch"
		if phone {
			n.Body, label = "Install it when you're ready.", "Install"
		}
		n.Actions = []system.NoteAction{{ID: "relaunch", Label: label}}
		n.Quiet = true
		return n, true
	}
	if !progress {
		return n, false
	}
	switch s.Phase {
	case appupdate.Downloading:
		n.Title = fmt.Sprintf("Downloading %s %s", appName, s.Version)
		n.Ongoing, n.Quiet, n.Progress = true, true, -1
		if s.Total > 0 {
			n.Progress = float64(s.Done) / float64(s.Total)
		}
		n.Actions = []system.NoteAction{{ID: "cancel", Label: "Cancel"}}
		return n, true
	case appupdate.Verifying:
		n.Title = fmt.Sprintf("Checking %s %s", appName, s.Version)
		n.Ongoing, n.Quiet, n.Progress = true, true, -1
		n.Actions = []system.NoteAction{{ID: "cancel", Label: "Cancel"}}
		return n, true
	case appupdate.Failed:
		switch s.Kind {
		case appupdate.Network, appupdate.Stalled, appupdate.Damaged, appupdate.Unchecked, appupdate.InstallFailed:
			n.Title = fmt.Sprintf("%s %s didn't download", appName, s.Version)
			n.Body = "Try again, or it's tried at the next check."
			n.Actions = []system.NoteAction{{ID: "retry", Label: "Try again"}}
			n.Quiet = true
			return n, true
		}
	}
	return n, false
}

// progressEvery is how often the download's progress is shown at most. Android sheds a
// notification's updates past a few a second, and the one shed may be the last, the release
// ready, which then never shows (seen on Android 16, 2026-11-23), as the Downloads
// notifications' once a second does.
const progressEvery = time.Second

// present shows s as the update's notification, or takes it away. Only the download's progress
// waits: a change of phase shows at once.
func (m *Manager) present(s appupdate.State) {
	if m.cfg.Notes == nil {
		return
	}
	if s.Phase == appupdate.Downloading && m.shownPhase == appupdate.Downloading && time.Since(m.shownAt) < progressEvery {
		return
	}
	m.shownPhase, m.shownAt = s.Phase, time.Now()
	if n, ok := note(s, m.cfg.ProgressNotes, m.cfg.ProgressNotes); ok {
		m.cfg.Notes.ShowNotification(n)
	} else {
		m.cfg.Notes.RemoveNotification(NoteID)
	}
	if m.cfg.Dock != nil {
		if s.Phase == appupdate.Downloading && s.Total > 0 {
			m.cfg.Dock(float64(s.Done) / float64(s.Total))
		} else {
			m.cfg.Dock(-1)
		}
	}
}

// NoteClicked is a click on the update's notification: its button's action, or "" for the
// notification itself, which only brings the app to the front.
func (m *Manager) NoteClicked(action string) {
	switch action {
	case "relaunch":
		m.Relaunch()
	case "cancel":
		m.Cancel()
	case "retry":
		m.Retry()
	}
}
