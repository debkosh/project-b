//go:build !android

package media

// linux lets the app use both: a Flatpak's devices are its manifest's, and the Camera portal is
// later. Ask answers yes, after the call that asked.
type linux struct{ returns returns }

var the = &linux{}

// New is this OS's Media.
func New() Media { return the }

func (*linux) Allowed(string) bool { return true }

func (*linux) Ask(kinds []string, answered func(map[string]bool)) {
	allowed := map[string]bool{}
	for _, k := range kinds {
		allowed[k] = true
	}
	var a asking
	a.waiting = []func(map[string]bool){answered}
	go a.answer(allowed)
}

func (*linux) OpenSettings(string) {}

func (l *linux) OnReturn(returned func()) { l.returns.set(returned) }
