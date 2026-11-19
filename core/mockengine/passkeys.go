package mockengine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"

	"projectb/engine"
)

// A page's passkey request, as the Mac's engine plays one (docs/passkeys.md): a dialog of
// kind passkey whose steps follow. A test starts one and fires the steps a key would
// bring; the mock itself answers what the engine answers without a key: Use a phone, Use
// a security key, Try again and Cancel.

// PasskeyAnswer is one thing the app told the engine the person did at a passkey step.
type PasskeyAnswer struct {
	ID     engine.DialogID `json:"id"`
	Action string          `json:"action"`
	Text   string          `json:"text"`
}

// passkeyRequest is a request waiting: how it started, for the steps the mock replays.
type passkeyRequest struct {
	request string
	phone   bool
	// Our own passkeys can answer it; they are the only way, as on a phone; the
	// credential IDs a sign-in allows and a new passkey's site already has, base64url; and
	// who a new passkey is for.
	own, only      bool
	allow, exclude []string
	user           string
}

// MockClientDataHash is the hash of the client data the mock asks our own passkeys to sign.
var MockClientDataHash = bytes.Repeat([]byte{7}, 32)

// mockLink is the text a phone's QR code carries, as the engine words one.
const mockLink = "FIDO:/0123456789012345678901234567890123456789012345678901234567890123456789"

// openPasskey is a page in tab asking for a passkey: value is "create" or "get", and after
// it, each after a space: nophone when phone as key isn't there; own when our own passkeys
// can answer, only when they are the only way; allow= and exclude= with credential IDs
// between commas; user= with the name a new passkey is for.
func (e *Engine) openPasskey(tab engine.TabID, value string) {
	words := strings.Fields(value)
	r := &passkeyRequest{request: words[0], phone: true, user: "asha"}
	for _, w := range words[1:] {
		k, v, _ := strings.Cut(w, "=")
		switch k {
		case "nophone":
			r.phone = false
		case "own":
			r.own = true
		case "only":
			r.own, r.only, r.phone = true, true, false
		case "allow":
			r.allow = strings.Split(v, ",")
		case "exclude":
			r.exclude = strings.Split(v, ",")
		case "user":
			r.user = v
		}
	}
	e.mu.Lock()
	if e.dialogs.waiting == nil {
		e.dialogs.waiting = map[engine.DialogID]engine.TabID{}
	}
	if e.dialogs.passkeys == nil {
		e.dialogs.passkeys = map[engine.DialogID]*passkeyRequest{}
	}
	e.dialogs.next++
	id := e.dialogs.next
	e.dialogs.waiting[id] = tab
	e.dialogs.passkeys[id] = r
	e.mu.Unlock()
	e.listener.DialogOpened(engine.Dialog{ID: id, Tab: tab, Kind: "passkey", Message: "example.com", Origin: "https://example.com"})
	e.keyStep(id, r)
}

func (e *Engine) keyStep(id engine.DialogID, r *passkeyRequest) {
	carries := map[string]any{"request": r.request, "site": "example.com", "phone": r.phone}
	if r.own {
		carries["own"], carries["only"] = true, r.only
		if r.request == "get" {
			carries["allow"] = r.allow
		}
	}
	b, _ := json.Marshal(carries)
	e.listener.PasskeyStep(id, "key", string(b))
}

// latestPasskey is the newest request waiting, or 0.
func (e *Engine) latestPasskey() (engine.DialogID, *passkeyRequest) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var latest engine.DialogID
	for id := range e.dialogs.passkeys {
		latest = max(latest, id)
	}
	return latest, e.dialogs.passkeys[latest]
}

// passkeyStep brings the newest request to a step, as a key or a phone would: value is
// the step's name, then what it carries as JSON; "end" ends the request from the engine's
// side, as when the key answered or the page gave up.
func (e *Engine) passkeyStep(value string) {
	id, _ := e.latestPasskey()
	if id == 0 {
		return
	}
	step, carries, _ := strings.Cut(value, " ")
	if step == "end" {
		e.endPasskey(id)
		return
	}
	if carries == "" {
		carries = "{}"
	}
	e.listener.PasskeyStep(id, step, carries)
}

func (e *Engine) endPasskey(id engine.DialogID) {
	e.mu.Lock()
	_, waiting := e.dialogs.passkeys[id]
	delete(e.dialogs.passkeys, id)
	delete(e.dialogs.waiting, id)
	e.mu.Unlock()
	if waiting {
		e.listener.DialogClosed(id)
	}
}

func (e *Engine) AnswerPasskey(id engine.DialogID, action, text string) {
	e.mu.Lock()
	r := e.dialogs.passkeys[id]
	if r != nil {
		e.dialogs.passkeyAnswers = append(e.dialogs.passkeyAnswers, PasskeyAnswer{ID: id, Action: action, Text: text})
	}
	e.mu.Unlock()
	if r == nil {
		return
	}
	// The engine answers these from inside the call, as content does.
	switch action {
	case "cancel":
		e.endPasskey(id)
	case "phone":
		b, _ := json.Marshal(map[string]any{"link": mockLink, "bluetooth": "on", "canTurnOn": false})
		e.listener.PasskeyStep(id, "phone", string(b))
	case "key", "retry":
		e.keyStep(id, r)
	case "own":
		// The request goes to our own passkeys, which ask Go to make or to sign.
		if !r.own {
			return
		}
		carries := map[string]any{"clientDataHash": base64.RawURLEncoding.EncodeToString(MockClientDataHash)}
		step := "ownSign"
		if r.request == "create" {
			step = "ownMake"
			carries = map[string]any{"exclude": r.exclude, "user": map[string]string{
				"id": base64.RawURLEncoding.EncodeToString([]byte("handle-" + r.user)), "name": r.user + "@example.com", "displayName": strings.ToUpper(r.user[:1]) + r.user[1:] + " Rao"}}
		}
		b, _ := json.Marshal(carries)
		e.listener.PasskeyStep(id, step, string(b))
	case "made", "signed", "ownFailed":
		// The page has its answer, and the request ends.
		e.endPasskey(id)
	}
}

// PasskeyAnswers are what the app told the engine of passkey steps, in order.
func (e *Engine) PasskeyAnswers() []PasskeyAnswer {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]PasskeyAnswer(nil), e.dialogs.passkeyAnswers...)
}
