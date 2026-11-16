package app_test

import (
	"slices"
	"testing"

	"projectb/engine"
)

// Print… prints the active tab's page, from the menu bar and from a right-click; a page's
// own window.print() prints it while its tab is the one shown, and not from behind.
func TestPrintingAPage(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	tab := r.activeID()

	r.e.Fire(1, 0, "menu", "page.print")
	r.e.Settle()
	if got := r.e.Printed(); !slices.Equal(got, []engine.TabID{tab}) {
		t.Fatalf("Print… printed %v", got)
	}

	r.choose(r.rightClick(`{}`), "page.print")
	r.e.Settle()
	if got := r.e.Printed(); !slices.Equal(got, []engine.TabID{tab, tab}) {
		t.Fatalf("the right-click's Print… printed %v", got)
	}

	r.e.Fire(1, tab, "print", "")
	r.e.Settle()
	if got := r.e.Printed(); !slices.Equal(got, []engine.TabID{tab, tab, tab}) {
		t.Fatalf("window.print() printed %v", got)
	}

	// A link opened in a new tab opens behind; its page can't print from there.
	r.choose(r.rightClick(`{"link":"https://b.test/"}`), "link.newTab")
	r.waitForTab(func(t map[string]any) bool { return t["url"] == "https://b.test/" && t["loading"] == false })
	var behind engine.TabID
	r.onMain(func() {
		for _, t := range r.a.Tabs.All() {
			if t.URL == "https://b.test/" {
				behind = t.ID
			}
		}
	})
	r.e.Fire(1, behind, "print", "")
	r.e.Settle()
	if got := r.e.Printed(); len(got) != 3 {
		t.Fatalf("a tab behind printed: %v", got)
	}
}
