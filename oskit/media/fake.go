package media

import "sync"

// Fake is a Media for tests: the OS allows what the test says, and answers an ask when the
// test does, as the person would.
type Fake struct {
	mu       sync.Mutex
	allowed  map[string]bool
	asked    [][]string
	answers  []func(map[string]bool)
	opened   []string
	returned func()
}

var _ Media = (*Fake)(nil)

// NewFake is a Fake that allows the kinds given.
func NewFake(allowed ...string) *Fake {
	f := &Fake{allowed: map[string]bool{}}
	for _, k := range allowed {
		f.allowed[k] = true
	}
	return f
}

func (f *Fake) Allowed(kind string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.allowed[kind]
}

// Set is the person changing the OS's setting for kind, as in its settings.
func (f *Fake) Set(kind string, allowed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.allowed[kind] = allowed
}

// Ask waits for the test's Answer.
func (f *Fake) Ask(kinds []string, answered func(map[string]bool)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, append([]string(nil), kinds...))
	f.answers = append(f.answers, answered)
}

// Asked is every ask's kinds, in order.
func (f *Fake) Asked() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]string(nil), f.asked...)
}

// Answer answers every waiting ask, the person allowing the kinds given and refusing the rest
// asked.
func (f *Fake) Answer(allow ...string) {
	f.mu.Lock()
	for _, k := range allow {
		f.allowed[k] = true
	}
	answers := f.answers
	f.answers = nil
	now := map[string]bool{}
	for k, v := range f.allowed {
		now[k] = v
	}
	f.mu.Unlock()
	for _, answered := range answers {
		answered(now)
	}
}

func (f *Fake) OpenSettings(kind string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, kind)
}

// Opened is every kind OpenSettings was given.
func (f *Fake) Opened() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.opened...)
}

func (f *Fake) OnReturn(returned func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.returned = returned
}

// Return is the app coming back to the front.
func (f *Fake) Return() {
	f.mu.Lock()
	r := f.returned
	f.mu.Unlock()
	if r != nil {
		r()
	}
}
