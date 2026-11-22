package passwords

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/sudiptadeb/project-b/browserdata"

	"projectb/forms"
)

// filling is a profile with a Filler over it: what it sent to form scripts, and the offers
// it told, nil for one taken away.
type filling struct {
	*profile
	f      *Filler
	sent   []sentForm
	offers []*Offer
	// gone makes the carrier say a document has gone.
	gone bool
}

type sentForm struct {
	to      forms.Stamp
	kind    string
	payload string
}

func newFilling(t *testing.T, logins ...browserdata.Login) *filling {
	t.Helper()
	x := &filling{profile: open(t, t.TempDir())}
	x.m.Import(logins)
	x.f = NewFiller(x.m, func(to forms.Stamp, kind string, payload any) bool {
		if x.gone {
			return false
		}
		b, _ := json.Marshal(payload)
		x.sent = append(x.sent, sentForm{to, kind, string(b)})
		return true
	}, func(o *Offer) { x.offers = append(x.offers, o) })
	return x
}

// of is what Go sent of one kind.
func (x *filling) of(kind string) []sentForm {
	var out []sentForm
	for _, s := range x.sent {
		if s.kind == kind {
			out = append(out, s)
		}
	}
	return out
}

// tell plays a frame's form script telling Go something.
func (x *filling) tell(s forms.Stamp, kind string, payload any) {
	b, _ := json.Marshal(payload)
	x.f.Message(s, kind, b)
}

// last is the offer showing, nil for none.
func (x *filling) last() *Offer {
	if len(x.offers) == 0 {
		return nil
	}
	return x.offers[len(x.offers)-1]
}

func usernames(o *Offer) []string {
	out := []string{}
	if o != nil {
		for _, l := range o.Logins {
			out = append(out, l.Username+"@"+l.SavedFor)
		}
	}
	return out
}

var (
	shop  = forms.Stamp{Tab: "7", Frame: "main", Document: "d1", Origin: "https://shop.example", Top: true, Secure: true}
	login = forms.Fields{Fields: []forms.Field{
		{Ref: "f1", Form: "F1", Tag: "input", Type: "email", Name: "email", Visible: true, Focusable: true, Empty: true},
		{Ref: "f2", Form: "F1", Tag: "input", Type: "password", Name: "password", Visible: true, Focusable: true, Empty: true},
		{Ref: "f3", Tag: "input", Type: "search", Name: "q", Visible: true, Focusable: true, Empty: true},
	}}
)

// A press on a sign-in's field offers the logins saved for that frame's site, with no
// password; a pick fills the username and the password into the fields Go found, with no
// check of the person; and the offer goes once it is filled.
func TestAPressOffersAndAPickFills(t *testing.T) {
	x := newFilling(t,
		browserdata.Login{Origin: "https://shop.example", Username: "asha", Password: "correct horse"},
		browserdata.Login{Origin: "https://shop.example", Username: "work", Password: "second one"},
		browserdata.Login{Origin: "https://other.example", Username: "elsewhere", Password: "never here"})
	x.tell(shop, "fields", login)
	if len(x.sent) != 0 {
		t.Fatalf("Go asked a page for something before any press: %+v", x.sent)
	}
	keys := func() string { return x.of("keys")[len(x.of("keys"))-1].payload }
	x.tell(shop, "pressed", forms.Pressed{Ref: "f2", Box: forms.Box{X: 10, Y: 20, W: 200, H: 30}})
	o := x.last()
	if o == nil || o.Tab != "7" || o.Site != "shop.example" || o.Framed || o.Field != forms.CurrentPassword || o.Box != (forms.Box{X: 10, Y: 20, W: 200, H: 30}) ||
		!slices.Equal(usernames(o), []string{"asha@", "work@"}) {
		t.Fatalf("the offer: %+v", o)
	}
	if b, _ := json.Marshal(o); strings.Contains(string(b), "correct horse") || strings.Contains(string(b), "second one") {
		t.Fatalf("the offer holds a password: %s", b)
	}
	if len(p(x).asked) != 0 {
		t.Fatal("an offer had the device check the person")
	}
	// While it shows, the field's arrow keys, Enter and Escape are the list's.
	if o.At != -1 || keys() != `{"ref":"f2"}` {
		t.Fatalf("the offer starts at %d, and the keys asked for are %s", o.At, keys())
	}
	if err := x.f.Pick(o.ID, o.Logins[1].ID); err != nil {
		t.Fatal(err)
	}
	if fills := x.of("fill"); len(fills) != 1 || fills[0].to != shop || fills[0].payload != `{"values":{"f1":"work","f2":"second one"}}` {
		t.Fatalf("the fill: %+v", x.sent)
	}
	if keys() != `{"ref":""}` {
		t.Fatalf("the field's keys are still asked for after the fill: %s", keys())
	}
	if len(p(x).asked) != 0 {
		t.Fatal("a fill had the device check the person")
	}
	if x.last() != nil {
		t.Fatal("the offer still shows after its pick")
	}
	// The login filled is the one used last, and so comes first next time.
	x.tell(shop, "pressed", forms.Pressed{Ref: "f1"})
	if o := x.last(); o == nil || o.Field != forms.Username || !slices.Equal(usernames(o), []string{"work@", "asha@"}) {
		t.Fatalf("the next offer: %+v", o)
	}
	// A pick of an offer that has gone fills nothing.
	old := x.last().ID
	x.f.Dismiss(old)
	if err := x.f.Pick(old, "x"); !errors.Is(err, ErrNoOffer) || len(x.of("fill")) != 1 {
		t.Fatalf("a pick of an offer taken away: %v, %d fills", err, len(x.of("fill")))
	}
}

