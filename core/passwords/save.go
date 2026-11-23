package passwords

import (
	"database/sql"
	"sort"
	"strings"
	"time"

	"projectb/forms"
	"projectb/store"
	"projectb/timeline"
)

// Saving (docs/password-autofill.md): a form that is sent proves nothing, so Go only notes
// what it held, and offers to save once the sign-in is seen to go through: its form went
// away with the page staying, or the tab went on to another page that, once loaded, shows
// no sign-in again. What a person typed is kept in memory for the offer alone, and no
// longer.

// Add saves a login a person agreed to save, and gives its id. A site they once said never
// to offer on is offered on again.
func (m *Manager) Add(origin, username, password string) (string, error) {
	if err := m.ready(); err != nil {
		return "", err
	}
	id := store.NewID()
	err := m.st.Wait(func(tx *store.Tx) error {
		sealed, err := m.keys.Seal("logins", id, "password", []byte(password), origin)
		if err != nil {
			return err
		}
		now := store.Now()
		if _, err := tx.Exec(`INSERT INTO logins (id, origin, username, password, created, edited, used) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, origin, username, sealed, now, now, now); err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM logins_never WHERE origin = ?`, origin)
		return err
	})
	if err == nil {
		m.changed()
	}
	return id, err
}

// Change gives a saved login a new password.
func (m *Manager) Change(id, password string) error {
	if err := m.ready(); err != nil {
		return err
	}
	err := m.st.Wait(func(tx *store.Tx) error {
		var origin string
		if err := tx.QueryRow(`SELECT origin FROM logins WHERE id = ?`, id).Scan(&origin); err != nil {
			if err == sql.ErrNoRows {
				return ErrNone
			}
			return err
		}
		sealed, err := m.keys.Seal("logins", id, "password", []byte(password), origin)
		if err != nil {
			return err
		}
		now := store.Now()
		_, err = tx.Exec(`UPDATE logins SET password = ?, edited = ?, used = ? WHERE id = ?`, sealed, now, now, id)
		return err
	})
	if err == nil {
		m.changed()
	}
	return err
}

// Never has saving never offered on a site again, until a login is saved for it or the
// person takes the site off the list (Allow).
func (m *Manager) Never(origin string) error {
	err := m.st.Wait(func(tx *store.Tx) error {
		_, err := tx.Exec(`INSERT OR REPLACE INTO logins_never (origin, at) VALUES (?, ?)`, origin, store.Now())
		return err
	})
	if err == nil {
		m.changed()
	}
	return err
}

// Allow has saving offered on a site again.
func (m *Manager) Allow(origin string) error {
	err := m.st.Wait(func(tx *store.Tx) error {
		_, err := tx.Exec(`DELETE FROM logins_never WHERE origin = ?`, origin)
		return err
	})
	if err == nil {
		m.changed()
	}
	return err
}

// NeverSites is the origins a person said never to offer to save on, in order.
func (m *Manager) NeverSites() ([]string, error) {
	out := []string{}
	err := m.st.Wait(func(tx *store.Tx) error {
		out = out[:0]
		r, err := tx.Query(`SELECT origin FROM logins_never ORDER BY origin`)
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			var o string
			if err := r.Scan(&o); err != nil {
				return err
			}
			out = append(out, o)
		}
		return r.Err()
	})
	return out, err
}

