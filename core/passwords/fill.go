package passwords

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
	"sync"

	"golang.org/x/net/publicsuffix"

	"projectb/forms"
	"projectb/store"
	"projectb/timeline"
)

// Filling (docs/password-autofill.md): a form script in each frame tells what fields a
// page has and which one the person pressed; Go finds the sign-in among them, offers the
// logins saved for that frame's own site, and fills the one the person picks. Nothing is
// filled until they pick, nothing is offered in a frame that mustn't be filled, and only
// the carrier's stamp says where a message came from.

// Match is a saved login offered in a frame.
type Match struct {
	Login
	// SavedFor is the host the login was saved for, when it isn't the frame's own: another
	// host of the same site. "" for the frame's own.
	SavedFor string `json:"savedFor,omitempty"`
}

// Offer is the logins offered at a field the person pressed, for the shell to list beside
// it. It holds no password.
type Offer struct {
	// ID names the offer, for the pick that answers it.
	ID  int64  `json:"id"`
	Tab string `json:"tab"`
	// Box is the field, in the tab's page.
	Box forms.Box `json:"box"`
	// Site is the frame's own host, and Framed whether that frame is one of another origin
	// than the frames above it, when the list says whose logins these are.
	Site   string `json:"site"`
	Framed bool   `json:"framed,omitempty"`
	// Field is the role of the field pressed: "username" or "current-password".
	Field  forms.Role `json:"field"`
	Logins []Match    `json:"logins"`
	// At is the login the arrow keys are at, from the page's field: its place in Logins, or
	// -1 for none.
	At int `json:"at"`
}

// Carrier sends a message of the page protocol to the form script of the document a stamp
// names. False says that document has gone.
type Carrier func(to forms.Stamp, kind string, payload any) bool

// Filler is one profile's filling. Its calls may come on any thread; what it tells, the
// offers, it tells on the thread post gives it.
type Filler struct {
	m       *Manager
	carry   Carrier
	offered func(*Offer)

	// saving is told each offer to save, and nil when it is no more (save.go).
	saving func(*SaveOffer)

	mu    sync.Mutex
	next  int64
	pages map[string]*page // by tab and frame
	open  *showing
	// sent is what each tab's last form held when it was sent, until it is seen to go
	// through and is answered; usernames the usernames sent alone, by tab and site.
	sent      map[string]*sent
	usernames map[string]typed
	// where is the site each tab is on, as its last address said.
	where map[string]string
}

// page is what a frame's document has told.
type page struct {
	stamp  forms.Stamp
	fields forms.Fields
	found  []forms.Found
}

// showing is the offer showing: where it is, the field it is at, and what a pick of it
// fills.
type showing struct {
	offer Offer
	stamp forms.Stamp
	ref   string
	found forms.Found
}

// NewFiller is filling over m's logins. carry reaches a frame's form script; offered is told
// each offer to show, and nil when the one showing is no more. Both are called on whatever
// thread the Filler's own call came on, or a worker's: the caller posts where it must.
func NewFiller(m *Manager, carry Carrier, offered func(*Offer)) *Filler {
	return &Filler{m: m, carry: carry, offered: offered, pages: map[string]*page{}, sent: map[string]*sent{}, usernames: map[string]typed{}, where: map[string]string{}}
}

func key(s forms.Stamp) string { return s.Tab + "\x00" + s.Frame }

// MayFill is whether anything may be filled in a frame, by the carrier's facts alone: not in
// a sandboxed frame or one with no origin of its own, and not over https with a certificate
// that doesn't check.
func MayFill(s forms.Stamp) bool {
	return !s.Opaque && (s.Secure || strings.HasPrefix(s.Origin, "http://"))
}

// Message takes a message of the page protocol from a frame's form script.
func (f *Filler) Message(stamp forms.Stamp, kind string, payload []byte) {
	switch kind {
	case "fields":
		var fields forms.Fields
		if json.Unmarshal(payload, &fields) != nil {
			return
		}
		found := forms.Roles(fields, nil)
		f.mu.Lock()
		f.pages[key(stamp)] = &page{stamp: stamp, fields: fields, found: found}
		f.mu.Unlock()
		f.again(stamp, found)
		f.watch(stamp, found)
	case "submitted", "gone":
		var s forms.Sent
		if json.Unmarshal(payload, &s) != nil {
			return
		}
		f.formSent(stamp, s, kind == "gone")
	case "pressed":
		var p forms.Pressed
		if json.Unmarshal(payload, &p) != nil {
			return
		}
		f.pressed(stamp, p)
	case "filled":
		f.close(func(o *showing) bool { return key(o.stamp) == key(stamp) })
	case "key":
		var k forms.Key
		if json.Unmarshal(payload, &k) != nil {
			return
		}
		f.keyed(stamp, k)
	}
}

