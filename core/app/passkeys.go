package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"rsc.io/qr"

	"projectb/engine"
	"projectb/timeline"
)

// A page's passkey request (docs/passkeys.md): a page dialog of kind passkey that changes
// in place. The engine says what the person must do now, a step; Go words it, with its
// buttons, and the shells draw it and send back what the person did, which Go passes to
// the engine. A PIN crosses from the shell to the engine and is kept nowhere.

// passkeyStep is what a step's card has beyond a dialog's title, words and two buttons.
// The dialog's OK is the card's primary button, "" for none, and its Cancel the button
// beside it; Host is the site, for a shell to set in bold.
type passkeyStep struct {
	// The step's name, as the engine names it.
	Step string `json:"step"`
	// Greater for each step Go sends, of any request: a shell starts a card afresh, its
	// fields empty and the keyboard on its first control, when this changes.
	Seq int `json:"seq"`
	// What the primary button sends: pin, retry or cancel.
	Primary string `json:"primary,omitempty"`
	// A quiet button on the left that goes to another way: its label and what it sends.
	Other *passkeyButton `json:"other,omitempty"`
	// The card is the wider one, 440px, for a long quiet button beside two others.
	Wide bool `json:"wide,omitempty"`
	// A PIN step's fields.
	Pin *passkeyPin `json:"pin,omitempty"`
	// An account step's rows, in the key's order; a row sends account with its index.
	Accounts []passkeyAccount `json:"accounts,omitempty"`
	// A phone step's QR code, a row of 0 and 1 for each row of modules with no quiet
	// zone, and the line under it when Bluetooth isn't on.
	Code      []string          `json:"code,omitempty"`
	Bluetooth *passkeyBluetooth `json:"bluetooth,omitempty"`
}

type passkeyButton struct {
	Label  string `json:"label"`
	Action string `json:"action"`
}

// passkeyPin is a PIN step's fields: their labels, one or a PIN and its confirmation;
// the words under the first when the key refused the last one; and what a shell says
// under a field of its own accord, for a PIN too short or two that differ.
type passkeyPin struct {
	Labels    []string `json:"labels"`
	Error     string   `json:"error,omitempty"`
	MinLength int      `json:"minLength"`
	Short     string   `json:"short"`
	Mismatch  string   `json:"mismatch,omitempty"`
}

type passkeyAccount struct {
	Title     string `json:"title"`
	Secondary string `json:"secondary,omitempty"`
	// The key has no name for it: the title is ours, for a shell to set apart.
	Unnamed bool `json:"unnamed,omitempty"`
}

type passkeyBluetooth struct {
	Words  string         `json:"words"`
	Button *passkeyButton `json:"button,omitempty"`
}

// tries words how many tries are left.
func tries(n int) string {
	if n == 1 {
		return "1 try left"
	}
	return fmt.Sprintf("%d tries left", n)
}

// passkeyError is an error step's title and words for the engine's reason, and whether
// trying again can help.
func passkeyError(reason, site string) (title, words string, again bool) {
	switch reason {
	case "timeout":
		return "That took too long", site + " stopped waiting. Try again when your key or phone is ready.", true
	case "not-registered":
		return "No passkey on this key", "This security key has no passkey for " + site + ". Try another key or a phone.", true
	case "already-registered":
		return "Already saved", "This security key already has a passkey for " + site + ".", false
	case "pin-soft-blocked":
		return "Too many wrong PINs", "Unplug your security key, plug it in again, and try again.", true
	case "pin-hard-blocked":
		return "Your security key is locked", "Too many wrong PINs. The key has to be reset before it can be used again, which erases its passkeys.", false
	case "key-removed":
		return "Your security key was unplugged", "Plug it in and try again.", true
	case "no-passkey-support", "no-uv", "no-large-blob", "no-algorithm":
		return "Use another key", "This security key can't do what " + site + " asks. Try another key or a phone.", true
	case "full":
		return "Your security key is full", "It has no room for another passkey. Remove one with the key maker's app, or use another key.", false
	case "phone-failed":
		return "Couldn't reach your phone", "Make sure Bluetooth is on for this Mac and your phone, and try again.", true
	}
	return "Something went wrong", "The request didn't finish.", true
}

// accountRow is an account's row: its display name with its name under it, the name alone
// when there is no other, and words of ours when it has neither.
func accountRow(name, displayName string) passkeyAccount {
	row := passkeyAccount{Title: displayName, Secondary: name}
	if row.Title == "" || row.Title == row.Secondary {
		row = passkeyAccount{Title: name}
	}
	if row.Title == "" {
		row.Title, row.Unnamed = "No name", true
	}
	return row
}

// qrRows are the modules of text's QR code, a row of 0 and 1 each, or none when it can't
// be made.
func qrRows(text string) []string {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		timeline.Warnf("passkey: the phone's code: %v", err)
		return nil
	}
	rows := make([]string, code.Size)
	for y := range rows {
		var row strings.Builder
		for x := 0; x < code.Size; x++ {
			if code.Black(x, y) {
				row.WriteByte('1')
			} else {
				row.WriteByte('0')
			}
		}
		rows[y] = row.String()
	}
	return rows
}

