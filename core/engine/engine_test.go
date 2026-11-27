package engine

import "testing"

func TestTheFirstRegionThatHoldsAPointDecides(t *testing.T) {
	l := WindowLayout{Regions: []Region{
		{Rect{X: 100, Y: 0, W: 50, H: 40}, AreaDrag},
		{Rect{X: 0, Y: 0, W: 800, H: 40}, AreaShell},
	}}
	for _, c := range []struct {
		x, y float64
		want Area
	}{{120, 10, AreaDrag}, {10, 10, AreaShell}, {150, 10, AreaShell}, {10, 40, AreaPage}} {
		if got := l.At(c.x, c.y); got != c.want {
			t.Errorf("At(%v, %v) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestAShellsPageNamesItsShell(t *testing.T) {
	if p := ShellPage("pbshell"); p != "shell/pbshell" || p.Shell() != "pbshell" {
		t.Errorf("ShellPage(standard) = %q, its shell %q", p, p.Shell())
	}
	if PagePicker.Shell() != "" {
		t.Errorf("the picker is shell %q", PagePicker.Shell())
	}
}

// An address's space, as Chromium's local network access sorts them, and which requests
// that keeps from where.
func TestAddressSpaces(t *testing.T) {
	for address, want := range map[string]AddressSpace{
		"127.0.0.1": SpaceLoopback, "::1": SpaceLoopback, "[::1]": SpaceLoopback, "::ffff:127.0.0.1": SpaceLoopback,
		"10.1.2.3": SpaceLocal, "172.16.0.9": SpaceLocal, "192.168.1.1": SpaceLocal, "169.254.169.254": SpaceLocal,
		"fd00::1": SpaceLocal, "fe80::1": SpaceLocal, "100.64.0.1": SpaceLocal, "0.0.0.0": SpaceLocal, "::ffff:192.168.0.2": SpaceLocal,
		"203.0.113.7": SpacePublic, "2001:db8::1": SpacePublic, "172.32.0.1": SpacePublic, "": SpacePublic, "cdn.test": SpacePublic,
	} {
		if got := SpaceOf(address); got != want {
			t.Errorf("%q is %d, want %d", address, got, want)
		}
	}
	for _, c := range []struct {
		to, from AddressSpace
		refused  bool
	}{
		{SpaceLoopback, SpacePublic, true}, {SpaceLocal, SpacePublic, true}, {SpacePublic, SpacePublic, false},
		{SpaceLoopback, SpaceLocal, false}, {SpaceLocal, SpaceLoopback, false}, {SpacePublic, SpaceLocal, false},
		{SpaceLoopback, SpaceAny, false}, {SpaceLocal, SpaceAny, false},
	} {
		if got := MorePrivate(c.to, c.from); got != c.refused {
			t.Errorf("to %d from %d: %v", c.to, c.from, got)
		}
	}
}
