//go:build js

// Package relaunch, in a web page, where the core runs as WebAssembly: there is no process
// to start again or copy, so each says on the timeline what it would have done.
package relaunch

import (
	"projectb/profiles"
	"projectb/timeline"
)

func After(executable string, args []string) {
	timeline.Errorf("relaunch: no process to start in a web page")
}

func Copy(args ...string) { timeline.Errorf("relaunch: no copy to start in a web page: %v", args) }

func Self() { timeline.Errorf("relaunch: no process to start again in a web page") }

type Copies struct {
	List    *profiles.Manager
	Profile string
}

func (c Copies) QuitOthers() []string { return []string{c.Profile} }

func (c Copies) StartAfter(executable string, profiles []string) {}
