package notify

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestShowReplacesAndRemoves(t *testing.T) {
	f := NewFake(true)
	f.Show(Note{ID: "a", Title: "one"})
	f.Show(Note{ID: "b", Title: "two"})
	f.Show(Note{ID: "a", Title: "three"})
	if got := f.Shown(); len(got) != 2 || got[0].Title != "three" || got[1].ID != "b" {
		t.Fatalf("shown %+v; want a (three), b", got)
	}
	f.Remove("a")
	f.Remove("gone")
	if got := f.Shown(); len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("shown %+v; want b", got)
	}
	if got := f.Removed(); !reflect.DeepEqual(got, []string{"a", "gone"}) {
		t.Fatalf("removed %v", got)
	}
}

func TestClickBeforeOnClickComesLater(t *testing.T) {
	f := NewFake(true)
	f.Click("started-the-app", "")
	var got [][2]string
	f.OnClick(func(id, action string) { got = append(got, [2]string{id, action}) })
	f.Click("b", "reply")
	want := [][2]string{{"started-the-app", ""}, {"b", "reply"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clicks %v; want %v", got, want)
	}
}

func TestAskWait(t *testing.T) {
	f := NewFake(false)
	go func() {
		for f.Asking() == 0 {
			time.Sleep(time.Millisecond)
		}
		f.Answer(true)
	}()
	ok, err := AskWait(context.Background(), f)
	if !ok || err != nil || !f.Allowed() {
		t.Fatalf("AskWait = %v, %v; allowed %v", ok, err, f.Allowed())
	}
}

func TestAskWaitEnds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if ok, err := AskWait(ctx, NewFake(false)); ok || err == nil {
		t.Fatalf("AskWait = %v, %v; want false and the context's error", ok, err)
	}
}
