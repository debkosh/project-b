package sparks

import "testing"

func TestBadPatternsDontCompile(t *testing.T) {
	for _, p := range []string{
		"",
		"example.com",
		"https://example.com",
		"ftp://example.com/*",
		"chrome://settings/*",
		"ws://example.com/*",
		"https:///path",
		"https://*example.com/*",
		"https://ex*ample.com/*",
		"https://*./*",
		"https://example.*/*",
		"https://example.com:*/*",
		"https://example.com:0/*",
		"https://example.com:65536/*",
		"https://example.com:08/*",
		"https://example.com:/*",
		"https://user@example.com/*",
		"file://host/path",
		"<all_url>",
		"*//example.com/*",
	} {
		if _, err := compile(p); err == nil {
			t.Errorf("%q compiled", p)
		}
	}
}

func TestPatternsMatchAsChromeDoes(t *testing.T) {
	for _, c := range []struct {
		pattern, url string
		want         bool
	}{
		{"<all_urls>", "https://example.com/", true},
		{"<all_urls>", "http://example.com:8080/a?b", true},
		{"<all_urls>", "wss://example.com/socket", true},
		{"<all_urls>", "ftp://example.com/file", true},
		{"<all_urls>", "file:///home/me/a.html", true},
		{"<all_urls>", "chrome://settings/", false},
		{"<all_urls>", "about:blank", false},
		{"<all_urls>", "data:text/html,hi", false},

		{"*://*/*", "http://example.com/", true},
		{"*://*/*", "https://a.b.c/x/y", true},
		{"*://*/*", "file:///etc/hosts", false},
		{"*://*/*", "chrome://settings/", false},
		{"*://*/*", "ftp://example.com/", false},
		{"*://*/*", "ws://example.com/", false},

		{"https://example.com/*", "https://example.com/", true},
		{"https://example.com/*", "https://example.com", true},
		{"https://example.com/*", "https://EXAMPLE.com/a", true},
		{"https://example.com/*", "HTTPS://example.com/a", true},
		{"https://example.com/*", "http://example.com/", false},
		{"https://example.com/*", "https://www.example.com/", false},
		{"https://example.com/*", "https://example.com.evil.net/", false},
		{"https://Example.COM/*", "https://example.com/", true},

		{"*://*.example.com/*", "https://example.com/", true},
		{"*://*.example.com/*", "https://www.example.com/", true},
		{"*://*.example.com/*", "https://a.b.example.com/x", true},
		{"*://*.example.com/*", "https://badexample.com/", false},
		{"*://*.example.com/*", "https://example.com.au/", false},
		{"*://*.example.com/*", "https://exampleXcom/", false},

		{"https://example.com/*", "https://example.com:8443/", true},
		{"https://example.com:8443/*", "https://example.com:8443/", true},
		{"https://example.com:8443/*", "https://example.com/", false},
		{"https://example.com:8443/*", "https://example.com:9443/", false},
		{"https://example.com:443/*", "https://example.com/", true},
		{"http://localhost:3000/*", "http://localhost:3000/app", true},
		{"http://localhost:3000/*", "http://localhost/app", false},
		{"http://*:3000/*", "http://anything:3000/", true},
		{"http://[::1]:8080/*", "http://[::1]:8080/", true},
		{"http://[::1]/*", "http://[::1]:8080/", true},
		{"http://[::1]/*", "http://[::2]/", false},

		{"https://example.com/", "https://example.com/", true},
		{"https://example.com/", "https://example.com/a", false},
		{"https://example.com/a/*", "https://example.com/a/", true},
		{"https://example.com/a/*", "https://example.com/a/b/c", true},
		{"https://example.com/a/*", "https://example.com/a", false},
		{"https://example.com/a/*", "https://example.com/ab", false},
		{"https://example.com/*.html", "https://example.com/x/y.html", true},
		{"https://example.com/*.html", "https://example.com/y.htm", false},
		{"https://example.com/*/edit", "https://example.com/doc/1/edit", true},
		{"https://example.com/*/edit", "https://example.com/doc/1/edit/more", false},
		{"https://example.com/*a*b*", "https://example.com/xxaxxbxx", true},
		{"https://example.com/*a*b*", "https://example.com/xxbxxaxx", false},
		{"https://example.com/Path", "https://example.com/path", false},

		{"https://example.com/search?q=*", "https://example.com/search?q=go", true},
		{"https://example.com/search", "https://example.com/search?q=go", false},
		{"https://example.com/search*", "https://example.com/search?q=go", true},
		{"https://example.com/*", "https://example.com/page#top", true},
		{"https://example.com/page", "https://example.com/page#top", true},
		{"https://example.com/*top", "https://example.com/page#top", false},

		{"file:///home/*", "file:///home/me/a.html", true},
		{"file:///home/*", "file:///etc/passwd", false},
		{"file:///*", "https://example.com/", false},
		{"file:///*", "file://server/share", false},
		{"https://*/*", "https://example.com/", true},
	} {
		m, err := compile(c.pattern)
		if err != nil {
			t.Errorf("%q: %v", c.pattern, err)
			continue
		}
		tg, ok := parseTarget(c.url)
		if !ok {
			if c.want {
				t.Errorf("%q: can't read %q", c.pattern, c.url)
			}
			continue
		}
		if got := m.match(tg); got != c.want {
			t.Errorf("%q on %q: %v, want %v", c.pattern, c.url, got, c.want)
		}
	}
}

func TestAddressesWithoutASchemeDontRead(t *testing.T) {
	for _, u := range []string{"", "example.com/a", "/path", "%zz://x"} {
		if _, ok := parseTarget(u); ok {
			t.Errorf("%q read", u)
		}
	}
}

func TestGlob(t *testing.T) {
	for _, c := range []struct {
		pattern, s string
		want       bool
	}{
		{"", "", true},
		{"*", "", true},
		{"*", "anything", true},
		{"a", "a", true},
		{"a", "b", false},
		{"a*", "abc", true},
		{"*c", "abc", true},
		{"*b*", "abc", true},
		{"*d*", "abc", false},
		{"a**c", "abc", true},
		{"a*c", "ac", true},
		{"a*c", "acb", false},
		{"*aab", "aaaab", true},
	} {
		if got := glob(c.pattern, c.s); got != c.want {
			t.Errorf("glob(%q, %q) = %v", c.pattern, c.s, got)
		}
	}
}
