// Package blocking is the lists manager (docs/blocking.md): the catalogue of block lists,
// the person's choices among them, the downloads from the lists release, the files on disk,
// and which engines the blocking library has loaded. Two blockers: the network one, whose
// rules the engine asks of every request and frame (engine.SetBlocker), and the safe one,
// which Go asks of each tab's navigation (Safe Browsing).
package blocking

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"projectb/timeline"
)

// Setup is what the manager needs from the app.
type Setup struct {
	// Dir is the lists' folder, which profiles share.
	Dir string
	// Library is the blocking library; Plain when nil.
	Library Library
	// Catalogue is the lists there are; the embedded one when nil.
	Catalogue *Catalogue
	// Languages are the app's, which turn a regional list on by default.
	Languages []string
	// BaseURL is where the release's files are, Releases in the app; "" downloads nothing.
	BaseURL string
	// Key checks the manifest's signature; PublicKey when nil.
	Key ed25519.PublicKey
	// Client fetches the files; one with a timeout when nil.
	Client *http.Client
	// Every is how often the lists are checked while the app runs; an hour when 0.
	Every time.Duration
	// Hosts are the built-in hosts (core/adblock), the network rules until the ads group's
	// lists are there.
	Hosts []string
	// Post runs a function on the main thread.
	Post func(func())
}

// Choices are what the person chose: Block ads (the network blocker), Warn before dangerous
// sites (the safe one), and the lists they turned on or off, where that differs from the
// default.
type Choices struct {
	Network bool
	Safe    bool
	Lists   map[string]bool
}

// Manager is the lists manager. Its methods are the main thread's; its work, the downloads,
// the files and compiling, is its own goroutine's.
type Manager struct {
	s       Setup
	cat     *Catalogue
	network Blocker

	// The main thread's: the choices as the worker last heard them, the status it last
	// sent, and who hears.
	choices  Choices
	status   Status
	onRules  []func()
	onStatus []func()

	// safe is the safe blocker's lists that are loaded, for Dangerous on any thread.
	safe atomic.Pointer[[]safeUnit]

	// What the main thread hands the worker, under mu: new choices, and whether to fetch.
	mu      sync.Mutex
	pending *Choices
	fetch   bool
	started bool
	wake    chan struct{}
	// ctx ends with Stop, and with it any download under way.
	ctx    context.Context
	cancel func()
	done   chan struct{}

	w          worker
	loadedOnce bool
}

// safeUnit is one list of the safe blocker, loaded in a blocker of its own so a match
// names it.
type safeUnit struct {
	list  List
	group Group
	b     Blocker
}

// New makes the manager. Load then loads what is on disk.
func New(s Setup) (*Manager, error) {
	cat := s.Catalogue
	if cat == nil {
		var err error
		if cat, err = Embedded(); err != nil {
			return nil, err
		}
	}
	if s.Library == nil {
		s.Library = Plain{}
	}
	if s.Key == nil {
		s.Key = PublicKey
	}
	if s.Client == nil {
		s.Client = &http.Client{Timeout: 2 * time.Minute}
	}
	if s.Every == 0 {
		s.Every = time.Hour
	}
	m := &Manager{s: s, cat: cat, wake: make(chan struct{}, 1), done: make(chan struct{})}
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.network = s.Library.New()
	m.safe.Store(&[]safeUnit{})
	m.w = worker{m: m, texts: map[string]string{}, resources: map[string]string{}, loaded: map[string]string{}, safe: map[string]Blocker{}}
	return m, nil
}

// Catalogue is the lists there are.
func (m *Manager) Catalogue() *Catalogue { return m.cat }

// Network is the network blocker, whose rules the engine asks.
func (m *Manager) Network() Blocker { return m.network }

// Hooks is the engine's table for the network blocker.
func (m *Manager) Hooks() unsafe.Pointer { return m.network.Hooks() }

// OnRulesChanged subscribes to the network blocker's rules changing, on the main thread.
func (m *Manager) OnRulesChanged(f func()) { m.onRules = append(m.onRules, f) }