// keyed follows a key the person pressed in the field a list is offered at: the arrows
// move through its logins, round and round; Enter fills the one they are at; Escape takes
// the list away.
func (f *Filler) keyed(stamp forms.Stamp, k forms.Key) {
	f.mu.Lock()
	o := f.open
	if o == nil || o.stamp != stamp || o.ref != k.Ref {
		f.mu.Unlock()
		return
	}
	n := len(o.offer.Logins)
	switch k.Key {
	case "ArrowDown":
		o.offer.At = (o.offer.At + 1) % n
	case "ArrowUp":
		o.offer.At = (o.offer.At + n) % (n + 1)
		if o.offer.At == n {
			o.offer.At = n - 1
		}
	}
	offer := o.offer
	f.mu.Unlock()
	switch k.Key {
	case "ArrowDown", "ArrowUp":
		f.offered(&offer)
	case "Escape":
		f.Dismiss(offer.ID)
	case "Enter":
		if offer.At >= 0 {
			if err := f.Pick(offer.ID, offer.Logins[offer.At].ID); err != nil && !errors.Is(err, ErrNoOffer) {
				timeline.Errorf("passwords: filling: %v", err)
			}
		}
	}
}

// pressed offers the frame's logins when the field pressed is a sign-in's username or
// password, and takes the offer away when it is anything else.
func (f *Filler) pressed(stamp forms.Stamp, p forms.Pressed) {
	f.mu.Lock()
	pg := f.pages[key(stamp)]
	var found forms.Found
	var role forms.Role
	if pg != nil && pg.stamp.Document == stamp.Document {
		for _, fd := range pg.found {
			// A new password is never filled from what is saved.
			if r := fd.Roles[p.Ref]; (r == forms.Username || r == forms.CurrentPassword) && fd.Kind != forms.SignUp && fd.Kind != forms.Reset {
				found, role = fd, r
			}
		}
	}
	f.mu.Unlock()
	if role == "" || !MayFill(stamp) {
		f.close(func(o *showing) bool { return o.stamp.Tab == stamp.Tab })
		return
	}
	logins, err := f.m.Matching(stamp.Origin)
	if err != nil {
		timeline.Errorf("passwords: the logins for %s: %v", stamp.Origin, err)
	}
	if len(logins) == 0 {
		f.close(func(o *showing) bool { return o.stamp.Tab == stamp.Tab })
		return
	}
	f.mu.Lock()
	f.next++
	site := stamp.Origin
	if u, err := url.Parse(stamp.Origin); err == nil && u.Host != "" {
		site = u.Host
	}
	was := f.open
	o := &showing{stamp: stamp, ref: p.Ref, found: found, offer: Offer{
		ID: f.next, Tab: stamp.Tab, Site: site, Framed: stamp.CrossOrigin, Field: role, Logins: logins, At: -1,
		Box: forms.Box{X: p.Box.X, Y: p.Box.Y, W: p.Box.W, H: p.Box.H},
	}}
	f.open = o
	f.mu.Unlock()
	// The field's arrow keys, Enter and Escape are the list's while it shows, and no
	// field's once it has gone.
	if was != nil && (was.stamp != stamp || was.ref != p.Ref) {
		f.carry(was.stamp, "keys", forms.Keys{})
	}
	f.carry(stamp, "keys", forms.Keys{Ref: p.Ref})
	offer := o.offer
	f.offered(&offer)
}

// close takes the offer showing away, when it is one that gone names.
func (f *Filler) close(gone func(*showing) bool) {
	f.mu.Lock()
	was := f.open
	if was == nil || !gone(was) {
		f.mu.Unlock()
		return
	}
	f.open = nil
	f.mu.Unlock()
	f.carry(was.stamp, "keys", forms.Keys{})
	f.offered(nil)
}

// At sets the login an offer's list is at, as the pointer does when it moves over one: the
// list has one current login, whichever of the keys and the pointer moved last, and Enter
// fills it. -1 is none.
func (f *Filler) At(offer int64, at int) {
	f.mu.Lock()
	o := f.open
	if o == nil || o.offer.ID != offer || at < -1 || at >= len(o.offer.Logins) || o.offer.At == at {
		f.mu.Unlock()
		return
	}
	o.offer.At = at
	now := o.offer
	f.mu.Unlock()
	f.offered(&now)
}

