//go:build darwin || (linux && !android)

package instance

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSecondCopyHandsOver(t *testing.T) {
	dir := t.TempDir()
	first, err := Claim(dir, nil)
	if err != nil || first == nil {
		t.Fatalf("first copy: %v, %v", first, err)
	}
	got := make(chan []string, 1)
	first.Serve(func(urls []string) { got <- urls }, func() {})

	want := []string{"https://example.com/", "https://example.org/"}
	second, err := Claim(dir, want)
	if err != nil || second != nil {
		t.Fatalf("second copy should hand over and quit: %v, %v", second, err)
	}
	select {
	case urls := <-got:
		if !reflect.DeepEqual(urls, want) {
			t.Fatalf("owner got %v, want %v", urls, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the owner never got the URLs")
	}

	first.Close()
	third, err := Claim(dir, nil)
	if err != nil || third == nil {
		t.Fatalf("after the owner closed, a new copy should own the folder: %v, %v", third, err)
	}
	third.Close()
}

func TestLongProfilePath(t *testing.T) {
	dir := t.TempDir() + "/" + strings.Repeat("a-long-folder-name/", 8)
	o, err := Claim(dir, nil)
	if err != nil || o == nil {
		t.Fatalf("a %d-byte profile path: %v, %v", len(dir), o, err)
	}
	o.Close()
}

// A copy relaunching for an update asks the owner of a folder to quit: the owner answers
// with its process and quits. A folder nobody owns has no copy to ask.
func TestTheOwnerAnswersQuitWithItsProcessAndQuits(t *testing.T) {
	dir := t.TempDir()
	if pid, err := Quit(dir); pid != 0 || err != nil {
		t.Fatalf("a folder no copy has owned: %d, %v", pid, err)
	}
	o, err := Claim(dir, nil)
	if err != nil || o == nil {
		t.Fatalf("claim: %v, %v", o, err)
	}
	quit := make(chan struct{})
	o.Serve(func([]string) { t.Error("a quit was handed over as URLs") }, func() { close(quit) })
	pid, err := Quit(dir)
	if err != nil || pid != os.Getpid() {
		t.Fatalf("the owner answered %d, %v", pid, err)
	}
	<-quit
	o.Close()
	if pid, err := Quit(dir); pid != 0 || err != nil {
		t.Fatalf("a folder whose owner has gone: %d, %v", pid, err)
	}
}

// A copy the Flatpak portal starts for an update, while the copy it replaces is quitting,
// waits for the folder rather than handing itself over; one whose owner never goes hands
// over once the wait is up.
func TestClaimAfterWaitsForTheOwnerToGo(t *testing.T) {
	dir := t.TempDir()
	old, err := Claim(dir, nil)
	if err != nil || old == nil {
		t.Fatalf("the old copy: %v, %v", old, err)
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		old.Close()
	}()
	started := time.Now()
	next, err := ClaimAfter(dir, 10*time.Second)
	if err != nil || next == nil {
		t.Fatalf("the new copy should own the folder once the old one has gone: %v, %v", next, err)
	}
	if waited := time.Since(started); waited < 250*time.Millisecond {
		t.Fatalf("it owned the folder after %v, while the old copy still held it", waited)
	}

	got := make(chan []string, 1)
	next.Serve(func(urls []string) { got <- urls }, func() {})
	defer next.Close()
	if o, err := ClaimAfter(dir, 200*time.Millisecond); err != nil || o != nil {
		t.Fatalf("with the owner staying, it should hand over: %v, %v", o, err)
	}
	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("the owner was never handed over to")
	}
}
