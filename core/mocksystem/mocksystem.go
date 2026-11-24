// Package mocksystem is a system.System with no OS under it, beside mockengine, for the tests
// and the mock engine's dev server. It answers a question as a person who says yes does, or,
// once Dismiss is set, as one who closes the question unanswered, and keeps what the app
// asked of it for a test to read.
package mocksystem

import (
	"sync"

	"projectb/crash"
	"projectb/system"
)

// System is the fake. post runs a function on the main thread, after the call that asked
// has returned, as the real OS's answers come.
type System struct {
	post func(func())

	mu        sync.Mutex
	dismiss   bool
	isDefault bool
	desktop   string
	// mediaOS is MediaOS's answer, and mediaRefused the kinds the OS refuses, as the person
	// refused them; mediaAsked is each AskMedia's kinds, mediaOpened OpenMediaSettings's, and
	// returned OnMediaReturn's func.
	mediaOS         string
	mediaRefused    map[string]bool
	mediaAsked      [][]string
	mediaOpened     []string
	returned        func()
	stale           bool // IsDefaultBrowser reads the OS's choice at start only
	allowed         bool
	dock            string
	opened, shown   []string
	defaultBrowser  system.Question
	notifications   system.Question
	asked           map[string]int
	posted, removed []string
	clicked         func(id string)
	links           func(urls []string)
	// The checks of the person: what each was for, the verdict they get, whether they wait
	// for AnswerCheck, those waiting, and how many were taken back.
	// secret is what CopySecret last put on the clipboard as a secret.
	secret string
	// scanner is whether there is a scanner, and scan what the next scan reads, "" for a
	// person who leaves it: SetScanner.
	scanner      bool
	scan         string
	checks       []system.PersonCheck
	noCheck      bool
	verdict      system.Verdict
	holdChecks   bool
	waitingCheck []func(system.Verdict)
	cancelled    int
}

var _ system.System = (*System)(nil)

// New is a fake whose answers run through post, the mock engine's Post.
func New(post func(func())) *System {
	return &System{post: post, asked: map[string]int{}}
}

// Dismiss makes every question from now on end unanswered, as a person who closes the sheet
// does: its answer says what is so, which asking didn't change.
func (s *System) Dismiss(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dismiss = on
}

func (s *System) OpenFile(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opened = append(s.opened, path)
}

// CanScanCode is as SetScanner said, no scanner to begin with.
func (s *System) CanScanCode() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scanner
}

// ScanCode answers with what SetScanner said the scanner reads, or with ok false for "".
func (s *System) ScanCode(prompt string, answered func(text string, ok bool)) {
	s.mu.Lock()
	text, has := s.scan, s.scanner
	s.mu.Unlock()
	s.post(func() { answered(text, has && text != "") })
}

// SetScanner gives the device a scanner that reads text, or one a person leaves for "".
func (s *System) SetScanner(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanner, s.scan = true, text
}

// CopySecret keeps text as the secret last copied, for Secret.
func (s *System) CopySecret(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secret = text
}

// Secret is what CopySecret last put on the clipboard as a secret.
func (s *System) Secret() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.secret
}

func (s *System) RevealFile(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shown = append(s.shown, path)
}

func (s *System) IsDefaultBrowser() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isDefault
}

// SetDefaultBrowser makes the app the default browser, or not, as the person may in the
// OS's settings, outside the app.
func (s *System) SetDefaultBrowser(is bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isDefault = is
}

// MediaOS is the OS SetMediaOS named, "" by default, where the OS lets the app use both.
func (s *System) MediaOS() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mediaOS
}

// SetMediaOS has the mock keep the OS's own camera and microphone permission, as name
// ("android", "macos") does, allowing both until SetMediaRefused says otherwise. Before the
// app starts.
func (s *System) SetMediaOS(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mediaOS = name
}

// SetMediaRefused is the OS refusing kinds, as the person did in its question or its
// settings, and allowing the rest.
func (s *System) SetMediaRefused(kinds ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mediaRefused = map[string]bool{}
	for _, k := range kinds {
		s.mediaRefused[k] = true
	}
}

func (s *System) MediaAllowed(kind string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.mediaRefused[kind]
}

// AskMedia answers at once, through post, with what SetMediaRefused left allowed.
func (s *System) AskMedia(kinds []string, answered func(map[string]bool)) {
	s.mu.Lock()
	s.mediaAsked = append(s.mediaAsked, append([]string(nil), kinds...))
	allowed := map[string]bool{}
	for _, k := range kinds {
		allowed[k] = !s.mediaRefused[k]
	}
	s.mu.Unlock()
	s.post(func() { answered(allowed) })
}

// MediaAsked is each AskMedia's kinds, in order.
func (s *System) MediaAsked() [][]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]string(nil), s.mediaAsked...)
}

func (s *System) OpenMediaSettings(kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mediaOpened = append(s.mediaOpened, kind)
}

// MediaOpened is each kind OpenMediaSettings was given.
func (s *System) MediaOpened() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.mediaOpened...)
}

func (s *System) OnMediaReturn(returned func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.returned = returned
}

// Return is the app coming back to the front, as after the OS's settings, through post.
func (s *System) Return() {
	s.mu.Lock()
	r := s.returned
	s.mu.Unlock()
	if r != nil {
		s.post(r)
	}
}

