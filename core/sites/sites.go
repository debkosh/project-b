// Package sites keeps each site's settings, by origin: the one place in Go a per-site
// setting lives (docs/daily-driver.md). Today they are what a page may use when it asks,
// its camera, its microphone and notifications: the engine sends each request and each read
// of one, the manager answers from the site's setting, and otherwise opens a prompt, one per
// tab at a time, for the tab's shell to draw. The settings are the profile's, kept in
// sites.json in its folder.
package sites

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/net/publicsuffix"

	"projectb/engine"
	"projectb/timeline"
)

// The kinds of setting the manager knows. A kind a page asks for is named as the
// Permissions API names it.
const (
	Camera         = "camera"
	Microphone     = "microphone"
	Notifications  = "notifications"
	ClipboardWrite = "clipboard-write"
	// Our own, which no page asks for (docs/site-features.md).
	Ads   = "ads"
	Links = "links"
	// Sparks is whether the person's sparks run on the site's pages, all of them or none.
	Sparks = "sparks"
)

// Kind is one kind of per-site setting, in our terms.
type Kind struct {
	Name string
	// Title is its name in settings.
	Title string
	// Requested kinds are ones a page asks for, which reach the manager from the engine.
	Requested bool
	// Askable kinds get a prompt while the site has no setting; the rest always have
	// Default.
	Askable bool
	Default engine.PermissionStatus
	// Gesture: a prompt only from a press or a key, so a page can't put one up on its own.
	Gesture bool
	// Once offers Allow this time.
	Once bool
	// Choices are the values one of our own kinds takes, its default first.
	Choices []string
}

// The values of our own kinds.
const (
	On  = "on"
	Off = "off"
)

// AdsDefault is whether the ad blocker is on where a site has no setting: on, the owner
// decided.
const AdsDefault = On

// Kinds is every kind the manager knows, in the order settings lists them. A requested
// kind it doesn't know is denied without asking; adding one here is all Go needs to offer
// it (the engine names a requested one in its adapter).
var Kinds = []Kind{
	{Name: Camera, Title: "Camera", Requested: true, Askable: true, Once: true},
	{Name: Microphone, Title: "Microphone", Requested: true, Askable: true, Once: true},
	{Name: Notifications, Title: "Notifications", Requested: true, Askable: true, Gesture: true},
	// As every browser grants it: Blink holds it to a focused page.
	{Name: ClipboardWrite, Title: "Clipboard", Requested: true, Default: engine.PermissionGranted},
	{Name: Ads, Title: "Ad blocker", Choices: adsChoices()},
	{Name: Links, Title: "Links to other sites", Choices: []string{Allow, Block}},
	{Name: Sparks, Title: "Sparks", Choices: []string{On, Off}},
}

func adsChoices() []string {
	if AdsDefault == On {
		return []string{On, Off}
	}
	return []string{Off, On}
}

// kindOf is the kind named name, and whether the manager knows it.
func kindOf(name string) (Kind, bool) {
	i := slices.IndexFunc(Kinds, func(k Kind) bool { return k.Name == name })
	if i < 0 {
		return Kind{}, false
	}
	return Kinds[i], true
}

// Default is a requested kind's answer while nothing else decides it: for an askable kind,
// asking; for one the manager doesn't know, denied.
func Default(kind string) engine.PermissionStatus {
	k, ok := kindOf(kind)
	if !ok || !k.Requested {
		return engine.PermissionDenied
	}
	return k.Default
}

// A site's setting of a kind, and the shell's choices on a prompt.
const (
	Allow   = "allow"
	Block   = "block"
	Once    = "once"
	Dismiss = "dismiss"
)

// Prompt is a request waiting for the person, as a shell is sent it.
type Prompt struct {
	ID     int          `json:"id"`
	Tab    engine.TabID `json:"tab"`
	Origin string       `json:"origin"`
	Kinds  []string     `json:"kinds"`
	// Once says the prompt may offer Allow this time: every kind it asks for offers it.
	Once bool `json:"once"`

	req engine.PermissionRequest
	// answers are the statuses of the request's kinds decided before the prompt, by kind.
	answers map[string]engine.PermissionStatus
	// url is the tab's address without its fragment when the prompt opened: a navigation
	// away from it drops the prompt.
	url string
}

// Site is one origin's settings, as settings lists them, in the kinds' order.
type Site struct {
	Origin   string    `json:"origin"`
	Settings []Setting `json:"settings"`
}

