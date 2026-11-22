// Package system is the Go core's door to the OS: what the managers ask of the OS itself, apart
// from the engine, which only shows pages (docs/layers.md). Each OS has its own implementation,
// in system/platform, which only the core's library links; mocksystem is the tests' and the
// mock engine's. This package is the interface alone, so the managers build and test with no cgo.
//
// Threading: every call is made on the main thread, and an implementation hops to the thread
// its OS wants itself. Each answer runs on the main thread exactly once for each question
// asked, after the call that asked has returned.
package system

import (
	"sync"

	"projectb/crash"
)

// System is what the managers ask of the OS.
type System interface {
	// OpenFile opens a file as the OS would, in the app it opens with. RevealFile shows it in
	// the file manager: Finder on the Mac, the Downloads app on a phone.
	OpenFile(path string)
	RevealFile(path string)

	// IsDefaultBrowser says whether the app is the default browser now. AskDefaultBrowser asks
	// the OS to make it so, which asks the person: macOS's question, Android's sheet. answered
	// says whether it is once the person has answered; a question dismissed says what is so.
	IsDefaultBrowser() bool
	AskDefaultBrowser(answered func(isDefault bool))
	// Desktop is the Linux desktop the app runs on, "gnome", "kde" or "other", for the pages'
	// words where the person must go to the desktop's own settings, as to set the default
	// browser; "" off Linux.
	Desktop() string

	// NotificationsAllowed says whether the app may show its own notifications, as a
	// download's. AskNotifications asks, where the OS asks the person: a phone, from Android 13.
	// A Mac's are the app's own to show: allowed, and answered says so.
	NotificationsAllowed() bool
	AskNotifications(answered func(allowed bool))

	// SetDockColor draws the app's Dock icon with a small dot in color, "#rrggbb", the
	// profile's, so each running profile's copy tells itself apart there; tones.None, no
	// colour picked, draws no dot. A phone has no Dock.
	SetDockColor(color string)

	// OpenFullDiskAccess opens the Mac's System Settings at Privacy & Security's Full Disk
	// Access, where the person can let the app read what macOS keeps behind it. A phone's data
	// is each app's own, and it does nothing.
	OpenFullDiskAccess()

	// ShowNotification shows a page's notification on the system, once Go has let it. A
	// click on it comes back through OnNotificationClicked; RemoveNotification takes it away,
	// as when its page closes it. A phone shows no page's notification yet, and does nothing.
	ShowNotification(n Notification)
	RemoveNotification(id string)
	// OnNotificationClicked is how a click on a notification ShowNotification showed reaches
	// Go: clicked runs on the main thread with its id. Set once, as the app starts.
	OnNotificationClicked(clicked func(id string))

	// OnOpenURLs is how the links and files the OS hands the app reach Go: on the Mac what
	// it opens with the app as the default browser or a file's app, on Android a link
	// another app sends. opened runs on the main thread with them. Set once, as the app or
	// the picker starts.
	OnOpenURLs(opened func(urls []string))

	// VerifyPerson checks that the person at the device is its owner, with whatever the
	// device has: Touch ID or the Mac's password; a fingerprint, a face or the screen lock
	// on a phone. The OS asks in a sheet of its own, each time, and nothing is remembered.
	// answered runs once with how it ended, unless the returned cancel ran first, which
	// takes the sheet away and leaves answered unrun.
	VerifyPerson(check PersonCheck, answered func(Verdict)) (cancel func())

	// CopySecret puts text on the clipboard marked as a secret, a password a person copied,
	// so that what keeps or shows the clipboard's history leaves it out: on the Mac the
	// pasteboard's concealed and transient types, which clipboard managers and Universal
	// Clipboard's history honour; on Android the clip's sensitive flag, which also hides the
	// system's preview of it from Android 13.
	CopySecret(text string)

	// Report is what a crash report says of this device, for the app at version: the
	// release, the OS and the device, and on Android the app's own log.
	Report(version string) []crash.Section
}

// Notification is a page's notification as the system shows it: its title, the site's host
// under it, and its body. ID is the engine's, for its click.
type Notification struct {
	ID, Title, Subtitle, Body string
}

// PersonCheck is what the OS's sheet says a check of the person is for. Reason ends the
// Mac's sentence, "Project B is trying to": "use a passkey for example.com". Title and
// Subtitle are a phone's sheet's.
type PersonCheck struct {
	Reason, Title, Subtitle string
}

// Verdict is how a check of the person ended.
type Verdict int

const (
	// Verified: the person is the device's owner.
	Verified Verdict = iota
	// Denied: the person said no, or didn't pass.
	Denied
	// NoScreenLock: the device has nothing to check a person with.
	NoScreenLock
)

// Question is one question the OS asks the person, which several callers may wait on at
// once: the OS is asked once, and each waiting answer runs once when it answers.
type Question struct {
	mu      sync.Mutex
	waiting []func(bool)
}

// Ask adds answered to those waiting, and asks the OS with ask when none waited before.
func (q *Question) Ask(answered func(bool), ask func()) {
	q.mu.Lock()
	q.waiting = append(q.waiting, answered)
	first := len(q.waiting) == 1
	q.mu.Unlock()
	if first {
		ask()
	}
}

// Answer runs every waiting answer once with v, on the caller's thread, the main one.
func (q *Question) Answer(v bool) {
	q.mu.Lock()
	waiting := q.waiting
	q.waiting = nil
	q.mu.Unlock()
	for _, answered := range waiting {
		answered(v)
	}
}
