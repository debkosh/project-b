package app

import "projectb/engine"

// PrintRequested lets a page print itself, as its window.print() asks, while its tab is the
// one its window shows: a tab in the background waits for no panel the person can't see.
// What prints and the panel are the engine's, as for Print… (docs/daily-driver.md).
func (a *App) PrintRequested(tab engine.TabID) bool {
	w := a.Tabs.WindowOf(tab)
	return w != 0 && a.Tabs.ActiveIn(w) == tab
}
