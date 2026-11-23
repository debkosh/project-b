package app_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"testing"
	"time"

	"projectb/mockengine"
	"projectb/passkeys"
	"projectb/system"
)

// answerTo waits for the engine to hear action of a passkey request, and returns it.
func (r *rig) answerTo(action string) mockengine.PasskeyAnswer {
	r.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, a := range r.e.PasskeyAnswers() {
			if a.Action == action {
				return a
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	r.t.Fatalf("the engine never heard %q: %v", action, r.e.PasskeyAnswers())
	return mockengine.PasskeyAnswer{}
}

func raw(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("not base64url: %q", s)
	}
	return b
}

// save makes a passkey for user through the Save card, and returns what the engine heard.
func (r *rig) save(user string) map[string]string {
	r.t.Helper()
	before := len(r.e.PasskeyAnswers())
	r.e.Fire(1, 0, "passkey", "create own user="+user)
	d, _ := r.passkey("save")
	r.passkeyAnswer(d["id"], "own", "")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if all := r.e.PasskeyAnswers(); len(all) > before && all[len(all)-1].Action == "made" {
			var made map[string]string
			json.Unmarshal([]byte(all[len(all)-1].Text), &made)
			r.dialogs(func(l []any) bool { return len(l) == 0 })
			return made
		}
		time.Sleep(5 * time.Millisecond)
	}
	r.t.Fatalf("no passkey was made for %s: %v", user, r.e.PasskeyAnswers())
	return nil
}

// A page's make offers Project B first. Save sends the request to ours; the card goes while
// the OS checks the person, which it is asked for by the site's name; and the engine hears
// the new passkey's id and its authenticator data, with our AAGUID.
func TestSavingAPasskeyInProjectB(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	r.e.Fire(1, 0, "passkey", "create own")
	d, p := r.passkey("save")
	words(t, d, "Create a passkey", "Save a passkey for example.com in Project B?", "Save", "Cancel")
	if d["host"] != "example.com" || p["primary"] != "own" || fmt.Sprint(p["other"]) != "map[action:other label:Use a security key or phone]" {
		t.Fatalf("save step = %v", d)
	}
	r.sys.HoldChecks(true)
	r.passkeyAnswer(d["id"], "own", "")
	// The card is away while the OS's sheet is up.
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	r.answerTo("own")
	for len(r.sys.Checks()) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if c := r.sys.Checks()[0]; c.Reason != "save a passkey for example.com" || c.Title != "Save your passkey" || c.Subtitle != "example.com" {
		t.Fatalf("the check says %+v", c)
	}
	if n, _ := r.a.Passkeys.Total(); n != 0 {
		t.Fatal("a passkey was made before the person was checked")
	}
	r.sys.AnswerCheck(system.Verified)

	var made map[string]string
	json.Unmarshal([]byte(r.answerTo("made").Text), &made)
	id, data := raw(t, made["credentialId"]), raw(t, made["authData"])
	hash := sha256.Sum256([]byte("example.com"))
	if len(id) != 16 || len(data) != 37+16+2+16+77 || string(data[:32]) != string(hash[:]) || data[32] != 0x4d ||
		string(data[37:53]) != string(passkeys.AAGUID[:]) || string(data[55:71]) != string(id) {
		t.Fatalf("made %x with data %x", id, data)
	}
	list, _ := r.a.Passkeys.List(context.Background(), true, "example.com", nil)
	if len(list) != 1 || list[0].UserName != "asha@example.com" || list[0].DisplayName != "Asha Rao" || string(list[0].UserHandle) != "handle-asha" {
		t.Fatalf("kept %+v", list)
	}
}

