package app_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"projectb/mockengine"
	"projectb/timeline"
)

// passkey waits for the one dialog to be a passkey card at step, and returns it and its
// passkey part.
func (r *rig) passkey(step string) (map[string]any, map[string]any) {
	r.t.Helper()
	var d, p map[string]any
	r.dialogs(func(l []any) bool {
		if len(l) != 1 {
			return false
		}
		d = l[0].(map[string]any)
		p, _ = d["passkey"].(map[string]any)
		return d["kind"] == "passkey" && p["step"] == step
	})
	return d, p
}

func (r *rig) passkeyAnswer(id any, action, text string) {
	r.t.Helper()
	r.shell(fmt.Sprintf(`{"t":"answerPasskey","id":%v,"action":%q,"text":%q}`, id, action, text))
}

// words checks a card's title, words, primary button and the button beside it.
func words(t *testing.T, d map[string]any, title, message, ok, cancel string) {
	t.Helper()
	if d["title"] != title || d["message"] != message || d["ok"] != ok || d["cancel"] != cancel {
		t.Fatalf("card = %q %q, buttons %q %q; want %q %q, %q %q", d["title"], d["message"], d["ok"], d["cancel"], title, message, ok, cancel)
	}
}

func TestAPasskeyRequestIsOneDialogThatChangesAtEachStep(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	// The key step, for making a passkey and for signing in, with a phone and without.
	r.e.Fire(1, 0, "passkey", "create")
	d, p := r.passkey("key")
	id := d["id"]
	words(t, d, "Create a passkey", "example.com wants to save a passkey on your security key. Plug it in and touch it.", "", "Cancel")
	if d["host"] != "example.com" || fmt.Sprint(p["other"]) != "map[action:phone label:Use a phone]" || p["primary"] != nil {
		t.Fatalf("key step = %v", d)
	}

	// The PIN: asked for, refused, to be set, and to be changed.
	r.e.Fire(1, 0, "passkeyStep", `pin {"reason":"enter","error":"","attempts":8,"minLength":4}`)
	d, p = r.passkey("pin")
	words(t, d, "Enter your security key's PIN", "", "Next", "Cancel")
	if d["id"] != id {
		t.Fatalf("a step is the same dialog: %v, was %v", d["id"], id)
	}
	if fmt.Sprint(p["pin"]) != "map[labels:[PIN] minLength:4 short:At least 4 characters]" || p["primary"] != "pin" {
		t.Fatalf("pin step = %v", p)
	}
	seq := p["seq"].(float64)
	pinError := func(carries, want string) {
		t.Helper()
		r.e.Fire(1, 0, "passkeyStep", "pin "+carries)
		var p map[string]any
		r.dialogs(func(l []any) bool {
			p, _ = l[0].(map[string]any)["passkey"].(map[string]any)
			return p["step"] == "pin" && p["seq"].(float64) > seq
		})
		seq = p["seq"].(float64)
		if got := p["pin"].(map[string]any)["error"]; got != want {
			t.Fatalf("pin error for %s = %v, want %q", carries, got, want)
		}
	}
	pinError(`{"reason":"enter","error":"wrong","attempts":3,"minLength":4}`, "Wrong PIN. 3 tries left.")
	pinError(`{"reason":"enter","error":"wrong","attempts":1,"minLength":4}`, "Wrong PIN. 1 try left before the key locks.")
	pinError(`{"reason":"enter","error":"uv-locked","attempts":8,"minLength":4}`, "Too many fingerprint tries. Enter your PIN instead.")
	pinError(`{"reason":"set","error":"short","minLength":6}`, "At least 6 characters")
	pinError(`{"reason":"change","error":"same","minLength":4}`, "The new PIN must be different from your current one.")
	r.e.Fire(1, 0, "passkeyStep", `pin {"reason":"set","error":"","minLength":6}`)
	d, p = r.passkey("pin")
	for d["ok"] != "Set PIN" {
		d, p = r.passkey("pin")
	}
	words(t, d, "Set a PIN for your security key", "example.com needs your key to have a PIN. At least 6 characters.", "Set PIN", "Cancel")
	if fmt.Sprint(p["pin"]) != "map[labels:[PIN Confirm PIN] minLength:6 mismatch:The PINs don't match short:At least 6 characters]" {
		t.Fatalf("set step = %v", p)
	}
	r.e.Fire(1, 0, "passkeyStep", `pin {"reason":"change","error":"","minLength":4}`)
	for d["ok"] != "Change PIN" {
		d, p = r.passkey("pin")
	}
	words(t, d, "Change your security key's PIN", "Your key needs a new PIN before it can be used.", "Change PIN", "Cancel")
	if fmt.Sprint(p["pin"].(map[string]any)["labels"]) != "[New PIN Confirm new PIN]" {
		t.Fatalf("change step = %v", p)
	}

	// The PIN goes to the engine as it was typed.
	r.passkeyAnswer(id, "pin", "12 34")
	r.e.Fire(1, 0, "passkeyStep", "touch")
	d, _ = r.passkey("touch")
	words(t, d, "Touch your security key", "Touch your security key again to finish.", "", "Cancel")
	r.e.Fire(1, 0, "passkeyStep", `touch {"attempts":2}`)
	r.dialogs(func(l []any) bool {
		return l[0].(map[string]any)["message"] == "That fingerprint didn't match. Touch your security key again. 2 tries left."
	})
	r.e.Fire(1, 0, "passkeyStep", `touch {"attempts":1}`)
	r.dialogs(func(l []any) bool {
		return l[0].(map[string]any)["message"] == "That fingerprint didn't match. Touch your security key again. 1 try left."
	})

	// The accounts: the display name first, the name alone when there is no other.
	r.e.Fire(1, 0, "passkeyStep", `account {"accounts":[{"name":"ada@example.com","displayName":"Ada Lovelace"},{"name":"grace","displayName":""},{"name":"same","displayName":"same"},{"name":"","displayName":""}]}`)
	d, p = r.passkey("account")
	words(t, d, "Choose an account", "Your security key has 4 accounts for example.com.", "", "Cancel")
	if fmt.Sprint(p["accounts"]) != "[map[secondary:ada@example.com title:Ada Lovelace] map[title:grace] map[title:same] map[title:No name unnamed:true]]" {
		t.Fatalf("accounts = %v", p["accounts"])
	}
	r.passkeyAnswer(id, "account", "1")

	// The request ending from the engine's side takes the card away.
	r.e.Fire(1, 0, "passkeyStep", "end")
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	r.e.Settle()
	want := []mockengine.PasskeyAnswer{{ID: 1, Action: "pin", Text: "12 34"}, {ID: 1, Action: "account", Text: "1"}}
	if got := r.e.PasskeyAnswers(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("answers = %v, want %v", got, want)
	}
	if got := r.e.DialogAnswers(); len(got) != 0 {
		t.Fatalf("a passkey request is answered as one: %v", got)
	}
}

