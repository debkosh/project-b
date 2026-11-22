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
//
// Saving: passwordSave {passwordSave}, the offer to save a login a person just signed in
// with, or to update a saved one's password, or null; the snapshot has it too. The shell
// says passwordSaveAnswer {offer, choice}, "save", "never" or "dismiss", and
// passwordSaveShow {offer}, which is answered to it alone with passwordSaveShown
// {passwordSaveShown: {offer, password}}: what the person typed a moment ago, for them to
// look at before they agree. It is null again once the offer has gone.

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
	// The offer to save, as the list's offer: to the window of its tab.
	var saving *passwords.SaveOffer
	a.Fill.OnSave(func(o *passwords.SaveOffer) {
		e.Post(func() {
			was := saving
			saving = o
			at := o
			if at == nil {
				at = was
			}
			if at != nil {
				w := a.Tabs.WindowOf(tabOf(forms.Stamp{Tab: at.Tab}))
				if o == nil {
					a.Shell.SendTo(w, map[string]any{"t": "passwordSaveShown", "passwordSaveShown": nil})
				}
				a.Shell.SendTo(w, map[string]any{"t": "passwordSave", "passwordSave": o})
			}
		})
	})

	// A tab that goes on to another page, or closes, takes its offer with it.
	at := map[engine.TabID]string{}
	a.Tabs.OnChanged(func(t tabs.Tab) {
		if at[t.ID] != t.URL {
			at[t.ID] = t.URL
			tab := strconv.FormatInt(int64(t.ID), 10)
			address := t.URL
			work <- func() { a.Fill.Navigated(tab, address) }
		}
		// A page done loading is when what the tab sent before is looked at, for saving.
		if !t.Loading {
			tab := strconv.FormatInt(int64(t.ID), 10)
			work <- func() { a.Fill.Loaded(tab) }
		}
	})
	a.Tabs.OnClosed(func(t tabs.Tab) {
		delete(at, t.ID)
		tab := strconv.FormatInt(int64(t.ID), 10)
		work <- func() { a.Fill.Left(tab, "") }
	})
	a.Shell.AddPart(shell.Part{
		Snapshot: func(into map[string]any) { into["passwordOffer"], into["passwordSave"] = showing, saving },
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			var c struct {
				Offer  int64  `json:"offer"`
				Login  string `json:"login"`
				At     int    `json:"at"`
				Choice string `json:"choice"`
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
			case "passwordSaveAnswer":
				json.Unmarshal(msg, &c)
				work <- func() {
					if err := a.Fill.Answer(c.Offer, c.Choice); err != nil && !errors.Is(err, passwords.ErrNoOffer) {
						timeline.Errorf("passwords: saving: %v", err)
					}
				}
			case "passwordSaveShow":
				json.Unmarshal(msg, &c)
				a.Shell.SendTo(w, map[string]any{"t": "passwordSaveShown", "passwordSaveShown": map[string]any{"offer": c.Offer, "password": a.Fill.Typed(c.Offer)}})
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
