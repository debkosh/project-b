package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sudiptadeb/project-b/browserdata"

	"projectb/app"
	"projectb/engine"
	"projectb/forms"
	"projectb/mockengine"
	"projectb/passwords"
)

// Filling a saved login into a page: the engine is given the form script; a frame's
// fields and a press on its password field, as the engine stamps and carries them, bring
// the tab's shell an offer of that site's logins with no password, placed where the field
// is in the tab's page; passwordPick fills the username and the password into that
// document with no check of the person; and the offer goes. passwordDismiss takes an
// offer away, as does the tab going on to another page; passwordAt says which login the
// pointer is over; passwordsManage opens Settings at Passwords.
func TestFillingASavedLogin(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	if r.e.FormScript() != forms.Script || forms.Script == "" {
		t.Fatal("the engine wasn't given the form script")
	}
	r.a.Passwords.Import([]browserdata.Login{
		{Origin: "https://shop.example", Username: "asha", Password: "correct horse"},
		{Origin: "https://other.example", Username: "elsewhere", Password: "never here"},
	})
	r.visit("https://shop.example/account")
	tab := r.activeTab()
	// A frame of the page, 40 across and 300 down in it, of the page's own origin.
	from := engine.FormFrame{Tab: tab, Frame: "frame-1", Document: "doc-1", Origin: "https://shop.example", Secure: true, X: 40, Y: 300}
	tell := func(kind, payload string) { r.e.Post(func() { r.e.FormMessage(from, kind, payload) }) }
	fields := `{"fields":[{"ref":"f1","form":"F1","tag":"input","type":"email","name":"email","visible":true,"focusable":true,"empty":true},
		{"ref":"f2","form":"F1","tag":"input","type":"password","name":"password","visible":true,"focusable":true,"empty":true}]}`
	offer := func() map[string]any {
		t.Helper()
		m := r.waitFor(func(m map[string]any) bool { return m["t"] == "passwordOffer" && m["passwordOffer"] != nil })
		return m["passwordOffer"].(map[string]any)
	}
	gone := func() {
		t.Helper()
		r.waitFor(func(m map[string]any) bool { return m["t"] == "passwordOffer" && m["passwordOffer"] == nil })
	}

	tell("fields", fields)
	tell("pressed", `{"ref":"f2","box":{"x":10,"y":20,"w":200,"h":30}}`)
	o := offer()
	logins := o["logins"].([]any)
	box := o["box"].(map[string]any)
	if o["site"] != "shop.example" || o["field"] != "current-password" || len(logins) != 1 || logins[0].(map[string]any)["username"] != "asha" ||
		box["x"] != float64(50) || box["y"] != float64(320) || box["w"] != float64(200) {
		t.Fatalf("the offer: %v", o)
	}
	if s := strings.ToLower(toJSON(o)); strings.Contains(s, "correct horse") || strings.Contains(s, "password\":") {
		t.Fatalf("the offer holds a password: %s", s)
	}
	r.shell(`{"t":"passwordPick","offer":` + toJSON(o["id"]) + `,"login":"` + logins[0].(map[string]any)["id"].(string) + `"}`)
	gone()
	fills := func() []mockengine.FormSent {
		var out []mockengine.FormSent
		for _, s := range r.e.FormsSent() {
			if s.Kind == "fill" {
				out = append(out, s)
			}
		}
		return out
	}
	sent := fills()
	if len(sent) != 1 || sent[0].To.Tab != tab || sent[0].To.Frame != "frame-1" || sent[0].To.Document != "doc-1" ||
		sent[0].Payload != `{"values":{"f1":"asha","f2":"correct horse"}}` {
		t.Fatalf("the fill: %+v", sent)
	}
	if n := len(r.sys.Checks()); n != 0 {
		t.Fatalf("a fill had the device check the person %d times", n)
	}

	// The pointer over a login makes it the list's current one: passwordAt.
	tell("pressed", `{"ref":"f1","box":{"x":10,"y":0,"w":200,"h":30}}`)
	up := offer()
	if up["at"] != float64(-1) {
		t.Fatalf("an offer starts at %v", up["at"])
	}
	r.shell(`{"t":"passwordAt","offer":` + toJSON(up["id"]) + `,"at":0}`)
	if at := offer(); at["at"] != float64(0) || at["id"] != up["id"] {
		t.Fatalf("after passwordAt the offer is %v", at)
	}
	// An offer taken away by the person, and one taken away by the tab going elsewhere.
	r.shell(`{"t":"passwordDismiss","offer":` + toJSON(up["id"]) + `}`)
	gone()
	tell("pressed", `{"ref":"f1","box":{"x":10,"y":0,"w":200,"h":30}}`)
	offer()
	r.shell(`{"t":"navigate","text":"https://shop.example/elsewhere"}`)
	gone()
	// A pick that comes after is of an offer that has gone: nothing more is filled.
	// Filling takes its work in order, so by the next offer the stale pick was dealt with.
	r.shell(`{"t":"passwordPick","offer":` + toJSON(o["id"]) + `,"login":"x"}`)
	tell("pressed", `{"ref":"f1","box":{"x":10,"y":0,"w":200,"h":30}}`)
	offer()
	if n := len(fills()); n != 1 {
		t.Fatalf("%d fills, want the one", n)
	}
	r.shell(`{"t":"passwordsManage"}`)
	r.waitForTab(func(t map[string]any) bool { return t["page"] == "settings" })
}

