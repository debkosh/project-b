//go:build linux && !android

package main

import (
	"os"
	"strings"
)

// languages are the session's, from its locale's variables in the order gettext reads
// them, which turn a regional block list on by default (docs/blocking.md).
func languages() []string {
	var out []string
	for _, name := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		for _, tag := range strings.Split(os.Getenv(name), ":") {
			if tag != "" && tag != "C" && tag != "POSIX" {
				out = append(out, tag)
			}
		}
	}
	return out
}