// A sign-in for a site with passkeys here shows them as rows, one too; a press on a row
// is the choice, the OS checks the person, and the engine hears a signature the passkey's
// public key verifies. A site with none goes to the security key's step as before.
func TestSigningInWithAPasskeyOfOurs(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	// None here yet: the key step, as it is.
	r.e.Fire(1, 0, "passkey", "get own")
	d, _ := r.passkey("key")
	words(t, d, "Sign in with a passkey", "example.com wants you to sign in with your security key. Plug it in and touch it.", "", "Cancel")
	r.passkeyAnswer(d["id"], "cancel", "")
	r.dialogs(func(l []any) bool { return len(l) == 0 })

	made := r.save("asha")
	work := r.save("work")

	r.e.Fire(1, 0, "passkey", "get own")
	d, p := r.passkey("own")
	words(t, d, "Sign in to example.com", "", "", "Cancel")
	if got := fmt.Sprint(p["accounts"]); got != "[map[secondary:work@example.com title:Work Rao] map[secondary:asha@example.com title:Asha Rao]]" &&
		got != "[map[secondary:asha@example.com title:Asha Rao] map[secondary:work@example.com title:Work Rao]]" {
		t.Fatalf("rows = %v", got)
	}
	if fmt.Sprint(p["other"]) != "map[action:other label:Use a security key or phone]" {
		t.Fatalf("own step = %v", p)
	}
	// The row of the work account, wherever it is.
	row := 0
	if p["accounts"].([]any)[0].(map[string]any)["title"] != "Work Rao" {
		row = 1
	}
	r.passkeyAnswer(d["id"], "account", fmt.Sprint(row))
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	var signed map[string]string
	json.Unmarshal([]byte(r.answerTo("signed").Text), &signed)
	if signed["credentialId"] != work["credentialId"] || string(raw(t, signed["userHandle"])) != "handle-work" {
		t.Fatalf("signed with %v, want the work account's %v", signed, work["credentialId"])
	}
	data := raw(t, signed["authData"])
	cose := raw(t, work["authData"])[71:]
	public := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(cose[10:42]), Y: new(big.Int).SetBytes(cose[45:])}
	digest := sha256.Sum256(append(data, mockengine.MockClientDataHash...))
	if len(data) != 37 || data[32] != 0x0d || !ecdsa.VerifyASN1(public, digest[:], raw(t, signed["signature"])) {
		t.Fatalf("the signature doesn't verify over %x and the client data's hash", data)
	}
	if c := r.sys.Checks(); c[len(c)-1].Reason != "use a passkey for example.com" || c[len(c)-1].Title != "Use your passkey" {
		t.Fatalf("the check says %+v", c[len(c)-1])
	}

	// A page that allows only one shows that one, and one it doesn't know none of ours.
	r.e.Fire(1, 0, "passkey", "get own allow="+made["credentialId"]+",bm90LW91cnM")
	d, p = r.passkey("own")
	if got := fmt.Sprint(p["accounts"]); got != "[map[secondary:asha@example.com title:Asha Rao]]" {
		t.Fatalf("allowed rows = %v", got)
	}
	r.passkeyAnswer(d["id"], "cancel", "")
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	r.e.Fire(1, 0, "passkey", "get own allow=bm90LW91cnM")
	r.passkey("key")
}

