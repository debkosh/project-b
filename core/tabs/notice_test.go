package tabs

import (
	"testing"

	"projectb/engine"
)

func actionIDs(n *Notice) []string {
	var ids []string
	for _, a := range n.Actions {
		ids = append(ids, a.ID)
	}
	return ids
}

func TestAFailedLoadGetsANoticeToTryAgain(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	id := m.Create(1)
	m.HandleNavigationStarting(id, 1, "https://www.gone.test/x")
	m.HandleLoadingChanged(id, true)
	if tab, _ := m.Get(id); tab.Notice != nil {
		t.Fatalf("a notice while loading: %+v", tab.Notice)
	}
	m.HandleLoadingChanged(id, false)
	m.HandleNavigationFailed(id, 1, "https://www.gone.test/x", engine.FailNotFound, "ERR_NAME_NOT_RESOLVED")
	tab, _ := m.Get(id)
	n := tab.Notice
	if n == nil || n.Kind != "failed" || n.Title != "Can't find gone.test" || n.Detail != "ERR_NAME_NOT_RESOLVED" || len(n.Actions) != 1 || n.Actions[0].ID != "reload" || !n.Actions[0].Primary {
		t.Fatalf("notice %+v", n)
	}
	m.NoticeAction(id, "proceed") // not offered: ignored
	m.NoticeAction(id, "reload")
	if e.reloads != 1 {
		t.Fatalf("%d reloads, want only Try again's", e.reloads)
	}
	m.HandleNavigationStarting(id, 1, "https://www.gone.test/x")
	m.HandleNavigationFinished(id, 1, "https://www.gone.test/x")
	if tab, _ := m.Get(id); tab.Notice != nil {
		t.Fatalf("the notice stayed after the page loaded: %+v", tab.Notice)
	}
}

func TestACrashedPageAndABlockedOneGetTheirNotices(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	id := m.Create(1)
	m.HandleNavigationStarting(id, 1, "https://a.test/")
	m.HandleNavigationFinished(id, 1, "https://a.test/")
	m.HandlePageCrashed(id)
	if tab, _ := m.Get(id); tab.Notice == nil || tab.Notice.Kind != "crashed" || tab.Notice.Actions[0].Title != "Reload" {
		t.Fatalf("crashed: %+v", tab.Notice)
	}
	m.AddGate(func(u string) bool { return u != "https://bad.test/" })
	m.HandleHistoryChanged(id, true, false)
	m.HandleNavigationStarting(id, 1, "https://bad.test/")
	tab, _ := m.Get(id)
	if n := tab.Notice; n == nil || n.Kind != "blocked" || n.Title != "bad.test is blocked" || len(n.Actions) != 1 || n.Actions[0].ID != "back" {
		t.Fatalf("blocked: %+v", n)
	}
}

func TestABadCertificateIsRefusedUntilThePersonPassesItsHost(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	id := m.Create(1)
	bad := engine.CertificateError{URL: "https://old.test/", Reason: "expired", MainFrame: true, Passable: true}
	load := func(e engine.CertificateError) bool {
		m.HandleNavigationStarting(id, 1, e.URL)
		if m.HandleCertificateError(id, e) {
			m.HandleNavigationFinished(id, 1, e.URL)
			return true
		}
		m.HandleNavigationFailed(id, 1, e.URL, engine.FailTLS, "ERR_CERT_DATE_INVALID")
		return false
	}
	if load(bad) {
		t.Fatal("a bad certificate was let through")
	}
	tab, _ := m.Get(id)
	n := tab.Notice
	if n == nil || n.Kind != "certificate" || n.Title != "old.test's certificate has expired" || n.Detail == "" || actionIDs(n)[len(n.Actions)-1] != "proceed" {
		t.Fatalf("notice %+v", n)
	}
	// A subresource of another host with a bad certificate is refused, with no notice.
	if m.HandleCertificateError(id, engine.CertificateError{URL: "https://cdn.test/a.js", Reason: "untrusted", Passable: true}) {
		t.Fatal("a subresource's bad certificate was let through")
	}
	m.NoticeAction(id, "proceed")
	if !load(bad) {
		t.Fatal("refused after Open anyway")
	}
	if tab, _ := m.Get(id); tab.Notice != nil {
		t.Fatalf("notice after it loaded: %+v", tab.Notice)
	}
	// A site that demands a valid certificate can't be passed, and offers no Open anyway.
	strict := engine.CertificateError{URL: "https://bank.test/", Reason: "wrong-host", MainFrame: true}
	if load(strict) {
		t.Fatal("a strict site's bad certificate was let through")
	}
	tab, _ = m.Get(id)
	for _, a := range tab.Notice.Actions {
		if a.ID == "proceed" {
			t.Fatal("a strict site offers Open anyway")
		}
	}
}

// loadsEngine records the addresses loaded.
type loadsEngine struct {
	fakeEngine
	loaded []string
}

