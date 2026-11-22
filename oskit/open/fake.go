package open

import (
	"sync"

	"github.com/sudiptadeb/oskit"
)

// Fake is an Opener for tests: it keeps what it was asked to open, and is the default browser
// when the test says, or once asked unless the test has the person say no.
type Fake struct {
	mu              sync.Mutex
	opened, shown   []string
	isDefault, deny bool
	urls            func([]string)
}

var _ Opener = (*Fake)(nil)

func (f *Fake) OpenFile(path string) { f.mu.Lock(); f.opened = append(f.opened, path); f.mu.Unlock() }
func (f *Fake) Reveal(path string)   { f.mu.Lock(); f.shown = append(f.shown, path); f.mu.Unlock() }

// Opened and Shown are the files OpenFile and Reveal were given.
func (f *Fake) Opened() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.opened...)
}
func (f *Fake) Shown() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.shown...)
}

func (f *Fake) IsDefaultBrowser() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.isDefault }

// SetDefaultBrowser makes the app the default browser or not, as the OS's settings would; Deny
// has the person say no when asked.
func (f *Fake) SetDefaultBrowser(is bool) { f.mu.Lock(); f.isDefault = is; f.mu.Unlock() }
func (f *Fake) Deny(no bool)              { f.mu.Lock(); f.deny = no; f.mu.Unlock() }

func (f *Fake) AskDefaultBrowser(answered func(bool)) {
	f.mu.Lock()
	if !f.deny {
		f.isDefault = true
	}
	is := f.isDefault
	f.mu.Unlock()
	oskit.Post(func() { answered(is) })
}

func (f *Fake) OnOpenURLs(opened func([]string)) { f.mu.Lock(); f.urls = opened; f.mu.Unlock() }

// Hand is the OS handing the running app urls.
func (f *Fake) Hand(urls []string) {
	f.mu.Lock()
	opened := f.urls
	f.mu.Unlock()
	if opened != nil {
		oskit.Post(func() { opened(urls) })
	}
}
