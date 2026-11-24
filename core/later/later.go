// Package later holds the app's background jobs back until it has started (docs/startup.md):
// one wait, from the end of app.Start, after which every job runs. A background job is what
// the app does for itself, with no one waiting on it, as checking for updates and sync's runs
// are; what a person does is none, and runs at once where it is.
package later

import (
	"sync"
	"time"
)

// Wait is how long after a start the background jobs wait. The tests set it to 0, and wait
// for nothing.
var Wait = 10 * time.Second

var (
	mu sync.Mutex
	// post runs a function on the main thread: the last Begin's.
	post func(func())
	// asked are the jobs of a start that hasn't begun its wait yet, and waiting those whose
	// wait goes on. over says the wait ended: a job asked for now is posted at once.
	asked, waiting []func()
	over           bool
	timer          *time.Timer
)

// Begin starts the wait; app.Start calls it as its last step, and asks for its jobs after
// it. post runs a function on the main thread. It starts over, as the tests start many apps
// in a process: jobs still waiting from a Begin before are dropped, which is why an app asks
// for its own after its Begin, where they can't be taken for the app's before.
func Begin(p func(func())) {
	mu.Lock()
	if timer != nil {
		timer.Stop()
	}
	post, waiting, asked, over = p, asked, nil, false
	if Wait <= 0 {
		run := end()
		mu.Unlock()
		run()
		return
	}
	var mine *time.Timer
	mine = time.AfterFunc(Wait, func() {
		mu.Lock()
		// A Begin or a Stop since has put this timer out.
		run := func() {}
		if timer == mine {
			run = end()
		}
		mu.Unlock()
		run()
	})
	timer = mine
	mu.Unlock()
}

// end is the wait over: what it returns posts every waiting job, in the order it was asked
// for. Call it with mu held, and what it returns with mu let go: a post may wait on the main
// thread, as the mock engine's does on its queue, and the main thread may be in Run, waiting
// for mu.
func end() func() {
	jobs, p := waiting, post
	waiting, over, timer = nil, true, nil
	return func() {
		for _, f := range jobs {
			p(f)
		}
	}
}

// Run runs f on the main thread once the wait is over, or soon if it already is: f is posted,
// never called here, so it runs at the same point whenever Run was called. Before Begin it
// is asked for, and waits with the rest. The jobs must not count on their order: one asked
// for as the wait ends may be posted before those that waited.
func Run(f func()) {
	mu.Lock()
	switch {
	case over:
		// Posted with mu let go, as end's are: a job may ask for another.
		p := post
		mu.Unlock()
		p(f)
		return
	case timer != nil:
		waiting = append(waiting, f)
	default:
		asked = append(asked, f)
	}
	mu.Unlock()
}

// Stop drops what is still waiting, which never runs; App.Stop calls it. A job asked for
// after it waits for a Begin, which an app that is quitting never calls.
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
	asked, waiting, over, timer = nil, nil, false, nil
}
