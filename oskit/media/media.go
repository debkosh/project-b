// Package media is the OS's own permission for the app's camera and microphone: Android's
// runtime permissions, macOS's Privacy & Security. An app asks it once a page it allows wants
// either, as Chrome does, and sends the person to the OS's settings when the OS says no.
package media

import (
	"context"
	"sync"

	"github.com/debkosh/project-b/oskit"
)

// The kinds.
const (
	Camera     = "camera"
	Microphone = "microphone"
)

// Media is the OS's camera and microphone permission.
type Media interface {
	// Allowed says whether the OS lets the app use kind now. A device with none of kind has
	// nothing to allow, and says yes, so the page gets the engine's own answer.
	Allowed(kind string) bool
	// Ask asks the OS for kinds, which may ask the person: Android's question, macOS's.
	// answered runs once, through oskit.Post, with whether each is allowed now. A kind the
	// person refused for good is answered no at once.
	Ask(kinds []string, answered func(allowed map[string]bool))
	// OpenSettings opens where the person can let the app use kind: Android's app details,
	// the Mac's Privacy & Security at Camera or Microphone.
	OpenSettings(kind string)
	// OnReturn is how the app hears it came back to the front, as after the OS's settings:
	// Android's activity resumed, the Mac app became active. Set once, as the app starts.
	OnReturn(returned func())
}

// AskWait is Ask that waits for the answer, or for ctx to end. Not on the main thread.
func AskWait(ctx context.Context, m Media, kinds []string) (map[string]bool, error) {
	return oskit.Wait(ctx, func(answered func(map[string]bool)) func() {
		m.Ask(kinds, answered)
		return nil
	})
}

// asking is one question to the OS at a time: asks while it is open wait for its answer, as
// the OS answers for every kind at once.
type asking struct {
	mu      sync.Mutex
	waiting []func(map[string]bool)
}

// ask adds answered to those waiting, and asks with ask when none waited before.
func (a *asking) ask(answered func(map[string]bool), ask func()) {
	a.mu.Lock()
	a.waiting = append(a.waiting, answered)
	first := len(a.waiting) == 1
	a.mu.Unlock()
	if first {
		ask()
	}
}

// answer runs every waiting answer once with allowed, through Post.
func (a *asking) answer(allowed map[string]bool) {
	a.mu.Lock()
	waiting := a.waiting
	a.waiting = nil
	a.mu.Unlock()
	oskit.Post(func() {
		for _, answered := range waiting {
			answered(allowed)
		}
	})
}

// returns holds OnReturn's func.
type returns struct {
	mu       sync.Mutex
	returned func()
}

func (r *returns) set(f func()) { r.mu.Lock(); r.returned = f; r.mu.Unlock() }

func (r *returns) fire() {
	r.mu.Lock()
	f := r.returned
	r.mu.Unlock()
	if f != nil {
		oskit.Post(f)
	}
}
