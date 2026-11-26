package app_test

import (
	"slices"
	"testing"

	"projectb/prefs"
)

// Secure DNS (docs/secure-dns.md): the engine hears Off at start; a mode set in Settings
// reaches it with its provider's template, and the provider is checked, its answer shown
// in Settings, "answers" or not; a custom template no server can answer says so; and the
// choice is kept.
func TestSecureDNSReachesTheEngineAndSettingsShowItsCheck(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	sent := func() (s []string) {
		r.onMain(func() { s = slices.Clone(r.e.SecureDNS()) })
		return
	}
	if s := sent(); !slices.Equal(s, []string{"off"}) {
		t.Fatalf("at start the engine had %v", s)
	}
	got, snap := r.openSettings()
	if p, _ := snap["dnsProviders"].([]any); len(p) < 5 {
		t.Fatalf("the snapshot's providers are %v", snap["dnsProviders"])
	}
	r.settings(`{"t":"setPref","name":"secureDns","value":"automatic"}`)
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "dnsCheck" })
	if m["host"] != "chrome.cloudflare-dns.com" || m["ok"] != true {
		t.Fatalf("the check said %v", m)
	}
	if s := sent(); s[len(s)-1] != "automatic https://chrome.cloudflare-dns.com/dns-query" {
		t.Fatalf("the engine had %v", s)
	}
	r.settings(`{"t":"setPref","name":"dnsCustom","value":"https://dns.invalid/dns-query{?dns}"}`)
	r.settings(`{"t":"setPref","name":"secureDns","value":"secure"}`)
	m = r.waitOn(got, func(m map[string]any) bool { return m["t"] == "dnsCheck" && m["host"] == "dns.invalid" })
	if m["ok"] != false {
		t.Fatalf("an unreachable provider's check said %v", m)
	}
	if s := sent(); s[len(s)-1] != "secure https://dns.invalid/dns-query{?dns}" {
		t.Fatalf("the engine had %v", s)
	}
	r.settings(`{"t":"setPref","name":"dnsCustom","value":"http://dns.example/q"}`)
	r.settings(`{"t":"setPref","name":"secureDns","value":"off"}`)
	r.waitOn(got, func(m map[string]any) bool {
		p, _ := m["prefs"].(map[string]any)
		return m["t"] == "prefs" && p["secureDns"] == "off"
	})
	if s := sent(); s[len(s)-1] != "off" {
		t.Fatalf("the engine had %v", s)
	}
	r.flushPrefs()
	if p := prefs.New(dir).All(); p.SecureDNS != "off" || p.DNSProvider != "custom" || p.DNSCustom != "https://dns.invalid/dns-query{?dns}" {
		t.Fatalf("kept %+v", p)
	}
}