// The arrow keys, pressed in the page's field, move through the offered logins and round;
// Enter fills the one they are at, and nothing while they are at none; Escape takes the
// list away. Keys from another field, or from a page the offer isn't for, do nothing.
func TestTheFieldsKeysMoveThroughTheList(t *testing.T) {
	x := newFilling(t,
		browserdata.Login{Origin: "https://shop.example", Username: "asha", Password: "correct horse"},
		browserdata.Login{Origin: "https://shop.example", Username: "work", Password: "second one"})
	x.tell(shop, "fields", login)
	x.tell(shop, "pressed", forms.Pressed{Ref: "f1"})
	press := func(s forms.Stamp, ref, k string) { x.tell(s, "key", forms.Key{Ref: ref, Key: k}) }
	at := func() int {
		if x.last() == nil {
			return -99
		}
		return x.last().At
	}
	id := x.last().ID
	press(shop, "f1", "Enter")
	if len(x.of("fill")) != 0 || x.last() == nil {
		t.Fatal("Enter with no login chosen filled or closed the list")
	}
	for _, step := range []struct {
		key  string
		want int
	}{{"ArrowDown", 0}, {"ArrowDown", 1}, {"ArrowDown", 0}, {"ArrowUp", 1}, {"ArrowUp", 0}} {
		press(shop, "f1", step.key)
		if at() != step.want || x.last().ID != id {
			t.Fatalf("after %s the list is at %d, want %d", step.key, at(), step.want)
		}
	}
	// Keys that aren't this field's, or this document's, are nobody's.
	other := shop
	other.Document = "d2"
	press(shop, "f2", "ArrowDown")
	press(other, "f1", "ArrowDown")
	if at() != 0 {
		t.Fatalf("a key from elsewhere moved the list to %d", at())
	}
	press(shop, "f1", "ArrowDown")
	press(shop, "f1", "Enter")
	if fills := x.of("fill"); len(fills) != 1 || fills[0].payload != `{"values":{"f1":"asha","f2":"correct horse"}}` && fills[0].payload != `{"values":{"f1":"work","f2":"second one"}}` {
		t.Fatalf("Enter filled %+v", fills)
	}
	if x.last() != nil {
		t.Fatal("the list stayed after Enter filled")
	}
	// ArrowUp from none goes to the last; Escape takes the list away.
	x.tell(shop, "pressed", forms.Pressed{Ref: "f1"})
	press(shop, "f1", "ArrowUp")
	if at() != 1 {
		t.Fatalf("ArrowUp from none is at %d, want the last", at())
	}
	press(shop, "f1", "Escape")
	if x.last() != nil || len(x.of("fill")) != 1 {
		t.Fatal("Escape left the list, or filled")
	}
}

func p(x *filling) *profile { return x.profile }

