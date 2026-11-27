package blocking

import (
	"bytes"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/net/publicsuffix"
)

// Library is the blocking library: libpbblock in the app (core/blocking/native, the
// pbblock build tag), and Plain, a small one in Go, for the tests and the mock. The lists
// manager knows nothing else of it.
type Library interface {
	// New makes a blocker, which lives as long as the process.
	New() Blocker
	// Compile builds one group's engine, in the library's own form, from its lists' texts.
	Compile(texts [][]byte) ([]byte, error)
}

// Blocker is one blocker of the library: its groups, each one compiled engine, swapped
// whole. Every method may be called from any goroutine.
type Blocker interface {
	// Load loads a group's engine in place of the one it had; false when the library can't
	// read it, being damaged or another version's.
	Load(group string, engine []byte) bool
	Unload(group string)
	// Resources gives the scriptlets and stand-ins, adblock-rust's resources JSON, to the
	// groups loaded after it.
	Resources(json []byte) bool
	// Request is the verdict on one request: Allow, Block or Redirect.
	Request(url, source, kind string) int
	// Hooks is the engine's PBBlockerHooks table for it, for engine.SetBlocker.
	Hooks() unsafe.Pointer
}

// A request's verdicts.
const (
	Allow    = 0
	BlockIt  = 1
	Redirect = 2
)

// Plain is a library in Go that reads only the simplest network rules, for the tests and
// the mock: "||host^" with $third-party, $document or $all, and "|address" for an address
// and what follows it, "|address|" for it alone. A host rule with no options holds for a
// page's own address too, as adblock-rust's does; an address rule only with $document or
// $all. Lines it can't read are skipped. Its engine is its texts, after PlainVersion.
type Plain struct{}

// PlainVersion starts each engine Plain compiles; one that starts otherwise is another
// version's, and doesn't load. A test changes it to make the engines on disk another's.
var PlainVersion = "plain/1\n"

func (Plain) New() Blocker {
	b := &plainBlocker{}
	b.groups.Store(&map[string][]plainRule{})
	return b
}

func (Plain) Compile(texts [][]byte) ([]byte, error) {
	out := []byte(PlainVersion)
	for _, t := range texts {
		out = append(out, t...)
		out = append(out, '\n')
	}
	return out, nil
}

type plainRule struct {
	host, prefix   string
	exact          bool
	thirdParty     bool
	document, page bool
}

type plainBlocker struct {
	mu     sync.Mutex
	groups atomic.Pointer[map[string][]plainRule]
}

func (b *plainBlocker) put(group string, rules []plainRule, keep bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	next := map[string][]plainRule{}
	for g, r := range *b.groups.Load() {
		if g != group {
			next[g] = r
		}
	}
	if keep {
		next[group] = rules
	}
	b.groups.Store(&next)
}

func (b *plainBlocker) Load(group string, engine []byte) bool {
	text, ok := bytes.CutPrefix(engine, []byte(PlainVersion))
	if !ok {
		return false
	}
	var rules []plainRule
	for _, line := range strings.Split(string(text), "\n") {
		if r, ok := parsePlain(strings.TrimSpace(line)); ok {
			rules = append(rules, r)
		}
	}
	b.put(group, rules, true)
	return true
}

func parsePlain(line string) (plainRule, bool) {
	var r plainRule
	line, opts, _ := strings.Cut(line, "$")
	for _, o := range strings.Split(opts, ",") {
		switch o {
		case "third-party":
			r.thirdParty = true
		case "document", "all":
			r.document = true
		}
	}
	switch {
	case strings.HasPrefix(line, "||") && strings.HasSuffix(line, "^"):
		r.host = strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(line, "||"), "^"))
		r.page = !r.thirdParty && opts == ""
		return r, r.host != "" && !strings.ContainsAny(r.host, "/*")
	case strings.HasPrefix(line, "|") && !strings.HasPrefix(line, "||"):
		r.prefix = strings.TrimPrefix(line, "|")
		r.prefix, r.exact = strings.CutSuffix(r.prefix, "|")
		return r, r.prefix != ""
	}
	return r, false
}

func (b *plainBlocker) Unload(group string) { b.put(group, nil, false) }

func (b *plainBlocker) Resources([]byte) bool { return true }

func (b *plainBlocker) Request(address, source, kind string) int {
	u, err := url.Parse(address)
	if err != nil {
		return Allow
	}
	host := strings.ToLower(u.Hostname())
	for _, rules := range *b.groups.Load() {
		for _, r := range rules {
			if kind == "document" && !r.document && !r.page {
				continue
			}
			if r.thirdParty && site(host) == site(hostOf(source)) {
				continue
			}
			if r.host != "" && (host == r.host || strings.HasSuffix(host, "."+r.host)) ||
				r.prefix != "" && (r.exact && address == r.prefix || !r.exact && strings.HasPrefix(address, r.prefix)) {
				return BlockIt
			}
		}
	}
	return Allow
}

// Hooks is the blocker itself: the mock engine only keeps it, and asks nothing.
func (b *plainBlocker) Hooks() unsafe.Pointer { return unsafe.Pointer(b) }

func hostOf(address string) string {
	u, err := url.Parse(address)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// site is a host's registrable domain, as a third party is told apart.
func site(host string) string {
	if s, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		return s
	}
	return host
}
