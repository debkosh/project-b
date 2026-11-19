package main

import "projectb/flags"

// contentSwitches are what the Mac adds to content's command line, in dir, the folder this
// copy owns. Go's runtime started when the binary loaded and handles SIGSEGV, SIGBUS and
// SIGFPE, passing on what isn't Go's. Content would reset them and, outside official builds,
// put its stack dumper in their place, so a nil dereference in Go would never reach Go:
// --disable-in-process-stack-traces is content's own switch for a runtime that owns these
// signals, as a sanitizer does. Then the flags saved in the profile, as switches
// (docs/flags.md).
func contentSwitches(dir string) []string {
	return append([]string{"--disable-in-process-stack-traces"}, flags.New(dir).Switches()...)
}
