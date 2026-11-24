package app_test

import (
	"testing"

	"projectb/links"
)

// Settings' Read more opens the help page it names in a tab beside Settings; a name with no
// page opens nothing.
func TestSettingsOpenHelp(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.openSettings()
	before := len(r.tabsNow(1))
	r.settings(`{"t":"openHelp","page":"no such page"}`)
	r.settings(`{"t":"openHelp","page":"keyring"}`)
	tabs := r.tabsNow(1)
	opened := 0
	for _, tab := range tabs {
		if tab.URL == links.Help("keyring") {
			opened++
		}
	}
	if opened != 1 || len(tabs) != before+1 {
		t.Fatalf("Read more opened %d tabs at %s, %d tabs from %d: %+v", opened, links.Help("keyring"), len(tabs), before, tabs)
	}
}