func (f *loadsEngine) LoadURL(_ engine.TabID, u string) { f.loaded = append(f.loaded, u) }

// A site the Safe Browsing lists name is refused, and its notice says why and which list
// said so, with Go back first; Open anyway lets its host through until the app quits and
// loads it again.
func TestADangerousSiteIsRefusedUntilThePersonOpensItAnyway(t *testing.T) {
	e := &loadsEngine{}
	m := oneWindow(e)
	id := m.Create(1)
	asked := 0
	m.SetDanger(func(u string) (Danger, bool) {
		asked++
		switch hostOf(u) {
		case "steal.test":
			return Danger{Kind: "phishing", List: "Phishing URL Blocklist"}, true
		case "harm.test":
			return Danger{Kind: "malware", List: "URLhaus filter"}, true
		}
		return Danger{}, false
	})
	m.HandleNavigationStarting(id, 1, "https://good.test/")
	m.HandleNavigationFinished(id, 1, "https://good.test/")
	m.HandleHistoryChanged(id, true, false)
	if got := m.HandleNavigationStarting(id, 2, "https://www.steal.test/login"); got != engine.NavigationBlock {
		t.Fatalf("a phishing site: %v", got)
	}
	tab, _ := m.Get(id)
	n := tab.Notice
	if n == nil || n.Kind != "unsafe" || n.Title != "steal.test may be trying to steal your information" ||
		n.Detail != "Phishing URL Blocklist lists it as a phishing site." {
		t.Fatalf("notice %+v", n)
	}
	if ids := actionIDs(n); len(ids) != 2 || ids[0] != "back" || !n.Actions[0].Primary || n.Actions[0].Title != "Go back" || ids[1] != "proceed" || n.Actions[1].Title != "Open anyway" {
		t.Fatalf("actions %+v", n.Actions)
	}
	if tab.Blocked {
		t.Fatal("a dangerous site is a blocked one")
	}
	if d := tab.Danger; d == nil || *d != (Danger{Kind: "phishing", List: "Phishing URL Blocklist"}) {
		t.Fatalf("the refused tab's danger %+v", d)
	}
	m.HandleNavigationStarting(id, 3, "https://harm.test/")
	if tab, _ := m.Get(id); tab.Notice == nil || tab.Notice.Title != "harm.test may harm your device" {
		t.Fatalf("malware: %+v", tab.Notice)
	}

	m.NoticeAction(id, "proceed")
	if len(e.loaded) != 1 || e.loaded[0] != "https://harm.test/" {
		t.Fatalf("Open anyway loaded %v", e.loaded)
	}
	if got := m.HandleNavigationStarting(id, 4, "https://harm.test/other"); got != engine.NavigationProceed {
		t.Fatalf("after Open anyway: %v", got)
	}
	m.HandleNavigationFinished(id, 4, "https://harm.test/other")
	if tab, _ := m.Get(id); tab.Notice != nil {
		t.Fatalf("notice after Open anyway: %+v", tab.Notice)
	}
	// Opened anyway, its pages are still dangerous, for the address field's Dangerous.
	if tab, _ := m.Get(id); tab.Danger == nil || *tab.Danger != (Danger{Kind: "malware", List: "URLhaus filter"}) {
		t.Fatalf("danger after Open anyway: %+v", tab.Danger)
	}
	// The other host is still refused, and asked again.
	before := asked
	if got := m.HandleNavigationStarting(id, 5, "https://steal.test/"); got != engine.NavigationBlock || asked != before+1 {
		t.Fatalf("another host: %v", got)
	}
	// A site the lists don't name has none.
	m.HandleNavigationStarting(id, 6, "https://good.test/")
	if tab, _ := m.Get(id); tab.Danger != nil {
		t.Fatalf("a good site's danger %+v", tab.Danger)
	}
}

// A dangerous site in a tab with nothing to go back to still has a safe answer first:
// Close tab, which closes it.
func TestUnsafeNoticeWithNothingBehind(t *testing.T) {
	e := &loadsEngine{}
	m := oneWindow(e)
	id := m.Create(1)
	m.SetDanger(func(u string) (Danger, bool) {
		return Danger{Kind: "malware", List: "URLhaus filter"}, hostOf(u) == "harm.test"
	})
	if got := m.HandleNavigationStarting(id, 1, "https://harm.test/"); got != engine.NavigationBlock {
		t.Fatalf("a malware site: %v", got)
	}
	tab, _ := m.Get(id)
	n := tab.Notice
	if ids := actionIDs(n); n == nil || len(ids) != 2 || ids[0] != "close" || !n.Actions[0].Primary || n.Actions[0].Title != "Close tab" || ids[1] != "proceed" {
		t.Fatalf("actions %+v", n)
	}
	m.NoticeAction(id, "close")
	if _, ok := m.Get(id); ok {
		t.Fatal("Close tab left the tab open")
	}
}
