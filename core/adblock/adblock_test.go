package adblock

import (
	"strings"
	"testing"
)

func TestTheListIsHostsOnly(t *testing.T) {
	hosts := Hosts()
	if len(hosts) < 100 {
		t.Fatalf("only %d hosts", len(hosts))
	}
	seen := map[string]bool{}
	for _, h := range hosts {
		if seen[h] || strings.ContainsAny(h, " /:*") || !strings.Contains(h, ".") || strings.HasPrefix(h, ".") {
			t.Errorf("not a host, or twice: %q", h)
		}
		seen[h] = true
	}
	if !seen["doubleclick.net"] {
		t.Error("doubleclick.net isn't listed")
	}
}
