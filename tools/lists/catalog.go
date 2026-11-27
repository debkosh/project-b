package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

// The catalogue, core/blocking/catalog.json, as the Go side reads it too (docs/blocking.md,
// "The catalogue"). Only a commit changes it; this program reads it and never writes it.
type Catalog struct {
	Groups  []Group   `json:"groups"`
	Lists   []List    `json:"lists"`
	Ignored []Ignored `json:"ignored"`
}

type Group struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	About   string `json:"about"`
	Blocker string `json:"blocker"` // network, safe, or both (the test group)
	Shown   bool   `json:"shown"`
}

type List struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	About     string   `json:"about"`
	Group     string   `json:"group"`
	On        bool     `json:"on"`
	Languages []string `json:"languages,omitempty"`
	Sources   []Source `json:"sources"`
	Licence   string   `json:"licence"`
	Home      string   `json:"home"`
	Brave     string   `json:"brave,omitempty"`
	Trusted   bool     `json:"trusted,omitempty"` // may use the trusted-* scriptlets; read by the app only
}

type Source struct {
	URL    string `json:"url"`
	Format string `json:"format"` // standard or hosts
}

// A Brave list, or one source of one, or Brave's default for one, that we leave out on purpose,
// so tools/lists-vs-brave.py doesn't raise it again. Only that script reads it.
type Ignored struct {
	Brave          string `json:"brave"`
	Source         string `json:"source,omitempty"`
	DefaultEnabled *bool  `json:"default_enabled,omitempty"`
	Title          string `json:"title"`
	Reason         string `json:"reason"`
}

var listID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func loadCatalog(path string) (*Catalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var c Catalog
	if err := d.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

func (c *Catalog) validate() error {
	groups := map[string]bool{}
	for _, g := range c.Groups {
		if !listID.MatchString(g.ID) || groups[g.ID] {
			return fmt.Errorf("group %q: a bad or repeated id", g.ID)
		}
		switch g.Blocker {
		case "network", "safe", "both":
		default:
			return fmt.Errorf("group %s: blocker %q", g.ID, g.Blocker)
		}
		groups[g.ID] = true
	}
	ids := map[string]bool{}
	for _, l := range c.Lists {
		// "resources" names the resources' own entry in the manifest.
		if !listID.MatchString(l.ID) || ids[l.ID] || l.ID == "resources" {
			return fmt.Errorf("list %q: a bad or repeated id", l.ID)
		}
		ids[l.ID] = true
		if !groups[l.Group] {
			return fmt.Errorf("list %s: no group %q", l.ID, l.Group)
		}
		if l.Title == "" || l.Licence == "" || l.Home == "" || len(l.Sources) == 0 {
			return fmt.Errorf("list %s: needs a title, a licence, a home and a source", l.ID)
		}
		for _, s := range l.Sources {
			if s.Format != "standard" && s.Format != "hosts" {
				return fmt.Errorf("list %s: format %q", l.ID, s.Format)
			}
			if len(s.URL) < 9 || s.URL[:8] != "https://" {
				return fmt.Errorf("list %s: source %q isn't https", l.ID, s.URL)
			}
		}
	}
	return nil
}