// Setting is one kind's setting for a site: Allow or Block.
type Setting struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Value string `json:"value"`
	// Choices are one of our own kinds' values; none for one a page asks for.
	Choices []string `json:"choices,omitempty"`
}

// Tabs is what the manager reads of the tabs.
type Tabs interface {
	URLOf(tab engine.TabID) (string, bool)
}

// Manager keeps one profile's decisions and the open prompts.
type Manager struct {
	path   string
	answer func(id engine.PermissionID, statuses []engine.PermissionStatus)
	tabs   Tabs
	// sites is origin → kind → Allow or Block.
	sites map[string]map[string]string
	// open is each tab's prompt, and waiting the requests behind it, in order.
	open    map[engine.TabID]*Prompt
	waiting map[engine.TabID][]engine.PermissionRequest
	// once is what each tab was allowed this time: its origin, and the kinds.
	once      map[engine.TabID]onceGrant
	last      int
	onPrompts []func()
	onSites   []func()
}

type onceGrant struct {
	origin string
	kinds  map[string]bool
}

// New reads the decisions kept in the profile folder dir. answer is how a request's
// statuses go back to the engine.
func New(dir string, t Tabs, answer func(engine.PermissionID, []engine.PermissionStatus)) *Manager {
	m := &Manager{
		path:    filepath.Join(dir, "sites.json"),
		answer:  answer,
		tabs:    t,
		sites:   map[string]map[string]string{},
		open:    map[engine.TabID]*Prompt{},
		waiting: map[engine.TabID][]engine.PermissionRequest{},
		once:    map[engine.TabID]onceGrant{},
	}
	b, err := os.ReadFile(m.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		timeline.Errorf("permissions: %v", err)
	default:
		if err := json.Unmarshal(b, &m.sites); err != nil {
			timeline.Errorf("permissions: %s: %v", m.path, err)
			m.sites = map[string]map[string]string{}
		}
		// A kind no longer known goes, as each spark's own switch did when one switch for
		// all of them took its place.
		for origin, kinds := range m.sites {
			for k := range kinds {
				if _, ok := kindOf(k); !ok {
					delete(kinds, k)
				}
			}
			if len(kinds) == 0 {
				delete(m.sites, origin)
			}
		}
	}
	return m
}

// OnPromptsChanged calls f whenever a prompt opens or goes.
func (m *Manager) OnPromptsChanged(f func()) { m.onPrompts = append(m.onPrompts, f) }

// OnSitesChanged calls f whenever a decision is kept or removed.
func (m *Manager) OnSitesChanged(f func()) { m.onSites = append(m.onSites, f) }

// Status is a kind's state for origin in tab's page, 0 for a page in no tab, such as a
// worker's: what a page reads, and what answers a request with no prompt.
func (m *Manager) Status(tab engine.TabID, origin, kind string) engine.PermissionStatus {
	if k, ok := kindOf(kind); !ok || !k.Requested || !k.Askable {
		return Default(kind)
	}
	switch m.sites[origin][kind] {
	case Allow:
		return engine.PermissionGranted
	case Block:
		return engine.PermissionDenied
	}
	if g, ok := m.once[tab]; ok && tab != 0 && g.origin == origin && g.kinds[kind] {
		return engine.PermissionGranted
	}
	return engine.PermissionAsk
}

// Request takes a page's request: answered at once when every kind is decided, else a
// prompt for the rest, or a wait behind the tab's prompt.
func (m *Manager) Request(r engine.PermissionRequest) {
	answers := map[string]engine.PermissionStatus{}
	var ask []string
	for _, k := range r.Kinds {
		s := m.Status(r.Tab, r.Origin, k)
		// A page in no tab has nowhere to show a prompt, and some kinds need a press.
		if s == engine.PermissionAsk && (r.Tab == 0 || (gesture(k) && !r.Gesture)) {
			answers[k] = engine.PermissionAsk
			continue
		}
		if s == engine.PermissionAsk {
			if !slices.Contains(ask, k) {
				ask = append(ask, k)
			}
			continue
		}
		answers[k] = s
	}
	if len(ask) == 0 {
		m.reply(r, answers)
		return
	}
	if m.open[r.Tab] != nil {
		m.waiting[r.Tab] = append(m.waiting[r.Tab], r)
		return
	}
	u, _ := m.tabs.URLOf(r.Tab)
	m.last++
	p := &Prompt{ID: m.last, Tab: r.Tab, Origin: r.Origin, Kinds: ask, req: r, answers: answers, url: page(u)}
	p.Once = true
	for _, k := range ask {
		kind, _ := kindOf(k)
		p.Once = p.Once && kind.Once
	}
	m.open[r.Tab] = p
	m.changed()
}