// Desktop is the Linux desktop SetDesktop gave, "" by default, as off Linux.
func (s *System) Desktop() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.desktop
}

// DefaultBrowserLive is true unless SetDefaultBrowserStale made it Linux's Flatpak's.
func (s *System) DefaultBrowserLive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.stale
}

func (s *System) SetDefaultBrowserStale(stale bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stale = stale
}

func (s *System) SetDesktop(d string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.desktop = d
}

func (s *System) AskDefaultBrowser(answered func(bool)) {
	s.defaultBrowser.Ask(answered, func() {
		s.note("defaultBrowser")
		s.post(func() {
			s.mu.Lock()
			if !s.dismiss {
				s.isDefault = true
			}
			v := s.isDefault
			s.mu.Unlock()
			s.defaultBrowser.Answer(v)
		})
	})
}

func (s *System) NotificationsAllowed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.allowed
}

func (s *System) AskNotifications(answered func(bool)) {
	s.notifications.Ask(answered, func() {
		s.note("notifications")
		s.post(func() {
			s.mu.Lock()
			if !s.dismiss {
				s.allowed = true
			}
			v := s.allowed
			s.mu.Unlock()
			s.notifications.Answer(v)
		})
	})
}

func (s *System) SetDockColor(color string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dock = color
}

// OpenFullDiskAccess opens nothing: a test gives the access itself, by making the files the
// app reads readable.
func (s *System) OpenFullDiskAccess() {}

func (s *System) ShowNotification(n system.Notification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.posted = append(s.posted, n.ID)
}

func (s *System) RemoveNotification(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removed = append(s.removed, id)
}

func (s *System) OnNotificationClicked(clicked func(id string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clicked = clicked
}

// Click is the person clicking the notification id on the system, as a test does it: the
// app hears it on the main thread.
func (s *System) Click(id string) {
	s.mu.Lock()
	clicked := s.clicked
	s.mu.Unlock()
	if clicked != nil {
		s.post(func() { clicked(id) })
	}
}

func (s *System) OnOpenURLs(opened func(urls []string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.links = opened
}

// Open is the OS handing the app urls, as a test does it: the app hears them on the main
// thread.
func (s *System) Open(urls ...string) {
	s.mu.Lock()
	links := s.links
	s.mu.Unlock()
	if links != nil {
		s.post(func() { links(urls) })
	}
}

// Notifications are the ids of the notifications the app showed, in order, and Removed those
// it took away.
func (s *System) Notifications() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.posted...)
}

func (s *System) Removed() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.removed...)
}

// VerifyPerson answers as the person SetVerdict set does, verified to begin with, or once
// HoldChecks is on waits for AnswerCheck, as a person looking at the sheet.
func (s *System) VerifyPerson(check system.PersonCheck, answered func(system.Verdict)) (cancel func()) {
	// live is whether the check still waits: its answer and its cancel each end it once.
	live := true
	end := func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		was := live
		live = false
		return was
	}
	answer := func(v system.Verdict) {
		if end() {
			answered(v)
		}
	}
	s.mu.Lock()
	s.checks = append(s.checks, check)
	hold, v := s.holdChecks, s.verdict
	if hold {
		s.waitingCheck = append(s.waitingCheck, answer)
	}
	s.mu.Unlock()
	if !hold {
		s.post(func() { answer(v) })
	}
	return func() {
		if end() {
			s.mu.Lock()
			s.cancelled++
			s.mu.Unlock()
		}
	}
}

// CanVerifyPerson is true until SetPersonCheck says the OS has no check, as Linux.
func (s *System) CanVerifyPerson() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.noCheck
}

// SetPersonCheck is whether the OS has a check of the person.
func (s *System) SetPersonCheck(has bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.noCheck = !has
}

// SetVerdict is how every check of the person ends from now on.
func (s *System) SetVerdict(v system.Verdict) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verdict = v
}

// HoldChecks makes a check of the person wait for AnswerCheck, as the OS's sheet waits for
// the person.
func (s *System) HoldChecks(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.holdChecks = on
}

// AnswerCheck ends the oldest check waiting with v, on the main thread; one taken back
// hears nothing.
func (s *System) AnswerCheck(v system.Verdict) {
	s.mu.Lock()
	var answer func(system.Verdict)
	if len(s.waitingCheck) > 0 {
		answer, s.waitingCheck = s.waitingCheck[0], s.waitingCheck[1:]
	}
	s.mu.Unlock()
	if answer != nil {
		s.post(func() { answer(v) })
	}
}

// Checks are the checks of the person the app asked for, in order, and CancelledChecks how
// many of them it took back.
func (s *System) Checks() []system.PersonCheck {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]system.PersonCheck{}, s.checks...)
}

func (s *System) CancelledChecks() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancelled
}

func (s *System) Report(string) []crash.Section {
	return []crash.Section{{Title: "Version", Text: "Project B on the mock engine"}}
}

func (s *System) note(question string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked[question]++
}

// Asked is how often the OS was asked question, "defaultBrowser" or "notifications".
func (s *System) Asked(question string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.asked[question]
}

// DockColor is the colour the app last put on its Dock icon, or "".
func (s *System) DockColor() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dock
}

// Opened and Shown are the files the app asked to open, and to show in the file manager.
func (s *System) Opened() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.opened...)
}

func (s *System) Shown() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.shown...)
}