func TestAPasskeyErrorSaysWhatWentWrongAndWhetherToTryAgain(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "passkey", "get")
	d, p := r.passkey("key")
	id := d["id"]
	words(t, d, "Sign in with a passkey", "example.com wants you to sign in with your security key. Plug it in and touch it.", "", "Cancel")

	for _, c := range []struct {
		reason, title, words string
		again                bool
	}{
		{"timeout", "That took too long", "example.com stopped waiting. Try again when your key or phone is ready.", true},
		{"not-registered", "No passkey on this key", "This security key has no passkey for example.com. Try another key or a phone.", true},
		{"already-registered", "Already saved", "This security key already has a passkey for example.com.", false},
		{"pin-soft-blocked", "Too many wrong PINs", "Unplug your security key, plug it in again, and try again.", true},
		{"pin-hard-blocked", "Your security key is locked", "Too many wrong PINs. The key has to be reset before it can be used again, which erases its passkeys.", false},
		{"key-removed", "Your security key was unplugged", "Plug it in and try again.", true},
		{"no-passkey-support", "Use another key", "This security key can't do what example.com asks. Try another key or a phone.", true},
		{"no-uv", "Use another key", "This security key can't do what example.com asks. Try another key or a phone.", true},
		{"no-large-blob", "Use another key", "This security key can't do what example.com asks. Try another key or a phone.", true},
		{"no-algorithm", "Use another key", "This security key can't do what example.com asks. Try another key or a phone.", true},
		{"full", "Your security key is full", "It has no room for another passkey. Remove one with the key maker's app, or use another key.", false},
		{"phone-failed", "Couldn't reach your phone", "Make sure Bluetooth is on for this Mac and your phone, and try again.", true},
		{"other", "Something went wrong", "The request didn't finish.", true},
	} {
		r.e.Fire(1, 0, "passkeyStep", fmt.Sprintf(`error {"reason":%q}`, c.reason))
		r.dialogs(func(l []any) bool {
			d = l[0].(map[string]any)
			p, _ = d["passkey"].(map[string]any)
			return p["step"] == "error" && d["title"] == c.title
		})
		// One primary: Try again where it can help, with Close beside it; Close otherwise.
		if c.again {
			words(t, d, c.title, c.words, "Try again", "Close")
		} else {
			words(t, d, c.title, c.words, "Close", "")
		}
		if want := map[bool]string{true: "retry", false: "cancel"}[c.again]; p["primary"] != want {
			t.Fatalf("%s: primary = %v, want %s", c.reason, p["primary"], want)
		}
	}

	// Try again starts the request over, at the key step; Close ends it.
	r.passkeyAnswer(id, "retry", "")
	r.passkey("key")
	r.passkeyAnswer(id, "cancel", "")
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	r.e.Settle()
	if got := r.e.PasskeyAnswers(); len(got) != 2 || got[0].Action != "retry" || got[1].Action != "cancel" {
		t.Fatalf("answers = %v", got)
	}
}

