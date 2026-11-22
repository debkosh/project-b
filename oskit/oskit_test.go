package oskit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitAnswered(t *testing.T) {
	v, err := Wait(context.Background(), func(answered func(int)) func() {
		go answered(7)
		return nil
	})
	if v != 7 || err != nil {
		t.Fatalf("Wait = %v, %v; want 7, nil", v, err)
	}
}

func TestWaitCancelled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	tookBack := false
	v, err := Wait(ctx, func(answered func(bool)) func() {
		return func() { tookBack = true }
	})
	if v || !errors.Is(err, context.DeadlineExceeded) || !tookBack {
		t.Fatalf("Wait = %v, %v, took back %v; want false, deadline, true", v, err, tookBack)
	}
}

func TestWaitAnsweredTwiceKeepsFirst(t *testing.T) {
	v, _ := Wait(context.Background(), func(answered func(int)) func() {
		answered(1)
		answered(2)
		return nil
	})
	if v != 1 {
		t.Fatalf("Wait = %v; want 1", v)
	}
}

func TestQuestionAsksOnce(t *testing.T) {
	var q Question
	asked := 0
	var got []bool
	for range 3 {
		q.Ask(func(v bool) { got = append(got, v) }, func() { asked++ })
	}
	q.Answer(true)
	if asked != 1 || len(got) != 3 || !got[0] || !got[2] {
		t.Fatalf("asked %d, got %v; want 1, three trues", asked, got)
	}
	q.Ask(func(bool) {}, func() { asked++ })
	if asked != 2 {
		t.Fatalf("asked %d after an answer; want a new question", asked)
	}
}

func TestPostRunsThroughPoster(t *testing.T) {
	defer poster.Store(nil)
	var queued []func()
	SetPost(func(f func()) { queued = append(queued, f) })
	ran := false
	Post(func() { ran = true })
	if ran || len(queued) != 1 {
		t.Fatalf("ran %v, queued %d; want posted, not run", ran, len(queued))
	}
	queued[0]()
	if !ran {
		t.Fatal("posted func didn't run")
	}
}