// IsNever is whether a person said never to offer to save on a site.
func (m *Manager) IsNever(origin string) (bool, error) {
	n := 0
	err := m.st.Wait(func(tx *store.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM logins_never WHERE origin = ?`, origin).Scan(&n)
	})
	return n > 0, err
}

// held is the logins saved for an origin, each with its password opened: what a sign-in
// that went through is compared with.
type held struct {
	Login
	password string
}

func (m *Manager) heldFor(origin string) ([]held, error) {
	var out []held
	err := m.st.Wait(func(tx *store.Tx) error {
		out = nil
		r, err := tx.Query(`SELECT id, username, password, created, used FROM logins WHERE origin = ? ORDER BY used DESC, created DESC`, origin)
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			h := held{Login: Login{Origin: origin}}
			var sealed []byte
			if err := r.Scan(&h.ID, &h.Username, &sealed, &h.Created, &h.Used); err != nil {
				return err
			}
			plain, err := m.keys.Open("logins", h.ID, "password", sealed, origin)
			if err != nil {
				return err
			}
			h.password = string(plain)
			out = append(out, h)
		}
		return r.Err()
	})
	return out, err
}

// SaveOffer is the offer to save a login a person just signed in with, or to update a saved
// one's password, for the shell to ask. It holds no password.
type SaveOffer struct {
	ID  int64  `json:"id"`
	Tab string `json:"tab"`
	// Site is the host the login is for.
	Site     string `json:"site"`
	Username string `json:"username"`
	// Update is an offer to change the password of a login saved for that username.
	Update bool `json:"update,omitempty"`
}

// Settle is how long a page a tab went on to has, once it is loaded, to show a sign-in
// again, which says the one before it failed, before that one is taken to have gone
// through: a page may draw its form a moment after it loads.
var Settle = 2 * time.Second

// usernameFor is how long a username sent alone, as the first step of a sign-in, is kept
// for the password that follows on the same site.
const usernameFor = 5 * time.Minute

// sent is what a form held when it was sent, waiting to be seen to go through.
type sent struct {
	id       int64
	stamp    forms.Stamp
	username string
	password string
	// update is the saved login whose password this would change, "" for a new one.
	update string
	// at is the site the tab was on when the offer was made: the offer stays while the tab
	// stays there.
	at string
	// moved is the tab having gone on since, settling the page it came to having loaded,
	// and offered the offer having been made.
	moved, settling, offered bool
}

// typed is a username sent alone.
type typed struct {
	value string
	at    time.Time
}

// OnSave has saving told each offer to make, and nil when the one made is no more. Set
// once, before any message.
func (f *Filler) OnSave(saving func(*SaveOffer)) { f.saving = saving }

// watch asks the page side for the values of the fields Go took for a role, when their form
// is sent or goes: the username and the passwords, and no other field's.
func (f *Filler) watch(stamp forms.Stamp, found []forms.Found) {
	w := forms.Watch{Refs: []string{}}
	for _, fd := range found {
		for ref, role := range fd.Roles {
			if role != forms.OneTimeCode {
				w.Refs = append(w.Refs, ref)
			}
		}
	}
	if len(w.Refs) > 0 {
		sort.Strings(w.Refs)
		f.carry(stamp, "watch", w)
	}
}

// formSent notes what a form held as it was sent. A username alone is kept for the
// password to come; a password is what may be saved, once it is seen to have worked.
func (f *Filler) formSent(stamp forms.Stamp, s forms.Sent, gone bool) {
	f.mu.Lock()
	pg := f.pages[key(stamp)]
	if pg == nil || pg.stamp.Document != stamp.Document {
		f.mu.Unlock()
		return
	}
	_, _, site := siteOf(stamp.Origin)
	var note *sent
	for _, fd := range forms.Roles(pg.fields, s.Values) {
		if fd.Form != s.Form {
			continue
		}
		username := strings.TrimSpace(s.Values[fd.Username])
		if fd.Kind == forms.UsernameOnly {
			if username != "" {
				f.usernames[stamp.Tab+"\x00"+site] = typed{username, time.Now()}
			}
			continue
		}
		password := s.Values[fd.New]
		if password == "" {
			password = s.Values[fd.Current]
		}
		// One letter is a box of a code split in pieces, not a password.
		if len([]rune(password)) < 2 {
			continue
		}
		if username == "" {
			if t, ok := f.usernames[stamp.Tab+"\x00"+site]; ok && time.Since(t.at) < usernameFor {
				username = t.value
			}
		}
		f.next++
		note = &sent{id: f.next, stamp: stamp, username: username, password: password}
	}
	// A new sign-in takes the place of the one before it, and of its offer if it had one.
	replaced := false
	if note != nil {
		replaced = f.sent[stamp.Tab] != nil && f.sent[stamp.Tab].offered
		f.sent[stamp.Tab] = note
	}
	now := f.sent[stamp.Tab]
	f.mu.Unlock()
	if replaced && f.saving != nil {
		f.saving(nil)
	}
	// A form that went with the page staying has gone through.
	if gone && now != nil && now.stamp == stamp {
		f.wentThrough(stamp.Tab, now.id)
	}
}

// placeOf is the site of a page's address, or its host where it is of no site, as an
// address by number is.
func placeOf(address string) string {
	_, host, site := siteOf(address)
	if site != "" {
		return site
	}
	return host
}

// moved is a tab going on to the page at an address. What it sent before waits for that
// page to load. An offer made already stays while the tab stays on the site it was made
// on, and goes when the tab goes to another: it mustn't follow a person across sites.
func (f *Filler) moved(tab, address string) {
	f.mu.Lock()
	f.where[tab] = placeOf(address)
	s := f.sent[tab]
	left := s != nil && s.offered && s.at != f.where[tab]
	if left {
		delete(f.sent, tab)
	} else if s != nil {
		s.moved = true
	}
	f.mu.Unlock()
	if left && f.saving != nil {
		f.saving(nil)
	}
}

// Loaded is a tab's page done loading. What the tab sent before it went on is taken to have
// gone through, unless the page it came to shows a sign-in again before long.
func (f *Filler) Loaded(tab string) {
	f.mu.Lock()
	s := f.sent[tab]
	if s == nil || !s.moved || s.settling || s.offered {
		f.mu.Unlock()
		return
	}
	s.settling = true
	id := s.id
	f.mu.Unlock()
	time.AfterFunc(Settle, func() { f.wentThrough(tab, id) })
}

// again is a tab's own page showing a sign-in after what was sent: that sign-in failed.
func (f *Filler) again(stamp forms.Stamp, found []forms.Found) {
	if !stamp.Top {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.sent[stamp.Tab]
	if s == nil || !s.moved || s.offered || s.stamp.Document == stamp.Document {
		return
	}
	for _, fd := range found {
		if fd.Kind == forms.SignIn && fd.Current != "" {
			delete(f.sent, stamp.Tab)
			return
		}
	}
}

// wentThrough offers to save what a tab sent, now that it is seen to have worked: nothing
// for a login saved as it is, an update for a username saved with another password, and
// a save otherwise. Nothing on a site a person said never to offer on, or in a frame that
// is never filled.
func (f *Filler) wentThrough(tab string, id int64) {
	f.mu.Lock()
	s := f.sent[tab]
	if s == nil || s.id != id || s.offered {
		f.mu.Unlock()
		return
	}
	f.mu.Unlock()
	drop := func() {
		f.mu.Lock()
		if f.sent[tab] == s {
			delete(f.sent, tab)
		}
		f.mu.Unlock()
	}
	origin := s.stamp.Origin
	if !MayFill(s.stamp) {
		drop()
		return
	}
	saved, err := f.m.heldFor(origin)
	if err != nil {
		timeline.Errorf("passwords: the logins for %s: %v", origin, err)
		drop()
		return
	}
	update := ""
	for _, h := range saved {
		if h.Username != s.username && s.username != "" {
			continue
		}
		// A form with no username, as one that changes a password, is the one login saved
		// for the site, when there is one.
		if s.username == "" && len(saved) != 1 {
			continue
		}
		if h.password == s.password {
			f.m.used(h.ID)
			drop()
			return
		}
		if update == "" {
			update = h.ID
			s.username = h.Username
		}
	}
	if never, _ := f.m.IsNever(origin); never && update == "" {
		drop()
		return
	}
	_, host, _ := siteOf(origin)
	f.mu.Lock()
	if f.sent[tab] != s {
		f.mu.Unlock()
		return
	}
	s.offered, s.update, s.at = true, update, f.where[tab]
	offer := SaveOffer{ID: s.id, Tab: tab, Site: host, Username: s.username, Update: update != ""}
	f.mu.Unlock()
	if f.saving != nil {
		f.saving(&offer)
	}
}

// Typed is the password a save offer would save, for the person to look at before they
// agree: they typed it a moment ago. "" when the offer has gone.
func (f *Filler) Typed(offer int64) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sent {
		if s.id == offer && s.offered {
			return s.password
		}
	}
	return ""
}

// Answer is the person's answer to a save offer: "save" saves the login, or gives the
// saved one its new password; "never" has the site never offered on again; anything else
// only takes the offer away. Not on the main thread.
func (f *Filler) Answer(offer int64, choice string) error {
	f.mu.Lock()
	var s *sent
	for tab, x := range f.sent {
		if x.id == offer && x.offered {
			s = x
			delete(f.sent, tab)
		}
	}
	f.mu.Unlock()
	if s == nil {
		return ErrNoOffer
	}
	if f.saving != nil {
		f.saving(nil)
	}
	switch choice {
	case "save":
		if s.update != "" {
			return f.m.Change(s.update, s.password)
		}
		_, err := f.m.Add(s.stamp.Origin, s.username, s.password)
		return err
	case "never":
		return f.m.Never(s.stamp.Origin)
	}
	return nil
}
