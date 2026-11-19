package main

// macOS keeps no reason a process ended for the app to ask: a crash.log left behind is the
// sign of a crash, and its report says the rest.
func (e *nativeEngine) ExitReason(int) string { return "" }
