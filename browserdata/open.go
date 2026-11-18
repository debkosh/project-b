package browserdata

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoBrowser is what Open says of a folder that holds no browser's profiles.
var ErrNoBrowser = errors.New("browserdata: no browser's profiles in that folder")

// Open reads a folder the person chose as a browser's, for a fork this package doesn't
// know or one kept somewhere else: a Chromium browser's folder of profiles (Local State, or
// profile folders) or one of its profiles (Bookmarks or History), or a Firefox one's folder
// (profiles.ini) or one of its profiles (places.sqlite). A known browser's folder, or one
// of its profiles, is that browser; another is OtherChromium or OtherFirefox, named after
// its folder. home is the home folder Find looks in.
func Open(home, dir string) (Browser, error) {
	dir = filepath.Clean(dir)
	support := filepath.Join(home, "Library/Application Support")
	known := func(d string) (kind, bool) {
		for _, k := range kinds {
			if k.data != "" && strings.EqualFold(filepath.Join(support, k.data), d) {
				return k, true
			}
		}
		return kind{}, false
	}
	// The browser a folder of profiles is: known, or named after it ("User Data" is
	// Chromium's own name for it, so the folder above names it).
	browser := func(d string, f family) Browser {
		if k, ok := known(d); ok {
			return Browser{ID: k.id, Name: k.name}
		}
		name := filepath.Base(d)
		if name == "User Data" {
			name = filepath.Base(filepath.Dir(d))
		}
		if f == firefox {
			return Browser{ID: OtherFirefox, Name: name}
		}
		return Browser{ID: OtherChromium, Name: name}
	}
	switch {
	case exists(filepath.Join(dir, "profiles.ini")):
		b := browser(dir, firefox)
		b.Profiles = firefoxProfiles(b.ID, dir)
		return listed(b)
	case exists(filepath.Join(dir, "places.sqlite")):
		// A Firefox profile: in Profiles, under its browser's folder.
		b := browser(filepath.Dir(filepath.Dir(dir)), firefox)
		b.Profiles = []Profile{{Browser: b.ID, Dir: dir, Name: filepath.Base(dir)}}
		return listed(b)
	case exists(filepath.Join(dir, "Local State")) || !chromiumProfile(dir):
		b := browser(dir, chromium)
		b.Profiles = chromiumProfiles(b.ID, dir)
		return listed(b)
	default:
		// A Chromium profile: its browser's folder is the one above, unless the folder is
		// the only profile there is, as Opera's can be.
		parent := filepath.Dir(dir)
		if !exists(filepath.Join(parent, "Local State")) {
			b := browser(dir, chromium)
			b.Profiles = []Profile{{Browser: b.ID, Dir: dir}}
			return listed(b)
		}
		b := browser(parent, chromium)
		for _, p := range chromiumProfiles(b.ID, parent) {
			if p.Dir == dir {
				b.Profiles = []Profile{p}
			}
		}
		if b.Profiles == nil {
			b.Profiles = []Profile{{Browser: b.ID, Dir: dir, Name: filepath.Base(dir)}}
		}
		return listed(b)
	}
}

func listed(b Browser) (Browser, error) {
	if len(b.Profiles) == 0 {
		return Browser{}, ErrNoBrowser
	}
	if _, err := os.Stat(b.Profiles[0].Dir); err != nil {
		return Browser{}, err
	}
	return b, nil
}
