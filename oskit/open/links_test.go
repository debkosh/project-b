package open

import (
	"reflect"
	"testing"
)

// A link the OS hands the app before OnOpenURLs is set, as one that started it, comes once it
// is, before later ones.
func TestEarlyLinksComeOnceSet(t *testing.T) {
	var l links
	l.open([]string{"https://started.example/"})
	var got [][]string
	l.set(func(urls []string) { got = append(got, urls) })
	l.open([]string{"https://a.example/", "file:///tmp/b.html"})
	want := [][]string{{"https://started.example/"}, {"https://a.example/", "file:///tmp/b.html"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v; want %v", got, want)
	}
}
