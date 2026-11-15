package sparks

import (
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// A match pattern is written as Chrome's extensions write one: <all_urls>, or
// scheme://host/path. The scheme is http, https, file, or * for either of http and https.
// The host is *, *.domain for the domain and every subdomain of it, or one host, with a
// port when only that port should match. The path is a glob, where * stands for any run of
// characters, and it is matched against the address's path with its query. A file: pattern
// has no host, as in file:///home/*.

// allURLs is every scheme <all_urls> matches.
var allURLs = []string{"http", "https", "ws", "wss", "ftp", "file"}

// matcher is one compiled match pattern.
type matcher struct {
	// schemes are the schemes it matches.
	schemes []string
	// all is <all_urls>, which matches every address of its schemes.
	all bool
	// host is the host to match, lowercase, "" for any host; sub also takes its subdomains.
	host string
	sub  bool
	// port is the one port to match, "" for any.
	port string
	path string
}

// target is an address in the parts a matcher looks at.
type target struct {
	scheme, host, port, path string
}

var errPattern = errors.New("not a match pattern")

// compile reads a match pattern.
func compile(pattern string) (matcher, error) {
	if pattern == "<all_urls>" {
		return matcher{schemes: allURLs, all: true}, nil
	}
	scheme, rest, ok := strings.Cut(pattern, "://")
	if !ok {
		return matcher{}, errPattern
	}
	var m matcher
	switch scheme {
	case "*":
		m.schemes = []string{"http", "https"}
	case "http", "https", "file":
		m.schemes = []string{scheme}
	default:
		return matcher{}, errPattern
	}
	host, path, ok := strings.Cut(rest, "/")
	if !ok {
		return matcher{}, errPattern
	}
	m.path = "/" + path
	if scheme == "file" {
		if host != "" {
			return matcher{}, errPattern
		}
		return m, nil
	}
	if host == "" {
		return matcher{}, errPattern
	}
	if i := strings.LastIndexByte(host, ':'); i >= 0 && !strings.HasSuffix(host, "]") {
		port := host[i+1:]
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
			return matcher{}, errPattern
		}
		host, m.port = host[:i], port
	}
	switch {
	case host == "*":
	case strings.HasPrefix(host, "*."):
		m.sub, m.host = true, host[2:]
	default:
		m.host = host
	}
	if m.host == "" && host != "*" || strings.Contains(m.host, "*") || strings.ContainsAny(m.host, "/?#@ ") {
		return matcher{}, errPattern
	}
	m.host = strings.ToLower(strings.Trim(m.host, "[]"))
	return m, nil
}

// parseTarget reads a full address, leaving out its fragment. It says false for one that
// can't be read or has no scheme.
func parseTarget(raw string) (target, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return target{}, false
	}
	t := target{scheme: strings.ToLower(u.Scheme), host: strings.ToLower(u.Hostname())}
	if u.Opaque != "" {
		t.path = u.Opaque
	} else {
		t.path = u.EscapedPath()
		if t.path == "" {
			t.path = "/"
		}
	}
	if u.RawQuery != "" || u.ForceQuery {
		t.path += "?" + u.RawQuery
	}
	t.port = u.Port()
	if t.port == "" {
		t.port = defaultPorts[t.scheme]
	}
	return t, true
}

// defaultPorts is the port an address of a scheme means when it gives none.
var defaultPorts = map[string]string{"http": "80", "https": "443", "ws": "80", "wss": "443", "ftp": "21"}

// match says whether the pattern takes the address.
func (m matcher) match(t target) bool {
	if !slices.Contains(m.schemes, t.scheme) {
		return false
	}
	if m.all {
		return true
	}
	if t.scheme == "file" {
		return t.host == "" && glob(m.path, t.path)
	}
	if t.host == "" {
		return false
	}
	if m.host != "" && t.host != m.host && !(m.sub && strings.HasSuffix(t.host, "."+m.host)) {
		return false
	}
	if m.port != "" && t.port != m.port {
		return false
	}
	return glob(m.path, t.path)
}

// glob says whether s matches pattern, where * stands for any run of characters and
// everything else for itself. On a miss it goes back only to the last *, so it never takes
// long.
func glob(pattern, s string) bool {
	p, i := 0, 0
	star, from := -1, 0
	for i < len(s) {
		switch {
		case p < len(pattern) && pattern[p] == '*':
			star, from = p, i
			p++
		case p < len(pattern) && pattern[p] == s[i]:
			p++
			i++
		case star >= 0:
			from++
			p, i = star+1, from
		default:
			return false
		}
	}
	for p < len(pattern) && pattern[p] == '*' {
		p++
	}
	return p == len(pattern)
}
