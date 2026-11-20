package sync_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/sudiptadeb/project-b/browserdata"

	"projectb/passwords"
	"projectb/sync"
)

// logins is a profile's saved logins as "site username password", sorted.
func (p *profile) logins() []string {
	p.t.Helper()
	list, err := p.passwords.List()
	if err != nil {
		p.t.Fatal(err)
	}
	out := []string{}
	for _, l := range list {
		pw, err := p.passwords.Password(context.Background(), l.ID)
		if err != nil {
			p.t.Fatal(err)
		}
		out = append(out, l.Origin+" "+l.Username+" "+pw)
	}
	slices.Sort(out)
	return out
}

// Saved logins sync: brought to one device, they are on the other, each password opening
// there under that profile's own key; the server reads none of it; one deleted on one goes
// from the other.
func TestSavedLoginsSync(t *testing.T) {
	s := newServer(t)
	a, b := newProfile(t), newProfile(t)
	a.startSync(s)
	b.startSync(s)
	a.passwords.Import([]browserdata.Login{
		{Origin: "https://mail.example", Username: "asha", Password: "correct horse"},
		{Origin: "https://mail.example", Username: "asha", Password: "an older one"},
		{Origin: "https://bank.example", Username: "", Password: "staple battery"},
	})
	a.settle()
	if err := a.connect(s.bearer(), "tok", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	if err := b.connect(s.bearer(), "tok", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	b.settle()
	want := []string{"https://bank.example  staple battery", "https://mail.example asha an older one", "https://mail.example asha correct horse"}
	if got := b.logins(); !slices.Equal(got, want) {
		t.Fatalf("b's logins: %q", got)
	}
	n := 0
	for _, it := range s.items("tok") {
		raw, _ := json.Marshal(it)
		for _, secret := range []string{"mail.example", "asha", "correct horse", "staple", "bank"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("the server can read %q: %s", secret, raw)
			}
		}
		if it.Collection == passwords.Collection {
			n++
		}
	}
	if n != 3 {
		t.Fatalf("the server holds %d logins, want 3", n)
	}
	// Sync details names one by its site and username, and opens nothing.
	if k := a.details().Kinds; !slices.ContainsFunc(k, func(k sync.Kind) bool { return k.Collection == passwords.Collection && k.Items == 3 }) {
		t.Fatalf("a's kinds: %+v", k)
	}

	list, _ := b.passwords.List()
	for _, l := range list {
		if l.Origin == "https://bank.example" {
			if err := b.passwords.Delete(l.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	b.settle()
	run(t, b, a)
	if got := a.logins(); !slices.Equal(got, want[1:]) {
		t.Fatalf("a's logins after b deleted one: %q", got)
	}
}

// The same login brought to two devices before they sync is one entry on both, and one
// item on the server: what makes two logins one is the site, the username and the
// password, as each device gave its row an id of its own. A second password for the same
// site and username stays a second entry.
func TestTheSameLoginOnTwoDevicesIsOne(t *testing.T) {
	s := newServer(t)
	a, b := newProfile(t), newProfile(t)
	a.startSync(s)
	b.startSync(s)
	both := browserdata.Login{Origin: "https://mail.example", Username: "asha", Password: "correct horse"}
	a.passwords.Import([]browserdata.Login{both, {Origin: "https://only-a.example", Username: "a", Password: "one"}})
	b.passwords.Import([]browserdata.Login{both, {Origin: "https://mail.example", Username: "asha", Password: "b's other one"}})
	a.settle()
	b.settle()
	if err := a.connect(s.bearer(), "tok", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	if err := b.connect(s.bearer(), "tok", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	run(t, a, b, a, b)
	want := []string{"https://mail.example asha b's other one", "https://mail.example asha correct horse", "https://only-a.example a one"}
	for name, p := range map[string]*profile{"a": a, "b": b} {
		if got := p.logins(); !slices.Equal(got, want) {
			t.Fatalf("%s's logins: %q", name, got)
		}
		if st := p.sync.Status(); st.Error != "" || st.Unsent != 0 {
			t.Fatalf("%s's status: %+v", name, st)
		}
		if n := p.count(`SELECT count(*) FROM sync_changes`); n != 0 {
			t.Fatalf("%s still has %d rows marked to go", name, n)
		}
	}
	live := 0
	for _, it := range s.items("tok") {
		if it.Collection == passwords.Collection && !it.Deleted {
			live++
		}
	}
	if live != 3 {
		t.Fatalf("the server holds %d live logins for three", live)
	}
	// Both devices hold the shared login as one row, under the id the first one gave it.
	ids := func(p *profile) string {
		var id string
		list, _ := p.passwords.List()
		for _, l := range list {
			if pw, _ := p.passwords.Password(context.Background(), l.ID); pw == "correct horse" {
				id = l.ID
			}
		}
		return id
	}
	if ids(a) == "" || ids(a) != ids(b) {
		t.Fatalf("the shared login's row: %q on a, %q on b", ids(a), ids(b))
	}
}
