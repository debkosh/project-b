package app

import (
	"encoding/json"
	"errors"
	"strconv"

	"projectb/engine"
	"projectb/forms"
	"projectb/passwords"
	"projectb/shell"
	"projectb/tabs"
	"projectb/timeline"
)

// Filling saved passwords into pages (docs/password-autofill.md). The engine carries each
// frame's form script's messages here, stamped with what the engine itself knows of the
// frame; core/passwords decides; and the shell of the tab's window lists the logins it
// offers, beside the field, outside the page. A pick fills. Nothing asks the device to
// check the person.
//
// What the shell hears: passwordOffer {passwordOffer}, the logins to list at a field of a
// tab's page, or null when there is none any more; the snapshot has it too. What it says:
// passwordPick {offer, login}, passwordAt {offer, at}, the login the pointer is over,
// passwordDismiss {offer}, and passwordsManage, which opens Settings at Passwords.

// stampOf is the page protocol's stamp for a frame as the engine named it.
func stampOf(f engine.FormFrame) forms.Stamp {
	return forms.Stamp{Tab: strconv.FormatInt(int64(f.Tab), 10), Frame: f.Frame, Document: f.Document, Origin: f.Origin,
		Top: f.Top, CrossOrigin: f.CrossOrigin, Opaque: f.Opaque, Secure: f.Secure}
}

func tabOf(stamp forms.Stamp) engine.TabID {
	n, _ := strconv.ParseInt(stamp.Tab, 10, 64)
	return engine.TabID(n)
}

// startFilling gives the engine the form script and sets filling up over the profile's
// saved logins. The Filler's work waits on the store, so it runs on a goroutine of its
// own, a message at a time in the order they came, and comes back to the main thread for
// the engine and the shell.
func (a *App) startFilling(e engine.Engine) {
	e.SetFormScript(forms.Script)
	work := make(chan func(), 256)
	go func() {
		for f := range work {
			f()
		}
	}()
	// The offer showing, by its tab, for a shell that starts while one shows.
	var showing *passwords.Offer
	carry := func(to forms.Stamp, kind string, payload any) bool {
		b, err := json.Marshal(payload)
		if err != nil {
			return false
		}
		went := make(chan bool, 1)
		e.Post(func() {
			went <- e.SendForm(engine.FormFrame{Tab: tabOf(to), Frame: to.Frame, Document: to.Document}, kind, string(b))
		})
		return <-went
	}
	offered := func(o *passwords.Offer) {
		e.Post(func() {
			was := showing
			showing = o
			// To the window of the offer's tab: the one that had the last, to take it away.
			at := o
			if at == nil {
				at = was
			}
			if at != nil {
				a.Shell.SendTo(a.Tabs.WindowOf(tabOf(forms.Stamp{Tab: at.Tab})), map[string]any{"t": "passwordOffer", "passwordOffer": o})
			}
		})
	}
	a.Fill = passwords.NewFiller(a.Passwords, carry, offered)
	a.fillWork = work

	// A tab that goes on to another page, or closes, takes its offer with it.
	at := map[engine.TabID]string{}
	a.Tabs.OnChanged(func(t tabs.Tab) {
		if at[t.ID] != t.URL {
			at[t.ID] = t.URL
			tab := strconv.FormatInt(int64(t.ID), 10)
			work <- func() { a.Fill.Navigated(tab) }
		}
	})
	a.Tabs.OnClosed(func(t tabs.Tab) {
		delete(at, t.ID)
		tab := strconv.FormatInt(int64(t.ID), 10)
		work <- func() { a.Fill.Left(tab, "") }
	})
	a.Shell.AddPart(shell.Part{
		Snapshot: func(into map[string]any) { into["passwordOffer"] = showing },
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			var c struct {
				Offer int64  `json:"offer"`
				Login string `json:"login"`
				At    int    `json:"at"`
			}
			switch t {
			case "passwordPick":
				json.Unmarshal(msg, &c)
				work <- func() {
					if err := a.Fill.Pick(c.Offer, c.Login); err != nil && !errors.Is(err, passwords.ErrNoOffer) {
						timeline.Errorf("passwords: filling: %v", err)
					}
				}
			case "passwordAt":
				json.Unmarshal(msg, &c)
				work <- func() { a.Fill.At(c.Offer, c.At) }
			case "passwordDismiss":
				json.Unmarshal(msg, &c)
				work <- func() { a.Fill.Dismiss(c.Offer) }
			case "passwordsManage":
				a.Settings.OpenSection(w, "passwords")
			default:
				return false
			}
			return true
		},
	})
}

// FormMessage is a message from a frame's form script. A pressed field's box is the
// frame's own: the frame's place in the tab's page is added, so the shell can list beside it.
func (a *App) FormMessage(from engine.FormFrame, kind, payload string) {
	if a.Fill == nil {
		return
	}
	stamp := stampOf(from)
	body := []byte(payload)
	if kind == "pressed" {
		var p forms.Pressed
		if json.Unmarshal(body, &p) != nil {
			return
		}
		p.Box.X += from.X
		p.Box.Y += from.Y
		body, _ = json.Marshal(p)
	}
	a.fillWork <- func() { a.Fill.Message(stamp, kind, body) }
}
