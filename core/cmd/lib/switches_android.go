package main

// contentSwitches are what Android adds to content's command line: none. Saved flags have
// never reached content on a phone, and content's own signal handling stays as it is.
func contentSwitches(dir string) []string { return nil }
