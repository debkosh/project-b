// Package search is the search engines a typed text can go to: a short built-in list, one
// of which the profile's pref names (docs/daily-driver.md).
package search

import (
	"net/url"
	"strings"
)

// Engine is one search engine: its id, which the pref keeps and never changes, its name,
// and its address, the words going where %s is.
type Engine struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"-"`
}

// Default is the engine a profile has until it chooses one.
const Default = "duckduckgo"

var engines = []Engine{
	{ID: "duckduckgo", Name: "DuckDuckGo", URL: "https://duckduckgo.com/?q=%s"},
	{ID: "google", Name: "Google", URL: "https://www.google.com/search?q=%s"},
	{ID: "bing", Name: "Bing", URL: "https://www.bing.com/search?q=%s"},
	{ID: "brave", Name: "Brave Search", URL: "https://search.brave.com/search?q=%s"},
	{ID: "ecosia", Name: "Ecosia", URL: "https://www.ecosia.org/search?q=%s"},
}

// All is the list, in the order a settings page shows it.
func All() []Engine { return append([]Engine(nil), engines...) }

// Get is the engine with id, and whether there is one.
func Get(id string) (Engine, bool) {
	for _, e := range engines {
		if e.ID == id {
			return e, true
		}
	}
	return Engine{}, false
}

// For is the address of a search for words with e.
func (e Engine) For(words string) string {
	return strings.Replace(e.URL, "%s", url.QueryEscape(strings.TrimSpace(words)), 1)
}
