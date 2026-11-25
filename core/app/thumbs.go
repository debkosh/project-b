package app

import (
	"time"

	"projectb/engine"
	"projectb/shell"
	"projectb/shells"
)

// thumbGap is the least time between two pictures a window asks for: one asked for sooner
// is dropped, not queued, so a burst of switches costs one copy.
const thumbGap = 500 * time.Millisecond

// thumbWidth is the widest picture asked for, in device pixels: a switcher card's 360 CSS
// pixels at a phone's density of 2 or more. The engine never makes one wider than the page.
const thumbWidth = 720

// pageThumbs asks the engine for pictures of the tabs' pages, which a phone shell's tab
// switcher draws (docs/shells.md, captureTab). Only while the profile's shell is a phone
// shell, so no desktop window ever pays for a copy: of the tab a window leaves as another
// becomes active there, while its page still shows, and of the active tab when the shell
// asks, as its switcher opens.
type pageThumbs struct {
	a *App
	e engine.Engine
	// last is when each window last asked for one.
	last map[engine.WindowID]time.Time
}

func newPageThumbs(a *App, e engine.Engine) *pageThumbs {
	t := &pageThumbs{a: a, e: e, last: map[engine.WindowID]time.Time{}}
	a.Tabs.OnLeaving(t.capture)
	return t
}

// capture asks for a picture of a tab's page. A tab with no page of its own yet shows the
// shell's new tab page, and one with a notice shows that: neither is a picture of a page.
func (t *pageThumbs) capture(id engine.TabID) {
	w := t.a.Tabs.WindowOf(id)
	tab, ok := t.a.Tabs.Get(id)
	if !ok || w == 0 || !shells.Phone(t.a.shells.InUse()) || tab.URL == "" || tab.Notice != nil {
		return
	}
	now := time.Now()
	if now.Sub(t.last[w]) < thumbGap {
		return
	}
	t.last[w] = now
	t.e.CaptureThumbnail(id, thumbWidth)
}

// shellPart is captureTab: a picture of the window's active tab.
func (t *pageThumbs) shellPart() shell.Part {
	return shell.Part{
		Snapshot: func(map[string]any) {},
		Handle: func(w engine.WindowID, kind string, _ []byte) bool {
			if kind != "captureTab" {
				return false
			}
			if id := t.a.Tabs.ActiveIn(w); id != 0 {
				t.capture(id)
			}
			return true
		},
	}
}

// Thumbnail is a new picture of a tab's page, which the tab's state points to.
func (a *App) Thumbnail(tab engine.TabID, revision int64) { a.Tabs.HandleThumbnail(tab, revision) }
