package app

import (
	"time"

	"projectb/engine"
	"projectb/prefs"
	"projectb/shells"
)

// settle is how long a window's size stays put before Android's shell follows it, so
// dragging a desktop window's corner doesn't reload the shell at every step.
var settle = 500 * time.Millisecond

// shellPick is the shell the windows load. On a desktop it is the profile's shell pref.
// On Android the window's size picks between the phone shell and the wide one, unless the
// profile keeps one at every size (docs/large-screens.md). Android has one window.
type shellPick struct {
	prefs *prefs.Manager
	post  func(func())
	// narrow is the window's size as last settled; sized says one has been reported.
	narrow, sized bool
	// keyboard is the soft keyboard being up. width and height are the size that counts,
	// the height being the one from before the keyboard while it is up, as a window the
	// keyboard shortens is not a narrow one; before is the height before the last change.
	keyboard              bool
	width, height, before float64
	// waiting is the size not yet settled, and waits counts the reports, so only the last
	// one's timer takes effect.
	waiting bool
	waits   int
	now     string
	on      []func(string)
}

func newShellPick(p *prefs.Manager, post func(func())) *shellPick {
	s := &shellPick{prefs: p, post: post, narrow: true}
	s.now = s.want()
	p.OnShellChanged(s.update)
	return s
}

// InUse is the id of the shell the windows load now.
func (s *shellPick) InUse() string { return s.now }

// OnChanged subscribes to another shell being in use.
func (s *shellPick) OnChanged(f func(string)) { s.on = append(s.on, f) }

func (s *shellPick) want() string {
	if !shells.OnAndroid {
		return s.prefs.Shell()
	}
	switch s.prefs.OneShell() {
	case prefs.OneShellPhone:
		return s.prefs.PhoneShell()
	case prefs.OneShellWide:
		return s.prefs.Shell()
	}
	if s.narrow {
		return s.prefs.PhoneShell()
	}
	return s.prefs.Shell()
}

func (s *shellPick) update() {
	if w := s.want(); w != s.now {
		s.now = w
		for _, f := range s.on {
			f(w)
		}
	}
}

// WindowChanged takes the window's new size. The first decides at once, before the shell
// has drawn much; a later one once the size has settled.
func (s *shellPick) WindowChanged(st engine.WindowState) {
	if !shells.OnAndroid || st.W == 0 || st.H == 0 {
		return
	}
	s.width = st.W
	if !s.keyboard || s.height == 0 {
		s.before, s.height = s.height, st.H
	}
	s.check()
}

// KeyboardChanged takes the soft keyboard coming up or going. Android may report the size
// it makes first: a change of shell that size set waiting is then the keyboard's, and the
// height from before it counts.
func (s *shellPick) KeyboardChanged(up bool) {
	if up && !s.keyboard && s.waiting && s.before != 0 {
		s.height = s.before
	}
	s.keyboard = up
	if s.sized {
		s.check()
	}
}

func (s *shellPick) check() {
	narrow := shells.Narrow(s.width, s.height, s.narrow)
	if !s.sized {
		s.sized, s.narrow = true, narrow
		s.update()
		return
	}
	s.waits++
	n := s.waits
	if narrow == s.narrow {
		s.waiting = false
		return
	}
	s.waiting = true
	time.AfterFunc(settle, func() {
		s.post(func() {
			if n == s.waits && s.waiting {
				s.waiting, s.narrow = false, narrow
				s.update()
			}
		})
	})
}
