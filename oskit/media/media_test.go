package media

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestAskingAsksTheOSOnceForEveryoneWaiting(t *testing.T) {
	var a asking
	asked := 0
	var got []map[string]bool
	for range 2 {
		a.ask(func(m map[string]bool) { got = append(got, m) }, func() { asked++ })
	}
	a.answer(map[string]bool{Camera: true})
	if asked != 1 || len(got) != 2 || !got[1][Camera] {
		t.Fatalf("asked %d, got %v", asked, got)
	}
}

func TestAskWaitWithTheFake(t *testing.T) {
	f := NewFake()
	go func() {
		for len(f.Asked()) == 0 {
			time.Sleep(time.Millisecond)
		}
		f.Answer(Camera)
	}()
	got, err := AskWait(context.Background(), f, []string{Camera, Microphone})
	if err != nil || !got[Camera] || got[Microphone] {
		t.Fatalf("AskWait = %v, %v", got, err)
	}
	if !reflect.DeepEqual(f.Asked(), [][]string{{Camera, Microphone}}) {
		t.Fatalf("asked %v", f.Asked())
	}
}
