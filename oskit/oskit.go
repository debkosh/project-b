// Package oskit is what an app asks of the OS itself, apart from its windows: notifications
// (notify), a device key (secret), a check of the person (verify), opening files and links
// (open) and the clipboard's secret copy (clip). Each package is an interface, New for this OS,
// and a fake for tests.
//
// Threading: nothing in oskit needs to be called on a particular thread. Every answer and
// event reaches the app through Post, never inside the call that asked. The Mac's and
// Android's halves bring it to the OS's main thread first; Post, which the app sets once with
// SetPost, takes it on from there.
package oskit

import (
	"context"
	"sync"
	"sync/atomic"
)

var poster atomic.Pointer[func(func())]

// SetPost has every answer and event run through post: an app with a main thread of its own
// posts to it. Set once, as the app starts. Unset, they run where the OS delivered them.
func SetPost(post func(func())) { poster.Store(&post) }

// Post runs f as SetPost said.
func Post(f func()) {
	if p := poster.Load(); p != nil {
		(*p)(f)
		return
	}
	f()
}

// Wait asks with ask, and waits for its answer or for ctx to end. Ended first, it runs the
// cancel ask returned, when there is one, and returns ctx's error. Call it off the main thread:
// the OS's sheet needs that thread to show.
func Wait[T any](ctx context.Context, ask func(answered func(T)) (cancel func())) (T, error) {
	got := make(chan T, 1)
	cancel := ask(func(v T) {
		select {
		case got <- v:
		default:
		}
	})
	select {
	case v := <-got:
		return v, nil
	case <-ctx.Done():
		if cancel != nil {
			cancel()
		}
		var zero T
		return zero, ctx.Err()
	}
}

// Question is one question the OS asks the person, which several callers may wait on at once:
// the OS is asked once, and each waiting answer runs once when it answers.
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

// Answer runs every waiting answer once with v, through Post.
func (q *Question) Answer(v bool) {
	q.mu.Lock()
	waiting := q.waiting
	q.waiting = nil
	q.mu.Unlock()
	Post(func() {
		for _, answered := range waiting {
			answered(v)
		}
	})
}