// passkeyDialog words a step of the request d, a dialog of kind passkey, from what the
// engine says it carries.
func passkeyDialog(d pageDialog, step, carries string) pageDialog {
	var c struct {
		Request   string `json:"request"`
		Site      string `json:"site"`
		Phone     bool   `json:"phone"`
		Reason    string `json:"reason"`
		Error     string `json:"error"`
		Attempts  int    `json:"attempts"`
		MinLength int    `json:"minLength"`
		Accounts  []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"accounts"`
		Link      string `json:"link"`
		Bluetooth string `json:"bluetooth"`
		CanTurnOn bool   `json:"canTurnOn"`
	}
	if err := json.Unmarshal([]byte(carries), &c); err != nil {
		timeline.Warnf("passkey: a bad %s step: %v", step, err)
	}
	site := d.Host
	p := pageDialog{ID: d.ID, Tab: d.Tab, Kind: d.Kind, Host: site, Cancel: "Cancel"}
	s := &passkeyStep{Step: step}
	p.Passkey = s
	switch step {
	case "key":
		p.Title, p.Message = "Sign in with a passkey", site+" wants you to sign in with your security key. Plug it in and touch it."
		if c.Request == "create" {
			p.Title, p.Message = "Create a passkey", site+" wants to save a passkey on your security key. Plug it in and touch it."
		}
		if c.Phone {
			s.Other = &passkeyButton{Label: "Use a phone", Action: "phone"}
		}
	case "pin":
		s.Primary = "pin"
		pin := &passkeyPin{MinLength: c.MinLength, Short: fmt.Sprintf("At least %d characters", c.MinLength)}
		s.Pin = pin
		switch c.Reason {
		case "set":
			p.Title, p.OK = "Set a PIN for your security key", "Set PIN"
			p.Message = fmt.Sprintf("%s needs your key to have a PIN. At least %d characters.", site, c.MinLength)
			pin.Labels, pin.Mismatch = []string{"PIN", "Confirm PIN"}, "The PINs don't match"
		case "change":
			p.Title, p.OK = "Change your security key's PIN", "Change PIN"
			p.Message = "Your key needs a new PIN before it can be used."
			pin.Labels, pin.Mismatch = []string{"New PIN", "Confirm new PIN"}, "The PINs don't match"
		default:
			p.Title, p.OK = "Enter your security key's PIN", "Next"
			pin.Labels = []string{"PIN"}
		}
		switch c.Error {
		case "wrong":
			pin.Error = "Wrong PIN. " + tries(c.Attempts) + "."
			if c.Attempts == 1 {
				pin.Error = "Wrong PIN. 1 try left before the key locks."
			}
		case "uv-locked":
			pin.Error = "Too many fingerprint tries. Enter your PIN instead."
		case "short":
			pin.Error = pin.Short
		case "invalid":
			pin.Error = "A PIN can't have those characters."
		case "same":
			pin.Error = "The new PIN must be different from your current one."
		}
	case "touch":
		p.Title, p.Message = "Touch your security key", "Touch your security key again to finish."
		if c.Attempts > 0 {
			p.Message = "That fingerprint didn't match. Touch your security key again. " + tries(c.Attempts) + "."
		}
	case "account":
		p.Title = "Choose an account"
		p.Message = fmt.Sprintf("Your security key has %d accounts for %s.", len(c.Accounts), site)
		for _, a := range c.Accounts {
			s.Accounts = append(s.Accounts, accountRow(a.Name, a.DisplayName))
		}
	case "phone":
		p.Title, p.Message = "Use your phone", "Scan this code with your phone's camera."
		s.Code = qrRows(c.Link)
		s.Other = &passkeyButton{Label: "Use a security key", Action: "key"}
		switch {
		case c.Bluetooth == "denied":
			s.Bluetooth = &passkeyBluetooth{
				Words:  "Project B can't use Bluetooth. Allow it in System Settings › Privacy & Security › Bluetooth.",
				Button: &passkeyButton{Label: "Open System Settings", Action: "bluetoothSettings"},
			}
		case c.Bluetooth == "off" && c.CanTurnOn:
			s.Bluetooth = &passkeyBluetooth{Words: "Bluetooth is off.", Button: &passkeyButton{Label: "Turn on Bluetooth", Action: "bluetoothOn"}}
		case c.Bluetooth == "off":
			s.Bluetooth = &passkeyBluetooth{Words: "Turn on Bluetooth on this Mac."}
		}
	case "phoneBusy":
		p.Title, p.Message = "Check your phone", "Follow the steps on your phone."
	default:
		// An error, and any step this build doesn't know, which can only be closed.
		var again bool
		p.Title, p.Message, again = passkeyError(c.Reason, site)
		if again {
			p.OK, p.Cancel, s.Primary = "Try again", "Close", "retry"
		} else {
			p.OK, p.Cancel, s.Primary = "Close", "", "cancel"
		}
	}
	return p
}

// passkeyCards keeps the passkey requests among the pages' dialogs: each from the engine's
// word of it to its first step, when the shells first see it.
type passkeyCards struct {
	p *pageDialogs
	// The requests with no card showing, which no shell has: one with no step yet, and
	// one of ours while the OS checks the person.
	waiting map[engine.DialogID]pageDialog
	seq     int
	// The requests our own passkeys can answer (passkeys_own.go).
	own map[engine.DialogID]*ownPasskey
}

// opened keeps a request until its first step: d's message is the site.
func (k *passkeyCards) opened(d engine.Dialog) {
	k.waiting[d.ID] = pageDialog{ID: d.ID, Tab: d.Tab, Kind: d.Kind, Host: d.Message}
}

// put words the request's card anew, in the place it had, or last when it had none.
func (k *passkeyCards) put(id engine.DialogID, word func(d pageDialog) pageDialog) {
	worded := func(d pageDialog) pageDialog {
		k.seq++
		p := word(d)
		p.Passkey.Seq = k.seq
		return p
	}
	if d, ok := k.waiting[id]; ok {
		delete(k.waiting, id)
		k.p.list = append(k.p.list, worded(d))
		k.p.changed()
		return
	}
	for i, d := range k.p.list {
		if d.ID == id && d.Kind == "passkey" {
			k.p.list[i] = worded(d)
			k.p.changed()
			return
		}
	}
}

// hide takes the request's card off the shells' list until its next step.
func (k *passkeyCards) hide(id engine.DialogID) {
	for i, d := range k.p.list {
		if d.ID == id && d.Kind == "passkey" {
			k.waiting[id] = pageDialog{ID: d.ID, Tab: d.Tab, Kind: d.Kind, Host: d.Host}
			k.p.list = append(k.p.list[:i:i], k.p.list[i+1:]...)
			k.p.changed()
			return
		}
	}
}

// siteOf is the site of request id, its RP ID.
func (k *passkeyCards) siteOf(id engine.DialogID) string {
	if d, ok := k.waiting[id]; ok {
		return d.Host
	}
	for _, d := range k.p.list {
		if d.ID == id {
			return d.Host
		}
	}
	return ""
}

// stepOf is the step request id's card shows, "" when it shows none.
func (k *passkeyCards) stepOf(id engine.DialogID) string {
	for _, d := range k.p.list {
		if d.ID == id && d.Passkey != nil {
			return d.Passkey.Step
		}
	}
	return ""
}

// show puts the request's card at a step Go chose: one of ours, or the engine's last key
// step.
func (k *passkeyCards) show(id engine.DialogID, step string, o *ownPasskey) {
	k.put(id, func(d pageDialog) pageDialog {
		if step == "key" {
			return passkeyDialog(d, "key", o.key)
		}
		return ownDialog(d, step, o)
	})
}

// step puts the request's card at a step of the engine's, unless the step is for our own
// passkeys.
func (k *passkeyCards) step(id engine.DialogID, step, carries string) {
	if k.ownStep(id, step, carries) {
		return
	}
	k.put(id, func(d pageDialog) pageDialog { return passkeyDialog(d, step, carries) })
}

// answer passes what the person did at request id's step to the engine, unless the card
// is one of ours. The card stays as it is: the engine's next step, or the request's end,
// changes it.
func (k *passkeyCards) answer(msg []byte) {
	var r struct {
		ID     engine.DialogID `json:"id"`
		Action string          `json:"action"`
		Text   string          `json:"text"`
	}
	json.Unmarshal(msg, &r)
	if k.p.kindOf(r.ID) != "passkey" {
		return
	}
	if k.ownAnswer(r.ID, k.stepOf(r.ID), r.Action, r.Text) {
		return
	}
	switch r.Action {
	case "cancel", "pin", "account", "phone", "key", "bluetoothOn", "bluetoothSettings", "retry":
		k.p.e.AnswerPasskey(r.ID, r.Action, r.Text)
	}
}

// answered is a shell answering request id's card as a plain dialog: yes is the step's
// primary button, and no is Cancel.
func (k *passkeyCards) answered(id engine.DialogID, yes bool) {
	action := "cancel"
	if yes {
		for _, d := range k.p.list {
			if d.ID == id && d.Passkey != nil {
				action = d.Passkey.Primary
			}
		}
		// A PIN needs the card's own field, and a step with no primary has nothing to press.
		if action == "" || action == "pin" {
			return
		}
	}
	msg, _ := json.Marshal(map[string]any{"id": id, "action": action})
	k.answer(msg)
}

func (a *App) PasskeyStep(id engine.DialogID, step, carries string) {
	a.dialogs.passkeys.step(id, step, carries)
}
