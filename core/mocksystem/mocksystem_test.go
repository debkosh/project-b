package mocksystem

import (
	"testing"

	"projectb/system"
)

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

// A check of the person: its answer comes after the asking call, once, with the verdict
// set; one held waits for AnswerCheck; and one taken back never answers.
func TestVerifyPerson(t *testing.T) {
	var q queue
	s := New(q.post)
	var heard []system.Verdict
	hear := func(v system.Verdict) { heard = append(heard, v) }
	s.VerifyPerson(system.PersonCheck{Reason: "use a passkey for example.com"}, hear)
	if len(heard) != 0 {
		t.Fatal("answered inside the call that asked")
	}
	q.run()
	s.SetVerdict(system.NoScreenLock)
	s.VerifyPerson(system.PersonCheck{}, hear)
	q.run()
	if len(heard) != 2 || heard[0] != system.Verified || heard[1] != system.NoScreenLock {
		t.Fatalf("heard %v", heard)
	}

	s.HoldChecks(true)
	cancel := s.VerifyPerson(system.PersonCheck{}, hear)
	s.VerifyPerson(system.PersonCheck{}, hear)
	q.run()
	if len(heard) != 2 {
		t.Fatalf("a held check answered: %v", heard)
	}
	cancel()
	cancel()
	s.AnswerCheck(system.Verified)
	s.AnswerCheck(system.Denied)
	q.run()
	if len(heard) != 3 || heard[2] != system.Denied || s.CancelledChecks() != 1 {
		t.Fatalf("heard %v, %d taken back", heard, s.CancelledChecks())
	}
	if got := s.Checks(); len(got) != 4 || got[0].Reason != "use a passkey for example.com" {
		t.Fatalf("checks %v", got)
	}
}
