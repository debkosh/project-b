package passwords

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sudiptadeb/project-b/browserdata"

	"projectb/keys"
	"projectb/store"
)

// profile is a profile folder named outright, with its store, its keys and its logins, and
// a device's check that counts how often it is asked and answers as told.
type profile struct {
	dir    string
	st     *store.Store
	keys   *keys.Keys
	m      *Manager
	asked  []string
	denied error
	closed bool
}

// close closes the store, once: a test that reads the files closes it first.
func (p *profile) close() {
	if !p.closed {
		p.closed = true
		p.st.Close()
	}
}

func open(t *testing.T, dir string) *profile {
	t.Helper()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := &profile{dir: dir, st: st, keys: keys.New(dir, "", st, nil)}
	t.Cleanup(p.close)
	p.m = New(st, p.keys, func(_ context.Context, origin string) error {
		p.asked = append(p.asked, origin)
		return p.denied
	})
	return p
}

var mail = browserdata.Login{Origin: "https://mail.example.org", Username: "asha@example.org", Password: "correct horse", Created: time.UnixMilli(1700000000000), LastUsed: time.UnixMilli(1700000500000)}

// Logins brought over are listed with no password and no check of the person; a password
// opens only after the device has checked them, and the store's file never holds it plain.
func TestImportListAndShow(t *testing.T) {
	p := open(t, t.TempDir())
	changes := 0
	p.m.OnChanged(func() { changes++ })
	added, already, skipped := p.m.Import([]browserdata.Login{
		mail,
		{Origin: "https://shop.example.com/", Username: "asha", Password: "tr0ub4dor"},
		{Origin: "android://com.example.app", Username: "asha", Password: "x"},
		{Origin: "https://empty.example.com", Username: "asha"},
	})
	if added != 2 || already != 0 || skipped != 2 || changes != 1 {
		t.Fatalf("brought: %d added, %d already, %d skipped, %d changes heard", added, already, skipped, changes)
	}
	if n, err := p.m.Total(); err != nil || n != 2 {
		t.Fatalf("the total: %d, %v", n, err)
	}
	list, err := p.m.List()
	if err != nil || len(list) != 2 || list[0].Origin != "https://mail.example.org" || list[0].Username != "asha@example.org" ||
		list[0].Created != 1700000000000 || list[0].Used != 1700000500000 || list[1].Origin != "https://shop.example.com" || list[1].Created == 0 || list[1].Used != 0 {
		t.Fatalf("the list: %+v, %v", list, err)
	}
	if len(p.asked) != 0 {
		t.Fatalf("listing asked the person: %v", p.asked)
	}
	got, err := p.m.Password(context.Background(), list[0].ID)
	if err != nil || got != "correct horse" || len(p.asked) != 1 || p.asked[0] != "https://mail.example.org" {
		t.Fatalf("the password: %q, %v, asked %v", got, err, p.asked)
	}
	p.denied = errors.New("the person said no")
	if got, err := p.m.Password(context.Background(), list[1].ID); err != p.denied || got != "" {
		t.Fatalf("a password the person wasn't checked for: %q, %v", got, err)
	}
	if _, err := p.m.Password(context.Background(), "no-such-id"); !errors.Is(err, ErrNone) || len(p.asked) != 2 {
		t.Fatalf("a login that isn't there: %v, asked %v", err, p.asked)
	}
	p.close()
	for _, name := range []string{"store.db", "store.db-wal"} {
		b, _ := os.ReadFile(filepath.Join(p.dir, name))
		for _, secret := range []string{"correct horse", "tr0ub4dor"} {
			if bytes.Contains(b, []byte(secret)) {
				t.Fatalf("%s holds the password %q plain", name, secret)
			}
		}
	}
}

// Brought again, a login saved as it is counts as already there; one for the same site and
// username with another password is added beside it, and both stay.
func TestImportAgainAndAClash(t *testing.T) {
	p := open(t, t.TempDir())
	p.m.Import([]browserdata.Login{mail})
	changed := mail
	changed.Password = "battery staple"
	added, already, skipped := p.m.Import([]browserdata.Login{mail, changed, changed, {Origin: "https://mail.example.org", Username: "other", Password: "correct horse"}})
	if added != 2 || already != 2 || skipped != 0 {
		t.Fatalf("brought again: %d added, %d already, %d skipped", added, already, skipped)
	}
	list, _ := p.m.List()
	if len(list) != 3 {
		t.Fatalf("the list after a clash: %+v", list)
	}
	both := map[string]bool{}
	for _, l := range list {
		if l.Username == mail.Username {
			pw, err := p.m.Password(context.Background(), l.ID)
			if err != nil {
				t.Fatal(err)
			}
			both[pw] = true
		}
	}
	if !both["correct horse"] || !both["battery staple"] {
		t.Fatalf("the two passwords for one account: %v", both)
	}
}

// A password is bound to its row and its site: moved to another row, it doesn't open. A
// login deleted is gone, and starting the profile's keys fresh deletes them all.
func TestBoundDeleteAndStartFresh(t *testing.T) {
	p := open(t, t.TempDir())
	p.m.Import([]browserdata.Login{mail, {Origin: "https://shop.example.com", Username: "asha", Password: "tr0ub4dor"}})
	list, _ := p.m.List()
	if err := p.st.Wait(func(tx *store.Tx) error {
		_, err := tx.Exec(`UPDATE logins SET password = (SELECT password FROM logins WHERE id = ?) WHERE id = ?`, list[0].ID, list[1].ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got, err := p.m.Password(context.Background(), list[1].ID); err == nil {
		t.Fatalf("another row's password opened as this one's: %q", got)
	}
	changes := 0
	p.m.OnChanged(func() { changes++ })
	if err := p.m.Delete(list[1].ID); err != nil || changes != 1 {
		t.Fatalf("deleting: %v, %d changes heard", err, changes)
	}
	if n, _ := p.m.Total(); n != 1 {
		t.Fatalf("%d logins after a delete", n)
	}
	if err := p.keys.StartFresh(); err != nil {
		t.Fatal(err)
	}
	if n, _ := p.m.Total(); n != 0 {
		t.Fatalf("%d logins after starting fresh", n)
	}
}

// A profile whose key doesn't open here keeps nothing brought to it, and says so of a
// password it holds.
func TestAProfileWhoseKeyDoesntOpenHere(t *testing.T) {
	dir := t.TempDir()
	p := open(t, dir)
	p.m.Import([]browserdata.Login{mail})
	list, _ := p.m.List()
	p.close()
	// The folder's own key is lost, as a folder copied without it.
	if err := os.Remove(filepath.Join(dir, "folder.key")); err != nil {
		t.Fatal(err)
	}
	p = open(t, dir)
	if _, err := p.m.Password(context.Background(), list[0].ID); !errors.Is(err, ErrNotHere) {
		t.Fatalf("a password of a profile that doesn't open here: %v", err)
	}
	if added, already, skipped := p.m.Import([]browserdata.Login{{Origin: "https://new.example.com", Username: "a", Password: "b"}}); added != 0 || already != 0 || skipped != 1 {
		t.Fatalf("brought to it: %d added, %d already, %d skipped", added, already, skipped)
	}
	if n, _ := p.m.Total(); n != 1 {
		t.Fatalf("%d logins: what it had should stay", n)
	}
}
