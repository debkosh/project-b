package mockengine

import (
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
}

// mockLink is the text a phone's QR code carries, as the engine words one.
const mockLink = "FIDO:/0123456789012345678901234567890123456789012345678901234567890123456789"

// openPasskey is a page in tab asking for a passkey: value is "create" or "get", with
// " nophone" after it when phone as key isn't there.
func (e *Engine) openPasskey(tab engine.TabID, value string) {
	request, rest, _ := strings.Cut(value, " ")
	r := &passkeyRequest{request: request, phone: rest != "nophone"}
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
	b, _ := json.Marshal(map[string]any{"request": r.request, "site": "example.com", "phone": r.phone})
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
	}
}

// PasskeyAnswers are what the app told the engine of passkey steps, in order.
func (e *Engine) PasskeyAnswers() []PasskeyAnswer {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]PasskeyAnswer(nil), e.dialogs.passkeyAnswers...)
}
