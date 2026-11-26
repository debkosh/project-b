package passwords

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/debkosh/project-b/browserdata"

	"projectb/forms"
)

// saving is a filling whose offers to save are kept, nil for one taken away.
type saving struct {
	*filling
	t      *testing.T
	offers []*SaveOffer
}

func newSaving(t *testing.T, logins ...browserdata.Login) *saving {
	t.Helper()
	old := Settle
	Settle = 30 * time.Millisecond
	t.Cleanup(func() { Settle = old })
	s := &saving{filling: newFilling(t, logins...), t: t}
	s.f.OnSave(func(o *SaveOffer) { s.offers = append(s.offers, o) })
	return s
}

// offer is the offer to save showing, nil for none, once a page has had time to settle.
func (s *saving) offer() *SaveOffer {
	time.Sleep(4 * Settle)
	if len(s.offers) == 0 {
		return nil
	}
	return s.offers[len(s.offers)-1]
}

// goesOn is a tab going on to another page, which then loads.
func (s *saving) goesOn(tab string) {
	s.f.Navigated(tab, "https://shop.example/account")
	s.f.Loaded(tab)
}

// signIn plays a sign-in on shop's page: its fields told, then its form sent holding a
// username and a password.
func (s *saving) signIn(doc, username, password string) forms.Stamp {
	at := shop
	at.Document = doc
	s.tell(at, "fields", login)
	s.tell(at, "submitted", forms.Sent{Form: "F1", Values: map[string]string{"f1": username, "f2": password}})
	return at
}

func (s *saving) passwords() []string {
	s.t.Helper()
	all, err := s.m.heldFor("https://shop.example")
	if err != nil {
		s.t.Fatal(err)
	}
	out := []string{}
	for _, h := range all {
		out = append(out, h.Username+":"+h.password)
	}
	return out
}

// A form sent proves nothing: saving is offered only once the tab goes on to a page that
// shows no sign-in again, or the form goes with the page staying. Save keeps the login,
// sealed; the offer holds no password, which the person may look at; and a sign-in that
// comes back to a sign-in offers nothing.
func TestSavingIsOfferedOnceASignInGoesThrough(t *testing.T) {
	s := newSaving(t)
	s.signIn("d1", "asha", "correct horse")
	if o := s.offer(); o != nil {
		t.Fatalf("a form sent, and nothing more, offered %+v", o)
	}
	s.f.Navigated("7", "https://shop.example/account")
	if o := s.offer(); o != nil {
		t.Fatalf("a tab gone on to a page still loading offered %+v", o)
	}
	s.f.Loaded("7")
	o := s.offer()
	if o == nil || o.Tab != "7" || o.Site != "shop.example" || o.Username != "asha" || o.Update {
		t.Fatalf("the offer after the tab went on: %+v", o)
	}
	if b, _ := json.Marshal(o); strings.Contains(string(b), "correct horse") {
		t.Fatalf("the offer holds the password: %s", b)
	}
	if s.f.Typed(o.ID) != "correct horse" || s.f.Typed(o.ID+99) != "" {
		t.Fatal("the password of the offer, for the person to look at")
	}
	if err := s.f.Answer(o.ID, "save"); err != nil {
		t.Fatal(err)
	}
	if got := s.passwords(); len(got) != 1 || got[0] != "asha:correct horse" || s.offers[len(s.offers)-1] != nil {
		t.Fatalf("after Save: %v, the offer %+v", got, s.offers[len(s.offers)-1])
	}
	if err := s.f.Answer(o.ID, "save"); !errors.Is(err, ErrNoOffer) || s.f.Typed(o.ID) != "" {
		t.Fatalf("an offer answered once: %v", err)
	}

	// A sign-in that fails: the page it comes to shows the sign-in again.
	n := len(s.offers)
	s.signIn("d2", "asha", "a wrong one")
	s.goesOn("7")
	back := shop
	back.Document = "d3"
	s.tell(back, "fields", login)
	if s.offer(); len(s.offers) != n {
		t.Fatalf("a sign-in that came back to a sign-in offered %+v", s.offers[len(s.offers)-1])
	}

	// An offer stays while its tab stays on the site, on whichever of its hosts, and goes
	// when the tab goes to another site.
	s.signIn("d3a", "other", "another one")
	s.goesOn("7")
	stays := s.offer()
	s.f.Navigated("7", "https://www.shop.example/orders")
	if last := s.offers[len(s.offers)-1]; stays == nil || last != stays {
		t.Fatalf("an offer went as its tab went on within the site: %+v, %+v", stays, last)
	}
	s.f.Navigated("7", "https://elsewhere.example/")
	if last := s.offers[len(s.offers)-1]; last != nil || s.f.Typed(stays.ID) != "" {
		t.Fatalf("an offer followed its tab to another site: %+v", last)
	}

	// A sign-in sent while an offer waits takes the offer away: it is of the one before.
	s.signIn("d3b", "asha", "correct horse")
	s.goesOn("7")
	n = len(s.offers)
	if s.offer(); len(s.offers) != n {
		t.Fatalf("a login saved as it is was offered: %+v", s.offers[len(s.offers)-1])
	}
	s.signIn("d3c", "other", "another one")
	s.goesOn("7")
	waiting := s.offer()
	s.signIn("d3d", "third", "third one")
	if last := s.offers[len(s.offers)-1]; waiting == nil || last != nil {
		t.Fatalf("an offer stayed after another sign-in was sent: %+v, %+v", waiting, last)
	}
	if err := s.f.Answer(waiting.ID, "save"); !errors.Is(err, ErrNoOffer) || len(s.passwords()) != 1 {
		t.Fatalf("the offer of a sign-in another took the place of: %v, %v", err, s.passwords())
	}

	// A page that stays, its form gone after the request: that went through.
	at := s.signIn("d4", "work", "second one")
	s.tell(at, "gone", forms.Sent{Form: "F1", Values: map[string]string{"f1": "work", "f2": "second one"}})
	if o := s.offer(); o == nil || o.Username != "work" || o.Update {
		t.Fatalf("the offer after the form went: %+v", o)
	}
}

