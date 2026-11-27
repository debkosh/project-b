package app

import (
	"encoding/json"
	"maps"
	"path/filepath"
	"slices"

	"projectb/adblock"
	"projectb/blocking"
	"projectb/engine"
	"projectb/prefs"
	"projectb/settings"
	"projectb/tabs"
	"projectb/timeline"
)

// The block lists (docs/blocking.md): the lists manager loads the engines on disk before any
// tab, the engine gets the network blocker's table while Block ads is on and the sites where
// it is off, each tab's navigation asks the safe blocker while Warn before dangerous sites
// is on, and Settings shows and switches the lists.

// listsDir is the lists' folder: beside the profiles list, which every profile shares, or in
// the profile for a folder named outright (a phone's one profile, a driver's), which has no
// list.
func listsDir(cfg Config) string {
	if cfg.Profile != "" && cfg.Profiles != nil {
		return filepath.Join(cfg.Profiles.Root(), "lists")
	}
	return filepath.Join(cfg.ProfileDir, "lists")
}

// choicesOf is the lists manager's view of the prefs.
func choicesOf(p prefs.Prefs) blocking.Choices {
	return blocking.Choices{Network: p.BlockAds, Safe: p.WarnDangerous, Lists: p.BlockLists}
}

func sameChoices(a, b blocking.Choices) bool {
	return a.Network == b.Network && a.Safe == b.Safe && maps.Equal(a.Lists, b.Lists)
}

func (a *App) startBlocking(e engine.Engine, cfg Config) {
	s := cfg.Blocking
	if s.Dir == "" {
		s.Dir = listsDir(cfg)
	}
	s.Hosts, s.Post = adblock.Hosts(), e.Post
	m, err := blocking.New(s)
	if err != nil {
		timeline.Errorf("blocking: %v; no lists", err)
		s.Catalogue = &blocking.Catalogue{}
		m, _ = blocking.New(s)
	}
	a.Blocking = m
	now := choicesOf(a.Prefs.All())
	m.Load(now)

	// The engine has the network blocker while Block ads is on, from before any tab.
	hooks := func(on bool) {
		if on {
			e.SetBlocker(m.Hooks())
		} else {
			e.SetBlocker(nil)
		}
	}
	hooks(now.Network)
	off := a.adsOff()
	e.SetBlockingOff(off)
	a.Sites.OnSitesChanged(func() {
		if o := a.adsOff(); !slices.Equal(o, off) {
			off = o
			e.SetBlockingOff(o)
		}
	})
	send := func() { a.Settings.Send(map[string]any{"t": "blocking", "blocking": m.Shown(now)}) }
	a.Prefs.OnChanged(func(p prefs.Prefs) {
		c := choicesOf(p)
		if sameChoices(c, now) {
			return
		}
		if c.Network != now.Network {
			hooks(c.Network)
		}
		now = c
		m.SetChoices(c)
		send()
	})
	m.OnRulesChanged(e.BlockerChanged)
	m.OnStatusChanged(send)

	// Safe Browsing: a web tab's navigation, and each of its redirects, asks the safe
	// blocker.
	a.Tabs.SetDanger(func(u string) (tabs.Danger, bool) {
		if !now.Safe {
			return tabs.Danger{}, false
		}
		l, _, ok := m.Dangerous(u)
		if !ok {
			return tabs.Danger{}, false
		}
		timeline.Infof("blocking: %s refused, as %s names it", u, l.ID)
		return tabs.Danger{Kind: m.Catalogue().KindOf(l), List: l.Title}, true
	})

	a.Settings.AddPart(settings.Part{
		Snapshot: func(into map[string]any) { into["blocking"] = m.Shown(now) },
		Handle: func(_ engine.TabID, t string, msg []byte) bool {
			var c struct {
				ID string `json:"id"`
				On bool   `json:"on"`
			}
			switch t {
			case "setBlockList":
				json.Unmarshal(msg, &c)
				a.Prefs.SetBlockLists(m.WithList(now.Lists, c.ID, c.On))
			case "setBlockGroup":
				json.Unmarshal(msg, &c)
				a.Prefs.SetBlockLists(m.WithGroup(now.Lists, c.ID, c.On))
			default:
				return false
			}
			return true
		},
	})
}