// OnStatusChanged subscribes to what Settings shows of the lists changing.
func (m *Manager) OnStatusChanged(f func()) { m.onStatus = append(m.onStatus, f) }

// Load loads the engines on disk for the choices, before the first window's page, and
// starts the worker. An engine that is missing or doesn't load, being another library
// version's, is compiled after the start-up wait; until then its group's rules are off,
// and the built-in hosts stand in for the ads group.
func (m *Manager) Load(c Choices) {
	m.choices = c
	m.w.choices = c
	m.w.scan()
	m.w.giveResources()
	m.w.reconcile(false, false)
	m.status = m.w.statusNow()
	m.loadedOnce = true
	go m.w.run()
}

// Start begins the downloads, now and then every hour; app.Start asks for it through
// later.Run, after the start-up wait.
func (m *Manager) Start() {
	m.mu.Lock()
	m.started, m.fetch = true, true
	m.mu.Unlock()
	m.poke()
}

// SetChoices gives the manager the person's choices as they changed. What they turned on
// is fetched and compiled at once, wait or no wait: it is something they did.
func (m *Manager) SetChoices(c Choices) {
	before := m.choices
	m.choices = c
	m.mu.Lock()
	m.pending = &c
	if m.turnedOn(before, c) {
		m.fetch = true
	}
	m.mu.Unlock()
	m.poke()
}

// turnedOn says whether c wants a list before didn't.
func (m *Manager) turnedOn(before, c Choices) bool {
	was := m.wanted(before)
	for id := range m.wanted(c) {
		if !was[id] {
			return true
		}
	}
	return false
}

func (m *Manager) poke() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// Stop ends the worker: a download under way ends, a compile is let finish. A manager
// never loaded has none.
func (m *Manager) Stop() {
	m.cancel()
	if m.loadedOnce {
		<-m.done
	}
}

// On says whether a list is on under c.
func (m *Manager) On(c Choices, l List) bool {
	if v, ok := c.Lists[l.ID]; ok {
		return v
	}
	return Default(l, m.s.Languages)
}

// wanted is the lists c wants: those on, in a group of a blocker that is on.
func (m *Manager) wanted(c Choices) map[string]bool {
	out := map[string]bool{}
	for _, l := range m.cat.Lists {
		b := m.cat.BlockerOf(l)
		if (b == Network && c.Network || b == Safe && c.Safe) && m.On(c, l) {
			out[l.ID] = true
		}
	}
	return out
}

// WithList is lists, the person's changes, with list id turned on or off: kept only where
// that isn't its default.
func (m *Manager) WithList(lists map[string]bool, id string, on bool) map[string]bool {
	out := maps.Clone(lists)
	if out == nil {
		out = map[string]bool{}
	}
	l, ok := m.cat.List(id)
	if !ok {
		return out
	}
	if Default(l, m.s.Languages) == on {
		delete(out, id)
	} else {
		out[id] = on
	}
	return out
}

// WithGroup is lists with a group turned off, every list of it off, or on, its lists back
// at their defaults; a group none of whose lists is on by default has them all turned on,
// so its switch is never on with nothing on.
func (m *Manager) WithGroup(lists map[string]bool, group string, on bool) map[string]bool {
	out := maps.Clone(lists)
	if out == nil {
		out = map[string]bool{}
	}
	in := m.cat.InGroup(group)
	any := false
	for _, l := range in {
		delete(out, l.ID)
		any = any || Default(l, m.s.Languages)
	}
	for _, l := range in {
		if want := on && !any || !on && Default(l, m.s.Languages); want {
			out[l.ID] = on
		}
	}
	return out
}

// Dangerous asks the safe blocker whether address, a page a tab is about to open, is a
// dangerous site's, and which list says so. Safe Browsing off, it says no. Any thread.
func (m *Manager) Dangerous(address string) (List, Group, bool) {
	for _, u := range *m.safe.Load() {
		if u.b.Request(address, address, "document") != Allow {
			return u.list, u.group, true
		}
	}
	return List{}, Group{}, false
}

