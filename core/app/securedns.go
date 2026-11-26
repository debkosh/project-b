package app

import (
	"projectb/engine"
	"projectb/prefs"
	"projectb/securedns"
	"projectb/settings"
	"projectb/timeline"
)

// Secure DNS (docs/secure-dns.md): the engine hears the profile's mode and provider at
// start, before any tab, and again whenever Settings changes them; each time it isn't Off,
// the provider is checked, and Settings shows the answer under it, "Cloudflare answers" or
// that it couldn't be reached.
type secureDNS struct {
	// set is the mode and template the engine has now.
	mode, template string
	// checked is the template last checked and ok its answer; checked is "" until one
	// answers.
	checked string
	ok      bool
}

// startSecureDNS gives the engine the profile's Secure DNS, and follows it.
func (a *App) startSecureDNS(e engine.Engine) {
	follow := func(p prefs.Prefs) {
		t := a.Prefs.SecureDNSTemplate()
		if p.SecureDNS == a.dns.mode && t == a.dns.template {
			return
		}
		a.dns.mode, a.dns.template = p.SecureDNS, t
		e.SetSecureDNS(p.SecureDNS, t)
		timeline.Infof("secure dns: %s %s", p.SecureDNS, securedns.Host(t))
		if t != "" && t != a.dns.checked {
			e.CheckSecureDNS(t)
		}
	}
	follow(a.Prefs.All())
	a.Prefs.OnChanged(follow)
}

// SecureDNSChecked keeps a check's answer and shows it in Settings, unless the provider
// changed again since.
func (a *App) SecureDNSChecked(template string, ok bool) {
	timeline.Infof("secure dns: %s answers %v", securedns.Host(template), ok)
	a.dns.checked, a.dns.ok = template, ok
	if template == a.dns.template {
		a.Settings.Send(a.dnsCheck())
	}
}

// dnsCheck is Settings' message of the last check: its provider's host and whether it
// answered, empty while the provider in use hasn't been checked.
func (a *App) dnsCheck() map[string]any {
	m := map[string]any{"t": "dnsCheck", "host": "", "ok": false}
	if a.dns.checked != "" && a.dns.checked == a.dns.template {
		m["host"], m["ok"] = securedns.Host(a.dns.checked), a.dns.ok
	}
	return m
}

// secureDNSSettingsPart gives Settings the providers and the last check.
func (a *App) secureDNSSettingsPart() settings.Part {
	return settings.Part{
		Snapshot: func(into map[string]any) {
			into["dnsProviders"] = securedns.Providers
			into["dnsCheck"] = a.dnsCheck()
		},
		Handle: func(engine.TabID, string, []byte) bool { return false },
	}
}
