package tabs

import (
	"slices"
	"testing"

	"projectb/engine"
)

// withPins is a manager with windows 1 and 2, the first in front, on a profile with pins
// p1 and p2, in that order.
func withPins() *Manager {
	m := New(&fakeEngine{}, &list{work, ""}, &pinList{"p1", "p2"})
	m.AddWindow(2)
	m.AddWindow(1)
	return m
}

func order(m *Manager, w engine.WindowID) []engine.TabID { return ids(m.AllIn(w)) }

// A tab bound to a pin carries it and goes among its window's pinned tabs, first, in the
// pins' order, out of its group; unbound, it is first after them.
func TestPinnedTabsComeFirstInThePinsOrder(t *testing.T) {
	m := withPins()
	a, b, c, d := m.Create(1), m.Create(1), m.Create(1), m.Create(1)
	g := m.CreateGroup([]engine.TabID{c, d}, "", "")
	lists := 0
	m.OnListChanged(func() { lists++ })
	m.Bind(c, "p2")
	if got := order(m, 1); !slices.Equal(got, []engine.TabID{c, a, b, d}) || m.PinIn(1, "p2") != c || lists != 1 {
		t.Fatalf("after pinning c: %v, lists %d", got, lists)
	}
	// The group keeps d, its last tab, until d goes too.
	if x, _ := m.Get(c); x.Pin != "p2" || x.Group != "" || len(m.GroupsIn(1)) != 1 {
		t.Fatalf("c is %+v, groups %+v (was in %s)", x, m.GroupsIn(1), g)
	}
	m.Bind(d, "p1")
	if got := order(m, 1); !slices.Equal(got, []engine.TabID{d, c, a, b}) || len(m.GroupsIn(1)) != 0 {
		t.Fatalf("after pinning d: %v, groups %+v", got, m.GroupsIn(1))
	}
	m.Unbind("p1")
	if got := order(m, 1); !slices.Equal(got, []engine.TabID{c, d, a, b}) || m.PinIn(1, "p1") != 0 {
		t.Fatalf("after unpinning d: %v", got)
	}
	// Closing a pinned tab closes it; the pin is the caller's to keep.
	m.Close(c)
	if got := order(m, 1); !slices.Equal(got, []engine.TabID{d, a, b}) || m.PinIn(1, "p2") != 0 {
		t.Fatalf("after closing c: %v", got)
	}
}

// OpenPin opens a tab for a pin in its place, in the pin's workspace at its address, and
// makes it active.
func TestOpenPinOpensATabInItsPlace(t *testing.T) {
	m := withPins()
	a, b := m.Create(1), m.Create(1)
	m.Bind(a, "p2")
	id := m.OpenPin(1, "p1", "", "https://p1.test/")
	x, _ := m.Get(id)
	if got := order(m, 1); !slices.Equal(got, []engine.TabID{id, a, b}) || m.ActiveID() != id || m.PinIn(1, "p1") != id {
		t.Fatalf("order %v, active %v", got, m.ActiveID())
	}
	if x.Pin != "p1" || x.Workspace != "" || x.URL != "https://p1.test/" {
		t.Fatalf("the pin's tab is %+v", x)
	}
}

// A pinned tab moved into another window stays bound, among the pinned tabs there; into a
// window with a tab for its pin it shows that tab and stays. Another tab moved in goes after
// the pinned ones.
func TestMovingTabsKeepsThePinnedFirst(t *testing.T) {
	m := withPins()
	a, b := m.Create(1), m.Create(1)
	c, d := m.Create(2), m.Create(2)
	m.Bind(a, "p1")
	m.Bind(c, "p2")
	m.Move(a, 2, 2)
	if got := order(m, 2); !slices.Equal(got, []engine.TabID{a, c, d}) || m.PinIn(2, "p1") != a || m.ActiveIn(2) != a {
		t.Fatalf("window 2 after a moved in: %v", got)
	}
	m.Move(b, 2, 0)
	if got := order(m, 2); !slices.Equal(got, []engine.TabID{a, c, b, d}) {
		t.Fatalf("window 2 after b moved in: %v", got)
	}
	e := m.Create(1)
	f := m.OpenPin(1, "p1", "", "https://p1.test/")
	m.Activate(d)
	m.Move(f, 2, 0)
	if got := order(m, 1); !slices.Equal(got, []engine.TabID{f, e}) || m.ActiveIn(2) != a {
		t.Fatalf("a pin's second tab moved: window 1 %v, window 2 shows %v", got, m.ActiveIn(2))
	}
}

// Reordering keeps an unpinned tab after the pinned ones and leaves a pinned tab where it
// is; a tab beside a pinned one goes after them; a pinned tab is in no group.
func TestReorderAndBesideKeepThePinsOrder(t *testing.T) {
	m := withPins()
	a, b, c, d := m.Create(1), m.Create(1), m.Create(1), m.Create(1)
	m.Bind(a, "p1")
	m.Bind(b, "p2")
	m.Reorder(d, 0)
	m.Reorder(a, 3)
	if got := order(m, 1); !slices.Equal(got, []engine.TabID{a, b, d, c}) {
		t.Fatalf("after the reorders: %v", got)
	}
	m.Activate(a)
	s := m.OpenPage(1, "settings")
	if got := order(m, 1); !slices.Equal(got, []engine.TabID{a, b, s, d, c}) {
		t.Fatalf("settings beside a pinned tab: %v", got)
	}
	if m.CreateGroup([]engine.TabID{a, c}, "", "") != "" {
		t.Fatal("a group of a pinned tab")
	}
	g := m.CreateGroup([]engine.TabID{c, b}, "", "")
	m.AddToGroup(a, g)
	if got := groupTabs(m, 1); len(got) != 1 || !slices.Equal(got[0], []engine.TabID{c}) {
		t.Fatalf("groups %v", got)
	}
}