// What is offered for what is saved already: nothing for a login saved as it is, an update
// for its username with another password, which gives the saved login the new one; Not now
// keeps nothing; Never has the site not offered on again, until a login is saved for it.
func TestUpdateNotNowAndNever(t *testing.T) {
	s := newSaving(t, browserdata.Login{Origin: "https://shop.example", Username: "asha", Password: "correct horse"})
	went := func(doc, username, password string) *SaveOffer {
		t.Helper()
		n := len(s.offers)
		s.signIn(doc, username, password)
		s.goesOn("7")
		if s.offer(); len(s.offers) == n {
			return nil
		}
		return s.offers[len(s.offers)-1]
	}
	if o := went("d1", "asha", "correct horse"); o != nil {
		t.Fatalf("a login saved as it is was offered: %+v", o)
	}
	o := went("d2", "asha", "a new one")
	if o == nil || !o.Update || o.Username != "asha" {
		t.Fatalf("a saved username with another password: %+v", o)
	}
	if err := s.f.Answer(o.ID, "save"); err != nil {
		t.Fatal(err)
	}
	if got := s.passwords(); len(got) != 1 || got[0] != "asha:a new one" {
		t.Fatalf("after Update: %v", got)
	}
	// Not now: the offer goes, nothing is kept, and the next sign-in is offered again.
	o = went("d3", "work", "second one")
	if err := s.f.Answer(o.ID, "dismiss"); err != nil || len(s.passwords()) != 1 {
		t.Fatalf("after Not now: %v, %v", err, s.passwords())
	}
	if o = went("d4", "work", "second one"); o == nil {
		t.Fatal("after Not now the next sign-in wasn't offered")
	}
	// Never: not offered on this site again, though a saved login's new password still is.
	if err := s.f.Answer(o.ID, "never"); err != nil {
		t.Fatal(err)
	}
	if o := went("d5", "work", "second one"); o != nil {
		t.Fatalf("after Never the site was offered on: %+v", o)
	}
	if o := went("d6", "asha", "yet another"); o == nil || !o.Update {
		t.Fatalf("after Never a saved login's new password wasn't offered: %+v", o)
	}
	// The site is on the list of those never offered on, and off it is offered on again.
	if sites, _ := s.m.NeverSites(); len(sites) != 1 || sites[0] != "https://shop.example" {
		t.Fatalf("the sites never offered on: %v", sites)
	}
	if err := s.m.Allow("https://shop.example"); err != nil {
		t.Fatal(err)
	}
	if sites, _ := s.m.NeverSites(); len(sites) != 0 {
		t.Fatalf("after Allow: %v", sites)
	}
	if o := went("d7", "work", "second one"); o == nil || o.Update {
		t.Fatalf("a site taken off the list wasn't offered on: %+v", o)
	}
}

// A sign-in in two steps: the username sent alone is kept for the password that follows on
// the same site. And where nothing is saved from: a frame with no origin of its own, a
// sign-in under a certificate that doesn't check, and a single letter, as a box of a code.
func TestTwoStepsAndWhereNothingIsSaved(t *testing.T) {
	s := newSaving(t)
	first := forms.Fields{Fields: []forms.Field{{Ref: "u", Form: "F", Tag: "input", Type: "email", Name: "identifier", Autocomplete: "username", Guess: "username", Visible: true, Focusable: true}}}
	second := forms.Fields{Fields: []forms.Field{{Ref: "p", Form: "F", Tag: "input", Type: "password", Name: "password", Guess: "password", Visible: true, Focusable: true}}}
	one := shop
	one.Document = "step-1"
	s.tell(one, "fields", first)
	s.tell(one, "submitted", forms.Sent{Form: "F", Values: map[string]string{"u": "asha@example.org"}})
	s.goesOn("7")
	if o := s.offer(); o != nil {
		t.Fatalf("a username alone offered %+v", o)
	}
	two := shop
	two.Document = "step-2"
	s.tell(two, "fields", second)
	s.tell(two, "submitted", forms.Sent{Form: "F", Values: map[string]string{"p": "correct horse"}})
	s.goesOn("7")
	if o := s.offer(); o == nil || o.Username != "asha@example.org" {
		t.Fatalf("the password of a sign-in in two steps: %+v", o)
	}
	s.f.Left("7", "")
	if last := s.offers[len(s.offers)-1]; last != nil {
		t.Fatalf("a closed tab's offer stayed: %+v", last)
	}

	for name, change := range map[string]func(*forms.Stamp, *string){
		"a frame with no origin of its own":       func(st *forms.Stamp, _ *string) { st.Opaque = true },
		"a certificate that doesn't check":        func(st *forms.Stamp, _ *string) { st.Secure = false },
		"one letter, as a box of a code split up": func(_ *forms.Stamp, p *string) { *p = "7" },
	} {
		n := len(s.offers)
		at := shop
		at.Tab, at.Document = name, name
		password := "correct horse"
		change(&at, &password)
		s.tell(at, "fields", login)
		s.tell(at, "submitted", forms.Sent{Form: "F1", Values: map[string]string{"f1": "asha", "f2": password}})
		s.goesOn(name)
		if s.offer(); len(s.offers) != n {
			t.Errorf("%s was offered to save: %+v", name, s.offers[len(s.offers)-1])
		}
	}
}
