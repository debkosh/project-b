package app_test

import (
	"fmt"
	"testing"
)

func TestTheSuggestionListAndChoosingFromIt(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	if e, _ := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })["searchEngine"].(map[string]any); e["id"] != "duckduckgo" || e["name"] != "DuckDuckGo" {
		t.Fatalf("snapshot's search engine = %v", e)
	}
	r.visit("https://mail.test/")
	r.show(1)

	// Words: the visited host they start, completed, which is open in a tab, so its row is
	// the tab's; then the search, with the profile's engine.
	r.shell(`{"t":"suggest","id":3,"text":"mail","kinds":["address","search","tab","history","action"]}`)
	items := r.waitFor(func(m map[string]any) bool { return m["t"] == "suggestions" && m["id"] == float64(3) })["items"].([]any)
	first, second := items[0].(map[string]any), items[1].(map[string]any)
	if first["kind"] != "tab" || first["tab"] != float64(1) || first["inline"] != ".test" || second["kind"] != "search" || second["url"] != "https://duckduckgo.com/?q=mail" {
		t.Fatalf("items = %v", items)
	}

	// Another engine chosen in settings: the shells hear, and words go to it.
	r.e.Post(func() { r.a.Prefs.SetSearchEngine("google") })
	r.waitFor(func(m map[string]any) bool {
		e, _ := m["searchEngine"].(map[string]any)
		return m["t"] == "searchEngine" && e["name"] == "Google"
	})
	r.shell(`{"t":"navigate","text":"two words"}`)
	r.waitForTab(func(tab map[string]any) bool {
		return tab["url"] == "https://www.google.com/search?q=two+words" && tab["loading"] == false && tab["title"] != ""
	})

	// A row chosen for a new tab opens one and shows it; a tab's row shows that tab.
	r.shell(`{"t":"chooseSuggestion","row":{"kind":"history","url":"https://news.test/"},"newTab":true}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
	if m["active"] == float64(1) {
		t.Fatalf("the new tab should show: %v", m)
	}
	r.waitForTab(func(tab map[string]any) bool {
		return tab["url"] == "https://news.test/" && tab["loading"] == false && tab["title"] != ""
	})
	r.shell(`{"t":"chooseSuggestion","row":{"kind":"tab","tab":1}}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && m["active"] == float64(1) })
	// An action's row runs it: Duplicate Tab makes a third.
	r.shell(`{"t":"chooseSuggestion","row":{"kind":"action","action":"tab.duplicate"}}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 3 })
}

// With nothing typed, a phone's field as it is tapped asks for the window's other tabs and
// the tabs closed lately; choosing a closed one reopens it, as Recently Closed does.
func TestNothingTypedOffersTabsAndReopensAClosedOne(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.shell(`{"t":"newTab"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
	b := m["active"].(float64)
	r.visit("https://b.test/")
	r.shell(`{"t":"newTab"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 3 })
	r.visit("https://c.test/")
	r.shell(fmt.Sprintf(`{"t":"closeTab","id":%v}`, b))
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })

	r.shell(`{"t":"suggest","id":9,"text":"","kinds":["tab","closed"]}`)
	items := r.waitFor(func(m map[string]any) bool { return m["t"] == "suggestions" && m["id"] == float64(9) })["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items = %v", items)
	}
	tab, closed := items[0].(map[string]any), items[1].(map[string]any)
	// c.test is the active tab, left out; a.test is the other.
	if tab["kind"] != "tab" || tab["url"] != "https://a.test/" || tab["tab"] != float64(1) {
		t.Fatalf("tab row = %v", tab)
	}
	if closed["kind"] != "closed" || closed["url"] != "https://b.test/" || closed["title"] != "Mock b.test" || closed["detail"] != "b.test" {
		t.Fatalf("closed row = %v", closed)
	}

	r.shell(`{"t":"chooseSuggestion","row":{"kind":"closed","url":"https://b.test/"}}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "paint" && m["span"] == "tab.reopen" })
	r.onMain(func() {
		in1 := r.a.Tabs.AllIn(1)
		if len(in1) != 3 || in1[1].URL != "https://b.test/" || r.a.Tabs.ActiveIn(1) != in1[1].ID {
			t.Errorf("after choosing the closed row: %+v, active %v", in1, r.a.Tabs.ActiveIn(1))
		}
		if len(r.a.Commands.ListItems("recentlyClosed")) != 0 {
			t.Error("Recently Closed still lists what came back")
		}
	})
}
