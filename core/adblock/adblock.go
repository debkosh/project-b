// Package adblock is the ad blocker's list: the hosts of ads and trackers, bundled with the
// app and updated with it (docs/site-features.md). The engine keeps a page's requests from
// them, and from any host under them, unless the blocker is off for the page's site.
package adblock

import (
	_ "embed"
	"strings"
)

//go:embed hosts.txt
var list string

// Hosts is the list, lower case, without its comments.
func Hosts() []string {
	var hosts []string
	for _, line := range strings.Split(list, "\n") {
		line, _, _ = strings.Cut(line, "#")
		if line = strings.ToLower(strings.TrimSpace(line)); line != "" {
			hosts = append(hosts, line)
		}
	}
	return hosts
}
