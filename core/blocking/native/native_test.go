//go:build pbblock

package native

import (
	"testing"

	"projectb/blocking"
)

// The library compiles and loads a group, blocks a listed third-party request, and answers
// a page's own address the way Safe Browsing asks it: a host rule and a whole address's
// rule with $document match the page, a request rule doesn't.
func TestTheLibraryBlocksRequestsAndAnswersSafeBrowsing(t *testing.T) {
	lib := Library{}
	b := lib.New()
	engine, err := lib.Compile([][]byte{
		[]byte("||ads.test^$third-party\n||harm.test^\n"),
		[]byte("|https://steal.test/login|$document\n||cdn.test/x.js^\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !b.Load("test", engine) {
		t.Fatal("the engine it compiled didn't load")
	}
	checks := []struct {
		url, source, kind string
		want              int
	}{
		{"https://ads.test/a.js", "https://site.test/", "script", blocking.BlockIt},
		{"https://ads.test/a.js", "https://ads.test/", "script", blocking.Allow},
		{"https://harm.test/", "https://harm.test/", "document", blocking.BlockIt},
		{"https://www.harm.test/page", "https://www.harm.test/page", "document", blocking.BlockIt},
		{"https://steal.test/login", "https://steal.test/login", "document", blocking.BlockIt},
		{"https://steal.test/", "https://steal.test/", "document", blocking.Allow},
		{"https://cdn.test/x.js", "https://cdn.test/x.js", "document", blocking.Allow},
		{"https://good.test/", "https://good.test/", "document", blocking.Allow},
	}
	for _, c := range checks {
		if got := b.Request(c.url, c.source, c.kind); got != c.want {
			t.Errorf("%s from %s as %s: %d, want %d", c.url, c.source, c.kind, got, c.want)
		}
	}
	// An engine of another version, or none, doesn't load, and leaves the one loaded.
	if b.Load("test", []byte("plain/1\n||good.test^\n")) {
		t.Fatal("a damaged engine loaded")
	}
	if b.Request("https://harm.test/", "https://harm.test/", "document") != blocking.BlockIt {
		t.Fatal("a failed load dropped the group")
	}
	b.Unload("test")
	if b.Request("https://harm.test/", "https://harm.test/", "document") != blocking.Allow {
		t.Fatal("unloaded, it still blocks")
	}
	if !b.Resources([]byte("[]")) || b.Resources([]byte("not json")) {
		t.Fatal("resources")
	}
	if b.Hooks() == nil || b.Hooks() != b.Hooks() {
		t.Fatal("hooks")
	}
}
