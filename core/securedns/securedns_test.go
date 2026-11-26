package securedns

import "testing"

// A template is taken only in the form Chromium takes it: https, a host, dns the only
// variable and never in the host.
func TestValidTakesOnlyWhatChromiumTakes(t *testing.T) {
	for _, c := range []struct {
		t  string
		ok bool
	}{
		{"https://dns.example/dns-query{?dns}", true},
		{"https://dns.example/dns-query", true},
		{"https://dns.example:8443/q/{dns}", true},
		{"https://127.0.0.1:9443/dns-query{?dns}", true},
		{"http://dns.example/dns-query{?dns}", false},
		{"dns.example/dns-query", false},
		{"https:///dns-query", false},
		{"https://{dns}.example/q", false},
		{"https://dns.example/q{?name}", false},
		{"https://dns.example/q{?dns", false},
		{"https://dns.example/q}", false},
		{"https://user@dns.example/q", false},
		{"https://dns.example/a b", false},
		{"", false},
	} {
		if got := Valid(c.t); got != c.ok {
			t.Errorf("Valid(%q) = %v, want %v", c.t, got, c.ok)
		}
	}
}

// Every listed provider's template is valid, and the default is listed.
func TestTheListedProvidersAreValid(t *testing.T) {
	for _, p := range Providers {
		if !Valid(p.Template) || p.Name == "" {
			t.Errorf("%+v", p)
		}
	}
	if _, ok := Get(Default); !ok {
		t.Fatal("the default isn't listed")
	}
	if Template(Custom, "https://d.example/q") != "https://d.example/q" || Template("google", "x") != "https://dns.google/dns-query{?dns}" {
		t.Fatal("Template")
	}
	if Host("https://dns.example:8443/q{?dns}") != "dns.example" {
		t.Fatal(Host("https://dns.example:8443/q{?dns}"))
	}
}
