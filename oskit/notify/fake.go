package notify

import "sync"

// Fake is a Notifier for tests: it keeps what it was asked, and answers and clicks when the
// test says.
type Fake struct {
	mu sync.Mutex
	allow   bool
	asking  []func(bool)
	shown   map[string]Note
	order   []string
	removed []string
	clicks  clicks
}

var _ Notifier = (*Fake)(nil)

// NewFake is a Fake that allows notifications when allowed.
func NewFake(allowed bool) *Fake { return &Fake{allow: allowed, shown: map[string]Note{}} }

func (f *Fake) Allowed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.allow
}

// Ask waits for the test's Answer.
func (f *Fake) Ask(answered func(bool)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asking = append(f.asking, answered)
}

// Asking is how many asks wait for an answer.
func (f *Fake) Asking() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.asking)
}

// Answer answers every waiting ask with allowed, as the person would, and keeps it.
func (f *Fake) Answer(allowed bool) {
	f.mu.Lock()
	f.allow = allowed
	asking := f.asking
	f.asking = nil
	f.mu.Unlock()
	for _, answered := range asking {
		answered(allowed)
	}
}

func (f *Fake) Show(n Note) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.shown[n.ID]; !ok {
		f.order = append(f.order, n.ID)
	}
	f.shown[n.ID] = n
}

func (f *Fake) Remove(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, id)
	if _, ok := f.shown[id]; ok {
		delete(f.shown, id)
		for i, o := range f.order {
			if o == id {
				f.order = append(f.order[:i], f.order[i+1:]...)
				break
			}
		}
	}
}

// Shown is the notes shown now, oldest first.
func (f *Fake) Shown() []Note {
	f.mu.Lock()
	defer f.mu.Unlock()
	notes := make([]Note, 0, len(f.order))
	for _, id := range f.order {
		notes = append(notes, f.shown[id])
	}
	return notes
}

// Removed is every id Remove was given, in order.
func (f *Fake) Removed() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.removed...)
}

func (f *Fake) OnClick(clicked func(id, action string)) { f.clicks.set(clicked) }

// Click is the person clicking note id, on its button action or, with "", on the note.
func (f *Fake) Click(id, action string) { f.clicks.click(id, action) }