// Status is what the lists on disk are: when the manifest was published, zero before any
// came, and each list's.
type Status struct {
	Published time.Time
	Lists     map[string]ListStatus
}

// ListStatus is a list's in the manifest: its size and when it last changed, and whether
// its text is here.
type ListStatus struct {
	Size    int64
	Changed time.Time
	Have    bool
}

// Shown is what the Lists page shows: the groups, in the catalogue's order, the test group
// never, each with its lists.
type Shown struct {
	// Downloaded is false until the first manifest came: "Lists not downloaded yet".
	Downloaded bool         `json:"downloaded"`
	Published  time.Time    `json:"published,omitzero"`
	Groups     []ShownGroup `json:"groups"`
}

// ShownGroup is a group's row: on while any of its lists is, with how many are of all.
type ShownGroup struct {
	ID      string      `json:"id"`
	Title   string      `json:"title"`
	About   string      `json:"about"`
	Blocker string      `json:"blocker"`
	On      bool        `json:"on"`
	OnCount int         `json:"onCount"`
	Total   int         `json:"total"`
	Lists   []ShownList `json:"lists"`
}

// ShownList is a list's row.
type ShownList struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	About   string    `json:"about"`
	Home    string    `json:"home"`
	Licence string    `json:"licence"`
	On      bool      `json:"on"`
	Changed time.Time `json:"changed,omitzero"`
	Size    int64     `json:"size,omitempty"`
	// Downloaded says its text is here.
	Downloaded bool `json:"downloaded"`
}

// Shown is the Lists page's model under c. Main thread.
func (m *Manager) Shown(c Choices) Shown {
	s := Shown{Downloaded: !m.status.Published.IsZero(), Published: m.status.Published, Groups: []ShownGroup{}}
	for _, g := range m.cat.Groups {
		if !g.Shown {
			continue
		}
		sg := ShownGroup{ID: g.ID, Title: g.Title, About: g.About, Blocker: g.Blocker, Lists: []ShownList{}}
		for _, l := range m.cat.InGroup(g.ID) {
			st := m.status.Lists[l.ID]
			on := m.On(c, l)
			sg.Lists = append(sg.Lists, ShownList{ID: l.ID, Title: l.Title, About: l.About, Home: l.Home, Licence: l.Licence, On: on, Changed: st.Changed, Size: st.Size, Downloaded: st.Have})
			sg.Total++
			if on {
				sg.OnCount++
			}
		}
		sg.On = sg.OnCount > 0
		s.Groups = append(s.Groups, sg)
	}
	return s
}

// unit is what compiles into one engine: a network group's lists that are on, or one list
// of the safe blocker; blocker is which.
type unit struct {
	name    string
	blocker string
	group   Group
	lists   []List
	b       Blocker
}

// prefix starts the names of the unit's engines' files under engines/; file is one's
// name, for a key of its lists' texts.
func (u unit) prefix() string { return u.blocker + "-" + u.name + "-" }

func (u unit) file(key string) string { return u.prefix() + key + ".engine" }

// engineKey is what follows an engine file's prefix.
var engineKey = regexp.MustCompile(`^[0-9a-f]{16}\.engine$`)

// key names the texts a unit compiles: two profiles with the same lists on share an
// engine, and another set of lists or a list's new text makes another.
func key(lists []List, texts map[string]string) string {
	var lines []string
	for _, l := range lists {
		line := l.ID + ":" + texts[l.ID]
		if l.Trusted {
			line += ":trusted"
		}
		lines = append(lines, line)
	}
	slices.Sort(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:8])
}

// rulesChanged and statusChanged tell the main thread's subscribers.
func (m *Manager) rulesChanged() {
	m.s.Post(func() {
		for _, f := range m.onRules {
			f()
		}
	})
}

func (m *Manager) statusChanged(st Status) {
	m.s.Post(func() {
		m.status = st
		for _, f := range m.onStatus {
			f()
		}
	})
}

func logf(format string, args ...any) { timeline.Infof("blocking: "+format, args...) }

func errorf(format string, args ...any) { timeline.Errorf("blocking: "+format, args...) }