// A second passkey saved for one account on a site stays beside the first, as a site that
// names no passkey to leave out lets a person make: a sign-in lists both, the newer first,
// each saying when it was made, as they read the same otherwise, and either signs. Another
// account's goes after or before the two, never between them.
func TestTwoPasskeysForOneAccountAreBothOffered(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	first := r.save("asha")
	time.Sleep(2 * time.Millisecond) // so the second is the newer by the store's clock
	second := r.save("asha")
	if first["credentialId"] == second["credentialId"] {
		t.Fatalf("the two saves made one passkey: %v", first)
	}
	if n, _ := r.a.Passkeys.Total(); n != 2 {
		t.Fatalf("the profile has %d passkeys, want both", n)
	}

	r.e.Fire(1, 0, "passkey", "get own")
	d, p := r.passkey("own")
	rows := p["accounts"].([]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %v, want both passkeys", rows)
	}
	// Both were made just now: today, so with the time, unless midnight fell between them.
	made := regexp.MustCompile(`^asha@example\.com · Made (today, \d\d:\d\d|today|\d{1,2} [A-Z][a-z]{2})$`)
	for _, row := range rows {
		if a := row.(map[string]any); a["title"] != "Asha Rao" || !made.MatchString(fmt.Sprint(a["secondary"])) {
			t.Fatalf("a row reads %v", a)
		}
	}
	// The older one, the second row, still signs, as itself.
	r.passkeyAnswer(d["id"], "account", "1")
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	var signed map[string]string
	json.Unmarshal([]byte(r.answerTo("signed").Text), &signed)
	if signed["credentialId"] != first["credentialId"] || string(raw(t, signed["userHandle"])) != "handle-asha" {
		t.Fatalf("signed with %v, want the first passkey %v", signed, first["credentialId"])
	}

	// Another account's, made between a third of the first account's and the two, says no
	// date, as its row reads as no other does, and comes after the three: the account with
	// the newest passkey is first, its passkeys together.
	time.Sleep(2 * time.Millisecond)
	work := r.save("work")
	time.Sleep(2 * time.Millisecond)
	third := r.save("asha")
	r.e.Fire(1, 0, "passkey", "get own")
	d, p = r.passkey("own")
	rows = p["accounts"].([]any)
	if len(rows) != 4 || fmt.Sprint(rows[3]) != "map[secondary:work@example.com title:Work Rao]" {
		t.Fatalf("rows with another account's = %v", rows)
	}
	// The rows are the passkeys in that order: the first row signs with the newest, the
	// last with the other account's.
	for row, want := range map[string]map[string]string{"0": third, "3": work} {
		before := len(r.e.PasskeyAnswers())
		r.passkeyAnswer(d["id"], "account", row)
		r.dialogs(func(l []any) bool { return len(l) == 0 })
		// The engine hears the press, then the signature.
		last := func() mockengine.PasskeyAnswer { all := r.e.PasskeyAnswers(); return all[len(all)-1] }
		deadline := time.Now().Add(5 * time.Second)
		for (len(r.e.PasskeyAnswers()) == before || last().Action != "signed") && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		json.Unmarshal([]byte(last().Text), &signed)
		if last().Action != "signed" || signed["credentialId"] != want["credentialId"] {
			t.Fatalf("row %s signed with %v, want %v", row, last(), want["credentialId"])
		}
		if row == "0" {
			r.e.Fire(1, 0, "passkey", "get own")
			d, _ = r.passkey("own")
		}
	}
}

// Use a security key or phone goes to the engine's key step, which waited all along, and
// stays there when the engine sends it again; Try again offers ours anew.
func TestTheWayToASecurityKeyOrPhone(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	r.e.Fire(1, 0, "passkey", "create own")
	d, _ := r.passkey("save")
	id := d["id"]
	r.passkeyAnswer(id, "other", "")
	d, p := r.passkey("key")
	words(t, d, "Create a passkey", "example.com wants to save a passkey on your security key. Plug it in and touch it.", "", "Cancel")
	if fmt.Sprint(p["other"]) != "map[action:phone label:Use a phone]" {
		t.Fatalf("key step = %v", p)
	}
	// To the phone and back: the key step, not Save again.
	r.passkeyAnswer(id, "phone", "")
	r.passkey("phone")
	r.passkeyAnswer(id, "key", "")
	r.passkey("key")
	// An error, and Try again: the request starts over, and ours is offered.
	r.e.Fire(1, 0, "passkeyStep", `error {"reason":"timeout"}`)
	r.passkey("error")
	r.passkeyAnswer(id, "retry", "")
	r.passkey("save")
	for _, a := range r.e.PasskeyAnswers() {
		if a.Action == "other" || a.Action == "own" {
			t.Fatalf("the engine heard %q, which the person never chose to send it", a.Action)
		}
	}
}

