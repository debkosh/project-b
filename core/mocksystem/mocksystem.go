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

	mu              sync.Mutex
	dismiss         bool
	isDefault       bool
	allowed         bool
	dock            string
	opened, shown   []string
	defaultBrowser  system.Question
	notifications   system.Question
	asked           map[string]int
	posted, removed []string
	clicked         func(id string)
	links           func(urls []string)
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
