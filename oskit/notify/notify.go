// Package notify is the system's notifications: an app's own, shown on the OS's notification
// centre or shade, with buttons, and their clicks back.
package notify

import (
	"context"
	"sync"

	"github.com/debkosh/project-b/oskit"
)

// Note is one notification. ID is the app's: showing another with the same ID replaces it,
// and clicks name it.
type Note struct {
	ID, Title, Subtitle, Body string
	// Actions are its buttons, up to three; the OS may show fewer.
	Actions []Action
}

// Action is a button on a note.
type Action struct{ ID, Label string }

// Notifier is the OS's notifications.
type Notifier interface {
	// Allowed says whether the app may show notifications now.
	Allowed() bool
	// Ask asks to be allowed, where the OS asks the person (Android from 13); elsewhere it
	// answers with Allowed. answered runs once.
	Ask(answered func(allowed bool))
	// Show shows n, replacing a shown note with its ID.
	Show(n Note)
	// Remove takes note id away, shown or not.
	Remove(id string)
	// OnClick is how clicks reach the app: action is the button's ID, "" for the note itself.
	// Clicks that came before it was set, as a click that started the app, come once it is.
	// Set once, as the app starts.
	OnClick(clicked func(id, action string))
}

// AskWait is Ask that waits for the answer, or for ctx to end. Not on the main thread.
func AskWait(ctx context.Context, n Notifier) (bool, error) {
	return oskit.Wait(ctx, func(answered func(bool)) func() {
		n.Ask(answered)
		return nil
	})
}

// clicks holds OnClick's func, and the clicks that came before it.
type clicks struct {
	mu      sync.Mutex
	clicked func(id, action string)
	early   [][2]string
}

func (c *clicks) set(clicked func(id, action string)) {
	c.mu.Lock()
	c.clicked = clicked
	early := c.early
	c.early = nil
	c.mu.Unlock()
	for _, e := range early {
		oskit.Post(func() { clicked(e[0], e[1]) })
	}
}

func (c *clicks) click(id, action string) {
	c.mu.Lock()
	clicked := c.clicked
	if clicked == nil {
		c.early = append(c.early, [2]string{id, action})
	}
	c.mu.Unlock()
	if clicked != nil {
		oskit.Post(func() { clicked(id, action) })
	}
}