// Saving a login a person signed in with: the form's values, as the form script tells them
// when it is sent, bring nothing until the tab goes on; then the tab's shell is offered to
// save, with no password in the offer; passwordSaveShow answers the asking shell with what
// was typed; passwordSaveAnswer with save keeps the login, and the offer goes. A second
// sign-in with another password offers an update, and never has the site not offered on.
func TestSavingALoginSignedInWith(t *testing.T) {
	defer app.SetPasswordSettle(30 * time.Millisecond)()
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://shop.example/login")
	tab := r.activeTab()
	fields := `{"fields":[{"ref":"f1","form":"F1","tag":"input","type":"email","name":"email","visible":true,"focusable":true,"empty":true},
		{"ref":"f2","form":"F1","tag":"input","type":"password","name":"password","visible":true,"focusable":true,"empty":true}]}`
	signIn := func(doc, username, password string) {
		from := engine.FormFrame{Tab: tab, Frame: "frame-1", Document: doc, Origin: "https://shop.example", Top: true, Secure: true}
		r.e.Post(func() { r.e.FormMessage(from, "fields", fields) })
		r.e.Post(func() {
			r.e.FormMessage(from, "submitted", `{"form":"F1","values":{"f1":`+toJSON(username)+`,"f2":`+toJSON(password)+`}}`)
		})
	}
	offer := func() map[string]any {
		t.Helper()
		m := r.waitFor(func(m map[string]any) bool { return m["t"] == "passwordSave" && m["passwordSave"] != nil })
		return m["passwordSave"].(map[string]any)
	}
	gone := func() {
		t.Helper()
		r.waitFor(func(m map[string]any) bool { return m["t"] == "passwordSave" && m["passwordSave"] == nil })
	}
	saved := func() []string {
		t.Helper()
		out := []string{}
		list, _ := r.a.Passwords.List()
		for _, l := range list {
			p, err := r.a.Passwords.Password(passwords.Checked(context.Background()), l.ID)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, l.Origin+" "+l.Username+" "+p)
		}
		return out
	}

	signIn("doc-1", "asha", "correct horse")
	r.shell(`{"t":"navigate","text":"https://shop.example/account"}`)
	o := offer()
	if o["site"] != "shop.example" || o["username"] != "asha" || o["update"] != nil || strings.Contains(toJSON(o), "correct horse") {
		t.Fatalf("the offer to save: %v", o)
	}
	r.shell(`{"t":"passwordSaveShow","offer":` + toJSON(o["id"]) + `}`)
	if m := r.waitFor(func(m map[string]any) bool { return m["t"] == "passwordSaveShown" })["passwordSaveShown"].(map[string]any); m["password"] != "correct horse" || m["offer"] != o["id"] {
		t.Fatalf("what was typed, shown: %v", m)
	}
	r.shell(`{"t":"passwordSaveAnswer","offer":` + toJSON(o["id"]) + `,"choice":"save"}`)
	gone()
	if got := saved(); len(got) != 1 || got[0] != "https://shop.example asha correct horse" {
		t.Fatalf("after Save: %v", got)
	}
	if n := len(r.sys.Checks()); n != 0 {
		t.Fatalf("saving had the device check the person %d times", n)
	}

	// Another password for the saved username: an update, which changes it.
	signIn("doc-2", "asha", "a new one")
	r.shell(`{"t":"navigate","text":"https://shop.example/again"}`)
	o = offer()
	if o["update"] != true || o["username"] != "asha" {
		t.Fatalf("the offer to update: %v", o)
	}
	r.shell(`{"t":"passwordSaveAnswer","offer":` + toJSON(o["id"]) + `,"choice":"save"}`)
	gone()
	if got := saved(); len(got) != 1 || got[0] != "https://shop.example asha a new one" {
		t.Fatalf("after Update: %v", got)
	}

	// Never: the offer goes, nothing is kept, and the site isn't offered on again.
	signIn("doc-3", "work", "second one")
	r.shell(`{"t":"navigate","text":"https://shop.example/third"}`)
	o = offer()
	r.shell(`{"t":"passwordSaveAnswer","offer":` + toJSON(o["id"]) + `,"choice":"never"}`)
	gone()
	if never, _ := r.a.Passwords.IsNever("https://shop.example"); !never || len(saved()) != 1 {
		t.Fatalf("after Never: never %v, saved %v", never, saved())
	}
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