func TestAPhoneStepCarriesItsCodeAndWhatBluetoothNeeds(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	// With no phone as key, the key step offers none.
	r.e.Fire(1, 0, "passkey", "get nophone")
	d, p := r.passkey("key")
	if p["other"] != nil {
		t.Fatalf("no phone, no Use a phone: %v", p)
	}
	r.passkeyAnswer(d["id"], "cancel", "")
	r.dialogs(func(l []any) bool { return len(l) == 0 })

	r.e.Fire(1, 0, "passkey", "get")
	d, _ = r.passkey("key")
	id := d["id"]
	r.passkeyAnswer(id, "phone", "")
	d, p = r.passkey("phone")
	words(t, d, "Use your phone", "Scan this code with your phone's camera.", "", "Cancel")
	if fmt.Sprint(p["other"]) != "map[action:key label:Use a security key]" || p["bluetooth"] != nil {
		t.Fatalf("phone step = %v", p)
	}
	// The code: square, of 0 and 1 only, with a finder pattern's dark edge along the top
	// left, and no quiet zone of its own.
	rows := p["code"].([]any)
	if len(rows) < 21 {
		t.Fatalf("code has %d rows", len(rows))
	}
	for _, row := range rows {
		if s := row.(string); len(s) != len(rows) || strings.Trim(s, "01") != "" {
			t.Fatalf("a row = %q, of %d", s, len(rows))
		}
	}
	if top := rows[0].(string); !strings.HasPrefix(top, "11111110") || !strings.HasPrefix(rows[1].(string), "10000010") {
		t.Fatalf("no finder pattern: %q", top)
	}

	for _, c := range []struct{ carries, words, button string }{
		{`{"link":"FIDO:/1","bluetooth":"off","canTurnOn":true}`, "Bluetooth is off.", "map[action:bluetoothOn label:Turn on Bluetooth]"},
		{`{"link":"FIDO:/1","bluetooth":"off","canTurnOn":false}`, "Turn on Bluetooth on this Mac.", "<nil>"},
		{`{"link":"FIDO:/1","bluetooth":"denied","canTurnOn":false}`, "Project B can't use Bluetooth. Allow it in System Settings › Privacy & Security › Bluetooth.", "map[action:bluetoothSettings label:Open System Settings]"},
	} {
		r.e.Fire(1, 0, "passkeyStep", "phone "+c.carries)
		var b map[string]any
		r.dialogs(func(l []any) bool {
			p, _ = l[0].(map[string]any)["passkey"].(map[string]any)
			b, _ = p["bluetooth"].(map[string]any)
			return b["words"] == c.words
		})
		if fmt.Sprint(b["button"]) != c.button {
			t.Fatalf("%s: button = %v", c.carries, b["button"])
		}
	}
	r.passkeyAnswer(id, "bluetoothOn", "")
	r.passkeyAnswer(id, "bluetoothSettings", "")
	// An action Go doesn't know goes nowhere.
	r.passkeyAnswer(id, "format", "")

	r.e.Fire(1, 0, "passkeyStep", "phoneBusy")
	d, p = r.passkey("phoneBusy")
	words(t, d, "Check your phone", "Follow the steps on your phone.", "", "Cancel")
	if p["other"] != nil || p["code"] != nil {
		t.Fatalf("phoneBusy = %v", p)
	}

	// Back to the key from the phone.
	r.e.Fire(1, 0, "passkeyStep", `phone {"link":"FIDO:/1","bluetooth":"on"}`)
	r.passkey("phone")
	r.passkeyAnswer(id, "key", "")
	r.passkey("key")
	r.e.Settle()
	var actions []string
	for _, a := range r.e.PasskeyAnswers() {
		actions = append(actions, a.Action)
	}
	if got := strings.Join(actions, " "); got != "cancel phone bluetoothOn bluetoothSettings key" {
		t.Fatalf("actions = %s", got)
	}
}