// Answer is the person's choice on prompt id: Allow, Once, Block or Dismiss. An answer to a
// prompt no longer open is ignored.
func (m *Manager) Answer(id int, choice string) {
	p := m.byID(id)
	if p == nil {
		return
	}
	var s engine.PermissionStatus
	switch choice {
	case Allow:
		s = engine.PermissionGranted
		m.keep(p.Origin, p.Kinds, Allow)
	case Once:
		if !p.Once {
			return
		}
		s = engine.PermissionGranted
		g := m.once[p.Tab]
		if g.origin != p.Origin {
			g = onceGrant{origin: p.Origin, kinds: map[string]bool{}}
		}
		for _, k := range p.Kinds {
			g.kinds[k] = true
		}
		m.once[p.Tab] = g
	case Block:
		s = engine.PermissionDenied
		m.keep(p.Origin, p.Kinds, Block)
	case Dismiss:
		s = engine.PermissionAsk
	default:
		return
	}
	m.close(p, s)
}

// Prompts is the open prompts, in the order they opened.
func (m *Manager) Prompts() []Prompt {
	list := []Prompt{}
	for _, p := range m.open {
		list = append(list, *p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// Sites is every origin with a decision, by origin.
func (m *Manager) Sites() []Site {
	list := []Site{}
	for o, values := range m.sites {
		site := Site{Origin: o, Settings: []Setting{}}
		for _, k := range Kinds {
			if v, ok := values[k.Name]; ok {
				site.Settings = append(site.Settings, Setting{Kind: k.Name, Title: k.Title, Value: v, Choices: k.Choices})
			}
		}
		list = append(list, site)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Origin < list[j].Origin })
	return list
}

// Set keeps value as origin's setting of kind, or forgets it for "": Allow or Block for a
// kind that can be asked about, one of its choices for one of ours, where its default is
// forgetting. The rest have their default.
func (m *Manager) Set(origin, kind, value string) {
	k, ok := kindOf(kind)
	if !ok || Origin(origin) != origin || origin == "" {
		return
	}
	if len(k.Choices) > 0 {
		switch {
		case value == "" || value == k.Choices[0]:
			m.Forget(origin, kind)
		case slices.Contains(k.Choices, value):
			m.keep(origin, []string{kind}, value)
		}
		return
	}
	if !k.Askable {
		return
	}
	switch value {
	case "":
		m.Forget(origin, kind)
	case Allow, Block:
		m.keep(origin, []string{kind}, value)
	}
}

// Value is origin's value of one of our own kinds: what it keeps, or the kind's default.
func (m *Manager) Value(origin, kind string) string {
	k, ok := kindOf(kind)
	if !ok || len(k.Choices) == 0 {
		return ""
	}
	// A value no longer among the kind's choices, as New tab of Links, reads as its default.
	if v, ok := m.sites[origin][kind]; ok && slices.Contains(k.Choices, v) {
		return v
	}
	return k.Choices[0]
}

// SettingsOf is origin's value of each kind settings can set, in order, "" where it has
// none.
func (m *Manager) SettingsOf(origin string) []Setting {
	list := Settable()
	for i := range list {
		if len(list[i].Choices) > 0 {
			list[i].Value = m.Value(origin, list[i].Kind)
		} else {
			list[i].Value = m.sites[origin][list[i].Kind]
		}
	}
	return list
}

// Settable is the kinds settings can set for a site, in order: each one's name and title.
func Settable() []Setting {
	list := []Setting{}
	for _, k := range Kinds {
		if k.Askable || len(k.Choices) > 0 {
			list = append(list, Setting{Kind: k.Name, Title: k.Title, Choices: k.Choices})
		}
	}
	return list
}

// Forget removes the decision for origin's kind, or every one of origin's with kind "".
// The site asks again next time.
func (m *Manager) Forget(origin, kind string) {
	kinds := m.sites[origin]
	if kinds == nil {
		return
	}
	if kind == "" {
		delete(m.sites, origin)
	} else {
		delete(kinds, kind)
		if len(kinds) == 0 {
			delete(m.sites, origin)
		}
	}
	m.save()
}

// TabChanged drops the tab's prompt when it left the page that asked, and what it was
// allowed this time when it left the origin.
func (m *Manager) TabChanged(tab engine.TabID, u string) {
	if g, ok := m.once[tab]; ok && Origin(u) != g.origin {
		delete(m.once, tab)
	}
	if p := m.open[tab]; p != nil && page(u) != p.url {
		m.dropTab(tab)
	}
}

// TabClosed answers the tab's prompts as dismissed and forgets the tab.
func (m *Manager) TabClosed(tab engine.TabID) {
	delete(m.once, tab)
	m.dropTab(tab)
}

func (m *Manager) dropTab(tab engine.TabID) {
	waiting := m.waiting[tab]
	delete(m.waiting, tab)
	for _, r := range waiting {
		m.reply(r, dismissed(r))
	}
	if p := m.open[tab]; p != nil {
		m.close(p, engine.PermissionAsk)
	}
}

func dismissed(r engine.PermissionRequest) map[string]engine.PermissionStatus {
	a := map[string]engine.PermissionStatus{}
	for _, k := range r.Kinds {
		a[k] = engine.PermissionAsk
	}
	return a
}

// close answers p's kinds with s, and takes the next request waiting in its tab.
func (m *Manager) close(p *Prompt, s engine.PermissionStatus) {
	delete(m.open, p.Tab)
	for _, k := range p.Kinds {
		p.answers[k] = s
	}
	m.reply(p.req, p.answers)
	m.changed()
	if next := m.waiting[p.Tab]; len(next) > 0 {
		m.waiting[p.Tab] = next[1:]
		if len(next) == 1 {
			delete(m.waiting, p.Tab)
		}
		m.Request(next[0])
	}
}

func (m *Manager) reply(r engine.PermissionRequest, answers map[string]engine.PermissionStatus) {
	statuses := make([]engine.PermissionStatus, len(r.Kinds))
	for i, k := range r.Kinds {
		statuses[i] = answers[k]
	}
	m.answer(r.ID, statuses)
}

func (m *Manager) byID(id int) *Prompt {
	for _, p := range m.open {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (m *Manager) keep(origin string, kinds []string, decision string) {
	if m.sites[origin] == nil {
		m.sites[origin] = map[string]string{}
	}
	for _, k := range kinds {
		m.sites[origin][k] = decision
	}
	m.save()
}

func (m *Manager) save() {
	b, err := json.MarshalIndent(m.sites, "", "  ")
	if err == nil {
		// A new file renamed over the old, so no crash leaves half of one.
		tmp := m.path + ".tmp"
		if err = os.WriteFile(tmp, append(bytes.TrimSpace(b), '\n'), 0o600); err == nil {
			err = os.Rename(tmp, m.path)
		}
	}
	if err != nil {
		timeline.Errorf("permissions: %v", err)
	}
	for _, f := range m.onSites {
		f()
	}
}

func (m *Manager) changed() {
	for _, f := range m.onPrompts {
		f()
	}
}

// page is an address without its fragment: a change of fragment stays in the document.
func page(u string) string {
	if i := strings.IndexByte(u, '#'); i >= 0 {
		return u[:i]
	}
	return u
}

// Origin is an address's origin as the engine writes one: scheme, host, and a port that
// isn't the scheme's own.
func Origin(u string) string {
	p, err := url.Parse(u)
	if err != nil || p.Host == "" {
		return ""
	}
	host := strings.ToLower(p.Host)
	if (p.Scheme == "https" && p.Port() == "443") || (p.Scheme == "http" && p.Port() == "80") {
		host = strings.TrimSuffix(host, ":"+p.Port())
	}
	o := p.Scheme + "://" + host
	return o
}

func gesture(kind string) bool {
	k, _ := kindOf(kind)
	return k.Gesture
}

// SameSite says whether two addresses are on one site: the same registrable domain, by the
// public suffix list (a.example.co.uk and b.example.co.uk are; example.co.uk and
// other.co.uk aren't), over http or https alike. The blank page, or an address with no
// host, is no site's.
func SameSite(a, b string) bool {
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	if errA != nil || errB != nil || ua.Hostname() == "" || ub.Hostname() == "" {
		return false
	}
	return SiteOf(ua.Hostname()) == SiteOf(ub.Hostname())
}

// SiteOf is a host's site, its registrable domain, as the Public Suffix List has it.
func SiteOf(host string) string {
	host = strings.ToLower(host)
	if d, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		return d
	}
	// An address, localhost, or a suffix itself is its own site.
	return host
}
