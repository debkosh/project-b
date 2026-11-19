package system

import (
	"slices"
	"testing"
)

// Two askers before the OS answers: the OS is asked once, and each hears the answer once.
// An answer with nobody waiting, as a second one from the OS, runs nothing.
func TestQuestionAsksOnceAndAnswersEachOnce(t *testing.T) {
	var q Question
	asked := 0
	var heard []string
	q.Ask(func(v bool) { heard = append(heard, "a") }, func() { asked++ })
	q.Ask(func(v bool) { heard = append(heard, "b") }, func() { asked++ })
	if asked != 1 {
		t.Fatalf("the OS was asked %d times", asked)
	}
	q.Answer(true)
	q.Answer(true)
	if !slices.Equal(heard, []string{"a", "b"}) {
		t.Fatalf("heard %v", heard)
	}
	q.Ask(func(bool) {}, func() { asked++ })
	if asked != 2 {
		t.Fatalf("a question after the answer asks again: asked %d times", asked)
	}
}

// An answer that asks again, as a page might on hearing no: it waits for the next answer.
func TestQuestionAskedFromItsAnswer(t *testing.T) {
	var q Question
	asked, heard := 0, 0
	var again func(bool)
	again = func(bool) {
		heard++
		if heard == 1 {
			q.Ask(again, func() { asked++ })
		}
	}
	q.Ask(again, func() { asked++ })
	q.Answer(false)
	if asked != 2 || heard != 1 {
		t.Fatalf("asked %d, heard %d", asked, heard)
	}
	q.Answer(true)
	if heard != 2 {
		t.Fatalf("heard %d", heard)
	}
}