// Dismiss takes the offer away, as the person's Escape or a press elsewhere does.
func (f *Filler) Dismiss(offer int64) {
	f.close(func(o *showing) bool { return o.offer.ID == offer })
}

// Navigated takes a tab's offer away: its page went on to another address. What its
// frames told stays, as an address can change with the page staying.
func (f *Filler) Navigated(tab, address string) {
	f.close(func(o *showing) bool { return o.stamp.Tab == tab })
	f.moved(tab, address)
}

// Left forgets what a tab's frames told, and takes its offer away: the tab closed, or a
// frame of it went. frame is one frame of it, or "" for all.
func (f *Filler) Left(tab, frame string) {
	f.mu.Lock()
	for k, pg := range f.pages {
		if pg.stamp.Tab == tab && (frame == "" || pg.stamp.Frame == frame) {
			delete(f.pages, k)
		}
	}
	// A closed tab's offer to save goes with it, and what it sent is forgotten.
	gone := frame == "" && f.sent[tab] != nil && f.sent[tab].offered
	if frame == "" {
		delete(f.sent, tab)
		delete(f.where, tab)
		for k := range f.usernames {
			if strings.HasPrefix(k, tab+"\x00") {
				delete(f.usernames, k)
			}
		}
	}
	f.mu.Unlock()
	if gone && f.saving != nil {
		f.saving(nil)
	}
	f.close(func(o *showing) bool { return o.stamp.Tab == tab && (frame == "" || o.stamp.Frame == frame) })
}

// ErrNoOffer: the offer a pick answers isn't showing any more.
var ErrNoOffer = errors.New("passwords: that offer has gone")

// Pick fills the login the person picked from an offer: its username, where the form has a
// field for one, and its password, where it has one for the current password. The password
// is opened with no check of the person: a fill asks nothing, and a locked profile is
// asked for by the call that opens it. Not on the main thread.
func (f *Filler) Pick(offer int64, login string) error {
	f.mu.Lock()
	o := f.open
	f.mu.Unlock()
	if o == nil || o.offer.ID != offer {
		return ErrNoOffer
	}
	var picked *Match
	for i := range o.offer.Logins {
		if o.offer.Logins[i].ID == login {
			picked = &o.offer.Logins[i]
		}
	}
	if picked == nil {
		return ErrNone
	}
	fill := forms.Fill{Values: map[string]string{}}
	if o.found.Username != "" {
		fill.Values[o.found.Username] = picked.Username
	}
	if o.found.Current != "" {
		password, err := f.m.Password(Checked(context.Background()), login)
		if err != nil {
			return err
		}
		fill.Values[o.found.Current] = password
	}
	f.close(func(now *showing) bool { return now.offer.ID == offer })
	if !f.carry(o.stamp, "fill", fill) {
		return ErrNoOffer
	}
	return f.m.used(login)
}

// used notes that a login was used now.
func (m *Manager) used(id string) error {
	return m.st.Wait(func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE logins SET used = ? WHERE id = ?`, store.Now(), id)
		return err
	})
}

// siteOf is an origin's host and the site it is of: its registrable domain by the public
// suffix list, the private part too, so two people's pages on one shared host are two sites.
func siteOf(origin string) (scheme, host, site string) {
	u, err := url.Parse(origin)
	if err != nil {
		return "", "", ""
	}
	site, _ = publicsuffix.EffectiveTLDPlusOne(u.Hostname())
	return u.Scheme, u.Host, site
}

// Matching is the logins offered in a frame of origin, the ones used last first: those
// saved for that origin; over https, one saved for the same host before it had https; and
// then those saved for another https host of the same site, each said to be saved for its
// own host. A login saved over https is never offered on a page that isn't.
func (m *Manager) Matching(origin string) ([]Match, error) {
	scheme, host, site := siteOf(origin)
	if host == "" || (scheme != "https" && scheme != "http") {
		return nil, nil
	}
	all, err := m.List()
	if err != nil {
		return nil, err
	}
	var own, others []Match
	for _, l := range all {
		ls, lh, lsite := siteOf(l.Origin)
		switch {
		case l.Origin == origin:
			own = append(own, Match{Login: l})
		case scheme == "https" && ls == "http" && lh == host:
			own = append(own, Match{Login: l})
		case scheme == "https" && ls == "https" && site != "" && lsite == site:
			others = append(others, Match{Login: l, SavedFor: lh})
		}
	}
	recent := func(list []Match) {
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Used != list[j].Used {
				return list[i].Used > list[j].Used
			}
			return list[i].Created > list[j].Created
		})
	}
	recent(own)
	recent(others)
	return append(own, others...), nil
}