// A shell that doesn't draw a passkey's steps answers as it does any dialog: its no
// cancels the request, its yes does nothing. The tab navigating drops the request.
func TestAPasskeyRequestEndsAsADialogDoes(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	r.e.Fire(1, 0, "passkey", "get")
	d, _ := r.passkey("key")
	r.shell(fmt.Sprintf(`{"t":"answerDialog","id":%v,"ok":true}`, d["id"]))
	r.e.Settle()
	if got := r.e.PasskeyAnswers(); len(got) != 0 {
		t.Fatalf("a yes says nothing: %v", got)
	}
	r.shell(fmt.Sprintf(`{"t":"answerDialog","id":%v,"ok":false}`, d["id"]))
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	r.e.Settle()
	if got := r.e.PasskeyAnswers(); len(got) != 1 || got[0].Action != "cancel" {
		t.Fatalf("a no cancels: %v", got)
	}
	// An answer to a request that has gone does nothing.
	r.passkeyAnswer(d["id"], "pin", "1234")

	r.e.Fire(1, 0, "passkey", "create")
	r.passkey("key")
	r.shell(`{"t":"navigate","text":"https://b.test/"}`)
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	r.e.Settle()
	if got := r.e.PasskeyAnswers(); len(got) != 1 {
		t.Fatalf("a dropped request gets no answer: %v", got)
	}
}

// A key's PIN crosses from the shell to the engine and is written down nowhere: not in
// the timeline, and not in what the shells are sent.
func TestAPasskeyPINLeavesNoTrace(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	_, from := timeline.Since(0)

	const pin = "pin-4c1e77-never-logged"
	r.e.Fire(1, 0, "passkey", "get")
	d, _ := r.passkey("key")
	r.e.Fire(1, 0, "passkeyStep", `pin {"reason":"enter","error":"","attempts":8,"minLength":4}`)
	r.passkey("pin")
	r.passkeyAnswer(d["id"], "pin", pin)
	// A malformed answer, which the shell drops with a warning, carries none of it either.
	r.shell(`{"t":"answerPasskey","id":"` + pin + `"}`)
	r.e.Fire(1, 0, "passkeyStep", "touch")
	sent, _ := r.passkey("touch")
	r.e.Settle()
	if got := r.e.PasskeyAnswers(); len(got) != 1 || got[0].Text != pin {
		t.Fatalf("the engine has the PIN: %v", got)
	}
	if b, _ := json.Marshal(sent); strings.Contains(string(b), pin) {
		t.Fatalf("the shells were sent the PIN: %s", b)
	}
	entries, _ := timeline.Since(from)
	if !strings.Contains(fmt.Sprintf("%+v", entries), "bad message") {
		t.Fatalf("the warning should be in the timeline: %+v", entries)
	}
	for _, e := range entries {
		if strings.Contains(fmt.Sprintf("%+v", e), pin) {
			t.Fatalf("the timeline holds the PIN: %+v", e)
		}
	}
}