// What can go wrong with ours: the site already has the passkey, which shows a card and
// tells the engine once it is closed; a device with nothing to check with; and a person
// who says no to the check, which ends the request as Cancel does, with no card.
func TestOurPasskeysErrors(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	made := r.save("asha")
	failed := func() []string {
		var out []string
		for _, a := range r.e.PasskeyAnswers() {
			if a.Action == "ownFailed" {
				out = append(out, a.Text)
			}
		}
		return out
	}

	r.e.Fire(1, 0, "passkey", "create own exclude="+made["credentialId"])
	d, _ := r.passkey("save")
	checks := len(r.sys.Checks())
	r.passkeyAnswer(d["id"], "own", "")
	d, p := r.passkey("ownError")
	words(t, d, "Already saved", "Project B already has a passkey for example.com for this account.", "Close", "")
	if p["primary"] != "cancel" || p["other"] != nil || len(r.sys.Checks()) != checks {
		t.Fatalf("error step = %v, %d checks asked", p, len(r.sys.Checks())-checks)
	}
	if got := failed(); len(got) != 0 {
		t.Fatalf("the engine heard %v before the card was closed", got)
	}
	r.passkeyAnswer(d["id"], "cancel", "")
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	if got := failed(); len(got) != 1 || got[0] != "excluded" {
		t.Fatalf("the engine heard %v, want excluded", got)
	}

	r.sys.SetVerdict(system.NoScreenLock)
	r.e.Fire(1, 0, "passkey", "create only user=work")
	d, p = r.passkey("save")
	if p["other"] != nil {
		t.Fatalf("a phone's save step has a way to a security key: %v", p)
	}
	r.passkeyAnswer(d["id"], "own", "")
	d, _ = r.passkey("ownError")
	words(t, d, "Set a screen lock first", "Project B checks it's you with your screen lock before using a passkey. Set one in your phone's Settings, then try again.", "Close", "")
	r.passkeyAnswer(d["id"], "cancel", "")
	r.dialogs(func(l []any) bool { return len(l) == 0 })

	r.sys.SetVerdict(system.Denied)
	r.e.Fire(1, 0, "passkey", "get own")
	d, _ = r.passkey("own")
	r.passkeyAnswer(d["id"], "account", "0")
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	deadline := time.Now().Add(5 * time.Second)
	for len(failed()) < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := failed(); fmt.Sprint(got) != "[excluded no-screen-lock denied]" {
		t.Fatalf("the engine heard %v", got)
	}
	if n, _ := r.a.Passkeys.Total(); n != 1 {
		t.Fatalf("%d passkeys, want the first alone", n)
	}
}

// On a phone ours is the only way: a sign-in for a site with no passkey here is refused
// at once, and its cards have no way to a security key.
func TestOnAPhoneOursIsTheOnlyWay(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "passkey", "get only")
	if a := r.answerTo("cancel"); a.Text != "" {
		t.Fatalf("cancel with %q", a.Text)
	}
	r.e.Settle()
	// A phone's shell draws the card as a plain dialog, and answers it as one: yes is the
	// step's own primary button, here Save.
	r.e.Fire(1, 0, "passkey", "create only")
	d, _ := r.passkey("save")
	r.shell(fmt.Sprintf(`{"t":"answerDialog","id":%v,"ok":true}`, d["id"]))
	r.answerTo("made")
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	r.e.Fire(1, 0, "passkey", "create only user=work")
	d, _ = r.passkey("save")
	before := len(r.e.PasskeyAnswers())
	r.shell(fmt.Sprintf(`{"t":"answerDialog","id":%v,"ok":false}`, d["id"]))
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	if all := r.e.PasskeyAnswers(); len(all) != before+1 || all[before].Action != "cancel" {
		t.Fatalf("no to the card told the engine %v", all[before:])
	}
	r.e.Fire(1, 0, "passkey", "get only")
	_, p := r.passkey("own")
	if p["other"] != nil || len(p["accounts"].([]any)) != 1 {
		t.Fatalf("a phone's own step = %v", p)
	}
}

// The page's request ending while the OS checks the person takes the check back, and
// nothing is made.
func TestARequestEndingDuringTheCheck(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.sys.HoldChecks(true)
	r.e.Fire(1, 0, "passkey", "create own")
	d, _ := r.passkey("save")
	r.passkeyAnswer(d["id"], "own", "")
	for len(r.sys.Checks()) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	r.e.Fire(1, 0, "passkeyStep", "end")
	deadline := time.Now().Add(5 * time.Second)
	for r.sys.CancelledChecks() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if r.sys.CancelledChecks() != 1 {
		t.Fatal("the check was left up after its request ended")
	}
	r.sys.AnswerCheck(system.Verified)
	r.e.Settle()
	if n, _ := r.a.Passkeys.Total(); n != 0 {
		t.Fatalf("%d passkeys made for a request that ended", n)
	}
}
