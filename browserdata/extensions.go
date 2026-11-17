package browserdata

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Extension is an extension the profile's person installed: its id, name and version, and
// why it can't be installed again from the Chrome Web Store by its id, or "" when it can.
type Extension struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Skip    Skip   `json:"skip,omitempty"`
}

// Skip is why an extension can't be brought over.
type Skip string

const (
	// SkipUnpacked is one loaded from a folder, which no store has.
	SkipUnpacked Skip = "unpacked"
	// SkipNotInStore is one from another store, such as Edge's, or from a file.
	SkipNotInStore Skip = "not-in-store"
	// SkipManifestV2 is one of Manifest V2, which Chrome no longer runs.
	SkipManifestV2 Skip = "manifest-v2"
)

// The Chrome Web Store's update address, which each of its packages' manifests names.
const webstoreUpdateURL = "https://clients2.google.com/service/update2/crx"

// Where Chromium says an extension came from (extensions/common/mojom/manifest.mojom).
const (
	locationInternal = 1 // a package, installed by its person or by the browser
	locationUnpacked = 4 // Load unpacked
)

// Extensions is what the profile's person installed, in the order of their names: those
// from a store or a file, and those loaded unpacked. What the browser or an administrator
// installed is left out, as are Firefox's and Safari's, which no Chromium browser runs.
func (p Profile) Extensions() ([]Extension, error) {
	k, ok := kindOf(p.Browser)
	if !ok || k.family != chromium {
		return nil, nil
	}
	return chromiumExtensions(p.Dir)
}

type chromiumExtension struct {
	Location           int             `json:"location"`
	FromWebstore       bool            `json:"from_webstore"`
	InstalledByDefault bool            `json:"was_installed_by_default"`
	InstalledByOEM     bool            `json:"was_installed_by_oem"`
	Path               string          `json:"path"`
	Manifest           json.RawMessage `json:"manifest"`
}

type chromiumManifest struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ManifestVersion int    `json:"manifest_version"`
	UpdateURL       string `json:"update_url"`
	DefaultLocale   string `json:"default_locale"`
}

// chromiumExtensions reads extensions.settings, which Chrome keeps in Secure Preferences
// and an older one in Preferences, and each extension's manifest: the copy kept there, or
// the one in its folder under Extensions, or, for an unpacked one, in the folder it was
// loaded from.
func chromiumExtensions(dir string) ([]Extension, error) {
	settings := map[string]chromiumExtension{}
	var firstErr error
	for _, name := range []string{"Preferences", "Secure Preferences"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if os.IsNotExist(err) {
			continue
		}
		var prefs struct {
			Extensions struct {
				Settings map[string]chromiumExtension `json:"settings"`
			} `json:"extensions"`
		}
		if err == nil {
			err = json.Unmarshal(b, &prefs)
		}
		if err != nil {
			firstErr = cmp.Or(firstErr, err)
			continue
		}
		for id, s := range prefs.Extensions.Settings {
			settings[id] = s
		}
	}
	var out []Extension
	for id, s := range settings {
		if (s.Location != locationInternal && s.Location != locationUnpacked) || s.InstalledByDefault || s.InstalledByOEM {
			continue
		}
		folder := s.Path
		if s.Location == locationInternal {
			folder = filepath.Join(dir, "Extensions", s.Path)
		}
		var m chromiumManifest
		if json.Unmarshal(s.Manifest, &m) != nil || m.Name == "" {
			b, err := os.ReadFile(filepath.Join(folder, "manifest.json"))
			switch {
			case err == nil && json.Unmarshal(b, &m) == nil:
			case s.Location == locationUnpacked:
				// Its folder is gone or unreadable: it is still listed, by the folder's name.
				m = chromiumManifest{Name: filepath.Base(s.Path)}
			default:
				continue
			}
		}
		e := Extension{ID: id, Name: localized(folder, m.DefaultLocale, m.Name), Version: m.Version}
		switch {
		case s.Location == locationUnpacked:
			e.Skip = SkipUnpacked
		case !s.FromWebstore && m.UpdateURL != webstoreUpdateURL:
			e.Skip = SkipNotInStore
		case m.ManifestVersion < 3:
			e.Skip = SkipManifestV2
		}
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b Extension) int {
		return strings.Compare(strings.ToLower(a.Name)+a.ID, strings.ToLower(b.Name)+b.ID)
	})
	return out, firstErr
}

// localized is a manifest's string, as __MSG_name__ in its default locale's messages, or
// English's, when it names one.
func localized(folder, locale, s string) string {
	key, ok := strings.CutPrefix(s, "__MSG_")
	if !ok {
		return s
	}
	key, ok = strings.CutSuffix(key, "__")
	if !ok {
		return s
	}
	for _, l := range []string{locale, "en", "en_US"} {
		if l == "" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(folder, "_locales", l, "messages.json"))
		if err != nil {
			continue
		}
		var messages map[string]struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(b, &messages) != nil {
			continue
		}
		// Message names are matched without regard to case.
		for name, m := range messages {
			if strings.EqualFold(name, key) && m.Message != "" {
				return m.Message
			}
		}
	}
	return s
}
