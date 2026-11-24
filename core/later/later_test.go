package later

import (
	"slices"
	"sync"
	"testing"
	"time"
)

// main stands for the main thread: what was posted, in order.
type main struct {
	mu  sync.Mutex
	ran []string
}

func (m *main) post(f func()) { f() }

func (m *main) job(name string) func() {
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.ran = append(m.ran, name)
	}
}

func (m *main) has(t *testing.T, what string, want ...string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if !slices.Equal(m.ran, want) {
		t.Fatalf("%s: ran %v, want %v", what, m.ran, want)
	}
}

func (m *main) within(t *testing.T, what string, want ...string) {
	t.Helper()
	for end := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		m.mu.Lock()
		got := slices.Clone(m.ran)
		m.mu.Unlock()
		if slices.Equal(got, want) {
			return
		}
		if time.Now().After(end) {
			t.Fatalf("%s: ran %v, want %v", what, got, want)
		}
	}
}

func wait(t *testing.T, d time.Duration) {
	was := Wait
	Wait = d
	t.Cleanup(func() { Stop(); Wait = was })
}

// Jobs asked for before Begin and during the wait run once it is over, in the order they
// were asked for, and none before; one asked for after it runs at once.
func TestJobsWaitForTheOneTimer(t *testing.T) {
	wait(t, 60*time.Millisecond)
	m := &main{}
	Run(m.job("asked before"))
	Begin(m.post)
	Run(m.job("asked during"))
	time.Sleep(20 * time.Millisecond)
	m.has(t, "during the wait")
	m.within(t, "once the wait is over", "asked before", "asked during")
	Run(m.job("asked after"))
	m.has(t, "after the wait", "asked before", "asked during", "asked after")
}

// A job is posted, never called inside Run: after the wait too it runs where the main thread
// takes it.
func TestAJobIsPostedNeverCalledInRun(t *testing.T) {
	wait(t, 0)
	var posted []func()
	Begin(func(f func()) { posted = append(posted, f) })
	ran := false
	Run(func() { ran = true })
	if ran || len(posted) != 1 {
		t.Fatalf("Run after the wait: ran %v, posted %d", ran, len(posted))
	}
	posted[0]()
	if !ran {
		t.Fatal("the posted job didn't run")
	}
}

// With no wait, as the tests have it, Begin posts what was asked for at once.
func TestNoWaitRunsAtBegin(t *testing.T) {
	wait(t, 0)
	m := &main{}
	Run(m.job("a"))
	Begin(m.post)
	m.has(t, "at Begin", "a")
}

// A quit within the wait: what waits never runs, and what is asked for after runs only if
// an app starts again.
func TestStopDropsWhatWaits(t *testing.T) {
	wait(t, 40*time.Millisecond)
	m := &main{}
	Run(m.job("a"))
	Begin(m.post)
	Stop()
	Run(m.job("after the quit"))
	time.Sleep(80 * time.Millisecond)
	m.has(t, "after a quit within the wait")
}

// A Begin while another's jobs wait, as a second app in a test process, drops them and
// starts the wait over for its own, which it asks for once it has begun.
func TestBeginStartsOver(t *testing.T) {
	wait(t, 40*time.Millisecond)
	first, second := &main{}, &main{}
	Begin(first.post)
	Run(first.job("the first app's"))
	Begin(second.post)
	Run(second.job("the second app's"))
	second.within(t, "the second app", "the second app's")
	time.Sleep(60 * time.Millisecond)
	first.has(t, "the first app")
}

// Nothing is posted with later's lock held: a post that runs its function there and then,
// and a job that asks for another, as a post that waits on the main thread would find the
// main thread waiting in Run.
func TestAJobMayAskForAnother(t *testing.T) {
	for name, d := range map[string]time.Duration{"with no wait": 0, "after a wait": 20 * time.Millisecond} {
		wait(t, d)
		m := &main{}
		Begin(m.post)
		Run(func() {
			m.job("the first")()
			Run(m.job("the one it asked for"))
		})
		m.within(t, name, "the first", "the one it asked for")
		Stop()
	}
}