// What takes an offer away, and where none is made.
func TestWhereNothingIsOffered(t *testing.T) {
	x := newFilling(t, browserdata.Login{Origin: "https://shop.example", Username: "asha", Password: "correct horse"})
	press := func(s forms.Stamp, ref string) *Offer {
		t.Helper()
		before := len(x.offers)
		x.tell(s, "pressed", forms.Pressed{Ref: ref})
		if len(x.offers) == before {
			return nil
		}
		return x.last()
	}
	x.tell(shop, "fields", login)
	if o := press(shop, "f3"); o != nil {
		t.Fatalf("a press on a search field offered %+v", o)
	}
	if o := press(shop, "f1"); o == nil {
		t.Fatal("a press on the username offered nothing")
	}
	if o := press(shop, "f3"); o != nil {
		t.Fatalf("a press elsewhere left the offer: %+v", o)
	}
	// A document that has gone: its fields are another page's, and nothing is offered on them.
	next := shop
	next.Document = "d2"
	if o := press(next, "f1"); o != nil {
		t.Fatalf("a field of a document Go was told nothing of: %+v", o)
	}
	// Frames that are never filled, by the carrier's word alone.
	for name, change := range map[string]func(*forms.Stamp){
		"a sandboxed frame":                    func(s *forms.Stamp) { s.Opaque = true },
		"a certificate that doesn't check":     func(s *forms.Stamp) { s.Secure = false },
		"the same host with no https":          func(s *forms.Stamp) { s.Origin, s.Secure = "http://shop.example", false },
		"a site with nothing saved":            func(s *forms.Stamp) { s.Origin = "https://nothing.example" },
		"a look-alike on a shared public host": func(s *forms.Stamp) { s.Origin = "https://shop.example.github.io" },
	} {
		s := shop
		s.Frame, s.Document = name, name
		change(&s)
		x.tell(s, "fields", login)
		if o := press(s, "f1"); o != nil {
			t.Errorf("%s was offered %v", name, usernames(o))
		}
	}
	// A sign-up's fields are never offered a saved login.
	signUp := forms.Fields{Fields: []forms.Field{
		{Ref: "u", Tag: "input", Type: "email", Name: "email", Autocomplete: "username", Visible: true, Focusable: true},
		{Ref: "n", Tag: "input", Type: "password", Name: "new", Autocomplete: "new-password", Visible: true, Focusable: true},
	}}
	s := shop
	s.Document = "d3"
	x.tell(s, "fields", signUp)
	if o := press(s, "u"); o != nil {
		t.Fatalf("a sign-up's username was offered %v", usernames(o))
	}
	// Leaving the page takes the offer with it.
	x.tell(s, "fields", login)
	if o := press(s, "f1"); o == nil {
		t.Fatal("no offer on the page again")
	}
	x.f.Left("7", "")
	if x.last() != nil {
		t.Fatal("the offer outlived its page")
	}
	// And a pick whose document has gone by the time it is filled says so.
	x.tell(s, "fields", login)
	o := press(s, "f1")
	x.gone = true
	if err := x.f.Pick(o.ID, o.Logins[0].ID); !errors.Is(err, ErrNoOffer) {
		t.Fatalf("a fill for a document that has gone: %v", err)
	}
}

// Which logins a frame is offered: its own origin's; over https one saved for the host
// before it had https; then other https hosts of the same site, each said to be saved for
// its host. Never an https login on a page that isn't, and never across a shared host.
func TestMatching(t *testing.T) {
	x := newFilling(t,
		browserdata.Login{Origin: "https://www.shop.example", Username: "own", Password: "a"},
		browserdata.Login{Origin: "http://www.shop.example", Username: "old-http", Password: "b"},
		browserdata.Login{Origin: "https://accounts.shop.example", Username: "sibling", Password: "c"},
		browserdata.Login{Origin: "https://www.shop.example:8443", Username: "other-port", Password: "d"},
		browserdata.Login{Origin: "https://asha.github.io", Username: "my-pages", Password: "e"},
		browserdata.Login{Origin: "https://unrelated.example", Username: "unrelated", Password: "f"})
	for origin, want := range map[string][]string{
		"https://www.shop.example":      {"own@", "old-http@", "sibling@accounts.shop.example", "other-port@www.shop.example:8443"},
		"https://accounts.shop.example": {"sibling@", "own@www.shop.example", "other-port@www.shop.example:8443"},
		"http://www.shop.example":       {"old-http@"},
		"https://asha.github.io":        {"my-pages@"},
		"https://mallory.github.io":     {},
		"https://shop.example.evil.com": {},
		"about:blank":                   {},
	} {
		got, err := x.m.Matching(origin)
		if err != nil {
			t.Fatal(err)
		}
		names := []string{}
		for _, m := range got {
			names = append(names, m.Username+"@"+m.SavedFor)
		}
		// Within a group the order is by use, which is the same for all here: compare as sets
		// within the own and the others.
		slices.Sort(names)
		w := slices.Clone(want)
		slices.Sort(w)
		if !slices.Equal(names, w) {
			t.Errorf("%s is offered %v, want %v", origin, names, want)
		}
	}
}
