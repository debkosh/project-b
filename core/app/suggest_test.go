package app_test

import (
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

	// Words: the search first, with the profile's engine, then the open tab.
	r.shell(`{"t":"suggest","id":3,"text":"mail","kinds":["address","search","tab","history","action"]}`)
	items := r.waitFor(func(m map[string]any) bool { return m["t"] == "suggestions" && m["id"] == float64(3) })["items"].([]any)
	first, second := items[0].(map[string]any), items[1].(map[string]any)
	if first["kind"] != "search" || first["url"] != "https://duckduckgo.com/?q=mail" || second["kind"] != "tab" || second["tab"] != float64(1) {
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
