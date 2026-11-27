package blocking

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// The two blockers a group can be in; Both is the test group's, whose lists go by their
// ids: one ending -malware or -phishing to the safe blocker, the rest to the network one.
const (
	Network = "network"
	Safe    = "safe"
	Both    = "both"
)

// Builtin is the group the bundled hosts are loaded as, in the network blocker, until the
// ads group's lists are there.
const Builtin = "builtin"

// Catalogue is catalog.json: the groups, in the order Settings shows them, their lists, and
// Brave's lists left out, with why.
type Catalogue struct {
	Groups  []Group   `json:"groups"`
	Lists   []List    `json:"lists"`
	Ignored []Ignored `json:"ignored"`
}

// Group is one group of lists, compiled together: in the network blocker one engine, so an
// exception list unbreaks the lists beside it; in the safe blocker each list its own, so a
// warning names the list that said so.
type Group struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	About   string `json:"about"`
	Blocker string `json:"blocker"`
	// Shown is false for the test group, which no page lists.
	Shown bool `json:"shown"`
}

// List is one published list. On is its default; with Languages it is on by default only
// where one of the app's languages is among them.
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
}

// Source is where the workflow fetches a list from, and its format: "standard" or "hosts".
type Source struct {
	URL    string `json:"url"`
	Format string `json:"format"`
}

// Ignored is a list of Brave's catalogue we leave out, and why.
type Ignored struct {
	Brave  string `json:"brave"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

// ParseCatalogue reads a catalogue and checks it holds together: each id once, each list in
// a group there is, each group in one of the two blockers.
func ParseCatalogue(b []byte) (*Catalogue, error) {
	var c Catalogue
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("the catalogue: %v", err)
	}
	groups := map[string]bool{}
	for _, g := range c.Groups {
		if g.ID == "" || g.ID == Builtin || groups[g.ID] {
			return nil, fmt.Errorf("the catalogue: group %q", g.ID)
		}
		if g.Blocker != Network && g.Blocker != Safe && g.Blocker != Both {
			return nil, fmt.Errorf("the catalogue: group %s is in blocker %q", g.ID, g.Blocker)
		}
		groups[g.ID] = true
	}
	lists := map[string]bool{}
	for _, l := range c.Lists {
		if !fileID.MatchString(l.ID) || lists[l.ID] || l.ID == "resources" {
			return nil, fmt.Errorf("the catalogue: list %q", l.ID)
		}
		if !groups[l.Group] {
			return nil, fmt.Errorf("the catalogue: list %s is in group %q, which it hasn't", l.ID, l.Group)
		}
		lists[l.ID] = true
	}
	return &c, nil
}

// Group is the group with id.
func (c *Catalogue) Group(id string) (Group, bool) {
	i := slices.IndexFunc(c.Groups, func(g Group) bool { return g.ID == id })
	if i < 0 {
		return Group{}, false
	}
	return c.Groups[i], true
}

// List is the list with id.
func (c *Catalogue) List(id string) (List, bool) {
	i := slices.IndexFunc(c.Lists, func(l List) bool { return l.ID == id })
	if i < 0 {
		return List{}, false
	}
	return c.Lists[i], true
}

// BlockerOf is the blocker a list goes in: its group's, or in a group of Both, by its id.
func (c *Catalogue) BlockerOf(l List) string {
	g, _ := c.Group(l.Group)
	if g.Blocker != Both {
		return g.Blocker
	}
	if strings.HasSuffix(l.ID, "-malware") || strings.HasSuffix(l.ID, "-phishing") {
		return Safe
	}
	return Network
}

// KindOf is what a safe list warns of: "phishing" for the phishing group's and a list whose
// id ends -phishing, else "malware".
func (c *Catalogue) KindOf(l List) string {
	if l.Group == "phishing" || strings.HasSuffix(l.ID, "-phishing") {
		return "phishing"
	}
	return "malware"
}

// InGroup is a group's lists, in the catalogue's order.
func (c *Catalogue) InGroup(group string) []List {
	var out []List
	for _, l := range c.Lists {
		if l.Group == group {
			out = append(out, l)
		}
	}
	return out
}

// Default says whether a list is on before the person changes it: its On, or with
// languages, whether one of the app's is among them.
func Default(l List, languages []string) bool {
	if len(l.Languages) == 0 {
		return l.On
	}
	for _, want := range languages {
		for _, have := range l.Languages {
			if strings.EqualFold(base(want), base(have)) {
				return true
			}
		}
	}
	return false
}

// base is a language tag's language alone: "pt-BR" and "pt_BR.UTF-8" are "pt".
func base(tag string) string {
	tag, _, _ = strings.Cut(tag, ".")
	tag, _, _ = strings.Cut(tag, "_")
	tag, _, _ = strings.Cut(tag, "-")
	return strings.ToLower(tag)
}
