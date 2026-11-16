package mockengine

import "projectb/engine"

// Print records the tab whose page the app printed.
func (e *Engine) Print(tab engine.TabID) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.printed = append(e.printed, tab)
}

// Printed are the tabs whose pages the app printed, in order.
func (e *Engine) Printed() []engine.TabID {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.TabID(nil), e.printed...)
}

// printRequested is a page's window.print(): it prints if the app says so.
func (e *Engine) printRequested(tab engine.TabID) {
	if e.listener.PrintRequested(tab) {
		e.Print(tab)
	}
}
