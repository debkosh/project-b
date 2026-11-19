package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"

	"projectb/engine"
	"projectb/passkeys"
	"projectb/system"
	"projectb/timeline"
)

// Our own passkeys in a page's request (docs/passkeys.md, Our own passkeys). The engine
// says, in its key step, that ours can answer the request; Go then offers ours first: Save
// for a new passkey, the site's passkeys here for a sign-in. The person's press sends the
// request to ours in the engine, which asks Go to make or to sign with two steps of its
// own, ownMake and ownSign; Go checks the person, makes or signs in core/passkeys, and
// answers made, signed or ownFailed. While the OS checks the person the card is off the
// shells' list: the OS's own sheet says what is asked.

// ownPasskey is what Go keeps of a request ours can answer.
type ownPasskey struct {
	// The request as the engine's last key step said it: create or get, the credential IDs
	// a sign-in allows, whether ours is the only way, as on a phone, and the step's own
	// JSON, for the way to a security key or a phone.
	request string
	allow   [][]byte
	only    bool
	key     string
	// declined: the person chose a security key or a phone, and the engine's key steps
	// show as they are.
	declined bool
	// rows are the passkeys the own step shows, and chosen the one pressed.
	rows   []passkeys.Passkey
	chosen []byte
	// failed is what the engine hears once the person closes our error's card.
	failed string
	// cancel ends whatever of ours still waits for this request.
	ctx    context.Context
	cancel context.CancelFunc
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// unb64 reads a page's base64url, with its padding or without.
func unb64(s string) []byte {
	for len(s) > 0 && s[len(s)-1] == '=' {
		s = s[:len(s)-1]
	}
	b, _ := base64.RawURLEncoding.DecodeString(s)
	return b
}

func unb64s(list []string) [][]byte {
	out := make([][]byte, 0, len(list))
	for _, s := range list {
		out = append(out, unb64(s))
	}
	return out
}

// verifyPerson is core/passkeys' check of the person, through the OS's own sheet: it asks
// on the main thread, and waits off it.
func verifyPerson(sys system.System, post func(func())) passkeys.Verify {
	return func(ctx context.Context, why passkeys.Why, rpID string) error {
		check := system.PersonCheck{Reason: "use a passkey for " + rpID, Title: "Use your passkey", Subtitle: rpID}
		if why == passkeys.ToSave {
			check = system.PersonCheck{Reason: "save a passkey for " + rpID, Title: "Save your passkey", Subtitle: rpID}
		}
		got := make(chan system.Verdict, 1)
		var cancel func()
		post(func() { cancel = sys.VerifyPerson(check, func(v system.Verdict) { got <- v }) })
		select {
		case v := <-got:
			switch v {
			case system.Verified:
				return nil
			case system.NoScreenLock:
				return passkeys.ErrNoScreenLock
			}
			return passkeys.ErrDenied
		case <-ctx.Done():
			// Posted after the asking, so cancel is set by now.
			post(func() { cancel() })
			return passkeys.ErrCancelled
		}
	}
}

// ownOf is the request's state while ours can answer it, made at its first key step.
func (k *passkeyCards) ownOf(id engine.DialogID) *ownPasskey {
	o := k.own[id]
	if o == nil {
		o = &ownPasskey{}
		o.ctx, o.cancel = context.WithCancel(context.Background())
		k.own[id] = o
	}
	return o
}

// closed forgets a request that ended, and ends what of ours waited for it.
func (k *passkeyCards) closed(id engine.DialogID) {
	if o := k.own[id]; o != nil {
		o.cancel()
		delete(k.own, id)
	}
}

// off runs work off the main thread and then, with what it returned, on it, unless the
// request ended meanwhile.
func (k *passkeyCards) off(id engine.DialogID, o *ownPasskey, work func() func()) {
	go func() {
		then := work()
		k.p.e.Post(func() {
			if k.own[id] == o {
				then()
			}
		})
	}()
}

// offer is the engine's key step for a request ours can answer: Save for a new passkey;
// for a sign-in the site's passkeys here, or, with none, the key step as it is.
func (k *passkeyCards) offer(id engine.DialogID, o *ownPasskey) {
	if o.request == "create" {
		k.show(id, "save", o)
		return
	}
	site := k.siteOf(id)
	k.off(id, o, func() func() {
		n, err := k.p.a.Passkeys.Count(site, o.allow)
		if err != nil || n == 0 {
			return func() { k.none(id, o) }
		}
		// Nobody has pressed anything yet: a locked profile isn't asked to unlock.
		rows, err := k.p.a.Passkeys.List(o.ctx, false, site, o.allow)
		return func() { k.listed(id, o, rows, err) }
	})
}

// none is a sign-in for a site with no passkey here: a security key or a phone, where
// there is one, and else the page hears no at once.
func (k *passkeyCards) none(id engine.DialogID, o *ownPasskey) {
	if o.only {
		k.p.e.AnswerPasskey(id, "cancel", "")
		return
	}
	o.declined = true
	k.show(id, "key", o)
}

// listed shows the site's passkeys, or, for a locked profile, the way to unlock: a card
// with Unlock… beside the way to a security key, or on a phone, where there is no other
// way, the unlock Dialog at once.
func (k *passkeyCards) listed(id engine.DialogID, o *ownPasskey, rows []passkeys.Passkey, err error) {
	switch {
	case errors.Is(err, passkeys.ErrLocked) && o.only:
		k.unlock(id, o)
	case errors.Is(err, passkeys.ErrLocked):
		k.show(id, "ownLocked", o)
	case err != nil || len(rows) == 0:
		k.none(id, o)
	default:
		o.rows = rows
		k.show(id, "own", o)
	}
}

// unlock lists the site's passkeys as a person asked to, which brings Profile Lock's
// unlock Dialog. Cancel there leaves the locked card, or on a phone ends the request.
func (k *passkeyCards) unlock(id engine.DialogID, o *ownPasskey) {
	site := k.siteOf(id)
	k.off(id, o, func() func() {
		rows, err := k.p.a.Passkeys.List(o.ctx, true, site, o.allow)
		return func() {
			if errors.Is(err, passkeys.ErrLocked) {
				if o.only {
					k.p.e.AnswerPasskey(id, "cancel", "")
				}
				return
			}
			k.listed(id, o, rows, err)
		}
	})
}

// send puts the request to ours in the engine, once the person pressed Save or a
// passkey's row, and takes the card off the shells' list: the OS's check comes next.
func (k *passkeyCards) send(id engine.DialogID) {
	k.hide(id)
	k.p.e.AnswerPasskey(id, "own", "")
}

// fail ends what the engine asked of ours. A reason the person should read shows first,
// as a card with Close alone, and the engine hears when it is closed.
func (k *passkeyCards) fail(id engine.DialogID, o *ownPasskey, err error) {
	reason := "other"
	switch {
	case errors.Is(err, passkeys.ErrCancelled), errors.Is(err, passkeys.ErrDenied), errors.Is(err, passkeys.ErrLocked):
		// The person said no, as with Cancel: the page hears at once.
		k.p.e.AnswerPasskey(id, "ownFailed", "denied")
		return
	case errors.Is(err, passkeys.ErrExcluded):
		reason = "excluded"
	case errors.Is(err, passkeys.ErrNoScreenLock):
		reason = "no-screen-lock"
	default:
		timeline.Errorf("passkey: %v", err)
	}
	o.failed = reason
	k.show(id, "ownError", o)
}

// make is the engine's ownMake: the request reached ours, with who the passkey is for and
// the passkeys the site already has.
func (k *passkeyCards) make(id engine.DialogID, o *ownPasskey, carries string) {
	var c struct {
		User struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"user"`
		Exclude []string `json:"exclude"`
	}
	json.Unmarshal([]byte(carries), &c)
	site, synced := k.siteOf(id), k.p.a.Sync.Status().On
	user := passkeys.User{Handle: unb64(c.User.ID), Name: c.User.Name, DisplayName: c.User.DisplayName}
	k.off(id, o, func() func() {
		made, cose, err := k.p.a.Passkeys.Make(o.ctx, true, site, user, unb64s(c.Exclude))
		return func() {
			if err != nil {
				k.fail(id, o, err)
				return
			}
			answer, _ := json.Marshal(map[string]string{"credentialId": b64(made), "authData": b64(passkeys.AttestedData(site, made, cose, synced))})
			k.p.e.AnswerPasskey(id, "made", string(answer))
		}
	})
}

// sign is the engine's ownSign: the request reached ours, with the hash of the client
// data to sign after the authenticator data.
func (k *passkeyCards) sign(id engine.DialogID, o *ownPasskey, carries string) {
	var c struct {
		ClientDataHash string `json:"clientDataHash"`
	}
	json.Unmarshal([]byte(carries), &c)
	site, chosen := k.siteOf(id), o.chosen
	data := passkeys.AssertionData(site, k.p.a.Sync.Status().On)
	k.off(id, o, func() func() {
		signature, handle, err := k.p.a.Passkeys.Sign(o.ctx, true, site, chosen, append(append([]byte{}, data...), unb64(c.ClientDataHash)...))
		return func() {
			if err != nil {
				k.fail(id, o, err)
				return
			}
			answer, _ := json.Marshal(map[string]string{"credentialId": b64(chosen), "authData": b64(data), "signature": b64(signature), "userHandle": b64(handle)})
			k.p.e.AnswerPasskey(id, "signed", string(answer))
		}
	})
}

// ownStep takes a step of the engine's that is ours to handle, and says whether it was.
func (k *passkeyCards) ownStep(id engine.DialogID, step, carries string) bool {
	switch step {
	case "key":
		var c struct {
			Request string   `json:"request"`
			Own     bool     `json:"own"`
			Only    bool     `json:"only"`
			Allow   []string `json:"allow"`
		}
		json.Unmarshal([]byte(carries), &c)
		if !c.Own {
			return false
		}
		o := k.ownOf(id)
		o.request, o.only, o.allow, o.key = c.Request, c.Only, unb64s(c.Allow), carries
		if o.declined {
			return false
		}
		k.offer(id, o)
		return true
	case "ownMake", "ownSign":
		o := k.own[id]
		if o == nil {
			k.p.e.AnswerPasskey(id, "ownFailed", "denied")
		} else if step == "ownMake" {
			k.make(id, o, carries)
		} else {
			k.sign(id, o, carries)
		}
		return true
	}
	return false
}

// ownAnswer takes what the person did at a card of ours, and says whether it was one.
func (k *passkeyCards) ownAnswer(id engine.DialogID, step, action, text string) bool {
	o := k.own[id]
	if o == nil {
		return false
	}
	switch {
	case step == "save" && action == "own":
		k.send(id)
	case step == "own" && action == "account":
		i, err := strconv.Atoi(text)
		if err != nil || i < 0 || i >= len(o.rows) {
			return true
		}
		o.chosen = o.rows[i].ID
		k.send(id)
	case step == "ownLocked" && action == "unlock":
		k.unlock(id, o)
	case (step == "save" || step == "own" || step == "ownLocked") && action == "other":
		// To the engine's key step, which waited all along.
		o.declined = true
		k.show(id, "key", o)
	case step == "ownError":
		k.p.e.AnswerPasskey(id, "ownFailed", o.failed)
	case action == "retry":
		// The engine starts the request over, and its key step offers ours again.
		o.declined, o.chosen, o.rows = false, nil, nil
		return false
	default:
		return false
	}
	return true
}

// ownDialog words a card of ours for the request d.
func ownDialog(d pageDialog, step string, o *ownPasskey) pageDialog {
	site := d.Host
	p := pageDialog{ID: d.ID, Tab: d.Tab, Kind: d.Kind, Host: site, Cancel: "Cancel"}
	s := &passkeyStep{Step: step}
	p.Passkey = s
	other := &passkeyButton{Label: "Use a security key or phone", Action: "other"}
	if o.only {
		other = nil
	}
	switch step {
	case "save":
		p.Title, p.Message, p.OK = "Create a passkey", "Save a passkey for "+site+" in Project B?", "Save"
		s.Primary, s.Other = "own", other
	case "own":
		p.Title = "Sign in to " + site
		s.Other = other
		for _, r := range o.rows {
			s.Accounts = append(s.Accounts, accountRow(r.UserName, r.DisplayName))
		}
	case "ownLocked":
		p.Title, p.Message, p.OK = "Sign in to "+site, "Your passkeys are locked.", "Unlock…"
		s.Primary, s.Other = "unlock", other
	case "ownError":
		p.OK, p.Cancel, s.Primary = "Close", "", "cancel"
		switch o.failed {
		case "excluded":
			p.Title, p.Message = "Already saved", "Project B already has a passkey for "+site+" for this account."
		case "no-screen-lock":
			p.Title, p.Message = "Set a password first", "Project B checks it's you with your Mac's password before using a passkey. Set one in System Settings, then try again."
			if o.only {
				p.Title, p.Message = "Set a screen lock first", "Project B checks it's you with your screen lock before using a passkey. Set one in your phone's Settings, then try again."
			}
		default:
			p.Title, p.Message = "Something went wrong", "The request didn't finish."
		}
	}
	return p
}
