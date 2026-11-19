package mocksystem

import "testing"

// queue is a main thread: posted functions run when the test says.
type queue []func()

func (q *queue) post(fn func()) { *q = append(*q, fn) }
func (q *queue) run() {
	for len(*q) > 0 {
		fn := (*q)[0]
		*q = (*q)[1:]
		fn()
	}
}

// A yes: the answer comes after the asking call, once, and the state follows.
func TestAskedAndAnswered(t *testing.T) {
	var q queue
	s := New(q.post)
	var heard []bool
	s.AskDefaultBrowser(func(v bool) { heard = append(heard, v) })
	if len(heard) != 0 {
		t.Fatal("answered inside the call that asked")
	}
	q.run()
	if len(heard) != 1 || !heard[0] || !s.IsDefaultBrowser() {
		t.Fatalf("heard %v, default %v", heard, s.IsDefaultBrowser())
	}
}

// A question dismissed: each caller hears once, with what is so, and the OS was asked once.
func TestDismissedAnswersOnceWithWhatIsSo(t *testing.T) {
	var q queue
	s := New(q.post)
	s.Dismiss(true)
	heard := 0
	for range 2 {
		s.AskNotifications(func(v bool) {
			heard++
			if v {
				t.Error("a dismissed question said yes")
			}
		})
	}
	q.run()
	if heard != 2 || s.Asked("notifications") != 1 || s.NotificationsAllowed() {
		t.Fatalf("heard %d, asked %d, allowed %v", heard, s.Asked("notifications"), s.NotificationsAllowed())
	}
}
