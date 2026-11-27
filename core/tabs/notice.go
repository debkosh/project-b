package tabs

import (
	"net/url"
	"strings"

	"projectb/engine"
)

// A tab's notice (docs/daily-driver.md): what a shell draws over a page that has nothing to
// show, because it failed to load, crashed, was blocked or has a bad certificate, and what
// the person can do about it. Go words it; the shell draws it and sends back the action.

// Notice is what a tab shows in place of its page.
type Notice struct {
	// Kind is "failed", "crashed", "blocked", "certificate" or "unsafe".
	Kind string `json:"kind"`
	// Title is one plain sentence; Detail a second line, "" for none.
	Title   string         `json:"title"`
	Detail  string         `json:"detail,omitempty"`
	Actions []NoticeAction `json:"actions"`
}

// NoticeAction is one thing the person can do: "reload", "back" or "proceed".
type NoticeAction struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Primary bool   `json:"primary,omitempty"`
}

// certificate is a tab's certificate error: the address that had it, why, and whether
// the person may pass it.
type certificate struct {
	url      string
	reason   string
	passable bool
}

// certificateTitles say what is wrong with a site's certificate, after its host, by reason.
var certificateTitles = map[string]string{
	"expired":    "'s certificate has expired",
	"untrusted":  "'s certificate isn't from an authority this Mac trusts",
	"wrong-host": "'s certificate is for another site",
	"revoked":    "'s certificate was revoked",
	"invalid":    "'s certificate isn't valid",
}

// certificateDetail is what a certificate warning means, under its title.
const certificateDetail = "This may not be the site it says it is, and what you send could be read by someone else."

// hostOf is the host a notice names for an address, without www.
func hostOf(u string) string {
	p, err := url.Parse(u)
	if err != nil || p.Host == "" {
		return u
	}
	return strings.TrimPrefix(p.Host, "www.")
}

// Danger is what the Safe Browsing lists say of a dangerous site (docs/blocking.md): Kind
// "malware" or "phishing", and the title of the list that said so. A tab has it while its
// page is one, refused or opened anyway, for the shells' Dangerous in the address field.
type Danger struct {
	Kind string `json:"kind"`
	List string `json:"list"`
}

// unsafeSite is a tab's refused dangerous site: its address and what the lists say of it.
type unsafeSite struct {
	url string
	Danger
}

// unsafeTitles say what a dangerous site may do, after its host, by kind.
var unsafeTitles = map[string]string{
	"malware":  " may harm your device",
	"phishing": " may be trying to steal your information",
}

// SetDanger gives the tabs the question each web tab's navigation asks, after the policy's
// gates: whether its address is a dangerous site's. One is refused, and the tab's notice
// says so, unless the person chose to open its host anyway since the app started.
func (m *Manager) SetDanger(f func(url string) (Danger, bool)) { m.danger = f }

// isDangerous asks the question for tab t's navigation to u, and keeps a yes for its
// notice. Either way t's Danger is what the lists say of u: a host opened anyway keeps
// what they said of it, as Not secure stays after a certificate is passed.
func (m *Manager) isDangerous(t *Tab, u string) bool {
	if m.danger == nil {
		return false
	}
	if d, ok := m.passedUnsafe[hostOf(u)]; ok {
		t.Danger = &d
		return false
	}
	d, ok := m.danger(u)
	if !ok {
		return false
	}
	if unsafeTitles[d.Kind] == "" {
		d.Kind = "malware"
	}
	m.unsafe[t.ID] = &unsafeSite{url: address(u), Danger: d}
	t.Danger = &d
	return true
}

// noticeOf is t's notice now, nil while its page has something to show or is loading.
func (m *Manager) noticeOf(t *Tab) *Notice {
	u := m.unsafe[t.ID]
	if u != nil && u.url != t.URL {
		u = nil
	}
	if t.URL == "" || t.Loading || !(t.Crashed || t.Blocked || t.Error != "" || u != nil) {
		return nil
	}
	host := hostOf(t.URL)
	back := NoticeAction{ID: "back", Title: "Go back"}
	n := &Notice{Actions: []NoticeAction{}}
	c := m.certificates[t.ID]
	switch {
	case u != nil:
		n.Kind, n.Title, n.Detail = "unsafe", host+unsafeTitles[u.Kind], u.List+" lists it as a "+u.Kind+" site."
		back.Primary = true
	case t.Blocked:
		n.Kind, n.Title = "blocked", host+" is blocked"
		back.Primary = true
	case t.Crashed:
		n.Kind, n.Title = "crashed", "This page crashed"
		n.Actions = append(n.Actions, NoticeAction{ID: "reload", Title: "Reload", Primary: true})
	case c != nil && c.url == t.URL:
		n.Kind, n.Title, n.Detail = "certificate", host+certificateTitles[c.reason], certificateDetail
		back.Primary = true
	default:
		n.Kind, n.Detail = "failed", t.ErrorCode
		switch t.Error {
		case engine.FailNotFound:
			n.Title = "Can't find " + host
		case engine.FailOffline:
			n.Title = "No internet connection"
		case engine.FailTimeout:
			n.Title = host + " took too long to answer"
		case engine.FailProxy:
			n.Title = "The proxy isn't answering"
		case engine.FailTLS:
			n.Title = "Can't connect securely to " + host
		default:
			n.Title = "Can't reach " + host
		}
		n.Actions = append(n.Actions, NoticeAction{ID: "reload", Title: "Try again", Primary: true})
	}
	if t.CanGoBack {
		n.Actions = append(n.Actions, back)
	}
	if n.Kind == "certificate" && c.passable || n.Kind == "unsafe" {
		n.Actions = append(n.Actions, NoticeAction{ID: "proceed", Title: "Open anyway"})
	}
	return n
}

// HandleCertificateError answers the engine about a certificate error on a request of
// tab's: true lets it through. A page's own address is refused, and the tab's notice then
// says why, unless the person passed this host's errors before; anything else a page
// loads is let through only from a host they passed.
func (m *Manager) HandleCertificateError(id engine.TabID, e engine.CertificateError) bool {
	host := hostOf(e.URL)
	if e.Passable && m.passed[host] {
		return true
	}
	if t := m.tabs[id]; t != nil && e.MainFrame {
		reason := e.Reason
		if certificateTitles[reason] == "" {
			reason = "invalid"
		}
		m.certificates[id] = &certificate{url: address(e.URL), reason: reason, passable: e.Passable}
	}
	return false
}

// NoticeAction does what the person chose on a tab's notice. One the notice doesn't offer
// is ignored.
func (m *Manager) NoticeAction(id engine.TabID, action string) {
	t := m.tabs[id]
	if t == nil || t.Notice == nil || !slicesHas(t.Notice.Actions, action) {
		return
	}
	switch action {
	case "reload":
		m.eng.Reload(id)
	case "back":
		m.eng.GoBack(id)
	case "proceed":
		// This host passes until the app quits; kept nowhere.
		if t.Notice.Kind == "unsafe" {
			// The refused navigation left no entry of its own to reload, so its address
			// loads again.
			u := m.unsafe[id]
			m.passedUnsafe[hostOf(u.url)] = u.Danger
			m.loadURL(id, u.url)
			return
		}
		m.passed[hostOf(m.certificates[id].url)] = true
		m.eng.Reload(id)
	}
}

func slicesHas(actions []NoticeAction, id string) bool {
	for _, a := range actions {
		if a.ID == id {
			return true
		}
	}
	return false
}
